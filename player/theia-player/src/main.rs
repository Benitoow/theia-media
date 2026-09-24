//! Theia's native player.
//!
//! A Tauri window whose only visible layer is the OSD: libmpv draws the film
//! into a child surface of that window, and the Svelte page floats above it
//! (spike A2b). The Rust side owns the engine, the session and the lifecycle;
//! the page owns every sentence the user reads, which is decision 25 applied to
//! a second interface.
//!
//! Usage:
//!   theia-player --media <path>   play one local file, for development
//!   theia-player                  browse the network for a server, or be told
//!                                 an address by the OSD
//!   theia-player --version        print which build this is and exit
//!   theia-player --diagnostics    print the session state once a second
//!   theia-player --window 640x360 --window-report <path>
//!                                 size the window to that many *real* pixels
//!                                 through Tauri's own API, and write what the
//!                                 window and the page inside it measured to
//!                                 that file. The verification path for the
//!                                 window's declared minimum (open risk 6 in
//!                                 docs/v3.3.md); it changes nothing else.

mod mpv;
mod server;

use mpv::Engine;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::sync::Mutex;
use std::time::{Duration, Instant};
use tauri::{Emitter, Manager};

/// Which audio path is actually in use.
///
/// Spike A3 measured the failure this exists for: when the endpoint refuses a
/// bitstream format, a player that pinned its audio output goes silent rather
/// than falling back. So passthrough is a *request* here, and the mode is only
/// ever set to what mpv reports back.
#[derive(Clone, Copy, PartialEq, Eq)]
enum AudioMode {
    /// Asking for the untouched stream. May be refused by the endpoint.
    /// Never reached on macOS: nothing is asked for there (`apply_audio_mode`),
    /// so the session starts in PCM and has nothing to withdraw.
    Passthrough,
    /// Decoding to PCM, because the endpoint would not take the raw stream -
    /// and, on macOS, because there is no request to make in the first place.
    Pcm,
}

impl AudioMode {
    fn code(self) -> &'static str {
        match self {
            AudioMode::Passthrough => "passthrough",
            AudioMode::Pcm => "pcm",
        }
    }
}

struct Session {
    engine: Engine,
    audio: AudioMode,
    /// The playback preferences the OSD last sent, mirrored here rather than
    /// read from the static at every use.
    ///
    /// The load path needs them - `load_options` decides `sid=no` from them, and
    /// every load re-applies the subtitle style and the language lists - and the
    /// load path runs while the session lock is held, which is where this file
    /// refuses to reach for a second lock. Written in exactly two places, both of
    /// which take it from [`PLAYBACK`]: when the session is created, and when the
    /// OSD saves the sheet.
    playback: PlaybackPreferences,
    media: Option<String>,
    /// What to call what is playing. The OSD shows this rather than picking a
    /// filename out of a URL, which is what a stream address would reduce to.
    title: Option<String>,
    /// The language the current film was made in, when the server knows it.
    ///
    /// This is what `vo` asks the engine for, and it is set by `play_movie`
    /// alone: a series carries no such field (`SeriesMetadata` in
    /// `internal/library/series.go` has no `original_language`), so an episode
    /// leaves it `None` and `vo` degrades to the file's own default rather than
    /// to a guess. Cleared by every load, because it belongs to the film that
    /// carried it.
    original_language: Option<String>,
    /// When the current file was handed to mpv. The audio watchdog needs a
    /// reference point that is not the playback clock: a refused output stalls
    /// that clock at zero, which is the whole reason the watchdog exists.
    loaded_at: Option<Instant>,
    /// Where this film was asked to start. The audio fallback reloads the file,
    /// and a reload without this would silently send the viewer back to the
    /// beginning - which the first resumed run did, and which nobody would
    /// forgive twice.
    start_at: f64,
    /// The tracks the viewer chose, for the same reason as `start_at`: the
    /// fallback reloads the file, and a reload that forgot the chosen language
    /// would put the film back into a tongue nobody asked for. Cleared when a
    /// different film is loaded, because a choice belongs to the film it was
    /// made for.
    aid: Option<i64>,
    sid: Option<i64>,
    /// The subtitle files sitting beside this film, which mpv cannot find by
    /// itself: it is handed an HTTP URL and a sidecar is a file on the server's
    /// disk. Kept here with `start_at` and the track choices, and for the same
    /// reason - the audio fallback reloads the file, and a reload that dropped
    /// them would take the subtitles away mid-film.
    sidecars: Vec<Sidecar>,
    /// Whether the automatic sidecar choice has been made for this load. Set
    /// once it has, and by any explicit choice of the viewer's, so a menu
    /// selection is never undone a second later by the player.
    sidecar_decided: bool,
    /// Whether the end of the current file has already been acted on.
    ///
    /// The end of a file lasts for as long as it stays loaded - `keep-open=yes`
    /// keeps it at the end rather than unloading it - so without this the
    /// telemetry thread would start the next episode twice a second. Re-armed by
    /// the engine's own answer rather than by our own load, deliberately: a load
    /// that has just been handed to mpv can still be reporting the file before it,
    /// and how long that lasts is not something this process can see from outside
    /// - a guard re-armed by the load would act inside that window and skip an
    /// episode. See `autoplay_next_episode`.
    end_handled: bool,
    /// The subtitle files still to be handed to mpv for this load.
    ///
    /// A queue rather than a flag, because an add can legitimately fail and
    /// succeed a moment later. Measured: straight after the audio fallback's
    /// reload, `sub-add` answers "error running command" - mpv is still
    /// switching files - and the same command works on the next tick. A boolean
    /// plus a retry of the whole list would add the successful ones twice.
    sidecars_pending: Vec<Sidecar>,
    /// How many ticks have tried. A sidecar the server cannot serve is given up
    /// on rather than retried for the length of the film.
    sidecar_tries: u32,
    /// Why the player is not in passthrough, as a code the OSD turns into a
    /// sentence. None while nothing has gone wrong.
    audio_reason: Option<&'static str>,
    /// Which rung of the quality ladder is loaded, None being the file as it
    /// is. Kept with `start_at` and the track choices because every reload has
    /// to open the stream that was chosen: the audio fallback reloads the film
    /// too, and one that came back at another quality would undo a choice the
    /// viewer made deliberately.
    quality: Option<i64>,
    /// How long the film is, when the stream cannot say.
    ///
    /// A converted stream is a pipe with no length, so the clock would have no
    /// end and the scrub bar nothing to draw against. The server knows - it is
    /// the same number the browser player's clock uses - and this is where the
    /// answer waits.
    duration_hint: Option<f64>,
}

impl Session {
    /// Records that a new film has been handed to mpv. Every path that starts
    /// playback must call this: the audio watchdog keys off it, and a load it
    /// does not know about is a stalled film nobody recovers from.
    ///
    /// It also forgets the previous film's track choices, which belonged to
    /// that film and not to this one, and takes the subtitle files that came
    /// with the new one.
    fn mark_loaded(
        &mut self,
        source: String,
        title: Option<String>,
        start_at: f64,
        sidecars: Vec<Sidecar>,
    ) {
        self.media = Some(source);
        self.title = title;
        self.start_at = start_at;
        // A new film starts as the file is: the rung belonged to the one before
        // it, and so did the tracks. The original language belonged to the film
        // as well, and `play_movie` sets it back when it has one - an episode
        // never does.
        self.original_language = None;
        self.quality = None;
        self.aid = None;
        self.sid = None;
        self.sidecars = sidecars;
        self.sidecars_pending = self.sidecars.clone();
        self.sidecar_tries = 0;
        self.sidecar_decided = false;
        self.loaded_at = Some(Instant::now());
    }

    /// Hands a file to mpv with everything this session has decided about it:
    /// where to start, which tracks, and the subtitle files beside the film.
    ///
    /// The first load and every reload come through here. They must: a reload
    /// that forgot any of it would undo a viewer's choices, which the resume
    /// point already taught this project once, and would take the sidecar
    /// subtitles away the moment the endpoint refused a bitstream.
    ///
    /// `pause` says what the film should do when it comes back, and only one
    /// caller passes it: a film started from the library plays, whatever the
    /// engine was doing before. Every reload passes `None` and inherits, which
    /// is both what a viewer means and what the engine does anyway.
    fn load(&mut self, url: &str, pause: Option<bool>) -> Result<(), String> {
        let options = self.load_options();
        let mut args: Vec<&str> = vec!["loadfile", url, "replace", "0"];
        if let Some(ref options) = options {
            args.push(options);
        }
        self.engine.command(&args)?;
        // The pause state is a property, set after the load rather than passed
        // with it. `loadfile`'s option list is for options that belong to a
        // *file* - `start=`, `aid=`, `sid=` - and `pause` is the player's own
        // state, which the load keeps from the file before it. Measured on
        // 24 September 2026: with `pause=no` in that list the next film still
        // came back paused, and a viewer who paused one film and opened another
        // got a frozen picture. `None` leaves the state alone, which is what the
        // audio fallback wants.
        if let Some(pause) = pause {
            self.engine
                .set_property("pause", if pause { "yes" } else { "no" })?;
        }
        // A reload drops everything that was added on top of the file, so the
        // two automatic steps below start again - from the same film, whose
        // sidecars this session still holds.
        self.sidecars_pending = self.sidecars.clone();
        self.sidecar_tries = 0;
        self.sidecar_decided = false;
        Ok(())
    }

    /// Where the film is, on the film's own clock.
    ///
    /// A converted stream is a pipe that starts at zero, so the `t=` it was
    /// asked for is an offset every number from mpv needs: without it the clock,
    /// the scrub bar and the resume point would each be short by however far in
    /// the viewer was when they changed quality.
    fn position(&self) -> Option<f64> {
        let raw = self
            .engine
            .property("time-pos")
            .and_then(|value| value.parse::<f64>().ok())?;
        Some(if self.quality.is_some() {
            self.start_at + raw
        } else {
            raw
        })
    }

    /// How long the film is: mpv's own answer for a file, the server's for a
    /// pipe.
    fn duration(&self) -> Option<f64> {
        if self.quality.is_some() {
            return self.duration_hint;
        }
        self.engine
            .property("duration")
            .and_then(|value| value.parse::<f64>().ok())
    }

    /// Opens the film again at another rung of the ladder, or at the same one.
    ///
    /// Everything that belongs to the *film* stays - its title, its subtitle
    /// files, the tracks the viewer chose - and only the stream address and
    /// where to start change. `mark_loaded` is the wrong tool for this: it
    /// forgets the track choices, and those belong to the film rather than to
    /// the stream it happens to be fetched from.
    ///
    /// `start_at` is where the film is *now*, not where it was first asked to
    /// start: a quality change ten minutes into a film that came back at the
    /// resume point would be a viewer losing ten minutes.
    fn reload_at(
        &mut self,
        source: String,
        start_at: f64,
        quality: Option<i64>,
        duration_hint: Option<f64>,
    ) -> Result<(), String> {
        self.media = Some(source.clone());
        self.start_at = start_at;
        self.quality = quality;
        self.duration_hint = duration_hint;
        // The audio watchdog keys off this, and a reload it does not know about
        // is a stalled film nobody recovers from.
        self.loaded_at = Some(Instant::now());
        // `None`: a reload keeps the pause state the engine already has, which
        // is what a viewer means - measured in the real window, a film paused
        // before a quality change came back paused and one playing came back
        // playing. Forcing it either way is worse than inheriting: `pause=yes`
        // set just after `loadfile` is lost when the new file starts, which was
        // tried first and left a paused film playing.
        self.load(&source, None)
    }

    /// Hands mpv the subtitle files that sit beside the film, keeping back the
    /// ones it refuses.
    ///
    /// They are added rather than passed with the load, because `sub-files=`
    /// carries no language and the OSD's menu leads with the language: a track
    /// named after a URL is a menu that cannot answer the only question being
    /// asked of it. `auto` takes the track only when nothing else is selected,
    /// so a film carrying its own subtitle keeps it.
    fn add_sidecars(&mut self) {
        let mut refused = Vec::new();
        for sidecar in std::mem::take(&mut self.sidecars_pending) {
            let mut add: Vec<&str> = vec!["sub-add", &sidecar.url, "auto"];
            // The language goes in the title position as well as its own,
            // because mpv fills an empty title by deriving one from the URL -
            // measured, the menu then read "6?profile=1". The OSD leads with the
            // language when it has one (the web player's own rule), so a title
            // that repeats the language is never drawn twice.
            let name = if sidecar.title.is_empty() {
                sidecar.language.as_str()
            } else {
                sidecar.title.as_str()
            };
            if !name.is_empty() {
                add.push(name);
                if !sidecar.language.is_empty() {
                    add.push(&sidecar.language);
                }
            }
            match self.engine.command(&add) {
                Ok(()) => println!("theia-player: subtitle file added: {}", sidecar.url),
                Err(e) => {
                    eprintln!("theia-player: adding {} failed: {e}", sidecar.url);
                    refused.push(sidecar);
                }
            }
        }
        self.sidecars_pending = refused;
    }

    /// The file-local options this film should be opened with. Used for the
    /// first load and again by the audio fallback, so both open the same film
    /// the same way - which is the whole point of keeping them here.
    ///
    fn load_options(&self) -> Option<String> {
        let mut options: Vec<String> = Vec::new();
        // A converted stream already begins where it was asked to: its `t=` is
        // in the address, and `start=` on top of it would decode and throw away
        // the same seconds a second time - or, worse, on a pipe with no ranges,
        // fail to skip at all.
        if self.start_at > 0.0 && self.quality.is_none() {
            options.push(format!("start={:.3}", self.start_at));
        }
        if let Some(aid) = self.aid {
            options.push(format!("aid={aid}"));
        }
        if let Some(sid) = self.sid {
            options.push(format!("sid={sid}"));
        } else if self.playback.subtitle_language == SubtitleLanguage::Off {
            // Somebody asked for no subtitles, and has not chosen a track for
            // this film, so the file is opened saying so. `sid=no` is mpv's own
            // word for it and a *file-local* option for the same reason `start=`
            // is: it belongs to the file being opened rather than to the player.
            // It matters that this is not left to `slang`: an empty list is the
            // file's own default, which is the opposite of what was asked.
            options.push("sid=no".to_string());
        }
        if options.is_empty() {
            None
        } else {
            Some(options.join(","))
        }
    }

    /// Returns the engine to its library state without ending the process.
    fn clear_media(&mut self) {
        self.media = None;
        self.title = None;
        self.original_language = None;
        self.loaded_at = None;
        self.start_at = 0.0;
        self.quality = None;
        self.duration_hint = None;
        self.aid = None;
        self.sid = None;
        self.sidecars.clear();
        self.sidecars_pending.clear();
        self.sidecar_tries = 0;
        self.sidecar_decided = false;
        // Nothing is loaded any more, so nothing has ended: the guard must not
        // survive the viewer closing the film, or a later film's end would be
        // read as one already dealt with.
        self.end_handled = false;
        self.audio_reason = None;
    }
}

static SESSION: Mutex<Option<Session>> = Mutex::new(None);

/// The connected server, kept apart from the session because browsing a library
/// must work even when the engine could not start: a player that shows nothing
/// at all because a DLL is missing is worse than one that says so.
static CLIENT: Mutex<Option<server::Client>> = Mutex::new(None);

#[derive(Clone, Copy)]
enum Playing {
    Movie(i64),
    /// The episode playing, and the one the server says follows it.
    ///
    /// The next id travels with the id of what is playing rather than in a
    /// static of its own, so that "what plays next" has one answer and one
    /// owner, and so the paths that already clear the media - `player_stop`,
    /// `clear_media` - clear this too. `None` is not "do not autoplay": it is
    /// the server saying there is no next episode.
    Episode { id: i64, next: Option<i64> },
}

/// The playable record currently owned by mpv. Keeping the kind beside the id
/// matters: films and episodes have deliberately parallel progress endpoints,
/// not one polymorphic endpoint pretending the distinction does not exist.
static CURRENT_MEDIA: Mutex<Option<Playing>> = Mutex::new(None);

// ---------------------------------------------------------------------------
// Playback preferences
//
// The settings sheet owns the widgets and the sentences; what each choice
// *means* to the engine is decided here, once, beside the properties it becomes.
// None of it is a matter of taste - every rule below is a unit conversion or a
// precedence, and each one carries the reason it is what it is. The engine was
// probed headlessly on this machine (libmpv-2.dll, client API 2.5, mpv 0.41) for
// every default and every range quoted here; the numbers are not read off a
// manual.
// ---------------------------------------------------------------------------

/// Everything the settings sheet decides about how a film is played.
///
/// `serde(default)` on the container and on every field, because the two
/// programs are updated separately: a payload written before one of these existed
/// is a payload with a default in it, not a refusal. The defaults are the ones
/// the sheet ships (the same table is in `player/ui/src/types.ts`) and they have
/// to agree: a `Default` that disagreed would change the picture for a viewer who
/// never opened the settings, with nothing anywhere to say why.
#[derive(Clone, Debug, PartialEq, serde::Deserialize, serde::Serialize)]
#[serde(default, rename_all = "camelCase")]
struct PlaybackPreferences {
    /// Whether the next episode follows this one when the file ends.
    auto_play_next: bool,
    audio_language: AudioLanguage,
    subtitle_language: SubtitleLanguage,
    subtitle_style: SubtitleStyle,
}

impl Default for PlaybackPreferences {
    fn default() -> Self {
        PlaybackPreferences {
            auto_play_next: true,
            audio_language: AudioLanguage::Auto,
            subtitle_language: SubtitleLanguage::Auto,
            subtitle_style: SubtitleStyle::default(),
        }
    }
}

impl PlaybackPreferences {
    /// Refuses a payload that carries the right fields with values that are not
    /// the ones the sheet can produce.
    ///
    /// Serde has already answered for the languages, the outline and the font -
    /// those are enumerated below, so an unknown one is a named error rather than
    /// a silent fallback. The two colours travel as free text, because a
    /// background is `none` or a colour, and a value that is neither has no
    /// defensible reading: every other setting can fall back to a default the
    /// sheet also ships, while a string that is not a colour can only be drawn as
    /// whatever the engine makes of it.
    fn validate(&self) -> Result<(), String> {
        let style = &self.subtitle_style;
        if rgb(&style.colour).is_none() {
            return Err(format!(
                "subtitleStyle.colour is {:?}, which is not a \"#RRGGBB\" colour",
                style.colour
            ));
        }
        if style.background != "none" && rgb(&style.background).is_none() {
            return Err(format!(
                "subtitleStyle.background is {:?}, which is neither \"none\" nor a \"#RRGGBB\" colour",
                style.background
            ));
        }
        Ok(())
    }
}

/// Which audio track the player asks the engine for.
#[derive(Clone, Copy, Debug, PartialEq, Eq, serde::Deserialize, serde::Serialize)]
#[serde(rename_all = "lowercase")]
enum AudioLanguage {
    /// Whatever the file itself was made to prefer.
    Auto,
    /// The French version, which is the one this household watches.
    Vf,
    /// The language the title was made in, when the server knows it.
    Vo,
}

/// Which subtitle track the player asks the engine for.
///
/// `Off` is not `Auto` with a different list: auto leaves the file's own answer
/// alone, off is somebody saying they do not want subtitles. Only a load option
/// can say that (see [`Session::load_options`]).
#[derive(Clone, Copy, Debug, PartialEq, Eq, serde::Deserialize, serde::Serialize)]
#[serde(rename_all = "lowercase")]
enum SubtitleLanguage {
    Auto,
    #[serde(rename = "none")]
    Off,
    Fr,
    En,
}

/// How the outline around the text is drawn, when there is no background colour
/// to draw instead - the precedence itself is in [`outline_properties`].
#[derive(Clone, Copy, Debug, PartialEq, Eq, serde::Deserialize, serde::Serialize)]
#[serde(rename_all = "lowercase")]
enum Outline {
    Shadow,
    Outline,
    #[serde(rename = "none")]
    Off,
}

/// Which family the engine draws the text in.
#[derive(Clone, Copy, Debug, PartialEq, Eq, serde::Deserialize, serde::Serialize)]
#[serde(rename_all = "lowercase")]
enum Font {
    Standard,
    Serif,
    Mono,
}

impl Font {
    /// The three generic names the engine's own font provider understands.
    ///
    /// Generic rather than named families: the engine resolves them through
    /// fontconfig on Unix and DirectWrite on Windows, which know what is
    /// installed - and the OSD ships no font the engine could load anyway, so a
    /// family name of ours would be one it had never heard of.
    fn family(self) -> &'static str {
        match self {
            Font::Standard => "sans-serif",
            Font::Serif => "serif",
            Font::Mono => "monospace",
        }
    }
}

/// The look of a subtitle line, in the units the settings sheet speaks.
#[derive(Clone, Debug, PartialEq, serde::Deserialize, serde::Serialize)]
#[serde(default, rename_all = "camelCase")]
struct SubtitleStyle {
    /// Text size, in pixels at a 1080-tall window. The sheet's slider runs 24 to
    /// 72 in steps of 2, and 36 sits in the middle of it.
    size_px: f64,
    /// How far above the bottom of the picture the text sits, in the same
    /// 1080-referenced pixels. The slider runs 24 to 200 in steps of 4.
    height_px: f64,
    /// The text colour, `#RRGGBB`. The interface never sends alpha.
    colour: String,
    outline: Outline,
    /// `none`, or a `#RRGGBB` the outline is replaced by.
    background: String,
    font: Font,
    bold: bool,
}

impl Default for SubtitleStyle {
    fn default() -> Self {
        SubtitleStyle {
            size_px: 36.0,
            height_px: 120.0,
            colour: "#EDE7DC".to_string(),
            outline: Outline::Shadow,
            background: "none".to_string(),
            font: Font::Standard,
            bold: false,
        }
    }
}

/// The preferences the OSD last sent.
///
/// Process-wide rather than per-film, because they are a setting and not a
/// property of what is on screen, and kept even when there is nothing to apply
/// them to: a player whose engine would not start still has to answer the sheet,
/// and the OSD keeps its own copy in `localStorage` for exactly the same reason.
/// Lazy because a `PlaybackPreferences` holds `String`s and a `static` needs a
/// const constructor.
static PLAYBACK: std::sync::LazyLock<Mutex<PlaybackPreferences>> =
    std::sync::LazyLock::new(|| Mutex::new(PlaybackPreferences::default()));

/// The preferences as they stand, copied out.
fn stored_playback() -> PlaybackPreferences {
    PLAYBACK.lock().unwrap().clone()
}

/// The settings sheet's save, and its first call after the page mounts.
///
/// The whole object arrives as one JSON string rather than as an argument per
/// setting: three of the four fields are nested or enumerated, and a command
/// whose signature had to be kept in step with a form would be a second copy of
/// the form.
///
/// Nothing is applied until the payload has been read and checked, and what has
/// been read is stored before it is applied: an engine that refused a property,
/// or that never started at all, must not leave the player holding choices
/// nobody made. An engine that is absent is an `Err` - the OSD answers that by
/// keeping the same choices in `localStorage` - but the store still happened.
#[tauri::command]
fn player_set_playback(prefs: String) -> Result<(), String> {
    let parsed: PlaybackPreferences =
        serde_json::from_str(&prefs).map_err(|e| format!("reading the playback preferences: {e}"))?;
    parsed.validate()?;
    *PLAYBACK.lock().unwrap() = parsed;
    apply_playback()
}

/// What the engine is actually holding for subtitles, read back from it.
///
/// The settings sheet decides seven things and sends them; whether the engine
/// took them is a different question, and it is the one a report of "the slider
/// does nothing" turns on. A property the engine refuses keeps the value it
/// already had, so reading these back is a measurement rather than a restatement
/// of what was sent. The two language lists travel with them because they are
/// what the *next* load will choose tracks with, and an empty one is what "the
/// file's own default" looks like from the outside.
fn player_subtitles() -> String {
    let guard = SESSION.lock().unwrap();
    let Some(session) = guard.as_ref() else {
        return "engine: not running".into();
    };
    let held = |name: &str| session.engine.property(name).unwrap_or_else(|| "?".into());
    let style = &session.playback.subtitle_style;
    format!(
        "asked {:.0}px at {:.0}px, {:?}, band {}, {:?}, {} | sub-font-size={} sub-pos={} \
sub-margin-y={} sub-color={} sub-border-style={} sub-border-size={} sub-shadow-offset={} \
sub-back-color={} sub-font={} sub-bold={} sub-ass-override={} sub-ass-force-style={} | \
alang={:?} slang={:?}",
        style.size_px,
        style.height_px,
        style.outline,
        style.background,
        style.font,
        if style.bold { "bold" } else { "normal" },
        held("sub-font-size"),
        held("sub-pos"),
        held("sub-margin-y"),
        held("sub-color"),
        held("sub-border-style"),
        held("sub-border-size"),
        held("sub-shadow-offset"),
        held("sub-back-color"),
        held("sub-font"),
        held("sub-bold"),
        held("sub-ass-override"),
        held("sub-ass-force-style"),
        session.engine.property("alang").unwrap_or_default(),
        session.engine.property("slang").unwrap_or_default(),
    )
}

/// Applies the stored preferences to the running engine, and nowhere else.
///
/// The engine is asked; it is never told what to think. Every value is set as a
/// property, so the style is live on a film that is already playing - which is
/// what makes the sheet's sliders answer immediately rather than at the next
/// film.
fn apply_playback() -> Result<(), String> {
    // Read and released before the session lock is taken: this file never holds
    // two of its locks at once, and that is the whole of its deadlock policy.
    let prefs = stored_playback();
    let mut guard = SESSION.lock().unwrap();
    let session = guard.as_mut().ok_or("the engine is not running")?;
    // The session keeps its own copy because `load_options` reads it while the
    // session lock is held, and this is one of the two places that writes it.
    session.playback = prefs;
    apply_playback_to(&session.engine, &session.playback, session.original_language.as_deref())
}

/// Sets every property the preferences decide, and names the ones the engine
/// would not take.
///
/// A refusal is a real error code with a reason in it - measured: an unknown
/// value for `sub-border-style` answers "unsupported format for accessing
/// property" - and a player that read that as success would render something
/// nobody chose while reporting that it had. So each refusal is collected and
/// named, and the caller decides what to do with the answer: the settings sheet
/// is told, a film is not stopped by one.
fn apply_playback_to(
    engine: &Engine,
    prefs: &PlaybackPreferences,
    original_language: Option<&str>,
) -> Result<(), String> {
    let mut refused: Vec<String> = Vec::new();
    for (name, value) in playback_properties(prefs, original_language) {
        if let Err(e) = engine.set_property(name, &value) {
            refused.push(e);
        }
    }
    if refused.is_empty() {
        Ok(())
    } else {
        Err(format!("the engine refused {}", refused.join("; ")))
    }
}

/// Every property the preferences decide.
///
/// One list rather than the same calls spelled out in two places: the set is
/// applied when the sheet is saved and again before every load, and a second
/// list would drift from this one the first time a setting was added.
fn playback_properties(
    prefs: &PlaybackPreferences,
    original_language: Option<&str>,
) -> Vec<(&'static str, String)> {
    let style = &prefs.subtitle_style;
    let mut properties: Vec<(&'static str, String)> = vec![
        (
            "sub-font-size",
            format!("{:.2}", subtitle_font_size(style.size_px)),
        ),
        (
            "sub-pos",
            format!("{:.2}", subtitle_position(style.height_px)),
        ),
        // Zeroed rather than left alone. The position above puts the band where
        // the slider says; the engine's own margin default is 34 scaled pixels,
        // which would push it up by about 23 of ours - or by whatever a previous
        // session or the user's own mpv.conf left behind. A margin that is not
        // part of the setting has no business being added to it.
        ("sub-margin-y", "0".to_string()),
        ("sub-color", opaque_colour(&style.colour)),
        // `force` rather than the engine's own default of `scale`, and this one is
        // measured: with `yes` the script kept its own font, size and colour - a
        // bone 24 px request was drawn as 27 px white Arial - and a style setting
        // that applies only to the tracks a film does not have is not a setting.
        ("sub-ass-override", "force".to_string()),
        // The script's own margins emptied, and nothing else: `sub-margin-y` is
        // not consulted on an ASS track (see `subtitle_position`), so this is what
        // stops a MarginV the script brought from winning. `Alignment` is
        // deliberately not forced - a script that places a line elsewhere keeps
        // its side of the picture, which is the only reason to force one field
        // rather than the whole style.
        ("sub-ass-force-style", "MarginV=0".to_string()),
        // Set rather than assumed, although it is the engine's own default: the
        // 2/3 conversion above is true only while this is on, and this process
        // reads the user's own mpv.conf at startup, which is free to have turned
        // it off. A player that inherits its units renders a size nobody chose.
        ("sub-scale-by-window", "yes".to_string()),
        ("sub-font", style.font.family().to_string()),
        ("sub-bold", if style.bold { "yes" } else { "no" }.to_string()),
    ];
    // The outline inverts on dark text - part of what the sheet promises - and
    // both colours are set even when a background box makes them inert, so that
    // clearing the background gives a correct outline back on the same tick.
    let ink = outline_ink(&style.colour);
    properties.push(("sub-border-color", ink.to_string()));
    properties.push(("sub-shadow-color", ink.to_string()));
    properties.push(("alang", audio_language_list(prefs, original_language)));
    properties.push(("slang", subtitle_language_list(prefs)));
    properties.extend(outline_properties(style));
    properties
}

/// How the outline is drawn: a precedence the settings sheet does not have to
/// know about.
///
/// A background colour wins over the outline, because a band behind the text is
/// what somebody who chose one is asking for, and it is drawn at 70 % alpha so
/// the picture stays visible through it. The style is `opaque-box` and not
/// `background-box`: the sheet's promise is a band like the one television draws
/// across the whole width of the picture, and `background-box` would fit the box
/// to the text instead.
///
/// With no background there are three cases and the engine has two styles:
///
/// * `outline` is `outline-and-shadow` with a border and no shadow. Both
///   properties are set in every case, because a `sub-shadow-offset` left over
///   from the case before is how a setting stops meaning anything.
/// * `shadow` is the same style with the border at zero and a shadow instead -
///   one engine style, two looks, which is what the engine offers.
/// * `none` has no style of its own: the engine's `sub-border-style` choices are
///   `outline-and-shadow`, `opaque-box` and `background-box`, and an unknown
///   value is refused outright. A `background-box` with alpha zero is how this
///   says none - transparent, and fitted to the text, so that nothing shows.
fn outline_properties(style: &SubtitleStyle) -> Vec<(&'static str, String)> {
    if style.background != "none" {
        let digits = style
            .background
            .strip_prefix('#')
            .unwrap_or(&style.background);
        return vec![
            ("sub-border-style", "opaque-box".to_string()),
            ("sub-back-color", format!("#B3{digits}")),
        ];
    }
    match style.outline {
        Outline::Outline => vec![
            ("sub-border-style", "outline-and-shadow".to_string()),
            ("sub-border-size", "2.4".to_string()),
            ("sub-shadow-offset", "0".to_string()),
        ],
        Outline::Shadow => vec![
            ("sub-border-style", "outline-and-shadow".to_string()),
            ("sub-border-size", "0".to_string()),
            ("sub-shadow-offset", "1.2".to_string()),
        ],
        Outline::Off => vec![
            ("sub-border-style", "background-box".to_string()),
            ("sub-back-color", "#00000000".to_string()),
        ],
    }
}

/// The engine's `sub-font-size` for a size in 1080-referenced pixels.
///
/// Two thirds, and the reason is a unit rather than a preference: with
/// `sub-scale-by-window` on, the engine measures subtitle text in pixels of a
/// 720-tall window, so 720/1080 turns a size chosen against a 1080-tall picture
/// into the engine's own unit and the text keeps its share of whatever window it
/// is drawn in. The sheet says 36 and the engine is given 24.00.
fn subtitle_font_size(size_px: f64) -> f64 {
    size_px * 2.0 / 3.0
}

/// The engine's `sub-pos` for a height above the bottom of the picture, in
/// 1080-referenced pixels: 100 % minus the same 2/3 conversion.
///
/// The position does the positioning, not the margin, and that is measured
/// rather than chosen. On a track that carries its own ASS script, `sub-margin-y`
/// is not consulted at all: a script setting MarginV=200 put its glyphs 207 px
/// from the bottom of a 720-tall picture while `sub-margin-y` asked for 80, and
/// that was with `sub-ass-override=force`. `sub-pos` is consulted on both kinds,
/// so the height is turned into a percentage of the picture - heightPx * 2/3
/// engine pixels over 720, which is heightPx / 10.8 - and taken off 100 %.
///
/// Measured after, at 1280x720: the glyph bottoms land at heightPx * 2/3 + 4 px,
/// identically for an SRT sidecar and for that ASS file - 40 px of height gives
/// 31, 120 gives 84, 200 gives 137. The 4 px are the descender below the
/// baseline and are the same on both kinds, so what is being placed is the box
/// the text sits in rather than the glyphs, which is exactly what the sheet's own
/// preview draws.
fn subtitle_position(height_px: f64) -> f64 {
    100.0 - height_px / 10.8
}

/// The six hex digits of a colour written `#RRGGBB`, or nothing for any other
/// shape. One rule, asked by the two callers that need it, so that "is this a
/// colour" has a single answer.
fn rgb_digits(colour: &str) -> Option<&str> {
    let digits = colour.strip_prefix('#')?;
    if digits.len() == 6 && digits.bytes().all(|byte| byte.is_ascii_hexdigit()) {
        Some(digits)
    } else {
        None
    }
}

/// `#RRGGBB` as three numbers, for the luminance below.
fn rgb(colour: &str) -> Option<(u8, u8, u8)> {
    let digits = rgb_digits(colour)?;
    let value = u32::from_str_radix(digits, 16).ok()?;
    Some(((value >> 16) as u8, (value >> 8) as u8, value as u8))
}

/// `#RRGGBB` as the engine takes it: eight digits, alpha first.
///
/// Measured against the engine: it prints colours as `#AARRGGBB` and reads eight
/// digits the same way, with the alpha leading. The sheet sends six, which carry
/// none - so the opaque `FF` is written here, saying what the colour is rather
/// than leaving the engine to infer it.
/// [`PlaybackPreferences::validate`] has already refused anything in another
/// shape; this passes such a value through unchanged rather than mangling it, so
/// the engine's own refusal is what gets logged.
fn opaque_colour(colour: &str) -> String {
    match rgb_digits(colour) {
        Some(digits) => format!("#FF{digits}"),
        None => colour.to_string(),
    }
}

/// Whether text in this colour reads as dark, by relative luminance.
///
/// The plain weighted sum of the sRGB values over 255, thresholded at 0.5 - not
/// the gamma-decoded one: what is being decided is which way round the outline
/// goes, and that is a question about the colour as drawn rather than about the
/// light it stands for.
fn is_dark(colour: &str) -> bool {
    let Some((red, green, blue)) = rgb(colour) else {
        // A colour that cannot be read is treated as light, which is what nearly
        // every subtitle is and which is the pair the interface's own ink is
        // drawn with.
        return false;
    };
    let luminance = 0.2126 * f64::from(red) / 255.0
        + 0.7152 * f64::from(green) / 255.0
        + 0.0722 * f64::from(blue) / 255.0;
    luminance < 0.5
}

/// The colour the outline and the shadow take: black behind light text, and the
/// interface's own paper behind dark text.
///
/// "the outline inverts automatically if you pick dark text" is what the sheet
/// promises, and this is the whole of it - one rule, used by both properties, so
/// the two can never disagree.
fn outline_ink(text_colour: &str) -> &'static str {
    if is_dark(text_colour) {
        "#FFEDE7DC"
    } else {
        "#FF000000"
    }
}

/// What the engine should look for when it picks the audio track.
///
/// An empty list is not "no sound": it is the file's own default, which is what
/// `auto` means and what an unknown original language has to fall back to. The
/// three-letter codes sit beside the two-letter ones because a file's tracks
/// carry whichever its maker wrote - `fre` and `fra` as well as `fr`.
///
/// `vo` is the honest case, and the limit is here: the player asks for the
/// language a title was made in only when the server said what that is, and
/// otherwise asks for nothing at all. Guessing from a filename, a country or the
/// library's own tongue would pick a track nobody asked for.
fn audio_language_list(prefs: &PlaybackPreferences, original_language: Option<&str>) -> String {
    match prefs.audio_language {
        AudioLanguage::Auto => String::new(),
        AudioLanguage::Vf => "fr,fre,fra".to_string(),
        AudioLanguage::Vo => original_language.unwrap_or_default().to_string(),
    }
}

/// What the engine should look for when it picks the subtitle track.
///
/// `auto` and `none` are the same list and different behaviour: auto leaves the
/// file's own answer alone, while none opens the file with `sid=no` (see
/// [`Session::load_options`]). A language list cannot ask for the absence of a
/// track, which is why that half of the choice is not here.
fn subtitle_language_list(prefs: &PlaybackPreferences) -> String {
    match prefs.subtitle_language {
        SubtitleLanguage::Auto | SubtitleLanguage::Off => String::new(),
        SubtitleLanguage::Fr => "fr,fre,fra".to_string(),
        SubtitleLanguage::En => "en,eng".to_string(),
    }
}

/// The engine options that differ by platform: how the picture reaches the
/// window, and how the sound leaves the machine.
///
/// Windows is exactly what this player has always set - D3D11 through gpu-next,
/// WASAPI for sound - and nothing here changes it. The shape is identical on
/// every platform on purpose: `start_engine` sets these options one by one and
/// treats a refusal as fatal, so a value mpv does not know becomes a player that
/// says why it will not start rather than one that renders on another backend in
/// silence.
#[cfg(windows)]
const PLATFORM_OPTIONS: &[(&str, &str)] = &[
    ("vo", "gpu-next"),
    ("gpu-api", "d3d11"),
    ("gpu-context", "d3d11"),
    ("hwdec", "d3d11va"),
    ("ao", "wasapi"),
];

/// macOS: the film is drawn by this program, not by mpv.
///
/// `vo=libmpv` means "no video output at all": the host creates a GL context and
/// calls `mpv_render_context_render` for every frame, which is the only way a
/// frame can reach a window on macOS with this engine - its build disables
/// `vulkan`, `macos-cocoa-cb` and `swift-build`, so none of mpv's own outputs can
/// present, and `--wid` is gone in 0.41. `RENDER-MACOS.md` is the specification
/// and the list of what the Mac still has to settle.
///
/// `videotoolbox` is the engine's hardware decoder (`videotoolbox-gl=enabled` in
/// its pin) and `coreaudio` its audio output. Nothing here asks for bitstream
/// passthrough: CoreAudio has no path for TrueHD, Atmos or DTS-HD MA, so the
/// session starts in PCM and stays there (`apply_audio_mode` is a no-op on macOS).
#[cfg(target_os = "macos")]
const PLATFORM_OPTIONS: &[(&str, &str)] = &[
    ("vo", "libmpv"),
    ("hwdec", "videotoolbox"),
    ("ao", "coreaudio"),
];

/// Every other Unix, unchanged: these are exactly the values the player set on
/// these platforms before macOS joined the product, and `player/libmpv.json`
/// pins no engine for them, so there is nothing here to check against.
#[cfg(not(any(windows, target_os = "macos")))]
const PLATFORM_OPTIONS: &[(&str, &str)] = &[
    ("vo", "gpu-next"),
    ("gpu-api", "d3d11"),
    ("gpu-context", "d3d11"),
    ("hwdec", "d3d11va"),
    ("ao", "wasapi"),
];

/// The options the player starts with. Kept in one place so the policy is
/// readable rather than scattered through the setup closure.
///
/// `wid` comes first and is the handle the platform gave us: an `HWND` on
/// Windows, an `NSView*` on macOS, nothing on the other Unix. mpv calls it `wid`
/// everywhere, so the entry itself does not vary; what varies is what the handle
/// is, and `main` decides that in one place. On macOS this entry is the Mac's
/// first check: mpv 0.41 no longer reads `wid` there at all - the paragraph
/// documenting an `NSView*` left the manual after 0.36, and no macOS file in
/// 0.41.0 reads the option - so the film is expected to appear in a window mpv
/// opens itself. `start_engine` keeps the player alive either way.
///
/// `silent` starts muted. It exists because a player that always makes a noise
/// is hostile in a shared room - and because every automated run of this
/// program has no business producing sound on somebody's machine.
fn base_options(wid: isize, silent: bool) -> Vec<(&'static str, String)> {
    let mut options: Vec<(&'static str, String)> = Vec::new();
    // `wid` is the handle the platform gave us: an `HWND` on Windows, and nothing
    // on macOS any more - the film there is drawn by this program into its own GL
    // context (`vo=libmpv`, see RENDER-MACOS.md), so mpv is given no window at
    // all. mpv 0.41 stopped reading `wid` on macOS anyway: the paragraph
    // documenting an `NSView*` left the manual after 0.36 and no macOS file in
    // 0.41.0 reads the option.
    #[cfg(not(target_os = "macos"))]
    options.push(("wid", wid.to_string()));
    #[cfg(target_os = "macos")]
    let _ = wid;
    options.extend(PLATFORM_OPTIONS.iter().map(|(k, v)| (*k, (*v).to_string())));
    options.extend([
        ("audio-channels", "auto".into()),
        ("mute", if silent { "yes".into() } else { "no".into() }),
        ("terminal", "no".into()),
        ("msg-level", "all=warn".into()),
        ("title", "theia-player".into()),
        // The film must not end the session: the window stays, and the OSD
        // decides what happens next. Idle keeps the engine alive between files.
        ("idle", "yes".into()),
        ("keep-open", "yes".into()),
        // And the window must exist before the first film does. The Tauri window
        // is transparent and the OSD draws on a transparent page, which is right
        // over a picture and wrong over nothing: measured with no file loaded,
        // the only children of the Tauri window were the WebView2 stack, so the
        // library was being painted over whatever was on the desktop behind it.
        // mpv paints this surface black instead, from startup, and it stays at
        // the bottom of the child z-order where the picture already goes.
        ("force-window", "yes".into()),
    ]);
    options
}

/// The passthrough request, kept apart from the options above because it is
/// withdrawn at runtime when the endpoint refuses it.
///
/// Named for what it is: the WASAPI bitstream path, `audio-spdif` plus
/// `audio-exclusive`. CoreAudio has no equivalent - mpv's own manual documents
/// the only macOS passthrough as `--coreaudio-spdif-hack`, which sends AC-3 and
/// DTS core as float PCM and *disables* normal AC-3 passthrough even on a device
/// that reports it, and neither it nor the `coreaudio` driver's redirection to
/// `coreaudio_exclusive` carries TrueHD, Atmos or DTS-HD MA - so macOS asks for
/// nothing at all and its audio mode is PCM.
#[cfg(not(target_os = "macos"))]
const SPDIF_CODECS: &str = "ac3,eac3,dts,dts-hd,truehd";

/// Asks the engine for the audio path its mode names.
///
/// Windows and the other Unix do exactly what they have always done: a
/// passthrough request, and PCM when it has to be withdrawn.
///
/// macOS does nothing, deliberately and in one place, because it has no request
/// to make and therefore none to withdraw: `SPDIF_CODECS` names formats
/// CoreAudio cannot carry (see above). The mode the session *starts* in is the
/// other half of that decision and lives in `start_engine`.
fn apply_audio_mode(engine: &Engine, mode: AudioMode) -> Result<(), String> {
    #[cfg(target_os = "macos")]
    {
        // The two arms would be the same no-op, so there is one: writing a
        // `audio-spdif` request here and relying on mpv to refuse it is the
        // silent request this port exists to remove.
        let _ = (engine, mode);
    }
    #[cfg(not(target_os = "macos"))]
    match mode {
        AudioMode::Passthrough => {
            engine.set_option("audio-spdif", SPDIF_CODECS)?;
            engine.set_option("audio-exclusive", "yes")?;
        }
        AudioMode::Pcm => {
            engine.set_option("audio-spdif", "")?;
            engine.set_option("audio-exclusive", "no")?;
        }
    }
    Ok(())
}

/// What the interface measured about itself, in the window since the last
/// report.
///
/// The OSD is the half of this application that can stutter without the engine
/// noticing: mpv presents its frames on its own thread, while the page shares
/// one main thread with React, the styles and the compositor. These numbers are
/// how a session says whether the interface kept up, and they exist because
/// "fluid" without a measurement is an opinion.
#[derive(Clone, Default)]
struct OsdStats {
    /// The longest frame interval seen while somebody was doing something, in
    /// milliseconds. This is the number a viewer feels.
    worst_frame_ms: f64,
    /// How many frames in the window took longer than two frames at 60 Hz.
    slow_frames: u32,
    /// How many frames the sampler drew. Zero with a zero worst frame means the
    /// instrument never ran, which is a different answer from "it was smooth".
    frames: u32,
    /// How many resize events arrived in the window: one drag of a window's
    /// edge is not one event, and the count is what says how many.
    resize_events: u32,
}

/// When this process started, so the first status frame can say how long the
/// application took to become useful. The frames begin 500 ms after the window
/// is shown, so the number reads about half a second high - written down here
/// rather than corrected, because a number whose offset is known is worth more
/// than one quietly adjusted.
static STARTED_AT: std::sync::OnceLock<std::time::Instant> = std::sync::OnceLock::new();

/// Written by `player_osd_stats`, read by the status frame. Absent until the
/// interface has reported once, which is the honest answer for "not measured
/// yet" rather than a zero.
static OSD_STATS: std::sync::Mutex<Option<OsdStats>> = std::sync::Mutex::new(None);

#[tauri::command]
fn player_osd_stats(worst_frame_ms: f64, slow_frames: u32, frames: u32, resize_events: u32) {
    if let Ok(mut guard) = OSD_STATS.lock() {
        *guard = Some(OsdStats { worst_frame_ms, slow_frames, frames, resize_events });
    }
}

#[tauri::command]
fn player_status() -> String {
    let guard = SESSION.lock().unwrap();
    let Some(session) = guard.as_ref() else {
        return "{\"ready\":false}".into();
    };
    let engine = &session.engine;
    // Built by a serialiser rather than by hand. The hand-built version emitted
    // mpv's own "no" for a flag, which is not JSON: the OSD parses in a
    // try/catch and would have ignored every frame in silence, leaving a
    // perfectly blank player with nothing in any log to explain it.
    // Absent until the interface has reported once. A zero here would be
    // indistinguishable from a measured zero, which is the difference between
    // "the interface is keeping up" and "nothing is measuring".
    let osd = OSD_STATS.lock().ok().and_then(|guard| guard.clone());
    // Read, not reset: a resize that has just ended is still the answer to
    // "did the window keep up", and the number is cleared when the next one
    // starts rather than by whoever reads it.
    let resize = RESIZE_TIMING.lock().map(|guard| *guard).unwrap_or((None, 0, 0.0, 0.0));
    let text = |name: &str| engine.property(name);
    let number = |name: &str| engine.property(name).and_then(|v| v.parse::<f64>().ok());
    let integer = |name: &str| engine.property(name).and_then(|v| v.parse::<i64>().ok());
    let flag = |name: &str| engine.property(name).map(|v| v == "yes");

    serde_json::json!({
        "ready": true,
        "engine": text("mpv-version"),
        "media": session.media,
        "title": session.title,
        "pause": flag("pause").unwrap_or(false),
        "mute": flag("mute").unwrap_or(false),
        // The engine's own unit is percent; the interface works in fractions,
        // so the conversion happens once, here, and `player_set_volume` accepts
        // what this reports.
        "volume": number("volume").map(|percent| percent / 100.0),
        // The film's clock, not mpv's: on a converted stream the two differ by
        // the `t=` the pipe was asked for. Every number the interface shows
        // about position and length comes through these two.
        "pos": session.position(),
        "duration": session.duration(),
        "vo": text("current-vo"),
        "hwdec": text("hwdec-current"),
        "ao": text("current-ao"),
        "audioMode": session.audio.code(),
        "audioReason": session.audio_reason,
        "videoCodec": text("video-format"),
        "startAt": session.start_at,
        "aid": integer("aid"),
        "sid": integer("sid"),
        // Which rung of the quality ladder is loaded, null being the file as it
        // is. Session state rather than an mpv property: the rung is a fact
        // about the address the film was fetched from.
        "quality": session.quality,
        // The picture's own accounting, which mpv has always kept and this
        // application never asked for: the rate it is actually presenting, and
        // the frames it or its decoder had to throw away. A film that looks
        // wrong is usually one of these two numbers, and no amount of reading
        // the interface would have shown it.
        "fps": number("estimated-vf-fps"),
        "drops": integer("frame-drop-count").unwrap_or(0),
        "decoderDrops": integer("decoder-frame-drop-count").unwrap_or(0),
        // And the interface's, reported by the page itself. Null until it has
        // said anything, which is not the same as zero.
        "sinceStartMs": STARTED_AT.get().map(|start| start.elapsed().as_millis() as u64).unwrap_or(0),
        "osdWorstFrameMs": osd.as_ref().map(|one| one.worst_frame_ms),
        "osdSlowFrames": osd.as_ref().map(|one| one.slow_frames),
        "osdFrames": osd.as_ref().map(|one| one.frames),
        "osdResizeEvents": osd.as_ref().map(|one| one.resize_events),
        "resizeEvents": resize.1,
        "resizeWorstGapMs": (resize.2 * 10.0).round() / 10.0,
        "resizeMeanGapMs": if resize.1 > 1 {
            ((resize.3 / f64::from(resize.1 - 1)) * 10.0).round() / 10.0
        } else {
            0.0
        },
    })
    .to_string()
}

#[tauri::command]
fn player_version() -> Option<String> {
    SESSION.lock().unwrap().as_ref().and_then(|s| s.engine.version())
}

/// The engine the player ships, as it was pinned.
///
/// Decision 118 accepts four obligations in exchange for redistributing libmpv
/// under the LGPL, and one of them is that the exact upstream source and its
/// digest are *named in the application's diagnostics*. This is that: the
/// manifest the build fetched the library with, compiled in, so the answer
/// cannot drift from the file that is actually loaded.
///
/// It is read once, lazily: the OSD asks for the version on load, and a screen
/// that shows which engine is running is worth one parse.
#[tauri::command]
fn player_engine_pin() -> String {
    PINNED_ENGINE.to_string()
}

/// player/libmpv.json, embedded at compile time. `include_str!` rather than a
/// runtime read: a file that ships beside a binary can be edited by anybody, and
/// a diagnostics screen that can be edited is not evidence of anything.
static PINNED_ENGINE: std::sync::LazyLock<String> = std::sync::LazyLock::new(|| {
    let raw: serde_json::Value =
        serde_json::from_str(include_str!("../../libmpv.json")).unwrap_or(serde_json::Value::Null);
    let platform = manifest_platform();
    let pinned = raw
        .get("platforms")
        .and_then(|platforms| platforms.get(&platform))
        .cloned()
        .unwrap_or(serde_json::Value::Null);
    serde_json::json!({
        "platform": platform,
        "engine": raw.get("engine").cloned().unwrap_or(serde_json::Value::Null),
        "pinned_at": raw.get("pinned_at").cloned().unwrap_or(serde_json::Value::Null),
        "provider": pinned.get("provider").cloned().unwrap_or(serde_json::Value::Null),
        "release": pinned.get("release").cloned().unwrap_or(serde_json::Value::Null),
        "asset": pinned.get("asset").cloned().unwrap_or(serde_json::Value::Null),
        "archive_sha256": pinned.get("archive_sha256").cloned().unwrap_or(serde_json::Value::Null),
        "library": pinned.get("library").cloned().unwrap_or(serde_json::Value::Null),
        "library_sha256": pinned.get("library_sha256").cloned().unwrap_or(serde_json::Value::Null),
        "licence": pinned.get("licence").cloned().unwrap_or(serde_json::Value::Null),
        "source_offer": pinned.get("source_offer").cloned().unwrap_or(serde_json::Value::Null),
        // What the engine in this process actually reports, which is the only
        // part of this that is observed rather than declared.
        "loaded": ENGINE_VERSION.get().cloned(),
    })
    .to_string()
});

/// The version string of the library that was actually loaded, set once at
/// startup. A pin that names a build which failed to load would be worse than no
/// pin at all.
static ENGINE_VERSION: std::sync::OnceLock<Option<String>> = std::sync::OnceLock::new();

/// The manifest's key for this machine.
///
/// Rust says `x86_64` where a release asset says `amd64`, and `macos` where it
/// says `darwin`; the manifest is keyed the way assets are named because it
/// describes an asset. Left unmapped, the lookup found nothing and the
/// diagnostics printed a page of nulls - which is how the first run of this
/// reported itself, for the architecture, and how the first run of the macOS
/// release job reported it again on 22 September 2026, for the operating
/// system: the pin was there, correct, under `darwin/arm64`, and the player
/// asked for `macos/arm64`.
fn manifest_platform() -> String {
    manifest_key(std::env::consts::OS, std::env::consts::ARCH)
}

/// The rule itself, with the platform passed in, because the bug above is a
/// mapping and a mapping is testable without the machine that has the problem.
fn manifest_key(os: &str, arch: &str) -> String {
    let os = match os {
        "macos" => "darwin",
        other => other,
    };
    let arch = match arch {
        "x86_64" => "amd64",
        "aarch64" => "arm64",
        other => other,
    };
    format!("{os}/{arch}")
}

#[tauri::command]
fn player_toggle_pause() -> Result<(), String> {
    let guard = SESSION.lock().unwrap();
    let session = guard.as_ref().ok_or("the engine is not running")?;
    session.engine.command(&["cycle", "pause"])
}

/// Mutes or unmutes. A player that always makes a noise is hostile in a shared
/// room, and this is what the control bar's volume button drives.
#[tauri::command]
fn player_set_muted(muted: bool) -> Result<(), String> {
    let guard = SESSION.lock().unwrap();
    let session = guard.as_ref().ok_or("the engine is not running")?;
    session
        .engine
        .set_property("mute", if muted { "yes" } else { "no" })
}

/// Sets the volume, as a fraction of full scale.
///
/// The engine keeps volume and mute as two properties; the rule that joins them
/// belongs here rather than in whichever interface is asking. A slider dragged
/// to the end is how somebody silences a film, and moving it off the end is how
/// they bring it back - the same rule the web player applies to its own slider,
/// so a thumb at either end of the track means the same thing in both players.
/// It is also why the two properties are written together: two commands from
/// the interface would leave a window where the sound is back but the icon
/// still says muted.
///
/// mpv counts in percent, so the fraction is scaled at this boundary and
/// nowhere else - the status frame answers in the unit this accepts.
#[tauri::command]
fn player_set_volume(volume: f64) -> Result<(), String> {
    if !volume.is_finite() {
        return Err(format!("volume {volume} is not a number"));
    }
    let fraction = volume.clamp(0.0, 1.0);
    let guard = SESSION.lock().unwrap();
    let session = guard.as_ref().ok_or("the engine is not running")?;
    session
        .engine
        .set_property("volume", &format!("{:.1}", fraction * 100.0))?;
    session
        .engine
        .set_property("mute", if fraction > 0.0 { "no" } else { "yes" })
}

/// Seeks, relative or absolute, in seconds. The mode is passed through rather
/// than guessed: a scrub bar asks for an absolute position and a skip button
/// asks for a relative one, and mixing them up lands in the wrong place in the
/// film.
#[tauri::command]
fn player_seek(seconds: f64, mode: String) -> Result<(), String> {
    let absolute = match mode.as_str() {
        "absolute" => true,
        "relative" => false,
        other => return Err(format!("unknown seek mode {other:?}")),
    };
    {
        let guard = SESSION.lock().unwrap();
        let session = guard.as_ref().ok_or("the engine is not running")?;
        // A converted stream cannot be seeked: it is a pipe with no length and
        // no ranges, so the only way to another position is to ask the server
        // for another one. A viewer scrubbing at 720p waits for a reload, which
        // is exactly what the browser player does at the same moment.
        if session.quality.is_some() {
            let current = session.position().unwrap_or(session.start_at);
            let target = if absolute { seconds } else { current + seconds }.max(0.0);
            let quality = session.quality;
            drop(guard);
            return reload_stream(quality, target);
        }
    }
    let guard = SESSION.lock().unwrap();
    let session = guard.as_ref().ok_or("the engine is not running")?;
    session.engine.command(&[
        "seek",
        &format!("{seconds:.3}"),
        if absolute { "absolute" } else { "relative" },
    ])
}

/// Chooses a subtitle file that was added from beside the film.
///
/// mpv selects a sidecar it finds by itself, because it knows where the file
/// is. Handed one over HTTP it does not: the track arrives after the load and
/// nothing chooses it, so a film whose only French subtitles are a `.srt` beside
/// it played with none. Measured over HTTP: four tracks and no selection, where
/// the same film opened from disk gave five and chose the sidecar.
///
/// Decided from what is observed rather than guessed at add time, because at
/// add time the film is not open yet and "nothing is selected" is not yet an
/// answer. Three things must all be true: the film is loaded, the sidecar has
/// arrived, and nothing at all is selected - a film carrying its own default
/// subtitle keeps it.
fn supervise_subtitles() {
    let mut guard = match SESSION.lock() {
        Ok(g) => g,
        Err(_) => return,
    };
    let Some(session) = guard.as_mut() else { return };
    if session.sidecars.is_empty() {
        return;
    }
    // `duration` is the load test the audio watchdog already uses: it exists
    // once the container has been read, and before that `sid` is absent, which
    // would otherwise be read as "nothing is selected". A file that never
    // reports a duration still gets its subtitles, via `path`.
    let open = session.engine.property("duration").is_some() || session.engine.property("path").is_some();
    if !open {
        return;
    }
    // The tracks themselves first, and the decision on a later tick: adding a
    // file is a command mpv has to fetch, so the track list a moment later is
    // the first one that can be read.
    if !session.sidecars_pending.is_empty() {
        // Six ticks, about three seconds. A sidecar the server cannot serve is
        // not worth asking about for the length of a film.
        const TRIES: u32 = 6;
        if session.sidecar_tries >= TRIES {
            session.sidecars_pending.clear();
            session.sidecar_decided = true;
            eprintln!("theia-player: gave up on the subtitle files beside this film");
            return;
        }
        session.add_sidecars();
        session.sidecar_tries += 1;
        return;
    }
    if session.sidecar_decided {
        return;
    }
    // A viewer who asked for no subtitles gets none. The load opens the film
    // with `sid=no` (`Session::load_options`), and without this the automatic
    // choice below would put a sidecar back a second later - it reads "nothing
    // is selected" as "choose something", and "no subtitles" is exactly that.
    // Read every tick rather than recorded, so changing the preference brings the
    // automatic choice back without reloading the film; the sidecar is still
    // listed in the menu either way, because it was added above.
    if session.sid.is_none() && session.playback.subtitle_language == SubtitleLanguage::Off {
        return;
    }
    match session.engine.property("sid").as_deref() {
        // Something is chosen, so there is nothing to decide. Recorded rather
        // than checked again on every tick.
        Some("no") | None => {}
        Some(_) => {
            session.sidecar_decided = true;
            return;
        }
    }
    let Some(list) = session.engine.property("track-list") else { return };
    let Ok(tracks) = serde_json::from_str::<Vec<serde_json::Value>>(&list) else { return };
    let wanted = tracks
        .iter()
        .find(|track| {
            track.get("type").and_then(|v| v.as_str()) == Some("sub")
                && track.get("external").and_then(|v| v.as_bool()) == Some(true)
        })
        .and_then(|track| track.get("id").and_then(|v| v.as_i64()));
    // The sidecar has not been added yet. Next tick.
    let Some(id) = wanted else { return };

    session.sidecar_decided = true;
    // Not remembered in `sid`, deliberately: an id means something only within
    // one load, and after the audio fallback's reload this decision is made
    // again from the same evidence. The viewer's own choices are the ones that
    // travel, because those are the ones a reload must not undo.
    if let Err(e) = session.engine.set_property("sid", &id.to_string()) {
        eprintln!("theia-player: choosing a subtitle file failed: {e}");
    } else {
        println!("theia-player: reading the subtitle file beside the film (track {id})");
    }
}

/// The tracks mpv sees: video, audio and subtitles, embedded or added later,
/// each with its language, title, codec and which one is playing.
///
/// mpv already answers this as JSON, so it is passed through rather than
/// rebuilt here: a struct on this side would be a second copy of somebody
/// else's schema, and it would be wrong the first time mpv added a field worth
/// showing.
#[tauri::command]
fn player_tracks() -> Result<String, String> {
    let guard = SESSION.lock().unwrap();
    let session = guard.as_ref().ok_or("the engine is not running")?;
    session
        .engine
        .property("track-list")
        .ok_or_else(|| "the engine has no track list yet".to_string())
}

/// Which file of which record mpv is holding.
///
/// Both ids are needed to build a stream address, and both come from one
/// request: the server decides which file is primary, so asking it again is what
/// keeps a rung change on the file that is actually playing rather than on
/// whichever one a list happened to put first.
struct Playable {
    movie: Option<i64>,
    episode: Option<i64>,
    file: i64,
}

fn current_playable() -> Result<Playable, String> {
    let media = *CURRENT_MEDIA.lock().unwrap();
    let guard = CLIENT.lock().unwrap();
    let client = guard.as_ref().ok_or("no server is connected")?;
    match media {
        Some(Playing::Movie(id)) => {
            let movie = client.movie(id)?;
            let file = movie
                .files
                .iter()
                .find(|f| f.is_primary)
                .or_else(|| movie.files.first())
                .ok_or("this film has no playable file")?;
            Ok(Playable {
                movie: Some(id),
                episode: None,
                file: file.id,
            })
        }
        Some(Playing::Episode { id, .. }) => {
            let episode = client.episode(id)?;
            let file = episode
                .files
                .iter()
                .find(|f| f.is_primary)
                .or_else(|| episode.files.first())
                .ok_or("this episode has no playable file")?;
            Ok(Playable {
                movie: None,
                episode: Some(id),
                file: file.id,
            })
        }
        None => Err("nothing is playing".into()),
    }
}

impl Playable {
    fn stream_url(&self, client: &server::Client, height: Option<i64>, start: f64) -> String {
        match (self.movie, self.episode) {
            (_, Some(episode)) => client.episode_stream_url(episode, self.file, height, start),
            (Some(movie), _) => client.stream_url(movie, self.file, height, start),
            _ => String::new(),
        }
    }
}

/// The rungs this machine can produce for what is playing, and what they cost.
///
/// Asked for when the menu opens rather than pushed with every status frame:
/// the ladder belongs to the file and not to the moment, and the answer is the
/// server's - including whether an encoder is free, which changes while
/// somebody else's film is being converted.
#[tauri::command]
fn player_qualities() -> Result<String, String> {
    let playable = current_playable()?;
    let (qualities, transcode) = {
        let guard = CLIENT.lock().unwrap();
        let client = guard.as_ref().ok_or("no server is connected")?;
        let info = match (playable.movie, playable.episode) {
            (_, Some(episode)) => client.episode_stream_info(episode, playable.file)?,
            (Some(movie), _) => client.stream_info(movie, playable.file)?,
            _ => return Err("nothing is playing".into()),
        };
        (info.qualities, info.transcode)
    };
    let current = SESSION.lock().unwrap().as_ref().and_then(|s| s.quality);
    Ok(serde_json::json!({
        "current": current,
        "qualities": qualities,
        "transcode": transcode,
    })
    .to_string())
}

/// Opens what is playing again, at another rung or at another second of it.
///
/// One path for both, because they are the same operation: a converted stream
/// carries its start position in its address, so "seek to 40 minutes" and
/// "change to 720p" are both a new address and a reload. Everything the film
/// owns - its title, its sidecar subtitles, the chosen tracks - is kept by
/// `Session::reload_at`.
fn reload_stream(height: Option<i64>, at: f64) -> Result<(), String> {
    let playable = current_playable()?;
    let (url, duration) = {
        let guard = CLIENT.lock().unwrap();
        let client = guard.as_ref().ok_or("no server is connected")?;
        let url = playable.stream_url(client, height, at);
        // A pipe carries no length of its own, so the film's length comes from
        // the server - the same number the browser player's clock uses. Asked
        // for only when there is a rung: a file knows its own duration.
        let duration = if height.filter(|height| *height > 0).is_some() {
            let info = match (playable.movie, playable.episode) {
                (_, Some(episode)) => client.episode_stream_info(episode, playable.file),
                (Some(movie), _) => client.stream_info(movie, playable.file),
                _ => Err("nothing is playing".to_string()),
            }?;
            Some(info.duration_seconds).filter(|seconds| *seconds > 0.0)
        } else {
            None
        };
        (url, duration)
    };
    let mut guard = SESSION.lock().unwrap();
    let session = guard.as_mut().ok_or("the engine is not running")?;
    session.reload_at(url, at, height, duration)
}

/// Plays the same film at another rung of the ladder.
///
/// A rung is a different stream, not a property, so this is a reload: the film
/// comes back at the second it left, with the tracks the viewer chose still
/// applied - the discipline the audio fallback's reload already follows. `None`
/// asks for the file as it is, which is the ladder's own first rung and the only
/// one that reaches an amplifier untouched.
#[tauri::command]
fn player_set_quality(height: Option<i64>) -> Result<(), String> {
    let at = {
        let guard = SESSION.lock().unwrap();
        let session = guard.as_ref().ok_or("the engine is not running")?;
        // Where the film is *now*. A reload that used the original resume point
        // would send somebody who changed quality an hour in back to the
        // beginning.
        session.position().unwrap_or(session.start_at)
    };
    reload_stream(height, at)
}

/// Chooses a track, or turns one off.
///
/// `kind` is a word rather than mpv's own property name, because the OSD has no
/// business knowing that audio is `aid`; the mapping lives here, once. A
/// non-positive id means "none", which is mpv's own convention for subtitles,
/// passed through rather than translated into something clever.
#[tauri::command]
fn player_set_track(kind: String, id: i64) -> Result<(), String> {
    let property = match kind.as_str() {
        "audio" => "aid",
        "subtitle" => "sid",
        "video" => "vid",
        other => return Err(format!("unknown track kind {other:?}")),
    };
    let mut guard = SESSION.lock().unwrap();
    let session = guard.as_mut().ok_or("the engine is not running")?;
    let value = if id <= 0 { "no".to_string() } else { id.to_string() };
    session.engine.set_property(property, &value)?;
    // Remembered so the audio fallback's reload opens the same tracks. A
    // non-positive id is a deliberate "none" (subtitles off) and is kept as
    // such, because forgetting the choice and forgetting to have no subtitles
    // are not the same thing.
    let chosen = Some(id);
    match property {
        "aid" => session.aid = chosen,
        "sid" => {
            session.sid = chosen;
            // A choice the viewer made is the end of the automatic one, even if
            // it is "no subtitles": the player brought a sidecar to the menu,
            // and it does not get to overrule the answer.
            session.sidecar_decided = true;
        }
        _ => {}
    }
    Ok(())
}

#[tauri::command]
fn player_load(path: String) -> Result<(), String> {
    let title = std::path::Path::new(&path)
        .file_name()
        .map(|n| n.to_string_lossy().into_owned());
    let mut guard = SESSION.lock().unwrap();
    let session = guard.as_mut().ok_or("the engine is not running")?;
    session.engine.command(&["loadfile", &path])?;
    // No sidecars to hand over: a local path is one mpv can look beside for
    // itself, which is exactly what it cannot do with an HTTP URL.
    session.mark_loaded(path, title, 0.0, Vec::new());
    Ok(())
}

// ---------------------------------------------------------------------------
// The server
//
// Locks are never held two at a time here. The player has three pieces of
// shared state and several threads, and the cheapest way to keep that from
// becoming a deadlock nobody can reproduce is to read one, drop it, then read
// the next.
// ---------------------------------------------------------------------------

/// Browses the local network. An empty list is a normal answer rather than a
/// failure: plenty of networks carry no multicast at all, and the OSD then
/// offers the address field instead of pretending nothing exists.
#[tauri::command]
fn player_discover() -> Result<String, String> {
    let found = server::discover(Duration::from_secs(3))?;
    serde_json::to_string(&found).map_err(|e| e.to_string())
}

/// The installer's minimal machine record. Extra fields are deliberately
/// ignored: the player only needs to know whether this machine owns a server.
#[derive(serde::Deserialize)]
struct MachineRecord {
    role: String,
}

/// The only server setting needed to reach the local all-in-one instance.
#[derive(serde::Deserialize)]
struct LocalServerConfig {
    #[serde(default = "default_server_port")]
    port: u16,
}

fn default_server_port() -> u16 {
    8383
}

/// The same data directory convention as the Go server and installer.
fn theia_data_dir() -> Option<PathBuf> {
    if let Some(path) = std::env::var_os("THEIA_DATA_DIR") {
        return Some(PathBuf::from(path));
    }

    #[cfg(windows)]
    {
        return std::env::var_os("APPDATA")
            .map(PathBuf::from)
            .map(|base| base.join("Theia"));
    }
    #[cfg(target_os = "macos")]
    {
        return std::env::var_os("HOME")
            .map(PathBuf::from)
            .map(|home| home.join("Library").join("Application Support").join("Theia"));
    }
    #[cfg(all(unix, not(target_os = "macos")))]
    {
        if let Some(base) = std::env::var_os("XDG_CONFIG_HOME") {
            return Some(PathBuf::from(base).join("theia"));
        }
        return std::env::var_os("HOME")
            .map(PathBuf::from)
            .map(|home| home.join(".config").join("theia"));
    }
    #[allow(unreachable_code)]
    None
}

/// Returns the local server this installation owns, if it is an all-in-one.
fn local_server_install() -> Result<Option<(PathBuf, String)>, String> {
    let Some(data_dir) = theia_data_dir() else {
        return Ok(None);
    };
    let record_path = data_dir.join("setup.json");
    let record = match std::fs::read(&record_path) {
        Ok(data) => serde_json::from_slice::<MachineRecord>(&data)
            .map_err(|e| format!("reading {}: {e}", record_path.display()))?,
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => return Ok(None),
        Err(e) => return Err(format!("reading {}: {e}", record_path.display())),
    };
    if record.role != "all-in-one" {
        return Ok(None);
    }

    let config_path = data_dir.join("config.json");
    let port = match std::fs::read(&config_path) {
        Ok(data) => serde_json::from_slice::<LocalServerConfig>(&data)
            .map_err(|e| format!("reading {}: {e}", config_path.display()))?
            .port,
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => default_server_port(),
        Err(e) => return Err(format!("reading {}: {e}", config_path.display())),
    };
    Ok(Some((data_dir, format!("http://127.0.0.1:{port}"))))
}

fn server_executable_beside_player() -> Result<PathBuf, String> {
    let player = std::env::current_exe().map_err(|e| format!("locating the player: {e}"))?;
    let Some(dir) = player.parent() else {
        return Err("the player has no installation directory".into());
    };
    let name = if cfg!(windows) {
        "theia-server.exe"
    } else {
        "theia-server"
    };
    let server = dir.join(name);
    if !server.is_file() {
        return Err(format!("{} is missing", server.display()));
    }
    Ok(server)
}

fn start_local_server(server: &Path, data_dir: &Path) -> Result<(), String> {
    let mut command = Command::new(server);
    command
        .arg("--data-dir")
        .arg(data_dir)
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null());
    #[cfg(windows)]
    {
        use std::os::windows::process::CommandExt;
        // CREATE_NO_WINDOW: an all-in-one player starts its server as product
        // infrastructure, not as a console the viewer has to dismiss. macOS has
        // no equivalent and needs none: a process started from the app bundle
        // has no console window to suppress (its output goes wherever the
        // bundle's own was sent).
        command.creation_flags(0x0800_0000);
    }
    command
        .spawn()
        .map(|_| ())
        .map_err(|e| format!("starting {}: {e}", server.display()))
}

/// Makes the all-in-one server reachable and returns its deterministic URL.
fn prepare_local_server() -> Result<Option<String>, String> {
    let Some((data_dir, url)) = local_server_install()? else {
        return Ok(None);
    };
    if server::reachable(&url, Duration::from_millis(750)) {
        return Ok(Some(url));
    }

    let server = server_executable_beside_player()?;
    start_local_server(&server, &data_dir)?;
    for _ in 0..40 {
        std::thread::sleep(Duration::from_millis(250));
        if server::reachable(&url, Duration::from_millis(750)) {
            return Ok(Some(url));
        }
    }
    Err(format!("the local server did not answer at {url}"))
}

/// Startup half of the all-in-one contract. Kept asynchronous because starting
/// and waiting for a server must never freeze the WebView's loading screen.
#[tauri::command]
async fn player_local_server() -> Result<Option<String>, String> {
    tauri::async_runtime::spawn_blocking(prepare_local_server)
        .await
        .map_err(|e| format!("preparing the local server: {e}"))?
}

/// Connects to a server at an address. This is the path that always works,
/// which is why the OSD offers it beside whatever discovery found.
fn connect_to(url: &str) -> Result<String, String> {
    let mut client = server::Client::new(url);
    let health = client.health()?;
    let profiles = client.profiles()?;
    // One profile is not a question (decision 50): adopt it rather than asking,
    // exactly as the web interface does.
    if profiles.len() == 1 {
        client.set_profile(Some(profiles[0].id));
    } else if let Some(profile) = profiles.iter().find(|profile| profile.is_default) {
        // Several profiles still have one household default. Zero friction does
        // not mean throwing progress into an unscoped bucket.
        client.set_profile(Some(profile.id));
    }
    let json = connection_payload(&client, &health, &profiles)?;
    *CLIENT.lock().unwrap() = Some(client);
    Ok(json)
}

/// The JSON a connected session is described by, in one place because two
/// callers need it: the connect command, and the question the interface asks at
/// startup about a connection it may already have.
fn connection_payload(
    client: &server::Client,
    health: &server::Health,
    profiles: &[server::Profile],
) -> Result<String, String> {
    let payload = serde_json::json!({
        "url": client.base(),
        "health": health,
        "profile": client.profile(),
        "profiles": profiles,
    });
    serde_json::to_string(&payload).map_err(|e| e.to_string())
}

/// The server this player is already talking to, if any.
///
/// The interface asks this before it adopts anything. A machine installed
/// all-in-one is normally served by its own server and the shell should find it
/// without being told - but not when the caller named a server on the command
/// line, which is what `--server` is for and what the shell used to override
/// silently before it drew its first frame.
#[tauri::command]
fn player_current_server() -> Result<Option<String>, String> {
    let guard = CLIENT.lock().unwrap();
    let Some(client) = guard.as_ref() else {
        return Ok(None);
    };
    let health = client.health()?;
    let profiles = client.profiles()?;
    Ok(Some(connection_payload(client, &health, &profiles)?))
}

/// Starts a film. mpv is handed the stream URL and fetches it itself: it does
/// range requests, buffering and seeking better than anything written here.
fn play_movie(id: i64) -> Result<String, String> {
    let (url, movie_id, title, resume_at, sidecars, original_language) = {
        let guard = CLIENT.lock().unwrap();
        let client = guard.as_ref().ok_or("no server is connected")?;
        // The list does not carry files; the detail does. One extra request is
        // the price of not shipping every file of every film to draw a row.
        let movie = client.movie(id)?;
        let file = movie
            .files
            .iter()
            .find(|f| f.is_primary)
            .or_else(|| movie.files.first())
            .ok_or("this film has no playable file")?;
        // The same rule the web player applies, so a film started in one is
        // resumed by the other: anything part-watched and not finished.
        let resume_at = if movie.progress.finished {
            0.0
        } else {
            movie.progress.position_seconds
        };
        // Subtitle files beside the film, which mpv cannot find by itself: it is
        // given an HTTP URL, and a sidecar is a file on the server's disk. A
        // failure here costs the external tracks and nothing else - the film
        // still plays, and anything embedded in the container is still there -
        // so it is not allowed to stop the load.
        let sidecars = match client.stream_info(movie.id, file.id) {
            Ok(info) => info
                .subtitle_tracks
                .iter()
                .filter(|track| track.fetchable())
                .map(|track| Sidecar {
                    url: client.subtitle_url(movie.id, file.id, track.id),
                    title: track.title.clone(),
                    language: track.language.clone(),
                })
                .collect(),
            Err(e) => {
                eprintln!("theia-player: no external subtitles for this film: {e}");
                Vec::new()
            }
        };
        (
            client.stream_url(movie.id, file.id, None, resume_at),
            movie.id,
            if movie.metadata.title.is_empty() {
                movie.title.clone()
            } else {
                movie.metadata.title.clone()
            },
            resume_at,
            sidecars,
            // The language this film was made in, which is what a `vo` audio
            // preference asks the engine for. Empty when TMDB was never matched
            // to the file, and empty is not a language: it becomes no request at
            // all, so the engine answers with the file's own default.
            Some(movie.metadata.original_language.clone()).filter(|language| !language.is_empty()),
        )
    };

    {
        let mut session_guard = SESSION.lock().unwrap();
        let session = session_guard.as_mut().ok_or("the engine is not running")?;
        // The resume point and the track choices are passed *with the load*,
        // not as properties set beforehand: `file-local-options/start` is
        // refused by mpv with "error accessing property", which the first run
        // of this found the honest way. A file-local option belongs to the
        // loadfile command's own argument list, and that is where it goes.
        session.mark_loaded(url.clone(), Some(title), resume_at, sidecars);
        session.original_language = original_language;
        // Applied before the load, because the language lists are read when the
        // file's own tracks are chosen and that happens in the load below: a
        // `vo` list left over from the previous film would pick this one's audio
        // by the wrong tongue. A refusal is logged and the film plays: a
        // subtitle colour the engine would not take is not a reason to refuse
        // somebody their film.
        if let Err(e) = apply_playback_to(
            &session.engine,
            &session.playback,
            session.original_language.as_deref(),
        ) {
            eprintln!("theia-player: playback preferences not applied to this film: {e}");
        }
        // A film started from the library plays. Without this it inherits the
        // pause state of whatever was on screen before, which is how a viewer
        // who paused one film and opened another got a frozen picture.
        if let Err(e) = session.load(&url, Some(false)) {
            // Nothing was loaded, so the watchdog must not act on it.
            session.loaded_at = None;
            return Err(e);
        }
    }
    *CURRENT_MEDIA.lock().unwrap() = Some(Playing::Movie(movie_id));
    Ok(url)
}

/// Starts one episode through the same engine and track path as a film.
fn play_episode(id: i64) -> Result<String, String> {
    let (url, episode_id, next_episode_id, title, resume_at, sidecars) = {
        let guard = CLIENT.lock().unwrap();
        let client = guard.as_ref().ok_or("no server is connected")?;
        let episode = client.episode(id)?;
        let file = episode
            .files
            .iter()
            .find(|file| file.is_primary)
            .or_else(|| episode.files.first())
            .ok_or("this episode has no playable file")?;
        let resume_at = if episode.progress.finished {
            0.0
        } else {
            episode.progress.position_seconds
        };
        let sidecars = match client.episode_stream_info(episode.id, file.id) {
            Ok(info) => info
                .subtitle_tracks
                .iter()
                .filter(|track| track.fetchable())
                .map(|track| Sidecar {
                    url: client.episode_subtitle_url(episode.id, file.id, track.id),
                    title: track.title.clone(),
                    language: track.language.clone(),
                })
                .collect(),
            Err(e) => {
                eprintln!("theia-player: no external subtitles for this episode: {e}");
                Vec::new()
            }
        };
        let number = episode
            .episode_numbers
            .iter()
            .map(|number| format!("E{number:02}"))
            .collect::<Vec<_>>()
            .join("-");
        let episode_title = episode.title();
        let title = if episode_title.is_empty() {
            format!("{} · S{:02}{number}", episode.series_title, episode.season_number)
        } else {
            format!(
                "{} · S{:02}{number} · {episode_title}",
                episode.series_title, episode.season_number
            )
        };
        (
            client.episode_stream_url(episode.id, file.id, None, resume_at),
            episode.id,
            // Read here, once, while the episode is being fetched anyway: this is
            // the answer to "what plays next", and it belongs to what is playing
            // rather than to a second request made when the file ends.
            episode.next_episode_id,
            title,
            resume_at,
            sidecars,
        )
    };

    {
        let mut session_guard = SESSION.lock().unwrap();
        let session = session_guard.as_mut().ok_or("the engine is not running")?;
        session.mark_loaded(url.clone(), Some(title), resume_at, sidecars);
        // No original language, deliberately, and this is where the limit bites:
        // `SeriesMetadata` in `internal/library/series.go` carries no
        // `original_language`, so a series cannot say what tongue it was made in
        // and there is nothing here to guess from. A `vo` preference therefore
        // degrades to the engine's own default on an episode - which is the
        // honest answer, since inventing one would pick a track nobody asked for.
        // `mark_loaded` has already left this `None`.
        // Applied before the load for the same reason as in `play_movie`: the
        // language lists are consulted when the file's tracks are chosen.
        if let Err(e) = apply_playback_to(
            &session.engine,
            &session.playback,
            session.original_language.as_deref(),
        ) {
            eprintln!("theia-player: playback preferences not applied to this episode: {e}");
        }
        // A film started from the library plays. Without this it inherits the
        // pause state of whatever was on screen before, which is how a viewer
        // who paused one film and opened another got a frozen picture.
        if let Err(e) = session.load(&url, Some(false)) {
            session.loaded_at = None;
            return Err(e);
        }
    }
    *CURRENT_MEDIA.lock().unwrap() = Some(Playing::Episode {
        id: episode_id,
        next: next_episode_id,
    });
    Ok(url)
}

/// One subtitle file beside a film, waiting to be handed to mpv.
#[derive(Clone)]
struct Sidecar {
    url: String,
    title: String,
    language: String,
}

#[tauri::command]
fn player_connect(url: String) -> Result<String, String> {
    connect_to(&url)
}

/// Chooses which viewing history the player writes to. It travels in the open,
/// as `?profile=`, and is never a credential (decision 49).
#[tauri::command]
fn player_set_profile(id: Option<i64>) -> Result<(), String> {
    let mut guard = CLIENT.lock().unwrap();
    let client = guard.as_mut().ok_or("no server is connected")?;
    client.set_profile(id);
    Ok(())
}

#[tauri::command]
fn player_profile_rename(id: i64, name: String) -> Result<String, String> {
    let profile = {
        let guard = CLIENT.lock().unwrap();
        guard
            .as_ref()
            .ok_or("no server is connected")?
            .rename_profile(id, &name)?
    };
    serde_json::to_string(&profile).map_err(|e| e.to_string())
}

#[tauri::command]
fn player_profile_set_avatar(
    id: i64,
    content_type: String,
    data: Vec<u8>,
) -> Result<String, String> {
    let profile = {
        let guard = CLIENT.lock().unwrap();
        guard
            .as_ref()
            .ok_or("no server is connected")?
            .set_profile_avatar(id, &content_type, &data)?
    };
    serde_json::to_string(&profile).map_err(|e| e.to_string())
}

#[tauri::command]
fn player_profile_clear_avatar(id: i64) -> Result<String, String> {
    let profile = {
        let guard = CLIENT.lock().unwrap();
        guard
            .as_ref()
            .ok_or("no server is connected")?
            .clear_profile_avatar(id)?
    };
    serde_json::to_string(&profile).map_err(|e| e.to_string())
}

#[tauri::command]
fn player_update_status() -> Result<String, String> {
    let status = {
        let guard = CLIENT.lock().unwrap();
        guard
            .as_ref()
            .ok_or("no server is connected")?
            .update_status()?
    };
    serde_json::to_string(&status).map_err(|e| e.to_string())
}

#[tauri::command]
fn player_update_check() -> Result<String, String> {
    let status = {
        let guard = CLIENT.lock().unwrap();
        guard
            .as_ref()
            .ok_or("no server is connected")?
            .check_update()?
    };
    serde_json::to_string(&status).map_err(|e| e.to_string())
}

#[tauri::command]
fn player_update_apply() -> Result<String, String> {
    let status = {
        let guard = CLIENT.lock().unwrap();
        guard
            .as_ref()
            .ok_or("no server is connected")?
            .apply_update()?
    };
    serde_json::to_string(&status).map_err(|e| e.to_string())
}

/// The library, for the OSD's grid.
///
/// Every page, not a page. It used to ask for sixty films, which is a sensible
/// first screen and a broken library: the maintainer's own collection is 274 of
/// them, so two thirds were unreachable from the player with nothing on screen
/// saying so. A personal library is a few hundred rows - the whole thing is
/// about 55 KB of JSON, and the grid scrolls.
///
/// The `limit` argument stays because a caller may want a page, and it is
/// clamped: this is not a way to ask a server for ten thousand films.
#[tauri::command]
fn player_library(limit: Option<u32>) -> Result<String, String> {
    let movies = {
        let guard = CLIENT.lock().unwrap();
        let client = guard.as_ref().ok_or("no server is connected")?;
        match limit {
            Some(wanted) => client.movies(wanted.min(server::LIBRARY_CEILING), 0)?,
            None => client.all_movies()?,
        }
    };
    serde_json::to_string(&movies).map_err(|e| e.to_string())
}

#[tauri::command]
fn player_series() -> Result<String, String> {
    let series = {
        let guard = CLIENT.lock().unwrap();
        guard
            .as_ref()
            .ok_or("no server is connected")?
            .series()?
    };
    serde_json::to_string(&series).map_err(|e| e.to_string())
}

#[tauri::command]
fn player_home() -> Result<String, String> {
    let home = {
        let guard = CLIENT.lock().unwrap();
        guard
            .as_ref()
            .ok_or("no server is connected")?
            .home()?
    };
    serde_json::to_string(&home).map_err(|e| e.to_string())
}

/// What was watched, for the settings sheet's viewing section.
///
/// The numbers are the server's, computed from the positions this player
/// reports: a player that counted for itself would be a second definition of
/// "watched" to keep in step with the first.
#[tauri::command]
fn player_watch_stats() -> Result<String, String> {
    let stats = {
        let guard = CLIENT.lock().unwrap();
        guard
            .as_ref()
            .ok_or("no server is connected")?
            .watch_stats()?
    };
    serde_json::to_string(&stats).map_err(|e| e.to_string())
}

#[tauri::command]
fn player_series_home() -> Result<String, String> {
    let home = {
        let guard = CLIENT.lock().unwrap();
        guard
            .as_ref()
            .ok_or("no server is connected")?
            .series_home()?
    };
    serde_json::to_string(&home).map_err(|e| e.to_string())
}

#[tauri::command]
fn player_series_detail(id: i64) -> Result<String, String> {
    let series = {
        let guard = CLIENT.lock().unwrap();
        guard
            .as_ref()
            .ok_or("no server is connected")?
            .series_detail(id)?
    };
    serde_json::to_string(&series).map_err(|e| e.to_string())
}

#[tauri::command]
fn player_season(series_id: i64, season_number: i32) -> Result<String, String> {
    let season = {
        let guard = CLIENT.lock().unwrap();
        guard
            .as_ref()
            .ok_or("no server is connected")?
            .season(series_id, season_number)?
    };
    serde_json::to_string(&season).map_err(|e| e.to_string())
}

#[tauri::command]
fn player_play(id: i64) -> Result<String, String> {
    play_movie(id)
}

#[tauri::command]
fn player_play_episode(id: i64) -> Result<String, String> {
    play_episode(id)
}

/// One line from the interface into the player's own output.
///
/// The OSD has no console anybody can read: a `console.error` in a WebView2 page
/// goes to a devtools window a release build does not open. That turned "the card
/// preview does nothing" into a report with no evidence in either direction - the
/// card swallows a failed preview by design, because the still is the fallback.
/// This is the missing wire, and it prints where the engine's own lines print.
#[tauri::command]
fn player_log(message: String) {
    println!("theia-player: osd: {message}");
}

/// The card preview: six seconds of the film, built by the server on demand.
///
/// Three states and no fourth: `ready` with a URL, `building` while the server
/// makes one, and an error the interface answers by showing the still it
/// already has. The URL comes back absolute for the same reason artwork does -
/// the interface never learns the server's address.
///
/// A series has no clip and cannot have one: a series is not a file. Its cards
/// keep their still, which is a limitation of the data rather than of this
/// command.
#[tauri::command]
fn player_preview(kind: String, id: i64) -> Result<String, String> {
    let guard = CLIENT.lock().unwrap();
    let client = guard.as_ref().ok_or("no server is connected")?;
    let payload = client.preview_clip(&kind, id)?;
    serde_json::to_string(&payload).map_err(|e| e.to_string())
}

/// Stops the current film and returns to the library without closing the app.
#[tauri::command]
fn player_stop() -> Result<(), String> {
    save_progress();
    {
        let mut guard = SESSION.lock().unwrap();
        let session = guard.as_mut().ok_or("the engine is not running")?;
        session.engine.command(&["stop"])?;
        session.clear_media();
    }
    *CURRENT_MEDIA.lock().unwrap() = None;
    Ok(())
}

/// The window's corner radius, in logical pixels. Section 6b of the design
/// system carries the reasoning: the reference is a Windows 11 caption bar, and
/// Windows 11 rounds its own windows at 8.
const WINDOW_CORNER_RADIUS: f64 = 8.0;

/// Rounds the operating system window itself, not the page.
///
/// The page has carried `border-radius` since the desktop rewrite and it does
/// round what the page paints - measured on 20 September 2026 from a capture of
/// the real screen: at the window's top row the painted content starts six
/// device-independent pixels in, exactly the declared radius. A viewer still
/// sees a square window, because the page is transparent outside that curve and
/// mpv's own surface sits behind it filling the whole rectangle. The corner
/// pixels read (0,0,0) - pure black, where the page's ink is (11,10,9) and the
/// desktop behind is (248,250,253) - and a CSS radius cannot clip a child window
/// that belongs to another process.
///
/// So the region does it. `SetWindowRgn` clips the window and everything inside
/// it, mpv included, and the region is cleared when the window is maximized or
/// fullscreen because there the curve is not wanted. The radius is scaled by the
/// monitor's factor so the curve is the same size to the eye at 100% and at 200%.
#[cfg(windows)]
fn apply_round_region(hwnd: isize, width: u32, height: u32, scale: f64, rounded: bool) {
    use windows_sys::Win32::Foundation::HWND;
    use windows_sys::Win32::Graphics::Gdi::{CreateRoundRectRgn, SetWindowRgn};

    if width == 0 || height == 0 {
        return;
    }
    let region = if rounded {
        let diameter = (WINDOW_CORNER_RADIUS * scale).round().max(1.0) as i32 * 2;
        // CreateRoundRectRgn takes the size of the ellipse that draws each
        // corner - twice the radius - and its rectangle is inclusive of both
        // edges, hence the +1.
        unsafe { CreateRoundRectRgn(0, 0, width as i32 + 1, height as i32 + 1, diameter, diameter) }
    } else {
        std::ptr::null_mut()
    };
    // SetWindowRgn takes ownership of the region when it succeeds, and a null
    // region is how it is cleared.
    unsafe { SetWindowRgn(hwnd as HWND, region, 1) };
}

/// macOS and the other Unix: no equivalent, and none is invented. There is no
/// window region to set - on macOS the window is a plain `NSWindow`, its corners
/// are the system's business, and the icon and the menu bar come from the app
/// bundle rather than from this code. The page's own `border-radius` still
/// rounds what the page paints; what is missing here is the clipping of the
/// surface below it, which only Windows offers.
#[cfg(not(windows))]
fn apply_round_region(_hwnd: isize, _width: u32, _height: u32, _scale: f64, _rounded: bool) {}

/// Clears the window region: one cheap call, and the corners are square until
/// it is put back.
#[cfg(windows)]
fn clear_round_region(hwnd: isize) {
    use windows_sys::Win32::Foundation::HWND;
    use windows_sys::Win32::Graphics::Gdi::SetWindowRgn;
    unsafe { SetWindowRgn(hwnd as HWND, std::ptr::null_mut(), 1) };
}

#[cfg(not(windows))]
fn clear_round_region(_hwnd: isize) {}

/// What the window's own resize looked like from inside this process: how many
/// events arrived, and the worst gap between two of them.
///
/// A drag is a modal loop in Windows: the events are sent as the pointer moves,
/// and the window paints between them. If this process is busy, it falls behind
/// the pointer and the events arrive late - so **this gap is the lag a person
/// sees**, and it is the number the page's own frame timing cannot see: the page
/// can draw at a hundred frames a second while the window it lives in is a
/// second behind the hand.
/// The third number is the total of the gaps, so the average can be read as
/// well as the worst: the worst is dominated by the first step of a drag, and a
/// change that only helps the steps after it would be invisible in it.
static RESIZE_TIMING: std::sync::Mutex<(Option<std::time::Instant>, u32, f64, f64)> =
    std::sync::Mutex::new((None, 0, 0.0, 0.0));

/// When the last resize arrived, and whether the rounded region is applied.
///
/// `None` for the time means no resize is in flight. The telemetry thread is
/// what puts the region back, so this costs no timer and no thread of its own.
static RESIZE_STATE: std::sync::Mutex<(Option<std::time::Instant>, bool)> =
    std::sync::Mutex::new((None, true));

/// Called from the window event hook, once per resize event.
///
/// The region is a snapshot of a size, so a stale one clips a bigger window -
/// but re-making it per event is what made a drag of the window's edge lag.
/// **Measured on 24 September 2026**, with a scripted drag of forty steps: five
/// frames drawn with the region re-made on each event, eighty-eight with it left
/// alone. So it is dropped once here, which cannot clip anything, and put back
/// once when the size has settled.
#[cfg(windows)]
fn note_resize(hwnd: isize) {
    let now = std::time::Instant::now();
    // Both numbers are totals since the process started, and deliberately so:
    // the first version reset them when a new drag began, and a reader (me) who
    // looked one second too late read the count of a two-pixel step. A number
    // that can be gathered wrong is worse than no number.
    if let Ok(mut timing) = RESIZE_TIMING.lock() {
        if let Some(previous) = timing.0 {
            let gap = now.duration_since(previous).as_secs_f64() * 1000.0;
            timing.2 = timing.2.max(gap);
            timing.3 += gap;
        }
        timing.0 = Some(now);
        timing.1 += 1;
    }
    let Ok(mut state) = RESIZE_STATE.lock() else { return };
    if state.1 {
        clear_round_region(hwnd);
        state.1 = false;
    }
    state.0 = Some(now);
}

#[cfg(not(windows))]
fn note_resize(_hwnd: isize) {}

/// Puts the region back once the size has stopped moving.
#[cfg(windows)]
fn settle_round_region(window: &tauri::WebviewWindow) {
    let Ok(mut state) = RESIZE_STATE.lock() else { return };
    let Some(at) = state.0 else { return };
    if state.1 || at.elapsed() < Duration::from_millis(250) {
        return;
    }
    state.1 = true;
    state.0 = None;
    drop(state);
    round_webview_window(window);
}

#[cfg(not(windows))]
fn settle_round_region(_window: &tauri::WebviewWindow) {}

/// Applies the region from a window's own geometry.
///
/// One wrapper rather than two, since decision 147's resize pass: the event hook
/// now calls `note_resize` and the region is put back by the telemetry thread
/// through `settle_round_region`, so the `tauri::Window` half had no caller left.
#[cfg(windows)]
fn round_webview_window(window: &tauri::WebviewWindow) {
    let Ok(hwnd) = window.hwnd() else { return };
    let size = window.inner_size().unwrap_or_default();
    let scale = window.scale_factor().unwrap_or(1.0);
    let full = window.is_maximized().unwrap_or(false) || window.is_fullscreen().unwrap_or(false);
    apply_round_region(hwnd.0 as isize, size.width, size.height, scale, !full);
}

// The stub below exists so the setup hook reads the same on every platform. On
// macOS and the other Unix it has nothing to do: the rounding is `SetWindowRgn`,
// and the window is a normal `NSWindow`.

#[cfg(not(windows))]
fn round_webview_window(_window: &tauri::WebviewWindow) {}

/// One resize event, handed to the timing and to the region.
///
/// Two functions rather than an `if let Ok(hwnd)` in the event hook, and this is
/// not tidiness: `tauri::Window::hwnd` does not exist on macOS, so asking for a
/// handle there is a build error rather than a no-op. The **dispatch of 24
/// September 2026** found that the hard way - the resize pass had only ever been
/// compiled on Windows, and the macOS player job refused the tree with
/// `no method named hwnd found for reference &tauri::Window`. A release is
/// measured on every platform it publishes, which is what caught it before a tag
/// did.
#[cfg(windows)]
fn note_resize_event(window: &tauri::Window) {
    if let Ok(hwnd) = window.hwnd() {
        note_resize(hwnd.0 as isize);
    }
}

/// Nothing on a Mac or the other Unix: there is no window region to drop, and
/// the timing that goes with it is a Windows measurement.
#[cfg(not(windows))]
fn note_resize_event(_window: &tauri::Window) {}

/// Chooses a 16:9 logical window that fits the current monitor at any DPI.
fn fitted_window_size(monitor_width: f64, monitor_height: f64) -> (f64, f64) {
    let usable_width = (monitor_width - 80.0).max(640.0);
    let usable_height = (monitor_height - 80.0).max(360.0);
    let mut width = usable_width.min(1280.0);
    let mut height = width * 9.0 / 16.0;
    if height > usable_height.min(720.0) {
        height = usable_height.min(720.0);
        width = height * 16.0 / 9.0;
    }
    (width, height)
}

fn fit_initial_window(window: &tauri::WebviewWindow) {
    let Ok(Some(monitor)) = window.current_monitor() else {
        return;
    };
    let scale = monitor.scale_factor();
    let size = monitor.size();
    let logical_width = size.width as f64 / scale;
    let logical_height = size.height as f64 / scale;
    let (width, height) = fitted_window_size(logical_width, logical_height);
    let _ = window.set_size(tauri::LogicalSize::new(width, height));
    let _ = window.center();
}

/// Reads `--window 640x360`: a size in **physical** pixels.
///
/// Physical and not logical on purpose. The question this exists for - what the
/// OSD makes of the window at its declared minimum - is a question about real
/// pixels, because the page counts CSS pixels and the scaling factor is what
/// sits between the two. A logical request would be the number the page already
/// sees, which is the answer, not the question.
///
/// A size that does not parse is treated as no size at all rather than as a
/// failure: the player still has to start, and a window at the designed size
/// with a report saying nothing was asked for is a better answer than a process
/// that refuses to open.
fn parse_window_size(value: &str) -> Option<(u32, u32)> {
    let (width, height) = value.split_once(['x', 'X'])?;
    let width = width.trim().parse().ok()?;
    let height = height.trim().parse().ok()?;
    (width > 0 && height > 0).then_some((width, height))
}

/// The one question this player cannot answer by looking at its own window:
/// what the page inside it believes it is drawing into.
///
/// The window's size is what Tauri was asked for; the document's is what the
/// OSD lays itself out against, and the two differ by exactly the scaling
/// factor. Reading it back needs no command in the interface - the page is
/// asked directly, and `eval_with_callback` hands the answer back as JSON.
///
/// The `try` is not decoration: on Windows a throw reaches the callback as an
/// empty string rather than as an error, and an empty string is not a
/// measurement. The bar is measured as well as the viewport, because "the
/// control bar is not in the picture" has three possible causes - it is not
/// drawn, it is drawn off the edge, or the furniture faded - and a photograph
/// cannot tell them apart on its own.
const VIEWPORT_PROBE: &str = r#"(function () {
  try {
    const bar = document.querySelector('.controls');
    const rect = bar ? bar.getBoundingClientRect() : null;
    const style = bar ? getComputedStyle(bar) : null;
    const controls = bar ? Array.from(bar.querySelectorAll('button.control')) : [];
    return {
      innerWidth: window.innerWidth,
      innerHeight: window.innerHeight,
      devicePixelRatio: window.devicePixelRatio,
      idle: document.querySelector('.osd')?.dataset.idle ?? null,
      bar: bar && rect && style ? {
        hidden: bar.classList.contains('controls--hidden'),
        display: style.display,
        opacity: style.opacity,
        left: rect.left,
        top: rect.top,
        width: rect.width,
        height: rect.height,
        bottom: rect.bottom,
        controls: controls.length,
        controlsVisible: controls.filter((b) => getComputedStyle(b).display !== 'none').length
      } : null
    };
  } catch (e) {
    return { error: String(e) };
  }
})()"#;

/// The size the window itself says it will not go below, in **real** pixels.
///
/// `WM_GETMINMAXINFO` is where Windows asks a window for its tracking limits,
/// and it is the only place the declared minimum actually exists: the config's
/// `minWidth`/`minHeight` reach tao as *logical* units and are multiplied by the
/// monitor's factor right there. So this answers, by measurement rather than by
/// reading somebody else's source, the question the whole check turns on - is
/// "the declared minimum" 640 real pixels, or 640 CSS pixels, which at 200%
/// scaling is 1280 of them?
///
/// It also says what the `--window` request does *not* get clamped by: the
/// constraint lives in this message alone, and Windows sends it while a person
/// drags an edge, not when the program resizes itself.
#[cfg(windows)]
fn minimum_track_size(hwnd: isize) -> Option<(i32, i32)> {
    use windows_sys::Win32::Foundation::HWND;
    use windows_sys::Win32::UI::WindowsAndMessaging::{SendMessageW, MINMAXINFO, WM_GETMINMAXINFO};

    let mut info: MINMAXINFO = unsafe { std::mem::zeroed() };
    // The window is in this process, so the pointer needs no marshalling: the
    // same call from outside would be a different question.
    unsafe {
        SendMessageW(
            hwnd as HWND,
            WM_GETMINMAXINFO,
            0,
            &mut info as *mut MINMAXINFO as isize,
        );
    }
    let (width, height) = (info.ptMinTrackSize.x, info.ptMinTrackSize.y);
    (width > 0 && height > 0).then_some((width, height))
}

/// macOS and the other Unix: `WM_GETMINMAXINFO` is a Win32 message with no
/// counterpart. There is nothing to ask, and nothing is invented: the window is
/// a normal `NSWindow` and the minimum a person feels while dragging an edge is
/// the one tao applies from `minWidth`/`minHeight` in logical points.
#[cfg(not(windows))]
fn minimum_track_size(_wid: isize) -> Option<(i32, i32)> {
    None
}

/// Writes what the window and the page measured, twice a second, to the file
/// `--window-report` named.
///
/// Open risk 6 in docs/v3.3.md is a picture nobody could trust: the shipped
/// player was resized to the minimum from *outside*, which bypasses Tauri's own
/// sizing, and the picture showed no control bar - a product fault or a
/// measurement fault, and the record says so rather than guessing. This is the
/// other half of settling it. The size is applied through Tauri, and the numbers
/// are written down where a probe can read them, because this is a GUI binary
/// with no console anybody can read.
///
/// It keeps writing because the picture is taken seconds after the window opens
/// and after the pointer has been nudged: a report written once at startup would
/// describe the window at load and not the moment the photograph shows.
///
/// `wid` is the platform's window handle from `main`; only the Windows path uses
/// it, for [`minimum_track_size`], and on macOS and the other Unix it is read by
/// nothing.
fn start_window_probe(window: tauri::WebviewWindow, wid: isize, requested: (u32, u32), path: PathBuf) {
    std::thread::spawn(move || {
        // Twenty seconds at two beats a second: long enough for a script to
        // launch the player, wait for the film to draw and photograph it, and
        // frequent enough that the file on disk describes the picture's own
        // moment rather than the moment the window opened.
        for _ in 0..40 {
            let (tx, rx) = std::sync::mpsc::channel();
            let asked = window.eval_with_callback(
                VIEWPORT_PROBE,
                move |answer| {
                    let _ = tx.send(answer);
                },
            );
            // The answer is waited for before the file is written, so the report
            // never says the page did not answer about a page that did.
            let page: serde_json::Value = match asked {
                Ok(()) => match rx.recv_timeout(Duration::from_millis(400)) {
                    Ok(answer) if !answer.is_empty() => {
                        serde_json::from_str(&answer).unwrap_or(serde_json::Value::Null)
                    }
                    Ok(_) => serde_json::Value::String("the page threw".into()),
                    Err(_) => serde_json::Value::Null,
                },
                Err(e) => serde_json::Value::String(format!("the page could not be asked: {e}")),
            };
            let physical = window.inner_size().unwrap_or_default();
            let outer = window.outer_size().unwrap_or_default();
            let scale = window.scale_factor().unwrap_or(1.0);
            let minimum = minimum_track_size(wid);
            let report = serde_json::json!({
                "requested": { "width": requested.0, "height": requested.1 },
                "physical": { "width": physical.width, "height": physical.height },
                "outer": { "width": outer.width, "height": outer.height },
                "scaleFactor": scale,
                "logical": {
                    "width": physical.width as f64 / scale,
                    "height": physical.height as f64 / scale,
                },
                // What the window itself answers to `WM_GETMINMAXINFO`: the
                // floor Windows enforces while an edge is dragged, in real
                // pixels. Null when the window did not answer.
                "minimumTrackSize": match minimum {
                    Some((width, height)) => serde_json::json!({ "width": width, "height": height }),
                    None => serde_json::Value::Null,
                },
                "page": page,
            });
            match serde_json::to_string_pretty(&report) {
                Ok(text) => {
                    if let Err(e) = std::fs::write(&path, text) {
                        eprintln!("theia-player: could not write {}: {e}", path.display());
                    }
                }
                Err(e) => eprintln!("theia-player: could not serialise the window report: {e}"),
            }
            std::thread::sleep(Duration::from_millis(500));
        }
    });
}


/// Writes the playhead back to the server. Deliberately the same call the
/// browser player makes, so a film started in one is resumable in the other.
fn save_progress() {
    let Some(media) = *CURRENT_MEDIA.lock().unwrap() else {
        return;
    };
    let (position, duration) = {
        let Ok(guard) = SESSION.lock() else { return };
        let Some(session) = guard.as_ref() else { return };
        // The film's clock, not mpv's: on a converted stream they differ by the
        // `t=` the pipe was asked for, and a resume point written short by that
        // much is a viewer sent backwards every time they change quality.
        (
            session.position().unwrap_or(0.0),
            session.duration().unwrap_or(0.0),
        )
    };
    if position <= 0.5 {
        return; // Nothing has been watched yet; a zero would erase a position.
    }

    let guard = match CLIENT.lock() {
        Ok(g) => g,
        Err(_) => return,
    };
    let Some(client) = guard.as_ref() else { return };
    let result = match media {
        Playing::Movie(id) => client.save_progress(id, position, duration),
        // The id is what progress belongs to; the next episode travelling beside
        // it is about what happens at the end of the file and has no business in
        // somebody's watch history.
        Playing::Episode { id, .. } => client.save_episode_progress(id, position, duration),
    };
    if let Err(e) = result {
        eprintln!("theia-player: {e}");
    }
}

/// Starts the next episode when a file ends, if the viewer asked for that.
///
/// The engine is what says the file ended: `eof-reached` is a property of the
/// file that is loaded, and it is absent while nothing is loaded - an absent
/// value is not the end of anything, and reading it as one would start an
/// episode from an idle player. `keep-open=yes` keeps the finished file loaded,
/// so the property stays set for as long as nobody touches the player, which is
/// why the session holds a guard: without it the telemetry thread would start
/// the next episode twice a second.
///
/// The guard is re-armed by the engine's own answer - a tick that reports no end
/// - rather than by the load the player itself issues, and that is deliberate. A
/// load that has just been handed to mpv can still be reporting the file before
/// it, and for how long is not something this process can observe from outside;
/// a guard re-armed by the load would act inside that window, so the episode
/// after the one just started would start on top of it. Keying on the property
/// needs no such judgement: the tick that re-arms is the same tick's reading of
/// the same value.
///
/// The film ending and the viewer closing the window are different events, and
/// only the first one autoplays: the window closing ends the process, and a
/// viewer who pressed stop left the media empty (see `player_stop`), which is
/// what the match below reads.
fn autoplay_next_episode() {
    // One lock at a time, each taken and released on its own: the session first,
    // because the answer comes from the engine, then the preferences, then the
    // media. This file never holds two of its locks at once.
    let ended = {
        let mut guard = match SESSION.lock() {
            Ok(guard) => guard,
            Err(_) => return,
        };
        let Some(session) = guard.as_mut() else { return };
        if session.engine.property("eof-reached").as_deref() == Some("yes") {
            if session.end_handled {
                return;
            }
            session.end_handled = true;
            true
        } else {
            // No file has ended, so the next end is a fresh one.
            session.end_handled = false;
            false
        }
    };
    if !ended {
        return;
    }
    // The end has been recorded whether or not anything comes of it: a
    // preference turned on ten minutes after the film finished must not start an
    // episode the viewer has moved on from.
    if !PLAYBACK.lock().map(|guard| guard.auto_play_next).unwrap_or(false) {
        return;
    }
    let next = match *CURRENT_MEDIA.lock().unwrap() {
        Some(Playing::Episode { next: Some(id), .. }) => id,
        // A film, an episode the server says is the last one, and a stopped
        // player all land here, which is the point of keeping the next id with
        // the media rather than in a static of its own.
        _ => return,
    };
    // The finished episode is written before the record it belongs to is
    // replaced: the periodic save runs every five seconds, so its last word on
    // this episode can be five seconds old, and after the switch there is
    // nothing left to write its end from.
    save_progress();
    // The same path the library's own click takes, so the next episode resumes
    // where it was left, brings its sidecars and records its own progress.
    match play_episode(next) {
        Ok(_) => println!("theia-player: autoplaying episode {next}"),
        // Logged and dropped, never fatal: a thread that died here would take the
        // status frames, the audio watchdog and the progress saves with it, and
        // nobody would know why the player went quiet.
        Err(e) => eprintln!("theia-player: could not autoplay episode {next}: {e}"),
    }
}

/// Starts the engine, or reports why it could not start. Called from setup so
/// a missing DLL becomes a message rather than a crash.
///
/// `wid` is the platform's window handle, the value [`base_options`] passes to
/// mpv as `wid`.
fn start_engine(wid: isize, media: Option<&str>, silent: bool) -> Result<(), String> {
    let dll = mpv::engine_path()?;
    let engine = Engine::load(&dll)?;

    for (name, value) in base_options(wid, silent) {
        match engine.set_option(name, &value) {
            Ok(()) => {}
            // One exception, and only on macOS: the window handle. mpv 0.41 does
            // not read `wid` there (see `base_options`), and what is expected is
            // an option accepted and ignored - but if the shipped engine refuses
            // the option outright instead, the player still has to start. That is
            // the Mac's first check, and this arm is what keeps a refusal from
            // being the answer. A film that plays that way plays in mpv's own
            // window, with the OSD in a window of its own rather than over the
            // picture, because the two are no longer the same window.
            Err(e) if cfg!(target_os = "macos") && name == "wid" => {
                eprintln!(
                    "theia-player: mpv would not take {name}={value} ({e}); \
                     a film will play in mpv's own window if it can draw one"
                );
            }
            // Everything else failing is fatal, which is the behaviour this loop
            // has always had: a vo, an ao or a hwdec this engine cannot honour
            // stops the player rather than letting it play something worse in
            // silence. (The comment this replaces said hwdec was allowed to fail;
            // it has never been allowed to.)
            Err(e) => return Err(e),
        }
    }
    // Windows, and every other Unix, start by asking for passthrough and fall
    // back to PCM when the endpoint refuses. macOS has no passthrough to ask for
    // - CoreAudio cannot carry what `SPDIF_CODECS` names - so it starts in PCM,
    // and the audio watchdog that watches for a refused bitstream has nothing to
    // watch there (`supervise_audio` returns unless the mode is Passthrough).
    let audio = if cfg!(target_os = "macos") {
        AudioMode::Pcm
    } else {
        AudioMode::Passthrough
    };
    apply_audio_mode(&engine, audio)?;
    engine.initialize()?;
    // What the loaded library says it is, recorded once: the pin in
    // player/libmpv.json is a claim about a file, and this is the observation
    // that it is the file actually running. Set before the session exists so a
    // diagnostics call can never race the engine's own startup.
    let _ = ENGINE_VERSION.set(engine.version());

    if let Some(path) = media {
        engine.command(&["loadfile", path])?;
    }

    let mut session = Session {
        engine,
        // The mode the engine was just configured for, not a second opinion:
        // `audio` is PCM on macOS and a passthrough request elsewhere.
        audio,
        // Whatever the OSD has already chosen, or the sheet's own defaults: this
        // is the copy the load path reads, and it is written only here and by
        // `apply_playback`.
        playback: stored_playback(),
        media: None,
        title: None,
        original_language: None,
        loaded_at: None,
        start_at: 0.0,
        aid: None,
        sid: None,
        sidecars: Vec::new(),
        sidecars_pending: Vec::new(),
        sidecar_tries: 0,
        sidecar_decided: false,
        end_handled: false,
        audio_reason: None,
        quality: None,
        duration_hint: None,
    };
    if let Some(path) = media {
        let title = std::path::Path::new(path)
            .file_name()
            .map(|n| n.to_string_lossy().into_owned());
        session.mark_loaded(path.to_string(), title, 0.0, Vec::new());
    }
    *SESSION.lock().unwrap() = Some(session);
    // The same application the sheet's save runs, once, here - so the process
    // never renders a default nobody chose. It is deliberately after the load
    // above: the properties are live, and the load is what needs the window, not
    // the settings. A refusal is logged and does not stop the player: an engine
    // that would not take a subtitle colour is still an engine.
    if let Err(e) = apply_playback() {
        eprintln!("theia-player: playback preferences not applied: {e}");
    }
    Ok(())
}

/// Reads `--flag value`. Written by hand rather than pulled from a crate: a
/// handful of flags does not justify a dependency, and the parser is four lines.
fn flag(name: &str) -> Option<String> {
    std::env::args().skip_while(|a| a != name).nth(1)
}

fn main() {
    // The clock the first status frame measures the startup against. Set here
    // rather than at the first report: what is being measured is how long this
    // process took to become useful, and that starts now.
    let _ = STARTED_AT.set(std::time::Instant::now());

    // A build that cannot name itself can never be updated by anything that
    // asks, which is decision 24 applied to the player: the installer next door
    // runs exactly this command to find out what is installed. Answered first,
    // before discovery, before the engine is looked for and before Tauri starts,
    // so it works with no display, no server and no engine present - and so a
    // caller gets a version and not a window.
    if std::env::args().any(|a| a == "--version" || a == "-version") {
        println!("theia-player {}", env!("THEIA_VERSION"));
        return;
    }

    // Discovery answers on stdout and exits: it needs no window, and it is the
    // only way to tell "no server is announced" from "the player cannot see
    // one" on a machine nobody can look at.
    if std::env::args().any(|a| a == "--discover") {
        match server::discover(Duration::from_secs(3)) {
            Ok(found) if found.is_empty() => {
                println!("no server answered for {}", server::SERVICE_TYPE);
            }
            Ok(found) => {
                for entry in found {
                    println!(
                        "{}\t{}\t{}",
                        entry.url,
                        entry.name,
                        entry.version.unwrap_or_else(|| "unknown".into())
                    );
                }
            }
            Err(e) => {
                eprintln!("discovery failed: {e}");
                std::process::exit(1);
            }
        }
        return;
    }

    // Listing a library needs neither the engine nor a window, so it is
    // answered before Tauri starts. It is the same call the OSD's panel makes -
    // no limit unless one is asked for, so it reads the whole library and not
    // just the first page - which means the panel's data path can be checked
    // without a click, and without a window appearing on somebody's screen.
    if let Some(url) = flag("--server") {
        if std::env::args().any(|a| a == "--list") {
            let limit = flag("--limit").and_then(|n| n.parse::<u32>().ok());
            match connect_to(&url).and_then(|_| player_library(limit)) {
                Ok(json) => println!("{json}"),
                Err(e) => {
                    eprintln!("theia-player: {e}");
                    std::process::exit(1);
                }
            }
            return;
        }
    }

    let media = flag("--media").filter(|p| std::path::Path::new(p).is_file());
    // The verification path for the window's declared minimum: a size in real
    // pixels, applied through Tauri's own API, and the measurement written to a
    // file because a GUI binary has no console to print it on. See
    // start_window_probe.
    let window_size = flag("--window").and_then(|value| parse_window_size(&value));
    let window_report = flag("--window-report");
    // Prints the session state once a second. It exists so a smoke test, or a
    // person debugging a machine they cannot see, has something to read instead
    // of a window that may or may not be drawing.
    let diagnostics = std::env::args().any(|a| a == "--diagnostics");
    // Starts muted. Every automated run of this program uses it: a test that
    // plays a tone on somebody's machine while they are working is a test that
    // gets the whole project turned off.
    let silent = std::env::args().any(|a| a == "--mute");

    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![
            player_status,
            player_version,
            player_engine_pin,
            player_toggle_pause,
            player_set_muted,
            player_set_volume,
            player_tracks,
            player_set_track,
            player_qualities,
            player_set_quality,
            player_seek,
            player_load,
            player_stop,
            player_local_server,
            player_discover,
            player_connect,
            player_set_profile,
            player_profile_rename,
            player_profile_set_avatar,
            player_profile_clear_avatar,
            player_update_status,
            player_update_check,
            player_update_apply,
            player_library,
            player_series,
            player_home,
            player_series_home,
            player_watch_stats,
            player_osd_stats,
            player_set_playback,
            player_series_detail,
            player_season,
            player_play,
            player_play_episode,
            player_preview,
            player_log,
            player_current_server,
            player_log
        ])
        .setup(move |app| {
            let window = app.get_webview_window("main").expect("the main window");
            // The handle mpv draws into, and the one place it is chosen. mpv
            // calls it `wid` on every platform; what it *is* does not travel: an
            // `HWND` on Windows, an `NSView*` on macOS (mpv 0.36's manual asked
            // for exactly that, `NSView*` cast to `intptr_t`; 0.41 no longer
            // reads the option at all - see `base_options`), and nothing on the
            // other Unix, where no window embedding has ever been attempted.
            let wid = {
                #[cfg(windows)]
                {
                    window.hwnd().expect("a window handle").0 as isize
                }
                #[cfg(target_os = "macos")]
                {
                    // The window's content view: the NSView the WebView is
                    // drawn in, which is where a film would have to go.
                    window.ns_view().expect("the window's content view") as isize
                }
                #[cfg(not(any(windows, target_os = "macos")))]
                {
                    0isize
                }
            };
            // The configured size is only a safe fallback. On a 200% display a
            // nominal 1280x720 window became 2560x1440 physical pixels and
            // opened mostly off-screen; size against this monitor before the
            // hidden window is ever shown.
            fit_initial_window(&window);

            // `--window` is applied after the fitting and before anything is
            // drawn into the window, so mpv's surface is created at the final
            // size rather than resized into it. The request goes through
            // Tauri's own window API: reaching this size the way the product
            // reaches it is the whole point of the check (open risk 6).
            if let Some((width, height)) = window_size {
                let _ = window.set_size(tauri::PhysicalSize::new(width, height));
            }
            round_webview_window(&window);

            if let (Some(requested), Some(path)) = (window_size, window_report.clone()) {
                start_window_probe(window.clone(), wid, requested, PathBuf::from(path));
            }

            match start_engine(wid, media.as_deref(), silent) {
                Ok(()) => {
                    let version = player_version().unwrap_or_else(|| "unknown".into());
                    println!("theia-player: engine {version}");
                    let _ = window.emit("player-event", "{\"kind\":\"engine\",\"state\":\"ready\"}");
                }
                Err(e) => {
                    // The page is the only thing that can explain this to a
                    // person, so it is shown and told, in that order.
                    eprintln!("theia-player: {e}");
                    let _ = window.emit(
                        "player-event",
                        "{\"kind\":\"engine\",\"state\":\"unavailable\"}",
                    );
                }
            }
            // Command-line connection, so the client can be exercised without a
            // click. The OSD drives the same two functions.
            if let Some(url) = flag("--server") {
                match connect_to(&url) {
                    Ok(info) => println!("theia-player: connected to {url} -> {info}"),
                    Err(e) => eprintln!("theia-player: could not connect to {url}: {e}"),
                }
                // The card preview, without a pointer. The interface asks for one
                // on hover and swallows a failure by design - the still is the
                // fallback - which made "the preview does nothing" a report with
                // no evidence behind it in either direction. This is the same
                // call the card makes, printed.
                if let Some(id) = flag("--preview").and_then(|v| v.parse::<i64>().ok()) {
                    let kind = flag("--preview-kind").unwrap_or_else(|| "movie".to_string());
                    match CLIENT
                        .lock()
                        .unwrap()
                        .as_ref()
                        .ok_or("no client".to_string())
                        .and_then(|client| client.preview_clip(&kind, id))
                    {
                        Ok(state) => println!(
                            "theia-player: preview state={} clip_url={}",
                            state.state, state.clip_url
                        ),
                        Err(e) => eprintln!("theia-player: preview for {kind} {id} failed: {e}"),
                    }
                }
                if let Some(id) = flag("--play").and_then(|v| v.parse::<i64>().ok()) {
                    match play_movie(id) {
                        Ok(stream) => println!("theia-player: playing {stream}"),
                        Err(e) => eprintln!("theia-player: could not start film {id}: {e}"),
                    }
                } else if let Some(id) = flag("--episode").and_then(|v| v.parse::<i64>().ok()) {
                    match play_episode(id) {
                        Ok(stream) => println!("theia-player: playing episode {stream}"),
                        Err(e) => eprintln!("theia-player: could not start episode {id}: {e}"),
                    }
                }
            }
            let _ = window.show();

            // Telemetry, mpv -> Rust -> OSD. The page never polls.
            //
            // A frame is sent when the state a viewer can see has moved, and
            // once every two seconds when it has not. What the interface does
            // with a frame is re-render, and re-rendering a screen that says the
            // same thing is work nobody asked for: measured on 24 September
            // 2026, an idle player used 2.8 % of a core and the number was
            // identical with one film in the library and with two hundred and
            // fifty. The diagnostics move every tick by design - the startup
            // clock, the interface's own measurements - so they are taken out
            // of the comparison rather than out of the frame.
            let emitter = window.clone();
            std::thread::spawn(move || {
                let mut tick: u32 = 0;
                let mut last_visible = String::new();
                let mut last_sent = std::time::Instant::now() - Duration::from_secs(5);
                loop {
                    std::thread::sleep(Duration::from_millis(500));
                    // The rounded corners come back once the size has settled.
                    settle_round_region(&emitter);
                    let frame = player_status();
                    let mut value: serde_json::Value =
                        serde_json::from_str(&frame).unwrap_or(serde_json::Value::Null);
                    if let Some(map) = value.as_object_mut() {
                        for key in ["sinceStartMs", "osdWorstFrameMs", "osdSlowFrames", "osdFrames", "osdResizeEvents"] {
                            map.remove(key);
                        }
                    }
                    let visible = value.to_string();
                    if visible != last_visible || last_sent.elapsed() >= Duration::from_secs(2) {
                        let _ = emitter.emit("player-status", frame);
                        last_visible = visible;
                        last_sent = std::time::Instant::now();
                    }
                    supervise_audio(&emitter);
                    supervise_subtitles();
                    // The three things that happen on their own, on the thread
                    // that already ticks: the sound falling back, the sidecar
                    // subtitles arriving, and the next episode following this
                    // one. Each reads mpv and does at most one thing.
                    autoplay_next_episode();
                    // Every five seconds is often enough for a resume point and
                    // rare enough that an idle player makes no traffic at all.
                    tick += 1;
                    if tick % 10 == 0 {
                        save_progress();
                    }
                }
            });

            // Track selection from the command line, applied once the file has
            // had time to open. It calls the same command the OSD's menu calls,
            // so the menu's path is exercised without a click - and the result
            // is printed, because "the menu looked right" is not evidence.
            let audio_choice = flag("--audio").and_then(|v| v.parse::<i64>().ok());
            let subtitle_choice = flag("--sub").and_then(|v| v.parse::<i64>().ok());
            if audio_choice.is_some() || subtitle_choice.is_some() {
                std::thread::spawn(move || {
                    // Short enough to land before the audio fallback reloads the
                    // film, which is exactly the sequence worth testing: a
                    // choice made in the first seconds must survive it.
                    std::thread::sleep(Duration::from_millis(1500));
                    if let Some(id) = audio_choice {
                        match player_set_track("audio".into(), id) {
                            Ok(()) => println!("theia-player: audio track -> {id}"),
                            Err(e) => eprintln!("theia-player: audio track {id}: {e}"),
                        }
                    }
                    if let Some(id) = subtitle_choice {
                        match player_set_track("subtitle".into(), id) {
                            Ok(()) => println!("theia-player: subtitle track -> {id}"),
                            Err(e) => eprintln!("theia-player: subtitle track {id}: {e}"),
                        }
                    }
                    std::thread::sleep(Duration::from_secs(1));
                    println!("theia-player: tracks now {}", player_tracks().unwrap_or_default());
                });
            }

            if diagnostics {
                std::thread::spawn(|| {
                    // Which build this is, before which engine it found. The
                    // engine pin below names the library; it does not say which
                    // Theia drew the picture, and a report that names neither
                    // cannot be acted on.
                    println!("theia-player: version {}", env!("THEIA_VERSION"));
                    // Once, at the top: which engine is running, where it came
                    // from and the digest it was pinned with. Decision 118
                    // accepts the LGPL's obligations in exchange for shipping
                    // libmpv, and naming the exact source and digest in the
                    // application's diagnostics is one of them - so it is
                    // printed where a bug report would be taken from, not buried
                    // in an about screen.
                    println!("engine-pin: {}", player_engine_pin());
                    let mut last = String::new();
                    let mut last_style = String::new();
                    loop {
                        std::thread::sleep(Duration::from_secs(1));
                        println!("{}", player_status());
                        // What the engine holds for subtitles, read back from it
                        // and printed on change: the sheet's own sliders change
                        // it while a film plays, and a refusal leaves the engine
                        // on its previous value, which is what a silent failure
                        // looks like from here.
                        let style = player_subtitles();
                        if style != last_style {
                            println!("subtitle-style: {style}");
                            last_style = style;
                        }
                        // Tracks arrive with the file and change when a
                        // subtitle is added, so they are printed on change
                        // rather than every second.
                        let tracks = player_tracks().unwrap_or_default();
                        if tracks != last {
                            println!("track-list: {tracks}");
                            last = tracks;
                        }
                    }
                });
            }

            Ok(())
        })
        .on_window_event(|window, event| {
            match event {
                tauri::WindowEvent::CloseRequested { .. } => {
                    // The five-second periodic save is a safety net. Closing is an
                    // explicit boundary and records the exact playhead before the
                    // process disappears.
                    save_progress();
                }
                // The region is a snapshot of a size, so a window that keeps
                // growing would be clipped to the shape it used to have - but
                // re-making it per event is what made a drag of the window's edge
                // lag (measured: five frames drawn against eighty-eight with it
                // left alone). So a resize *drops* the region, which cannot clip
                // anything, and the telemetry thread puts a fresh one back once
                // the size has settled. Decision 147 carries the numbers.
                tauri::WindowEvent::Resized(_) | tauri::WindowEvent::ScaleFactorChanged { .. } => {
                    note_resize_event(window);
                }
                _ => {}
            }
        })
        .run(tauri::generate_context!())
        .expect("theia-player could not start");
}

/// Watches for the failure spike A3 measured, and which the first run of this
/// player made worse than expected: when the endpoint refuses the raw stream,
/// mpv does not merely lose its sound, it never advances the film at all -
/// `time-pos` stays at zero with no audio output. So the trigger cannot be the
/// clock; it is "a file has been loaded, playback is not paused, and several
/// seconds later there is still no audio output".
///
/// When that happens the passthrough request is withdrawn, PCM is
/// re-established, and the OSD is told *why* as a code - never a sentence,
/// because the sentence belongs to the catalogue.
fn supervise_audio(app: &tauri::WebviewWindow) {
    const GRACE: std::time::Duration = std::time::Duration::from_millis(2500);

    let mut guard = match SESSION.lock() {
        Ok(g) => g,
        Err(_) => return,
    };
    let Some(session) = guard.as_mut() else { return };
    if session.audio != AudioMode::Passthrough {
        return;
    }
    let Some(loaded_at) = session.loaded_at else { return };
    if loaded_at.elapsed() < GRACE {
        return;
    }
    // Nothing to play, or the viewer paused: not an audio failure.
    let paused = session.engine.property("pause").as_deref() == Some("yes");
    if paused || session.engine.property("duration").is_none() {
        return;
    }
    if session.engine.property("current-ao").is_some() {
        return;
    }

    session.audio = AudioMode::Pcm;
    session.audio_reason = Some("endpoint-refused-bitstream");
    if let Err(e) = apply_audio_mode(&session.engine, AudioMode::Pcm) {
        eprintln!("theia-player: could not withdraw the passthrough request: {e}");
    }
    let _ = session.engine.command(&["ao-reload"]);
    // Reload so the stalled film starts again now that sound exists - from
    // where it was asked to start, not from the beginning, and with the
    // subtitle files beside it, which are not part of the file being reloaded.
    if let Some(media) = session.media.clone() {
        if let Err(e) = session.load(&media, None) {
            eprintln!("theia-player: could not restart the film on the PCM path: {e}");
        }
    }
    let _ = app.emit(
        "player-event",
        "{\"kind\":\"audio\",\"mode\":\"pcm\",\"reason\":\"endpoint-refused-bitstream\"}",
    );
    println!("theia-player: passthrough refused by the endpoint, fell back to PCM");
}

#[cfg(test)]
mod player_window_tests {
    use super::{fitted_window_size, manifest_key};

    #[test]
    fn the_manifest_is_keyed_the_way_assets_are_named() {
        // The three keys player/libmpv.json actually holds, asked the way Rust
        // would ask for them on each machine. The macOS row is the one that was
        // wrong: `std::env::consts::OS` says "macos" and the manifest says
        // "darwin", and the diagnostics printed a page of nulls because of it.
        assert_eq!(manifest_key("macos", "aarch64"), "darwin/arm64");
        assert_eq!(manifest_key("windows", "x86_64"), "windows/amd64");
        assert_eq!(manifest_key("linux", "aarch64"), "linux/arm64");
    }

    #[test]
    fn every_key_this_machine_could_produce_is_in_the_manifest() {
        let manifest: serde_json::Value =
            serde_json::from_str(include_str!("../../libmpv.json")).expect("the manifest parses");
        let platforms = manifest
            .get("platforms")
            .and_then(|value| value.as_object())
            .expect("the manifest has platforms");
        for (os, arch) in [("macos", "aarch64"), ("windows", "x86_64")] {
            let key = manifest_key(os, arch);
            assert!(
                platforms.contains_key(&key),
                "the manifest has no entry for {key}, so this machine's diagnostics would print nulls"
            );
        }
    }

    #[test]
    fn high_dpi_small_logical_monitor_stays_on_screen() {
        let (width, height) = fitted_window_size(720.0, 450.0);
        assert_eq!((width, height), (640.0, 360.0));
    }

    #[test]
    fn ordinary_desktop_keeps_the_designed_player_size() {
        let (width, height) = fitted_window_size(1920.0, 1080.0);
        assert_eq!((width, height), (1280.0, 720.0));
    }

    #[test]
    fn short_monitor_reduces_both_axes_without_distortion() {
        let (width, height) = fitted_window_size(1366.0, 768.0);
        assert!(width <= 1286.0 && height <= 688.0);
        assert!((width / height - 16.0 / 9.0).abs() < 0.001);
    }
}

/// The playback preferences and what they become in the engine.
///
/// These are tests of the mapping functions and not of the engine: they assert
/// the *value* each setting turns into, which is the whole of what the settings
/// sheet promises. Whether the picture then looks right is a question for the
/// pixel check over the real player, and is not claimed here.
#[cfg(test)]
mod playback_tests {
    use super::*;

    /// The defaults as the struct builds them, with one part of the style
    /// changed - so a test says only what it is about.
    fn style_with(change: impl FnOnce(&mut SubtitleStyle)) -> PlaybackPreferences {
        let mut prefs = PlaybackPreferences::default();
        change(&mut prefs.subtitle_style);
        prefs
    }

    /// The value the table would set `name` to, or a panic naming the property:
    /// a setting that vanished from the table is the failure these tests exist
    /// for, and `.unwrap_or_default()` would hide it behind an empty string.
    fn table_for(
        prefs: &PlaybackPreferences,
        original_language: Option<&str>,
        name: &str,
    ) -> String {
        playback_properties(prefs, original_language)
            .into_iter()
            .find(|(key, _)| *key == name)
            .map(|(_, value)| value)
            .unwrap_or_else(|| panic!("the table sets no {name}"))
    }

    fn property(prefs: &PlaybackPreferences, name: &str) -> String {
        table_for(prefs, None, name)
    }

    fn sets(prefs: &PlaybackPreferences, name: &str) -> bool {
        playback_properties(prefs, None)
            .iter()
            .any(|(key, _)| *key == name)
    }

    #[test]
    fn an_empty_payload_is_the_defaults_the_sheet_ships() {
        let parsed: PlaybackPreferences = serde_json::from_str("{}").expect("an empty payload");
        assert_eq!(parsed, PlaybackPreferences::default());
        // Spelled out rather than compared with the struct's `Default` alone:
        // these are the numbers in the page's own `types.ts`, and a default that
        // moved on one side only is the silent behaviour change this test is for.
        assert!(parsed.auto_play_next);
        assert_eq!(parsed.audio_language, AudioLanguage::Auto);
        assert_eq!(parsed.subtitle_language, SubtitleLanguage::Auto);
        assert_eq!(parsed.subtitle_style.size_px, 36.0);
        assert_eq!(parsed.subtitle_style.height_px, 120.0);
        assert_eq!(parsed.subtitle_style.colour, "#EDE7DC");
        assert_eq!(parsed.subtitle_style.outline, Outline::Shadow);
        assert_eq!(parsed.subtitle_style.background, "none");
        assert_eq!(parsed.subtitle_style.font, Font::Standard);
        assert!(!parsed.subtitle_style.bold);
    }

    #[test]
    fn a_payload_from_the_sheet_is_read_field_for_field() {
        // The shape the sheet actually sends, camelCase and nesting included: a
        // renamed field would leave every setting at its default with nothing
        // anywhere to show for it.
        let payload = r##"{
            "autoPlayNext": false,
            "audioLanguage": "vf",
            "subtitleLanguage": "en",
            "subtitleStyle": {
                "sizePx": 48,
                "heightPx": 160,
                "colour": "#112233",
                "outline": "outline",
                "background": "#445566",
                "font": "mono",
                "bold": true
            }
        }"##;
        let parsed: PlaybackPreferences =
            serde_json::from_str(payload).expect("the sheet's own payload");
        assert!(!parsed.auto_play_next);
        assert_eq!(parsed.audio_language, AudioLanguage::Vf);
        assert_eq!(parsed.subtitle_language, SubtitleLanguage::En);
        assert_eq!(parsed.subtitle_style.size_px, 48.0);
        assert_eq!(parsed.subtitle_style.height_px, 160.0);
        assert_eq!(parsed.subtitle_style.colour, "#112233");
        assert_eq!(parsed.subtitle_style.outline, Outline::Outline);
        assert_eq!(parsed.subtitle_style.background, "#445566");
        assert_eq!(parsed.subtitle_style.font, Font::Mono);
        assert!(parsed.subtitle_style.bold);
    }

    #[test]
    fn a_value_outside_the_contract_is_refused_rather_than_defaulted() {
        // The enumerated fields are refused by serde itself, with the value that
        // was wrong in the message.
        for payload in [
            r#"{"audioLanguage": "de"}"#,
            r#"{"subtitleLanguage": "de"}"#,
            r#"{"subtitleStyle": {"outline": "outline-and-shadow"}}"#,
            r#"{"subtitleStyle": {"font": "comic"}}"#,
        ] {
            let error = serde_json::from_str::<PlaybackPreferences>(payload)
                .expect_err("this payload names no setting the sheet can produce");
            assert!(error.to_string().contains("unknown variant"), "{payload}: {error}");
        }
        // The two colours travel as free text, because a background is `none` or
        // a colour - so they are checked by the validator, which names the field
        // the way the sheet knows it.
        let wrong_ink = style_with(|style| style.colour = "blue".to_string());
        assert!(wrong_ink.validate().unwrap_err().contains("subtitleStyle.colour"));
        let wrong_box = style_with(|style| style.background = "#EDE7D".to_string());
        assert!(wrong_box.validate().unwrap_err().contains("subtitleStyle.background"));
        // And what the sheet can produce passes, including the background box.
        assert!(PlaybackPreferences::default().validate().is_ok());
        let boxed = style_with(|style| style.background = "#203040".to_string());
        assert!(boxed.validate().is_ok());
    }

    #[test]
    fn the_sheet_sizes_are_converted_into_the_engines_own_unit() {
        // Two thirds, because the engine measures in pixels of a 720-tall window
        // while `sub-scale-by-window` is on and the sheet's slider is drawn
        // against a 1080-tall picture. The default and the top of the slider.
        assert_eq!(subtitle_font_size(36.0), 24.0);
        assert_eq!(subtitle_font_size(72.0), 48.0);
        // The height becomes a percentage of the picture rather than a margin:
        // the same 2/3, measured against the 720 rows it is a percentage of.
        // Checked at the three heights the engine was measured with.
        for height in [40.0, 120.0, 200.0] {
            let from_the_bottom = (100.0 - subtitle_position(height)) * 7.2;
            assert!(
                (from_the_bottom - height * 2.0 / 3.0).abs() < 0.001,
                "{height} px of height became {from_the_bottom} engine pixels"
            );
        }
    }

    #[test]
    fn the_properties_carry_those_values_as_the_engine_prints_them() {
        let prefs = style_with(|style| {
            style.size_px = 36.0;
            style.height_px = 120.0;
        });
        assert_eq!(property(&prefs, "sub-font-size"), "24.00");
        // The default height as a percentage: 100 - 120 / 10.8.
        assert_eq!(property(&prefs, "sub-pos"), "88.89");
        // And no margin on top of it: the engine's own default of 34 would lift
        // the band by about 23 of our pixels, above what the slider asked for.
        assert_eq!(property(&prefs, "sub-margin-y"), "0");
        // `force`, because with anything less the script's own font, size and
        // colour would be the ones drawn, and the script's own margins emptied -
        // `sub-pos` cannot override those on an ASS track.
        assert_eq!(property(&prefs, "sub-ass-override"), "force");
        assert_eq!(property(&prefs, "sub-ass-force-style"), "MarginV=0");
    }

    #[test]
    fn a_colour_reaches_the_engine_alpha_first() {
        // Measured against the engine: it prints colours as `#AARRGGBB` and reads
        // eight digits the same way, alpha first. The sheet sends six, which carry
        // none, so the alpha is written here - opaque, which is what the paper the
        // interface is drawn on is.
        assert_eq!(opaque_colour("#EDE7DC"), "#FFEDE7DC");
        assert_eq!(property(&PlaybackPreferences::default(), "sub-color"), "#FFEDE7DC");
        // Another shape goes through unchanged rather than mangled: the engine's
        // own refusal is the answer, and `apply_playback_to` prints it.
        assert_eq!(opaque_colour("red"), "red");
        assert_eq!(opaque_colour("#EDE7D"), "#EDE7D");
    }

    #[test]
    fn a_background_colour_wins_over_the_outline() {
        let prefs = style_with(|style| {
            style.outline = Outline::Outline;
            style.background = "#203040".to_string();
        });
        // A band across the picture, which is what television does; the engine's
        // `background-box` would fit the box to the text instead.
        assert_eq!(property(&prefs, "sub-border-style"), "opaque-box");
        // 70 % alpha, so the picture stays readable through the band.
        assert_eq!(property(&prefs, "sub-back-color"), "#B3203040");
        // And the outline's own properties are not set at all in this case, so
        // there is nothing stale for the next change to inherit.
        assert!(!sets(&prefs, "sub-border-size"));
        assert!(!sets(&prefs, "sub-shadow-offset"));
    }

    #[test]
    fn the_three_outlines_are_the_engines_two_styles() {
        let shadow = style_with(|style| style.outline = Outline::Shadow);
        assert_eq!(property(&shadow, "sub-border-style"), "outline-and-shadow");
        assert_eq!(property(&shadow, "sub-border-size"), "0");
        assert_eq!(property(&shadow, "sub-shadow-offset"), "1.2");
        assert!(!sets(&shadow, "sub-back-color"));

        let outline = style_with(|style| style.outline = Outline::Outline);
        assert_eq!(property(&outline, "sub-border-style"), "outline-and-shadow");
        assert_eq!(property(&outline, "sub-border-size"), "2.4");
        assert_eq!(property(&outline, "sub-shadow-offset"), "0");

        // The engine has no borderless style - its choices are
        // `outline-and-shadow`, `opaque-box` and `background-box` - so a fully
        // transparent box is how none is said.
        let none = style_with(|style| style.outline = Outline::Off);
        assert_eq!(property(&none, "sub-border-style"), "background-box");
        assert_eq!(property(&none, "sub-back-color"), "#00000000");
        assert!(!sets(&none, "sub-border-size"));
    }

    #[test]
    fn dark_text_inverts_its_own_outline() {
        assert!(!is_dark("#EDE7DC"));
        assert!(is_dark("#101010"));
        // The threshold, on both sides of it: #808080 is a luminance of 0.502 and
        // #7F7F7F is 0.498.
        assert!(!is_dark("#808080"));
        assert!(is_dark("#7F7F7F"));
        // A colour that cannot be read is treated as light, which is what nearly
        // every subtitle is.
        assert!(!is_dark("red"));

        let light = PlaybackPreferences::default();
        assert_eq!(property(&light, "sub-border-color"), "#FF000000");
        assert_eq!(property(&light, "sub-shadow-color"), "#FF000000");
        let dark = style_with(|style| style.colour = "#101010".to_string());
        assert_eq!(property(&dark, "sub-border-color"), "#FFEDE7DC");
        assert_eq!(property(&dark, "sub-shadow-color"), "#FFEDE7DC");
    }

    #[test]
    fn the_language_lists_are_what_the_engine_reads() {
        let with = |audio, subtitles| PlaybackPreferences {
            audio_language: audio,
            subtitle_language: subtitles,
            ..Default::default()
        };

        // `auto` asks for nothing, which is the file's own default. It stays
        // nothing even when the server did name the film's original language:
        // auto means the file decides, not the metadata.
        let auto = with(AudioLanguage::Auto, SubtitleLanguage::Auto);
        assert_eq!(audio_language_list(&auto, None), "");
        assert_eq!(audio_language_list(&auto, Some("en")), "");
        assert_eq!(subtitle_language_list(&auto), "");

        // The French version, and French subtitles: three-letter codes beside the
        // two-letter one, because a file's tracks carry whichever its maker wrote.
        let vf = with(AudioLanguage::Vf, SubtitleLanguage::Fr);
        assert_eq!(audio_language_list(&vf, None), "fr,fre,fra");
        assert_eq!(subtitle_language_list(&vf), "fr,fre,fra");

        // The original language, known and unknown: the film's own tongue when
        // the server said what it is, and no request at all when it did not.
        let vo = with(AudioLanguage::Vo, SubtitleLanguage::En);
        assert_eq!(audio_language_list(&vo, Some("de")), "de");
        assert_eq!(audio_language_list(&vo, None), "");
        assert_eq!(subtitle_language_list(&vo), "en,eng");

        // `none` and `auto` send the same empty list and mean different things:
        // no subtitles is said by the load option, not by a language list.
        let off = with(AudioLanguage::Auto, SubtitleLanguage::Off);
        assert_eq!(subtitle_language_list(&off), "");
    }

    #[test]
    fn the_table_carries_every_property_the_preferences_decide() {
        let prefs = PlaybackPreferences {
            audio_language: AudioLanguage::Vf,
            subtitle_language: SubtitleLanguage::Off,
            ..Default::default()
        };
        for name in [
            // The style, then the two language lists, then the outline - which is
            // what the two callers of this table apply, in one place.
            "sub-font-size",
            "sub-margin-y",
            "sub-pos",
            "sub-color",
            "sub-ass-override",
            "sub-ass-force-style",
            "sub-scale-by-window",
            "sub-font",
            "sub-bold",
            "sub-border-color",
            "sub-shadow-color",
            "alang",
            "slang",
            "sub-border-style",
        ] {
            assert!(sets(&prefs, name), "the table sets no {name}");
        }
        // `force` rather than the engine's own default of `scale`: an ASS track
        // would otherwise take none of the style, which was measured as a 27 px
        // white Arial where the sheet asked for bone at 24 px.
        assert_eq!(property(&prefs, "sub-ass-override"), "force");
        // And set rather than assumed, because the 2/3 conversion is only true
        // while it is on.
        assert_eq!(property(&prefs, "sub-scale-by-window"), "yes");
        assert_eq!(table_for(&prefs, Some("de"), "alang"), "fr,fre,fra");
        assert_eq!(table_for(&prefs, Some("de"), "slang"), "");
    }

    #[test]
    fn the_font_choices_are_the_generic_families() {
        // Generic names, because the engine resolves them through its own font
        // provider and the OSD ships no font the engine could load.
        assert_eq!(property(&style_with(|style| style.font = Font::Standard), "sub-font"), "sans-serif");
        assert_eq!(property(&style_with(|style| style.font = Font::Serif), "sub-font"), "serif");
        assert_eq!(property(&style_with(|style| style.font = Font::Mono), "sub-font"), "monospace");
    }

    #[test]
    fn bold_is_the_engines_own_flag() {
        assert_eq!(property(&PlaybackPreferences::default(), "sub-bold"), "no");
        assert_eq!(property(&style_with(|style| style.bold = true), "sub-bold"), "yes");
    }

    #[test]
    fn a_serialised_payload_reads_back_as_itself() {
        // The two programs are updated separately, so the shape this side writes
        // has to be the shape it reads - which is what a settings sheet upgraded
        // after the player is relying on.
        let prefs = style_with(|style| {
            style.size_px = 52.0;
            style.height_px = 168.0;
            style.colour = "#203040".to_string();
            style.outline = Outline::Off;
            style.background = "#AABBCC".to_string();
            style.font = Font::Serif;
            style.bold = true;
        });
        let text = serde_json::to_string(&prefs).expect("the preferences serialise");
        let back: PlaybackPreferences = serde_json::from_str(&text).expect("and read back");
        assert_eq!(back, prefs);
    }
}

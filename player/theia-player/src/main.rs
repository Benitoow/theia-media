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
    media: Option<String>,
    /// What to call what is playing. The OSD shows this rather than picking a
    /// filename out of a URL, which is what a stream address would reduce to.
    title: Option<String>,
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
    /// The first load and the audio fallback's reload both come through here.
    /// They must: a reload that forgot any of it would undo a viewer's choices,
    /// which the resume point already taught this project once, and would take
    /// the sidecar subtitles away the moment the endpoint refused a bitstream.
    fn load(&mut self, url: &str) -> Result<(), String> {
        let options = self.load_options();
        let mut args: Vec<&str> = vec!["loadfile", url, "replace", "0"];
        if let Some(ref options) = options {
            args.push(options);
        }
        self.engine.command(&args)?;
        // A reload drops everything that was added on top of the file, so the
        // two automatic steps below start again - from the same film, whose
        // sidecars this session still holds.
        self.sidecars_pending = self.sidecars.clone();
        self.sidecar_tries = 0;
        self.sidecar_decided = false;
        Ok(())
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
    fn load_options(&self) -> Option<String> {
        let mut options: Vec<String> = Vec::new();
        if self.start_at > 0.0 {
            options.push(format!("start={:.3}", self.start_at));
        }
        if let Some(aid) = self.aid {
            options.push(format!("aid={aid}"));
        }
        if let Some(sid) = self.sid {
            options.push(format!("sid={sid}"));
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
        self.loaded_at = None;
        self.start_at = 0.0;
        self.aid = None;
        self.sid = None;
        self.sidecars.clear();
        self.sidecars_pending.clear();
        self.sidecar_tries = 0;
        self.sidecar_decided = false;
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
    Episode(i64),
}

/// The playable record currently owned by mpv. Keeping the kind beside the id
/// matters: films and episodes have deliberately parallel progress endpoints,
/// not one polymorphic endpoint pretending the distinction does not exist.
static CURRENT_MEDIA: Mutex<Option<Playing>> = Mutex::new(None);

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
        "pos": number("time-pos"),
        "duration": number("duration"),
        "vo": text("current-vo"),
        "hwdec": text("hwdec-current"),
        "ao": text("current-ao"),
        "audioMode": session.audio.code(),
        "audioReason": session.audio_reason,
        "videoCodec": text("video-format"),
        "startAt": session.start_at,
        "aid": integer("aid"),
        "sid": integer("sid"),
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
/// Rust says `x86_64` where a release asset says `amd64`, and the manifest is
/// keyed the way assets are named because it describes an asset. Left unmapped,
/// the lookup found nothing and the diagnostics printed a page of nulls - which
/// is how the first run of this reported itself: the pin was there, correct, and
/// silently not the one being asked about.
fn manifest_platform() -> String {
    let arch = match std::env::consts::ARCH {
        "x86_64" => "amd64",
        "aarch64" => "arm64",
        other => other,
    };
    format!("{}/{}", std::env::consts::OS, arch)
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

/// Seeks, relative or absolute, in seconds. The mode is passed through rather
/// than guessed: a scrub bar asks for an absolute position and a skip button
/// asks for a relative one, and mixing them up lands in the wrong place in the
/// film.
#[tauri::command]
fn player_seek(seconds: f64, mode: String) -> Result<(), String> {
    let mode = match mode.as_str() {
        "absolute" => "absolute",
        "relative" => "relative",
        other => return Err(format!("unknown seek mode {other:?}")),
    };
    let guard = SESSION.lock().unwrap();
    let session = guard.as_ref().ok_or("the engine is not running")?;
    session
        .engine
        .command(&["seek", &format!("{seconds:.3}"), mode])
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
    let (url, movie_id, title, resume_at, sidecars) = {
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
            client.stream_url(movie.id, file.id),
            movie.id,
            if movie.metadata.title.is_empty() {
                movie.title.clone()
            } else {
                movie.metadata.title.clone()
            },
            resume_at,
            sidecars,
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
        if let Err(e) = session.load(&url) {
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
    let (url, episode_id, title, resume_at, sidecars) = {
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
            client.episode_stream_url(episode.id, file.id),
            episode.id,
            title,
            resume_at,
            sidecars,
        )
    };

    {
        let mut session_guard = SESSION.lock().unwrap();
        let session = session_guard.as_mut().ok_or("the engine is not running")?;
        session.mark_loaded(url.clone(), Some(title), resume_at, sidecars);
        if let Err(e) = session.load(&url) {
            session.loaded_at = None;
            return Err(e);
        }
    }
    *CURRENT_MEDIA.lock().unwrap() = Some(Playing::Episode(episode_id));
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

/// Applies the region from a window's own geometry. Two thin wrappers, because
/// the setup hook holds a `WebviewWindow` and the event hook a `Window`, and
/// both answer the same three questions.
#[cfg(windows)]
fn round_window(window: &tauri::Window) {
    let Ok(hwnd) = window.hwnd() else { return };
    let size = window.inner_size().unwrap_or_default();
    let scale = window.scale_factor().unwrap_or(1.0);
    let full = window.is_maximized().unwrap_or(false) || window.is_fullscreen().unwrap_or(false);
    apply_round_region(hwnd.0 as isize, size.width, size.height, scale, !full);
}

#[cfg(windows)]
fn round_webview_window(window: &tauri::WebviewWindow) {
    let Ok(hwnd) = window.hwnd() else { return };
    let size = window.inner_size().unwrap_or_default();
    let scale = window.scale_factor().unwrap_or(1.0);
    let full = window.is_maximized().unwrap_or(false) || window.is_fullscreen().unwrap_or(false);
    apply_round_region(hwnd.0 as isize, size.width, size.height, scale, !full);
}

// The two stubs below exist so the setup and event hooks read the same on every
// platform. On macOS and the other Unix they have nothing to do: the rounding is
// `SetWindowRgn`, and the window is a normal `NSWindow`.

#[cfg(not(windows))]
fn round_window(_window: &tauri::Window) {}

#[cfg(not(windows))]
fn round_webview_window(_window: &tauri::WebviewWindow) {}

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
        let number = |name: &str| {
            session
                .engine
                .property(name)
                .and_then(|v| v.parse::<f64>().ok())
                .unwrap_or(0.0)
        };
        (number("time-pos"), number("duration"))
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
        Playing::Episode(id) => client.save_episode_progress(id, position, duration),
    };
    if let Err(e) = result {
        eprintln!("theia-player: {e}");
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
        media: None,
        title: None,
        loaded_at: None,
        start_at: 0.0,
        aid: None,
        sid: None,
        sidecars: Vec::new(),
        sidecars_pending: Vec::new(),
        sidecar_tries: 0,
        sidecar_decided: false,
        audio_reason: None,
    };
    if let Some(path) = media {
        let title = std::path::Path::new(path)
            .file_name()
            .map(|n| n.to_string_lossy().into_owned());
        session.mark_loaded(path.to_string(), title, 0.0, Vec::new());
    }
    *SESSION.lock().unwrap() = Some(session);
    Ok(())
}

/// Reads `--flag value`. Written by hand rather than pulled from a crate: a
/// handful of flags does not justify a dependency, and the parser is four lines.
fn flag(name: &str) -> Option<String> {
    std::env::args().skip_while(|a| a != name).nth(1)
}

fn main() {
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
            player_tracks,
            player_set_track,
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
            let emitter = window.clone();
            std::thread::spawn(move || {
                let mut tick: u32 = 0;
                loop {
                    std::thread::sleep(Duration::from_millis(500));
                    let _ = emitter.emit("player-status", player_status());
                    supervise_audio(&emitter);
                    supervise_subtitles();
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
                    loop {
                        std::thread::sleep(Duration::from_secs(1));
                        println!("{}", player_status());
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
                // The region is a snapshot of a size, so every resize needs a new
                // one: a stale region would clip a bigger window to the shape it
                // used to have. Maximizing and fullscreen clear it instead, which
                // is why the helper asks the window rather than a flag of ours.
                tauri::WindowEvent::Resized(_) | tauri::WindowEvent::ScaleFactorChanged { .. } => {
                    round_window(window);
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
        if let Err(e) = session.load(&media) {
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
    use super::fitted_window_size;

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

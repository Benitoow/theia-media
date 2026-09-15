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
//!   theia-player --diagnostics    print the session state once a second

mod mpv;
mod server;

use mpv::Engine;
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
    Passthrough,
    /// Decoding to PCM, because the endpoint would not take the raw stream.
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
}

static SESSION: Mutex<Option<Session>> = Mutex::new(None);

/// The connected server, kept apart from the session because browsing a library
/// must work even when the engine could not start: a player that shows nothing
/// at all because a DLL is missing is worse than one that says so.
static CLIENT: Mutex<Option<server::Client>> = Mutex::new(None);

/// The film being watched, so progress can be written back to the server. The
/// engine knows where the playhead is; only this side knows which record it
/// belongs to.
static CURRENT_MOVIE: Mutex<Option<i64>> = Mutex::new(None);

/// The options the player starts with. Kept in one place so the policy is
/// readable rather than scattered through the setup closure.
///
/// `silent` starts muted. It exists because a player that always makes a noise
/// is hostile in a shared room - and because every automated run of this
/// program has no business producing sound on somebody's machine.
fn base_options(hwnd: isize, silent: bool) -> Vec<(&'static str, String)> {
    vec![
        ("wid", hwnd.to_string()),
        ("vo", "gpu-next".into()),
        ("gpu-api", "d3d11".into()),
        ("gpu-context", "d3d11".into()),
        ("hwdec", "d3d11va".into()),
        ("ao", "wasapi".into()),
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
    ]
}

/// The passthrough request, kept apart from the options above because it is
/// withdrawn at runtime when the endpoint refuses it.
const SPDIF_CODECS: &str = "ac3,eac3,dts,dts-hd,truehd";

fn apply_audio_mode(engine: &Engine, mode: AudioMode) -> Result<(), String> {
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
    }
    let payload = serde_json::json!({
        "url": client.base(),
        "health": health,
        "profile": client.profile(),
        "profiles": profiles,
    });
    let json = serde_json::to_string(&payload).map_err(|e| e.to_string())?;
    *CLIENT.lock().unwrap() = Some(client);
    Ok(json)
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
            movie.title.clone(),
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
    *CURRENT_MOVIE.lock().unwrap() = Some(movie_id);
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
fn player_play(id: i64) -> Result<String, String> {
    play_movie(id)
}

/// Writes the playhead back to the server. Deliberately the same call the
/// browser player makes, so a film started in one is resumable in the other.
fn save_progress() {
    let Some(movie_id) = *CURRENT_MOVIE.lock().unwrap() else {
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
    if let Err(e) = client.save_progress(movie_id, position, duration) {
        eprintln!("theia-player: {e}");
    }
}

/// Starts the engine, or reports why it could not start. Called from setup so
/// a missing DLL becomes a message rather than a crash.
fn start_engine(hwnd: isize, media: Option<&str>, silent: bool) -> Result<(), String> {
    let dll = mpv::engine_path()?;
    let engine = Engine::load(&dll)?;

    for (name, value) in base_options(hwnd, silent) {
        // hwdec and the passthrough request are allowed to fail on a machine
        // that cannot honour them; everything else failing is fatal.
        engine.set_option(name, &value)?;
    }
    apply_audio_mode(&engine, AudioMode::Passthrough)?;
    engine.initialize()?;

    if let Some(path) = media {
        engine.command(&["loadfile", path])?;
    }

    let mut session = Session {
        engine,
        audio: AudioMode::Passthrough,
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
            player_toggle_pause,
            player_set_muted,
            player_tracks,
            player_set_track,
            player_seek,
            player_load,
            player_discover,
            player_connect,
            player_set_profile,
            player_library,
            player_play
        ])
        .setup(move |app| {
            let window = app.get_webview_window("main").expect("the main window");
            let hwnd = {
                #[cfg(windows)]
                {
                    window.hwnd().expect("a window handle").0 as isize
                }
                #[cfg(not(windows))]
                {
                    0isize
                }
            };

            match start_engine(hwnd, media.as_deref(), silent) {
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
            let _ = window.show();

            // Command-line connection, so the client can be exercised without a
            // click. The OSD drives the same two functions.
            if let Some(url) = flag("--server") {
                match connect_to(&url) {
                    Ok(info) => println!("theia-player: connected to {url} -> {info}"),
                    Err(e) => eprintln!("theia-player: could not connect to {url}: {e}"),
                }
                if let Some(id) = flag("--play").and_then(|v| v.parse::<i64>().ok()) {
                    match play_movie(id) {
                        Ok(stream) => println!("theia-player: playing {stream}"),
                        Err(e) => eprintln!("theia-player: could not start film {id}: {e}"),
                    }
                }
            }

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

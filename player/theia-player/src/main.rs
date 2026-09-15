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
    /// that film and not to this one.
    fn mark_loaded(&mut self, source: String, title: Option<String>, start_at: f64) {
        self.media = Some(source);
        self.title = title;
        self.start_at = start_at;
        self.aid = None;
        self.sid = None;
        self.loaded_at = Some(Instant::now());
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
        "sid" => session.sid = chosen,
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
    session.mark_loaded(path, title, 0.0);
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
    let (url, movie_id, title, resume_at) = {
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
        (
            client.stream_url(movie.id, file.id),
            movie.id,
            movie.title.clone(),
            resume_at,
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
        session.mark_loaded(url.clone(), Some(title), resume_at);
        let options = session.load_options();
        let mut args: Vec<&str> = vec!["loadfile", &url, "replace", "0"];
        if let Some(ref options) = options {
            args.push(options);
        }
        if let Err(e) = session.engine.command(&args) {
            // Nothing was loaded, so the watchdog must not act on it.
            session.loaded_at = None;
            return Err(e);
        }
    }
    *CURRENT_MOVIE.lock().unwrap() = Some(movie_id);
    Ok(url)
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
        audio_reason: None,
    };
    if let Some(path) = media {
        let title = std::path::Path::new(path)
            .file_name()
            .map(|n| n.to_string_lossy().into_owned());
        session.mark_loaded(path.to_string(), title, 0.0);
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
    // where it was asked to start, not from the beginning.
    if let Some(media) = session.media.clone() {
        let options = session.load_options();
        let mut args: Vec<&str> = vec!["loadfile", &media, "replace", "0"];
        if let Some(ref options) = options {
            args.push(options);
        }
        if let Err(e) = session.engine.command(&args) {
            eprintln!("theia-player: could not restart the film on the PCM path: {e}");
        }
    }
    let _ = app.emit(
        "player-event",
        "{\"kind\":\"audio\",\"mode\":\"pcm\",\"reason\":\"endpoint-refused-bitstream\"}",
    );
    println!("theia-player: passthrough refused by the endpoint, fell back to PCM");
}

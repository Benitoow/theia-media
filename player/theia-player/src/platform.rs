//! mpv video, audio and native window-handle policy for each platform.

/// The engine options that differ by platform: how the picture reaches the
/// window, and how the sound leaves the machine.
///
/// The shape is identical on every platform on purpose: `start_engine` sets
/// these options one by one and treats a refusal as fatal, so a value mpv does
/// not know becomes a player that says why it will not start rather than one that
/// renders on another backend in silence.
///
/// **The platform is an argument rather than a `cfg` block, and that is the fix
/// for a real fault.** Decision 149 found Linux being told to use `d3d11` and
/// `wasapi` - a Windows table, in a `cfg` block no Windows machine and no test
/// could read, which is why it survived from the day the table was written until
/// somebody audited the source. The same reasoning as `manifest_key`: a mapping
/// is testable without the machine that has the problem.
///
/// Linux's host-painted output is independent of GPU availability.
pub(super) fn platform_options(
    platform: Platform,
    dri_present: bool,
) -> Vec<(&'static str, String)> {
    let table = match platform {
        Platform::Windows => WINDOWS_OPTIONS,
        Platform::Macos => MACOS_OPTIONS,
        Platform::Unix => UNIX_OPTIONS,
    };
    table
        .iter()
        .map(|(key, value)| {
            let value = if *key == "vo" && platform == Platform::Unix {
                linux_video_output(dri_present)
            } else {
                value
            };
            (*key, value.to_string())
        })
        .collect()
}

/// Whether this machine exposes a DRI device, which is what EGL and Vulkan need
/// on Linux. Always false elsewhere, where the question does not exist.
pub(super) fn dri_present() -> bool {
    cfg!(not(any(windows, target_os = "macos")))
        && std::fs::read_dir("/dev/dri")
            .map(|entries| {
                entries.flatten().any(|entry| {
                    entry.file_name().to_string_lossy().starts_with("renderD")
                        && std::fs::OpenOptions::new()
                            .read(true)
                            .write(true)
                            .open(entry.path())
                            .is_ok()
                })
            })
            .unwrap_or(false)
}

/// GTK owns Linux's painted film plane. GPU availability affects the web
/// compositor policy, but cannot select a video child that covers the OSD.
pub(super) fn linux_video_output(_dri_present: bool) -> &'static str {
    "libmpv"
}

/// Windows: exactly what this player has always set - D3D11 through gpu-next,
/// WASAPI for sound - and nothing here changes it.
const WINDOWS_OPTIONS: &[(&str, &str)] = &[
    ("vo", "gpu-next"),
    ("gpu-api", "d3d11"),
    ("gpu-context", "d3d11"),
    ("hwdec", "d3d11va"),
    ("ao", "wasapi"),
];

/// macOS: the requested output uses the AppKit/OpenGL render bridge. It still
/// needs native frame and compositing proof (decision 149).
///
/// `vo=libmpv` means "no video output at all": the host creates a GL context and
/// must call `mpv_render_context_render` for every frame, which is the only way a
/// frame can reach a window on macOS with this engine - its build disables
/// `vulkan`, `macos-cocoa-cb` and `swift-build`, so none of mpv's own outputs can
/// present, and `--wid` is gone in 0.41. `RENDER-MACOS.md` is the specification
/// and the list of what the Mac still has to settle.
///
/// `videotoolbox` is the engine's hardware decoder (`videotoolbox-gl=enabled` in
/// its pin) and `coreaudio` its audio output. Nothing here asks for bitstream
/// passthrough: CoreAudio has no path for TrueHD, Atmos or DTS-HD MA, so the
/// session starts in PCM and stays there (`apply_audio_mode` is a no-op on macOS).
const MACOS_OPTIONS: &[(&str, &str)] = &[
    ("vo", "libmpv"),
    ("hwdec", "videotoolbox"),
    ("ao", "coreaudio"),
];

/// Initial Linux path: software decoding/rendering through the host's GTK
/// canvas. No GPU or HDR capability is inferred from other platform engines.
const UNIX_OPTIONS: &[(&str, &str)] = &[
    ("vo", "libmpv"),
    ("hwdec", "no"),
    ("ao", "pipewire,pulse,alsa"),
];

/// Whether this platform takes a window id at all.
///
/// macOS does not: mpv 0.41 removed `--wid` there - the file that read a view out
/// of `WinID` is gone - and the film reaches the window through the render bridge
/// instead. Linux also uses the host render API. Only Windows takes an `HWND`.
///
/// A mapping rather than a `cfg`, for the reason the option tables are one: the
/// first version of the test below asserted the Windows behaviour unconditionally
/// and failed on the macOS runner, which is the same class of fault decision 149
/// found on Linux - a platform rule that only existed where nobody could read it.
pub(super) fn takes_window_id(platform: Platform) -> bool {
    platform == Platform::Windows
}

/// The platforms this player builds for, named so the tables above can be read
/// and tested from any of them.
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub(super) enum Platform {
    Windows,
    Macos,
    Unix,
}

/// The platform this build is for.
///
/// Read from `std::env::consts::OS` rather than chosen by a `cfg`, for two
/// reasons: it is the same source `manifest_key` maps from, so the table and the
/// engine pin cannot disagree about what this machine is; and every variant is
/// constructed in one place, which is what keeps a three-arm table free of the
/// dead-code warning that a `cfg`-selected constant earns on the two platforms it
/// is not compiled for.
fn current_platform() -> Platform {
    match std::env::consts::OS {
        "windows" => Platform::Windows,
        "macos" => Platform::Macos,
        _ => Platform::Unix,
    }
}

/// The options the player starts with. Kept in one place so the policy is
/// readable rather than scattered through the setup closure.
///
/// `wid` is sent only on Windows. The Mac and Linux host render bridges are
/// attached after mpv starts, before the first file loads.
///
/// `silent` starts muted. It exists because a player that always makes a noise
/// is hostile in a shared room - and because every automated run of this
/// program has no business producing sound on somebody's machine.
pub(super) fn base_options(wid: isize, silent: bool) -> Vec<(&'static str, String)> {
    let mut options: Vec<(&'static str, String)> = Vec::new();
    // `wid` is the handle the platform gave us: an `HWND` on Windows, an X11
    // window id on Linux, and nothing on macOS any more. Its NSView goes to
    // render_macos after mpv starts.
    // mpv 0.41 stopped reading `wid` on macOS: the paragraph
    // documenting an `NSView*` left the manual after 0.36 and no macOS file in
    // 0.41.0 reads the option.
    //
    // A zero is left out rather than sent. `wid=0` is not "no embedding" to mpv,
    // it is a request for a window of the engine's own - so a Wayland session,
    // which has no id to hand over, would put the film in a second window beside
    // the OSD instead of under it, and say nothing about why. macOS takes no id
    // at all, whatever it is given: see [`takes_window_id`].
    if takes_window_id(current_platform()) && wid != 0 {
        options.push(("wid", wid.to_string()));
    }
    options.extend(platform_options(current_platform(), dri_present()));
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
        // On Mac the host owns the film surface. Creating mpv's VO during
        // initialize(), before the render context exists, is invalid and
        // produces "No render context set". It must start with the first file.
        (
            "force-window",
            if current_platform() == Platform::Windows {
                "yes".into()
            } else {
                "no".into()
            },
        ),
    ]);
    options
}

/// Prepare the host-painted GTK canvas, then name the outer X window for
/// diagnostics/probes. This id is never passed to the Linux media engine.
#[cfg(not(any(windows, target_os = "macos")))]
pub(super) fn x11_window_id(window: &tauri::WebviewWindow) -> isize {
    use gtk::prelude::*;
    if let Err(error) = crate::render_linux::prepare(window) {
        eprintln!("theia-player: GTK film canvas: {error}");
        return 0;
    }
    let Ok(host) = window.gtk_window() else {
        return 0;
    };
    host.realize();
    host.window()
        .and_then(|window| window.downcast::<gdkx11::X11Window>().ok())
        .map(|window| window.xid() as isize)
        .unwrap_or(0)
}

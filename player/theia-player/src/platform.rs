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
/// `dri_present` is the second half of the same idea: on Linux one option is a
/// question for the machine, and a test cannot answer it either - so it is passed
/// in rather than looked up here. See [`linux_video_output`].
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
    cfg!(not(any(windows, target_os = "macos"))) &&
        std::fs::read_dir("/dev/dri").map(|entries| {
            entries.flatten().any(|entry| {
                entry.file_name().to_string_lossy().starts_with("renderD") &&
                    std::fs::OpenOptions::new().read(true).write(true).open(entry.path()).is_ok()
            })
        }).unwrap_or(false)
}

/// The video output Linux is told to use, which is not always the same one.
///
/// `gpu-next` is the output that *should* be used: libplacebo over Vulkan or EGL,
/// with hardware decoding, and it is what this player asks for on Windows and
/// macOS. A Linux desktop with a working graphics stack answers it.
///
/// But mpv 0.41 cannot be *asked* whether the stack works. When EGL cannot create
/// a screen, its X11 GL path fails and then hits `vo_x11_init`'s own assertion
/// (`!vo->x11` failed), which aborts the process - measured twice on 27 September
/// 2026 in WSLg, where `/dev/dri` does not exist: once through the player, once
/// through `mpv --vo=gpu-next` alone, with `Suspect software renderer or indirect
/// context` immediately before it. There is no refusal for the player to react to,
/// and the abort takes the whole application with it, so the choice has to be made
/// before the engine starts.
///
/// A machine with no DRI device has no EGL and no Vulkan, so the software X11
/// output is the one that will draw: `vo=x11` is a legacy output with bad
/// performance and a picture, and a picture is what the alternative does not have.
pub(super) fn linux_video_output(dri_present: bool) -> &'static str {
    if dri_present {
        // A DRI directory does not establish a usable device/context: native
        // Ubuntu runners expose it while their X display uses llvmpipe. mpv
        // tries the software output only when GPU initialisation refuses.
        "gpu-next,x11"
    } else {
        "x11"
    }
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

/// Linux: mpv's own Linux stack, and nothing borrowed from another platform.
///
/// `gpu-api` and `gpu-context` are deliberately **absent**. Which pair is right
/// depends on the machine - Vulkan or OpenGL, X11 or Wayland - and mpv probes
/// for that itself. Naming one here would refuse to start on every machine that
/// chose the other, and a refusal is fatal by design, so it would be a player
/// that will not open a film rather than one that picked the slower backend.
///
/// `hwdec=auto-safe` is the engine's own conservative choice: VA-API, VDPAU or
/// NVDEC where the driver offers them, software where it does not.
///
/// `ao` is a preference list rather than one driver, because the answer belongs
/// to the machine: PipeWire on a current desktop, PulseAudio on an older one,
/// ALSA on a bare console.
const UNIX_OPTIONS: &[(&str, &str)] = &[
    ("vo", "gpu-next"),
    ("hwdec", "auto-safe"),
    ("ao", "pipewire,pulse,alsa"),
];

/// Whether this platform takes a window id at all.
///
/// macOS does not: mpv 0.41 removed `--wid` there - the file that read a view out
/// of `WinID` is gone - and the film reaches the window through the render bridge
/// instead. Windows and Linux both do: an `HWND`, and an X11 window id.
///
/// A mapping rather than a `cfg`, for the reason the option tables are one: the
/// first version of the test below asserted the Windows behaviour unconditionally
/// and failed on the macOS runner, which is the same class of fault decision 149
/// found on Linux - a platform rule that only existed where nobody could read it.
pub(super) fn takes_window_id(platform: Platform) -> bool {
    platform != Platform::Macos
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
/// `wid` is sent on Windows and Linux. macOS omits it because mpv 0.41 does not
/// read an `NSView*` through this option. The NSView is instead passed to the
/// render bridge after mpv starts.
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
        ("force-window", if current_platform() == Platform::Macos { "no".into() } else { "yes".into() }),
    ]);
    options
}

/// The X11 window id mpv should draw into, or zero when the session has none.
///
/// Read from GTK's own window, because the portable way does not work here: a
/// Tauri window on Linux is a `gtk::ApplicationWindow`, and its
/// `raw_window_handle` handle answers `Unavailable` - measured on 27 September
/// 2026 in WSLg, where the player printed `could not read the window handle: the
/// underlying handle is not available` and handed mpv a zero. `gtk` and `gdkx11`
/// are already in the graph through tauri and wry; they are named here because
/// this crate calls them.
///
/// Wayland answers zero, and that is not a failure to look it up: a Wayland
/// surface belongs to the client that created it, so there is no id another
/// process could be pointed at. `base_options` leaves `wid` out in that case, and
/// mpv opens a window of its own - a film that plays in its own window beside an
/// OSD that cannot sit over it. That is a known limit of this platform, not a
/// silent one: `RENDER-LINUX.md` records what a Wayland session needs instead.
#[cfg(not(any(windows, target_os = "macos")))]
pub(super) fn x11_window_id(window: &tauri::WebviewWindow) -> isize {
    use gtk::prelude::*;

    let gtk_window = match window.gtk_window() {
        Ok(window) => window,
        Err(error) => {
            eprintln!("theia-player: could not read the GTK window: {error}");
            return 0;
        }
    };
    // mpv creates a native child inside its `wid`. Pointing it at the whole
    // GTK window covers a windowless WebKit widget, even though its DOM says
    // the controls are visible. Keep video and the transparent web plane in
    // sibling native windows; GTK Overlay then owns their stacking order.
    let content = match gtk_window.child() {
        Some(content) => content,
        None => {
            eprintln!("theia-player: the GTK window has no interface child");
            return 0;
        }
    };
    gtk_window.remove(&content);
    let video = gtk::DrawingArea::new();
    let osd = gtk::EventBox::new();
    osd.set_visible_window(true);
    osd.set_app_paintable(true);
    if let Some(screen) = gtk::prelude::WidgetExt::screen(&gtk_window) {
        // X11's software video output writes RGB pixels, not a premultiplied
        // alpha channel. Inheriting the transparent parent's RGBA visual makes
        // those pixels transparent to the compositor. Only the OSD uses RGBA.
        if let Some(video_visual) = screen.system_visual() {
            video.set_visual(Some(&video_visual));
            eprintln!("theia-player: GTK video visual depth {}", video_visual.depth());
        }
        if let Some(visual) = screen.rgba_visual() {
            osd.set_visual(Some(&visual));
        }
    }
    let css = gtk::CssProvider::new();
    if let Err(error) = css.load_from_data(b".theia-osd-plane { background-color: transparent; }") {
        eprintln!("theia-player: transparent GTK plane: {error}");
    }
    osd.style_context().add_class("theia-osd-plane");
    osd.style_context().add_provider(&css, gtk::STYLE_PROVIDER_PRIORITY_APPLICATION);
    osd.add(&content);
    let planes = gtk::Overlay::new();
    planes.add(&video);
    planes.add_overlay(&osd);
    planes.set_overlay_pass_through(&osd, false);
    gtk_window.add(&planes);
    planes.show_all();
    let gdk_window = match video.window() {
        Some(window) => window,
        None => {
            // GTK creates the GdkWindow when the widget is realised, and this
            // runs before the window is shown - the engine starts first, so a
            // film that cannot open does not flash a window at somebody.
            // Realising creates the GdkWindow without mapping it: the id exists,
            // and the window stays hidden until `show`.
            gtk_window.realize();
            video.realize();
            match video.window() {
                Some(window) => window,
                None => {
                    eprintln!(
                        "theia-player: the GTK window could not be realised, so mpv gets no window id"
                    );
                    return 0;
                }
            }
        }
    };
    match gdk_window.downcast::<gdkx11::X11Window>() {
        Ok(x11) => x11.xid() as isize,
        Err(_) => {
            eprintln!(
                "theia-player: this session has no X11 window id, so the film opens in the engine's own window"
            );
            0
        }
    }
}

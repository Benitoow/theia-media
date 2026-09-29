//! Native window sizing, shape, resize timing and verification probes.

#[cfg(target_os = "macos")]
use crate::render_macos;
use std::path::PathBuf;
use std::time::Duration;

/// The window's corner radius, in logical pixels. Section 6b of the design
/// system carries the reasoning: the reference is a Windows 11 caption bar, and
/// Windows 11 rounds its own windows at 8.
#[cfg(windows)]
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
        unsafe {
            CreateRoundRectRgn(
                0,
                0,
                width as i32 + 1,
                height as i32 + 1,
                diameter,
                diameter,
            )
        }
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
/// Clears the window region: one cheap call, and the corners are square until
/// it is put back.
#[cfg(windows)]
fn clear_round_region(hwnd: isize) {
    use windows_sys::Win32::Foundation::HWND;
    use windows_sys::Win32::Graphics::Gdi::SetWindowRgn;
    unsafe { SetWindowRgn(hwnd as HWND, std::ptr::null_mut(), 1) };
}

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

/// Snapshot for the player status; a read never resets resize measurements.
pub(super) fn resize_timing() -> (Option<std::time::Instant>, u32, f64, f64) {
    RESIZE_TIMING
        .lock()
        .map(|guard| *guard)
        .unwrap_or((None, 0, 0.0, 0.0))
}

/// When the last resize arrived, and whether the rounded region is applied.
///
/// `None` for the time means no resize is in flight. The telemetry thread is
/// what puts the region back, so this costs no timer and no thread of its own.
#[cfg(windows)]
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
    let Ok(mut state) = RESIZE_STATE.lock() else {
        return;
    };
    if state.1 {
        clear_round_region(hwnd);
        state.1 = false;
    }
    state.0 = Some(now);
}

/// Puts the region back once the size has stopped moving.
#[cfg(windows)]
pub(super) fn settle_round_region(window: &tauri::WebviewWindow) {
    let Ok(mut state) = RESIZE_STATE.lock() else {
        return;
    };
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
pub(super) fn settle_round_region(_window: &tauri::WebviewWindow) {}

/// Applies the region from a window's own geometry.
///
/// One wrapper rather than two, since decision 147's resize pass: the event hook
/// now calls `note_resize` and the region is put back by the telemetry thread
/// through `settle_round_region`, so the `tauri::Window` half had no caller left.
#[cfg(windows)]
pub(super) fn round_webview_window(window: &tauri::WebviewWindow) {
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
pub(super) fn round_webview_window(_window: &tauri::WebviewWindow) {}

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
pub(super) fn note_resize_event(window: &tauri::Window) {
    if let Ok(hwnd) = window.hwnd() {
        note_resize(hwnd.0 as isize);
    }
}

/// Nothing on a Mac or the other Unix: there is no window region to drop, and
/// the timing that goes with it is a Windows measurement.
#[cfg(not(windows))]
pub(super) fn note_resize_event(_window: &tauri::Window) {}

/// Chooses a 16:9 logical window that fits the current monitor at any DPI.
pub(super) fn fitted_window_size(monitor_width: f64, monitor_height: f64) -> (f64, f64) {
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

pub(super) fn fit_initial_window(window: &tauri::WebviewWindow) {
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
pub(super) fn parse_window_size(value: &str) -> Option<(u32, u32)> {
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
pub(super) fn start_window_probe(
    window: tauri::WebviewWindow,
    wid: isize,
    requested: (u32, u32),
    path: PathBuf,
    wake_osd: bool,
) {
    std::thread::spawn(move || {
        // A hosted Mac's software renderer can take seconds per frame. Keep
        // asking while the verifier waits for the next one.
        let probe = if wake_osd {
            format!("if (!window.__theiaProofWake) {{ window.__theiaProofWake = window.setInterval(() => document.querySelector('.osd')?.dispatchEvent(new MouseEvent('mousemove', {{ bubbles: true }})), 500); }} {VIEWPORT_PROBE}")
        } else {
            VIEWPORT_PROBE.to_string()
        };
        for _ in 0..120 {
            let (tx, rx) = std::sync::mpsc::channel();
            let asked = window.eval_with_callback(&probe, move |answer| {
                let _ = tx.send(answer);
            });
            // The answer is waited for before the file is written, so the report
            // never says the page did not answer about a page that did.
            let page: serde_json::Value = match asked {
                Ok(()) => match rx.recv_timeout(if cfg!(target_os = "macos") {
                    Duration::from_secs(8)
                } else {
                    Duration::from_millis(400)
                }) {
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
            // The window server's id, on the platform that has one. `screencapture
            // -l` takes this number, and it is how the Mac verifier photographs
            // the film's window instead of the screen it may never appear on.
            #[cfg(target_os = "macos")]
            let window_number = window
                .ns_window()
                .map(render_macos::window_number)
                .unwrap_or(0);
            #[cfg(not(target_os = "macos"))]
            let window_number = 0_u64;
            let report = serde_json::json!({
                "requested": { "width": requested.0, "height": requested.1 },
                "physical": { "width": physical.width, "height": physical.height },
                "outer": { "width": outer.width, "height": outer.height },
                "scaleFactor": scale,
                "windowNumber": window_number,
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

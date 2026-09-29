//! Device preferences are independent of the viewing profile.
use std::{
    path::PathBuf,
    sync::{Mutex, OnceLock},
};
#[derive(Default, Clone, serde::Serialize, serde::Deserialize)]
struct State {
    volume: Option<f64>,
    muted: bool,
    window: Option<Geometry>,
}
#[derive(Clone, serde::Serialize, serde::Deserialize)]
struct Geometry {
    x: i32,
    y: i32,
    width: u32,
    height: u32,
    scale: f64,
    maximized: bool,
}
static GEOMETRY_PENDING: Mutex<Option<(tauri::Window, std::time::Instant)>> = Mutex::new(None);
static GEOMETRY_WORKER: std::sync::atomic::AtomicBool = std::sync::atomic::AtomicBool::new(false);
static STATE: OnceLock<Mutex<(PathBuf, State)>> = OnceLock::new();
pub(super) fn init(path: PathBuf) {
    let state = std::fs::read(&path)
        .ok()
        .and_then(|bytes| serde_json::from_slice(&bytes).ok())
        .unwrap_or_default();
    let _ = STATE.set(Mutex::new((path, state)));
}
fn update(change: impl FnOnce(&mut State)) {
    if let Some(state) = STATE.get() {
        if let Ok(mut guard) = state.lock() {
            change(&mut guard.1);
            if let Some(parent) = guard.0.parent() {
                let _ = std::fs::create_dir_all(parent);
            }
            if let Ok(bytes) = serde_json::to_vec(&guard.1) {
                let temporary = guard.0.with_extension("json.tmp");
                if std::fs::write(&temporary, bytes).is_ok() {
                    let _ = std::fs::rename(temporary, &guard.0);
                }
            }
        }
    }
}
pub(super) fn audio(volume: Option<f64>, muted: bool) {
    update(|state| {
        if let Some(volume) = volume {
            state.volume = Some(volume);
        }
        state.muted = muted;
    });
}
pub(super) fn restore_audio() {
    let state = STATE.get().and_then(|s| s.lock().ok().map(|g| g.1.clone()));
    if let Some(state) = state {
        if let Ok(guard) = crate::SESSION.lock() {
            if let Some(session) = guard.as_ref() {
                if let Some(volume) = state
                    .volume
                    .filter(|v| v.is_finite() && (0.0..=1.0).contains(v))
                {
                    let _ = session
                        .engine
                        .set_property("volume", &(volume * 100.0).to_string());
                }
                let _ = session
                    .engine
                    .set_property("mute", if state.muted { "yes" } else { "no" });
            }
        }
    }
}
pub(super) fn save_window(window: &tauri::Window) {
    if window.is_fullscreen().unwrap_or(true) || window.is_minimized().unwrap_or(true) {
        return;
    }
    if window.is_maximized().unwrap_or(false) {
        update(|s| {
            if let Some(g) = &mut s.window {
                g.maximized = true;
            }
        });
        return;
    }
    if let (Ok(position), Ok(size), Ok(scale)) = (
        window.outer_position(),
        window.inner_size(),
        window.scale_factor(),
    ) {
        update(|state| {
            state.window = Some(Geometry {
                x: position.x,
                y: position.y,
                width: size.width,
                height: size.height,
                scale,
                maximized: false,
            })
        });
    }
}
pub(super) fn schedule_window(window: &tauri::Window) {
    *GEOMETRY_PENDING.lock().unwrap() = Some((window.clone(), std::time::Instant::now()));
    if GEOMETRY_WORKER.swap(true, std::sync::atomic::Ordering::SeqCst) {
        return;
    }
    std::thread::spawn(|| loop {
        std::thread::sleep(std::time::Duration::from_millis(100));
        let target = {
            let mut pending = GEOMETRY_PENDING.lock().unwrap();
            match pending.as_ref() {
                None => {
                    GEOMETRY_WORKER.store(false, std::sync::atomic::Ordering::SeqCst);
                    return;
                }
                Some((_, at)) if at.elapsed() < std::time::Duration::from_millis(500) => None,
                _ => pending.take().map(|(window, _)| window),
            }
        };
        if let Some(window) = target {
            let target = window.clone();
            let _ = window.run_on_main_thread(move || save_window(&target));
        }
    });
}
pub(super) fn restore_window(window: &tauri::WebviewWindow) {
    let geometry = STATE
        .get()
        .and_then(|s| s.lock().ok().and_then(|g| g.1.window.clone()));
    let Some(geometry) = geometry else {
        return;
    };
    let Ok(monitors) = window.available_monitors() else {
        return;
    };
    let monitor = monitors
        .iter()
        .find(|m| {
            let p = m.position();
            let z = m.size();
            geometry.x >= p.x
                && geometry.y >= p.y
                && geometry.x < p.x + z.width as i32
                && geometry.y < p.y + z.height as i32
        })
        .or(monitors.first());
    let Some(monitor) = monitor else {
        return;
    };
    let area = monitor.work_area();
    let scale = monitor.scale_factor();
    let (x, y, width, height) = clamp_geometry(
        geometry.x,
        geometry.y,
        geometry.width,
        geometry.height,
        geometry.scale,
        scale,
        area.position.x,
        area.position.y,
        area.size.width,
        area.size.height,
    );
    let _ = window.set_size(tauri::PhysicalSize::new(width, height));
    let _ = window.set_position(tauri::PhysicalPosition::new(x, y));
    if geometry.maximized {
        let _ = window.maximize();
    }
}
#[allow(clippy::too_many_arguments)]
fn clamp_geometry(
    x: i32,
    y: i32,
    width: u32,
    height: u32,
    old_scale: f64,
    scale: f64,
    left: i32,
    top: i32,
    available_width: u32,
    available_height: u32,
) -> (i32, i32, u32, u32) {
    let ratio = if old_scale.is_finite() && old_scale > 0.0 {
        scale / old_scale
    } else {
        1.0
    };
    let width = ((width as f64 * ratio) as u32)
        .max((320.0 * scale) as u32)
        .min(available_width);
    let height = ((height as f64 * ratio) as u32)
        .max((180.0 * scale) as u32)
        .min(available_height);
    (
        x.clamp(left, left + (available_width - width) as i32),
        y.clamp(top, top + (available_height - height) as i32),
        width,
        height,
    )
}
#[cfg(test)]
mod tests {
    #[test]
    fn detached_monitor_and_new_dpi_keep_the_window_inside_the_work_area() {
        let (x, y, w, h) =
            super::clamp_geometry(4000, -900, 2560, 1440, 2.0, 1.0, 0, 0, 1920, 1040);
        assert_eq!((x, y, w, h), (640, 0, 1280, 720));
    }
}

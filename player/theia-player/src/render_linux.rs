//! GTK owns both painted planes; no mpv child window can cover the controls.
//! The initial Linux renderer uses mpv's CPU API. Rendering stays on its own
//! thread; GTK only consumes the latest image and never calls the engine.

use crate::mpv::Engine;
use gtk::prelude::*;
use libloading::Library;
use std::cell::RefCell;
use std::ffi::{c_int, c_void};
use std::path::Path;
use std::rc::Rc;
use std::sync::{
    atomic::{AtomicBool, AtomicU64, Ordering},
    Arc, Mutex,
};
use std::thread::JoinHandle;
use std::time::{Duration, Instant};

#[repr(C)]
struct Param {
    kind: c_int,
    data: *mut c_void,
}
type Create = unsafe extern "C" fn(*mut *mut c_void, *mut c_void, *mut Param) -> c_int;
type Update = unsafe extern "C" fn(*mut c_void) -> u64;
type Render = unsafe extern "C" fn(*mut c_void, *mut Param) -> c_int;
type Swap = unsafe extern "C" fn(*mut c_void);
type Callback =
    unsafe extern "C" fn(*mut c_void, Option<unsafe extern "C" fn(*mut c_void)>, *mut c_void);
type Free = unsafe extern "C" fn(*mut c_void);

struct Frame {
    pixels: Vec<u8>,
    width: i32,
    height: i32,
    stride: i32,
    scale: i32,
}
#[derive(Clone)]
struct Surface {
    frame: Arc<Mutex<Option<Frame>>>,
    size: Arc<Mutex<(i32, i32, i32)>>,
}
thread_local! { static SURFACE: RefCell<Option<Surface>> = const { RefCell::new(None) }; }
struct Worker {
    stop: Arc<AtomicBool>,
    join: JoinHandle<()>,
}
static WORKER: Mutex<Option<Worker>> = Mutex::new(None);
static FRAMES: AtomicU64 = AtomicU64::new(0);

pub fn prepare(window: &tauri::WebviewWindow) -> Result<(), String> {
    let host = window.gtk_window().map_err(|e| e.to_string())?;
    let content = host.child().ok_or("the GTK window has no interface")?;
    host.remove(&content);
    let canvas = gtk::DrawingArea::new();
    canvas.set_has_window(false);
    canvas.set_hexpand(true);
    canvas.set_vexpand(true);
    let surface = Surface {
        frame: Arc::new(Mutex::new(None)),
        size: Arc::new(Mutex::new((1280, 720, 1))),
    };
    let sizing = surface.size.clone();
    canvas.connect_size_allocate(move |widget, allocation| {
        let scale = widget.scale_factor();
        *sizing.lock().unwrap() = (
            allocation.width().max(1) * scale,
            allocation.height().max(1) * scale,
            scale,
        );
    });
    let latest = surface.frame.clone();
    let painted = Rc::new(RefCell::new(None::<(gtk::cairo::ImageSurface, i32)>));
    canvas.connect_draw(move |_, cairo| {
        if let Some(frame) = latest.lock().unwrap().take() {
            match gtk::cairo::ImageSurface::create_for_data(
                frame.pixels,
                gtk::cairo::Format::Rgb24,
                frame.width,
                frame.height,
                frame.stride,
            ) {
                Ok(image) => *painted.borrow_mut() = Some((image, frame.scale)),
                Err(error) => eprintln!("theia-player: GTK film image: {error}"),
            }
        }
        cairo.set_source_rgb(0.0, 0.0, 0.0);
        let _ = cairo.paint();
        if let Some((image, scale)) = painted.borrow().as_ref() {
            let _ = cairo.save();
            cairo.scale(1.0 / f64::from(*scale), 1.0 / f64::from(*scale));
            if cairo.set_source_surface(image, 0.0, 0.0).is_ok() {
                let _ = cairo.paint();
            }
            let _ = cairo.restore();
        }
        gtk::glib::Propagation::Proceed
    });
    let planes = gtk::Overlay::new();
    planes.add(&canvas);
    // WebKit needs its own transparent native window. Sharing the canvas's
    // parent drawing window prevented navigation/IPC on the native runner.
    let osd = gtk::EventBox::new();
    osd.set_visible_window(true);
    osd.set_app_paintable(true);
    if let Some(screen) = gtk::prelude::WidgetExt::screen(&host) {
        if let Some(visual) = screen.rgba_visual() {
            osd.set_visual(Some(&visual));
        }
    }
    let css = gtk::CssProvider::new();
    css.load_from_data(b".theia-osd-plane { background-color: transparent; }")
        .map_err(|error| error.to_string())?;
    osd.style_context().add_class("theia-osd-plane");
    osd.style_context()
        .add_provider(&css, gtk::STYLE_PROVIDER_PRIORITY_APPLICATION);
    osd.add(&content);
    planes.add_overlay(&osd);
    planes.set_overlay_pass_through(&osd, false);
    host.add(&planes);
    planes.show_all();
    let weak_canvas = canvas.downgrade();
    gtk::glib::timeout_add_local(Duration::from_millis(16), move || {
        match weak_canvas.upgrade() {
            Some(canvas) => {
                canvas.queue_draw();
                gtk::glib::ControlFlow::Continue
            }
            None => gtk::glib::ControlFlow::Break,
        }
    });
    SURFACE.with(|slot| *slot.borrow_mut() = Some(surface));
    Ok(())
}

unsafe extern "C" fn notified(_: *mut c_void) {}

pub fn attach(engine: &Engine, library: &Path) -> Result<(), String> {
    detach();
    let surface = SURFACE
        .with(|slot| slot.borrow().clone())
        .ok_or("the GTK film canvas is absent")?;
    let lib = unsafe { Library::new(library) }.map_err(|e| e.to_string())?;
    let (create, update, render, swap, callback, free): (
        Create,
        Update,
        Render,
        Swap,
        Callback,
        Free,
    ) = unsafe {
        (
            *lib.get(b"mpv_render_context_create")
                .map_err(|e| e.to_string())?,
            *lib.get(b"mpv_render_context_update")
                .map_err(|e| e.to_string())?,
            *lib.get(b"mpv_render_context_render")
                .map_err(|e| e.to_string())?,
            *lib.get(b"mpv_render_context_report_swap")
                .map_err(|e| e.to_string())?,
            *lib.get(b"mpv_render_context_set_update_callback")
                .map_err(|e| e.to_string())?,
            *lib.get(b"mpv_render_context_free")
                .map_err(|e| e.to_string())?,
        )
    };
    let handle = engine.raw_context() as usize;
    let stop = Arc::new(AtomicBool::new(false));
    let stopped = stop.clone();
    let (ready, receiver) = std::sync::mpsc::channel();
    FRAMES.store(0, Ordering::Relaxed);
    let join = std::thread::spawn(move || {
        let _library = lib;
        let mut context = std::ptr::null_mut();
        let mut advanced = 1_i32;
        let mut params = [
            Param {
                kind: 1,
                data: c"sw".as_ptr().cast_mut().cast(),
            },
            Param {
                kind: 10,
                data: (&mut advanced as *mut i32).cast(),
            },
            Param {
                kind: 0,
                data: std::ptr::null_mut(),
            },
        ];
        let result = unsafe { create(&mut context, handle as *mut c_void, params.as_mut_ptr()) };
        if result < 0 {
            let _ = ready.send(Err(format!("mpv software renderer refused ({result})")));
            return;
        }
        unsafe {
            callback(context, Some(notified), std::ptr::null_mut());
        }
        let _ = ready.send(Ok(()));
        let mut storage = Vec::<u8>::new();
        let mut rendered_size = None;
        while !stopped.load(Ordering::Relaxed) {
            let tick = Instant::now();
            let dimensions = *surface.size.lock().unwrap();
            let resized = rendered_size.is_some_and(|previous| previous != dimensions);
            // A paused film must still fill a resized window. Rendering the
            // existing frame does not require a fresh decoder notification.
            if unsafe { update(context) } & 1 != 0 || resized {
                let (width, height, scale) = dimensions;
                let mut size = [width, height];
                let mut stride = (width as usize * 4 + 63) & !63;
                let bytes = stride * height as usize;
                storage.resize(bytes + 63, 0);
                let offset = (64 - storage.as_ptr() as usize % 64) % 64;
                let mut params = [
                    Param {
                        kind: 17,
                        data: size.as_mut_ptr().cast(),
                    },
                    Param {
                        kind: 18,
                        data: c"bgr0".as_ptr().cast_mut().cast(),
                    },
                    Param {
                        kind: 19,
                        data: (&mut stride as *mut usize).cast(),
                    },
                    Param {
                        kind: 20,
                        data: unsafe { storage.as_mut_ptr().add(offset) }.cast(),
                    },
                    Param {
                        kind: 0,
                        data: std::ptr::null_mut(),
                    },
                ];
                let result = unsafe { render(context, params.as_mut_ptr()) };
                if result >= 0 {
                    *surface.frame.lock().unwrap() = Some(Frame {
                        pixels: storage[offset..offset + bytes].to_vec(),
                        width,
                        height,
                        stride: stride as i32,
                        scale,
                    });
                    unsafe {
                        swap(context);
                    }
                    FRAMES.fetch_add(1, Ordering::Relaxed);
                    rendered_size = Some(dimensions);
                } else {
                    eprintln!("theia-player: software frame refused ({result})");
                }
            }
            std::thread::sleep(Duration::from_millis(16).saturating_sub(tick.elapsed()));
        }
        unsafe {
            callback(context, None, std::ptr::null_mut());
            free(context);
        }
    });
    let result = receiver
        .recv()
        .unwrap_or_else(|error| Err(error.to_string()));
    if result.is_err() {
        let _ = join.join();
        return result;
    }
    *WORKER.lock().unwrap() = Some(Worker { stop, join });
    eprintln!("theia-player: Linux GTK software renderer ready");
    Ok(())
}

pub fn frames() -> u64 {
    FRAMES.load(Ordering::Relaxed)
}

pub fn detach() {
    if let Some(worker) = WORKER.lock().unwrap().take() {
        worker.stop.store(true, Ordering::Relaxed);
        let _ = worker.join.join();
    }
}

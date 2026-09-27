//! The small Rust boundary to the AppKit/OpenGL film surface.
//! All view and render-context work stays on the main thread in render_macos.m.
//! Tauri gives us the window content view; the bridge finds Wry's WKWebView
//! child and places the film surface below it.

use crate::mpv::Engine;
use std::ffi::{c_char, c_void, CStr, CString};
use std::os::unix::ffi::OsStrExt;
use std::path::Path;

extern "C" {
    fn theia_render_attach(
        webview: *mut c_void,
        mpv: *mut c_void,
        library: *const c_char,
        error: *mut c_char,
        capacity: usize,
    ) -> bool;
    fn theia_render_frames() -> u64;
    fn theia_render_detach();
}

pub fn attach(view: isize, engine: &Engine, library: &Path) -> Result<(), String> {
    let path = CString::new(library.as_os_str().as_bytes())
        .map_err(|_| "the media engine path contains a NUL byte".to_string())?;
    let mut error = [0_i8; 256];
    let success = unsafe {
        theia_render_attach(
            view as *mut c_void,
            engine.raw_context(),
            path.as_ptr(),
            error.as_mut_ptr(),
            error.len(),
        )
    };
    if success {
        Ok(())
    } else {
        Err(unsafe { CStr::from_ptr(error.as_ptr()) }
            .to_string_lossy()
            .into_owned())
    }
}

pub fn frames() -> u64 {
    unsafe { theia_render_frames() }
}

pub fn detach() {
    unsafe { theia_render_detach() }
}

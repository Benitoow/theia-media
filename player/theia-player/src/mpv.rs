//! The libmpv binding.
//!
//! The engine is loaded at runtime instead of linked, for three reasons that
//! all come from decision 118: it ships as a separate replaceable DLL, its
//! licence is the LGPL, which expects exactly that arrangement, and a missing
//! or mismatched engine has to become a sentence the player can show rather
//! than a process that dies before its window exists.
//!
//! Every symbol is declared here, on one page, so the unsafe surface stays
//! small enough to read in a sitting. Nothing above this module touches a raw
//! pointer.

use libloading::{Library, Symbol};
use std::ffi::{c_char, c_int, c_void, CStr, CString};
use std::path::{Path, PathBuf};

type MpvCreate = unsafe extern "C" fn() -> *mut c_void;
type MpvInitialize = unsafe extern "C" fn(*mut c_void) -> c_int;
type MpvSetOptionString = unsafe extern "C" fn(*mut c_void, *const c_char, *const c_char) -> c_int;
type MpvSetPropertyString = unsafe extern "C" fn(*mut c_void, *const c_char, *const c_char) -> c_int;
type MpvGetPropertyString = unsafe extern "C" fn(*mut c_void, *const c_char) -> *mut c_char;
type MpvCommand = unsafe extern "C" fn(*mut c_void, *const *const c_char) -> c_int;
type MpvFree = unsafe extern "C" fn(*mut c_void);
type MpvErrorString = unsafe extern "C" fn(c_int) -> *const c_char;
type MpvTerminateDestroy = unsafe extern "C" fn(*mut c_void);
type MpvClientApiVersion = unsafe extern "C" fn() -> u32;

/// The engine, loaded and ready. Dropping it terminates mpv.
pub struct Engine {
    /// Declared first so it outlives every function pointer below it: on
    /// Windows, unloading the library invalidates them.
    _lib: Library,
    ctx: *mut c_void,
    initialize: MpvInitialize,
    set_option: MpvSetOptionString,
    set_property: MpvSetPropertyString,
    get_property: MpvGetPropertyString,
    command: MpvCommand,
    free: MpvFree,
    error_string: MpvErrorString,
    terminate: MpvTerminateDestroy,
}

// The context is mpv's own and is documented as safe to use from several
// threads. Rust cannot know that, so it is asserted here, once, rather than
// wrapping every call site in a lock it does not need.
unsafe impl Send for Engine {}
unsafe impl Sync for Engine {}

/// Where the engine is looked for. `THEIA_LIBMPV` exists for development and
/// for anyone who prefers their own build - which the LGPL requires us to
/// allow anyway.
pub fn engine_path() -> Result<PathBuf, String> {
    if let Ok(explicit) = std::env::var("THEIA_LIBMPV") {
        let path = PathBuf::from(&explicit);
        if path.is_file() {
            return Ok(path);
        }
        return Err(format!(
            "THEIA_LIBMPV points at {explicit}, which is not a file"
        ));
    }

    let exe = std::env::current_exe().map_err(|e| format!("locating the executable: {e}"))?;
    let dir = exe.parent().ok_or("the executable has no parent directory")?;
    let candidate = dir.join(if cfg!(windows) {
        "libmpv-2.dll"
    } else {
        "libmpv.so.2"
    });
    if candidate.is_file() {
        return Ok(candidate);
    }

    Err(format!(
        "no media engine found. Looked for {} next to the executable, and for THEIA_LIBMPV",
        candidate.display()
    ))
}

impl Engine {
    /// Loads the library, creates a context and initialises it. Options are
    /// set through [`Engine::set_option`] *before* this call, which is why the
    /// two steps are separate.
    ///
    /// The option list is applied by the caller; this function deliberately
    /// knows nothing about playback policy.
    pub fn load(dll: &Path) -> Result<Engine, String> {
        let lib =
            unsafe { Library::new(dll) }.map_err(|e| format!("loading {}: {e}", dll.display()))?;

        unsafe {
            let api_version: Symbol<MpvClientApiVersion> = lib
                .get(b"mpv_client_api_version")
                .map_err(|e| format!("this file is not libmpv: {e}"))?;
            let version = api_version();
            let (major, minor) = (version >> 16, version & 0xffff);
            if major != 2 {
                return Err(format!(
                    "libmpv client API {major}.{minor} is not supported; this player needs 2.x"
                ));
            }

            let create: Symbol<MpvCreate> = lib
                .get(b"mpv_create")
                .map_err(|e| format!("mpv_create: {e}"))?;
            let ctx = create();
            if ctx.is_null() {
                return Err("mpv_create returned nothing".into());
            }

            let engine = Engine {
                initialize: *lib
                    .get(b"mpv_initialize")
                    .map_err(|e| format!("mpv_initialize: {e}"))?,
                set_option: *lib
                    .get(b"mpv_set_option_string")
                    .map_err(|e| format!("mpv_set_option_string: {e}"))?,
                set_property: *lib
                    .get(b"mpv_set_property_string")
                    .map_err(|e| format!("mpv_set_property_string: {e}"))?,
                get_property: *lib
                    .get(b"mpv_get_property_string")
                    .map_err(|e| format!("mpv_get_property_string: {e}"))?,
                command: *lib
                    .get(b"mpv_command")
                    .map_err(|e| format!("mpv_command: {e}"))?,
                free: *lib.get(b"mpv_free").map_err(|e| format!("mpv_free: {e}"))?,
                error_string: *lib
                    .get(b"mpv_error_string")
                    .map_err(|e| format!("mpv_error_string: {e}"))?,
                terminate: *lib
                    .get(b"mpv_terminate_destroy")
                    .map_err(|e| format!("mpv_terminate_destroy: {e}"))?,
                _lib: lib,
                ctx,
            };
            Ok(engine)
        }
    }

    /// Applies the options collected by the caller and starts mpv.
    pub fn initialize(&self) -> Result<(), String> {
        let code = unsafe { (self.initialize)(self.ctx) };
        if code < 0 {
            return Err(format!("mpv_initialize failed: {}", self.error(code)));
        }
        Ok(())
    }

    fn error(&self, code: c_int) -> String {
        unsafe {
            let p = (self.error_string)(code);
            if p.is_null() {
                format!("error {code}")
            } else {
                CStr::from_ptr(p).to_string_lossy().into_owned()
            }
        }
    }

    /// Sets an option. Must be called before [`Engine::initialize`]; the error
    /// names the option rather than surfacing later as silence.
    pub fn set_option(&self, name: &str, value: &str) -> Result<(), String> {
        let n = CString::new(name).map_err(|_| format!("option name {name:?} contains a NUL"))?;
        let v = CString::new(value).map_err(|_| format!("value for {name} contains a NUL"))?;
        let code = unsafe { (self.set_option)(self.ctx, n.as_ptr(), v.as_ptr()) };
        if code < 0 {
            return Err(format!("{name}={value} refused: {}", self.error(code)));
        }
        Ok(())
    }

    /// Sets a property. Unlike an option, this is how the player tells a
    /// running mpv something - a resume point, a track choice, a mute.
    pub fn set_property(&self, name: &str, value: &str) -> Result<(), String> {
        let n = CString::new(name).map_err(|_| format!("property name {name:?} contains a NUL"))?;
        let v = CString::new(value).map_err(|_| format!("value for {name} contains a NUL"))?;
        let code = unsafe { (self.set_property)(self.ctx, n.as_ptr(), v.as_ptr()) };
        if code < 0 {
            return Err(format!("{name}={value} refused: {}", self.error(code)));
        }
        Ok(())
    }

    /// Runs an mpv command such as `["loadfile", path]` or `["cycle", "pause"]`.
    pub fn command(&self, args: &[&str]) -> Result<(), String> {
        let owned: Vec<CString> = args
            .iter()
            .map(|a| CString::new(*a).map_err(|_| format!("argument {a:?} contains a NUL")))
            .collect::<Result<_, _>>()?;
        let mut argv: Vec<*const c_char> = owned.iter().map(|c| c.as_ptr()).collect();
        argv.push(std::ptr::null());
        let code = unsafe { (self.command)(self.ctx, argv.as_ptr()) };
        if code < 0 {
            return Err(format!("{args:?} failed: {}", self.error(code)));
        }
        Ok(())
    }

    /// A string property, or `None` when mpv has none to give. Absent and
    /// empty are different answers and are kept different: the OSD must be
    /// able to say "unknown" rather than draw a zero.
    pub fn property(&self, name: &str) -> Option<String> {
        let n = CString::new(name).ok()?;
        let p = unsafe { (self.get_property)(self.ctx, n.as_ptr()) };
        if p.is_null() {
            return None;
        }
        let value = unsafe { CStr::from_ptr(p).to_string_lossy().into_owned() };
        unsafe { (self.free)(p as *mut c_void) };
        Some(value)
    }

    /// The version string of the loaded engine, for diagnostics and for the
    /// about panel. Never invented: absent when mpv will not say.
    pub fn version(&self) -> Option<String> {
        self.property("mpv-version")
    }

    fn shutdown(&mut self) {
        if !self.ctx.is_null() {
            unsafe { (self.terminate)(self.ctx) };
            self.ctx = std::ptr::null_mut();
        }
    }
}

impl Drop for Engine {
    fn drop(&mut self) {
        self.shutdown();
    }
}

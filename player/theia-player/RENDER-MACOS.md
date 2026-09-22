# Drawing the film on macOS

Decision 144: on macOS the player draws the film itself. This file is the
specification for that work - the API surface verified against the pinned
headers and against `docs.rs`, the exact call sequence, and the questions the Mac
has to settle. It is written before the code because the code cannot be compiled
anywhere else: this repository is developed on Windows, and there is no macOS
toolchain here.

## Why, in one paragraph

The pinned macOS engine has **no video output that can present a frame**: its
build disables `vulkan`, `macos-cocoa-cb` and `swift-build`, and `libmpv.2.dylib`'s
own load commands name no AppKit, Metal, QuartzCore or Vulkan framework - only
OpenGL. mpv 0.41 also removed `--wid` on macOS (`video/out/cocoa_common.m`, the
file that read a view out of `WinID`, is gone). So the film can only appear in a
window Theia owns by Theia drawing it: `--vo=libmpv` plus
`mpv_render_context_render` for every frame, over a GL context the player creates.

## The API surface, verified

From the pinned archive's own headers
(`include/mpv/render.h`, `include/mpv/render_gl.h` - checked symbol by symbol):

| Symbol | Signature / shape |
|---|---|
| `mpv_render_context_create` | `(mpv_render_context **res, mpv_handle *mpv, mpv_render_param *params)` |
| `mpv_render_context_set_update_callback` | `(ctx, void (*fn)(void *), void *fn_ctx)` |
| `mpv_render_context_update` | `(ctx) -> uint64_t`; `MPV_RENDER_UPDATE_FRAME = 1 << 0` |
| `mpv_render_context_render` | `(ctx, mpv_render_param *params)` |
| `mpv_render_context_report_swap` | `(ctx)` |
| `mpv_render_context_free` | `(ctx)` |
| `MPV_RENDER_PARAM_API_TYPE` | `char*` = `MPV_RENDER_API_TYPE_OPENGL` (`"opengl"`) |
| `MPV_RENDER_PARAM_OPENGL_INIT_PARAMS` | `mpv_opengl_init_params*` = `{ get_proc_address(ctx, name), ctx }` |
| `MPV_RENDER_PARAM_OPENGL_FBO` | `mpv_opengl_fbo*` = `{ fbo, w, h, internal_format }` |
| `MPV_RENDER_PARAM_FLIP_Y` | `int*`, 1 for a default framebuffer |
| `MPV_RENDER_PARAM_ADVANCED_CONTROL` | `int*`; once set, `update` must be called after each callback or mpv's core thread can block |
| `mpv_render_param` | `{ mpv_render_param_type type; void *data; }`, array terminated by `{0}` |

The threading rules the header states, and they are hard requirements:

- every `mpv_render_*` call must happen with the **same GL context current** on
  the calling thread as `mpv_render_context_create` was given;
- the update callback may be called from mpv's own thread, so it may only set a
  flag - the render itself happens on the thread that owns the context;
- with `ADVANCED_CONTROL`, `mpv_render_context_update` must be called after every
  callback. Not doing so can deadlock mpv's core thread, which the header says in
  as many words.

From `docs.rs` (checked for the current versions, Apple-only crates, `cfg`-gated):

| Item | Where | Note |
|---|---|---|
| `NSOpenGLView` | `objc2_app_kit` | deprecated in favour of `MTKView`, present; needs features `NSOpenGLView`, `NSOpenGL`, `NSView`, `NSResponder` |
| `NSOpenGLView::initWithFrame_pixelFormat` | `objc2_app_kit` | `(Allocated<Self>, NSRect, Option<&NSOpenGLPixelFormat>) -> Option<Retained<Self>>` |
| `NSOpenGLView::openGLContext` / `setOpenGLContext` | `objc2_app_kit` | the context the render API must be given |
| `NSOpenGLView::setWantsBestResolutionOpenGLSurface` | `objc2_app_kit` | true, or the surface is drawn at half resolution on a Retina display |
| `define_class!` | `objc2` | `#[unsafe(super(NSOpenGLView))]`, `#[ivars = ...]`, `#[unsafe(method(drawRect:))]`, and a `Drop` impl becomes `dealloc` |
| `NSView::addSubview_positioned_relativeTo` | `objc2_app_kit` | `NSWindowBelow` relative to the webview's view keeps the OSD above the film |
| `MainThreadMarker` | `objc2` | AppKit objects are main-thread only, and the type says so |
| `NSTimer` | `objc2_foundation` | enough for the render tick; a `CVDisplayLink` is the better clock later, not the first version |

## The sequence to implement

In `player/theia-player/src/render_macos.rs`, `cfg(target_os = "macos")` only:

1. **The view.** `NSOpenGLPixelFormat` with `NSOpenGLProfileVersion3_2Core`,
   double buffered, 24-bit colour, 8-bit alpha. An `NSOpenGLView` sized to the
   window's content view, `setWantsBestResolutionOpenGLSurface(true)`, added
   **below** the webview (`addSubview_positioned_relativeTo(NSWindowBelow, Some(webview_view))`).
2. **The context.** `view.openGLContext()`; `makeCurrentContext()` on the thread
   that will render - the main thread, which is where AppKit wants us anyway.
3. **The mpv render context.** `mpv_render_context_create` with
   `API_TYPE=MPV_RENDER_API_TYPE_OPENGL`, the GL init params (whose
   `get_proc_address` is `dlsym(RTLD_DEFAULT, name)` - verified sufficient by the
   spike, whose symbols are all present in the pinned headers),
   `ADVANCED_CONTROL=1`.
4. **The update callback.** Sets an `AtomicBool`; it must not touch GL.
5. **The render tick** (an `NSTimer` at the display's rate to begin with):
   if the flag is set, `mpv_render_context_update`, and when
   `MPV_RENDER_UPDATE_FRAME` comes back, `setNeedsDisplay` on the view. The
   `drawRect:` implementation, written in Rust with `define_class!`, makes the
   context current, renders with `OPENGL_FBO` (`fbo = 0`, `w`/`h` from the view's
   bounds **times the backing scale factor**) and `FLIP_Y = 1`, calls
   `flushBuffer`, then `mpv_render_context_report_swap`.
6. **Resize.** Nothing special: the FBO size is read from the view every frame, so
   a resize is picked up by the next render. The view must keep
   `autoresizingMask` covering width and height.
7. **Teardown.** `mpv_render_context_free` before the engine is freed, and the
   view removed; the header is explicit that freeing the context from a thread
   where a *different* context is current is not safe.

## What the Mac has to settle, in this order

1. **Does the spike pass?** `player/spike-macos/render-spike.m` answers whether
   `mpv_render_context_create` succeeds with this pin and whether frames arrive.
   If it fails, the pin is wrong and the engine must be built by this project
   (mpv + FFmpeg + libplacebo with Vulkan, or with `cocoa-cb`) - a separate
   decision, and this file's sequence does not change.
2. **Does `hwdec=videotoolbox` decode under the render API?** mpv's render API
   does the hwdec interop itself for `vo=libmpv`, but that has never been run
   here. The check: `--diagnostics` must report `hwdec = videotoolbox` and the
   position must advance on an HEVC file. If it does not, the fallback is
   `hwdec=videotoolbox-copy`, which is slower and honest.
3. **Does the OSD composite above the GL view?** `composite-spike.m` answers the
   mechanism; in the player the check is that the title and the control bar are
   visible over the film, and that a click on the picture reaches the OSD.
4. **Does the window still round its corners?** On Windows `SetWindowRgn` does it;
   on macOS the window is a normal `NSWindow` with the bundle's chrome, and the
   GL surface must not paint outside the rounded region - the Mac's eye decides
   whether the default behaviour is right.

## What this file does not decide

Nothing about the OSD's markup, the tracks popover, subtitles or the status
telemetry: those are platform-independent and already work. Nothing about Windows:
`wid` stays exactly as it is there, and the `gpu-api`/`gpu-context` values are
unchanged. And nothing about what macOS may claim - the verification record does
that, and it only changes when a film has played on the Mac.

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

The bridge compiles these AppKit classes and methods with the macOS SDK:

| Item | Where | Note |
|---|---|---|
| `NSOpenGLView` | AppKit | the film surface, with an OpenGL 3.2 core context |
| `openGLContext` / `makeCurrentContext` | AppKit | the context the render API must use |
| `setWantsBestResolutionOpenGLSurface` | AppKit | Retina pixels rather than logical points |
| `drawRect:` | AppKit subclass | renders a frame and reports its swap to mpv |
| `addSubview:positioned:relativeTo:` | AppKit | `NSWindowBelow` relative to the WebView keeps the OSD above the film |
| `NSTimer` | Foundation | main-thread render tick, including common run-loop modes |

## Integrated candidate: the first Mac answer, and what it changed

`src/render_macos.m` implements the sequence below as an AppKit bridge;
`src/render_macos.rs` passes the borrowed mpv handle and view from the Tauri
application. The build uses headers from the same digest-checked archive as its
libmpv dylib.

**Measured 28 September 2026**, on the `platform-proof` dispatch of commit
`49917a6`: the app built and launched, its window reported 1280x720, playback
advanced from 0.73 s to 3.92 s of the fixture - and the verifier failed the
picture, `the application rendered no advancing frames (1 -> 1)`. Exactly one
frame was rendered and never another. The old order created the render context
*after* `mpv` had been handed the file; `mpv/render.h` says the context "needs to
be created with `mpv_render_context_create()` before you start playback (or
otherwise cause a VO to be created)", and `start_engine` now attaches the
surface before its first `loadfile` for that reason.

A frame count cannot say **where** a chain stopped, so each link now counts
itself and `--diagnostics` prints them as one line:

| Field | What a zero rules out |
|---|---|
| `attached` | the surface exists at all; 0 after a successful launch means the bridge never ran |
| `elapsed-ms` | how long the surface has existed - the denominator for `ticks` |
| `ticks` / `last-tick-ms` | the main-thread timer runs at all, and when it last did |
| `ready` | mpv offered a frame (`MPV_RENDER_UPDATE_FRAME`) |
| `draws` / `last-draw-ms` | AppKit asked the view to draw, and when |
| `in-render` | a `mpv_render_context_render` call is in flight right now (never entering it is the normal state) |
| `errors` | this bridge's own `mpv_render_context_render` returned a failure |
| `frames` | a frame was rendered, flushed and reported swapped |
| `notifications` | mpv's own update callback fired (monotonic, so it must rise) |
| `on-screen` | the window was visible and not fully occluded at the last tick |
| `surface` | the backing size the last draw asked for, `WxH`, `0x0` before one |

**Measured 28 September 2026**, on the dispatch of commit `2cb6ca7` (the
`macos-intel` job), and this is the line it printed:

```
render-diagnostics: attached=1 ticks=12 ready=1 draws=2 errors=0 frames=1
  notifications=20 on-screen=1 surface=1280x720
```

Everything on mpv's side is healthy: the callback fires (20 times), a frame is
offered, the surface is the right size, the window is on screen, and the render
call never *failed*. What stopped is the host: 12 ticks in a run of several
seconds, and a second draw that was entered and never left.

That is the failure the pinned header names. Its *Threading* section says the
thread calling `mpv_render_*` "does not call libmpv API functions other than the
mpv_render_* functions", that "there must be no lock or wait dependency from the
render thread to a thread using other libmpv functions", and:

> If you set `MPV_RENDER_PARAM_ADVANCED_CONTROL`, you promise that this won't
> happen, and must absolutely guarantee it, or a real deadlock will freeze the
> mpv core thread forever.

This bridge breaks exactly that rule: it renders on the application's main
thread, and the main thread is also the one calling `loadfile`, `set_option` and
every status read. So `MPV_RENDER_PARAM_ADVANCED_CONTROL` is gone for now. Its
cost is in the same header - without it the same mistake degrades to a timeout
("playback quality will be degraded") instead of a freeze - and that cost is
accepted only until rendering moves to a thread of its own, which the header
recommends and which this file still owes. `elapsed-ms`, `last-tick-ms`,
`last-draw-ms` and `in-render` are in the line above because that rewrite is the
next thing to verify: they will say whether the main thread is still the thing
that stops, or whether the loop now runs free.

`scripts/verify-macos.sh` echoes the last line into its log and copies the
player's whole raw diagnostics - mpv's messages included - into the proof
artifact, so a failure names its link without downloading anything by hand.

**Measured 28 September 2026, the dispatch after that one**, and this is the
series the artifact now carries:

```
elapsed-ms=2366 ticks=8  ready=1 draws=2 in-render=1 frames=1
elapsed-ms=3492 ticks=8  ready=1 draws=2 in-render=1 frames=1
elapsed-ms=4622 ticks=8  ready=1 draws=2 in-render=1 frames=1
elapsed-ms=5662 ticks=9  ready=2 draws=3 in-render=1 frames=2
elapsed-ms=6821 ticks=10 ready=3 draws=3 in-render=0 frames=3
```

The verifier passes: `PASS the actual application rendered frames: 1 -> 3`,
`29 passed, 0 failed`. Frames reach the screen, and the deadlock is gone - but
look at what the numbers cost. `in-render=1` for three consecutive samples, and
one frame every ~1.2 s: **each `mpv_render_context_render` on that machine takes
about a second**. The runner is a virtual Intel Mac with no GPU, so
`gl-renderer` (now in the line) is the software renderer, and libplacebo's
conversion runs on the CPU. A rate asserted in the verifier would therefore be a
property of the runner written down as a property of the player; it is printed as
`INFO ... fps drawn, ... ticks/s offered` instead.

Two things follow, and they are not the same size. **And the picture arrived**:
dispatch `36476965274` photographed the fixture's colour bars and timecode in the
application's own window - named by `windowNumber` and read with
`screencapture -l` - and on the screen as well. Both pictures are torn, which is
what reading a surface mid-render looks like at half a frame per second on a
software renderer. **The architecture debt stands**: rendering still happens on
the thread that owns the client API, and the rewrite to a thread of its own is
what makes an app on real hardware honest.

Until that rewrite, the frame check here is flaky by construction on this
machine. The dispatch after the one above (`36479591146`) failed the same check on
the same code:

```
attached=1 elapsed-ms=8625 ... draws=2 last-draw-ms=2856 in-render=1
last-render-ms=2857 frames=1 ... gl-renderer="Apple Software Renderer"
```

A draw entered at 2.8 s was still inside `mpv_render_context_render` when that
line was printed at 8.6 s: one render took six seconds where the passing
dispatch's took one, so whether two frames landed inside the verifier's window was
luck. Two changes follow. `last-render-ms` joins the line, so "slow" and "stuck"
are different numbers rather than the same `in-render=1`. And the verifier waits
- bounded, thirty seconds - for the frame count to advance before it photographs
and judges, because "frames advanced" is the check that decides whether the
picture is a picture. Whether the OSD reads well over a moving film remains a
person's question, and `scripts/verify-macos.sh` still says `LOOK` for it.

## Render sequence

In the macOS-only bridge:

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
   `ADVANCED_CONTROL=1`. **Before the first `loadfile`**, not after: the header
   requires it "before you start playback (or otherwise cause a VO to be
   created)", and the run of 28 September showed the cost of the other order -
   one frame, then nothing, on a film whose position kept advancing.
4. **The update callback.** Sets an `AtomicBool`; it must not touch GL.
5. **The render tick** (an `NSTimer` at the display's rate to begin with):
   `mpv_render_context_update` on every tick - `ADVANCED_CONTROL` requires it
   after every callback, and not calling it can block mpv's core thread - and
   when `MPV_RENDER_UPDATE_FRAME` comes back, `setNeedsDisplay` on the view. The
   `drawRect:` implementation in the Objective-C bridge makes the
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

### 30 September: the Intel picture must survive OSD composition

Run 36708087170 advanced render frames and painted controls, but both captures
showed black where the generated film should have been. The automated success
does not qualify Intel for publication. The film view now opts into AppKit layer
backing and passes the context's currently bound draw framebuffer to mpv.
[Apple's layer-backed OpenGL guidance](https://developer.apple.com/library/archive/documentation/GraphicsAnimation/Conceptual/HighResolutionOSX/CapturingScreenContents/CapturingScreenContents.html)
describes the separate context AppKit assigns to this view. This repair still
requires a new Intel and Apple Silicon capture before either changed binary ships.

Nothing about the OSD's markup, the tracks popover, subtitles or the status
telemetry: those are platform-independent and already work. Nothing about Windows:
`wid` stays exactly as it is there, and the `gpu-api`/`gpu-context` values are
unchanged. And nothing about what macOS may claim - the verification record does
that, and it only changes when a film has played on the Mac.

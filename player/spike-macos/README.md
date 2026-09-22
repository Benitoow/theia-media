# The macOS render spike

A throwaway program, and the first thing to run on the Mac. It answers one
question that decides the shape of the whole macOS player:

> With the pinned engine, can Theia draw the film itself?

It exists because the answer turned out not to be the one the port assumed. The
pinned macOS engine (`178meorg/libmpv-macos-build` v0.41.4, profile
`enhanced-lgpl`) is built with `vulkan=disabled`, `macos-cocoa-cb=disabled` and
`swift-build=disabled`, so **none of mpv's own video outputs can present a frame
on macOS** - not into its own window, not into ours. mpv 0.41 also removed the
`--wid` support that would have let us hand it a view (`video/out/cocoa_common.m`
is gone; the macOS backend is Swift now). Its one video path is the **libmpv
render API over OpenGL** (`plain-gl=enabled`), where the host owns the GL context
and calls `mpv_render_context_render` for every frame.

That is the path the Windows player deliberately avoided - it hands Theia a
device, a swapchain and their formats to get wrong - and on macOS there is no
alternative. So it gets a spike before it gets an implementation, exactly as the
Windows player did (spikes A1, A2a, A2b in `docs/v3.3.md`).

## Running it

```sh
# 1. the pinned engine, by digest, exactly as the build script fetches it
go run ./scripts/fetch-libmpv -platform darwin/arm64 -out player/vendor-darwin

# 2. the spike
clang -fobjc-arc -framework Cocoa -framework OpenGL \
  -Iplayer/vendor-darwin/../spike-macos/include \
  player/spike-macos/render-spike.m \
  -Lplayer/vendor-darwin -lmpv -o /tmp/render-spike

# 3. a film you own, and ten seconds
DYLD_LIBRARY_PATH=player/vendor-darwin /tmp/render-spike "/path/to/a/film.mkv" 10
```

`include/mpv/*.h` comes from the same archive as the dylibs: the fetch tool lays
the libraries out flat, so unpack the headers once if the compile cannot find
them (`tar -xzf` the pinned tarball and copy `include/`).

## What to look at

| Observation | What it means |
|---|---|
| `render context created (OpenGL, advanced control)` | the engine has the GL render path this pin was chosen for |
| `frames rendered into our GL view: N`, N > 0 | **the path works**: Theia can draw the film itself |
| `vo=libmpv`, `hwdec=videotoolbox` in the per-second lines | VideoToolbox decode is live, which is the macOS equivalent of the Windows `d3d11va` runs |
| `ao=coreaudio` | the audio path opened (muted; this is not a passthrough test - macOS has no TrueHD/Atmos path at all) |
| the window shows the film | the frames are real, not counted |
| `mpv_render_context_create failed: ...` | **the pin is wrong for macOS.** Then the choice is a self-built engine (mpv + FFmpeg + libplacebo with Vulkan, or with cocoa-cb) instead of a pinned prebuilt, and that is a decision with its own entry |

Two honest limits. The spike does not answer whether a *transparent* Tauri
webview composites above this GL surface - that is the next spike, and it is what
the OSD over the picture depends on. And nothing here is about the real player's
structure: this file exists to be read once and then deleted, or kept as the
record of a measurement.

## The second spike, which is independent of it

```sh
clang -fobjc-arc -framework Cocoa -framework OpenGL -framework WebKit \
  player/spike-macos/composite-spike.m -o /tmp/composite-spike
/tmp/composite-spike
```

`composite-spike.m` asks the other half of the question, and it does not use mpv
at all: a window whose bottom layer is an `NSOpenGLView` drawing a moving colour,
with a **transparent `WKWebView` above it** showing a title, a control bar and a
click counter. Ten seconds later it prints what it observed and says what to look
at:

| Observation | What it means |
|---|---|
| the title and the bar are drawn over the moving colour | a transparent page composites above a GL surface in one window - the OSD over the film is possible |
| the page's background is white or opaque | the configuration is wrong, and the fix belongs in the player's window setup, not in the OSD |
| the click counter increases with coordinates | the page receives presses through its own transparent areas, which is what the player's click-on-the-picture-to-pause needs |
| the GL swaps keep rising | the surface underneath keeps drawing while the page is up |
| `body background: rgba(0, 0, 0, 0)` in the log | the page really is transparent, not merely believed to be |

The two spikes are deliberately independent: one is about mpv drawing into our
surface, the other about a page sitting above it. If the first fails, the second
is still the answer to a question the player will have either way.

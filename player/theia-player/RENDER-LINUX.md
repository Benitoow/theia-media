# Drawing the film on Linux

Decision 149 named two source-level blockers for Linux: its `PLATFORM_OPTIONS`
selected Windows D3D11 and WASAPI, and the window handle it handed to mpv was
zero. This file is the specification and the record for that work - what the
platform needs, what has been measured, and what is still open. It is the Linux
counterpart of `RENDER-MACOS.md`, and the difference between the two is the whole
point: **Linux can embed, macOS cannot**.

## Why the shape is not macOS's

macOS needs a render bridge because its pinned engine has no video output that
can present a frame, and mpv 0.41 removed `--wid` there. Linux has no such
problem: `--wid` is an X11 window id, mpv creates a child window inside it and
draws there, and the transparent OSD floats above it in the same window - exactly
the arrangement Windows uses with an `HWND`.

So the Linux work is a *fix*, not a new architecture:

| Item | Before | Now |
|---|---|---|
| `vo` / `gpu-api` / `gpu-context` | `gpu-next` + `d3d11` + `d3d11` | `gpu-next` where the machine has a DRI device, `x11` where it does not, and the API and context are left to mpv |
| `hwdec` | `d3d11va` | `auto-safe` (VA-API, VDPAU, NVDEC where the driver has them) |
| `ao` | `wasapi` | `pipewire,pulse,alsa` - a preference list, because the answer belongs to the machine |
| audio mode | passthrough asked for, with the WASAPI-only `audio-exclusive` | PCM: only Windows asks, because `audio-exclusive` does not exist here (see below) |
| window handle | `0` | the X11 window id, or zero on Wayland (see below) |

`gpu-api` and `gpu-context` are deliberately absent rather than set to a Linux
value: which pair is right depends on the machine - Vulkan or OpenGL, X11 or
Wayland - and `start_engine` treats a refused option as fatal, so naming one
would be a player that will not open a film on every machine that chose the
other.

The tables are a **mapping** now (`platform_options(platform)`), not three `cfg`
blocks, and that is the fix for how the fault survived: a `cfg` block on Linux is
invisible to every Windows machine and to every test. Three tests pin it -
`the_linux_table_names_no_windows_backend`,
`a_zero_window_id_is_not_sent_as_a_wid` and
`linux_asks_for_the_output_its_machine_can_use` - and all three run on any host.

## Wayland, which is the open half

The id itself is read from **GTK's own window**, not from the portable
`raw_window_handle`: a Tauri window on Linux is a `gtk::ApplicationWindow` and its
portable handle answers `Unavailable` - measured on 27 September 2026, where the
player printed `could not read the window handle: the underlying handle is not
available` and handed mpv a zero. GTK creates the `GdkWindow` on realise, and the
engine starts before the window is shown, so the bridge realises the widget
without mapping it: the id exists and the window stays hidden.

A Wayland surface belongs to the client that created it, so there is no id
another process can be pointed at: `x11_window_id` answers zero there, and
`base_options` leaves `wid` out rather than sending `wid=0` (which is not "no
embedding" to mpv, it is a request for a window of the engine's own). The
consequence is honest and unwelcome: on a Wayland session the film opens in mpv's
own window, beside an OSD that cannot sit over it.

Two ways out, and neither is written:

1. **Run the player on X11** - `GDK_BACKEND=x11`, which on most desktops means
   XWayland and therefore a window id. Cheap, and it makes the X11 path the
   supported one.
2. **A render bridge**, the macOS design (`--vo=libmpv` plus
   `mpv_render_context_render` over a GL/Vulkan surface the player owns). This is
   the only arrangement that composites the OSD over the film on a Wayland-native
   session, and it is a decision rather than a repair - the same one decision 144
   took for macOS.

## The engine

`player/libmpv.json` pins no Linux engine, so there is nothing to bundle and
nothing to verify against. A candidate has been measured - **not adopted** - from
the same kind of provider the other platforms use (a GitHub release of prebuilt
LGPL libraries, self-contained, with a per-architecture archive and a published
digest):

| | |
|---|---|
| provider | `jason-yau/mpv-prebuilt`, release `20260906162952` (6 September 2026) |
| assets | `libmpv-0.41.0-linux-x86_64-lgpl-20260906162952.tar.gz`, `libmpv-0.41.0-linux-arm64-lgpl-...` |
| measured | x86_64 25,346,847 bytes, sha256 `202b6c16e272f47cfc261023b23551d70db34598519978b8621ead33bde9bb17`; arm64 24,393,695 bytes, sha256 `62ed443a6d189548d36d52fbc8832088833ef3c0a93e0c5ecf1cf915da13ea4f` |
| extracted | `<root>/lib/libmpv.so.2.5.0` with the `libmpv.so.2` and `libmpv.so` symlinks, `include/`, `licenses/` (per-dependency texts, `ffmpeg-COPYING.LGPLv2.1` among them), `manifest.txt` |
| library digest | `libmpv.so.2.5.0` (x86_64) `6b3f664efda6821876ae4f2e85ad6a90a85e57299c4dcd1d28ea8741ae80c88e` |
| manifest | `mpv=0.41.0`, `ffmpeg=9.0.1`, `license=LGPL-2.1-or-later`, `linux_bundled_deps=yes` |
| dependencies | `ldd` resolves X11, Wayland, EGL, ALSA, PipeWire, PulseAudio, VA-API and VDPAU **from the archive's own `lib/`**, with nothing missing: the same "libraries beside each other" layout the macOS pin uses |

**Two things the maintainer has to decide, and neither is a detail.** The first
is the provider: adopting it is a decision entry, exactly as decision 118 was for
Windows. The second is the FFmpeg generation: this build carries FFmpeg 9.0.1,
where the server's pinned FFmpeg is 8.1.2 and the macOS pin is 8.1.2 as well. A
player decoding with a newer generation than the server converts with is not a
fault - the two programs do different jobs - but it is a difference the pins
currently describe as deliberate, so it belongs in the record rather than in a
silent commit.

What a Linux engine would also need before it ships: a `fetch-libmpv` branch for
a `.tar.gz` with symlinks and a licence directory, a `linux-amd64`/`linux-arm64`
target in `build-release.ps1`, and a runner run of the real player on both
architectures - which is what `platform-proof.yml`'s Linux job is not today: it
is explicitly a compilation diagnostic.

`scripts/verify-linux.sh` is the instrument that run would use, in the shape of
`scripts/verify-macos.sh`: it runs a player that already exists, asserts that the
engine loaded, that mpv was given a window id and that the position advanced, and
writes a picture for a person to look at. Verified in WSLg on 27 September 2026:
`4 passed, 0 failed, 1 for a person to look at` on an X11 session, and on a
Wayland one it fails on the window id - which is the point of it.

## The video output is chosen by the machine, not by the engine

`gpu-next` is what a Linux desktop should use. It cannot be *asked* for, because
mpv 0.41 aborts rather than refuses when the graphics stack is not there: with EGL
unable to create a screen, its X11 GL path fails and then trips its own assertion
in `vo_x11_init` (`!vo->x11` failed), and the abort takes the whole application
with it. Measured twice in WSLg on 27 September 2026, where `/dev/dri` does not
exist: once through the player, once through `mpv --vo=gpu-next` alone, with
`Suspect software renderer or indirect context` immediately before it.

So the choice is made before the engine starts: a machine with no DRI device has
no EGL and no Vulkan, and gets `vo=x11` - a legacy output with bad performance and
a picture, where the alternative has neither. `linux_video_output` is a pure
function of that one fact and is tested from any host, and `platform_options`
takes the answer as an argument for the same reason it takes the platform.

## Audio: only Windows asks for bitstream

`audio-exclusive` is a WASAPI option. It used to be requested on "everything that
is not macOS", so every Linux film began with a refused option, a fallback and a
reload: measured in WSLg, `passthrough refused by the endpoint, fell back to PCM`
inside the first second, and after the fix eighteen status frames of
`"audioMode":"pcm"` with no refusal at all. mpv does have `audio-spdif` on Linux,
but nothing here has measured it against a receiver, and a player declares nothing
it cannot observe: the option to add is `audio-spdif` alone, on a machine where
the answer can be heard.

## What is measured, and on what

Measured on 27 September 2026 in WSLg (Ubuntu 26.04, mpv 0.41.0 from the distro,
the player built from this tree at `player/target/release`, a generated
`testsrc2` fixture), which is a Linux machine and not a shipping target:

- **The film plays and mpv embeds into the Theia window.** With
  `GDK_BACKEND=x11` the player printed `window id 6291460`, the X11 root window
  held one player window and not two, and a screenshot of that window showed the
  fixture's colour bars with its own burned-in timecode. Playback advanced
  (`"pos"` 0.56 to 12.72 across runs), the engine loaded (`loaded: mpv v0.41.0`),
  and the chosen output was `"vo":"x11"` because this machine has no `/dev/dri`.
- **Wayland is measured too, and behaves as this file says it would.** With
  `GDK_BACKEND=wayland` the player printed `this session has no X11 window id, so
  the film opens in the engine's own window`; the X11 tree then held one window
  (the engine's) and the Tauri window was a Wayland one, absent from it. The film
  played in its own window, and the OSD could not sit over it.
- **Not verified here, and not claimed:** the OSD's *paint* over the film. The
  page loaded and ran - the player's own `osdFrames` counter advanced - but
  WebKitGTK could not composite in this environment (no DRI device; the rest of
  the window stayed unpainted, and `WEBKIT_DISABLE_COMPOSITING_MODE=1` with
  `WEBKIT_DISABLE_DMABUF_RENDERER=1` did not change that). A machine with a
  working graphics stack is needed for that half, and so is a run of the *shipped*
  bundle: this run used the distribution's libmpv through `THEIA_LIBMPV`, not a
  pinned engine.

## What this file does not decide

Nothing about the OSD, the tracks menu, subtitles or the status telemetry: those
are platform-independent. Nothing about Windows or macOS: their tables and their
window handles are unchanged, and the macOS render bridge stays as
`RENDER-MACOS.md` describes it. And nothing about what Linux may claim: a
compile, a cross-build or a WSL run is not a Mac or an ARM machine, and the
verification record only changes when a film has played on the platform itself.

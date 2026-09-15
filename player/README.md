# theia-player

Theia's native player: a Tauri 2 window whose only visible layer is the OSD,
with libmpv drawing the film into a child surface of the same window.

It exists because a browser cannot hand an untouched Dolby TrueHD, DTS-HD MA or
Atmos stream to an amplifier, renders only the HDR10 base layer of a Dolby
Vision profile 7 file, and remuxes every Matroska file before it can play one.
The reasoning, the superseded clauses and the validation boundary are in
[decision 117](../docs/DECISIONS.md) and [`docs/v3.3.md`](../docs/v3.3.md).

## Building

The OSD has to exist before Rust compiles, because `tauri-build` embeds it:

```bash
cd player/ui && npm install && npm run build   # writes player/ui/dist
cd ../.. && cargo build --manifest-path player/Cargo.toml
```

`cargo build` alone fails on a fresh clone with a missing `../ui/dist`, which is
the intended order rather than an accident. `player/ui/dist`, `player/target`
and `player/theia-player/gen` are generated and never committed.

## Looking at the OSD without playing anything

```bash
cd player/ui
npm run build
npm run preview -- --port 5199          # in one shell
npm run check:render -- http://localhost:5199/
```

`check:render` opens the built OSD in Chromium with a faked `window.__TAURI__`,
which is the whole external surface the page has, writes pictures of it to
`player/ui/render-check/`, and asserts what a picture cannot: the number of films
the library panel draws, the rows and ticks in the track menu, that the phone
control row still carries four controls, and that no state overflows
horizontally at 390px. It borrows Playwright from the web application rather than
installing a second copy. It has earned its place twice already - it found the
design tokens undefined in the OSD bundle and the phone control row painting a
third of itself off-screen, and neither is visible by reading the code.

## Running

```bash
./player/target/debug/theia-player --media /path/to/a/film
```

Options that exist today:

| Flag | Effect |
|---|---|
| `--media <path>` | Loads one local file, for development. |
| `--server <url>` | Connects to a server at startup. |
| `--play <id>` | Starts that film. Needs `--server`. |
| `--list` | Prints the library as JSON and exits. Needs `--server`; starts no engine and no window. The whole library, not a page: it is the same call the OSD's grid makes. |
| `--limit <n>` | With `--list`, asks for one page of that size instead. |
| `--discover` | Browses the network for servers, prints what answered, and exits. |
| `--mute` | Starts muted. Every automated run of this program uses it: a test that plays a tone on somebody's machine while they are working is a test that gets the project turned off. |
| `--audio <id>` / `--sub <id>` | Chooses a track shortly after the file opens, through the same command the OSD's menu calls. `--sub 0` turns subtitles off. |
| `--diagnostics` | Prints the session state once a second as JSON, and the track list whenever it changes. |

The engine is looked for in this order:

1. `THEIA_LIBMPV`, an explicit path, which also exists so anyone may substitute
   their own build - which the LGPL requires us to allow;
2. `libmpv-2.dll` (Windows) or `libmpv.so.2` next to the executable.

A missing engine is a sentence in the interface, not a crash: the process starts,
the window appears and says what is wrong.

The engine is not in this repository yet. Decision 118 pins the LGPL build and
its SHA-256; vendoring it is a packaging step with a licence file and a source
pointer, and it belongs to the release work rather than to the source tree.

## What is verified, and where

Windows 11, AMD Radeon 890M, mpv `v0.41.0-1049-g0b7ed670f`:

- the engine loads through FFI and reports its version;
- HEVC Main 10 in Matroska decodes with `d3d11va` and renders through
  `vo=gpu-next` on a D3D11 context, flip model, 10-bit swapchain;
- the OSD's WebView sits above mpv's surface with no z-order forcing;
- the film advances to the end and stops there (`keep-open`);
- the player connects to a real server, lists its library, resolves a film to
  its primary file, and plays it over HTTP with mpv doing the range requests;
- a part-watched film resumes where it was left, and **the audio fallback keeps
  that position** instead of sending the viewer back to the beginning;
- progress is written back to the same table the browser player writes to, so a
  film started in one is resumable in the other;
- the status the OSD reads is built by the serialiser and parses strictly. The
  hand-built version emitted mpv's own `no` for a flag, which is not JSON, and
  the OSD's `try`/`catch` would have swallowed every frame in silence;
- the OSD renders as designed in a browser, at 1280 and at 390: the library as a
  card grid with section 6.1's three artwork fallbacks, the part-watched rule
  drawn once and never on a finished film, every row of the track menu, both
  numbers in the clock, and nothing hanging outside the frame in any of the four
  phone states measured;
- the engine owns a visible surface from startup, with no film loaded, so the
  transparent OSD is never composed over the desktop;
- the whole library is read, not its first page: against a bench of 250 films,
  `--list` returns all 250 in 0.7 s and 107 KB, and both artwork URLs for a
  TMDB-matched film answer `200 image/jpeg` from the server's own cache;
- a `.srt` beside a film reaches the viewer even though mpv is given an HTTP URL
  and cannot look for one itself: the player asks the server what sits beside the
  film, hands each text sidecar to mpv as WebVTT with its language, chooses it
  when the film offers no subtitle of its own - and it survives the audio
  fallback's reload, which no longer takes the subtitles away with the sound.

One cosmetic difference and one open question, both small and both recorded
rather than hidden: a sidecar served as WebVTT is reported by mpv as `webvtt` in
the menu's detail line, where the file on disk is a `.srt`; and serving the
sidecar untouched would need a server route the API does not have. That belongs
to the server's own phase.

Not verified anywhere yet: macOS, Linux, television browsers, bitstream
passthrough reaching an actual amplifier, and the OSD drawn by the real WebView2
window over moving picture rather than by Chromium over a still frame. That last
one is the maintainer's look, and it is the only result that could overturn the
`wid` composition choice.

## The audio path, and the failure it recovers from

A test on a machine whose endpoint refuses the raw stream found something worse
than silence: mpv never established an audio output, so it never advanced the
film at all. `time-pos` stayed at zero.

The player therefore treats passthrough as a **request**. It asks for
`audio-spdif`, watches for a loaded film that is not paused and still has no
audio output after a grace period, withdraws the request, re-establishes PCM,
reloads the film and tells the OSD *why* - as a code
(`endpoint-refused-bitstream`), never as a sentence. The OSD owns the sentence,
which is decision 25 applied to a second interface.

## Layout

| Path | Contents |
|---|---|
| `theia-player/src/mpv.rs` | The whole FFI surface, on one page. Nothing above it touches a raw pointer. |
| `theia-player/src/main.rs` | Window, session, IPC commands, the audio watchdog. |
| `ui/src/App.svelte` | The OSD: transport, clock, notices, keyboard. |
| `ui/src/lib/catalogues.js` | French and English sentences. The Rust side sends codes. |
| `ui/src/components/FilmCard.svelte` | One film as the card grid draws it, with section 6.1's artwork fallbacks. |
| `ui/src/osd.css` | Only what a player adds to the design system. |
| `ui/scripts/render-check.mjs` | Renders the OSD in a real browser and asserts its layout. |

The OSD imports `web/src/lib/tokens.css` and
`web/src/lib/components/Icon.svelte` directly. Two palettes is how two
identities start, and this product has one.

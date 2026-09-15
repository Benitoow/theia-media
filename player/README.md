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

## Shipping the engine with it

```bash
./build-player.ps1 -Release -Bundle
```

A player without an engine plays nothing, so the bundle is what gets distributed:
`theia-player.exe`, `libmpv-2.dll`, `LICENSE-libmpv.txt` and `NOTICE.md`, zipped
into `dist/theia-player-windows-amd64.zip` (about 43 MB). The engine is fetched by
`scripts/fetch-libmpv`, which reads `player/libmpv.json` and **checks two
digests** - the archive's before extracting, the library's after - and refuses to
hand anything on if either disagrees:

```bash
go run ./scripts/fetch-libmpv -print-pin      # what is pinned, and where from
go run ./scripts/fetch-libmpv -out player/vendor
```

The upstream project keeps thirty days of builds, so `THEIA_LIBMPV_MIRROR` points
the fetch at a mirror first; the digest is what decides, not the URL. Only
`windows/amd64` is pinned, because Windows is the only platform the player has
been run and verified on.

### The licence, which is not optional

Decision 118 accepts the LGPL's obligations in exchange for redistributing the
engine, and `NOTICE.md` records how each one is kept: the licence text travels in
the bundle, the library stays a separate replaceable file, the exact source and
digests are named by `theia-player --diagnostics`, and Theia's own source is
public. A bundle missing any of those is a licence breach rather than an
incomplete download, which is why the release pipeline checks for all four files
before publishing anything.

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

A fourth block was added on 15 September 2026, and it asserts four more things
that the first three passed straight through:

- **the faces the OSD actually wears.** The chain, not one link of it: a
  `@font-face` rule declares the family with a real `src`, a FontFace of that
  family reached status `loaded`, and the element that should wear it resolves to
  it. `document.fonts.check()` is deliberately **not** the check — measured with
  no `@font-face` in the document at all, it answered `true` for `Cinzel
  Variable`, which is how a release can ship in Georgia and look fine;
- **every target clears 44×44** (design system section 9). The timeline is
  included rather than exempted: section 6b says 24px and section 9 says 44px,
  and the contradiction was settled in favour of 44px around the 4px line;
- **typing an address is typing.** The OSD listens for keys on the window, so
  every shortcut it owns is also live in a text field. The check types an address
  made of the letters the OSD has claimed and reads the field back;
- **one press is one command**, counted rather than eyeballed, because a
  double-handled click is invisible in a picture and obvious in a film that
  toggles twice.

It also asserts the three-second hide on both sides of the boundary (visible at
2.5 s, hidden by 3.5 s with the pointer over the picture), that the furniture
never hides while the film is paused, while the engine is not ready, while a menu
the viewer opened is on screen, while nothing is loaded at all, or with focus on
a control, that a key or a pointer move brings it back in under half a second,
and that the pointer follows the furniture rather than being hidden by the
stylesheet for good. The track menu owns the arrows while it is open and gives
focus back to its button when Escape closes it, and with nothing loaded the
control bar is not drawn at all - so the language lives in the header for exactly
those states, and returns to the bar when a film starts. Finding servers is
asserted for the three answers a network can give: none lists nothing and says so,
one connects by itself, and several are listed with their names and addresses.

What the discovery assertions cannot prove is mDNS itself: this machine's
responder refuses an IPv6 multicast bind and answers nothing, so what is checked
is the OSD's behaviour given an answer, not that an answer arrives. A real server
on a real network is the other half, and it is not claimed.

**This block is deliberately red while the defects it was written for are still
open.** It was added in phase 1 and left failing on purpose: a check that is
written green against the code it is meant to catch proves nothing. The failures
it reports today are the phase-0 findings, and each one goes green in the unit
that fixes it - the pointer with the cursor work, the faces and the keyboard with
the OSD pass, the timeline with the target rule. A run is read by counting
failures, not by looking for the word "passed".

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
| `PROTOCOL.md` | The numbered gestures the player is accepted against, and which of them a machine can prove. |

The OSD imports `web/src/lib/tokens.css` and
`web/src/lib/components/Icon.svelte` directly. Two palettes is how two
identities start, and this product has one.

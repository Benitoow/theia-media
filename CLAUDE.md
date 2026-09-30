# Working on Theia

Theia is a personal media server: no configuration, no cloud account, no paywall
for the server, library or ordinary viewing. Optional future social/supporter
features follow decision 161; the existing GPL licence is unchanged. One user,
their own films, their own machine.

**V3.4 completes the native generation introduced in V3.3.** `v3.2.0` was
the last single-binary release; `v3.3.1` through `v3.3.6` refined the native
installation and playback paths. The product has three principal artifacts:
`theia-server` (Go, headless, still serving the Svelte administration and settings
interface; the legacy web viewer remains available without active viewing support,
while administration and settings remain maintained under decision 159), `theia-player` (Tauri 2 + Rust + libmpv - the native player,
and where films are now meant to be watched) and `theia-setup` (Go + Charm, the
installer and maintenance tool). A fourth program travels with them: `theia`,
the command somebody types - the installer puts it and its siblings on the
user's PATH - which starts the server if it is not answering and then opens the
player (decision 139). The reason is a platform ceiling: a browser
cannot pass TrueHD/Atmos or DTS-HD MA to an amplifier, does not carry Dolby
Vision profile 7, and does not read Matroska natively.

Read decision 117 and `docs/spec-fondatrice.md` §14 before touching anything.
3.4 is the final planned V3 release; no 3.4.1, 3.5 or 3.6 is planned. The next
release line is V4, documented in `docs/v4.md` (decision 160). Desktop apps and
the server remain maintained. Phone and TV apps are deferred without a date.
They record exactly which founding clauses were superseded and which still bind.
The published V3.3.6 has complete Windows x64 and Apple Silicon artifacts, but a
post-release audit found no integrated Mac render context (decision 149): old
playback-position checks did not establish a picture. The V3.4 branch adds the
AppKit/OpenGL renderer with a CPU fallback and six complete package candidates.
V3.4.0 was published on 30 September 2026 from 82da1af, after complete
six-platform dry run 36759560332 and successful tag run 36760923851.
All installed native products passed captured generated-film and painted-control
checks. Both Ubuntu packages also passed pause, resize, fullscreen, restoration
and resumed playback. All twelve public assets were downloaded and matched
GitHub SHA-256 digests; all six setup payloads matched their updater servers.
Linux uses a GTK-owned software render canvas: SDR and PCM, without hardware
acceleration or HDR. Physical Mac hardware decoding/HDR remains unverified.
Keep the
current evidence and publication blockers in `docs/v3.4-release-readiness.md`;
do not promote a platform because compilation or playback counters passed.

**Publication is explicit.** Preparing a release does not authorize publishing
it. Never push, create or push a tag, create a release,
or upload an asset without the maintainer's explicit instruction for that exact
action. `release.yml` fires on a pushed `v*` tag, so a tag pushed "just to see
CI" publishes binaries. `scripts/stub-release` remains the local release-shaped
test path. When a task does not explicitly authorize publication, report what
was verified locally instead. **A dispatch measures the pipeline without
publishing** (decision 140): `gh workflow run release.yml --ref main` runs the
gates and builds artifacts, and `publish` refuses anything that is not a pushed
tag. Captured generated-film colour and visible-controls checks now gate native
packages. A failed dispatch is evidence of that gate, not a release.

## Read these first, every session

Three documents govern every change. Read them before proposing or writing
anything; they answer most questions that would otherwise be asked again.

| Document | What it settles |
|---|---|
| [`docs/spec-fondatrice.md`](docs/spec-fondatrice.md) | What Theia is and what it refuses to be. The scope of v1, and the technical prohibitions. Start here. |
| [`docs/DECISIONS.md`](docs/DECISIONS.md) | Every decision already taken, with its reasoning and, where it matters, the bug that forced it. Read the index first: every entry carries `**Status:**`, `**Today:**` where a decision replaced it, and `**Topics:**`; `node scripts/decisions.mjs --list --topic <topic>` answers without reading the file (decision 141). |
| [`docs/design-system.md`](docs/design-system.md) | Colour, type, spacing, motion, focus. §6 - *the card grid is exempt* - is the single most important constraint in the interface. |

**V3 shipped in `v3.0.0`.** Its verified product and playback boundaries are in
[`docs/v3.md`](docs/v3.md). V3.2 is the engine baseline; its release summary is
[`docs/releases/v3.2.0.md`](docs/releases/v3.2.0.md) and the detailed campaigns
are indexed in [`docs/archive/v3.2/`](docs/archive/v3.2/README.md).

**V3.3 introduced the native generation.** Its scope, validation boundary and
verification record live in [`docs/v3.3.md`](docs/v3.3.md) - update that record
in the same commit as the work, and state what was measured rather than what
was intended. Field testing still follows decision 97 for library-facing
features: they wait for evidence from roughly ten real household libraries,
while security, data-loss, playback, regression and compatibility fixes remain
admissible. Decision 117 reopens the playback path only.

Finished coordination notes and measurement campaigns live in
[`docs/archive/`](docs/archive/README.md). They retain useful reasoning but do
not describe the current product. Read the code, the V3 record and the three
governing documents above instead.

If a change contradicts one of them, the document is changed first, in the same
commit, with the reasoning written down. In `DECISIONS.md` the numbers are
identities: nothing is renumbered or deleted, an entry whose rule was replaced
keeps its text and says so in its `**Status:**` line, and one whose rule still
binds is kept current in place. `node scripts/decisions.mjs` checks all of it
(decision 141), and `git config core.hooksPath .githooks` runs that guard on
every commit that touches the record.

## Standing constraints

From the founding spec, §3, as amended by §14 for V3.3. These are not
preferences:

- **No CGO, ever.** `modernc.org/sqlite`, not `mattn/go-sqlite3`. This governs
  the Go code; `theia-player` is a separate Rust artifact and is not an excuse
  to link C into the server.
- **No runtime dependency beyond ffmpeg** for `theia-server`, which downloads
  it itself, pinned and checksum-verified. **One written exception:** the native
  player adds libmpv, with the same discipline - pinned source, SHA-256, checked
  licence. Nothing else gets in without a decision entry.
- **Docker is never required.**
- **No telemetry, no cloud account.** The only internet calls initiated by
  Theia are to TMDB and GitHub Releases. Remote access passively accepts
  encrypted WireGuard UDP from explicitly configured peers; it never contacts a
  control plane, relay, STUN service or endpoint-discovery service.
- **No unverified image and no unverified binary.** This repository is public
  and GPL-3.0. Never fetch decorative imagery from the web; the maintainer
  supplies licence-checked assets. A screen that needs filling gets CSS texture
  and a note. The same rule governs libmpv and FFmpeg builds, including the
  licences of what they link.
- **The platform webview is the one named exception** to that rule. Tauri draws
  the OSD in WebView2 on Windows, WKWebView on macOS and WebKitGTK on Linux;
  Theia neither ships nor pins it. On Windows it is a Microsoft-serviced
  runtime; on Linux it is a package the user must already have, and the
  installer says so instead of failing obscurely. Decision 162 extends the
  Linux exception to Ubuntu's codec, audio and driver libraries: `libmpv2`,
  WebKitGTK and the desktop libraries are declared OS prerequisites, never
  silently installed or redistributed by Theia.
- **The player declares nothing it cannot observe.** Codec support in a file is
  never presented as proof that the current display, HDMI chain or receiver can
  reproduce it.

## Language

Code, comments, commit messages and internal error strings are **English**, for
contributors. The user interface ships in **French and English**, with
**English as the base** (decision 137): it is what answers a browser that has
never chosen, and what an unknown language code means. The language an
installation was set up in is asked by `theia-setup`, written to the
configuration and handed to both interfaces by the server; a choice made in a
browser wins over it, and nothing overwrites that choice. The founding spec
stays French on purpose - English is the base of the product, not of the
maintainer's own documents. User-facing copy and locale-specific formatters live
in `web/src/lib/i18n/locales/fr.js` and `web/src/lib/i18n/locales/en.js`. A new
language is a new catalogue, not a hunt through Svelte markup.

**The server never writes what the user reads** (decision 25). The API sends
codes - a scan problem is `{kind, path}`, an update failure carries a `reason`,
a home row carries a `kind` - and the interface owns every sentence. This rule
exists because the settings page once showed somebody a Windows syscall name
wrapped in English in the middle of a French page.

The selected interface language belongs to the browser and is stored in
`localStorage`; it is not a server setting.
Language changes are live: visible copy, accessible names, document `lang`,
dates, numbers, durations and file sizes must all follow the active catalogue
without a reload. `web/scripts/check-locales.mjs` guards catalogue parity during
the frontend build.

TMDB metadata is a separate data concern. Existing titles, synopses, genres and
credits were fetched and cached as `fr-FR`; switching the interface does not
translate them and must not trigger a TMDB re-fetch.

## Building

Go lives at `C:\Users\starx\go-toolchain\go` and is not on `PATH`.

```bash
./build.ps1
```

Use the script. **Never run `npm run build` from `web/` on its own** unless you
know why: the frontend build wipes `web-dist/`, and `web-dist/.gitkeep` is
tracked. There is a `postbuild` hook that restores it, but the script is the
tested path. Deleting that file has turned CI red before.

```bash
go test ./...                       # the whole suite
node scripts/contrast.mjs           # guards the documented colour ratios
node scripts/decisions.mjs          # checks the decision record; --write regenerates its index
node web/scripts/check-locales.mjs  # guards French/English catalogue parity
cd web && npm run check             # checks JavaScript and Svelte markup
cd web && npm test                  # drives a real browser at 375, 1280 and 1920
cd web && npm run test:playback      # generated playable films and episodes
```

The browser suites need a built binary and start it against throwaway
directories. The playback suite also needs Go and downloads the pinned FFmpeg
once; THEIA_TEST_FFMPEG can point to an already downloaded copy. The layout suite
asserts the four things a screenshot cannot: nothing overflows,
every declared font actually loaded, every target clears 44px, and the page has
one left edge. Decision 82 lists the faults that earned it.

To judge anything at library scale, fill a bench rather than hunting for the
real library:

```bash
go run ./scripts/bench -data <a throwaway data dir> -count 250
```

## Building the installer

```bash
go build -trimpath -o theia-setup.exe ./cmd/theia-setup
```

`theia-setup` is the third artifact: Go and Charm, no CGO, and it crosses the
same six targets with `CGO_ENABLED=0` (checked by CI, and locally with
`GOOS`/`GOARCH`). It writes the server's configuration through
`internal/config` rather than by hand, its record of the machine's declared role
in `setup.json` inside the data directory, and - beside the programs it installs -
`installation.json`, the one fact the programs there cannot work out for
themselves: which data directory this installation was made with. `theia`,
`theia-setup` and `theia-server` all read it, ahead of the standard location and
behind `THEIA_DATA_DIR` (decision 153).

**It installs the programs.** `%LOCALAPPDATA%\Programs\Theia` on Windows, per-user
because elevation is never requested (decision 120). It finds what it needs
**in the installation, beside itself, or on `PATH`, under either the short names
(`theia-server.exe`) or the published ones (`theia-server-windows-amd64.exe`)** -
both are real, the first is what a working tree builds and the second is what a
release publishes, and looking for only the first is how a folder containing every
published file reported the server as missing. What is not there is **fetched from
GitHub Releases, digest first**: a release that advertises no SHA-256, a download
that disagrees with it, or a file whose size is not what was announced is refused
and deleted. `internal/release` owns the asset names and that rule, because the
updater asks the same questions.

It then writes Start Menu entries (one folder, named Theia) and one on the Desktop,
registers the installation in the **per-user applications list** so Windows, a
launcher and *Settings → Apps* can all see and remove it, and copies itself into
the installation as the maintenance tool the registered uninstall command points
at. See decisions 122 and 123.

**The names are commands.** The installer puts `%LOCALAPPDATA%\Programs\Theia` on
the user's PATH - `HKCU\Environment`, removed again by `--uninstall`, and every
other entry written back exactly as it was - and registers `theia.exe` under App
Paths, which is what the Run dialog and several launchers read. The `Theia` entry
starts `cmd/theia` rather than the player, so it cannot land on a search that
cannot succeed. From a terminal: `theia` starts the server if it is not answering
and then opens the player, `theia server` and `theia player` start one half,
`theia-server` and `theia-player` run their own in the console, and
`theia -version` prints the build. See decision 139.

```bash
go test ./internal/setup/ -v        # roles, plan validation, the form, the entries
./theia-setup.exe                   # the form
./theia-setup.exe --check --lang en # what this machine is, changing nothing
./theia-setup.exe --check-update    # what the server's release page says
./theia-setup.exe --update          # the server, through its own updater
./theia-setup.exe --check-player    # what the player's release page says
./theia-setup.exe --update-player   # the player's bundle; refuses while it runs
./theia-setup.exe --role player --install-dir <dir> --data-dir <dir> --yes
./theia-setup.exe --role all-in-one --from <folder|zip> --force --yes
./theia-setup.exe --uninstall       # programs and entries out, data kept
```

`--force` reinstalls the programs even when they are already there. It used to be
the only way to refresh the player - the server updates itself through the updater
and the player had no path of its own - and since decision 143 the player has one:
`theia-setup --check-player` asks GitHub Releases what is published, and
`--update-player` verifies the bundle's digest, proves the player inside it runs
and names itself, then swaps the four files with a rollback if any step fails.
`--force` remains what it was for: an installation that is wrong on disk.
`--uninstall` keeps `%APPDATA%\Theia` - that is somebody's library and watch
history - and prints where it is.

The terminal form is checked by driving the real Huh model with key messages,
which is the only way to test a TUI without a terminal - and it is how the
confirmation was caught throwing its own answer away. The autostart entry, the
shortcuts and the applications-list entry are tested against a **redirected
`APPDATA`, temporary directories and a registry key the test owns**, so the tests
never touch the Start Menu, the Desktop, the folder Windows reads at logon, or the
machine's real entry. See decision 120 for the Windows mechanisms and why
elevation is never requested.

Two tools exist for looking at the real thing, and they earned their place:

```bash
go run ./scripts/stub-release -dir <folder> -rate 8   # a release page, locally
./scripts/capture-tui.ps1 -Probe -Keys @('{ENTER}')   # a screenshot and the colours drawn
./scripts/capture-window.ps1 -Exe <program.exe>       # a window, DPI-aware
```

`stub-release` serves a folder as a release, with each file's real SHA-256, so the
download path can be driven end to end before anything is published - point
`THEIA_UPDATE_API` at it. `capture-tui.ps1` photographs the form in a real console
and reads the console buffer back, which is the only way to tell "the theme asked
for no colour" from "the terminal refused it". It removes `NO_COLOR` before
launching, because a capture shell that has it produces a monochrome picture of a
colourful product, which is exactly the false alarm it was written to settle.

`capture-window.ps1` is the same idea for a program with a window, and it declares
per-monitor DPI awareness before loading anything that touches one: on this machine
(200% scaling) a DPI-unaware capture asking for an 1100-pixel window gets one twice
that size, so the picture is clipped by the screen edge and looks like a layout
fault that is not there. It prints the DPI it found, and takes the foreground by
attaching to the thread that holds it, because `SetForegroundWindow` is otherwise
refused and the picture shows whatever was in front.

**Stop the OSD preview before rebuilding the player.** `npm run preview` holds
`rollup.win32-x64-msvc.node` and `esbuild.exe` open, and `npm install` then fails
with EPERM while cleaning up.

## Building the release archive

```bash
./build-release.ps1 -Version 3.3.6     # -> dist/theia-3.3.6-windows-amd64.zip
```

The ZIP built here is now an **internal setup payload**: it contains the setup,
server, player, engine, LGPL licence and `START-HERE.txt`. The release job appends
it to the public setup executable so that one download installs the platform
product without another network fetch. The ZIP is not a separate public asset.

The currently published v3.3.6 still has the older fourteen-asset layout.
Decisions 150 and 162 change the next release to six complete setups and six
exact server binary names for installed updaters. Each setup still requires
native picture and installation proof before publication. The twelve-name
`scripts/check-release-assets.ps1` enforces this candidate surface. Upgrading
from V3.3 requires running the new setup to replace the desktop programs; a
server-only automatic update does not update the previous player.

## Building the native player

`theia-player` is a second toolchain, and the order matters: `tauri-build`
embeds the OSD at compile time, so the frontend has to exist before Rust
compiles.

```bash
./build-player.ps1              # or -Release
```

Underneath it does:

```bash
cd player/ui && npm install && npm run build   # writes player/ui/dist
cargo build --manifest-path player/Cargo.toml
```

`player/target`, `player/ui/dist` and `player/theia-player/gen` are generated and
ignored. A missing `player/ui/dist` is the intended build order, not an accident.

**And the order alone is not enough: a change to the OSD alone reaches no binary
until the crate is recompiled.** `tauri::generate_context!` reads `player/ui/dist`
at compile time, and cargo watches this crate's sources, not that directory - so
editing only `player/ui/` built a player that still carried the interface before
the change. On 20 September 2026 that shipped a preview two revisions old to the
maintainer's screen and cost an afternoon. `player/theia-player/build.rs` declares
`cargo:rerun-if-changed=../ui/dist`, and `build-player.ps1` refuses to finish
unless the executable carries the hashed asset names from the dist it just built.
See [`player/README.md`](player/README.md) and decision 118.

What ships is the **bundle**, not the executable:

```bash
./build-player.ps1 -Release -Bundle     # -> dist/theia-player-windows-amd64.zip
```

It adds the engine (`libmpv-2.dll`), the LGPL text and the notice naming the
pinned build. `scripts/fetch-libmpv` downloads the archive from
`player/libmpv.json` and checks **two** digests - the archive before extracting,
the library after - refusing anything that disagrees. All six targets have pins.
Windows and macOS use LGPL engines with their licence notices; Ubuntu 24.04
uses its unmodified GPL-compatible libmpv package and OS-managed dependencies
(decision 162). The pipeline verifies engine digests, licence payloads and
native installation/playback. A bundle missing its licence is a breach rather
than an incomplete download.

The OSD is a web page whose only external dependency is `window.__TAURI__`, so it
can be looked at without launching the player or playing anything:

```bash
cd player/ui && npm run build && npm run preview -- --port 5199
npm run check:render            # in another shell; needs the preview above
```

It writes pictures to `player/ui/render-check/` and asserts what a picture does
not settle: how many films the library panel drew, the rows and ticks in the
track menu, the controls that survive a 390px window, and that no state
overflows. Run it after touching `player/ui/`; it has already found two faults
that reading the code did not - the design tokens undefined in the OSD bundle,
and the phone control row painting a third of itself off-screen.

## Verifying

The standard on this project is **report what you verified, not what you
assumed.** A milestone is not done because the code looks right; it is done when
it has been run against the real library of 274 films and the result observed.

Two traps already paid for:

- **A screenshot does not verify a web font.** A refused face renders as the
  fallback behind it, which is a perfectly good font, so the page looks entirely
  correct. `document.fonts.check()` is the only thing that answers; the guard
  asserts it. This cost a shipped release - decision 79.
- The in-app preview pane does **not** composite frames. `requestAnimationFrame`
  never fires there, so no CSS animation, transition or smooth scroll advances,
  and computed styles for a `position: fixed` subtree can be stale. Anything
  involving motion or hover has to be checked in a real browser, or reported
  honestly as unverified.
- Port **8383** is the maintainer's own release binary. Test on **8395**.

If something could not be verified, say so plainly rather than burying it in an
optimistic summary.

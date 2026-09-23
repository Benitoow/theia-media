<p align="center">
  <img src="assets/theia-logo.png" width="520" alt="Theia">
</p>

<h1 align="center">Theia</h1>

<p align="center">
  <strong>Your films and series. Your network. No account.</strong>
</p>

<p align="center">
  <a href="https://github.com/Benitoow/theia-media/releases/latest"><img alt="Latest release" src="https://img.shields.io/github/v/release/Benitoow/theia-media?style=flat-square&color=C8A24A"></a>
  <a href="https://github.com/Benitoow/theia-media/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/Benitoow/theia-media/ci.yml?branch=main&style=flat-square&label=CI"></a>
  <a href="LICENSE"><img alt="GPL-3.0" src="https://img.shields.io/github/license/Benitoow/theia-media?style=flat-square"></a>
  <img alt="Windows x64" src="https://img.shields.io/badge/Windows%20x64-555?style=flat-square">
  <img alt="macOS Apple Silicon" src="https://img.shields.io/badge/macOS%20Apple%20Silicon-555?style=flat-square">
</p>

<p align="center">
  <a href="https://github.com/Benitoow/theia-media/releases/download/v3.3.4/theia-setup-windows-amd64.exe">Download for Windows x64</a> ·
  <a href="https://github.com/Benitoow/theia-media/releases/download/v3.3.4/theia-setup-darwin-arm64">Download for macOS Apple Silicon</a> ·
  <a href="#three-minute-setup">Setup</a> ·
  <a href="#theia-plex-jellyfin-or-emby">Compare</a> ·
  <a href="https://discord.gg/p4Rp4zHdHf">Discord</a>
</p>

![The Theia home screen: the navigation, the film showing tonight and the rows below it](docs/screenshots/home.webp)

Theia turns folders of films and series into a private cinema for the browsers
already on your television, phone and computer. Run one installer, add your
folders and watch. There is no Theia account, subscription, Docker stack,
external database or separate web app to install.

| One file | Built for choosing | Private by default |
| --- | --- | --- |
| The Go server, SQLite database driver and Svelte interface ship in one binary - the V3.3 player is the part that moves out. FFmpeg is downloaded only when a file needs conversion. | Resume, profiles, watchlists, duration filters, one search across films and series, and a nightly pick. | No telemetry or cloud library. Metadata comes from TMDB; updates come from GitHub Releases. |

## V3.3: playback leaves the browser

> [!IMPORTANT]
> **`v3.3.4` is the current download.** The product is now three programs:
> `theia-server` (the headless Go backend you already run, which keeps serving
> the web interface for administration and fallback playback), `theia-player`
> (a native desktop player built on Tauri and libmpv) and `theia-setup` (the
> installer, and the one file you download). A fourth command, `theia`, is what
> you type: it starts the server if it is not answering and then opens the
> player, and the installer puts it and its siblings on your `PATH`. `v3.2.0`
> was the last release of the single-binary line, and an installed v3.2 updates
> into this one by itself. **`v3.3.4` is the first release that also installs on
> macOS Apple Silicon**: the player there draws the film itself, because the
> pinned engine presents no window of its own on that platform (decision 144).

The reason is not novelty. A browser cannot hand an untouched Dolby TrueHD,
DTS-HD MA or Atmos stream to an amplifier, renders only the HDR10 base layer of
a Dolby Vision profile 7 file, and remuxes every Matroska file before it can
play it. A native player can. The server stays as small as it is: no GPU driver,
no graphical dependency, the same SQLite, the same library.

The reasoning, the superseded clauses and the honest validation boundary are
written down in [decision 117](docs/DECISIONS.md) and
[the V3.3 record](docs/v3.3.md). Windows is the platform verified on real
hardware this project owns. macOS Apple Silicon was verified on a GitHub-hosted
`macos-14` runner - real Apple Silicon, with the film on screen and photographed
there - and what that runner could not answer (hardware decoding, a real audio
endpoint, Gatekeeper's first refusal of an unsigned download, and the interface
over a moving film) is named in the release notes rather than implied. Linux and
Windows on ARM have the server and no player.

## Which program goes where

Theia is three programs, and which ones belong on a machine depends on the
house, not on taste.

| Your setup | What to install | Why |
| --- | --- | --- |
| One computer that holds the films and is plugged into the screen | **All-in-one** - the default answer in the installer | It serves and it plays. Nothing travels over the network, so nothing is limited by it. |
| A small machine in a cupboard or a NAS, and a television, a laptop or a desktop you watch on | **Server only** on that machine, then **player only** on each device you watch on | The server indexes, stores and streams; the player uses the sound and picture hardware of the machine in front of you, which is where the difference is heard. |
| A computer that only watches, with the films held elsewhere | **Player only** | No library is scanned or stored locally. It asks the server for the catalogue and the files. |
| Anything else - a phone, a tablet, a television browser, a machine you have not decided about | **Nothing.** Open the address the server prints in any browser | The web interface is the complete administration surface and a working fallback player. It is simply not where the best sound and picture live. |

Two things the table cannot say. **The player runs on Windows x64 and, since
3.3.4, on macOS Apple Silicon** - Windows on ARM and Linux have the server but
no player, and the project does not ship what it has not seen work. **The browser
is not a second-class citizen**: it plays everything it can decode, it holds the
settings, and it is how you check what the server is doing.

## Project phase: field testing

> [!IMPORTANT]
> **The field test asked for roughly ten real households and did not get them.**
> The pivot to a native player was taken on platform limits and the maintainer's
> own decision instead, which is written down in
> [decision 117](docs/DECISIONS.md) rather than dressed up as evidence. The
> `3.3.x` line is for the faults found since: interface, wording, rough edges,
> and whatever the first real users report.

This phase is about replacing guesses with evidence. Theia needs people who will
run it against their own film and series libraries for at least a week, on the
screens they actually use, and report both failures and uneventful success.
[Join the field test](https://github.com/Benitoow/theia-media/issues/new?template=field_test.yml),
read the short [testing guide](docs/field-testing.md), or talk to other testers
in the [Discord server](https://discord.gg/p4Rp4zHdHf).

Feature ideas are still welcome and will be collected. Library-facing
development resumes when those real libraries have shown which problems deserve
to shape the next release. Shipping features into an evidence vacuum is just
expensive fan-fiction.

## What you get

- **A library that stays current.** Theia scans several folders, watches for new
  files, groups alternate versions under one title and keeps files where they
  already live.
- **Films and series with useful records.** Titles, posters, backdrops, cast,
  crew, runtime, certification, seasons and episodes are cached locally from
  TMDB. A wrong match can be replaced from the film or series page.
- **A cinema that remembers people.** Local profiles keep separate progress,
  watched state and watchlists. Profiles are household identities, not accounts;
  they have no passwords.
- **A player that adapts.** Theia direct-plays compatible media, remuxes when it
  can and transcodes when it must. Audio, text subtitles, quality, resume,
  seeking and preview frames stay in the player.
- **Access outside the house without a Theia cloud.** The built-in remote mode
  uses device-keyed WireGuard and exposes viewer capabilities only. There is no
  relay, rendezvous server or control plane.
- **Updates with a way back.** Theia verifies release digests, waits for playback
  to stop, swaps the executable atomically and keeps the previous version for
  rollback. The player's bundle is updated by the installer the same way:
  `theia-setup --check-player` asks, `--update-player` verifies the published
  digest, runs the new player before replacing anything, and refuses while the
  player is open.

Theia deliberately has no live TV, DVR, music library, plugins or multi-user
permissions. A native desktop player arrived in V3.3 - Windows x64 and, since
3.3.4, macOS Apple Silicon; a
television and a mobile application are the next generation, not a promise. If those matter, the comparison below saves
you an installation you would later resent.

## Three-minute setup

1. Download **[Theia 3.3.4 for Windows x64](https://github.com/Benitoow/theia-media/releases/download/v3.3.4/theia-setup-windows-amd64.exe)**
   - or, on a Mac with Apple Silicon, **[Theia 3.3.4 for macOS](https://github.com/Benitoow/theia-media/releases/download/v3.3.4/theia-setup-darwin-arm64)**
   - and run it. That one installer asks which
   language Theia should speak, what this machine is for, where to keep its data,
   the port it listens on, the name it answers to on the network, and which
   folders hold your films - then shows the whole plan before writing anything.
2. It fetches what this machine needs - the server, the native player, and the
   media engine the player uses - checking the SHA-256 digest GitHub publishes for
   each file and refusing anything that does not match. On Windows the programs
   are copied into `%LOCALAPPDATA%\Programs\Theia`, that folder is added to your
   `PATH`, entries appear in the Start Menu under **Theia** and on the Desktop,
   and Theia is registered as an installed application: it can be launched by
   name from the Start Menu or a launcher such as Flow Launcher, and removed from
   **Settings → Apps** like anything else. On a Mac the same installer is per-user
   too: the programs go to `~/.local/lib/theia`, `Theia.app` is linked into
   `~/Applications` so Finder, Spotlight and Launchpad see it, and `theia` is
   linked into `~/.local/bin` - with the one `PATH` line printed rather than
   written into your shell profile.
3. Start the server from that entry, or let the installer start it automatically:
   it offers an autostart entry - a launchd agent on macOS - and asks for no
   administrator rights to put one in place. From a terminal, `theia` starts the
   server if it is not answering and then opens the player; `theia server` and
   `theia player` start one half alone, and `theia-server` / `theia-player` run
   them in the console.
4. Open **Settings**, add or confirm your media folders, then start the scan.

To remove it later, `theia-setup --uninstall` takes away the programs, the entries
and the autostart record - the launchd agent included - and **keeps your data**:
the library, the progress marks and the configuration stay in `%APPDATA%\Theia` on
Windows and `~/Library/Application Support/Theia` on macOS, and the command says
where they are. Nothing asks for administrator rights in either direction.

| Platform | The download |
| --- | --- |
| Windows x64 | [`theia-setup-windows-amd64.exe`](https://github.com/Benitoow/theia-media/releases/download/v3.3.4/theia-setup-windows-amd64.exe) (one file) |
| macOS Apple Silicon | [`theia-setup-darwin-arm64`](https://github.com/Benitoow/theia-media/releases/download/v3.3.4/theia-setup-darwin-arm64) (one file) |
| Windows on ARM, Intel Macs, Linux | not yet - see *what is verified* below |

If you would rather install with nothing downloaded, use the
[`theia-3.3.4-windows-amd64.zip`](https://github.com/Benitoow/theia-media/releases/download/v3.3.4/theia-3.3.4-windows-amd64.zip)
or the
[`theia-3.3.4-darwin-arm64.zip`](https://github.com/Benitoow/theia-media/releases/download/v3.3.4/theia-3.3.4-darwin-arm64.zip)
offline bundle. Each holds every file for its platform. Unpack it and run
`theia-setup.exe`, or `theia-setup` on a Mac, from inside that folder: it finds
the programs **beside itself**, copies them, and needs no network at all.

The individual pieces are published as separate assets too, for a script, a
mirror, or the updater:

| Asset | What it is |
| --- | --- |
| `theia-server-<os>-<arch>[.exe]` | The server alone. This is what the updater selects by name, and what the installer fetches. |
| `theia-setup-windows-amd64.exe`, `theia-setup-darwin-arm64` | The installers, and the only executables a person downloads. Each fetches the programs above, or copies them from beside itself or from `--from <folder\|zip>`. |
| `theia-player-<os>-<arch>.zip` | The player and its engine, with the engine's licence and notice. |
| `theia-launcher-<os>-<arch>[.exe]` | The `theia` command - installed as `theia.exe` on Windows and as `theia` in `~/.local/bin` on macOS: it starts the server if it is not answering and then opens the player. |
| `theia-<version>-<os>-<arch>.zip` | Everything, for an install with no network. |

The release page labels the installers and the offline bundles as the four human
downloads - two per platform. Everything named `server`, `player` or `launcher`
is installer/updater plumbing, published separately because an existing
installation selects it by exact name.

Release binaries are unsigned and run in the foreground. Windows may show a
reputation warning. The macOS build is ad-hoc signed - there is no Apple Developer
certificate behind it - so the first launch is refused once, and the archive's
`START-HERE.txt` says exactly how to allow it.

> [!WARNING]
> The LAN service has no login. Anyone who can reach TCP `8383` can browse,
> stream and administer Theia. Keep it on a trusted network and **never forward
> port 8383 to the internet**. Use Theia's remote-access screen instead. The
> [security policy](.github/SECURITY.md) explains the boundary.

## Hardware guide

The expensive part is video conversion. Direct play mostly reads a file and
writes it to the network, so buying a server before checking what your clients
decode is an excellent way to purchase an idle CPU.

| | Minimum for direct play | Recommended when conversion is likely |
| --- | --- | --- |
| CPU | Any supported 64-bit `amd64` or `arm64` processor | 4 recent CPU cores, or a supported hardware H.264 encoder |
| Free RAM | 512 MB | 2 GB |
| App disk, excluding media | 250 MB | 1 GB for a larger artwork cache and working room |
| Network | Faster than the media file's bitrate | Gigabit Ethernet for high-bitrate 4K remuxes |
| Viewer | A current browser that decodes the media codec | A device with hardware decode for the codecs in your library |

The V3.2 Windows candidate measured **27.3 MB of resident memory** before a
100-cycle stream endurance run and **32.9 MB** at its observed end and peak. A
separate 10,000-film catalogue benchmark ended at **44.0 MB** of working set;
the workloads are not interchangeable. The binary is **17.7 MB** and the
installed FFmpeg runtime is about **87.9 MB**. Hardware encoders and decoders
are tested on the host before Theia chooses one. Software conversion still
needs enough CPU to remain above real time.

Windows x64 is the real-device validation platform today: it is the only place a
player has played the maintainer's own library on hardware he owns. Release CI
builds all six x64/ARM64 targets and executes the Linux x64 binary, but the ARM64
server binaries have not yet had the same physical-device playback pass, and macOS
Apple Silicon was verified on a GitHub-hosted `macos-14` runner rather than on a
Mac somebody owns - the film was on screen there and photographed, and what that
runner could not answer (hardware decoding, a real audio endpoint, Gatekeeper's
first refusal, the interface over a moving film) is named in the release notes.
That gap is stated here because an architecture badge is not a benchmark.

For perspective, Plex currently recommends at least a Core i3 and typically 4 GB
RAM, while Jellyfin's current general recommendation is 8 GB and a modern media
engine for transcoding. Their workloads and feature sets are larger, so those
numbers are context rather than a benchmark against Theia. See the official
[Plex requirements](https://support.plex.tv/articles/200375666-plex-media-server-requirements/)
and [Jellyfin hardware guide](https://jellyfin.org/docs/general/administration/hardware-selection/).

## Theia, Plex, Jellyfin or Emby?

Choose the product whose compromises match your home. A wall of green ticks
would be advertising wearing a Markdown costume.

| | **Theia** | **Plex** | **Jellyfin** | **Emby** |
| --- | --- | --- | --- | --- |
| Best fit | One household that wants the smallest possible film-and-series server | The broadest polished client ecosystem | A full open-source, multi-user media platform | A configurable commercial media platform |
| Cost | Free, GPL-3.0 | Local personal video is free; remote video and hardware transcoding use paid passes | Free, GPL-2.0, no premium tier | Free tier; several server and app features use Premiere |
| Identity | No account; passwordless local profiles | Plex account model | Local users and permissions | Local users; optional Emby Connect |
| Server setup | One native binary for the server, no Docker or external runtime; the V3.3 player is a second native application | Installers and NAS packages | Native packages, containers and NAS options | Installers, containers and many NAS options |
| Clients | A native desktop player, and a responsive browser for administration and fallback | Browser plus wide TV, mobile, desktop and console coverage | Browser plus official and community apps | Browser plus TV and mobile apps |
| Hardware transcoding | Included; host capabilities are probed | Plex Pass | Included | Generally Premiere, with documented device exceptions |
| Remote model | Embedded WireGuard, device keys, viewer-only routes | Account-based remote streaming; a paid pass is required for personal video away from home | You configure networking or a proxy | Manual access or Emby Connect |
| Live TV, music, plugins | No | Yes | Yes | Yes |

The competitor rows were checked against their official documentation in
September 2026: [Plex plans](https://www.plex.tv/plans/) and
[hardware transcoding](https://support.plex.tv/articles/115002178853-using-hardware-accelerated-streaming/),
[Jellyfin installation](https://jellyfin.org/docs/general/installation/) and
[source repository](https://github.com/jellyfin/jellyfin), and
[Emby installation](https://emby.media/support/articles/Installation.html) and
[Premiere feature matrix](https://emby.media/support/articles/Premiere-Feature-Matrix.html).
Those products change; follow the links before spending money or rebuilding a
server around one row.

## Privacy, data and limitations

Theia stores configuration, SQLite data and cached artwork under `%APPDATA%\Theia`
on Windows, `~/Library/Application Support/Theia` on macOS and `~/.config/theia`
on Linux. Media stays in the folders you selected.

The only outbound services are TMDB for metadata and GitHub Releases for updates.
There is no telemetry endpoint, analytics SDK, cloud account or frontend CDN.
Remote access accepts encrypted WireGuard traffic but contacts no Theia-operated
service.

Image subtitle formats such as PGS and VobSub cannot be drawn without burning
them into the video; Theia names them instead of pretending they work. The LAN
site uses HTTP. Remote traffic is encrypted by WireGuard.

## Build and contribute

The build uses Go `1.26.6` and Node.js `22`:

```bash
git clone https://github.com/Benitoow/theia-media.git
cd theia-media
# Windows: .\build.ps1
# macOS or Linux: make build
```

Run `go test ./...` for the server and `npm test` under `web/` for the real-browser
interface guard. The guard covers phone, desktop and television widths, font
loading, minimum targets and horizontal overflow.

Read [CONTRIBUTING.md](.github/CONTRIBUTING.md) before changing the project. The
[documentation index](docs/README.md) separates current rules, release notes and
technical archives. The [founding spec](docs/spec-fondatrice.md),
[decision record](docs/DECISIONS.md) and [design system](docs/design-system.md)
are the source of truth. Theia is a single-maintainer project built with
disclosed AI assistance; every change is still reviewed and verified on the
running product.

## Licence and attribution

Theia is free software under the [GNU General Public License v3.0](LICENSE).

This product uses the [TMDB API](https://www.themoviedb.org/) but is not endorsed
or certified by TMDB. FFmpeg is downloaded from its pinned upstream release and
remains under its own licence. The embedded Cinzel and Jost typefaces use the SIL
Open Font License 1.1.

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
  <a href="LICENSE"><img alt="Source-available: PolyForm Noncommercial from 4.0, GPL-3.0 up to 3.4" src="https://img.shields.io/badge/licence-PolyForm%20NC%20(4.0%2B)%20%C2%B7%20GPL--3.0%20(%E2%89%A43.4)-555?style=flat-square"></a>
  <img alt="Windows x64 and ARM64" src="https://img.shields.io/badge/Windows-x64%20%2F%20ARM64-555?style=flat-square">
  <img alt="macOS Intel and Apple Silicon" src="https://img.shields.io/badge/macOS-Intel%20%2F%20Apple%20Silicon-555?style=flat-square">
  <img alt="Ubuntu 24.04 x64 and ARM64" src="https://img.shields.io/badge/Ubuntu%2024.04-x64%20%2F%20ARM64-555?style=flat-square">
</p>

<p align="center">
  <a href="#downloads">Desktop downloads</a> ·
  <a href="https://github.com/Benitoow/theia-media/releases/tag/v3.4.0">3.4 release notes</a> ·
  <a href="#three-minute-setup">Setup</a> ·
  <a href="#theia-plex-jellyfin-or-emby">Compare</a> ·
  <a href="https://discord.gg/p4Rp4zHdHf">Discord</a>
</p>

![The existing web library: navigation, tonight's film and library rows](docs/screenshots/home.webp)

*The existing web library view. Desktop playback uses the native application.*

Theia turns folders of films and series into a private cinema. Desktop apps
handle playback, and a small server looks after your library. Phones and TVs
can still use the existing browser viewer. There is no cloud account,
telemetry, Docker stack or external database to install.

| One download | Built for choosing | Private by default |
| --- | --- | --- |
| One setup includes the server, launcher, native player and media engine. Choose all-in-one, server-only or player-only. FFmpeg is downloaded only when the server needs conversion. | Resume, profiles, watchlists, duration filters, one search across films and series, and a nightly pick. | No telemetry or cloud library. Metadata comes from TMDB; updates come from GitHub Releases. |

## Theia 3.4: V3 is complete

> [!IMPORTANT]
> **`v3.4.0` closes V3.** Native film and series details, darker previews,
> episode progress, next-episode playback, remembered server/audio/window
> settings and a redesigned cast rail are together in this release.
> Desktop applications and the server receive active support. The web viewer
> remains available for phones and TVs without ongoing viewing development;
> web server settings and administration remain maintained.
>
> The download is a complete setup containing the server, launcher, native
> player and media engine. It offers server-only, player-only and all-in-one.
> Run the new setup to upgrade from 3.3; an automatic server update alone does
> not replace the old player and installer. [Read the release notes](docs/releases/v3.4.0.md).

The native player reads Matroska and renders subtitles through libmpv. Sound
and picture capabilities depend on the operating system and the actual device:
Windows can use supported bitstream outputs, while macOS and Linux use PCM.
This first Linux release uses software decoding and SDR rendering. The server
remains a separate Go binary with SQLite and no graphical dependency.

The reasoning and validation boundary are recorded in
[decision 117](docs/DECISIONS.md), [the V3.3 record](docs/v3.3.md) and
[the release verification record](docs/v3.4-release-readiness.md). Native
runner tests cover installation, the launcher and a generated film with visible
controls. They do not establish HDR, hardware decoding, real audio endpoints or
downloaded-app reputation checks on every machine.

## Which program goes where

Theia is three programs, and which ones belong on a machine depends on the
house, not on taste.

| Your setup | What to install | Why |
| --- | --- | --- |
| One computer that holds the films and is plugged into the screen | **All-in-one** - the default answer in the installer | It serves and it plays. Nothing travels over the network, so nothing is limited by it. |
| A small machine in a cupboard or a NAS, and a television, a laptop or a desktop you watch on | **Server only** on that machine, then **player only** on each device you watch on | The server indexes, stores and streams; the player uses the sound and picture hardware of the machine in front of you, which is where the difference is heard. |
| A computer that only watches, with the films held elsewhere | **Player only** | No library is scanned or stored locally. It asks the server for the catalogue and the files. |
| A phone, tablet or television browser | **Nothing**; open the address the server prints in a browser | The existing web viewer remains available. Active web viewing support ends with 3.4; server settings and administration remain maintained. |

The desktop downloads target Windows x64/ARM64, macOS Intel/Apple Silicon and
Ubuntu 24.04 x64/ARM64. Linux needs X11 or XWayland and the platform packages
listed below. Other Linux distributions are not verified for these binaries.

## After V3

3.4 is the final planned V3 release. No 3.4.1, 3.5 or 3.6 is planned.
[V4](docs/v4.md) moves toward watch parties, a startup local-profile chooser,
comments and likes/dislikes on films. Direct messages and calls are outside
the scope. Dedicated phone and TV apps are deferred without a date.

Theia's server, library and ordinary viewing remain free. From 4.0 the source
stays public but is *source-available*, not open source: free for personal and
noncommercial use, with no commercial use (see [Licence](#licence-and-attribution)).
Optional shared-viewing extras may one day be paid, without a monthly
subscription; nothing paid exists and nothing is decided. Late 2026 or early
2027 is an estimate, not a commitment.

## Help test 3.4 and shape V4

All six desktop packages passed native installation and generated-film checks
for 3.4. Those tests cannot cover every screen, audio device or household library.
Reports from Intel Macs, Apple Silicon Macs, Windows ARM64 and Ubuntu machines
are especially useful. Try it for a week if you can, and tell us what worked
as well as what failed.
[Join the field test](https://github.com/Benitoow/theia-media/issues/new?template=field_test.yml),
read the short [testing guide](docs/field-testing.md), or talk to other testers
in the [Discord server](https://discord.gg/p4Rp4zHdHf).

Ideas for V4 are welcome too. Watch parties, local-profile selection at startup
and film comments/reactions are the accepted direction; their implementation
still needs specifications and tests. A [GitHub star](https://github.com/Benitoow/theia-media)
or a useful test report helps this small project reach more people.

## What you get

- **A library that stays current.** Theia scans several folders, watches for new
  files, groups alternate versions under one title and keeps files where they
  already live.
- **Films and series with proper detail pages.** Artwork, synopsis, cast and
  available files are shown in the desktop app, with TMDB metadata cached by
  the server. You can inspect resolution and HDR metadata before playing;
  device capability checks describe what the driver reports.
- **A cinema that remembers people.** Local profiles keep separate progress,
  watched state and watchlists. Profiles are household identities, not accounts;
  they have no passwords.
- **Playback that remembers.** The desktop app saves your server address,
  volume, mute state and window size. Audio and subtitle preferences apply to
  films and episodes. Series show resume progress and offer the next episode,
  with optional autoplay and time to cancel.
- **A server that adapts.** Direct play, remux and transcoding serve different
  client capabilities. The native player handles its own decoding and subtitles;
  available output formats depend on the platform and device.
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
permissions. Native desktop apps now ship for all six platforms listed below.
Dedicated phone and TV apps have no announced release date; those devices use
the existing web viewer for now.

## Three-minute setup

1. Download the setup for your computer from the table below. On macOS/Linux,
   make it executable with `chmod +x <filename>`, then run it. The installer
   asks for your language, the role of this machine, data location, port,
   hostname and media folders, and shows its plan before writing.
2. The setup already contains the complete product and installs without
   fetching additional Theia components. Windows installs per user under
   `%LOCALAPPDATA%\Programs\Theia`, adds Start Menu/Desktop entries and
   registers **Settings → Apps** removal. macOS installs under
   `~/.local/lib/theia`, links the app into `~/Applications` and the command
   into `~/.local/bin`. Linux installs under `~/.local/lib/theia`; start the
   `theia` command there. Services remain opt-in and no elevation is required.

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
where they are. Linux data defaults to `~/.config/theia`. Custom data locations
are kept too. Theia's install and removal do not ask for administrator rights;
installing Ubuntu's prerequisite packages is a separate system operation.
From 3.4, on Windows, `theia-setup --repair-registration` restores a missing
**Settings → Apps** entry without reinstalling or starting the player.

## Downloads

All six downloads are complete setups from [v3.4.0](https://github.com/Benitoow/theia-media/releases/tag/v3.4.0).

| Platform | The download |
| --- | --- |
| Windows x64 | [Windows x64 setup](https://github.com/Benitoow/theia-media/releases/download/v3.4.0/theia-setup-windows-amd64.exe) |
| Windows ARM64 | [Windows ARM64 setup](https://github.com/Benitoow/theia-media/releases/download/v3.4.0/theia-setup-windows-arm64.exe) |
| macOS Apple Silicon | [Apple Silicon setup](https://github.com/Benitoow/theia-media/releases/download/v3.4.0/theia-setup-darwin-arm64) |
| macOS Intel | [Intel Mac setup](https://github.com/Benitoow/theia-media/releases/download/v3.4.0/theia-setup-darwin-amd64) |
| Ubuntu 24.04 x64 | [Linux x64 setup](https://github.com/Benitoow/theia-media/releases/download/v3.4.0/theia-setup-linux-amd64) |
| Ubuntu 24.04 ARM64 | [Linux ARM64 setup](https://github.com/Benitoow/theia-media/releases/download/v3.4.0/theia-setup-linux-arm64) |

Ubuntu desktop prerequisites:

```sh
sudo apt install libmpv2 libwebkit2gtk-4.1-0 libayatana-appindicator3-1
```

Theia ships a pinned libmpv library; Ubuntu supplies its desktop, codec, audio
and driver dependencies. The first Linux release uses software decoding and
rendering in SDR. Hardware acceleration and HDR output are not available yet.

The six standalone `theia-server-<os>-<arch>[.exe]` files remain for existing
server updaters. Player, launcher and engine files are carried inside the six
public setups; the internal payload ZIPs are not additional downloads. Older
installers expecting separate player ZIPs should be replaced by the 3.4 setup.

Windows binaries are unsigned and may show a reputation warning. Mac apps
are ad-hoc signed and not notarised, so macOS may require first-launch approval
in Privacy & Security. The setup payload includes `START-HERE.txt` with platform
instructions.

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
| Viewer | A supported desktop app, or the existing phone/TV web viewer | A device whose tested decoding and output capabilities match your library |

The V3.2 Windows candidate measured **27.3 MB of resident memory** before a
100-cycle stream endurance run and **32.9 MB** at its observed end and peak. A
separate 10,000-film catalogue benchmark ended at **44.0 MB** of working set;
the workloads are not interchangeable. The binary is **17.7 MB** and the
installed FFmpeg runtime is about **87.9 MB**. Hardware encoders and decoders
are tested on the host before Theia chooses one. Software conversion still
needs enough CPU to remain above real time.

Windows x64 is the maintainer's real-device validation platform: it is where the
app has played his own library on hardware he owns. Release CI builds all six
x64/ARM64 targets. Desktop promotion additionally requires an installed-product
test and a captured moving test film with visible playback controls on a native
runner. The publication verification record is in
[`docs/v3.4-release-readiness.md`](docs/v3.4-release-readiness.md).
Hosted runners cannot establish hardware decoding performance, real audio
endpoints, HDR output or a first Gatekeeper launch on your own computer. Those
still need reports from real users.

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
| Cost | Free core, source-available (noncommercial licence from 4.0; GPL-3.0 up to 3.4); optional future extras are not decided | Local personal video is free; remote video and hardware transcoding use paid passes | Free, GPL-2.0, no premium tier | Free tier; several server and app features use Premiere |
| Identity | No account; passwordless local profiles | Plex account model | Local users and permissions | Local users; optional Emby Connect |
| Server setup | One native binary for the server, no Docker or external runtime; the desktop player is a separate native application | Installers and NAS packages | Native packages, containers and NAS options | Installers, containers and many NAS options |
| Clients | A native desktop player, and a responsive browser for administration and server settings | Browser plus wide TV, mobile, desktop and console coverage | Browser plus official and community apps | Browser plus TV and mobile apps |
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

The legacy web viewer cannot draw image subtitle formats such as PGS and
VobSub without burning them into video. The native player uses libmpv to render
them directly. The LAN
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

The project keeps one branch, `main`; dependency updates are reviewed manually.
Read [CONTRIBUTING.md](.github/CONTRIBUTING.md) before changing the project. The
[documentation index](docs/README.md) separates current rules, release notes and
technical archives. The [founding spec](docs/spec-fondatrice.md),
[decision record](docs/DECISIONS.md) and [design system](docs/design-system.md)
are the source of truth. Theia is a single-maintainer project built with
disclosed AI assistance; every change is still reviewed and verified on the
running product.

## Licence and attribution

Theia 3.4.0 and earlier are free software under the [GNU General Public License v3.0](LICENSE-GPL-3.0).
Everything after 3.4.0, including 4.0, is source-available under the
[PolyForm Noncommercial License 1.0.0](LICENSE): you may use, modify and share it
for noncommercial purposes, and commercial use needs a separate licence. It is not
an Open Source Initiative licence (decision 163).

This product uses the [TMDB API](https://www.themoviedb.org/) but is not endorsed
or certified by TMDB. FFmpeg is downloaded from its pinned upstream release and
remains under its own licence. The embedded Cinzel and Jost typefaces use the SIL
Open Font License 1.1.

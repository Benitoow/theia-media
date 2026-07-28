# Theia

A personal media server in a single binary. No configuration, no account, no
paywall. Plug in the machine, open a browser, watch a film.

Navidrome proved a media server could be one Go binary using 50 MB of RAM with
an interface people actually enjoy. Theia does the same thing for video.

> **Status: milestone M7.** The server starts, announces itself on the local
> network, shows a QR code on first launch so another device can reach it,
> scans the directories you point it at, fetches posters, synopses and credits
> from TMDB, presents the lot as a browsable library, plays films, remembers
> where you stopped, and updates itself from GitHub releases. Release
> packaging and subtitles are still to come. See
> [docs/DECISIONS.md](docs/DECISIONS.md) for what is coming and what has been
> deliberately left out, and [docs/design-system.md](docs/design-system.md)
> before touching the interface.

This product uses the TMDB API but is not endorsed or certified by TMDB.

---

## Security

**Theia performs no authentication whatsoever. Never expose it directly to the
internet.**

This is a deliberate v1 scope decision, not an oversight. Theia assumes a single
user on a trusted home network: anyone who can reach the port can browse the
library, play everything in it, and change the settings. There is no login
because there are no accounts.

If you want to reach your library from outside your home, put it behind a VPN
such as Tailscale or WireGuard. Do not forward the port on your router.

---

## Running it

Download the binary for your platform from the releases page and run it:

```bash
./theia
```

That is the whole installation. On first launch Theia creates its data
directory, prints every address it can be reached at, and starts serving:

```
  Theia 0.1.0

  Local     http://localhost:8383
  Network   http://192.168.1.19:8383
  mDNS      http://theia.local:8383

  Data      /home/you/.config/theia
```

Open any of those from any device on the same network.

On first launch Theia shows a QR code. Point a phone at it and you are in;
there is nothing to configure first.

The code encodes the numeric address, never `theia.local`. That name works on
Windows, macOS, iOS and most Linux desktops, but **not on Android**, and plenty
of smart-TV browsers do not resolve it either — so it is offered as a
convenience underneath and never as the only way in. The same panel is on the
settings page for the evening a second device turns up.

If the code leads nowhere, the machine has several network adapters and Theia
picked the wrong one; the screen lists the others.

### Options

| Flag | Default | Meaning |
|---|---|---|
| `-port` | `8383` | TCP port to listen on |
| `-data-dir` | OS-dependent | Where configuration, database and cache live |
| `-verbose` | off | Log every HTTP request |
| `-version` | | Print the version and exit |

Everything except `-data-dir` is also settable in `config.json` inside the data
directory. `THEIA_DATA_DIR` overrides the location of that directory, which is
what you want for a portable install on an external drive.

Point Theia at your films by adding directories to `library_paths`:

```json
{
  "port": 8383,
  "library_paths": ["/media/films", "/mnt/nas/cinema"]
}
```

It scans them at startup and whenever you ask it to. Files it cannot identify
still appear, listed under whatever the filename said — a badly named file is
better shown than silently dropped, and the same goes for one TMDB has never
heard of.

### Metadata

Official releases ship with a TMDB key, so metadata works out of the box. If you
would rather spend your own API quota, put your key in `tmdb_api_key` in
`config.json` and it takes precedence. With no key at all Theia still scans and
lists your library, it simply has no posters, and says so in the interface
rather than leaving you to wonder.

Posters and synopses are cached locally and refreshed every few months on their
own. Renaming a file re-runs its lookup immediately, which is how you correct a
wrong match.

### Playback

Files a browser can already read — H.264 and AAC in an MP4, typically — are sent
untouched, so seeking is instant and nothing is spawned.

Anything else is rewrapped on the fly: the picture is copied, and the sound is
re-encoded to AAC when it is AC3, DTS or TrueHD, which is what stops an ordinary
MKV from playing with a picture and no sound. Re-encoding the *picture* is out
of scope for v1, so a file whose video codec no browser reads is reported as
such rather than quietly consuming a CPU for two hours.

Rewrapping needs ffmpeg, which Theia downloads the first time something actually
requires it — never at launch, and never at all for a library it can play
directly. It comes from a pinned GitHub release, its SHA-256 is checked before
it is ever executed, and it lives in the data directory rather than inside the
Theia binary.

### Updates

Theia checks GitHub for a newer release at startup and every few hours, and says
so in the settings. **Installing is something you ask for**, not something that
happens while you are watching something.

A download is only installed once its SHA-256 matches the digest GitHub
publishes *and* the downloaded binary has been run to confirm it works. If
either check fails, the update is abandoned and the version you had keeps
running, untouched. The previous binary is kept beside the new one until the
next start, so a bad release can be undone by hand.

An update will refuse to install while anything is playing, and say so.

### Resume

Stopping mid-film puts it in the **Continuer à regarder** row, newest first,
with a progress bar on its poster. Reopening offers to resume where you left
off, or to start again from the beginning.

A film watched to the end leaves the row and stays out. Starting it again brings
it back. Opening one for a few seconds and closing it is not remembered at all.

---

## Building from source

You need Go 1.23+ and Node 20+.

```bash
make
```

That builds the frontend into `web-dist/`, embeds it into the binary and leaves
`theia` in the repository root. On Windows without `make`:

```powershell
.\build.ps1
```

### Working on the frontend

Run the Go server and the Vite dev server side by side. Vite proxies `/api` to
the Go process, so you get hot reload without rebuilding Go:

```bash
go run ./cmd/theia     # terminal 1
cd web && npm run dev  # terminal 2
```

A `config.local.json` in the repository root overrides the real configuration at
runtime, which is where your own TMDB key belongs:

```json
{ "tmdb_api_key": "…" }
```

It is git-ignored, it is never written back into the real `config.json`, and
logging the configuration prints the key redacted. Do not commit it, and do not
paste it anywhere this repository can see.

### Layout

```
cmd/theia/           entry point and startup orchestration
internal/api/        HTTP routes, JSON API, static file serving
internal/config/     configuration file and data directory
internal/db/         SQLite: opening, migrations
internal/discovery/  mDNS announcement, LAN address selection
internal/library/    the domain model, filename parsing, scan reconciliation
internal/scanner/    walking directories, finding video files
web/                 SvelteKit source
web-dist/            compiled frontend, embedded via go:embed (generated)
assets/              brand assets, not shipped in the binary
docs/                design system and decision record
```

`embed.go` at the repository root is what pulls `web-dist/` into the binary;
`//go:embed` cannot reach outside its own directory, which is why it lives there
rather than next to the code that uses it.

---

## Non-negotiables

These are enforced in review and in CI:

- **No CGO, ever.** It breaks cross-compilation, which is the whole delivery
  model. The SQLite driver is `modernc.org/sqlite` for this reason.
- **No runtime dependency beyond ffmpeg**, which Theia downloads itself on first
  use. Nothing is ever installed by hand.
- **Docker is never required.** It may be offered as one distribution option
  among others, never as the way in.
- **No telemetry, no cloud account.** The only external services contacted are
  TMDB for metadata and GitHub for updates.

## Language

Code, comments and internal error messages are English. The user interface is
French for v1, kept in `web/src/lib/strings.js` so that adding another language
is a matter of adding a file rather than editing markup.

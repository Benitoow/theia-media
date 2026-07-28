# Theia

A personal media server in a single binary. No configuration, no account, no
paywall. Plug in the machine, open a browser, watch a film.

Navidrome proved a media server could be one Go binary using 50 MB of RAM with
an interface people actually enjoy. Theia does the same thing for video.

Point it at a folder of films and it does the rest: finds them, fetches posters
and synopses, and plays them in a browser on any device in the house.

TV series, subtitles and remote access are deliberately not in this version —
see [docs/DECISIONS.md](docs/DECISIONS.md) for what was left out and why.

This product uses the TMDB API but is not endorsed or certified by TMDB.
Theia is free software under the [GPL-3.0](LICENSE).

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

Download the binary for your platform from the
[releases page](https://github.com/Benitoow/theia-media/releases) and run it.
There is no installer and nothing to configure first.

```bash
./theia
```

On Windows, double-clicking `theia-windows-amd64.exe` is enough. On macOS and
Linux, mark it executable first:

```bash
chmod +x theia-linux-amd64 && ./theia-linux-amd64
```

It prints every address it answers on:

```
  Theia v1.0.0

  Local     http://localhost:8383
  Network   http://192.168.1.19:8383
  mDNS      http://theia.local:8383

  Data      /home/you/.config/theia
```

### First launch

Open the local address and Theia shows a QR code. Point a phone at it and you
are in — that is the whole setup on a second device.

The code encodes the numeric address rather than `theia.local`, because **that
name does not resolve on Android** and plenty of TV browsers ignore it too. It
is offered underneath as a convenience, never as the only way in.

Then tell Theia where your films are, in **Réglages**, and it scans them.

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

`THEIA_DATA_DIR` overrides where the configuration, database and cache live,
which is what you want for a portable install on an external drive.

### Settings

There are three, all on the **Réglages** page, and deliberately nothing else:

| Setting | What it does |
|---|---|
| Watched folders | Where your films are. Add as many as you like |
| Port | Which port to listen on. Takes effect on the next start |
| TMDB key | Optional. Yours instead of the one Theia ships with |

They live in `config.json` in the data directory if you would rather edit a
file. Theia scans the folders at startup and whenever you ask it to. Files it
cannot identify still appear, listed under whatever the filename said — a badly
named file is better shown than silently dropped, and the same goes for one TMDB
has never heard of.

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

Anything the interface shows a user is a code the server sends and the interface
words — never a message the server wrote. That rule exists because an earlier
version put Go error strings straight on screen, and people saw
`GetFileAttributesEx D:\Films: ...` where they should have read "that drive is
not plugged in".

---

## Licence

GPL-3.0. See [LICENSE](LICENSE).

Theia is free software and will stay that way: the licence is what makes a
closed, paid fork impossible, which is the entire point of building an
alternative to Plex.

Metadata comes from [TMDB](https://www.themoviedb.org/). **This product uses the
TMDB API but is not endorsed or certified by TMDB.**

`ffmpeg`, downloaded on demand rather than bundled, is licensed separately by
its own authors and is never modified or redistributed by this project.

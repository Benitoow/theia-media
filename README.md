# Theia

A personal media server in a single binary. No configuration, no account, no
paywall. Plug in the machine, open a browser, watch a film.

Navidrome proved a media server could be one Go binary using 50 MB of RAM with
an interface people actually enjoy. Theia does the same thing for video.

> **Status: milestone M0.** The server starts, announces itself on the local
> network and serves the web interface. It does not scan, catalogue or play
> anything yet. See [docs/DECISIONS.md](docs/DECISIONS.md) for what is coming
> and what has been deliberately left out.

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

`theia.local` works on devices that resolve mDNS hostnames -- Windows, macOS,
iOS and most Linux desktops. Plenty of smart-TV browsers do not, and some
machines already run another mDNS responder that owns the port. When that
happens Theia says so at startup and the numeric address still works; from M6
on, a QR code shown at first launch removes the need to type either.

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

### Layout

```
cmd/theia/       entry point and startup orchestration
internal/api/    HTTP routes, JSON API, static file serving
internal/config/ configuration file and data directory
internal/discovery/  mDNS announcement, LAN address selection
web/             SvelteKit source
web-dist/        compiled frontend, embedded via go:embed (generated)
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

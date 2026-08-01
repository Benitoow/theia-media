# Working on Theia

Theia is a personal media server in a single Go binary: no configuration, no
account, no paywall. One user, their own films, their own machine.

## Read these first, every session

Three documents govern this project. Read them before proposing or writing
anything; they answer most questions that would otherwise be asked again.

| Document | What it settles |
|---|---|
| [`docs/spec-fondatrice.md`](docs/spec-fondatrice.md) | What Theia is and what it refuses to be. The scope of v1, and the technical prohibitions. Start here. |
| [`docs/DECISIONS.md`](docs/DECISIONS.md) | Every decision already taken, with its reasoning and, where it matters, the bug that forced it. Check here before re-opening a question. |
| [`docs/design-system.md`](docs/design-system.md) | Colour, type, spacing, motion, focus. §6 — *the card grid is exempt* — is the single most important constraint in the interface. |

If a change contradicts one of them, the document is changed first, in the same
commit, with the reasoning written down. `DECISIONS.md` is append-only in
spirit: supersede an entry, do not quietly rewrite it.

## Standing constraints

From the founding spec, §3. These are not preferences:

- **No CGO, ever.** `modernc.org/sqlite`, not `mattn/go-sqlite3`.
- **No runtime dependency beyond ffmpeg**, which Theia downloads itself, pinned
  and checksum-verified.
- **Docker is never required.**
- **No telemetry, no cloud account.** The only outbound calls Theia may make are
  to TMDB and to GitHub Releases. Nothing else, ever.
- **No unverified image.** This repository is public and GPL-3.0. Never fetch
  decorative imagery from the web; the maintainer supplies licence-checked
  assets. A screen that needs filling gets CSS texture and a note.

## Language

Code, comments, commit messages and internal error strings are **English**, for
contributors. The user interface ships in **French and English**, with French as
the default. User-facing copy and locale-specific formatters live in
`web/src/lib/i18n/locales/fr.js` and `web/src/lib/i18n/locales/en.js`. A new
language is a new catalogue, not a hunt through Svelte markup.

**The server never writes what the user reads** (decision 25). The API sends
codes — a scan problem is `{kind, path}`, an update failure carries a `reason`,
a home row carries a `kind` — and the interface owns every sentence. This rule
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
node web/scripts/check-locales.mjs  # guards French/English catalogue parity
```

## Verifying

The standard on this project is **report what you verified, not what you
assumed.** A milestone is not done because the code looks right; it is done when
it has been run against the real library of 274 films and the result observed.

Two traps already paid for:

- The in-app preview pane does **not** composite frames. `requestAnimationFrame`
  never fires there, so no CSS animation, transition or smooth scroll advances,
  and computed styles for a `position: fixed` subtree can be stale. Anything
  involving motion or hover has to be checked in a real browser, or reported
  honestly as unverified.
- Port **8383** is the maintainer's own release binary. Test on **8395**.

If something could not be verified, say so plainly rather than burying it in an
optimistic summary.

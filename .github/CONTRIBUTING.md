# Contributing to Theia

Thank you for looking. Theia is a small, opinionated project, and the fastest way
to have a change accepted is to know what it already decided.

## Read these first

Three documents govern every change. They are not background reading; they answer
most questions before they are asked, and a pull request that contradicts one of
them will be asked to change the document first.

| Document | What it settles |
|---|---|
| [`docs/spec-fondatrice.md`](../docs/spec-fondatrice.md) | What Theia is and what it refuses to be. Start here. |
| [`docs/DECISIONS.md`](../docs/DECISIONS.md) | Every decision already taken, with its reasoning and, where it matters, the bug that forced it. |
| [`docs/design-system.md`](../docs/design-system.md) | Colour, type, spacing, motion, focus. §6 — *the card grid is exempt* — is the single most important interface constraint. |

If your change contradicts one of them, that is not automatically wrong. It means
the document changes first, in the same commit, with the reasoning written down.
`DECISIONS.md` is append-only in spirit: supersede an entry, do not quietly
rewrite it.

## Constraints that are not preferences

From §3 of the founding spec:

- **No CGO, ever.** `modernc.org/sqlite`, never `mattn/go-sqlite3`.
- **No runtime dependency beyond FFmpeg**, which Theia downloads itself, pinned
  and checksum-verified.
- **Docker is never required.**
- **No telemetry, no cloud account.** The only outbound calls are to TMDB and
  GitHub Releases. Remote access passively accepts WireGuard UDP from configured
  peers; it never contacts a control plane, relay or STUN service.
- **No unverified image.** This repository is public and GPL-3.0. Do not add
  decorative imagery from the web; every shipped asset needs its licence checked
  first. A screen that needs filling gets CSS texture and a note.

## Language

Code, comments, commit messages and internal error strings are **English**, for
contributors.

The interface ships in **French and English**, with French as the default. Every
user-facing string lives in `web/src/lib/i18n/locales/fr.js` and `en.js`. A new
language is a new catalogue, not a hunt through Svelte markup, and
`web/scripts/check-locales.mjs` fails the build if the two drift apart.

**The server never writes what the user reads.** The API sends codes — a scan
problem is `{kind, path}`, an update failure carries a `reason`, a home row
carries a `kind` — and the interface owns every sentence. This rule exists
because the settings page once showed somebody a Windows syscall name wrapped in
English in the middle of a French page.

## Building and checking

```bash
./build.ps1        # Windows
make build         # macOS and Linux
```

Use the script rather than `npm run build` from `web/`: the frontend build wipes
`web-dist/`, and `web-dist/.gitkeep` is tracked. There is a `postbuild` hook that
restores it, but the script is the tested path.

```bash
go test ./...                       # the whole suite
node scripts/contrast.mjs           # guards the documented colour ratios
node web/scripts/check-locales.mjs  # guards French/English catalogue parity
```

CI runs `go vet`, the test suite and six cross-compiled builds. All of it must be
green.

## The standard for "done"

**Report what you verified, not what you assumed.** A change is not finished
because the code looks right; it is finished when it has been run and the result
observed. Pull requests that say what was tested, and on what, get reviewed
faster than pull requests that say a feature is complete.

If something could not be verified, say so plainly. That is a perfectly
acceptable pull request; a silently optimistic one is not.

## Commit messages

Written as full sentences that say what changed and why, in English. The history
is a record somebody will read in a year, and "fix stuff" costs them an
afternoon. Look at recent commits for the tone.

## Reporting a bug

Open an issue with the template. The three things that make a media-server bug
solvable are the **exact file** involved (container, video codec, audio codec —
`ffmpeg -i` output is ideal), the **browser and device**, and whether it happens
in direct play, remux or re-encode. Without those, most playback reports cannot
be reproduced.

## Security

Do not open a public issue for a security problem. See
[SECURITY.md](SECURITY.md).

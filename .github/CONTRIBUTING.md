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
| [`docs/design-system.md`](../docs/design-system.md) | Colour, type, spacing, motion, focus. §6 - *the card grid is exempt* - is the single most important interface constraint. |

If your change contradicts one of them, that is not automatically wrong. It means
the document changes first, in the same commit, with the reasoning written down.
In `DECISIONS.md`, numbers are identities: nothing is renumbered or deleted, an
entry whose rule was replaced keeps its text, says so in its `**Status:**` line
and states what stands in its `**Today:**` line, and one whose rule still binds
is kept current in place. `node scripts/decisions.mjs` checks the record -
numbering, every citation in the repository, the status, today and topic
grammar, supersession reciprocity - and `--write` regenerates its index
(decision 141). Enable it once per clone with
`git config core.hooksPath .githooks`: the hook runs it on any commit that
touches the record, which matters because a commit carrying `[skip ci]` never
reaches CI.

## Current phase: V3.3, playback leaves the browser

`v3.2.0` is the last release of the single-binary line. V3.3 splits the product
into `theia-server`, `theia-player` and `theia-setup`; the reasoning, the
superseded clauses and the validation boundary are in decision 117 and
`docs/spec-fondatrice.md` §14, and the live record is
[`docs/v3.3.md`](../docs/v3.3.md). Windows is the only platform the work can be
verified on today.

Library-facing features stay paused while Theia is exercised by its first ten
real households (decision 97). During this phase, changes are prioritised when
they fix a security problem, a data-loss risk, blocked playback, a regression or
concrete platform and codec compatibility. Documentation and test coverage that
make a report reproducible are also welcome.

Feature requests remain open and are valuable evidence, but feature pull
requests may be deferred until the field test has produced enough repeated
problems to set the next roadmap. If you are using Theia on a real library, the
[field-testing guide](../docs/field-testing.md) and dedicated issue form are the
most useful place to start.

## Constraints that are not preferences

From §3 of the founding spec, as amended by §14 for V3.3:

- **No CGO, ever.** `modernc.org/sqlite`, never `mattn/go-sqlite3`. This governs
  the Go code; `theia-player` is a separate Rust artifact.
- **No runtime dependency beyond FFmpeg** for `theia-server`, which downloads it
  itself, pinned and checksum-verified. The native player adds **libmpv** under
  the same discipline - pinned source, SHA-256, checked licence - and uses the
  platform webview (WebView2, WKWebView, WebKitGTK), which Theia neither ships
  nor pins. Nothing else gets in without a decision entry.
- **Docker is never required.**
- **No telemetry, no cloud account.** The only outbound calls are to TMDB and
  GitHub Releases. Remote access passively accepts WireGuard UDP from configured
  peers; it never contacts a control plane, relay or STUN service.
- **No unverified image and no unverified binary.** This repository is public and
  GPL-3.0. Do not add decorative imagery from the web; every shipped asset needs
  its licence checked first. A screen that needs filling gets CSS texture and a
  note. The same rule governs libmpv and FFmpeg builds.
- **The player declares nothing it cannot observe.** A codec present in a file is
  never presented as proof that the display, HDMI chain or receiver can reproduce
  it.

## Language

Code, comments, commit messages and internal error strings are **English**, for
contributors.

The interface ships in **French and English**, with **English as the base**
(decision 137). Every user-facing string lives in
`web/src/lib/i18n/locales/fr.js` and `en.js`. A new
language is a new catalogue, not a hunt through Svelte markup, and
`web/scripts/check-locales.mjs` fails the build if the two drift apart.

**The server never writes what the user reads.** The API sends codes - a scan
problem is `{kind, path}`, an update failure carries a `reason`, a home row
carries a `kind` - and the interface owns every sentence. This rule exists
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
node scripts/decisions.mjs          # checks the decision record; --write regenerates its index
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

**Never let the skip marker appear in a message unless the push must genuinely
skip CI.** GitHub scans the whole message - prose, quotations and explanations
included - and skips every workflow for that push, so a sentence *about* the
marker silences CI just as effectively as an intentional one. It is how a
change to the decision record can reach `main` without the guard ever seeing it:
the 21 September push that first carried the guard did exactly that, quoted the
marker while explaining the hook, and ran no workflow at all.

## Reporting a bug

Open an issue with the template. The three things that make a media-server bug
solvable are the **exact file** involved (container, video codec, audio codec -
`ffmpeg -i` output is ideal), the **browser and device**, and whether it happens
in direct play, remux or re-encode. Without those, most playback reports cannot
be reproduced.

If you can use Theia for a week before reporting, use the dedicated field-test
form. A report that says what worked, on which clients and media, is useful even
when nothing broke.

## Security

Do not open a public issue for a security problem. See
[SECURITY.md](SECURITY.md).

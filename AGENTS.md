# Working on Theia

**The guidance lives in [`CLAUDE.md`](CLAUDE.md). Read that file first, in full.**

This file exists because `AGENTS.md` is the name several tools look for, and more
than one assistant has committed to this repository. It is a pointer rather than
a copy: two files with the same content drift, and the first thing to rot would
be the list of documents that are supposed to stop exactly that.

If you only take three things from it:

- Read [`docs/spec-fondatrice.md`](docs/spec-fondatrice.md),
  [`docs/DECISIONS.md`](docs/DECISIONS.md) and
  [`docs/design-system.md`](docs/design-system.md) before proposing anything.
  They answer most questions, and a change that contradicts one of them changes
  the document first, in the same commit, with the reasoning written down.
- The constraints in §3 of the founding spec are not preferences: no CGO, no
  runtime dependency beyond FFmpeg, Docker never required, no telemetry, and no
  image whose licence has not been checked.
- **Report what you verified, not what you assumed.** A change is finished when
  it has been run against the real library and the result observed. "I could not
  verify this" is an acceptable answer; an optimistic summary is not.

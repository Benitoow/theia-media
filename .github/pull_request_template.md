<!--
Thank you for the change. Please read .github/CONTRIBUTING.md if you have not —
in particular the three governing documents and the §3 constraints, which are
what most rejected pull requests run into.
-->

## What this changes

<!-- What it does and why. If it fixes an issue, say "Fixes #123". -->

## What I verified

<!--
The project standard is: report what you verified, not what you assumed.
Say what you ran and what you observed. For playback changes, name the file
(container, video codec, audio codec), the browser and the mode — direct play,
remux or re-encode.

"Could not verify X because Y" is a perfectly acceptable answer here.
An optimistic summary is not.
-->

- [ ] `./build.ps1` (or `make build`) succeeds
- [ ] `go test ./...` passes
- [ ] `node scripts/contrast.mjs` passes (if colours changed)
- [ ] `node web/scripts/check-locales.mjs` passes (if strings changed)

## Checks

- [ ] No CGO. No new runtime dependency beyond FFmpeg. Docker still not required.
- [ ] No new outbound network call other than TMDB or GitHub Releases.
- [ ] Any new user-facing string exists in **both** `fr.js` and `en.js`; the server
      sends a code, not a sentence.
- [ ] No image added without a checked licence.
- [ ] If this contradicts the spec, a decision or the design system, that document
      is updated **in this pull request**, with the reasoning — superseding the old
      entry rather than rewriting it.

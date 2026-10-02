# The public page

One static page: HTML, CSS and a few lines of vanilla JavaScript. No framework,
nothing to `npm install`, no remote subresource. It exists to put the right
setup on the right machine, to be found by a search engine, and to give a pasted
link a clean preview. Decision 164 says why it returned after decision 116
removed the earlier site, what it is and is not, and that **the repository stays
the source of truth**: when this page and the README, the release notes or
`docs/DECISIONS.md` disagree, the page is wrong.

Nothing here is deployed. No workflow is active and no setting was changed.
Publishing follows the rule in `CLAUDE.md`: only on the maintainer's explicit
instruction.

## What is in this folder

| | |
| --- | --- |
| `index.html` | The page. Release facts and the canonical URL are `{{tokens}}`, filled in by the build |
| `assets/css/site.css` | The stylesheet, inlined into the page by the build. Tokens are the design system's (§3, §4) |
| `assets/js/site.js` | The station and the recording's scene list. The page works without it |
| `assets/img`, `assets/video`, `assets/fonts` | The media. **`assets/PROVENANCE.md` says where each comes from and what is not verified** |
| `release.json` | A snapshot of the latest GitHub release: version, date, and the size and SHA-256 of the six setups |
| `site.config.json` | The canonical URL, in one place |
| `robots.txt`, `sitemap.xml` | Templates; the build fills in the URL |
| `build.mjs` | Builds `dist/`. No dependencies |
| `check.mjs` | Structural checks on `dist/`. No dependencies |
| `fetch-release.mjs` | Refreshes `release.json` from the GitHub Releases API |
| `serve.mjs` | A static server for looking at `dist/`. It answers Range requests, so the recording can be scrubbed |
| `github-pages.workflow.yml` | A Pages workflow, **inert** until it is moved to `.github/workflows/` |
| `hero-motion.html` | An earlier experiment, not used by the page and not copied to `dist/` |
| `dist/` | The output. Not tracked |

## Build, check, look

```sh
node site/build.mjs --check     # builds site/dist, then runs check.mjs on it
node site/serve.mjs             # http://127.0.0.1:4180/
```

Port 8383 is the maintainer's own Theia and 8395 is the test port for the
application; the page uses neither.

`check.mjs` verifies what a screenshot cannot: that nothing is fetched from another
site, that every local reference resolves, that there is one `h1` and no skipped
heading level, that every image declares its size, that the share card is the
1200 × 630 it claims, that the video does not autoplay and has captions, that the
JSON-LD parses and carries no rating or review, and that the page keeps the
words decisions 161 and 163 ask for (never "open source" for 4.0, no price, no
paid tier, no "coming soon", no claim of Atmos or DTS-HD MA reaching an amplifier,
the TMDB sentence verbatim, the illustrations labelled). It does not judge copy
against the repository; that is a reading, and the table below is how to do it.

## Changing the canonical URL

The URL is not decided (a GitHub Pages address, or a domain of the maintainer's).
It is the only build-time variable, and it is read in this order:

1. `--site-url https://example.org` on the command line;
2. the `SITE_URL` environment variable;
3. `site.config.json`, currently `https://benitoow.github.io/theia-media`, the
   address the removed site used.

```sh
SITE_URL=https://theia.example node site/build.mjs --check
```

It reaches the canonical link, Open Graph and Twitter tags (the share card's
absolute address included), the JSON-LD, `sitemap.xml` and `robots.txt`. Every
other link is relative, so the page works under any base path, a project page
included. For a domain of your own, set it under Settings > Pages as well and
point DNS at GitHub.

## Refreshing the release facts

```sh
node site/fetch-release.mjs     # writes site/release.json from the GitHub API
node site/build.mjs --check
```

The page shows the version, its date, its licence and the size and SHA-256 of the
six setups, all from `release.json`. The script refuses to write a snapshot in
which one of the six setups, or one digest, is missing; the build refuses to run
without all six. The download links use GitHub's `releases/latest/download/`
contract, so they follow the newest release by themselves, and only the printed
facts go stale: **run the script after every release.** The licence printed beside
the version is computed from it: 3.4 and earlier GPL-3.0, later PolyForm
Noncommercial 1.0.0 (decision 163).

## Deploying on GitHub Pages

Read `github-pages.workflow.yml`; it is commented step by step. In short: push the
licence change first, decide the address, `git mv` the file to
`.github/workflows/pages.yml`, set Settings > Pages > Source to *GitHub Actions*,
run it by hand. It builds with `--check` and publishes `site/dist`. It is manual on
purpose.

## Before the page goes live

- **The licence change of decision 163 must be pushed first.** The page says the
  repository is source-available from 4.0 and links the GPL text at the tag
  `v3.4.0`; on `origin/main` today `LICENSE` is still the GPL text and
  `LICENSE-GPL-3.0` does not exist.
- **Rights in the recording are not verified** (`assets/PROVENANCE.md`): TMDB
  artwork and portraits, a film still used as wallpaper, and a profile photo of
  unknown origin. The clean fix is to record it again with the repository's demo
  media, a neutral wallpaper and an avatar made for the purpose.
- **The published 3.4.0 release notes say "free and open source"** (and
  `docs/v3.4-release-readiness.md` ends the same way); decision 163 replaced that
  wording. The page says *source-available*.
- Issue #10 is titled "Theia 3.3 field test"; the page links to it as asked.
- The TMDB logo is not on the page: decision 10 keeps text alone.

## Where each claim comes from

The page may say less than the repository; it may never say more. When one of these
sources changes, change the page.

| The page says | Source |
| --- | --- |
| Folders of films and series become a private cinema; a small server, a native desktop player; no cloud account, telemetry or Docker | `README.md`, introduction |
| One setup carries the server, launcher, player and engine; all-in-one, server-only or player-only | `README.md` "Three-minute setup"; `docs/releases/v3.4.0.md` |
| Windows x64 is the only platform validated on real hardware, with the maintainer's library | `README.md` "Hardware guide"; `docs/v3.4-release-readiness.md`; `docs/v3.3.md` |
| macOS, Linux and Windows ARM64 passed automated install and test-film checks; hosted Macs use CPU rendering; hardware decoding and HDR unverified on a physical Mac | `docs/v3.4-release-readiness.md` (table and "Limits") |
| Linux: Ubuntu 24.04, X11 or XWayland, software decoding, SDR, PCM, the `apt` line | `README.md` "Downloads"; `docs/releases/v3.4.0.md`; decision 162 |
| No Linux package of 4.x until the GPL engine is replaced by an LGPL one | decision 163; `docs/v4.md` item 5 |
| Bitstream passthrough is requested and falls back to PCM (verified); delivery to a receiver is not, the maintainer has none | `docs/v3.3.md` (audio rows and the finding), `player/README.md` |
| Unsigned Windows setups; ad-hoc signed, not notarised Mac apps | `README.md` "Downloads" |
| No login on the LAN; port 8383 is never forwarded | `README.md` warning; `.github/SECURITY.md` |
| Phones and TVs use the browser viewer, without playback development; no live TV, DVR, music, plugins or multi-user permissions | `README.md`; `docs/releases/v3.4.0.md`; decision 159 |
| Remote access over device-keyed WireGuard, with no relay, rendezvous server or control plane | `README.md` "What you get"; decisions 43 to 46; `CLAUDE.md` |
| The native player reads Matroska, draws PGS and VobSub itself, remembers progress, offers the next episode, keeps audio, subtitle, volume and window choices | `README.md`; `docs/releases/v3.4.0.md` |
| The library is scanned, watched and grouped; artwork and cast come from TMDB and are cached | `README.md` "What you get" |
| The profile editor changes a name and a photo (PNG, JPEG or WebP) in 3.4.0 | `player/ui/src/lib/catalogues.js` and `ProfileDialog.tsx` at tag `v3.4.0`; decision 136 |
| Licence wording; GPL for 3.4.0 and earlier; PolyForm Noncommercial from 4.0; source-available; nothing paid exists | `LICENSE`; decisions 161 and 163; `README.md` "After V3" |
| The field-test invitation and the Discord | `README.md`; `docs/field-testing.md`; issue #10 |
| The TMDB sentence, verbatim | decision 10; `internal/api/settings.go` |
| "One maintainer, built with disclosed AI assistance" | `README.md` "Build and contribute" |
| Cinzel and Jost under the OFL 1.1 | `README.md` "Licence and attribution" |
| Version, date, size and SHA-256 of the setups | GitHub Releases API, through `release.json` |
| The recording shows 3.4.0 on Windows and what its captions say | the recording itself; `assets/PROVENANCE.md` |

## What was measured, and when

On 2 October 2026, against `dist/` served on the loopback address, in the
Chromium that Playwright installs for `web/` and in Microsoft Edge (the only one
of the two with H.264):

- Lighthouse 13.5.0, Edge headless, against a local server, with its own
  simulated throttling. Mobile: Performance **99** with text gzipped, as GitHub
  Pages serves it (First Contentful Paint 0.9 s, Largest Contentful Paint 2.0 s),
  **98** without compression (LCP 2.4 s); Accessibility 100, Best Practices 100,
  SEO 100; Total Blocking Time 0 and Layout Shift 0 in both. Desktop: 100, 100,
  100, 100. The Largest Contentful Paint is the hero picture; nothing else
  scored under 1.
- 375 and 1440 px screenshots of every section, and no horizontal overflow at 375,
  700, 800, 1024, 1180 and 1440 px.
- The page requests nothing from another origin, in Chromium, Firefox and WebKit;
  Cinzel and Jost both load (`document.fonts`) in each; no console message in
  Chromium and WebKit (Firefox logs a notice that the preload whose media query
  does not match was ignored, which is the intent); no horizontal overflow in
  Firefox or WebKit at 375 and 1440 px.
- Without JavaScript all three systems and the six links are visible; with it, the
  system is chosen from six user-agent cases (Windows, macOS, Linux, an Android
  phone, an iPhone, an iPad that reports as a Mac), the three phones and tablets
  getting the notice about the browser viewer, and a pill click starts no download.
- In Edge the recording loads (1920 × 1080, 104.03 s), a scene time seeks and
  plays it, the scene list follows the playhead, and nine caption cues load.
- A HEAD request to each of the seventeen external links; none is dead.
- The checker was run against ten deliberate faults (a font CDN, a missing image,
  a second `h1`, "coming soon", "open source", an unlisted host, autoplay, a
  third-party script, an Atmos claim, an image without a size) and failed on each.

Also checked: the HTML with `html-validate` (the stock recommended rules, which
only asked for an upper-case `DOCTYPE`, since done).

Not measured: playback in Firefox or WebKit, any physical phone, a screen reader,
the printed page, and the page served by GitHub Pages itself.

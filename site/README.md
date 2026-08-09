# The presentation site

The page people land on before they have downloaded anything:
**<https://benitoow.github.io/theia-media/>**

```bash
node site/build.mjs
node site/check.mjs
```

The build writes `site/dist`. Serve that directory with any static server; for
example:

```bash
python -m http.server 8397 --directory site/dist
```

## Structure

| File | Purpose |
|---|---|
| `locales/fr.js`, `locales/en.js` | Every user-facing sentence. |
| `page.mjs` | One semantic template for both complete pages. |
| `styles.css` | The public-site implementation of the Theia design system. |
| `release.json` | Verified offline release metadata used by local builds. |
| `fetch-release.mjs` | Refreshes that shape from GitHub during Pages builds. |
| `build.mjs` | Validates, renders and copies only the publishable assets. |
| `check.mjs` | Checks downloads, SEO, local assets, sitemap and robots output. |

The build recursively compares catalogue objects and array lengths. A heading,
warning or FAQ cannot land in one language without its counterpart. The output
is French at `/` and English at `/en/`, with canonical, `hreflang`, JSON-LD,
`sitemap.xml` and `robots.txt` generated together.

## Release data

Every download link uses
`releases/latest/download/<asset>`, so the file itself never goes stale.
Version, date, size and SHA-256 are build-time facts, not browser guesses:

- local builds read the verified `site/release.json` snapshot;
- GitHub Pages runs `site/fetch-release.mjs` and supplies the generated file to
  `site/build.mjs`;
- the Pages workflow also runs after a successful Release workflow, because a
  new tag changes release facts without changing `main`;
- if one optional fact is absent, the page omits it rather than inventing it.

JavaScript never selects an architecture. It only reveals the operating-system
panel a visitor explicitly chose. Without JavaScript, all three panels and all
six download links remain in the document.

## Visual assets and provenance

The hero uses `docs/screenshots/player.webp`, captured from the real Theia
player on an isolated library. Its only media file was generated from
`docs/screenshots/source-player-demo-media.svg`, an original vector authored in
this repository. The controls above that capture are labelled as a
demonstration.

`assets/social-preview.png` is rendered from
`assets/social-preview-source.svg`. That source uses the same verified player
capture and no Internet image. The two other page captures are the existing
`docs/screenshots/library.webp` and `settings.webp` files.

Fonts are the Latin Inter and Playfair Display WOFF2 files already installed by
`web/`, under the SIL Open Font License 1.1. No font service, CDN, analytics or
runtime API is used. The page loads only its own HTML, CSS, fonts and images.

## Validation contract

`site/check.mjs` is the fast structural layer. A change is still not finished
until the generated FR and EN pages have been rendered in a browser at the
documented responsive widths, with the download station, player controls,
keyboard path, no-JavaScript fallback and reduced-motion state exercised.

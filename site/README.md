# The presentation site

The page people land on before they have downloaded anything:
**<https://benitoow.github.io/theia-media/>**

Astro 7 + React 19 + Tailwind CSS 4. Most of the page is build-time static
HTML; the two interactive parts (download station, player chrome) are React
islands hydrated only when they approach the viewport. The site performs no
runtime API call, loads no CDN, no analytics and no remote subresource.

```bash
cd site
npm install
npm run build     # writes dist/ (sync-assets runs first)
npm run check     # structural checks on the generated output
npm run dev       # local dev server
```

Serve `dist` with any static server for a final look, for example:

```bash
python -m http.server 8397 --directory site/dist
```

The site is published at a repository subpath; every asset reference resolves
relatively or through Astro's base, so the same build works locally, in
previews and on Pages.

## Structure

| File | Purpose |
|---|---|
| `src/lib/copy.ts` | Every user-facing sentence, English only, typed. |
| `src/pages/index.astro` | The page: static sections plus the two islands. |
| `src/components/DownloadStation.tsx` | The two-step download choice (island). |
| `src/components/PlayerDemo.tsx` | The demonstration player chrome (island). |
| `src/lib/release.ts` | Loads and validates release metadata at build time. |
| `src/lib/format.ts` | Pure formatters the island may bundle. |
| `src/styles/global.css` | The design system restated as Tailwind theme tokens. |
| `scripts/sync-assets.mjs` | Copies screenshots, fonts and icons into `public/`. |
| `scripts/check.mjs` | Structural checks on `dist/`. |
| `release.json` | Verified offline release metadata for local builds. |
| `fetch-release.mjs` | Refreshes that shape from GitHub during Pages builds. |

## Release data

Every download link uses
`releases/latest/download/<asset>`, so the file itself never goes stale.
Version, date, size and SHA-256 are build-time facts, not browser guesses:

- local builds read the verified `site/release.json` snapshot;
- GitHub Pages runs `site/fetch-release.mjs` and supplies the generated file
  through `THEIA_RELEASE_JSON`;
- the Pages workflow also runs after a successful Release workflow, because a
  new tag changes release facts without changing `main`;
- if one optional fact is absent, the page omits it rather than inventing it.

JavaScript never selects an architecture. It only reveals the operating-system
panel a visitor explicitly chose. Without JavaScript, all three panels and all
six download links remain in the document.

## Visual assets and provenance

The hero proof uses `docs/screenshots/player.webp`, captured from the real
Theia player on an isolated library. Its only media file was generated from
`docs/screenshots/source-player-demo-media.svg`, an original vector authored in
this repository. The controls around that capture are labelled as a
demonstration.

`assets/social-preview.png` is rendered from
`assets/social-preview-source.svg`. That source uses the same verified player
capture and no Internet image. The two other page captures are the existing
`docs/screenshots/library.webp` and `settings.webp` files.

Fonts are the Latin Inter and Playfair Display WOFF2 files already installed by
`web/`, under the SIL Open Font License 1.1. No font service, CDN, analytics or
runtime API is used. The page loads only its own HTML, CSS, fonts and images.

## Validation contract

`site/scripts/check.mjs` is the fast structural layer. A change is still not
finished until the generated page has been rendered in a browser at the
documented responsive widths (1100px, 700px, 390px floor), with the download
station, player controls, keyboard path, no-JavaScript fallback and
reduced-motion state exercised.

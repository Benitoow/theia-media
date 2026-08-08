# The presentation site

The page people land on before they have downloaded anything:
**<https://benitoow.github.io/theia-media/>**

```bash
node site/build.mjs
```

Writes `site/dist`. Serve it with anything static:

```bash
python -m http.server 8397 --directory site/dist
```

`.github/workflows/pages.yml` runs the same command on every push to `main`
that touches this folder, the screenshots or the social image, and publishes
the result to GitHub Pages.

## How it is put together

| File | What it holds |
|---|---|
| `locales/fr.js`, `locales/en.js` | Every sentence on the page. Nothing else. |
| `page.mjs` | The markup, once, as a function of a catalogue. |
| `styles.css` | The design system, restated outside the application bundle. |
| `build.mjs` | Renders both languages, copies the assets, checks the catalogues. |

**Both languages render from the same template**, so markup cannot land in one
and not the other, and the build fails if a key exists in one catalogue and not
the other — the guarantee `web/scripts/check-locales.mjs` gives the
application. A new language is a third file in `locales/` and one line in
`build.mjs`.

**No bundler and no dependency of its own.** The two fonts come from the
`@fontsource-variable` packages `web/` already installs, so `npm ci` has to have
run in `web/` before a local build; the script says so rather than shipping a
page that quietly falls back to Georgia.

**Nothing is fetched at runtime.** No analytics, no CDN, no web font service, no
call to the GitHub API. Download links point at
`releases/latest/download/<asset>`, which GitHub redirects to the newest
release, so they cannot go stale and need no JavaScript. The only script on the
page promotes the likely platform in the download list; without it all six are
still there and still work.

## Rules that apply here too

- `docs/design-system.md` is the authority for colour, type, spacing and motion.
  `styles.css` is a second implementation of it, not a place to invent. A value
  that differs from `web/src/app.css` is a bug.
- **No unverified image.** The screenshots come from `docs/screenshots/`, the
  social image from `assets/`. Nothing is fetched from the web.
- Figures on the page are measured, never estimated. If one cannot be
  remeasured, it comes off the page.

# Brand & Product Images

All product marks and icons are derived from four maintainer-supplied, rights-cleared master images stored here:

| Master file | Resolution | Purpose |
| --- | --- | --- |
| `assets/theia-icon.png` | 1024 × 1024 PNG | Square emblem (**T** over the solar surface). Single source for app icons, favicons, tray icon, Windows `.syso` resources, TUI boot grid, and Play Store icon |
| `assets/theia-icon-no-text.png` | 1024 × 1024 PNG | Textless solar emblem used for Android adaptive icon outer bleed background |
| `assets/theia-logo.png` | 1600 × 1061 PNG | Wide header banner with the **THEIA** wordmark over the solar limb, rendered in the top-level `README.md` |
| `assets/theia-logo-no-text.png` | 1600 × 1061 PNG | Textless solar limb backdrop kept for wide promotional banners and social cards |

## Derived assets & where each image lives

| Target | Dimensions / format | Generator |
| --- | --- | --- |
| `player/theia-player/icons/icon.png` | 512 × 512 RGBA PNG | `python scripts/generate-brand-assets.py` |
| `player/theia-player/icons/icon.ico` | Multi-resolution RGBA ICO (`16..256` px; tight crop ≤32 px) | `python scripts/generate-brand-assets.py` |
| `internal/tray/icon.ico` | Multi-resolution RGBA ICO (`16..256` px; tight crop ≤32 px) | `python scripts/generate-brand-assets.py` |
| `cmd/{theia,theia-server,theia-setup}/rsrc_windows_{amd64,arm64}.syso` | Windows PE icon resource objects | `rsrc -arch <amd64\|arm64> -ico internal/tray/icon.ico` |
| `internal/setup/logo_gen.go` | 18 × 18 RGB half-block grid compiled into `theia-setup` | `go run ./scripts/gen-setup-logo` |
| `web/static/{favicon-16,favicon-32,apple-touch-icon}.png` | 16×16, 32×32, 180×180 PNG | `python scripts/generate-brand-assets.py` |
| `site/assets/img/{favicon-16,favicon-32,apple-touch-icon}.png` | Copied from `web/static/` | `python scripts/generate-brand-assets.py` |
| `player/android/app/src/main/res/drawable-xxxhdpi/ic_launcher_background.png` | 432 × 432 RGB PNG (72 dp safe square over blurred `theia-icon-no-text.png`) | `python scripts/generate-brand-assets.py` |
| `player/android/app/src/main/res/drawable-xhdpi/banner.png` | 320 × 180 RGB PNG (Android TV / Leanback banner, Cinzel + Electric Flame) | Chromium + `theia-icon.png` |
| `docs/play-store/icon-512x512.png` | 512 × 512 RGB PNG | `python scripts/generate-brand-assets.py` |
| `docs/play-store/feature-graphic*.png`, `phone/*.png`, `tv/*.png` | Store listing graphics | `python scripts/generate-play-store-assets.py` |

## Other image assets (non-brand)

- `web/static/chrome/{theia-empty,theia-offline,theia-onboarding}.webp`: Classical illustrations / empty-state backgrounds served by the web interface.
- `player/ui/src/assets/media-not-found.webp`: Missing-poster fallback bundled inside the native player UI.
- `docs/screenshots/*.webp` & `site/assets/img/shot-*.webp`: Product interface captures on the synthetic sample library (Cinzel text wordmark; see `docs/screenshots/README.md`).
- `site/assets/icons/`: Upstream pinned SVG icon sets (`sources.json`, verified by `node scripts/site-icons.mjs`).

# Theia Design System

The stable visual reference for the project. Read this before writing any
component; change it here before changing it in code.

---

## 1. Where it came from

Four references were given as direction: Luxam, Rinascimento, Lavoza and Taste
the Notes. Stripped of their subject matter, they share five moves:

1. **Two type registers, violently unequal in scale.** A display serif at
   80–200px sits directly beside a 10px sans label. Nothing occupies the middle.
2. **Wide-tracked uppercase micro-labels.** `YEAR`, `1400–1500`, `ART & CRAFT`.
   Small, letterspaced far apart, always sans, always secondary information.
3. **A cream that is never white.** Text and paper are bone, écru, parchment.
   Pure `#FFF` appears nowhere and reads as cheap next to them.
4. **One accent, used four or five times per screen.** Not a palette. One colour
   that means "look here".
5. **Negative space as the primary compositional element.** The subject occupies
   a third of the canvas; the rest is deliberately empty.

Theia inverts the paper references to a dark ground, because a media server is
looked at in a dark room and everything in it is a bright rectangle.

## 2. The accent: gold, not oxblood

Both were on the table. Gold wins on three counts:

- **The name argues for it.** Theia is the titan of sight and heavenly light,
  mother of Helios. A warm light source is the brand, literally.
- **Red is already spoken for.** In an interface, red means error. Spending it
  on decoration means inventing a second alarm colour later.
- **Red is what the competition looks like.** Netflix, Plex and YouTube are all
  red. The project's entire pitch is being the other thing.

This is one token. If you disagree, change `--accent` and its two variants in
one place and the whole app follows.

## 3. Colour

All contrast ratios below are measured against `--ink` and stated explicitly,
because a dark theme with cream text fails silently in exactly the places
nobody checks.

### Surfaces

| Token | Value | Use |
|---|---|---|
| `--ink` | `#0B0A09` | Page background. Near-black, faintly warm — pure black reads as an OLED void, not a cinema |
| `--surface` | `#131211` | Cards, panels, the settings sheet |
| `--raised` | `#1C1A18` | Hover state on a surface, active row |
| `--line` | `#2A2724` | Hairline borders. Never lighter than this, or the grid starts shouting |

### Text

| Token | Value | Contrast on `--ink` | Use |
|---|---|---|---|
| `--bone` | `#EDE7DC` | **16.07:1** | Primary text, headings, hero type |
| `--parchment` | `#D6CFC2` | **12.78:1** | Secondary text, body copy at length |
| `--muted` | `#8C857A` | **5.42:1** | Labels, metadata, captions. Passes AA for normal text |
| `--faint` | `#5A544C` | **2.64:1** | Disabled and decorative only. **Never for text a user must read** |

### Accent

| Token | Value | Contrast on `--ink` | Use |
|---|---|---|---|
| `--accent` | `#C8A24A` | **8.22:1** | The single accent. Focus rings, the active nav item, the play affordance, the progress bar |
| `--accent-bright` | `#E3C173` | 11.44:1 | Hover and focus states of the above |
| `--accent-dim` | `#7A6330` | 3.44:1 | Borders and glows only, never text |

**Budget: five uses of `--accent` per screen.** If a sixth appears, something
else on the screen has stopped being important. This is the rule the references
actually obey and it is the one most easily broken.

### Semantic

Red is free precisely because the accent is gold.

| Token | Value | Contrast | Use |
|---|---|---|---|
| `--error` | `#D06A5D` | 5.56:1 | Failed scans, unreachable server, destructive confirmation |
| `--warning` | `#C9964A` | 7.48:1 | Degraded state — mDNS on IPv6 only, ffmpeg missing |

The first candidate for `--error` was `#C75B4F`, which measures 4.75:1 — over the
4.5 AA threshold, but only just. Error text is the one thing a user reads while
already annoyed, often on a TV across a room, so it was lightened until it had
real headroom.

Every ratio in this document was computed from the WCAG relative-luminance
formula rather than eyeballed; the script lives in
[`scripts/contrast.mjs`](../scripts/contrast.mjs) and should be re-run whenever a
colour changes.

`--warning` sits deliberately close to `--accent`. A warning in this app is
almost always "it still works, but less well", and it should feel adjacent to
the brand rather than alarming.

## 4. Typography

### The two registers

```css
--font-display: Didot, "Bodoni MT", "Playfair Display", Georgia, "Times New Roman", serif;
--font-ui: Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
```

`--font-display` is for hero titles, film titles on a detail page, and empty-state
headlines. Nothing else. `--font-ui` is for everything else without exception:
nav, labels, body copy, buttons, metadata, the poster grid.

The stacks are system fonts on purpose — see §9, this is an open decision.

### Scale

| Token | Size | Tracking | Line height | Use |
|---|---|---|---|---|
| `--text-hero` | `clamp(3.5rem, 12vw, 9rem)` | `-0.03em` | `0.9` | One per screen, maximum |
| `--text-display` | `clamp(2.25rem, 6vw, 4rem)` | `-0.02em` | `1.0` | Section openers, film title on a detail page |
| `--text-title` | `1.5rem` | `-0.01em` | `1.2` | Card headings, dialog titles |
| `--text-body` | `1rem` | `0` | `1.6` | Prose. The only token with comfortable leading |
| `--text-small` | `0.875rem` | `0` | `1.5` | Secondary copy, form help |
| `--text-label` | `0.6875rem` | `0.18em` | `1.2` | **The signature move.** Uppercase, sans, wide. `ANNÉE`, `DURÉE`, `EN COURS` |
| `--text-micro` | `0.625rem` | `0.22em` | `1.2` | Uppercase. Footer, version string, legal |

**The tracking rule, which is the whole system in one line:** large type tracks
in, small type tracks out. A hero at `-0.03em` and a label at `+0.18em` on the
same screen is what makes it look designed rather than merely dark.

### Weight

Display serif at 400 only. The high-contrast thick–thin does the work; a bold
Didone at hero size is a smear. UI sans uses 400 for body, 500 for nav and
buttons, 600 as the ceiling.

## 5. Spacing

4px base. The scale is deliberately gappy at the top — hero screens need room
that a 4/8/12/16 ramp cannot express.

```css
--space-1: 0.25rem;   /*  4px */
--space-2: 0.5rem;    /*  8px */
--space-3: 0.75rem;   /* 12px */
--space-4: 1rem;      /* 16px */
--space-6: 1.5rem;    /* 24px */
--space-8: 2rem;      /* 32px */
--space-12: 3rem;     /* 48px */
--space-16: 4rem;     /* 64px */
--space-24: 6rem;     /* 96px */
--space-32: 8rem;     /* 128px */
--space-48: 12rem;    /* 192px */
```

Hero and empty-state screens start at `--space-24` for vertical rhythm. Dense
surfaces — the poster grid, settings rows — live between `--space-2` and
`--space-6`. There is no middle ground and that is intentional.

Page gutters: `--space-6` on mobile, `--space-16` from 1024px up.
Content max width: `72rem`. Prose max width: `38rem`.

## 6. The poster grid is exempt

**This is the most important constraint in the document.**

Everything above governs the *chrome*: nav, hero, empty states, the QR onboarding
screen, settings, error states. It does **not** govern the poster grid.

TMDB posters arrive with their own typography, colour and contrast — a hundred
competing art directions in a hundred rectangles. Applying a dramatic serif and
generous negative space on top of that produces something slow to scan and ugly
in a way that is hard to diagnose. The grid's job is density and speed.

Rules for the grid, which override §4 and §5:

- Card titles use `--font-ui` at `--text-small`, never the display serif.
- Grid gap is `--space-3`, not the chrome spacing scale.
- Cards carry no accent colour at rest. Gold appears on hover and focus only,
  as a 1px `--accent` border and nothing more.
- Poster aspect ratio is locked to `2/3`. Never crop, never letterbox.
- Card chrome is minimal: poster, title, year. Everything else belongs on the
  detail page.
- While a poster loads, the card shows `--surface` with a subtle shimmer, never
  a spinner and never a broken-image icon.

The transition between the two worlds is the point: dramatic, near-empty chrome
framing a dense, fast, businesslike grid.

## 7. Texture and imagery

**No external images, ever.** This repository is public and GPL-3.0; every asset
has to have its licence verified by the maintainer before it lands. No agent
working on this project fetches decorative imagery from the web.

Where a screen needs atmosphere, generate it in CSS. Two authored recipes, both
inline, neither making a network request:

```css
/* Vignette: pulls the eye to the centre of a hero. */
--texture-vignette: radial-gradient(
  ellipse 120% 80% at 50% 40%,
  transparent 0%,
  rgba(0, 0, 0, 0.45) 100%
);

/* Film grain: authored SVG turbulence as a data URI. Keep opacity under 0.05
   or it stops being texture and starts being noise. */
--texture-grain: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='140' height='140'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.85' numOctaves='3' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)' opacity='0.035'/%3E%3C/svg%3E");

/* Warm light source, for the top of hero screens. The Helios reference. */
--texture-glow: radial-gradient(
  ellipse 80% 50% at 50% -10%,
  rgba(200, 162, 74, 0.10) 0%,
  transparent 70%
);
```

The one image the app ships is the favicon, authored as SVG in this repository.

## 8. Motion

Slow and eased, never bouncy. The reference for timing is a camera move, not a
UI toast.

```css
--ease: cubic-bezier(0.16, 1, 0.3, 1);   /* expo-out: fast start, long settle */
--duration-fast: 160ms;    /* hover, focus ring */
--duration-base: 320ms;    /* panels, cards, state changes */
--duration-slow: 700ms;    /* hero and first-paint entrances */
```

Hero type and empty-state content fade up 16px on entrance at `--duration-slow`.
Everything else uses `--duration-base`.

**Every animation must be wrapped in a reduced-motion guard.** Not optional:

```css
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.01ms !important;
    transition-duration: 0.01ms !important;
  }
}
```

## 9. Focus and accessibility

- Focus ring: `2px solid var(--accent)` with `2px` offset. Gold on near-black is
  8.2:1 and unmistakable. Never remove the outline without replacing it.
- Use `:focus-visible`, not `:focus`, so mouse users do not see rings.
- Every interactive target is at least 44×44px, including in the poster grid.
- `--faint` and `--accent-dim` fail normal-text contrast. They are for disabled
  states, decorative rules and borders. This is written down because it is the
  rule most likely to be broken by accident.
- The interface is French; `<html lang="fr">` is already set and stays set.

## 10. Open decisions

**Fonts are unresolved, and this is the one thing blocking a faithful
implementation.**

The stacks in §4 are system fonts. On macOS, `Didot` renders close to the
references. On Windows, `Bodoni MT` exists only if Microsoft Office is installed,
so most Windows users will fall back to Georgia — a perfectly decent serif that
is nowhere near dramatic enough for what the references promise.

Getting the real thing means bundling font files in the repository, which runs
into two project constraints at once:

- **No external network calls.** Google Fonts as a CDN link is out. Fonts have
  to be self-hosted and embedded in the binary alongside the rest of `web-dist`.
- **Licence verification is yours.** Per your instruction, I do not fetch assets
  from the web myself.

Candidates worth your review, all under the SIL Open Font License, which is
GPL-compatible and imposes no obligation on the rest of the project:

| Font | Register | Why |
|---|---|---|
| **Playfair Display** | Display serif | High-contrast transitional, closest to the Rinascimento reference. Variable weight |
| **Cormorant Garamond** | Display serif | Lighter, more literary, closer to Luxam. Beautiful at hero size, weak below 24px |
| **Inter** | UI sans | Designed for screens, excellent at 11px with wide tracking — exactly the label register |

Two subset WOFF2 files would add roughly 80–120 KB to the binary.

Say the word and which ones, and I will wire them in. Until then the system
fonts are a real fallback rather than a placeholder — the layout, scale,
tracking and colour are all correct without them.

## 11. Migration note

M0 shipped a provisional palette in `web/src/app.css` — `--color-ink: #0b0b0f`,
`--color-helios: #f5a623` and three others. Those predate this document and are
superseded by it. Rewriting them is the first task of the next milestone.

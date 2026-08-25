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

The v1 television pass adds three more precise references:

- **Apple TV+ sets the interaction scale.** Large targets, short lines, obvious
  focus and information that survives three metres of distance are the real
  product constraints.
- **Netflix contributes one structural idea only:** a strong hero followed by
  horizontal rows. Its visual language is deliberately not copied — and neither
  is its purpose. Theia's home screen is not a shop front trying to hold you: it
  is four short rows about your own library, and every one of them points at the
  page that does the real browsing. See decision 29.
- **Squarespace confirms the palette and display register:** warm darkness,
  cinematic imagery and a dramatic serif belong to the chrome, not to the film
  grid.

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

### The three registers

```css
--font-display: "Cinzel Variable", "Trajan Pro", Georgia, "Times New Roman", serif;
--font-label:   "Jost Variable", Futura, "Century Gothic", "Segoe UI", Roboto, sans-serif;
--font-ui:      -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
```

There were two registers until the faces changed. There are three now because
the new display face cannot do the work the old one did in the middle.

`--font-display` is the voice: hero titles, film titles on a detail page,
empty-state headlines, the wordmark. Nothing else.

`--font-label` is the chrome: `.label`, `.micro`, section headings, the
navigation pill and the phone's tab bar. Short, deliberate strings — never
prose.

`--font-ui` is for reading, and is deliberately the platform's own interface
face rather than anything shipped. Both shipped faces are display faces; a
synopsis set in small caps at the three-metre rule is a legibility problem, not
a style.

**Cinzel draws lowercase as small capitals.** Measured, not assumed: `abcdef`
is 150 units against `ABCDEF` at 158, so there is a real case distinction and a
title keeps its shape without `text-transform`. Write titles in ordinary
sentence case in the catalogues; the face does the rest.

**Jost has true lowercase** (116 against 145) and real weights, being a
variable face, so a section heading can carry weight 500 rather than a
synthesised one.

Both were checked glyph by glyph for É È Ê Ï Ô Ù Ç Œ œ and for `?` `!` `·` `—`.
Both are complete. That list is not ceremony: the face this replaced drew its
question mark as a figure 9, and one of the candidates refused alongside it
drew accented letters with no accent at all.

The first family in each stack is self-hosted; the remaining system fonts are
deliberate fallbacks. See §10 for the shipped files.

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

### The three-metre rule

The primary client is a television read from a sofa, not a laptop held at arm's
length. The base scale above remains the small-screen floor; wide viewports add
a viewing-distance layer without inventing a third type register:

- chrome body copy scales from 16px to 19px;
- row headings scale from 18px to 24px and stay in the UI sans;
- card titles scale from `--text-small` (14px) to 16px, still in the UI sans;
- tracked labels scale from 11px to 13px. They remain metadata, never the only
  wording on a primary action.

**Above 100rem there is a second step, and it is the one written for a sofa.**
The numbers above are the large-screen layer and were always measured on a
desktop; a television is not a large desktop. Amazon's Fire TV guidance puts the
floor for body text at 28px on 1080p, Android TV asks 24sp, and the standing
advice is two and a half to three times a phone's sizes — against which 16px to
19px is 1.19x. At 1600px and above the type tokens themselves move: labels to
16px, secondary copy to 18px, body to 24px, titles to 30px, with card titles at
19px and row headings at 30px. The display serif is untouched; it already fills
a screen. The micro register moves least, because it is the footer and the legal
line and must not start competing with the film.

The step is at 100rem rather than 80rem so that a desktop window keeps the sizes
it has. Viewport width cannot tell a 1920 television from a 1920 monitor, and
1600px is simply above every desktop this interface has been looked at on.

The test is blunt: the main action, current row and focused film must be
identifiable from three metres away on a 1080p screen. A desktop screenshot at
100% zoom is not evidence that this passes.

## 5. Spacing and the page frame

4px base, and it is **Tailwind's scale**, not a parallel one. An earlier draft of
this document declared a `--space-1…48` ramp that was never implemented; the code
had always used Tailwind's utilities and fluid `clamp()`. Rather than build a
second system that would have to be kept in step by hand, the ramp was deleted
and this section was rewritten to describe what actually ships. See
`DECISIONS.md` for the reasoning.

The rhythm the ramp was trying to express still holds, and is the real rule:
**chrome breathes, dense surfaces do not.** Hero and full-bleed message screens
work in multiples of 4rem and up. The card grid and settings rows work between
0.5rem and 1.5rem. There is no middle ground and that is intentional.

What *is* tokenised is the page frame, because those are contracts rather than
taste — every screen has to agree on them or they drift:

```css
--page-gutter: clamp(1.5rem, 4vw, 5.5rem);  /* phone to television */
--content-wide: 96rem;                      /* content max width */
--nav-height: 4rem;                         /* 4.5rem at TV width */
--nav-inset: 1rem;                           /* the bar's distance from the top edge */
--nav-offset: calc(var(--nav-height) + var(--nav-inset) * 2);
```

**Above 100rem all three change, for the safe area.** Every television guideline
agrees on the outer five per cent — roughly 96px horizontally and 54px
vertically on 1920×1080 — kept clear so an overscanning set cannot crop it. The
gutter becomes `6rem`, the bar's inset becomes `3.5rem`, and the player's own
furniture moves in to match. `--content-wide` becomes `120rem` at the same step,
which is not about width but about alignment: at 96rem the shell stops touching
the gutters past about 1712px and centres itself instead, so the row headings
ended up 184px right of their own cards and the wordmark 108px right of the hero
title. Letting the shell fill the safe area puts every left edge back on one
line. Prose keeps its own 38rem cap and is unaffected.

`--nav-offset` exists because the nav floats over the page, so every screen has
to start below it. Four screens each guessed their own top padding — `pt-32`,
`pt-36`, `pt-32 lg:pt-40`, `pt-36 lg:pt-44` — and no two agreed. Two classes now
own it: `.page-body` for a normal page, `.page-body--hero` for a screen whose
content hangs off the bottom of a full viewport. Prose max width stays `38rem`.

Both were later measured and cut. `.page-body` added 3–5rem *on top of*
`--nav-offset`, which already clears the bar with air to spare, so every utility
page began 184px down and a library showed one row of films on a laptop; it now
adds 2–3rem. `.page-body--hero` added 6–9rem at the top of a section whose
content is bottom-aligned inside a `min-height` — padding nobody could see,
which pushed the home hero to 824px on a 768px window. The hero was therefore
cut off at the bottom and not one pixel of the first row ever showed. A row edge
under the fold is what tells somebody there is a library beneath the film;
without it the home screen reads as a single poster. The top value now only has
to clear the bar, and the section is `78svh`.

**Picture veils are classes, not inline styles.** The hero, the film and series
detail headers and the welcome screen each carried their gradient stack in a
`style` attribute with hand-typed `rgba`. Those were the last places in the
application restating a token's channels by hand, and an inline gradient is a
gradient no stylesheet can find. They are `.picture-veil` plus a modifier, built
from the channel tokens in §3. The artwork under them was also held at 55–60%
and then covered by three fades, which is what made every hero read as muted;
the veil already does the protecting, so the picture runs at 72–78%. The
resulting contrast was measured on the composited pixels rather than judged:
`--muted` over the metadata row reads 5.43:1 on the home hero and 5.49:1 on the
film page, both above the 4.5 AA floor.


**Every backdrop the navigation bar floats over is framed from the top, not the
centre.** A 16/9 still in a header two or three times as wide as it is tall is
cropped top and bottom, and where that crop is taken is a layout question rather
than a per-image one: the pill floats over the top 128px and the title, metadata,
poster and buttons occupy the bottom, so the headroom is the only part of the
frame nothing else is already drawn over. `object-top` spends the crop on the
bottom, where the veil and the copy are, and moves the subject down out from
under the bar.

Measured at 1920×1080, per header:

| | box | vertical overflow | centring hid |
|---|---|---|---|
| Film detail, 68svh | 734px | 338px | **169px** of headroom |
| Series detail, 58svh | 626px | 446px | **223px** |
| Home hero, 78svh | 842px | 230px | **115px** |

All three sit under a bar 128px deep. Rendered and looked at rather than
inferred: centred, a face and a helmet sat inside the bar; from the top, both
clear it. On a phone the box is narrower than the picture, the crop is
horizontal, and this changes nothing.

**Two deliberate exceptions**, both explicit rather than accidental.
`.chrome-scene-image` is framed at `68% center` because its copy sits in the left
third and the photograph belongs on the right. The welcome screen stays centred
because its asset is exactly 1920×1080: on a 16/9 window it fits with no crop at
all, and on a wider one, framing it from the top brings the winged figure's face
into the bar rather than out of it — top alignment is already the furthest that
picture can move down, so a head out of shot beats a head cut in half.

**`web/scripts/check-backdrops.mjs` enforces this**, and exists because writing it
down did not. The rule was documented when the home hero was fixed, and the film
and series headers shipped centred anyway; the same fault was reported twice. The
script fails the frontend build if a full-bleed picture — pinned to every edge
with `object-cover`, in markup or in this stylesheet — does not say where it is
framed. It does not insist on `object-top`; it insists the choice be made on
purpose.

Headings follow the same principle. `.hero-title` is the largest register and
stays reserved for what fills a viewport. `.page-title` is one step down, sized
from `--text-display`, for a page with work to do underneath its heading;
`.page-title--feature` is its one exception, for a film's own title on its own
page. Before these existed, three screens reached for `.hero-title` and then
shouted it back down with an `!important` arbitrary value.

## 6. The card grid is exempt

**This is the most important constraint in the document.**

Everything above governs the *chrome*: nav, hero, empty states, the QR onboarding
screen, settings, error states. It does **not** govern the card grid.

Cover art arrives with its own typography, colour and contrast — a hundred
competing art directions in a hundred rectangles. Applying a dramatic serif and
generous negative space on top of that produces something slow to scan and ugly
in a way that is hard to diagnose. The grid's job is density and speed.

### 6.1 Cards are wide, landscape and generously rounded

**This replaces the locked `2/3` portrait poster that stood from M3 to v1.3.0.**
The change was a deliberate change of direction by the maintainer, not a
correction — the old rule was coherent, it was simply not the look wanted. Do not
reinstate the portrait card because an older comment or commit still describes
it. See `DECISIONS.md` 33.

- **Aspect ratio is `16 / 9`.** Fixed, on every card, in both the home rows and
  the library grid.
- **The artwork is the film's backdrop**, at `w780`, because a backdrop is
  already 16/9 and so is not cropped to fit. Posters are not used on cards.
- **Corners use `--radius-card`** — `clamp(1rem, 1.6vw, 1.375rem)`. The interface
  rounds two ways: pills at `999px` for anything you press, and a panel radius
  from `1rem` up for anything you look at. A card belongs to the second family,
  scaled slightly down for a smaller box. The old `4px` is gone.
- **Widths.** Row cards are `clamp(14rem, 19vw, 19rem)`, rising to
  `clamp(16rem, 17vw, 21rem)` past 80rem. The library grid is
  `auto-fill minmax(12rem, 1fr)`, and `15rem` past 64rem. **Below 36rem it is
  two fixed columns instead**, because `auto-fill` against a 12rem minimum
  resolves to exactly one column on a phone: at 390px the content box is 342px
  and two cards plus their gap need 398. One card per row is a screen and a half
  of library at a time. Two columns give a 164px card there — larger than the
  thumbnail any phone-sized catalogue uses, still plainly readable, and four rows
  visible instead of one. The 12rem floor below stays what it is; it was measured
  on a desktop and it is right there. The minimum is what a
  backdrop needs to stay identifiable, not what fits the most columns: below
  about 12rem a landscape still is a smudge, and a grid of smudges scans slower
  than a grid of fewer readable ones.
- **Wider does not mean sparser.** A 16/9 card is far shorter than the 2:3 poster
  it replaced, so more rows fit. Measured on the 274-film library at 1280×800:
  before, six columns of 432px-tall cards put **11 films on screen**; after, four
  columns of 192px-tall cards put **17**. Any future change to these numbers is
  to be checked the same way, not reasoned about.

Artwork that is missing is a normal state, and the fallbacks are ordered:

1. the backdrop, covering the frame;
2. failing that, the poster, **contained** rather than covering — a portrait
   image cropped to fill a landscape box loses its title off the top and bottom.
   On the current library this case is empty: artwork arrives in pairs or not at
   all;
3. failing both, the title as text on `--surface`. Never a broken-image icon.

**A real 2:3 poster still exists** — on the film detail page, which has the room
for it. It takes `--radius-card` like everything else. What changed is that cards
show backdrops; posters were not banished from the interface.

### 6.2 Everything else about a card

Rules that override §4 and §5:

- Card titles use `--font-ui` at `--text-small`, never the display serif. At TV
  width they may scale up to 1rem under the three-metre rule; this is still the
  small UI register, not a card-specific heading style.
- Grid gap is `clamp(0.9rem, 1.6vw, 1.5rem)`, deliberately tighter than chrome
  spacing.
- Cards carry no accent colour at rest. Gold appears on hover and focus as a
  1px `--accent` border. Keyboard or remote focus also keeps the global 2px
  focus ring around the whole link; it is navigation state, not card chrome,
  and a one-pixel border alone disappears from a sofa.
- **The card has weight at rest and says what it does on hover.** At rest the
  frame keeps a 1px hairline at `--bone` 6% and a soft shadow: artwork sitting
  directly on the page with no edge reads as a picture printed on the
  background rather than an object to pick up. The border is 1px in every
  state, including transparent ones, so gaining the gold one never reflows the
  card by a pixel.
  On hover and on keyboard focus the frame lifts `0.5rem`, scales `1.015`, and
  draws a radial scrim with a filled play mark at its centre. The mark is
  `--bone`, not gold — the accent still means "look here" elsewhere on the
  screen, exactly as it does for the player's one filled control (§6b) — and it
  is `aria-hidden`, because the link around it already says where it goes.
  Radial rather than a bottom-up gradient: the title sits *under* the card, so
  weighting the bottom would darken artwork to protect text that is not there.
  This is the one thing the grid gained beyond §6's density rule, and it earns
  it: a hairline that turns gold says "this one is selected", and never says
  "this one plays", which is the only thing any of them do. It costs the grid
  nothing at rest, and it is suppressed entirely on touch pointers, where a
  hover state would hide the affordance from the device most likely to be
  holding it.
- **One exception, added in M5: the playback progress bar.** A 3px gold rule
  across the bottom of a part-watched card. It earns its place because it is
  information rather than decoration — it is the entire reason the
  "continue watching" row exists — and because no amount of hover state can
  convey it. It is drawn only for films actually part-watched: never at zero,
  never on a finished film. Nothing else in the grid may take this exemption
  without being written down here first.
- Card chrome is minimal: artwork, title, one compact legend. Home rows use the
  year. On `/films`, the legend follows the active sort — year, rating, date
  added or runtime — and stays visible at rest. It is never revealed only on
  hover: a library operated with a D-pad cannot hide useful information behind
  a mouse gesture. Title sorting keeps the year as its useful secondary value.
  The legend stays in the muted UI register with no accent; everything else
  belongs on the detail page.
- While artwork loads, the card shows `--surface` with a subtle shimmer, never
  a spinner and never a broken-image icon.

The transition between the two worlds is the point: dramatic, near-empty chrome
framing a dense, fast, businesslike grid.

### 6.3 Rows scroll without a scrollbar

Home rows hide the native scrollbar entirely (`.scrollbar-hidden`). The Windows
default is a wide grey slab that cuts a row in half, and thinned to 6px it was
still a second, worse answer to a question the chevrons already answer.

Nothing is lost, and this is the check to repeat before changing it: a mouse gets
the hover chevrons, a trackpad and a touchscreen swipe, a keyboard and a D-pad
move card by card through the row's own arrow handling. The chevrons stay
`tabindex="-1"` and outside the directional-navigation graph, so they add no
stops to either.

**The library page follows the same split.** Its toolbar — search, sort, genre,
watch state — is chrome and carries the treatment: a rounded bar, glass, the
label register. The grid beneath it is a wrapping `auto-fill` grid of the same
plain cards, because that page exists to find one film among hundreds and a row
you have to drag through is the wrong shape for it. Nothing about a card changes
between the two screens except its width, which the grid column decides, and the
contextual legend defined above.

**Page loading uses the page's silhouette, never a brand spinner.** The home
screen, library grid and film detail render quiet card, toolbar and text blocks
immediately while their first API request is pending. These skeletons use
`--surface` and `--raised`, and must be kept in step with the real card by hand:
they carry the same `16 / 9`, the same `--radius-card` and the same column
minimums, because a silhouette that does not match what replaces it makes the
page visibly jump at the moment it finishes loading. They shimmer only when
reduced motion is not requested, describe structure rather than fake content, and
expose one polite loading status to assistive technology.

## 6b. The player is chrome, and it gets out of the way

The picture fills the frame; everything else floats over it. A video boxed inside
a header, a strip of text buttons and a page background is a preview window, not
a player.

- **Controls are icons**, drawn on one 24-unit grid at one stroke weight in
  `Icon.svelte`, never text. `LIRE` and `COUPER LE SON` spelled out read as a
  debug panel. The words survive as accessible names, which is where they belong.
- **One filled control**, the play button, in `--bone` rather than gold: the
  accent still has to mean "look here" everywhere else on the screen.
- **The furniture hides** after three seconds of no pointer, no key and no state
  change, and takes the cursor with it. It comes back on any sign of life. It
  never hides while paused, seeking or buffering, and never with focus stranded
  on a control — focus moves to the dialog first.
- **The scrub bar shows three things**: played in gold, buffered in a lighter
  bone, and the rest. The bar itself is 4px because that reads as precision; its
  hit area is 24px because a thumb is not a mouse. A hover anywhere on it shows
  the timestamp under the pointer.
- **Scrims, not a wash.** Two gradients, top and bottom, so the text has a floor
  without dimming the film.
- **The bar never wraps.** A control row that reflows into two lines moves the
  play button out from under a thumb mid-press. Below 30rem the volume control
  and the shortcuts button go instead — a phone has hardware volume keys, and a
  keyboard-shortcuts panel on a touchscreen is help for a keyboard nobody is
  holding. Everything that stays keeps its 3.25rem target.
- **The clock shows two numbers.** Elapsed in `--bone` at 500, total in
  `--muted`, separated by a drawn hairline rather than a slash glyph, which sits
  at the wrong optical height at this size. A third number for the remaining
  time is the other two subtracted, and it was printed in `--faint`.
- **Audio and subtitles are a popover, not a panel.** It is a child of the
  button that opens it, so it is anchored by construction: `right: 0` against
  the button's own box rather than a measured offset from the frame. Positioned
  against the frame it sat 117px from its control on a 900px viewport and read
  as a slab that happened to appear. Below 30rem it pins to the frame instead,
  because a 21rem panel aligned to a button 60px from the right edge starts off
  the left of a 390px screen.
- **A track is two lines.** The language leads at reading size, because that is
  what the choice is made on; codec, channels and provenance sit under it as a
  tracked label in `--muted`. One middot-joined string put four facts at one
  weight, which at three metres is a wall. A detail that merely repeats the line
  above it is dropped — a French track titled "Français" read as
  `Français · Français`.
- **The chosen track carries a tick**, in gold, in a column that is reserved
  whether or not it is drawn. A 2px rule at 6% fill does not survive the room.
  The popover has no dismiss button: Escape, the toggle, and a press outside.
- **Subtitles are shadowed, never boxed.** A `background` on `::cue` is painted
  per line, so a two-line cue gets two slabs of different widths with a ragged
  step between them. A tight four-way shadow plus one soft drop follows the
  letterforms instead of boxing them, and holds on saturated colour bars — the
  worst case there is. Type runs to 2.375rem, about 3.5% of a 1080p frame,
  which is broadcast practice; the previous ceiling measured 2.8% and read as a
  caption on a monitor.
- **Cues lift when the furniture appears.** `line` as a count of lines snaps to
  a height the engine picks, and `lineAlign: 'end'` is ignored by Chrome, so the
  position is computed from the cue's own line count and the measured height of
  the control bar. Measured at 1080p: 79.06% with the bar up, 90.76% with it
  down. The engine maps that request through a safe area of its own; what this
  controls is the delta, which is all it needs to control.
- **Keyboard shortcuts are discoverable.** `?` opens an in-player help panel,
  also reachable from an icon button in the control bar. While it is open the
  underlying controls are inert, Tab stays inside the panel, Escape closes the
  help before it closes the player, and focus returns to the control that
  opened it. If the panel overflows on a short screen, vertical arrows,
  Page Up/Down and Home/End scroll the help itself rather than controlling the
  film underneath; its heading and close control remain pinned so the active
  focus never leaves the viewport.

## 7. Texture and imagery

**No unverified image, ever.** This repository is public and GPL-3.0; every
shipped asset has to have its licence verified by the maintainer before it
lands. No agent working on this project fetches decorative imagery from the
web. A maintainer-supplied, licence-checked pack may be used under the following
constraints:

- imagery belongs to the chrome only: hero, onboarding, empty and error states;
- the card grid remains exempt under §6 — no decorative image, overlay,
  colour grade or authored photographic treatment is added to it;
- source files never ship as-is: crop to the rendered aspect ratio, resize for
  the largest real viewport, then encode as WebP or AVIF before `web-dist`;
- every image remains legible under the interface's own text and focus states,
  and is decorative (`alt=""`) unless it conveys information not written
  elsewhere.

CSS texture remains the fallback when a screen needs atmosphere without a
maintainer-verified asset. The authored recipes below are inline and make no
network request.

**Only the grain is wired up**, as `--texture-grain` in `app.css`, applied by
`.grain` on `<body>` in `app.html`. The vignette and glow are recipes on file,
not live tokens: they were declared in `:root` for months with no consumer, and
`Hero.svelte` authors its own halo positioned differently on purpose. A token
nothing reads is a token that rots, so they were removed from the stylesheet and
kept here. Paste one in when a screen needs it.

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

The favicon remains authored as SVG in this repository. Any additional shipped
image must satisfy the rules above and stay self-hosted.

### 7.1 The mark

The identity is **the word and the rule**: THEIA set in the display serif,
tracked at `--text-label--letter-spacing`, underlined by a gold rule that fades
to the right so it reads as a horizon running off rather than an underline that
stopped. The rule is the mark's single use of the accent.

Where the word does not fit, the mark is **its initial standing on the same
rule** — a crop of the lockup, not a second mark beside it. That is the favicon
and, rendered from the same file, the touch icon.

Two rules that are not preferences:

- **The letter is drawn as paths.** A favicon is fetched as an image and no
  webfont reaches it; a text element would fall back to whatever serif the
  platform has. This costs some didone contrast, because thick/thin is what
  vanishes first at 16px, and legibility there wins.
- **A photographic wordmark is a prestige piece, never chrome.** Founding spec
  §11.10 said so and decision 51 measured it: at 28px the fill's horizon band
  runs through the letters and reads as a strikethrough. It lives in `assets/`
  for the README, outside the binary.

See decisions 51 and 54.

## 8. Motion

Slow and eased, never bouncy. The reference for timing is a camera move, not a
UI toast.

```css
--ease-cine: cubic-bezier(0.16, 1, 0.3, 1);  /* expo-out: fast start, long settle */
--duration-fast: 160ms;    /* hover, focus ring, a control answering a press */
--duration-base: 320ms;    /* panels, cards, state changes */
--duration-slow: 700ms;    /* hero and first-paint entrances */
```

These are real tokens in `app.css`. They were not, for a long time: the three
durations were written out as literals in thirty-two places, which is how a
stray fourth value ended up sitting beside the fast one with nobody able to say
why. Reach for the token; if a new duration seems necessary, change this
document first.

Hero type and full-bleed message content fade up 16px on entrance at
`--duration-slow`, via `.enter`. `.enter-2` and `.enter-3` stagger by 90ms each
so a heading leads its own paragraph instead of the block arriving as one slab.
Everything else uses `--duration-base`. **The card grid never animates in** —
a hundred cards fading up is a slideshow, not a library (§6).

**Every animation must be wrapped in a reduced-motion guard.** Not optional:

```css
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    animation-delay: 0ms !important;
    transition-duration: 0.01ms !important;
  }
}
```

**Never put `backdrop-filter` in a transition.** A backdrop blur is the most
expensive thing a compositor does — it reads back everything behind the element
and blurs it — and transitioning one asks for that work on every frame. The
navigation bar used to go from `blur(14px)` to `blur(24px)` on scroll: the one
element on screen the whole time, re-blurring the page behind it, triggered by
the one gesture that already costs the most. It now holds a single `blur(20px)`
at both states and cross-fades only background, border and shadow, which is what
the eye was reading anyway. Blur values may change between breakpoints; they may
not be animated between them.

Two traps this guard sets, both of which have to be handled at the animation
rather than here:

- **An entrance needs `animation-fill-mode: both`.** The guard collapses the
  duration to nothing; without a forwards fill the element is left at its `from`
  state, so an accessibility preference would blank the page. `.enter` sets it.
- **A looping shimmer needs `animation: none`**, declared before the guard on
  the element itself. Collapsing a loop to 0.01ms leaves it stopped at an
  arbitrary frame rather than at rest. The loading skeleton does this, and its
  resting state is a flat `--surface`.

## 9. Focus and accessibility

- Focus ring: `2px solid var(--accent)` with `2px` offset, rising to `4px` and a
  `3px` offset above 100rem. Gold on near-black is 8.2:1 and unmistakable. Never
  remove the outline without replacing it. The television step exists because
  focus *is* the cursor there — no pointer, no hover — and 2px at 1920 on a
  55-inch set subtends about one arcminute from a sofa, which is the threshold
  of seeing a line at all before a panel's motion blur gets to it. Cards are
  already exempt: §6.2 gave them a border, a lift, a scale and a shadow together
  for exactly this reason.
- Use `:focus-visible`, not `:focus`, so mouse users do not see rings.
- Every interactive target is at least 44×44px, including in the card grid.
- Primary actions are at least 56px high. Navigation and secondary controls are
  at least 52px, rising to 56px on viewports 1280px and wider.
- Poster cards are at least 160px wide and rise to 192–224px on TV-width
  viewports. More tiny posters is not more useful content when none can be read.
- The first D-pad arrow enters at the screen's primary action. Up and down then
  use spatial navigation; left and right move deterministically inside a film
  row. Tab remains available and focus is never trapped at an edge. During
  playback, left and right seek only when focus is not on a discrete control.
- **A vertical list of options owns its own axis, and its rows share one width.**
  Spatial navigation ranks candidates by centre-to-centre distance with the
  horizontal axis weighted `2.25`, which is hostile to stacked full-width rows:
  a wide row's centre sits far right of every narrow control in the same column,
  so a distant link wins the press and the list is skipped entirely. Three rules
  follow, and they are not tuning — they were measured on the film page before
  they were written down (decision 47):
  - a list of options handles up and down itself, in reading order, and does
    **not** consume the press at either edge, so leaving the list still falls
    through to the page;
  - every option in one list keeps the same width, or the shortest label sits
    furthest left and gets skipped;
  - option rows stay near `26rem` rather than full-bleed, and the list sits
    close to the action it serves rather than at the far end of the page.

  This applies to any screen with a list of choices — files, episodes, devices —
  not only to the film page where it was found.
- Fine-pointer desktop users may reveal scroll chevrons by hovering a home row.
  They are supplemental controls with `tabindex="-1"` and sit outside the row's
  directional-navigation handler, so they do not add stops to either Tab or the
  D-pad graph. Touch layouts do not render them.
- `--faint` and `--accent-dim` fail normal-text contrast. They are for disabled
  states, decorative rules and borders. This is written down because it is the
  rule most likely to be broken by accident.
- The interface defaults to French and also ships in English. `<html lang>`,
  visible copy, accessible names and locale-sensitive formatters follow the
  active browser language immediately; none of them waits for a reload.

## 10. Fonts, as shipped

**Cinzel Variable** for the display register and **Jost Variable** for the label
register. Both SIL Open Font License 1.1, which is GPL-compatible and imposes
nothing on the rest of the project. Prose has no shipped face at all — see §4.

They are self-hosted, not linked. The packages come from npm
(`@fontsource-variable/*`), but their stylesheets are deliberately not imported:
those ship Cyrillic and Latin-Extended alongside Latin, and while a browser
would only download the subset it needs, every subset would still be embedded in
the binary. `web/src/app.css` declares both `@font-face` rules by hand against
the Latin files only, which covers both shipped interface languages completely.

Cost: two WOFF2 files, 52 KB together, hashed into `_app/immutable/` and so
covered by the immutable cache header the Go server already sets. They are not
gzipped on the way out — WOFF2 carries its own compression and doing it twice
only spends CPU. `font/ttf` *is* in the compressible list for anything dropped
into `static/`, and a test pins the WOFF exclusion.

### What was tried in between, and why it was withdrawn

v2.2.0 shipped **Augustus** and **Dalek Pinpoint Bold**, and §10 said plainly
that neither carried an open licence. Checked properly afterwards, both were
worse than unclear:

- **Dalek Pinpoint** is K-Type's, and K-Type's free fonts are for "personal use
  among friends and family". Webfont use requires a paid licence, embedding in
  a software product requires the Enterprise tier, and every tier forbids giving
  the file to others — which a public repository does by existing.
- **Augustus** is not orphaned. It is Paulo W's, published by Intellecta Design,
  a commercial foundry that sells it; the copy circulating on free-font sites
  carries no licence and only an "ALLTYPE" conversion stamp.

Neither could ship in a GPL-3.0 repository that publishes binaries, so the
identity was kept and the files were changed. Cinzel is the same Roman
inscriptional brief as Augustus and does it better — real small capitals where
Augustus had no case distinction at all, and a working question mark where
Augustus drew a figure 9. Jost gives the label register the geometric contrast
Dalek gave it, with real weights instead of one.

Three other OFL candidates were rendered and refused: **Marcellus SC** and
**Cormorant SC** are Roman small-caps faces too close to Cinzel to give the two
registers any contrast, and **Julius Sans One** is too light to hold a label at
three metres.

### Verify a face, do not look at one

The rule that came out of shipping a font nobody had seen: **a screenshot does
not verify a web font.** Georgia sits behind the display stack and Georgia is a
good serif, so a refused face renders a page that looks entirely correct —
including the specimen sheet a face is chosen from. `document.fonts.check()` is
one line and is the only thing that answers the question. The frontend guard in
`web/tests/` asserts it on every page.

**There is no CDN request anywhere in the application**, which is not a
preference but the project's no-external-calls rule.

The system stacks are still listed behind both faces in §4 and still work — they
are what renders during the swap.

## 11. Implementation status

The v1 feature set was published as `v1.0.0`. The current visual system is
implemented in `web/src/app.css` and the Svelte components; the provisional M0
palette (`--color-ink: #0b0b0f`, `--color-helios: #f5a623`) is gone.

The running interface now implements the hero-and-rows library, detail page,
player, onboarding, empty state and settings chrome described above. Cards keep
the §6 exemption: their artwork is untouched, and gold appears only for progress
and interaction state. Their ratio is `16 / 9` as of §6.1 — earlier revisions of
this paragraph said `2/3`, which was true until v1.3.0.

Colour, type scale, tracking, target sizes and directional row navigation are
verified against a running build rather than inferred from source. The
three-metre pass uses 52–56px controls and 192–224px cards at TV widths, while
the responsive floor remains usable without horizontal page overflow.

`scripts/contrast.mjs` guards the documented colour ratios. The two photographic
chrome assets are cropped to their rendered ratio, resized to 1920×1080 and
encoded as WebP before the frontend build; the licence-checked source pack stays
outside `web-dist/`.

**The shared layer, added after `v1.1.0`.** Three passes of feature work had each
built their own plumbing, and the document had drifted ahead of the code in two
places. That was closed:

- `--duration-fast/base/slow` are real tokens; the thirty-two literals are gone.
- `--nav-offset`, `.page-body` and `.page-body--hero` own the top of every page,
  which four screens used to guess independently.
- `.page-title` and `.page-title--feature` own page headings, which three
  screens used to override with `!important`.
- The `--space-1…48` ramp this document declared but never shipped is gone from
  §5, replaced by a description of what the code actually does.
- `--texture-glow` and `--texture-vignette` were declared with no consumer and
  are now recipes in §7 rather than dead tokens.
- The §8 entrance exists at last, as `.enter`, with the fill-mode and
  looping-animation traps of the reduced-motion guard written down beside it.
- The nav says where you are, keyed off `aria-current` so the fact serves the
  eye and a screen reader at once; `+error.svelte` means a mistyped address gets
  a catalogued screen in the active language instead of SvelteKit's untouched
  built-in one.

**The home screen, rebuilt after that.** It is now four short rows — continue
watching, recently added, best rated, tonight's suggestion — instead of a hero
and eight genre rows. The hero shows the film you left, with how far in you are
and what is left, and its button opens the player rather than a detail page. The
grid underneath was unchanged at the time and still exempt under §6: the only
gold at rest is the 3px progress rule and no card title takes the display serif.
Verified against the 274-film library rather than inferred. The card shape itself
changed later — see below.

**The interface language layer.** French is the default
and English ships as a second complete catalogue. The choice is local to each
browser, and changing it updates copy, accessible names, `<html lang>` and
formatters without replacing the current page or losing focus. Catalogue parity
is a frontend build check. TMDB titles, synopses, genres and credits already
cached as `fr-FR` do not change and are not fetched again: they are film data,
not interface chrome.

**The record on a detail page, added post-v2.** A film and a series now show the
whole TMDB record rather than a synopsis and a cast list of names — see decision
85 for why none of it costs a request. Four things it settles about this document:

- A **tagline** is prose, so it takes `--font-ui` italic at `calc(var(--text-body)
  * 1.15)` and not the display serif. §4 reserves Cinzel for titles; a marketing
  line in small capitals reads as a second heading arguing with the first.
- A **certificate** (`.certificate`) is a box on `--color-muted` with
  `--color-parchment` type, never the accent: the five-use accent budget on a film
  page is already spent on the rating beside it. Its tracking is `0.08em` rather
  than a `.label`'s `0.18em`, because "12" tracked out reads as "1 2".
- **Cast portraits** (`.cast-list`, `.cast-portrait`) are 2:3 at 3.25rem, two
  columns, three at the 100rem step where they grow to 5rem. Missing portraits get
  the same composed stand-in a card uses for missing artwork, so an empty frame
  among nine photographs cannot read as a fault.
- A **saga row** is the existing row component with its heading overridden, not a
  second kind of strip. It sits outside the detail column so its cards land on the
  same page gutter as every other row: measured at 96px on both at 1920.


**File badges, added post-v2.** `.badges` / `.badge` on the film detail page:
`4K`, `HDR`, `DOLBY VISION`, `TRUEHD`, `ATMOS`, `7.1`, in the label register on
`--surface` inside a `--line` hairline, text in `--muted`. That measures
**5.12:1** here rather than the 5.42:1 the token table states, because the table
measures against the page background and a badge sits on a panel — still clear of
the 4.5 AA floor, and `scripts/contrast.mjs` now carries a second section for
text set on a surface, since this was the first thing to need one.

Built entirely from tokens that already existed, and deliberately **not** in
gold — §3 allows five uses of `--accent` per screen and the film page has already
spent them on the rating and the play affordance. A sixth and seventh in
gold would also make the loudest thing on a film page its audio codec, which is
the wrong answer to "what is this film".

They sit under the metadata line rather than in it: the year, the runtime and the
director describe a *film*, while these describe one *encode* of it, and a
household holding two files of the same title needs to see which one is on
screen. Tracking is the full `--text-label` 0.18em, unlike `.certificate`, whose
tighter 0.08em exists because "12" at 0.18em reads as "1 2" — an argument that
does not apply to a word.

Measured in a running build at 375px: six badges wrap to two rows with no page
overflow. A file that has not been inspected renders no row at all, and no gap
where one would have been.

**The rating carries its scale.** `.rating` on the film page and the hero: the
figure in `--font-display` at 1.25rem in `--accent`, the `/ 10` in the label
register in `--muted`, baseline-aligned. It was a bare gold number at the end of
a row of a year, a runtime and a certificate, so it read as one more duration and
spent an accent use with nothing to justify it. The scale is what makes it a
score, and it stays the quiet half so the gold still lands on the figure alone.
The figure moves to the display face because §4 keeps that register for the
page's voice, and a score is nearer a title than a caption.
## 12. Public presentation site

The application helps somebody choose a film they already own. The public site
has a different first job: prove what Theia is, make the cost and limits legible,
then let somebody try it. It keeps the identity above and changes the
composition, not the brand.

### 12.1 The first screen contains proof

The hero places three things in one sequence: the promise, a product proof and
the download station. A display title is still allowed, but it no longer earns a
half-empty viewport merely by being large. The player is the one flourish on the
page and takes the visual weight the old negative space held.

A player proof has two valid forms:

1. a capture of the real player, with the media source and its right to be used
   recorded beside the asset;
2. an interactive reconstruction of the player's chrome, explicitly labelled
   as a demonstration.

The current hero combines them honestly: a real capture produced from the
repository-authored demo media, with demonstration controls above it. A mockup
must never be worded or framed as a capture. Replacing the media later changes
the asset and provenance record, not this rule.

### 12.2 Three moments are the ceiling

The public site shows at most three visual product moments: discover the local
library, watch through the player, and reach an authorised device remotely. It
does not reproduce the application's navigation or turn every feature into a
section. Detailed capability lists and the exhaustive competitor table live in
the README, where somebody asking for detail can choose to read them.

Screenshots keep their application colours and aspect ratio. No page-wide grade,
decorative image or second flourish competes with the player. Missing imagery is
replaced by a structured fact panel, not a stock photograph.

### 12.3 Download station

Download is a two-step choice:

1. operating system: Windows, macOS or Linux;
2. architecture: x64/ARM64 or Intel/Apple Silicon.

The system controls are 54px pills. Architecture rows are at least 62px high and
show the exact file facts available at build time. JavaScript may reveal one
system panel, but it never guesses architecture or starts a download. Without
JavaScript, the three panels and six links remain readable and usable.

Signing and launch warnings appear before the architecture links, not after the
download has surprised somebody. Version, date, size and SHA-256 are build-time
metadata; a missing fact is omitted. Links themselves use GitHub's
`releases/latest/download/` contract and need no runtime API.

### 12.4 Copy and emphasis

- Consequence precedes jargon: « un appareil autorisé » before « WireGuard ».
- Three differences and three limits are enough. Limits appear before the final
  invitation to download.
- No superlative claims that depend on another platform's current product.
- Gold remains reserved for the mark's rule, focus, and playback progress. A
  selected option uses `--raised` and `--fg`, not another gold badge.
- The same French/English template, visible FAQ and accessible names are a
  publishing contract, not optional polish.

The site reflows at 1100px, 700px and the 390px floor. Controls remain at least
44px, the range hit area remains 44px even though its painted rule is 4px, and
the track popover stays inside the player. Motion uses §8 and disappears under
`prefers-reduced-motion`; information does not.

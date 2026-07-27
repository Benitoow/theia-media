# Decisions

The founding spec fixed the stack and the scope. This file records the calls
made on top of it, in the order they came up, so that neither a future
contributor nor a future AI session has to re-litigate them.

---

## 1. Playback: remux video, transcode audio when needed

**Decided.** Direct play whenever the browser can read the file. When it cannot,
remux the container and re-encode *audio only* if the track is AC3, DTS or
TrueHD. Full video transcoding stays out of v1.

The spec says "remux", which taken literally means changing the container and
copying every stream untouched. That would ship a product where a perfectly
ordinary H.264 + AC3 MKV plays with a picture and no sound — a technical success
that is a product failure. Audio re-encoding to AAC costs about one CPU core;
video transcoding is the real furnace, and that is what remains out of scope.

## 2. TV series are out of v1

**Decided.** Films only. Series get their own milestone immediately after M8.

The spec mentioned series in passing without scoping them. They are not a small
addition: a three-level model (series → season → episode), `SxxExx` parsing, the
TMDB TV endpoints rather than the movie ones, a different detail page, and a
"continue watching" row that has to roll on to the next episode. Bolting that
onto v1 would have blurred every milestone it touched.

## 3. Subtitles: text tracks only

**Decided.** External `.srt` files, plus SRT and ASS tracks extracted from
containers with ffmpeg. Image-based subtitles — PGS, VobSub — are out.

Extracting a text track is nearly free given ffmpeg is already a dependency.
Image subtitles cannot be extracted into anything a browser renders; they have
to be burned into the video, which drags in exactly the full-transcode pipeline
decision 1 keeps out.

## 4. The TMDB key ships with the binary

**Decided.** A key is compiled in. `tmdb_api_key` in the settings overrides it
for anyone who would rather spend their own quota.

Requiring the user to register with TMDB, generate a key and paste it in before
a single poster appears puts the project's worst friction in front of its best
first impression. "Zero configuration" cannot mean "zero configuration except
for the one step that makes it look like a media server." Jellyfin ships a key
the same way.

## 5. The QR code is the contract, mDNS is a bonus

**Decided.** Reaching the server must never depend on `theia.local` resolving.

Announcing over mDNS and *resolving* a `.local` name are different things.
Windows and Apple devices resolve it; many Android builds and smart-TV browsers
do not. On top of that, a second mDNS responder on the machine can quietly take
the port — this actually happened during M0 development on the dev machine, and
`hashicorp/mdns` reports it as success, so `internal/discovery` probes the
sockets itself and warns when only one IP family came up.

The reliable path is the numeric address, and from M6 the QR code shown at first
launch. mDNS is the pleasant shortcut layered on top.

**Related:** the interface is served over plain HTTP. That means it cannot be
installed as a PWA, which needs HTTPS. A self-signed certificate on a LAN trades
that for a browser warning screen on every device, which is worse. Responsive
web, not installable, is the v1 answer.

## 6. No authentication at all

**Decided.** Anyone who can reach the port can do anything, settings included.

A deliberate scope decision for single-user home use. It is called out at the
top of the README because the first support request will come from someone who
forwarded the port on their router.

Worth knowing that this is the hardest decision here to reverse: adding accounts
later touches every route, the player and the settings page at once.

## 7. Run it by hand; installing a service is opt-in

**Decided.** The binary runs in the foreground. A `theia install-service`
command will exist for people who want it, and will be the only thing that ever
asks for elevation.

Installing a boot service needs administrator rights on every OS. Asking for
that on first launch contradicts "plug it in and it works" before the user has
seen a single frame.

## 7b. The TMDB key never lives in the repository

**Decided.** Decision 4 says a key ships with the binary. That cannot mean a
constant in a source file: this repository is public, and a key committed here
is a key published.

Two separate mechanisms, easy to confuse:

- **Development.** A `config.local.json` in the working directory overrides the
  real configuration at runtime. It is in `.gitignore`, `Save` never writes its
  values back into the persisted config, and `Config` implements
  `slog.LogValuer` so that logging one prints `eyJh…OM4w` rather than the key.
- **Distribution.** The shipped default has to be injected at build time from a
  CI secret — `-ldflags "-X ...=$TMDB_KEY"` — not committed. That work belongs
  with M2, when there is finally something to call TMDB about.

## 7c. Reconciliation counts scans, not seconds

**Decided.** Every scan bumps an integer in `scan_generation` and stamps the
rows it touches with the result. Nothing about deciding what changed reads the
clock.

The first implementation used timestamps, and two tests caught it immediately:
two scans within the same second are indistinguishable, so an update was
reported as an insert and a file that had genuinely disappeared was never
pruned. Seconds are not fine-grained enough for a scan of a small library, and
an NTP correction between two scans breaks the ordering outright. A counter has
neither failure mode.

Timestamps are still stored, but nothing branches on them — they are there for
people to read.

## 9. Metadata is cached, but never frozen

**Decided.** A TMDB record is considered fresh for **90 days**. A film TMDB did
not recognise is retried after **7 days**. Both are re-fetched silently by the
next scan; nothing asks the user to press anything.

The two lifetimes differ because the failures differ. A TMDB record changes
slowly — synopses get rewritten, a poster gets replaced, a missing runtime gets
filled in — so re-reading it more than a few times a year is pure waste. A
*miss*, though, is usually our fault rather than TMDB's: a mangled filename, or
a title parsed badly enough that a slightly different guess would have landed.
Making somebody wait three months to see a poster appear after they fixed a
filename would feel broken, so misses come back around within the week.

There is a third trigger that beats both: **renaming a file invalidates its
metadata immediately.** Renaming is how a user corrects a bad match, and the
upsert resets the row to `pending` whenever the parsed title or year changes.
Waiting out ninety days after fixing a name would make the fix look like it did
nothing.

Images are separate. TMDB image paths are content-addressed — a given path
always returns the same picture — so a cached file never expires. What can
change is which path a film points at, and that is covered by the metadata
lifetime above. Images are also fetched lazily, on first request rather than
during a scan: a first scan of a large library would otherwise download
thousands of posters nobody has scrolled to yet.

The metadata pass is capped at 200 lookups per scan for the same reason. A large
library fills in over the first few scans, which is visible progress, rather
than sitting silent through several thousand requests.

## 10. TMDB attribution

**Required, not chosen.** TMDB's terms of use oblige any application using their
API to display:

> This product uses the TMDB API but is not endorsed or certified by TMDB.

It is served by `/api/settings` and shown at the bottom of the home screen. The
wording is theirs and is not ours to paraphrase or translate. It stays visible
wherever the metadata section ends up in later milestones.

Their logo is not shipped: no copy of it has had its licence checked, and
nothing in this repository fetches assets from the web. Text alone satisfies the
requirement.

## 8. Logistics

- **Repository:** public, `theia-media`, from M0.
- **Language:** code, comments and internal error messages in English, for
  contributors. User interface in French, isolated in
  `web/src/lib/strings.js` so a second language is a new file rather than a
  rewrite.
- **Updater:** built from the documented GitHub Releases pattern — check the
  API, download, swap atomically, restart — rather than ported from Hermes,
  whose source was not available. The Windows case needs care: a running `.exe`
  cannot overwrite itself, so it takes a small relay process that waits for the
  parent to exit, replaces the file and relaunches.

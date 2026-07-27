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

## 11. TMDB search is ranked by relevance, not popularity

**Found the hard way, on a 268-file library.** `search/movie` returns results in
TMDB's own text-relevance order. The `popularity` field exists but does not
order anything.

Searching `The Handmaiden` with `primary_release_year=2016` returns:

| popularity | title |
|---|---|
| 0.6 | Making of The Handmaiden |
| 19.1 | Mademoiselle |

Taking the first result catalogues a library of documentaries *about* films
rather than the films. Two of 268 test files landed that way, and the same
happened to *Crouching Tiger, Hidden Dragon*.

The exact-title check that was supposed to catch this failed for a second
reason: the request asks for `fr-FR`, so `title` comes back translated. A file
named "The Handmaiden" matches neither the French title ("Mademoiselle") nor the
Korean original. Both safeguards failed at once.

`internal/tmdb.pick` now checks `title` **and** `original_title` for an exact
match, and falls back to the **most popular** result rather than the first. A
film is reliably more popular than the making-of about it.

## 12. Release group names are not sample markers

**Also found on the large library.** The scanner's sample-file word list
contained `rarbg`, which silently deleted every film whose filename carried that
release group — nine real films out of 277.

The junk it was meant to catch (`RARBG.txt` and similar) is not a video file and
never reaches that function, so the rule could only ever do harm. Only words
that describe *the file* belong in that list, never words that describe who
packaged it.

## 13. The hero is chosen by rating, not just recency

On a first scan every row is inserted within the same second, so `added_at` is
identical across the whole library and "most recently added" collapses to
insertion order. On the test library that put a mis-identified junk file on the
front page.

The hero is now ordered by recency *then* rating, with a floor of 6.0 and a
requirement for both a backdrop and a synopsis — a hero without artwork is a
hole, and one without text is a title floating in the dark. On a first scan,
where recency says nothing, the best-rated film wins; afterwards a genuinely new
film takes the slot if it is worth showing. The floor drops to zero if nothing
clears it, because a modest library still deserves a front page.

This is the same failure mode as the scan-generation bug in decision 7c:
second-resolution timestamps cannot order events that happen inside one second.

## 14. Where ffmpeg comes from

**Decided: `eugeneware/ffmpeg-static`, tag `b6.1.1`, on GitHub Releases.**

The obvious candidates were the ones every guide names: gyan.dev for Windows,
johnvansickle.com for Linux, evermeet.cx for macOS. Two of those three are not
GitHub, and this project is allowed to contact exactly two hosts — TMDB and
GitHub Releases. Using them would have meant widening that rule for a
convenience.

`BtbN/FFmpeg-Builds` is on GitHub but publishes under the tag `latest`, which is
republished in place. A pinned checksum against a moving target fails the day
upstream rebuilds, which is the worst kind of failure: it works for months and
then breaks for everyone at once.

`eugeneware/ffmpeg-static` tags every build, ships bare binaries rather than
archives — no zip or tar.xz handling — and covers all six OS/architecture pairs
Theia builds for. Its Windows binary identifies itself as
`6.1.1-essentials_build-www.gyan.dev`, so it is gyan.dev's build after all,
served from a host the project is already allowed to talk to.

There is no upstream Windows ARM64 build. That platform gets the x64 binary,
which Windows runs under emulation — better than leaving it unable to remux.

**Checksums come from the GitHub release API**, which reports a `digest` for
every asset. They were pinned without downloading 450 MB and without trusting a
separately published checksum file.

**Nothing is executed before its digest matches.** The download goes to a
temporary file with no executable bit; the bit is set and the file moved into
place only after the hash is verified. A binary fetched over the network and
then run as a subprocess is exactly the thing that has to be checked.

## 15. ffmpeg arrives on first need, not first launch

**Decided.** A library of MP4s never downloads it.

This is why the "how will this play" endpoint answers from the container alone
rather than probing: probing means running ffmpeg, and running it means fetching
80 MB. The cost is that an MP4 hiding an exotic codec is attempted directly and
fails in the browser — the player catches that and retries as a remux, so the
worst case is a moment's delay rather than a wrong answer.

Only the remux endpoint downloads. Verified on the running binary: asking for
stream info left no `bin/` directory behind.

## 16. Three playback paths

| Container | Video | Audio | What happens |
|---|---|---|---|
| MP4, M4V, WebM | browser-friendly | browser-friendly | **Direct play**, byte for byte, with range requests |
| anything else | H.264, VP8/9, AV1 | AAC, MP3, Opus, Vorbis, FLAC | **Remux**, both streams copied |
| anything else | same | AC3, DTS, TrueHD… | **Remux**, video copied, audio re-encoded to AAC |
| any | MPEG-2, VC-1, … | any | **Refused**, with the reason |

The third row is decision 1 in practice: an ordinary H.264 + AC3 MKV would
otherwise remux into a film with a picture and no sound. The fourth is decision
1's other half — re-encoding video is the furnace v1 refuses to light, and
saying so beats pinning a CPU for two hours.

HEVC sits between the rows: it copies into MP4 and plays in Safari but generally
not in Chrome. It is attempted and flagged, rather than refused outright.

**Seeking.** Direct play seeks natively — `http.ServeContent` answers byte
ranges and the browser's scrub bar just works. The remux path is a pipe with no
length, so it seeks by restarting ffmpeg at a timestamp (`?t=`). `-ss` goes
*before* `-i`, which seeks by keyframe without decoding everything in between;
the cost is landing on the keyframe before the requested time. Measured on a
20-second clip with one keyframe per second: asking for t=12 yielded 9.02
seconds remaining rather than 8. That granularity is inherent to stream copy.
The endpoint exists and works; the interface does not expose it yet, and M5's
resume will be its first real user.

**No ffprobe.** The pinned upstream ships only `ffmpeg`. Running it with an
input and no output prints the stream table and exits non-zero on purpose, so
the exit code is ignored and the table is parsed instead.

## 17. When a film counts as watched

**Decided.** Finished when the remaining time is under **two minutes**, or under
**five per cent** of the running time — *whichever sits closer to the end*.

The first version combined the two the other way round, taking whichever
triggered first, and a test caught what that meant: a ten-minute short was
called finished at eight minutes, because two minutes is a fifth of it. Taking
the stricter threshold gives the behaviour both cases want — a two-hour film
clears when the credits roll, a ten-minute one needs its last thirty seconds,
and a three-hour epic does not demand nine minutes of credits.

**Finished is recomputed on every report, never latched.** Starting a film again
clears it and the film returns to the continue-watching row, which is what
anybody rewatching something expects.

**Nothing under thirty seconds is remembered.** Opening a film to look at the
poster and closing it again must not fill the row. One consequence worth
knowing: a clip shorter than about ten minutes can never appear in
continue-watching, since thirty seconds is already past its finishing
threshold. Irrelevant for films, and it surfaced while testing with short
clips rather than in use.

## 18. Seeking a stream that has no length

A remuxed stream is a pipe: no Content-Length, no byte ranges, and a native
scrub bar with nothing to draw. That is why the player has its own controls
rather than the browser's.

Two clocks are in play. The video element always starts at zero, whatever
timestamp ffmpeg actually began at, so the player keeps an **offset** and shows
`offset + element.currentTime`. Seeking means restarting ffmpeg at a new
timestamp and moving the offset with it.

Getting that wrong is subtle and was caught in the browser rather than by a
test: setting the source without moving the offset left the displayed clock
reading the old position while the stream played the new one — and saved that
wrong number as the resume point.

The duration comes from the server, since the stream cannot supply it: the
ffmpeg probe records it on first playback, with TMDB's runtime as a fallback
until then.

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

# Decisions

The [founding spec](spec-fondatrice.md) fixed the stack and the scope. This file
records the calls made on top of it, in the order they came up, so that neither
a future contributor nor a future AI session has to re-litigate them.

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

## 19. The QR code encodes an IP address, never the mDNS name

**Settled by measurement, not by argument.** `theia.local` does not resolve on
Android — tested on a real phone, five milestones after decision 5 predicted it.
It is a platform limitation rather than a Theia bug, and it is exactly why
decision 5 made the numeric address the contract and mDNS the convenience.

So the QR code encodes `http://<ip>:<port>`. The mDNS name appears underneath as
a second line, with the Android caveat written out rather than left for somebody
to discover.

**Choosing which IP is the hard part.** A machine running Docker, WSL, Hyper-V,
VirtualBox or a VPN has several private IPv4 addresses, all equally valid to a
naive check, and only one that a phone on the sofa can reach. A QR code pointing
at a Hyper-V switch is worse than no QR code at all: it scans perfectly and
leads nowhere.

There is no portable way in pure Go to ask which interface carries the default
route, so `discovery.Candidates` ranks by what can be observed — real adapters
before virtual ones, private addresses before public — using a list of adapter
name markers. Ranking cannot be certain, so the other addresses are offered in
the interface with their interface names, and the screen says what to do if the
code leads nowhere.

## 20. The welcome screen appears once, and never for an existing install

Dismissal is recorded in a new `app_state` table rather than in `config.json`,
because that file belongs to the user and nothing in it should change by itself.
The updater will want the same table for its last-check timestamp.

There is one extra rule at startup: **a database that already contains films is
marked as onboarded without ever showing the screen.** Somebody upgrading into
the version that added onboarding has already pointed Theia at a folder and
watched it scan; a welcome screen would be telling them something they did years
ago.

**Verifying the code without a camera.** The encoder is a well-used library and
is taken on trust. The SVG rendering is code written here, and that is where a
bug would live — inverted modules, an off-by-one in the run merging, a missing
quiet zone. So the test parses the generated SVG back into a module matrix and
compares it against the encoder's own bitmap, module by module. A symbol that
survives that scans, or the encoder is wrong.

## 21. Updating: everything reversible happens first

**The governing rule is that a failed update leaves a working installation.**
The order of operations is the whole design, and it is deliberate:

1. Fetch the release, find the asset for this platform, read its SHA-256 from
   the GitHub API. **No digest, no update** — "we could not check it" is not a
   reason to install something.
2. Download to a temporary file *beside the executable* — a rename across
   filesystems is a copy, and a copy is not atomic — with no executable bit.
3. Verify the digest. Mismatch: delete, stop, nothing has been touched.
4. Set the executable bit and **run the downloaded binary with `-version`**.
5. Only now rename the current binary aside and the new one into place.

Step 4 is the one that is easy to leave out. A correct checksum proves the bytes
match what was published; it says nothing about whether that file runs *here* —
wrong architecture, a release built from a broken commit. Finding that out
before the swap is the difference between a failed update and a dead
installation.

If the second rename fails, the first is undone. An installation with no
executable at all is the one outcome that must never happen.

## 22. No relay process is needed on Windows

**Measured, not assumed.** The plan called for a helper binary because a running
`.exe` cannot replace itself. Testing what Windows actually forbids:

| Operation on a running executable | Result |
|---|---|
| Rename it | **allowed** |
| Write a new file at the freed path | **allowed** |
| Delete it | refused while the process lives |
| Delete it after the process exits | allowed |

Only deletion is blocked. So the swap is two renames, and the outgoing binary
sits as `theia.exe.old` until the next start clears it away — which also makes
it the manual way back if a release turns out to be bad. Verified across two
chained updates: `.old` appears at the swap, is byte-identical to the previous
version, and is gone after the restart.

Fewer moving parts on the most dangerous path in the project is worth more than
following the original plan.

## 23. Updates never interrupt playback, and never happen unannounced

Checking is automatic — at startup and every six hours. **Installing is not.**
A media server that restarts itself in the middle of an evening is one nobody
trusts, so the interface says an update is available and waits to be told.

Even then it can refuse. `internal/activity` tracks streaming, and `Apply`
re-checks at the moment of installing rather than trusting the button press,
because another device may have started watching since. The two playback paths
look completely different from the server's side — a remux is one request held
open for the film's length, direct play is a burst of short range requests with
long gaps — so the tracker combines in-flight requests with a ninety-second
memory of the last one. Neither signal alone describes both.

## 24. A build that cannot name itself never updates

Any version string that is not semver — `dev`, a bare commit hash — puts the
updater in an unsupported state permanently. There is nothing to compare a
release against, and guessing would overwrite a developer's working binary with
whatever was published last.

**The asset naming contract**, which M8's release workflow has to honour
exactly: `theia-<goos>-<goarch>`, with `.exe` appended on Windows. A mismatch
produces an updater that silently never finds its own binary; `TestAssetName`
pins it so the two cannot drift apart unnoticed.

## 25. The server never writes what the user reads

**Decided in M8, after finding the rule broken in three places.** Anything shown
in the interface is a *code* the server sends and the interface words. The
server's own error strings are for the log and for anybody reading the API
directly.

The rule exists because the settings page was showing people this:

> `D:\Films is unreadable, is the drive connected? (GetFileAttributesEx D:\Films: Le fichier spécifié est introuvable.)`

A syscall name, wrapped in English, in a French interface, telling somebody
whose USB drive is unplugged nothing they can act on. Scan problems are now
`{kind, path}`, update failures carry a `reason` code, and the interface owns
every sentence. The path still travels, because it is the one technical detail
that helps: it says *which* drive.

## 26. Three settings, and the fourth one that is not on the page

The spec allows exactly three: watched folders, port, TMDB key. The settings
page has those three and nothing else, editable, so nobody has to find
`config.json`.

`hostname` is a fourth field in the configuration file. It is the name announced
over mDNS, and it exists because announcing *something* is unavoidable and
hard-coding "theia" would break anybody running two instances. It is
deliberately not on the settings page: an advanced knob for a file, not a
setting for a person.

Two behaviours worth writing down. A folder that does not exist is **saved
anyway** and reported — configuring a drive before plugging it in is a normal
thing to do, and refusing would be wrong. And a TMDB key is never sent to the
browser, so an empty field means "leave it alone" rather than "clear it";
clearing is possible, but only deliberately.

## 27. A TV interface has to work with only a D-pad

**Added after v1.0.0.** The founding scope required a television-friendly web
interface, but did not define remote-control navigation. Relying on Tab or on a
browser's optional spatial-navigation implementation made "TV-first" a visual
claim rather than an interaction contract.

The contract is now explicit:

1. The first arrow press enters at a declared primary action when the screen
   has one, otherwise at its first visible control.
2. Up and down use geometry, preferring controls whose rectangles overlap on
   the perpendicular axis before considering diagonal candidates.
3. Left and right inside a film row move to the adjacent poster
   deterministically and scroll it into view.
4. Tab remains available. Arrow handling never steals input from text fields,
   selects, editable content, video elements or the playback slider.
5. An open player is a hard navigation boundary: focus moves into it, Tab wraps
   inside it, directional navigation is scoped to it, and closing restores
   focus to the button that opened it.

The screen-level implementation belongs in `web/src/routes/+layout.svelte`
because navigation has to cross the hero, global nav and separate rows.
`Row.svelte` keeps the stricter horizontal rule because "next poster" is more
predictable than a geometric guess. `Player.svelte` owns playback semantics:
left and right seek by ten seconds only when focus is not on a discrete
control, so one key press cannot both move focus and seek.

This was verified against the embedded production build, not only the source:
at 1280×720 a D-pad can enter the hero, reach the first row, move across and
between rows, open the player, reach every control and close it without losing
focus. A slider ArrowRight advances exactly ten seconds rather than firing both
the slider and window handlers. The same build was checked at 1600×900 and
390×844 with no horizontal page overflow.

## 28. Spacing stays Tailwind's; no parallel token ramp

`docs/design-system.md` §5 declared a `--space-1…48` ramp from the very first
draft. It was never implemented. The code had always used Tailwind's own 4px
scale and fluid `clamp()`, and nobody had noticed the document was describing a
system that did not exist.

Two ways out: implement the ramp everywhere, or delete it and describe reality.

Reality won. Tailwind already ships a coherent 4px scale; adding `--space-*` on
top would mean two ladders for one job, kept in step by hand, with no way to
tell which one a given number came from. The ramp bought nothing that
`mb-14`/`gap-4` did not already buy, and cost a synchronisation duty forever.

What *was* worth tokenising is the page frame, because those are contracts
between screens rather than matters of taste — and unlike spacing, they had
visibly drifted. `--nav-offset`, `.page-body` and `.page-title` were added for
exactly that reason: four screens had each guessed their own top padding and
three had overridden the heading style with `!important`. The rule that
separates the two cases: **tokenise what two screens must agree on, not what one
screen chooses.**

## 29. The home screen is a personal surface, not a second catalogue

Until now the home screen carried a hero, continue-watching, recently added and
then **a row per genre** — eight of them, twenty films each. That made sense when
it was the only way to reach anything. It stopped making sense the moment `/films`
arrived with search across title, director, genre and year, five sorts and two
filters over the whole library.

Two screens were answering the same question, and the worse one was the front
door. So the home screen now answers a narrower one: *what were you watching,
what is new, and what should you put on tonight.* The genre rows are gone; genre
browsing belongs to the page built for it, and the rows that remain are short —
twelve rather than twenty — with a link through to `/films` pre-filtered to match.

Three consequences worth stating:

- **The hero is no longer a fixed slot.** If a film is in progress it takes the
  hero, with its progress, what is left to watch, and a button that opens the
  player rather than the detail page. Only when nothing is under way does the
  recent-and-well-rated pick appear. Somebody who stopped forty minutes into a
  film last night wants that film, not a recommendation.
- **Rows carry a code, not a title.** `{kind: "continue"}`, never
  `{title: "Continuer à regarder"}`. This was a live breach of decision 25 that
  predated it, and it meant a second interface language would have had to be
  added in Go.
- **`Store.Genres` and `Store.ByGenre` were deleted** rather than left behind. A
  query nothing calls is the same debt as a design token nothing reads.

## 30. Tonight's suggestion is shuffled in Go, not in SQL

The "au hasard ce soir" row must be stable for the evening: `ORDER BY RANDOM()`
reshuffles on every page load, which turns a suggestion into a slot machine —
you reload until you like the answer, and the row means nothing.

Seeding the date into a SQL `ORDER BY` took three attempts, and the first two
failed in ways that read as correct:

1. `(id * constant + seed) % p` — adding the seed shifts every row equally, so
   the order changes only for a row whose value happens to cross the modulus.
   With a dozen rows that is almost never: the row showed the same films daily.
2. `(id * seed) % p` — a date seed is about 2e7, so `id * seed` for a few hundred
   films never reaches a 2.1e9 modulus. Nothing wraps, and the result is plain
   id order.
3. `(id * k) % p` with `k` spread across the full range — this passed every test
   written for it, and was still wrong. Sorting by a modular multiply and taking
   the first twelve selects ids at a **fixed stride**. On the real library the
   row came back `257, 234, 211, 188 …`: every twenty-third film, every time.

That third one is the interesting failure. It was stable, it turned over daily,
it passed. It was only caught by printing the ids and looking at them.

The row is now built in Go: fetch the eligible ids, shuffle them with a
date-seeded `math/rand`, fetch the chosen rows. Two queries instead of one, for
a few hundred integers, in exchange for a shuffle whose correctness is obvious
rather than argued. `TestTonightIsNotAnArithmeticProgression` guards it.

## 31. Household profiles separate progress; they do not create accounts

The founding scope excluded multi-user accounts and permissions. That remains
true. The post-v1 profile selector adds a much smaller thing: a name, an
optional local photo and an independent playback history. There is no password,
PIN, role, token or ownership rule. Anyone who can reach Theia on the LAN may
select and edit every profile, exactly as decision 6 already allows them to edit
every setting.

The selected profile belongs to the browser, not to the server. Its integer id
is kept in `localStorage` and sent as `X-Theia-Profile` on requests that read or
write personal playback state. This avoids the broken alternative where
changing profile on the television would also change it on a laptop mid-film.
The header is a selector, not an authentication credential. An absent header
uses the default profile for v1.2 client compatibility; a malformed or stale
explicit id fails instead of silently contaminating the default history.

Progress moved conceptually out of `movies` into
`playback_progress(profile_id, movie_id)`. File duration stays on `movies`
because it describes the media, not the viewer. Migration 0005 creates one
language-neutral default profile (`name IS NULL`) and copies the existing
single-viewer history into it, so the frontend can call that profile
`Profil principal`, `Default profile` or a future locale without localised text
in SQLite.

The three legacy progress columns remain for one release cycle. The updater
keeps the previous executable as a rollback target, and v1.2 still selects
those columns; dropping them would make rollback fail at startup. New default
profile writes are mirrored to the old columns, and a SQLite trigger mirrors
old-binary writes back into the default profile after a rollback. Other
profiles remain in the new table untouched until the newer binary returns.

Uploaded portraits are bounded, decoded, corrected from their JPEG EXIF
orientation, centre-cropped and re-encoded as 512×512 JPEG before their bytes
enter SQLite. This strips metadata and prevents the profile endpoint from
becoming an arbitrary-file host. The immutable URL includes an avatar version;
no photo falls back to a CSS Theia mark. Twelve profiles, forty Unicode
characters per name and an 8 MiB upload ceiling are deliberate
unauthenticated-LAN bounds, not account policy.

## 32. Interface language belongs to the browser, not the server

French remains the default interface language and English ships beside it.
The selection is stored in the browser's `localStorage`, not in `config.json`
or SQLite. Two devices can therefore use different languages against the same
server; changing a television to English cannot switch a laptop's interface
underneath somebody who is using it. This is local client state, not an account
preference or an authentication credential.

The catalogues live in `web/src/lib/i18n/locales/`. Adding a language means
adding one catalogue with the same shape, not adding conditions throughout the
components. The frontend build runs `web/scripts/check-locales.mjs` and fails
when a catalogue loses a key, changes a value type or drifts from a formatter's
signature. A half-translated build is a broken build, not a runtime fallback
strategy.

Changing language does not reload the application. Visible copy, page titles,
accessible names and `<html lang>` follow the active catalogue immediately, as
do locale-sensitive dates, ratings, durations, uptime and file sizes. Keeping
the words reactive while leaving an `aria-label` or a formatter frozen in
French would be cosmetic translation rather than internationalisation.

This boundary stops at Theia's own interface. TMDB metadata has already been
requested and cached as `fr-FR`: titles, synopses, genres and credit roles are
library data, not catalogue strings. A language switch neither translates those
rows nor downloads them again. Supporting per-language metadata later would
need its own cache model and quota decision; it is not smuggled into an
interface preference.

## 33. Cards are 16/9 landscape; the locked 2:3 poster is retired

**A change of direction by the maintainer, not a correction.** The `2/3` portrait
poster was locked in §6 from M3 and held until v1.3.0. It was coherent and it was
verified; it was simply not the look wanted. Recorded here so that the next
session does not read an old commit, an old screenshot or the phrase "never crop,
never letterbox" and quietly restore it.

The new reference is §6.1 of the design system. In short: `16 / 9`, the film's
**backdrop** rather than its poster, and `--radius-card`
(`clamp(1rem, 1.6vw, 1.375rem)`) instead of the old 4px.

Three things made it work rather than merely look different:

- **The artwork already exists at the right shape.** TMDB ships a 16/9
  `backdrop_path`, so nothing is cropped to fit — which matters, because the old
  rule's "never crop" instinct was right even though its ratio is gone. On the
  274-film library 257 films (93.8%) carry a backdrop, and **not one carries a
  poster without one**: artwork arrives in pairs or not at all. The remaining 17
  are unmatched files that fall through to the title as text, exactly as before.
- **Wider cards did not cost density, they gained it.** This was the real risk and
  it was measured rather than assumed, at 1280×800 on the full library: six
  columns of 432px-tall portrait cards put **11 films on screen**; four columns of
  192px-tall landscape cards put **17**. A 16/9 card is short, so more rows fit
  than the lost columns cost.
- **The radius came from the family already in the interface** rather than being
  invented: pills at `999px` for what you press, a panel radius from `1rem` up for
  what you look at. A card is the second kind.

Two consequences worth stating:

- **A real 2:3 poster still exists**, on the film detail page, which has the room
  for it. Posters were not banished; cards simply stopped using them.
- **Skeletons must be kept in step by hand.** They carry the same ratio, radius
  and column minimums as the real card. A silhouette that does not match what
  replaces it makes the page jump at the moment it finishes loading, which is
  worse than showing no skeleton.

## 34. Rows hide the native scrollbar

The horizontal scrollbar under a home row is gone. The Windows default is a wide
grey slab that cuts a row in half, and thinning it to 6px left a second, worse
answer to a question the hover chevrons already answer.

The check before ever reversing this is that every input still has a way to move
a row, and all four do: a mouse gets the chevrons, a trackpad and a touchscreen
swipe, a keyboard and a D-pad move card by card through the row's own arrow
handling. The chevrons stay `tabindex="-1"` and outside the directional-navigation
graph, so hiding the bar added no focus stops.

Verified on the real library: the three rows that genuinely overflow report
3394px of content in 1265px of visible width with **zero** scrollbar height, and
each still renders its chevron. The one row too short to scroll renders none.

## 35. Choosing a profile is a screen, not a pill in the nav

The profile model from decision 31 is unchanged: same `playback_progress` table,
same `X-Theia-Profile` header, same `localStorage` selection, same absence of
any password or permission. **Only the way in changed.**

The first implementation put the active profile in the navigation bar, as an
avatar and a name beside "Films" and "Réglages". Three things were wrong with
that, and they are the same three constraints the rest of the interface has
followed since M6:

- **A D-pad lands on the wrong thing.** The first arrow key entering the page
  reached a row of navigation links. Choosing who is watching is the question
  the app opens with, and it deserves the focus, not fourth place in a pill.
- **It fails the three-metre rule.** A 2rem avatar and an 11px tracked name are
  legible at a desk and not from a sofa. The chooser now uses the same scale as
  the rest of the chrome: cards of 240×303px at 1280 wide, and the title in
  Playfair at 112px.
- **It did not fit.** Four targets in one pill overflowed the viewport at the
  documented 320px floor by 35px, and `overflow-x: hidden` turned that into a
  control nobody could reach. Removing it left three targets, which fit once
  their horizontal padding is tightened below 26rem.

So `/profils` is now a full screen with the nav suppressed for that route only.
It appears on arrival when a profile is needed — the existing `needsSelection`
guard, untouched — and is reached from a new **Profils** section in settings the
rest of the time. Profile management (add, rename, photo, delete) stays behind
the same "Gérer les profils" toggle on that screen rather than moving to
settings: one place for everything about profiles is easier to describe than
two.

The way out is deliberate. A "Retour" link appears **only when a profile is
already active**. Arriving here because the app needs an answer must not offer a
way to leave without giving one.

Verified against the running binary with three profiles: the D-pad enters on a
card and moves between them, Enter selects and returns to where you came from,
`localStorage` follows, and the same film reports 0 / 1200 / 5400 seconds for
profiles 1 / 2 / 3 — the isolation of decision 31 survived the rework intact.

## 36. Household profiles are removed; playback is single-viewer again

**Supersedes decisions 31 and 35 after v1.5.0.** The feature is removed rather
than redesigned. The selector, management screen, local photos, browser
selection, `X-Theia-Profile` header and profile API no longer exist. Theia is
again one library with one playback position per film, matching the founding
scope and the warning that there are no accounts.

Progress is stored in the original columns on `movies`. Those columns were kept
in sync with the default profile throughout v1.3.0-v1.5.0 specifically for
rollback compatibility, so the main viewing history survives the return to the
single-viewer model. The cleanup migration then removes `playback_progress`,
the profile rows, uploaded avatar bytes and their trigger. Histories belonging
to additional profiles are deliberately deleted; silently merging several
people's positions into one would corrupt the one state that remains.

The old decisions stay here because this log records what shipped and why it
was later reversed. They are history, not an invitation to recreate the
feature. Any future multi-viewer work starts as a new scoped decision, not by
resurrecting the retired API or database model.

## 37. V2 backend and frontend advance as sequential, documented tracks

V2 milestones remain product milestones, but their implementation is split
into a backend track and a frontend track. The maintainer currently advances
the backend with Codex, milestone by milestone, then hands the merged result to
Claude for the corresponding frontend. This is sequencing, not two agents
editing the same surface in parallel.

The shared roadmap owns product scope. `theia-v2-backend.md` owns data, API,
migration and runtime work; `theia-v2-frontend.md` owns the interface that
consumes those contracts. A backend milestone is not handed off by saying that
it is done. Its merge must document the exact endpoints and payloads, errors,
migration impact, verification evidence and commit hash. Any breaking contract
change updates the frontend track and this decision log in the same commit.

Frontend work starts from merged `main` and the published handoff, never from
an external conversation or an inferred response shape. The product milestone
is complete only after both tracks have been verified. For V2-M1, the visible
flow is fixed now: one catalogue card and one film page, with the available
files chosen manually on that page. The mechanism that associates files with a
film remains a backend decision and must prefer an unmerged duplicate over a
silent false merge.

## 38. A film owns files; selection stays manual and media inspection stays explicit

**Decided and verified in V2-M1-BE.** `movies` is now the film identity, its
metadata and its one playback position. `movie_files` owns the playable paths,
and `movie_file_audio_tracks` owns the measured tracks beneath a file. The
oldest film id survives a consolidation so existing links and progress do not
move; the newest recognised metadata and most recently watched progress are
copied before duplicate rows are deleted.

Association is deliberately layered. A known path wins first. A unique unseen
file with identical size and modification time is a move/rename. Parsed title
plus non-zero year may group files unless existing TMDB ids conflict. With no
year, only an identical base name outside case and extension is accepted. An
identical non-zero TMDB id is the final proof for yearless or localized names.
No fuzzy title score, resolution token, file size or « probably the same film »
heuristic is allowed to merge two records.

The server exposes stable film, file and audio-track ids and validates their
ownership in that order. Absolute paths no longer cross the JSON boundary, and
the resolved file must remain under a configured library root after symlink or
junction resolution. The existing routes without a file id remain temporarily
bound to the primary file so the v1 frontend keeps working during the backend-
first handoff.

File inspection is an explicit `POST`, not a side effect of opening a detail or
asking for stream info. This preserves the first-need ffmpeg rule: a read-only
page cannot unexpectedly fetch the binary. Measured data is invalidated when
the file size or modification time changes. A renamed identity clears its old
TMDB payload before enrichment, and any scanner or SQLite write problem blocks
the deletion pass. Choosing an audio track forces a remux even for a directly
playable MP4, because direct play cannot guarantee which embedded track the
browser will activate. A local ffmpeg preparation failure is distinct from an
unreadable media file and is never cached against that file. The user still
chooses every file manually; no primary, resolution or bitrate becomes an
automatic quality policy. Live video quality conversion remains M6.

On the copied real database, startup consolidated 26 duplicate rows into 25
multi-file films: 274 file rows became 248 film cards while all 274 files, all
metadata states and the existing 60/240-second progress survived. Six valid
playback clips from the real `_Tests` directory then exercised direct play,
audio-copy remux, AC3-to-AAC remux and unsupported MPEG-2. A separate two-audio-
track fixture proved that selecting the French track maps only its stored ffmpeg
stream index. The original database hash and timestamp did not change.

## 39. Series use parallel tables and a playable episode item

**Decided and verified in V2-M3-BE.** Series do not force the stable film model
through a polymorphic media tree. `series`, `seasons` and `episodes` own the
catalogue hierarchy; `episode_items` owns what the player can actually resume;
`episode_files` and `episode_file_audio_tracks` mirror the measured M1 file
contract. The migration is additive and does not rewrite a movie row.

The extra playable layer is not decorative normalization. `S01E01E02` is one
physical timeline with two TMDB members, so it becomes one item with
`episode_numbers: [1, 2]`, one progress position and any number of selectable
encodes. Pretending it is two independent cards would launch both at second
zero and call the lie a feature. Files are grouped only when their complete,
ordered episode key is identical.

Films and episodes keep separate file tables so each foreign key, cascade and
ownership check remains enforceable by SQLite. Go shares the measured media
types and rules; SQL stays explicit. A path that changes family is removed from
the old family in the same transaction, so it is never playable as both a film
and an episode.

## 40. Episode classification is strict, and uncertain input remains visible

The production grammar intentionally recognizes only boundary-safe `SxxExx`
and `1x02`, including compact or separated multi-episode forms. It accepts
`S00` as specials and uses the nearest non-season parent only when the filename
has no series prefix. Numeric `101`, absolute anime numbering, dates and a bare
“Episode 2” remain unclassified until real collections justify precise rules.

A reliable episode marker without a series title produces
`episode_series_unknown`; it is not quietly indexed as a film. This problem is
non-blocking for deletion because the disk walk succeeded and the omission was
deliberate. Every unreadable directory/file or rejected SQLite write still
blocks both movie and episode pruning. The global `shorts` directory exclusion
was removed: once Theia supports episodic shorts, that name is no longer proof
that a whole subtree is disposable.

Move detection remains conservative. Same size and modification time is
accepted directly only when unique. If several unseen files share those bytes,
the episode branch ranks exact item, season and series context; it acts only on
a unique best candidate and otherwise inserts rather than steals. This rule was
added after the real HTTP validation reproduced the ambiguity with three copied
clips. A changed logical episode receives a new `episode_item.id`, while the
physical `episode_file.id`, measured media and most recent progress survive.

## 41. Series playback is single-viewer until profiles return

M3 does not resurrect the deleted profile API from decisions 31 and 35.
Progress lives on `episode_items` under the same 30-second memory floor and
near-end rule as films. This gives the currently shipped single-viewer product
an honest series history now. M2 may later migrate both film and episode
progress into its new profile model after the maintainer's visual contract is
known; M3 does not invent that model on M2's behalf.

“Next episode” means the next playable item owned locally. A missing number is
reported by `next_has_gap` instead of blocking playback, and the first episode
of the next numbered season follows the last local item without being called a
gap. Specials (`S00`) are a separate visible season and never enter automatic
mainline playback. A combined item advances from its highest member number.

Series continuation is exposed separately from the unchanged film home payload
at `/api/library/series/home`. That additive boundary keeps the released
frontend working while M3-FE decides how to compose mixed rows. It returns
episode items in progress and recently added series; it does not smuggle an
episode into the existing `Movie` JSON shape.

## 42. TMDB TV and episode streams reuse M1's explicit boundaries

Series metadata uses TMDB's TV endpoints, French metadata and the existing
90-day success / 7-day miss-or-error cache. A lookup requests one series detail
and only seasons present locally. It never materializes remote-only seasons.
Exact localized/original name wins search selection, then popularity; a shared
non-null TMDB id is the only proof strong enough to consolidate differently
named local series. Contradictory existing TMDB ids block local association
instead of letting the oldest row win by accident.

Episode detail exposes stable item, file and audio-track ids without a path.
Inspection remains an explicit `POST`; catalogue and stream-info reads cannot
download ffmpeg. A selected audio track forces remux even for direct-play MP4,
and direct play still supports HTTP ranges. The episode stream routes live
below `/api/library/episodes/.../stream`: putting them below the legacy
`/api/stream/{movie}` wildcard creates genuinely overlapping patterns in Go's
`ServeMux`, not a merely aesthetic routing disagreement.

The verified controlled corpus covered a special, a combined item, two encodes
of one episode, an English/French MP4, real TMDB TV responses, HTTP 206 direct
play and a selected-French remux decoded back through ffmpeg. An isolated scan
of the current household library found 274 video files, 254 film identities and
zero episode false positives. It still contains no user-owned series, so that
negative result is reported as such rather than promoted into positive proof.

## 43. Remote access is embedded userspace WireGuard, not a public web login

**Decided and verified in V2-M4-BE.** Decision 6 still governs the trusted LAN:
Theia has no account, password or household permission system. It is refined at
the internet boundary, not discarded. A remote device authenticates by proving
possession of its own WireGuard private key before any HTTP byte reaches the
application. That device identity is transport access, not a person, profile or
Theia account.

The server uses the official pure-Go WireGuard implementation with gVisor's
userspace netstack. It creates neither an OS tunnel interface nor a route,
requires no administrator/root privilege, starts no helper process and keeps
all six CGO-free build targets. `tsnet` was tested as an architectural option
and rejected despite its excellent traversal UX: joining a tailnet requires a
control plane, account and cloud dependency that contradict the founding
contract. Requiring a separately installed VPN was rejected for the same reason
as Docker-as-installation: it turns “one binary” into marketing punctuation.

Theia initiates no discovery, relay, STUN, UPnP or certificate request. It
passively accepts WireGuard UDP on the configured port. The owner supplies the
public `host:port` and router mapping; CGNAT remains an explicit unsupported
case rather than a hidden third-party service.

## 44. LAN administration and remote viewing are two different capabilities

The historical TCP listener remains the full, zero-login administration
surface, but now rejects public source addresses and ignores all forwarded-IP
headers. Private, loopback, link-local and actually attached interface prefixes
are accepted. This guard is defence in depth, not permission to forward TCP
8383: a local reverse proxy or source-NAT router can still make an outside call
look local.

The HTTP listener inside WireGuard accepts only the `/32` address of an active
peer and the exact host `10.77.0.1:<Theia port>`. Its allowlist contains the
static UI, health/session, catalogue, artwork, streams, explicit media
inspection and playback progress. Settings, scans, onboarding/LAN addresses,
updates and all peer management remain LAN-only. Unknown paths fail closed.

Remote browser writes require same-origin evidence. Cross-site subresources,
foreign `Origin`, DNS rebinding hosts and framing are rejected even for read
routes where a hostile page could otherwise embed a stream and spend server
resources. Native WireGuard clients do not need to invent browser headers.
This split is enforced before the shared API handler, so adding a new route does
not accidentally make it remote: it must be admitted deliberately.

## 45. A remote device receives one private key once

The server has one stable WireGuard private key. On Windows its file is bound to
the current OS user through DPAPI; on Linux and macOS it is owner-readable only
with mode `0600`. The database stores active and revoked client public keys,
names and tunnel addresses, never a server or client private key.

Provisioning generates a fresh client keypair and returns the private material
once as a standard WireGuard configuration and QR SVG with `Cache-Control:
no-store`. Status responses cannot reproduce it. Losing the configuration means
revoking that peer and creating a new one. A revoked public key is never
reactivated; this avoids stale sessions and makes the operator's mental model
match the cryptographic state.

Each client routes only `10.77.0.1/32`, not `0.0.0.0/0`, and receives one address
inside `10.77.0.0/24`. The fixed subnet exists only in the process netstack, so
even a household LAN using the same `/24` keeps its routes. At most 32 peers may
be active; a revoked address may be reused by a new key.

## 46. Reachability is observed, configuration failures fail closed, LAN recovers

`listen_port` is the local UDP port. `endpoint` is opaque configuration written
into future client files; Theia never contacts it. Changing only the endpoint
therefore does not interrupt active streams, but existing clients must edit it
or be reprovisioned. Changing the listening port restarts WireGuard.

The status says `reachability: unverified` until a live peer completes a real
handshake, then `confirmed`. No external probe means no green fiction based on
a successful local bind. If enable or reconfiguration fails, the new listener
is not persisted and the old one is restored where possible. If runtime repair
or revocation cannot be applied safely, the whole remote tunnel closes while
the LAN server stays alive.

A corrupt or machine-bound key never gets silently replaced, because doing so
would strand every provisioned device while claiming continuity. The owner can
always disable remote access from the LAN, remove the unusable key file and
provision fresh devices. Windows data directories copied to another user or
machine require exactly that recovery.

## 47. The file chooser owns its own vertical axis, and its rows are width-capped

**Decided and verified in V2-M1-FE.** The film page now lists the files the
server returned and lets the viewer pick one, plus a measured audio track when
the file has more than one. Nothing is inferred: a file whose `media.status` is
not `ok` shows "characteristics not measured" rather than reading `2160p` out of
its own filename, which is exactly what `Amelie.2001.2160p...mkv` invites.
Inspection stays an explicit button per file, and only `media_unreadable` is
mirrored into the local state, because that is the only failure the server
caches against the file (decision 38).

The interesting part was the remote. The layout's spatial navigation ranks
candidates by centre-to-centre distance with the horizontal axis weighted
`2.25`, and that weight is hostile to a list of wide rows:

- With the inspect button stacked **under** its file, the narrow button won
  every downward press: from "Lire" the first file option scored 1705 against
  the button's 1095, and the file options were unreachable by D-pad entirely.
- Moved beside its file, the rows still lost to `← Retour` at the foot of a
  short page — 1454 against 1272 — because a full-bleed row's centre sits ~384px
  right of every narrow control in the column.
- Making every row full width made it worse, not better: all five options then
  scored ~1170 against Retour's 915.
- Within the list, the *narrowest* option was skipped for the same reason:
  shrink-to-fit "Piste par défaut" scored 869 against the wider
  "ENG · English stereo" at 771.

Three things fix it together, and the third is the one that matters:

1. the chooser sits directly under the play button rather than after the cast
   list — editorially right anyway, since choosing the file is part of deciding
   to watch;
2. option rows are capped at `26rem` and share one width, so no sibling is
   penalised for having a shorter label;
3. **the section handles Up and Down itself**, in the spirit of decision 27's
   film row, so movement inside the list is reading order rather than geometry.
   An edge press is deliberately not consumed, so leaving the list still falls
   through to the page's own navigation.

Tuning the width alone was tried and rejected: it only ever won by twenty-odd
points against whatever happened to sit below, which is a number that changes
with the length of a synopsis. Any future screen with a list of options — M3's
episodes and M4's device list both qualify — inherits this constraint.

Verified against the maintainer's real library rather than a fixture: 279 files
resolved to 253 films with 25 genuinely multi-file cards, including a three-file
*Amélie* consolidated across a localised title. Playback of a chosen file with a
chosen track reached `readyState 4` at 1280×720 through
`/api/stream/3/files/4/remux?t=0&audio=2`, and the server log confirms it mapped
stable track id 2 to ffmpeg stream index 2 — the browser never sends a stream
index. Film-level progress survived a change of file: 62 seconds saved on one
file resumed at `t=62` on the other.

## 48. Profiles return as a local viewer, from a new contract rather than the old one

**The maintainer's references arrived on 2026-08-03 and this is the scoped
decision that decision 36 required.** Nothing is restored from the removed
implementation: not the screen, not `X-Theia-Profile`, not `playback_progress`,
not the avatar endpoint. What follows is what the interface needs; the model and
the API are M2-BE's to design for these actions.

The references are three Netflix-shaped screens, supplied for **layout, not
style**. Read literally they carry an account system, and Theia has none. These
are refused outright rather than translated: "Se déconnecter" (there is nothing
to log out of, decisions 6 and 36), email, "Membre depuis", "Rôle", "Statut",
the "Abonné" badge and crown (the founding pitch says zero paywall), "Transférer
un profil", "Compte" and "Centre d'aide" (all require a cloud Theia never
contacts), and the notification bell. The reference artwork cannot ship either:
the repository is public and GPL-3.0, and the rule is no unverified image, ever.

Four things were settled with the maintainer before any code:

- **The chooser is a full screen, and the nav only points at it.** The reference
  shows an inline dropdown; decision 35 had already measured why that fails —
  a 2rem avatar and an 11px name are unreadable at three metres, four targets in
  one pill overflowed the 320px floor, and the first D-pad arrow landed on
  navigation rather than on the question the app opens with. The nav entry
  therefore opens the screen instead of expanding a menu. A reference is a
  layout, not a licence to repeat a measured failure.
- **The detail panel keeps its shape and loses its subject.** Two stacked
  panels, identity above and a label/value list below, with the destructive
  action isolated at the foot. The rows become local facts — created, films
  started, films finished, last watched — and the foot action is "delete this
  profile".
- **An avatar is an image the viewer supplies.** Generated marks were offered
  and refused. This revives a mechanism, not the retired code, and it revives
  its hazards with it: bounded upload, decode, EXIF-orientation correction,
  centre-crop, re-encode to a fixed square, metadata stripped before the bytes
  reach storage, and a version in the immutable URL. That endpoint must not
  become an arbitrary-file host.
- **The startup gate returns.** With no profile chosen in this browser the
  chooser appears on arrival; the selection is local to the browser, as the
  interface language already is (decision 32), so a television and a laptop do
  not fight over who is watching.

Two constraints M2-BE inherits and must not discover late: progress now lives in
**two** places, `movies` from M1 and `episode_items` from M3 (decision 41), so
the migration owns both or it corrupts one. And a profile remains neither an
account nor a permission — anyone on the LAN may select and edit every profile,
exactly as they may already change every setting.

## 49. A profile travels in the open, and the rollback mirror follows whoever is default

**Implementation decisions from V2-M2-BE, which decision 48 left to the backend.**

The active profile is `?profile={id}` on the routes that read or write a
position — not a header. The removed implementation used `X-Theia-Profile`, and
the shape was the problem as much as the name: a header reads like a credential,
and this is not one. Anyone on the LAN may pass any id, exactly as anyone on the
LAN may already change every setting. Putting it in the URL keeps that honest,
and keeps it debuggable.

An absent id falls back to the oldest profile, so the released frontend and any
client that has never heard of profiles keep working. An id that names nothing
is **refused** rather than quietly redirected to the default: writing one
viewer's position into another's history because a television held a stale id is
the kind of corruption nobody notices and nobody reports.

Eight profiles, and the number comes from the screen rather than from taste. The
chooser is one horizontal row read from three metres, where a card may not fall
below the design system's 160px legibility floor; past eight the row wraps or
shrinks under it. The previous implementation allowed twelve, chosen before that
screen existed.

Two hazards were found by running the thing rather than reading it:

- **Consolidation and rename lost every viewer but one.** Both paths moved
  progress through the single legacy columns, so merging two copies of a film —
  or renaming an episode — discarded the other profiles' rows when the duplicate
  cascaded away. Both now move `movie_progress` / `episode_progress` per profile,
  most recently watched winning within each profile rather than across all of
  them.
- **Deleting the default profile stranded the rollback mirror.** The legacy
  columns on `movies` exist so a rolled-back v1.5.0 still finds a history, and
  they follow whichever profile is oldest. Deleting that profile promotes the
  next one, and the columns kept serving the deleted viewer's positions —
  a history belonging to nobody. Deletion now re-points them.

The avatar endpoint re-encodes everything to JPEG regardless of what arrived, so
a hostile PNG cannot be stored and served back intact, and bounds both the bytes
read and the pixels claimed — a decompression bomb is a few bytes of header
asking for gigabytes. One EXIF tag is parsed by hand, because a full parser is a
dependency and a much larger surface for a single integer, and every other field
in that block is precisely what Theia throws away.

## 50. One profile is not a question

**A deliberate narrowing of the maintainer's answer in decision 48, recorded
because it is a narrowing.** The startup gate was specified as "show the chooser
whenever no profile is active in this browser". Built exactly that way, a
household that never creates a second profile meets "Qui regarde ?" on every new
device, with one card to press. That is a click whose answer is already known,
standing between a television and a film — and the founding criterion is a film
in under three clicks.

So a lone profile is adopted silently, and the chooser appears the moment there
is an actual choice: two or more profiles, or a stored selection that named a
profile since deleted elsewhere. Everything else in decision 48 stands. If the
maintainer wants the screen unconditionally, the change is one condition in
`profiles.svelte.js`.

Two things were found by looking at the built screens rather than the code:

- **A middot ended a wrapped line.** The file facts render as separate children
  with separators between them, so `2:30 ·` could sit alone at the right edge
  with `2 pistes audio` below. The separator is now inside the same inline box
  as the fact it introduces.
- **Deleting an unknown profile reported the wrong reason.** The last-profile
  rule was checked before existence, so removing an id that did not exist
  answered "the last profile cannot be deleted" whenever one remained — a true
  sentence about the wrong subject. Existence is checked first.

A profile's page is addressable as `/profils?gerer=1&profil=<id>` so a reload
does not throw the viewer back to the row they came from.

## 51. The wordmark is set, not placed

The photographic wordmark — serif letterforms filled with an earth horizon — is
the prestige piece the founding spec §11.10 describes, and the same paragraph
warns that photographic detail does not survive the reduction to nav-bar size.
It does not, and the failure is specific rather than a matter of taste. At the
28px the bar renders, the atmosphere band runs straight through the middle of
the letters and reads as a strikethrough; the lens flare blows out the E and the
I; the T and A come out gold while the H, E and I come out white, so it reads as
two words; and the image's box shows as a rectangle against the near-black bar.
That is exactly the "mal intégré, rectangulaire, mauvaise couleur perçue"
already sitting in the roadmap's open points, and the fallback recorded beside
it was plain "THEIA" text.

So the mark is typography until M5 does the real art direction: uppercase
display serif, tracked at `0.32em` in the manner of the reference moodboards,
with a negative end margin because letter-spacing also applies after the last
letter and would otherwise push the mark a third of an em right of everything
below it. It costs no asset, no licence question and no request, it carries the
name to a screen reader without an alt attribute, it cannot reflow the bar while
it loads, and it is legible from a favicon to a television.

`web/static/theia-wordmark.webp` is now referenced by nothing. It is left in
place rather than deleted — it is the maintainer's licence-checked asset and
M5's starting point — and joins `icon-512.png` in the roadmap's open points.
Designing a new mark is still M5; this is integration, not identity.

## 52. A series is a catalogue of items, not of episodes

**Decided and verified in V2-M3-FE.** The catalogue, the series page and the
episode page consume the M3-BE contract as published. Three things follow from
the model rather than from taste:

- **A combined file appears once.** `S01E02E03` is one item with one timeline,
  so the row reads "épisodes 2 à 3", joins both TMDB titles, and carries one
  resume position. Two cards would launch the same file twice from second zero.
  Each member still gets its own synopsis on the episode page, because that is
  information the item genuinely holds.
- **A gap is stated, never enforced.** `next_has_gap` renders as a line beside
  the next-episode action, in the warning register, and blocks nothing.
  Specials are a named season and lead nowhere: `S00` never receives a
  `next_episode_id`, so the page says so instead of inventing a successor.
- **Seasons are options, not pages.** Switching season on a television should
  not be a navigation, and the season payload is compact by design — files live
  on the episode page. The row obeys §9: one shared width, its own axis.

The file chooser and the player are the components M1 built, not copies. Both
now take the resource they act on — `basePath` for inspection, `streamBase` and
`progressPath` for playback — because an episode is a different resource, not a
different interaction. The same is true of the card: `PosterCard` gained an
artwork and title override so an episode can use it without pretending in its
data to be a film.

Series rows are appended after the film rows on the home screen and disappear
when the endpoint returns nothing, which is what keeps a films-only library
looking exactly as it did.

Verified against a real corpus rather than a fixture: TMDB recognised
*Severance*, the specials and season 1 filled, a two-track MKV inspected at
1280×720 with `eng`/`fra`, the French track remuxed from twenty seconds in and
decoded back through ffmpeg without error, and the chain reported
`[1] → [2,3] → [5]` with the gap flagged on the middle item and nothing after
the special.

## 53. A remote session does not ask for what it may not have

**Decided and verified in V2-M4-FE.** The layout reads
`GET /api/remote-access/session` before anything else, because what follows
depends on which side of the tunnel the browser is. On the LAN nothing changes.
Inside the tunnel the settings link is not rendered and the home screen does not
request onboarding.

Not requesting is the point. `Promise.allSettled` would swallow the 403 quite
happily, and the released home already did. But a deliberate refusal is a
security boundary working, not a failure to tolerate: asking anyway trains the
interface to treat a correct answer as noise, and the first person to add
logging would see a wall of forbidden requests from every remote device. A
session whose mode is unknown is treated as LAN, because hiding the settings
from somebody sitting at home is the worse of the two mistakes.

The panel says the uncomfortable parts out loud, before anything can be switched
on: the router forwards **UDP** and never Theia's TCP port, which has no
authentication at all; and CGNAT is stated as unsupported rather than quietly
attempted. `unverified` is presented as a fact, not a fault — no device has
proven the path, and Theia owns no probe that could say otherwise. `confirmed`
says "since this start", not "guaranteed". The byte counters are labelled as
tunnel traffic so nobody reads them as viewing statistics.

Both consequences of a change are stated before the save, not discovered after
it: a new endpoint does not reach devices already provisioned, and a new port
restarts the listener. Only fields that actually changed are sent, so saving an
endpoint never restarts a tunnel nobody asked to restart. Every error state
keeps **Disable** reachable, because that is the documented way back and the LAN
never went away.

The provisioning dialog holds the one copy of a private key that will ever
exist. It lives in component memory: verified in a real browser that after
creating a device, `PrivateKey` appears in no localStorage value, no
sessionStorage value, no IndexedDB database, not in the URL and not in the
document — the QR renders as SVG and the configuration text is never written
into the DOM. Closing without copying asks first, and closing clears it. Losing
it offers "revoke and recreate", never "show it again", because a server that
could show it twice would not have thrown it away.

Two things found by running it: `.profile-input` carries a row-direction flex
basis for the profile page, and reused in this column it stretched the port
field to the height of the whole group, stranding the value at the bottom of a
very tall box. And a device name typed with accents round-trips intact — the
mojibake in the first screenshot came from the shell that created the peer, not
from the server, which was checked rather than assumed.

## 54. The identity is the word and the rule, and the icon is a crop of it

**Decided in V2-M5, from four directions put in front of the maintainer before
anything was written.** The brief was the one the roadmap set: real art
direction, several options, validation before implementation — not a cosmetic
retouch smuggled into another milestone.

Four were drawn and rendered at 16px, 28px and 96px, then in a navigation
lockup: a Greek theta, a drawn sun, the word underlined by a rule, and a
geometric sunrise. Rendering them at their real sizes is what decided it. The
sun lost its rays at 16px and became a gold dot, which is exactly the failure
decision 51 had just removed. The maintainer chose **the word and the rule**:
THEIA in the display serif, underlined by a gold rule that reads as a horizon.

Its weakness was known when it was chosen — at 16px the word does not fit — so
the reduction was designed and tested rather than assumed. Three candidates were
rendered in a real browser tab: a disc crossed by the rule read as a ringed
planet and said nothing about the lockup; the rule alone became a grey smudge;
the **initial standing on the rule** stayed legible and is a crop of the lockup
rather than a second mark invented beside it. That is the icon.

Three consequences:

- **The T is drawn as paths, never as a text element.** A favicon is fetched as
  an image, so no webfont reaches it: a text node would fall back to whatever
  serif the platform happens to have and the letter would differ per machine.
  The trade is that the serifs are blockier than a true didone, because
  thick/thin contrast is what disappears first at 16px, and legibility there
  outranks fidelity at 180.
- **The touch icon is generated from the same SVG**, so the two files cannot
  drift. It exists only because iOS will not take an SVG for a home screen.
- **Two dead assets left the binary.** `icon-512.png` was 208 KB serving a PWA
  manifest that the founding spec §11.5 excludes from v1, and
  `theia-wordmark.webp` was the nav crop that decision 51 stopped using; the
  README's prestige logo lives in `assets/`, outside the binary, and is
  untouched. Static assets fall from about 840 KB to 584 KB.

The favicon was verified the way the four-release bug of decision 22 taught: not
by the file existing, but by loading it into an `Image()` and reading back
`32×32` after a strict XML parse.

Both of the roadmap's open points are now closed. What remains genuinely open is
whether the photographic wordmark ever earns a surface inside the application;
it currently has none, and none is invented for it.

## 8. Logistics

- **Repository:** public, `theia-media`, from M0.
- **Language:** code, comments and internal error messages in English, for
  contributors. The interface defaults to French and includes English; both
  catalogues live under `web/src/lib/i18n/locales/` and must pass the parity
  check described in decision 32.
- **Updater:** built from the documented GitHub Releases pattern — check the
  API, download, swap atomically, restart — rather than ported from Hermes,
  whose source was not available. This section originally assumed Windows would
  need a relay process to replace a running `.exe`; testing showed it does not,
  because Windows allows a running executable to be *renamed*. See decision 22,
  which supersedes that assumption.

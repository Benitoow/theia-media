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

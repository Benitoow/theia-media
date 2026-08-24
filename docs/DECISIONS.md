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

## 55. Audio and subtitles are chosen in the player, and the player asks for itself

**Decided in V2-M5b, from a report after a real evening of watching.** The
maintainer's words: *"pas de choix de piste audio quand disponible, et non plus
pour les sous-titres."*

The audio chooser existed. It was on the film page, under the file list, and it
was invisible in exactly the case that matters. M1's contract is that a file is
measured lazily — asking how a file will play must never download ffmpeg — so a
file played for the first time is probed *by that playback*. The page had loaded
before the probe, its copy of the tracks was empty, and it stayed empty until
somebody reloaded. The chooser was not missing; it was one page load behind
reality.

Two things follow, and only the second is a bug fix.

**The choice moves into the player.** Nobody leaves a film to change the
language. `/info` was already the one request the player makes before playing,
so it now carries `audio_tracks` and `subtitle_tracks` alongside the mode and the
duration, and the player asks for it itself rather than inheriting a snapshot
from the page behind it. The stale-copy failure cannot recur, because there is
no copy. When playback probes a file that had never been measured, `/info` is
asked once more — guarded by a flag, and silently, so a refresh that fails costs
a menu entry and never a running film.

**Subtitles are built, at last.** Decision 3 settled them in the first week and
nothing had been written: text tracks in, image tracks out. `internal/subtitles`
converts SRT and WebVTT in-process; ffmpeg extracts embedded tracks as WebVTT.
Three details cost something to get right.

- **The offset is not cosmetic.** A remuxed stream is a pipe restarted at a
  timestamp, so the element's clock begins again at zero on every seek. Text on
  the film's own clock would sit as far from the picture as the viewer has
  travelled into it. The same `-ss` that seeks the video seeks the subtitle, so
  the two cannot drift apart — measured on a generated MKV, a cue at 00:07.023
  comes back at 00:00.023 for `t=7`.
- **`.srt` files beside a film need no ffmpeg.** They are found by reading the
  directory when the player asks how the file will play, which is once per
  playback: a subtitle dropped in this afternoon is offered this evening without
  a rescan, and a deleted one stops being offered. A browser-friendly MP4 with a
  subtitle next to it gains subtitles without downloading 80 MB.
- **windows-1252 is decoded, not assumed away.** French subtitle files are that
  encoding as often as UTF-8, and a mis-decoded one is not a cosmetic problem:
  every accented word in the film is wrong. Valid UTF-8 is left alone; anything
  else is decoded as windows-1252, the safest single guess, and it needs no
  dependency.

**An image track is listed and refused, not hidden.** A BluRay rip whose only
subtitles are PGS would otherwise look like a film with none, and somebody would
go hunting for a setting that does not exist. The menu names the track and says
why; the endpoint answers `subtitle_image_based` with 415.

Two smaller things were fixed on the way. Episodes were still taking audio track
zero, so the `PreferredAudio` correction made for films now applies to series
too. And the subtitle's position is computed rather than declared: `line` as a
count of lines snaps to a height the browser picks, and left the second line of a
cue under the scrub bar; `lineAlign: 'end'` is the right idea and Chrome ignores
it. The cue is placed from its own line count and the measured height of the
control bar, in units of the picture — because that is what browsers scale
subtitle text against, whatever the stylesheet says.

A library measured before this change has every fact except this one, which is
indistinguishable from a file that genuinely has no subtitles. Rather than reset
274 films to `pending` and re-probe the lot — throwing away durations and
resolutions already measured, to learn one new thing — `subtitles_scanned` marks
the difference, and the next playback of a file re-probes it once. It was about
to run ffmpeg anyway.

## 56. The router is asked, not the owner

**Decided in V2-M5b, over a first refusal.** The maintainer wrote: *"accès à
distance à une documentation complexe et devrait se faire automatiquement, on
avait rien de compliquer, on clique et ça marche."* The first answer given was
that this could not be done without breaking decision 43, because "automatic"
sounded like a relay. That answer was wrong about what was being asked. The
clarification — *"quand je dis auto je parle de l'url et du port etc"* — named
two facts, and neither of them needs a relay to obtain.

M4 shipped a panel with a UDP port field, a public endpoint field, and a
paragraph explaining how to forward a port on a router Theia has never seen.
Four steps, three of them in somebody else's admin interface, in a product whose
founding promise is no configuration.

**Both facts belong to the gateway, and there are two standard ways to ask it.**
UPnP IGD — SSDP to 239.255.255.250, then SOAP — and NAT-PMP — two datagrams to
UDP 5351. Both are tried concurrently, because they fail by timing out and a
router that speaks neither would otherwise cost the sum of two waits with
somebody watching a spinner. `internal/portmap` implements both in the standard
library, so the constraint on runtime dependencies is untouched.

**Decision 43 is untouched, and that is the point rather than an excuse.** Both
protocols speak only to the gateway on the local network. There is no relay, no
rendezvous server, no STUN and no endpoint-discovery service anywhere in the
path; the only internet calls Theia makes are still TMDB and GitHub Releases.
The router is not the internet. It is the thing standing between this machine
and it, and it already holds the address because it was given it.

Three things this refuses to do:

- **It will not lie about carrier-grade NAT.** A router behind CGNAT reports its
  own 100.64.0.0/10 address perfectly happily, and a client configuration
  pointing at one cannot work. That is checked and reported as the specific
  thing it is — `remote_carrier_nat`, with the advice that asking the operator
  for a public address is usually free — rather than left to fail as a timeout
  on the evening somebody is away from home.
- **It will not follow an SSDP reply anywhere.** That reply is an
  unauthenticated datagram from anybody on the network, naming a URL this
  process is about to fetch. The address is checked before the fetch: numeric,
  private, and the same host that answered. Redirects are not followed and
  connections are not kept.
- **It will not leave a hole open.** Disabling withdraws the mapping. Closing
  Theia does not, because somebody who shut the application for the night has
  not asked their router to forget anything — and the next start re-checks the
  address anyway, since a domestic connection is renumbered by a reboot.

**The manual fields survive, folded away.** UPnP is off by default on some ISP
firmware, CGNAT cannot be forwarded through at all, and somebody who has already
forwarded the port by hand must not be told to undo it. The disclosure opens by
itself exactly when the router has refused, which is the one moment it is the
way forward. Typing an endpoint turns automatic off, because otherwise the next
discovery would overwrite it and the field would read as not saving.

Verified against the maintainer's own router, which is the only test that
counts: UPnP answered in 0.74 s, mapped the UDP port, and reported a public
address. Enabling through the API took 438 ms end to end and produced a running
tunnel whose generated client configuration carried the discovered endpoint.

## 57. A broken-image icon is a bug anywhere, so the rule is enforced anywhere

**Decided in V2-M5b, from a screenshot.** A specials episode drew the browser's
torn-page glyph in the middle of the row: TMDB had recorded a `still_path`, and
the image behind it was not there. The `{#if}` guard was correct and
insufficient — it covers a missing path, not a path that 404s.

The rule already existed for profile marks and had been written one component at
a time, which is why it did not hold: posters, backdrops, stills and avatars all
come from the same cache and any of them can fail the same way. It is now one
capture-phase listener in the layout, because `error` from an `<img>` does not
bubble and because the rule has to cover artwork nobody has added yet. The image
is hidden rather than removed, which leaves the empty frame the design already
draws for a film with no poster.

## 58. The furnace is lit, because the evidence finally arrived

**Decided in V2-M6.** Decision 1 kept full video transcoding out of v1 and
called it "the real furnace". The founding spec put GPU transcoding in v2 under
one condition, written before a line of this project existed: *only if direct
play and CPU remux prove insufficient in real use.*

They did, and not in theory. The maintainer watched a film and reported that
the sound was badly out of sync. It was not: Chrome had loaded an HEVC Main 10
rip, played its audio, and never produced a picture — `canPlayType` empty,
`videoWidth` 0, not one frame decoded. Direct play cannot help, a remux copies
the same undecodable stream, and the honest answer M5b shipped was to name the
codec and stop. That is a file the household owns and cannot watch.

**Nothing here is inferred from a list.** The pinned ffmpeg advertises
`h264_nvenc`, `h264_qsv`, `h264_amf`, `h264_mf` and `libx264` on every platform,
because all five are compiled in; whether any runs depends on the card and the
driver. Each candidate is asked to encode one frame of nothing, and only the
ones that come back are offered. Measured on the maintainer's desktop:
`usable=h264_amf,h264_mf,libx264`, `refused=h264_nvenc,h264_qsv,h264_vaapi,
h264_videotoolbox` — NVENC cannot load `nvcuda.dll`, QSV cannot create an MFX
session, both instantly, because there is no NVIDIA card and no Intel graphics.

**The ceiling is a measurement, not a preference.** On a 1080p HEVC source at
720p, `libx264` runs at **1.04×** real time and `h264_amf` at **4.56×**. One
software transcode therefore consumes the entire margin; a second does not run
at half speed, it makes both stall, and neither viewer can tell why. So the
limit is one in software and three on hardware, and beyond it the answer is a
refusal with its own code rather than a queue nobody can see.

**The browser is the only thing that knows.** `hevc` is classified risky rather
than unsupported because Safari plays it and Chrome does not, and no server can
ask a client what it will decode. So the client answers: when the picture never
arrives, the player asks for the same film again with `video=transcode`. Once,
guarded, because a transcode that also fails must not loop. Verified end to end
on the file that started this — the log shows `mode=remux` at 63 s, then
`mode=transcode video_encoder=h264_amf` at 63 s, and Chrome reports 1920×804
with frames decoding. Star Wars plays.

Three refusals kept:

- **Nothing above the source.** A 1080p file offers 720 and below. Upscaling
  spends a GPU inventing detail that is not in the file.
- **Nothing this machine cannot make.** With no encoder that runs, the player
  shows no quality section at all rather than a button that fails. The rung
  list comes from the probe, and `/info` only probes an ffmpeg already on disk —
  M1's promise was that asking how a file plays must never *download* it.
- **Nothing about the cost is hidden.** The section heading says "carte
  graphique" or "processeur", because on this machine those are 4.56× and 1.04×
  and the difference belongs to whoever is deciding.

## 59. Whether a browser keeps up is measured, never asked

Decision 58 lit the furnace for a picture Chrome **never decodes**. This is its
other half: a picture Chrome decodes and cannot keep up with.

Reported as "a very big delay between the sound and the image" on a real HEVC
Main 10 rip, in "Qualité du fichier" — the remux. The same file at 720p, same
DTS track, is perfectly in sync. The remux copies the video, so the browser does
the decoding; it produced a picture, reported `1920x804` and `readyState 4`, and
then rendered far slower than the film runs while the audio, trivially
transcoded to AAC, kept perfect time. The existing guard did not fire because it
tests `videoWidth === 0`, which is the case where nothing is decoded at all.

**No API answers this in advance, and two of them lie about it.** Measured on the
machine where playback is broken:

| Asked | Answer |
|---|---|
| `canPlayType('video/mp4; codecs="hev1…"')` | `probably` |
| `mediaCapabilities.decodingInfo(…)` | `supported: true, smooth: true, powerEfficient: true` |

`decodingInfo` returned the same for AV1. It is designed for exactly this
question and it is not usable for it, so nothing here consults it. Do not
reintroduce a pre-flight check on that basis.

What is used instead is the playback itself: presented frames per second **of
film**, sampled twice about 2.5 s apart, `Δ totalVideoFrames / Δ currentTime`.
Both halves of that ratio stop together, so buffering cannot flatter it. Below
**10** the browser is not keeping up. The floor is fixed rather than derived from
the source frame rate, which the server does not store: every real film runs at
23.976 or more and the broken case measures near zero, so a constant is both
correct and free of a schema change.

Three consequences:

- **The re-encode keeps the source resolution.** `?video=transcode` with no `h`
  reaches `TranscodeArgs` with `height = 0`, adds no `scale`, and returns
  1920x804 H.264 — verified. Only the codec changes. Falling back to 720p would
  have cost definition on a machine measured at eight times real time.
- **The verdict belongs to the browser**, in `localStorage`, for the reason
  decision 32 gives for the language: the television and the laptop have
  different GPUs and different builds, and an answer from one is a lie on the
  other. It is keyed by codec rather than by "risky", so the next codec to join
  that set inherits nothing. Settings carry the way back, for the day a driver
  fixes it.
- **Only a risky remux is watched.** H.264 that already plays is never measured.

Two things left standing on purpose. The legacy `/api/stream/{id}/info` route
carries neither flag, so a playback started through it cannot arm the detector;
the film page always passes a file id, so the interface does not use that route,
and teaching a third handler the decision logic costs more than it returns. And
after a switch the menu still labels the top rung "Qualité du fichier" while a
re-encode runs, because `qualityLadder` only rewrites that entry for
`ModeUnsupported`.

**Unverified here, and it must be checked on the maintainer's browser.** The
preview pane does not composite frames, and it inverted this very measurement:
it reported the HEVC remux at 24 frames per second and the transcode at 0.5,
which is the opposite of the machine the bug was reported from. The threshold is
reasoned and the plumbing around it is verified end to end; the threshold itself
is not.

**Superseded in part by decision 86.** The fixed floor of ten and the single
sample both survived only as long as the files were 1080p. The reasoning above
still holds — no API answers this, and the playback itself is the only honest
signal — but the threshold is now six tenths of the file's own frame rate, which
the server stores since migration 0014, and the measurement repeats for as long
as the film plays.

## 60. Hardware decoding is probed, and `auto` is not a candidate

`TranscodeArgs` encoded on the GPU and decoded on the CPU. `internal/ffmpeg/decoders.go`
now probes an `-hwaccel` the same way `capabilities.go` probes encoders: each
candidate is asked to decode one frame, once, lazily, on the first playback that
transcodes. On the maintainer's AMD desktop it reports
`usable=d3d11va,dxva2 refused=cuda chosen=d3d11va`.

The ordering is measured, not alphabetical, transcoding that HEVC rip to 720p:

| | Speed |
|---|---|
| none, software decode | 8.1x |
| `d3d11va` | **9.86x** |
| `dxva2` | 5.37x |
| `auto` | 3.8x |

**Two accelerations are slower than none, including the one that picks for
itself.** `auto` is therefore not in the candidate list and `dxva2` sits behind
`d3d11va`. The empty string remains a valid answer: software decoding at 8.1x
has ample margin and is correct everywhere.

The honest limit of this: a probe proves a method *starts*, not that it is
*faster*. If the chosen one is ever slower in real use, replace the liveness
probe with a short benchmark against software. That is more truthful and more
expensive, and not worth paying until it is needed.

## 61. V2 is released, and its planning documents are archived rather than kept

`v2.0.0` publishes roughly twenty-four thousand lines that had accumulated on
`main` since `v1.5.0` across seven milestones: series, multi-file films,
household profiles, WireGuard remote access, track selection in the player,
hardware-probed transcoding, and English beside French.

The five documents that ran that cycle — the roadmap, the backend and frontend
handoffs, and the two discovery notes — move to
[`archive/`](archive/README.md). **This supersedes the pointers in decision 41**,
which named `theia-v2-backend.md` and `theia-v2-frontend.md` at their old paths.
That decision stays as written, because it was true: those documents did own
those surfaces, and the sequencing it describes is how V2 got built.

They are archived rather than deleted for the reason this whole file exists. A
finished plan still holds why a milestone was ordered where it was and what was
rejected, and none of that survives in the code. What they must not do is look
current: five process documents sitting beside the three governing ones is how a
newcomer mistakes a finished plan for the present state.

The two-track, backend-first split ends with the cycle it was invented for.
Post-v2 work reads the founding spec, this file and the design system, and
records a decision when it settles an argument.

The one thing left undone at release: **the README screenshots predate the v2
interface.** They show the 2:3 poster grid, before the 16/9 cards, the wordmark
in the navigation, profiles and series. Neither browser available to the agent
could produce replacements — the preview pane does not composite frames, and the
Chrome extension was not connected — so the release ships with a note above the
images saying so, rather than with images that quietly misrepresent the product.

## 62. The screenshots are taken by a scripted headless Chrome, against a library built for the purpose

The note decision 61 left in the README is gone: **the screenshots now show the
v2 interface.** Both obstacles turned out to be worth writing down, because the
next person to need a screenshot will meet them again.

**The one-shot route does not work here.** `chrome --headless --screenshot`
cannot seed `localStorage`, and the profile chooser intercepts every route until
a profile is chosen, so every capture came back as *Qui regarde ?*. What works is
a scripted session: launch headless Chrome with `--remote-debugging-port`, drive
it over CDP, set `theia.profile`, then navigate and capture. Two details are
easily lost — a device scale factor of 2 downscaled to 1600px is what makes the
type crisp, and the scroll offset must be set explicitly on *every* shot,
including to zero, because navigating back to a URL restores the offset the tab
had last time and silently reframes the picture.

**A screenshot needs a library, and there was not one.** After the placeholder
files were deleted, the catalogue held one real film and seven codec fixtures
with no artwork. The library here is forty films and twelve series as two-second
H.264 clips cloned from one 3.5 kB master, named the way releases are named, in a
folder outside the maintainer's own. TMDB matched all fifty-two with no failures,
which is what puts real posters, backdrops and episode stills on screen. It is a
few hundred kilobytes, and it is *valid media* — unlike the 3 TB of unopenable
placeholders it replaces, which is the trap decision 59 was written about.

The frames avoid the panels that expose the rig — the watched folder, the data
directory, `VERSION dev` — not to flatter the product but because a reader
should not have to work out which parts of a screenshot are real.

**Two more traps, found taking the v2.4.0 film-page shots.** Navigating straight
to a deep link right after seeding `localStorage` lands on the *home screen*: the
application has not finished booting when the URL changes, and the capture comes
back looking plausible and wrong. Navigate, then read `location.pathname` back and
retry until it matches. And cast portraits are `loading="lazy"`, so a frame that
scrolls to them still photographs empty frames unless the images in view are
flipped to eager and given a moment — the screenshot is taken from a page that was
never scrolled by a human, and nothing else triggers the load.

**This is also how the profile bug in decision 63 was found.** A screenshot of a
feature is a test of it: the home screen refused to show the resume hero, and
that refusal was correct.

## 63. A page's onMount runs before its layout's, and the profile went out with the request

Every screen that reads progress fetched it **without `?profile=`** on a cold
load. Svelte runs a page's `onMount` before its layout's, and the layout is where
`profiles.load()` lived, so `profiles.url()` was called while `activeID` was
still `null`.

What that looked like: the home screen offered to resume a film nobody in the
house had started, and showed the default profile's *Continuer à regarder*.
Worse, and silently: the player wrote its position against the wrong viewer, so
one person's evening landed in another person's history. Navigating within the
application fixed it, which is exactly why it survived M2 and shipped in
`v2.0.0` — it is invisible unless the first page you land on is the one that
matters.

The fix is a shared promise. `profiles.ready()` starts the load on first ask and
hands every later caller the same promise, so the page and the layout race to
start the *same* request instead of two, and everything that builds a
`?profile=` URL awaits it first. A failed load resolves rather than rejects: a
server without profiles still serves the library, and no screen should be held
back over the question of who is watching.

Measured before and after on a cold load of `/`:

| | Request | Hero |
|---|---|---|
| Before | `/api/library/home` | featured |
| After | `/api/library/home?profile=2` | resume |

The general lesson, which is not about Svelte: **a component that reads shared
async state must await it, not assume somebody above it already did.** Ordering
that happens to hold is not ordering.

## 64. The presentation site lives in the repository, builds without a bundler, and fetches nothing

`site/` holds a landing page, published to GitHub Pages by
`.github/workflows/pages.yml`. It exists because a README is written for someone
who already found the repository, and the question the project actually has to
answer first is *what is this and where do I get it*.

Four decisions inside it, each of which had an easier alternative:

**Both languages render from one template.** `page.mjs` is the markup once, as a
function of a catalogue; `build.mjs` writes `index.html` and `en/index.html`
from it and **fails if a key exists in one catalogue and not the other**. The
easy version — a page in French with a JavaScript toggle — puts one language in
the markup and the other in a script, which is the arrangement decision 32 was
written against. A new language is a third file in `locales/`.

**No bundler and no dependency of its own.** Plain Node writing strings. The two
fonts come from the `@fontsource-variable` packages `web/` already installs, so
the build errors out if `npm ci` has not run there rather than shipping a page
that silently falls back to Georgia.

**Nothing is fetched at runtime.** No analytics, no CDN, no font service, no
call to the GitHub API. A site for a project whose whole argument is that it
does not phone home cannot itself phone home. The version number is the one
thing that would have justified an API call, and it is avoided instead: download
links point at `releases/latest/download/<asset>`, which GitHub redirects to the
newest release, so they never go stale and need no script. The only JavaScript
on the page promotes the likely platform in a list that is fully present in the
markup.

**`styles.css` restates the design system rather than importing it.** The site
is outside the application bundle and cannot reach `app.css`. That duplication
is deliberate but it is a liability: a value here that differs from the
application is a bug, not a liberty, and the file says so at the top.

The comparison table is the README's, including the five rows that go against
Theia. A landing page is exactly where the temptation is to drop them, which is
exactly why they stay.

## 65. The public site proves the product before it inventories it

**Supersedes decision 64's page composition, not its technical boundary.** The
site still lives in `site/`, renders French and English from one strict template,
builds without a bundler, serves local fonts and images, and performs no runtime
request. What changed is the question it answers first. The old page opened with
a slogan, then made somebody scroll through a feature catalogue and seven
screenshots before the download. It described the product faithfully and made
trying it feel like homework.

The first screen now puts the reader and the download decision beside the
promise. The player image is a real capture of the shipped `Player.svelte`, made
against an isolated one-file library. The film in it is generated from the
original vector at `docs/screenshots/source-player-demo-media.svg`; no poster,
film frame or Internet asset entered the capture. The controls layered above it
are interactive reconstruction and say so. That distinction is permanent: a
demonstration is labelled, a capture names its provenance, and neither borrows
credibility from the other.

The rest is deliberately short: three product moments, three differences, three
limits, one safety note and three FAQ entries. The exhaustive comparison stays
in the README. This reverses decision 64's choice to duplicate its whole table
on the landing page: visible limits build the same trust without turning the
site into a spreadsheet somebody has to survive before downloading.

Downloading is two explicit decisions: operating system, then architecture.
JavaScript may reveal the chosen system panel; it never reads the user agent to
pick x64, ARM64, Intel or Apple Silicon. With scripts off, all three panels and
all six `releases/latest/download/` links remain in the markup.

Version, publication date, asset size and SHA-256 come from a build-time release
file. `site/release.json` is the verified offline snapshot; GitHub Pages refreshes
the same schema from the release API and rebuilds after every successful Release
workflow. Missing optional metadata disappears rather than becoming a guessed
number. `sitemap.xml`, `robots.txt`, canonical/hreflang links and the visible FAQ
schema are generated by the same build.

The social image follows the same provenance rule. Its editable SVG is authored
in the repository and embeds only the verified player capture. The published
PNG therefore explains the product without inheriting the unverified poster
rights that made the first concept unsuitable for Open Graph.

## 66. The library reads the disk on its own, by looking rather than by listening

**The zero-configuration promise had a hole in it.** Adding a film meant opening
the settings page and pressing a button. The founding scenario is "plug the
machine in, it works"; nothing in it says "and then go and tell it what you did".
A scan was only ever triggered at startup or by hand, and the only tickers in the
whole binary belonged to the updater and to remote access.

**Filesystem notifications were considered first, and rejected.** `fsnotify` is
the obvious answer and it is a pure-Go dependency, so no guardrail forbids it.
What forbids it is the storage this project expects. `internal/scanner` opens by
saying that a real library lives on external drives and network shares; inotify
and `ReadDirectoryChangesW` do not cross an SMB mount. A notifier that silently
delivers nothing on exactly the storage the product is built for is worse than no
notifier, because it looks like it works. It also needs a watch descriptor per
directory, which is a limit to run into rather than a limit to reason about.

**So the disk is read, and the answer is compared with the last one.** A pass
walks the roots with `scanner.Scan` — the same walk, stat calls only — and
reduces what it saw to one 64-bit fingerprint. Unchanged means nothing happens:
no generation is burned, no row is written, nothing is logged. Reading the disk
and reconciling with the database are two very different costs, and only the
cheap one is paid on a quiet minute.

**Sixty seconds, because the thing being spent is not CPU.** A stat-only walk of
a few thousand entries costs nothing measurable. An external drive that has spun
down is what a shorter interval actually spends. A film appearing within a minute
is indistinguishable from instant for the person who just put it there.

**A file written in the last fifteen seconds is left out of the fingerprint.** A
film arriving over the network is not there all at once, and indexing it halfway
produces a card with no duration, a failed inspection and a wasted TMDB lookup,
all of which have to be undone. Excluding recent files means a copy in progress
reads as "nothing has changed yet" rather than as a new film — and the pass after
it finishes sees it properly. This is also why the walk logs nothing: an unplugged
drive is reported once per root per pass, which at a pass a minute is fourteen
hundred identical warnings a day burying the one line that matters.

**Outstanding metadata reconciles even when the disk has not moved.** Metadata
work is not created by files changing: a library larger than one batch of two
hundred spreads over several passes by design, and an outage or a rejected key
leaves rows to retry. Judged by the disk alone a settled library would keep its
gaps for ever, because nothing on disk is ever going to change again to prompt
another look. The queries that choose what to fetch are the ones that decide what
counts as outstanding, so a lookup that failed answers "nothing due" until its
retry falls due and this cannot spin.

**The watcher owns the folder list.** The settings handler hands it a new one and
asks for an immediate pass, so a folder added on that page fills the library
while the user is still looking at it. It owns the list rather than reading the
configuration because a background goroutine and an HTTP request were otherwise
reading and writing the same slice.

Verified against the configured library: startup scan, then a film copied in at
18:47:39 and indexed at 18:48:13 with its metadata fetched. Three scans in three
minutes — startup, one metadata catch-up, one real change — and none on the
quiet ticks.

## 67. A wrong match is corrected in the interface, not on the filesystem

**Supersedes the README's "manual TMDB matching" refusal.** That row said
correcting a wrong match was done by renaming the file. It was the only entry in
that table that made the user pay for the server's mistake rather than describing
a boundary, and it is unactionable from the television the remote belongs to.

**It is not a metadata editor, and must not become one.** There are no fields to
type into and nothing to correct by hand. There is a list of the records the
automatic search passed over, ranked the way `pick` ranks them — exact title
first, then popularity — so that the film the matcher chose leads and the
alternative sits directly under it. Decision 11 already documented the failure
this exists for: searching "The Handmaiden" returns the making-of above the film,
because TMDB orders by text relevance. Confirmed again on the real library, where
the candidate list for Star Wars is the film and then *The Making of Star Wars*.

**A correction is an identity, not a snapshot.** Decision 9 keeps metadata cached
and never frozen, so every record is re-read when it ages out. If that re-read
searched the filename again it would rediscover the same wrong film and quietly
undo the correction ninety days after somebody made it. So a corrected row is
marked `tmdb_locked` and refreshed **by id** instead. The identity stops moving;
the data does not.

**A series is corrected with its whole cascade.** Every season and every episode
title came from whichever show the series was matched to, so replacing only the
series record leaves a page headed by the right show and filled with another
one's episodes — worse than the original mistake, because it looks deliberate.
`refreshSeries` was split out of `enrichSeries` for this and is reused whole,
rather than a second implementation of the same cascade drifting beside the first.

**Correcting stays on the LAN.** It changes what a file *is* for the whole
household and it spends the TMDB quota, which puts it on the administration side
of decision 44. The list of candidates is refused remotely too, for the second of
those reasons.

**Reverting exists.** Handing a film back to the automatic matcher clears the pin
and puts the row back in the queue, and asks the watcher for a pass so the poster
is right again in a minute rather than whenever the folder next changes.

## 68. Being watched is something a viewer may simply state

Decision 17 defines when a film *becomes* watched, from the position reported
during playback. Two ordinary situations have no position to report. A film
abandoned twenty minutes in sits in the continue-watching row for ever, because
nothing will ever finish it. A film seen somewhere else is not on this server's
record at all.

**Stored as a statement, not as a position at the end.** `finishedRule` is
recomputed from the position on every report — deliberately, so that starting a
film again returns it to the row — which means a position wound to the end would
be erased by the first second of playback. The position is cleared instead:
a film already seen starts at the beginning when it goes on again, not eight
seconds before the credits.

**Marking unwatched is forgetting the position**, so it is the same operation
`DELETE /progress` already performed, under the name the client means when it
asks. A film nobody has watched and a film somebody un-watched are the same
state, and keeping them apart would be inventing a distinction to store.

**It travels.** Saying where you got to and saying you have seen it are both the
viewer describing their own viewing, so both are allowed on the remote listener.
Decision 44's line is at managing the household and the server; it does not move.

## 69. One search, answered by the server, over both catalogues

`/films` searched films and `/series` searched series, each by filtering a
catalogue it had already downloaded. Both are good pages. Between them they made
you decide whether the thing you half-remembered was a film or a series before
you were allowed to look for it.

**The server answers, which is what makes it work from outside the house.** A
phone on the WireGuard listener asks a question and gets twenty rows back,
instead of pulling the whole library down to filter it locally. `/films` keeps
its client-side filtering: its sorts, genres and watch-state filters are instant
and that trade still holds at household scale.

**Matching happens in Go, not in SQL.** SQLite's `LIKE` cannot see past an
accent — "amelie" matches "Amélie" in no collation available without CGO, and CGO
is the first prohibition in the founding spec. `searchKey` therefore mirrors the
one in `web/src/lib/api.js`, folding accents through an explicit table rather
than buying `golang.org/x/text` for one function. The candidate query stays
narrow, so what crosses the boundary is a few columns per title rather than every
synopsis and cast list; only the rows that matched are read in full. If a library
ever grows large enough for that to be felt, the answer is a folded column
written at scan time, not a different matching rule.

**A page, not a field in the navigation pill.** At three metres the pill is
already carrying as much as it can, and a text box there would take the first
D-pad press away from the library — decision 35 measured exactly that for the
profile control.

## 70. The measurements are shown, because the standard is to report what was verified

Decisions 58, 59 and 60 measure what a machine can do: which encoders answer when
asked to produce one frame, whether a hardware decoder exists, what the software
fallback costs in real time. All of it was used silently. The project's own
standard for a milestone is to report what was verified rather than what was
assumed, and that is easier to hold to when the verification is on a page
somebody can read.

The settings page now names the ffmpeg state, the encoders that answered and what
each one runs on, the hardware decoder, the size of the artwork cache, the folders
being watched and how often, and what the last pass did.

**Nothing here probes anything that is not already on disk.** `Capabilities`
reaches `Manager.Path`, which downloads ffmpeg when it is missing. Asking what
this machine can do must never be the thing that causes a download, so the probe
is gated on `Available()` and the page says "not measured yet" rather than
fetching sixty megabytes to fill a field. This is the same promise M1 made for
`/info`.

While writing it, one older breach of decision 25 was removed: `settingsResponse`
carried a French sentence advising the user to add a TMDB key. Nothing in the
interface ever read it — the catalogues already own that sentence — and a French
paragraph inside a Go struct is precisely what that decision exists to prevent.

## 71. The seek strip is built from keyframes, once, and is never waited for

Dragging the bar showed a timestamp and nothing else, which tells you where you
are and not what is there. The frames now shown under the cursor are one JPEG
sheet of a hundred tiles, windowed by `background-position` — one decode for the
whole strip rather than a hundred image elements.

**Keyframes only.** `-skip_frame nokey` is what makes this affordable: a
two-hour film is read at a fraction of the cost of a full decode. The `fps`
filter still lays the frames on an even time grid, taking the nearest decoded
frame to each mark, so one tile reliably means one interval even though a
source's keyframes are not evenly spaced. A hundred frames over two hours is one
every seventy seconds, which is what a scrub preview is for; more would be a
longer encode and a larger download for a picture nobody studies.

**It is a comfort, and behaves like one.** It is asked for once when a player
opens and never awaited. Three states reach the interface — ready, building, or
nothing — and the last two draw the timestamp alone, exactly as before. One
encode runs at a time, because decision 58 measured that a single software
transcode consumes the whole real-time margin on this machine and the film
somebody is watching is worth more than the strip under their cursor.

**And it never downloads ffmpeg.** `Manager.Path` fetches the binary when it is
missing, so the build is gated on `Available()` and a machine without one simply
has no previews. Same promise as `/info` and the encoder probe.

**The client measures the tile.** The pinned upstream build ships ffmpeg and no
ffprobe — the reason `Probe` already shells out to ffmpeg — so the server cannot
say how wide a tile came out without guessing an aspect ratio the file may not
have. The browser divides the loaded sheet's natural width by the column count
instead, which is a fact rather than a guess.

Two faults found by running it rather than by reading it. ffmpeg chooses its
muxer from the file extension and refused `.jpg.tmp` outright — *"Unable to
choose an output format"* — so the format is now named and the temporary file
carries a real extension. And a file that cannot be built was re-attempted on
every request: three identical failures in a third of a second while a player
polled. Failures are now remembered for the life of the process, which is not
persisted on purpose, since a restart is usually an upgrade and an upgrade is a
reason to try again.

Verified against a four-minute file: a hundred tiles at 2.4-second intervals, a
1600×900 sheet of 160×90 tiles weighing 205 KB, and the browser resolving the
midpoint of the bar to `background-position: 0px -450px` — column 0, row 5,
tile 50 — with the timestamp reading 2:00. The hover itself was not seen: the
in-app preview pane does not composite frames, so the appearance of the strip
under a moving cursor remains unverified.

## 72. A phone navigates from the bottom, and the bar above keeps only the mark

The navigation was one object at every width: a floating pill holding the
wordmark, four tracked uppercase labels and the profile mark. On a television
and a laptop it is the right object and it stays. On a phone it never was.

**Measured before anything was changed.** At 375px the pill is 327px wide and
its contents need 459px. Decision-free tightening of the padding had already
been applied and was not enough; "Réglages" was cut in half and the profile mark
sat 113px past the right edge — in the document, painted nowhere, reachable by
nothing. `overflow-x: hidden` on the body was the only reason the page did not
also scroll sideways, which is to say the clipping was hiding the symptom.

The first answer was to let the row wrap. It worked, in the sense that every
control was on screen and every target kept its size. Looking at it on a phone
for the first time settled it: two rows of chrome, 140px, seventeen per cent of
the viewport, permanently, with the wordmark alone and centred on a line of its
own. It was a fix, not a design.

**So the destinations move to where the thumb already is.** Five tabs —
accueil, films, séries, rechercher, réglages — pinned to the bottom, icons over
a micro label, the current one in gold. The pill stays above carrying the two
things that are not destinations: the mark, and whose history is being written.

This is what every streaming application on a phone converged on, and the reason
is reach rather than fashion: the top of a six-inch screen is the one place a
hand holding the phone cannot go. It is also the only part of this interface
where that argument applies, which is why the bar is hidden above 36rem and on
the television. Section 9's contract — the first D-pad arrow enters at the
primary action, the pill is the surface it enters — is written against the pill
and is untouched.

Three glyphs were drawn for it, on the 24-unit grid and 1.7 stroke the icon file
opens by insisting on: a house, a film strip, a television. The tab label tracks
*in* at 0.03em rather than out, against §4's rule, because five tabs across
375px give each one 75px and "Rechercher" measured 77px at the label register's
normal tracking — its own slot exactly, touching both neighbours. The tracking
is what stretched it, so the tracking is what gave.

**Two things followed from having a phone layout at all.** The library toolbar
is one pill on a wide screen and stacked at 390px, where a 999px radius on a
280px-tall box turns the ends into two enormous arcs; it becomes a plain stack
there, with the field in a pill of its own and the filters in one horizontal row
that scrolls. And `ChromeScene`'s veil is a left-to-right gradient, opaque where
a wide screen puts its copy and down to 18% at the right edge — on a phone the
copy is full width and lands in that 18%, so "Aucune série" was grey type over a
lit statue. Same veil, rotated to face the copy.

Seen rather than measured, finally: Chrome will not resize below 1430px on this
machine, so the phone was verified through an iframe of the running application
at 390px inside a full-size tab. One caution for anyone repeating it — a CSS
animation inside that iframe does not advance (`theia-enter` reports
`running` with `currentTime: 0`), so every `.enter` element reads as
`opacity: 0` and looks like a contrast bug that is not there. The rig disables
`.enter` before judging anything.

## 73. A television gets its own step, at 100rem, and it starts with the safe area

The interface had one large-screen layer, at 80rem, and it was written on a
desktop. A television is not a large desktop: it is the same pixels at four
times the distance, cropped at the edges by a set nobody has calibrated, driven
by a remote that cannot hover. Measured on a 1905px viewport, every one of those
four things was unaccounted for.

**The safe area, first, because it is the one that loses content.** Every
television guideline agrees on the outer five per cent — about 96px horizontally
and 54px vertically on 1920×1080. The bar sat 16px from the top edge, the gutter
was 77px, and the player's own furniture was 32px from the sides and 28px from
the bottom: the play button and both ends of the scrub bar were inside the part
of the picture a set is allowed to eat. All three move out to 96 and 56.

**Then one left edge.** `.page-shell` is `min(100%, --content-wide)` and 96rem
is 1536px, so past about 1712px the shell stops touching the gutters and centres
itself while anything positioned from the gutter does not. The result at 1920
was four different left edges: the wordmark at 185, the hero title at 77, the
row headings at 261 — 184px to the right of their own cards — and the settings
column at 384, floating in the middle of the picture with nothing to line up
against. `--content-wide` becomes 120rem at this step, the settings column gives
up its centring, and everything starts at 96.

**Then the type, which is the part that argues.** Section 4's three-metre rule
tops out at 19px body, 24px row headings, 16px card titles and 13px labels, and
the code implements it exactly — so it is the rule that is short, not the
implementation. Fire TV puts the floor for body text at 28px on 1080p, Android
TV asks 24sp, and the common advice is two and a half to three times a phone's
sizes; 16px to 19px is 1.19x. It shows in the one place a television is actually
read: the metadata under a hero — year, runtime, director — is the label
register, and from a sofa it is a grey smudge.

The tokens move rather than the classes. Sixty-one places in the markup reach
for `text-small` and `text-label` as utilities rather than through `.label` or
`.tv-copy`; overriding the classes alone left those behind, which is how a
film's rating stayed 13px while the director beside it became 16px.

**And the focus ring, because on a television focus is the cursor.** 2px at 1920
on a 55-inch set is about 1.3mm, subtending roughly one arcminute from three
metres — the threshold of seeing a line at all, before a panel's motion blur and
a stream's compression get to it. It becomes 4px with a 3px offset. Cards were
already exempt: decision 33's §6.2 gave them a border, a lift, a scale and a
shadow together, for exactly this reason.

**Why 100rem and not 80rem.** Viewport width cannot tell a 1920 television from
a 1920 monitor. 1600px is simply above every desktop window this interface has
been looked at on and below what a television reports, so the sofa layer can be
assertive without a 1440px desktop inheriting any of it. Verified: at 1425px the
labels are still 11px and the card titles still 14px.

**One bug fell out of looking.** `.poster-card--fluid` is declared inside
`@layer components`; the 80rem step that gives a row card its fixed width sits
at the foot of the file, outside every layer. An unlayered rule beats a layered
one whatever the specificity says, so every card in every grid took the row
width: 326px cards in 265px columns at 1920, overlapping their neighbours by
37px with the 24px gap swallowed whole. A 1440px desktop had the same fault at
10px, which is why nobody had seen it. The fluid rule is now declared in the
same unlayered block.

The remote was checked rather than assumed, and needs no work: the first arrow
lands on the primary action, down enters the first row, right walks it card by
card, up leaves for the navigation and stops there without trapping. That is
section 9's contract, and it holds.

## 74. Text answers travel compressed, and nothing else does

The frontend bundle was 394 KB of JavaScript and CSS going over the wire
verbatim, and the film catalogue is a JSON array that grows with the library.
Measured on a 192-film bench, on the request the frontend actually makes
(`limit=500`): `/api/library/movies` was 148,142 bytes and the stylesheet alone
74,613. On the machine serving them that is free. Over house
Wi-Fi to a television, or down the WireGuard tunnel to a phone on mobile data,
it is the difference between a page that appears and a page that arrives.

Both listeners share one handler, so the same middleware covers LAN and remote —
which is the right way round, because remote is where it matters most.

Measured after: the catalogue 148,142 → 20,860 bytes (−86%), the stylesheet
74,613 → 19,942 (−73%), the home screen 29,488 → 4,422 (−85%).

**The exclusions are the load-bearing part.** A film is already compressed, so
gzipping it burns CPU to make it bigger. Worse, a range request answered with a
compressed body no longer means what the player asked for, because the offsets
it seeked to are offsets into the file, not into a gzip stream. So: never a
request carrying `Range`, and only a named list of text types — never video,
never JPEG, never the sprite sheets. There is a test for each of those, and the
range one is the one that would be a bug rather than a missed optimisation.

Below 1400 bytes nothing is compressed either: gzip's own header is longer than
`/api/health`'s whole body.

## 75. A card asks for the picture it can show, and the first screen does not wait

Every card was served the 780px backdrop and every card was `loading="lazy"`,
including the ones already on screen.

Neither was right. A card is between 158 and 336 CSS pixels wide depending on
the screen. On the real cache, one backdrop is 80,499 bytes at w780, 44,247 at
w500 and 22,552 at w342 — so a television was downloading nearly four times the
picture it could display. And a lazy image on the first screen is a page that
paints its artwork late for no reason at all.

So the card offers three widths and lets the browser choose, which is better
than choosing here because the answer depends on the device pixel ratio as well
as the layout. Verified, cold-cache, in Chrome:

	television 1920, 1 dpr, 336px card   → w342   −72%
	phone 375, 2 dpr, 158px card         → w500   −45%
	HiDPI laptop, 2 dpr, 266px card      → w780   unchanged, and correct

The last row is the point: a screen that can use the pixels still gets them.

The first six cards of a grid, and the first four of the home page's first row,
are `eager` with `fetchpriority="high"`. Six covers one row on a television and
three on a phone. Only the first row opts in — eager-loading every row would
fetch the whole page at once, which is the same fault in the other direction.

## 76. The decoder is benchmarked, not probed, because the answer is a property of the machine

Decision 58's note ended by saying that if the chosen hardware decoder ever
turned out slower in real use, the liveness probe should be replaced by a short
benchmark against software decoding — more honest, more expensive, and not worth
paying for until it was needed.

It was needed. The same `-hwaccel d3d11va` that won by 22% on the maintainer's
AMD desktop loses by 30% on a laptop whose Radeon 890M shares memory with the
CPU. Transcoding one minute of 1080p HEVC Main 10 to 720p:

	                     desktop        laptop
	software decode      8.1x           5.85x
	d3d11va              9.86x          4.07x

`-hwaccel` without a GPU filter chain decodes on the GPU and then copies every
frame back for the scaler. Whether that copy is worth making depends on the bus
between them, and no ordering of a candidate list can answer it.

Worse, the old probe could not have known: it fed the accelerator a `lavfi`
pattern, which is generated as raw frames, so the decoder under test had nothing
to decode and every hwaccel "passed".

So the candidate list now says only which methods are worth *trying*, and a
benchmark decides: build two seconds of 1080p H.264, time software decoding,
then time each candidate in the shape a transcode uses it — including the scale
filter, because timing a bare decode would hide exactly the readback being
looked for. Sequentially, because two accelerators timed at once contend for the
same silicon and both come out looking slow.

A hardware path has to be **10% faster** to be taken. Anything inside the noise
keeps software decoding: it is the one path correct on every machine and the one
it costs nothing to be wrong about. Observed in the log on the laptop:

	software=104ms  d3d11va=252ms  dxva2=316ms  chosen=""

The whole benchmark cost 849ms, once, lazily, on the first transcode — cheaper
than the probe's worst case, which was a 15-second timeout.

## 77. libx264 is not tuned for a video call

The software encoder carried `-tune zerolatency`. That setting exists so a frame
comes out the instant it goes in: it turns off b-frames and lookahead and
switches x264 to slice-based threading. Theia is not a video call. It writes a
fragmented MP4 into a pipe that a player buffers, and it was paying for an
encoder delay nobody can perceive.

Measured on a Ryzen AI 9 HX 370, one minute of 1080p HEVC Main 10:

	                            720p    1080p   first 192 KB
	veryfast -tune zerolatency  4.52x   3.60x   1108 ms
	veryfast, threads unbound   5.26x   4.08x   1400 ms
	veryfast -threads 8         5.35x      -    1250 ms

Eighteen per cent more throughput, with b-frames and lookahead back at the same
bitrate, for 142ms more before the first fragment. On this machine that margin
is spare; on the machine in decision 58, which managed 1.04x real time, it is
the difference between a margin and a coin toss against a stall.

The thread cap is what keeps the 142ms small: x264 holds roughly one frame in
flight per thread before it emits anything, so an unbounded twenty-four-thread
encode starts noticeably later and finishes no sooner. Capped at
`min(NumCPU, 8)`, so a small machine is never asked for threads it has not got.

Hardware encoders keep their own defaults. Their `-usage` and `-quality` knobs
were measured and changed nothing — 10,679ms against 10,786ms and 10,768ms —
because on this hardware the transcode is decode-bound, not encode-bound, which
is the same finding as decision 76 seen from the other end.

## 78. The type changed faces, and gained a register doing it

The maintainer supplied five faces and asked for the shipped two to go. Two were
taken, and the reasons the other three were not are the useful part.

**Capitalis TypOasis** maps 65 glyphs. Every French accent is missing, and four
of the ten digits are absent — "1 h 45" rendered as a row of architectural
dingbats. **Greek Freak** looked complete in the character map but draws its
accented codepoints without accents, so "Réalisé" came out "REALISE": a defect
the cmap could not show and only rendering did. **Poseidon AOE** is a brush
face, legible at hero size and a smudge anywhere below it.

**Augustus** takes the display register and **Dalek Pinpoint Bold** the labels.

That is two faces where there were two, but the registers went from two to
three, because Augustus cannot do what Playfair did in the middle: it has no
lowercase at all — `abcdef` and `ABCDEF` measure identically — and Dalek is
small caps. Neither can carry a synopsis. So prose falls to the platform's own
interface face, which ships nothing and is the one font on any device already
tuned for reading.

The wire cost went down: 76 KB of gzipped TrueType against 85 KB of WOFF2.

**The licence did not survive the change, and that is recorded rather than
glossed.** Section 10 used to say both faces were SIL Open Font License 1.1,
GPL-compatible, imposing nothing. Augustus records only "Converted by ALLTYPE";
Dalek Pinpoint is © Keith Bates / K-Type and points at a licence page rather
than stating terms. This repository is public and GPL-3.0, and a font inside a
released binary is redistributed. The maintainer supplied them and the call is
theirs; the previous claim was load-bearing, so its removal is written down.

## 79. A font that fails to load looks exactly like a font that loaded

Augustus was refused by every browser and the page looked completely fine,
because the fallback in the stack is Georgia and Georgia is a perfectly good
serif. The screenshot that was supposed to prove the new face was working was
in fact proof of the old fallback doing its job — including the specimen sheet
the face was *chosen* from, which means the choice was very nearly made on the
strength of a font nobody had actually seen.

What caught it was `document.fonts`, which reported `Augustus: error` next to
`Dalek Pinpoint: loaded`, and the console behind it:

	OTS parsing error: overlapping tables
	OTS parsing error: cmap: language id should be zero: 1

Two defects of a 1999 ALLTYPE conversion, both described in §10 of the design
system along with the repair. Both are recorded because the general lesson is
cheap and the specific one was nearly expensive: **a rendered screenshot does
not verify a web font.** `document.fonts.check()` does, and it is one line.

`font/ttf` joined the compressible types in the same change (decision 74's
middleware). WOFF and WOFF2 are deliberately excluded — they carry their own
compression, and a test pins that.

## 80. The version tells itself

The settings page ended with "Theia v1", written into both catalogues by hand,
four screens below the running version the same page had already fetched from
the server. It had been wrong since v2.0.0.

It is now a function of the running version, `milestone(version)`, fed from
`settings.version`, so the footer cannot disagree with the binary again. Decision
25 in miniature: the server sends the fact, the interface writes the sentence.

The player's failure message referred to "la v1" as the reason a format was
unsupported, which stopped being true when v2 started re-encoding MPEG-2 and
VC-1. It now says the format is unsupported without blaming a version.

## 81. The type keeps its face and changes its files

v2.2.0 shipped Augustus and Dalek Pinpoint and said, in section 10, that neither
carried an open licence. Read properly afterwards, both were worse than unclear.

K-Type's free fonts are "for personal use among friends and family". Webfont use
requires a paid licence, embedding in a software product requires the Enterprise
tier, and every tier forbids giving the file to others -- which a public
repository does by existing. And Augustus is not orphaned at all: it is Paulo W's,
published by Intellecta Design, a commercial foundry that sells it. The copy
circulating on free-font sites carries no licence and an "ALLTYPE" conversion
stamp.

Neither could ship in a GPL-3.0 repository that publishes six binaries per
release, so the identity was kept and the files were changed. **Cinzel** takes
the display register and **Jost** the labels, both SIL Open Font License 1.1.

The replacements are better on their own merits, which is the part worth
recording. Cinzel is the same Roman inscriptional brief as Augustus and does it
properly: `abcdef` measures 150 against `ABCDEF` at 158, so there is a real case
distinction, where Augustus measured 169 for both and had no lowercase at all.
Augustus also drew its question mark as a figure 9 -- found by rendering "Où
étiez-vous ?" beside a face that could. Jost gives the label register the same
geometric contrast Dalek gave it, with a variable weight axis instead of one
bold.

Three OFL candidates were rendered and refused: Marcellus SC and Cormorant SC
are Roman small-caps faces too close to Cinzel for the two registers to read as
two, and Julius Sans One is too light to hold a label at three metres.

52 KB of WOFF2 where v2.2.0 had 76 KB of gzipped TrueType, and 85 KB before that.

## 82. The interface has a guard, because looking at it does not work

Every visual fault this project has shipped rendered a page that looked
completely correct.

Cards overlapped their neighbours by 37px because an unlayered rule beat a
layered one, and the grid looked like a grid. A phone rule was written above the
rule it meant to override and did nothing at all, and the phone looked fine. A
display font was refused by every browser and the fallback behind it is Georgia,
so the page -- and the specimen sheet the face had been chosen from -- looked
exactly as intended.

None of these were caught by reading CSS and all of them would have been caught
by a handful of assertions in a real browser. So `web/tests/` now runs four,
across the three widths section 4 and section 9 are written against:

- **nothing overflows** the viewport, excluding elements inside a scroller built
  to hold them, and excluding full-bleed artwork behind the content -- the hero
  backdrop is over-scaled by 1.5% on purpose;
- **every declared face loaded**, checked through `document.fonts` and the
  console, and the first family of each register is one of them, so renaming a
  token cannot quietly leave the page on a fallback;
- **every target is at least 44px**, section 9's floor;
- **the page has one left edge**, and on the television step that edge is inside
  the 96px safe area.

It deliberately tests no behaviour. The Go suite covers the API, and clicking
through the player in CI would be slow and flaky for no benefit.

The guard was verified by breaking something on purpose: `Cinzel Variable`
misspelled in its token produced "Cinzel Vairable is named by a token but never
loaded" on every page at every width. A guard nobody has watched fail is not
known to work.

Playwright is a devDependency and Chromium is the only engine installed --
assertions about geometry do not differ between engines, and the other two would
triple the download. The no-dependency rule this project holds is about what
ships in the binary, and nothing here does.

## 83. A library to look at, without a library

The standard is to report what was verified against the real library rather than
what was assumed, and the real library is 274 films on one machine. Everywhere
else -- a fresh clone, a CI runner, the same machine with the drive unplugged --
there is nothing to look at, and a grid of eight test files says nothing about a
grid.

`scripts/bench` fills a throwaway database with as many films as there is cached
artwork for, drawing on the image cache already on disk so it fetches nothing and
needs no network. It was written after building the same thing by hand twice in
one afternoon.

The rows point at paths that do not exist, which is deliberate: this is a library
to look at, not one to play. It also puts every row in the current scan
generation, because that is what makes a row visible, and forgetting it is how
the hand-built version silently produced an empty library twice.

## 84. Subtitles can be pushed against the picture

A rip whose subtitle track was muxed from a different cut runs a second or two
out, and nothing else in the interface could rescue it: the file is the file, and
decision 3 refuses to burn anything into the image. The track panel now carries
a sync control -- half a second a press, thirty seconds either way, and the
readout doubles as the reset.

The cue times are rewritten rather than the clock being read through an offset,
so the browser's own `cuechange` keeps firing and everything downstream carries
on unchanged. The original times are held aside and every shift is computed
against those, or repeated halves walk: measured, +1.5s then -2.5s returns to
exactly 1.000, not 0.9999.

**Not persisted, on purpose.** It belongs to this viewing rather than to the
film. A stored offset is one more piece of state to be wrong later -- against a
different file of the same film, or after somebody remuxes the one it was
measured on -- and the control is two presses away whenever it is wanted.

It only appears when a subtitle is actually showing. A sync control under a film
displaying none is a setting for nothing, and that panel is already three
sections long.

## 85. The whole TMDB record, because it had already been paid for

**Decided post-v2, from reading the client rather than the interface.** Theia
asked TMDB for a film and kept eleven fields out of the answer. The tagline, the
original title, the age certificate, the collection the film belongs to, every
crew credit except the director, and the portrait of every actor named on the
page were all in the payload, already downloaded, and thrown away — the film page
was showing roughly a tenth of the record its own request had returned.

**Nothing here costs a request.** `append_to_response=credits,release_dates` on
the film call and `credits,content_ratings` on the series call turn what would be
four round trips into one larger body; the tagline, the original title and
`belongs_to_collection` are plain fields on a details response that was already
being parsed. A scan of the real library makes exactly as many TMDB calls after
this change as before it.

**The interface still owns every word.** A crew credit crosses the API as a role
code — `writing`, `music`, `cinematography` — never as TMDB's English job title,
and a series carries `ended` or `returning` rather than "Returning Series".
Decision 25 exists because a Windows syscall name once appeared in the middle of
a French page; "Original Music Composer" would have been the same fault with a
nicer accent. A job title this whitelist does not know is dropped rather than
passed through. The one exception is deliberate: a certificate is *data* — "12",
"TP", "R" is what the board wrote — and the country beside it is named by
`Intl.DisplayNames` in the active locale, so no catalogue carries a list of two
hundred countries to print one of them.

**A collection lists what can be played, and nothing else.** TMDB knows all six
parts of a saga and would happily name them. A household that owns parts one and
three sees two films, not one film and two absences: the home screen is a
personal surface rather than a second catalogue (decision 29), and that rule does
not stop being true one page down. The row is the ordinary `Row` component with
its heading overridden, so it scrolls, snaps and answers a D-pad exactly like
every other row in the interface — measured at 1920, its cards start on the same
96px rule as the home screen's.

**The backfill is a column, not a migration that guesses.** Every film already in
a library carries `metadata_status = 'ok'` and a recent fetch timestamp, so
decision 9's ninety-day lifetime would have left the new columns empty until
November on a library scanned in August — for data sitting in an answer TMDB had
already given. `metadata_version` records the field set a row was written with,
and a row behind the current one is stale regardless of its age. It refetches in
the ordinary scan batches at the ordinary rate limit, and the next new TMDB field
is a constant bump rather than a hand-written UPDATE. Rows TMDB never matched are
untouched: there are no new fields to fetch for a film it does not know.

**Where the type went.** The tagline is a sentence, so it is set in the reading
face and not in the display serif — section 4 keeps Cinzel for titles, and a
marketing line in small capitals reads as a second heading arguing with the
first. The certificate is a box drawn with the line colour rather than the accent,
whose five-per-screen budget was already spent on the rating beside it, and its
tracking is tighter than a `.label` because "12" at 0.18em reads as "1 2". Cast
portraits are 2:3 at w185 — the smallest size the image cache whitelists, still
twice what the frame draws at — and they keep `loading="lazy"`, because the cast
is below the fold on every screen this runs on.

**The catalogue got lighter, not heavier.** Sending the record with every film in
a list would have been the obvious cost of this change, and it was: measured on
250 films, the cast, crew, taglines and certificates were 31% of the `/films`
response, for fields no list view reads — the library page draws cards showing a
title and a year, filters on genre and sorts on rating. So `collectMovies`, which
every list read goes through and no single-film read does, drops what only a
detail page shows; the two heroes do the same, being one film each but not a
detail page either. That seam was chosen over a second SQL projection on purpose:
the column list and the scan order are already paired by hand, and a third
pairing is a shifted field waiting to happen. Reading a few extra columns out of a
local SQLite file costs nothing; the wire is what decision 74 is about.

Measured on the bench, same server, before and after the slimming: `/films` at
250 films went from 450 KB to 194 KB uncompressed and 45.3 KB to 23.9 KB gzipped,
and the home screen from 8.0 KB to 4.3 KB gzipped. Against what the response
carried *before this whole change* — cast names, no portraits — it is roughly 36%
smaller uncompressed. The film page gained its whole record and the library page
pays less than it used to.

**Verified, and what was not.** Against a real film with live TMDB data: the
tagline, `TP (France)`, the Star Wars collection, John Williams under *Musique*,
ten portraits fetched at 185px. Against a 250-film bench for the rest — sagas,
missing certificates, a cast member with no portrait, an odd cast count — at 375,
1280 and 1920: nothing overflows, the page never scrolls sideways, both faces
load, no target is under 44px, and the type moves with the 100rem step (tagline
27.6px, cast names 18px, credits 16/18px). Language switching was driven through
the settings screen and back: `Classification R, États-Unis` becomes `Rated R,
United States` without a reload.

The interface guard still does not reach either detail page — its harness starts
against an empty throwaway library, and there is no API that creates a film — so
everything above was measured by hand in a browser. That gap is the reason this
paragraph exists rather than a passing test.

## 86. The floor under "is the browser keeping up" is the film's own cadence

**Decided post-v2, from a 13.9 GB file and four measurements.** Reported as big
sound-against-picture desync, on a 2160p HEVC Main 10 Dolby Vision remux with two
TrueHD Atmos 7.1 tracks — 2 h 35, `bt2020nc/bt2020/smpte2084`, 23.98 fps.

**Three suspects were measured and cleared before anything was changed**, which
is most of what this entry is worth:

| Asked of the real file | Answer |
|---|---|
| Remux drift over 20 minutes, Theia's exact arguments | video 1200.114 s, audio 1200.083 s — 83 ms of offset, **constant** |
| The same at `-ss 3600`, `3605`, `3607.5` | the same 83 ms; input seek does not desynchronise the two streams |
| Transcoding 4K with `h264_amf`, Theia's exact arguments | **2.36×** real time — the pipe does not starve |

So neither the remux, nor seeking, nor the encoder was the cause, and the honest
place to look next was the guard that was supposed to catch this and did not.

**Decision 59's floor was a constant for a reason it stated, and the reason
expired.** It measures frames decoded per second *of film* and compares them to
ten, "deliberately not the source frame rate, which the server does not store".
That was calibrated on a 1080p file measuring near zero. On a 4K one, a decoder
managing fourteen frames of a 23.98 fps film is losing two fifths of a second of
picture every second — a minute of drift every two and a half — and fourteen sits
comfortably above ten. The guard stays silent for the entire running time.

The frame rate is now measured and stored. It was always printed on the stream
line ffmpeg already produces, next to `tbr` and `tbn`, which carry lookalike
numbers and are why the pattern is anchored on the unit. The floor is six tenths
of it; the constant survives as the fallback for a file that never said, and for
a file inspected before migration 0014, whose column is NULL until it is
re-inspected.

**And it watched once.** The check sampled 2.5 s after playback began and never
again, with three of its own guards — paused, seeking, too little film elapsed —
returning without rearming. A film started paused, or scrubbed in its first
seconds, spent the rest of its length unwatched; so did one whose decoder was
fine at the opening titles and not fine an hour later. It now rearms at the end
of every branch, and needs **two consecutive** slow windows before acting,
because acting means restarting ffmpeg under somebody who is watching and one bad
sample can be a buffer emptying.

**Verified end to end on the server side**, against the real file through a
running binary on port 8395: the probe reads `frame_rate: 23.98`, migration 0014
carries it, and `/api/stream/1/files/1/info` returns it beside `video_risky:
true` and `reason_code: audio_transcode`. Migration 0014 was applied to a copy of
the real database — 8 films, 8 files, 5 audio tracks, 2 inspections, all
unchanged, source hash identical afterwards.

**Not verified, and it must be checked in a real browser.** Whether Chrome on
this machine actually decodes this file below 14.4 frames per second, and
therefore whether the new floor fires where the old one did not, is a measurement
only a real decode can make. The preview pane does not composite frames and
inverted this very measurement once before, which is the whole reason decision 59
ends the same way.

Two limits left standing on purpose. The episode info route does not carry a
frame rate, so episodes keep the constant — the reported fault is a film, and
teaching a second handler this costs more than it returns until an episode shows
the same thing. And `totalVideoFrames` counts frames decoded rather than
presented, which is the right half of the pair here: dropping frames is how a
player *stays* in sync, while decoding them late is what makes the picture fall
behind.

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

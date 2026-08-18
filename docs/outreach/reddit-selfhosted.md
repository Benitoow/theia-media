# r/selfhosted — draft

Written to be read by people who are tired of being sold to. The rules that
shaped it, because they are worth keeping if you rewrite this in your own voice:

- **No superlatives.** That subreddit downvotes "blazing fast", "modern",
  "beautiful" on sight. Every claim below is a number or a refusal.
- **Lead with the frustration, not the product.** Nobody there wants another
  media server. They want the specific problem you solved.
- **Say what it does not do, early.** The refusal table is the most trustworthy
  thing the project has. It also filters out the people who would post an angry
  comment forty minutes later.
- **Be visibly the author.** Trying to sound like a happy user is transparent
  and is the fastest way to be buried.
- **Flair:** `Release` (or `Product Announcement` if the sub requires it).
- **Post at a European evening / US morning**, weekday. Then stay for two hours
  and answer everything — the comment thread is what decides the post.

Do not cross-post to r/DataHoarder on the same day. Wait for the reception here
first, and if it is good, post there in a fortnight with the storage angle
instead.

---

## Title

Pick one. The first is the safest — it states the differentiator without
adjectives.

1. `Theia — a media server that is one Go binary: no Docker, no account, no config file`
2. `I got tired of a compose file for four films, so I wrote a media server that is a single binary`
3. `Theia: self-hosted films and series in one 17 MB binary, GPL-3.0`

---

## Body

I kept bouncing off the same wall with media servers: a compose file, a
database container, a reverse proxy and an account to sign into — for a folder
of films on a machine in my own house. So I wrote the thing I actually wanted.

**Theia is one binary.** You download it, run it, point it at a folder. There
is no configuration file to write, no container, no account, and no paid tier.
It is GPL-3.0.

What that means concretely:

- **17 MB on disk.** The Go server, SQLite, the compiled frontend and both fonts
  are linked into the executable. No `node_modules` on the host, no application
  server, no separate database process.
- **~18 MB of RAM** at rest, ~22 MB during a sustained remux.
- **No CGO**, so it cross-compiles to Windows, macOS and Linux on amd64 and
  arm64 from one CI job, and there is no libc version to match on the target.
- The only runtime dependency is **FFmpeg**, downloaded on first need from a
  pinned release with a hard-coded SHA-256, and only if a file actually needs
  remuxing.

**What it does:** scans your folders and keeps watching them, pulls metadata and
artwork from TMDB, handles films and TV series with per-episode resume, direct
plays what the browser can already decode and remuxes what it cannot, gives each
person in the house their own resume history, and has a built-in WireGuard
listener for watching from outside — device-keyed, viewer-only, no relay and no
control plane.

The interface is responsive and built for a D-pad, so it works on the television
without a separate app. French by default, English is a complete second
catalogue, switchable without a reload.

**What it deliberately does not do**, because this is the part that will save
you a download:

- **There is no login on the LAN port.** Anyone who can reach it on your network
  can browse, stream, and change settings. That is a deliberate decision for a
  single-household server on a trusted network, and it is why you should not
  forward that port. Remote access is a separate WireGuard listener with a
  viewer-only capability set.
- No hand-editing of metadata. You can *replace* a wrong TMDB match by picking
  from the records it passed over, but you cannot type a synopsis in.
- No image subtitles (PGS, VobSub) — they cannot be shown without burning them
  into the picture. They are named rather than silently missing. Text subtitles
  can be nudged in and out of sync when a rip was muxed from a different cut.
- No live TV, no DVR, no plugins. Permanently.
- No HTTPS on the LAN, no PWA, no native apps, no background-service installer.

It is early — first release was three weeks ago — and it is one person. I am
saying that plainly because "will this still exist next year" is the right
question to ask about anything posted here. What I can point at instead of
promises: every decision in the project is written down with the reasoning and,
where it applies, the bug that forced it. There are 84 of them so far.

Download: https://github.com/Benitoow/theia-media/releases/latest
Source: https://github.com/Benitoow/theia-media
Screenshots and the comparison against Plex/Jellyfin/Emby are in the README.

Happy to answer anything, including the awkward questions about the LAN port.

---

## Two comments to have ready

You will get these. Having a straight answer in the first hour matters more than
the post.

**"Why not just use Jellyfin?"**
Jellyfin is more capable than this and is the right answer for most people — it
has clients on every platform, live TV, plugins and a real community. Theia is
narrower on purpose: one household, films and series, one file to run. If you
want the plugin ecosystem, use Jellyfin. If a compose file for a folder of films
annoys you, this might suit.

**"No auth on the LAN is insane."**
It is the v1 threat model, stated rather than hidden, and the README warns about
it in a box before anything else. The assumption is a trusted home network and a
port you do not forward. If that assumption does not hold for your network, this
is not the right software for you today, and I would rather you knew that before
downloading than after.

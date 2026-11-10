# The screenshots

The pictures in the README of this repository show **Theia 5.1.1** on a **sample
library**, taken on 10 November 2026 in the Electric Flame colours on pure OLED
black (decision 239). They replace the 5.0 set of 7 November, which showed the
earlier gold palette. None of them is a mockup, and none shows anybody's real
library.

## The sample library

- **132 films and 40 series, 300 files,** all recent: 2023 to 2026. Every file is a
  two-second black clip named the way a household names its files; they carry
  nothing worth watching. `scripts/sample-library/make.mjs` and `titles.json` in
  the private repository build them.
- **Posters, backdrops, synopses, ratings and episode names are TMDB's,** fetched by
  a real Theia server exactly as it does for anybody's library, in English. This
  product uses the TMDB API but is not endorsed or certified by TMDB. The artwork
  belongs to its studios and is shown as the product shows it.
- **One film is not a clip:** *Dune: Part Two* stands in with a 2 h 46 min video
  made from its own TMDB backdrop, panned and zoomed, so the player has a picture
  to draw and the timeline preview has different scenes to show. It is not the
  film.
- **The household is invented:** seven profiles (Alex and six friends), some films
  watched, some part-way, and a handful of series under way. The faces of the
  friends are generated gradients with no letters or features, so nobody can be
  mistaken for a real person.

The server was a throwaway one (port 8395, a copy of the sample data folder), built
from the 5.1.1 tree and reporting version 5.1.1, never the maintainer's real
library. This time it was cut off from everything, TMDB included: the posters,
backdrops and texts were already in the sample data's cache, so nothing left the
machine, and nothing reached Theia's Firebase database or GitHub. The film stand-in
is the 2 h 46 min video again, swapped in for its two-second clip for the
duration of the session and put back afterwards. The friends sheet needed the
Theia+ early-access code that the beta accepts for tests (`THEIA-PLUS-TEST`), typed
into the throwaway server only.

## What each picture is

| Picture | How it was taken |
| --- | --- |
| `onboarding`, `profiles`, `home`, `home-rows`, `library`, `film`, `series`, `series-detail`, `search`, `settings`, `settings-playback`, `friends` | The 5.1.1 interface of the native player (the current `player/ui` build), rendered in Microsoft Edge (1400 x 825 CSS pixels at 2x, a little taller where a page needs it) against the sample server, with only the window's own commands answered by a stand-in (the server's address, the window size). The page is served by a small proxy as `theia.localhost:5200`, which is the address the settings overview prints. The title bar's buttons are drawn by the interface itself. It is the same page the native window shows; it is not a photograph of the native window. |
| `friends` | As above, plus two alterations: every friend is shown as living in another household, and the friends service is reported reachable, because the sample server is cut off from Firebase. The friends' presence (online, watching, up for a film, do not disturb, inactive, offline) is the server's own, kept alive by a script posting the beacons a player would. The picture is the sheet alone, whole. |
| `phone-*` | The Android application, a debug build of the 5.1.1 tree, running in an Android emulator (a Pixel 7 profile, Android 16, 1080 x 2400), connected to the sample server through `adb reverse`. The page was driven over devtools and each picture is the emulator's own screen capture, status bar included. `phone-connect` is the first screen with no server saved, and shows the camera scan of decision 240. It is not a photograph of a physical phone. |
| `tv-*` | The same application in an Android TV emulator (Google TV system image, Android 16, 1920 x 1080), driven with the keys a remote sends; the system's first-run setup was skipped with `settings put` before the application was installed. The ring is the interface's own focus ring. It is not a photograph of a television, and no television was involved. |
| `player` | **Not a native capture any more.** The player's own interface (the timeline preview, the island, the bar) rendered in Edge, after Resume was pressed on *Dune: Part Two* and the window's status frames were answered by a stand-in. Edge has no video engine, so the picture behind the controls is the film's TMDB backdrop, zoomed and panned. The pointer is at 2:02:17 and the scene above the bar is decoded at that second by the sample server from the 2 h 46 min stand-in video; it is that video's frame, not the film's. The real Windows player's capture of 20 October 2026 (4.4.2, gold palette) is retired because it no longer showed the product. |

Three things were left out on purpose. The *Server & sources* page of the settings
prints the folders of the machine the pictures were taken on, so it is not shown.
The welcome page of the server's own web interface prints the machine's addresses,
so it is not shown either. And no picture claims that a given phone or television
plays a given file: the emulators show how the screens look, not what a device can
decode.

Not retaken, and not claimed: the native window itself (the real player draws the
film with its own engine, which this session did not run), and the site's
recording, which is a capture of the shipped application and still shows the
earlier palette.

## Replacing them

Rebuild the sample server from the tree, seed the household, and shoot the same
screens. The scripts that built the library, seeded the household, kept the friends
present and drove the pages (`make.mjs`, `seed.mjs`, `friends.mjs`, `proxy.mjs`,
`shoot-readme.mjs`) are in `scripts/sample-library/` of the private repository. They
are not published here. The 10 November 2026 set was shot with the tour scripts of
`.theia-private/ui-tour-20261110/` (`final.mjs` for the interface, `phone.mjs` and
`tv.mjs` for the emulators, `og.mjs` for the share card), which are not tracked.

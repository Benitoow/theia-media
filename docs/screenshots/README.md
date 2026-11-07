# The screenshots

The pictures in the README of this repository show **Theia 5.0** on a **sample
library**, taken on 7 November 2026. None of them is a mockup, and none shows
anybody's real library.

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

The server was a throwaway one (port 8395, its own data folder), built from the 5.0
tree and reporting version 5.0.0, never the maintainer's real library. Every
outbound connection except TMDB was refused, so nothing reached Theia's Firebase
database or GitHub.

## What each picture is

| Picture | How it was taken |
| --- | --- |
| `onboarding`, `profiles`, `home`, `home-rows`, `library`, `film`, `series`, `series-detail`, `search`, `settings`, `settings-playback`, `friends` | The 5.0 interface of the native player, rendered in Microsoft Edge (1400 x 825 CSS pixels at 2x, a little taller where a page needs it) against the sample server, with only the window's own commands answered by a stand-in (the server's address, the window size). The title bar's buttons are drawn by the interface itself. It is the same page the native window shows; it is not a photograph of the native window. |
| `friends` | As above, plus two alterations: every friend is shown as living in another household, and the friends service is reported reachable, because the sample server is cut off from Firebase. The friends' presence (online, watching, up for a film, do not disturb, inactive, offline) is the server's own, kept alive by a script posting the beacons a player would. The picture is cropped to the sheet. |
| `phone-*` | The Android application, a debug build of the 5.0 tree, running in an Android emulator (a Pixel 7 profile, Android 16), connected to the sample server. The status bar is the emulator's. It is not a photograph of a physical phone. |
| `tv-*` | The same application in an Android TV emulator (Google TV system image, Android 16, 1920 x 1080), driven with the keys a remote sends. The ring is the interface's own focus ring. It is not a photograph of a television, and no television was involved. |
| `player` | The real Windows player, built from the 4.4.2 source, in a 1400 x 825 window at 200 % scaling, driven with real mouse input and photographed on 20 October 2026. It shows the timeline preview: the pointer is at 2:02:17 and the scene is decoded at that second by the player's second engine instance. The window has no sound, and its picture is the video described above. It has not been retaken for 5.0. |

Two things were left out on purpose. The *Server & sources* page of the settings prints
the folders of the machine the pictures were taken on, so it is not shown. And no
picture claims that a given phone or television plays a given file: the emulators
show how the screens look, not what a device can decode.

## Replacing them

Rebuild the sample server from the tree, seed the household, and shoot the same
screens. The scripts that built the library, seeded the household, kept the friends
present and drove the pages (`make.mjs`, `seed.mjs`, `friends.mjs`, `proxy.mjs`,
`shoot-readme.mjs`) are in `scripts/sample-library/` of the private repository. They
are not published here.

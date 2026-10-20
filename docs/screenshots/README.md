# The screenshots

The pictures in the README of this repository show Theia 4.4.2 on a **sample
library**, taken on 20 October 2026. None of them is a mockup.

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
- **The household is invented:** three profiles (Alex, Sam, Noor), some films
  watched, some part-way, a few on the watch-later list, and a handful of series
  under way. The faces of the friends are generated gradients.

The server was a throwaway one (port 8395, its own data folder), never the
maintainer's real library, and every outbound connection except TMDB was refused,
so nothing reached Theia's Firebase database or GitHub.

## What is native and what is not

| Picture | How it was taken |
| --- | --- |
| `library`, `film`, `series`, `series-detail`, `settings`, `home-rows`, `player` | The real Windows player, built from the 4.4.2 source, in a 1400x825 window at 200 % scaling, driven with real mouse input and photographed. |
| `home`, `friends` | The same interface rendered in Edge, against the same sample server. The sample server is cut off from Firebase, so its friends answer was altered in two ways: every friend is shown as living in another household, and the friends service is reported reachable. Friends in your own house are not counted in the library bar, and an unreachable service shows a warning, which is why. Their presence (online, watching, up for a film, do not disturb, inactive, offline) is the server's own, kept alive by a script posting the beacons a player would. |

`player` shows the timeline preview: the pointer is at 2:02:17 and the scene is
decoded at that second by the player's second engine instance. The window has no
sound, and its picture is the video described above.

## Replacing them

Rebuild the player and the sample server from the same tree, seed the household,
and photograph the same screens. The scripts that built the library, seeded the
household and drove the window are in `scripts/sample-library/` of the private
repository, with the steps in its README. They are not published here.

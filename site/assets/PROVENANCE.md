# Provenance of the page's media

Decision 164 and design system §12.1 ask for the source of every asset, and the
right to use it, to be recorded beside it. This is that record, written on
2 October 2026 from the files themselves. It is documentation: the build does
not ship it.

Two kinds of image live on the page and are never confused. **The recording is a
real capture** of the shipped application. **The illustrations are generated**,
are labelled *Illustration* wherever they appear, and are never framed as a
screenshot.

## The four illustrations

Generated with GPT Image (OpenAI) and supplied by the maintainer, who asked on
2 October 2026 for them to be used as illustrations on the page. The originals
are PNG files in the maintainer's `Downloads` folder, saved between 13:11 and
13:15 that day. Each embeds a Content Credentials (C2PA) manifest, in a `caBX`
chunk, whose text names OpenAI; that is how the origin was established. The
conversion to WebP drops the manifest, which is one reason the page labels them.
The prompts were not kept.

| Published file | Original | Original SHA-256 | Where it is used |
| --- | --- | --- | --- |
| `img/hero-statue.webp`, `img/hero-statue-narrow.webp` | `Statue de marbre face à l’aube.png`, 1672 × 941. The narrow file is the right-hand 912 px, scaled to 800 × 826 | `53674c7c836f9d24a2080aac69f19d845626a0adb6bcb27fbd5c91a7eb061424` | The hero, and the share card |
| `img/moment-library.webp` | `Galerie sombre aux cadres dorés lumineux.png`, 1448 × 1086 | `ad4cc6a33c61b4bde5509899eed2fe0111e91294e687a9a09ef621632b3fdfcb` | "Your library" |
| `img/moment-player.webp` | `Main de marbre tenant un astre doré.png`, 1730 × 909, cropped to 1212 × 909 (x 420 to 1632) | `87a9150e3f28eae53d5569b4ada4fbe64923658c46e82c60313dc05ff3606dfe` | "The player" |
| `img/moment-remote.webp` | `Villas reliées par un fil doré.png`, 1448 × 1086 | `dd1acd615a13f7f37f880c0bff20bd46b87314c551286603514a2e65632ac793` | "Away from home" |

Copyright in AI-generated images is unsettled in several jurisdictions, as it is
for AI-assisted code (decision 163). Nothing here was scraped or fetched from the
web (CLAUDE.md, "No unverified image").

## The recording

| | |
| --- | --- |
| Published | `video/theia-install-tour.mp4`: H.264 High, 1920 × 1080, 30 fps, no audio, 1 min 44 s, about 12 MB |
| Source | `TUTO_THEIA_3K_60FPS.webm`, supplied by the maintainer: VP9, **1920 × 1080** (the "3K" in its name is not its size), 60 fps, no audio, 206,511,566 bytes, muxed by Mediabunny. SHA-256 `6aaff7765909f739b742da5629880c034e8b002820f3c555e0bfa9acb82fc989` |
| Poster | `video/theia-install-tour-poster.webp`: the frame at 58.5 s, 1600 × 900, WebP quality 76 |
| Captions | `video/theia-install-tour.en.vtt`: written from the recording, scene times checked against its frames |

Re-encoded with the FFmpeg that Theia manages (Jellyfin FFmpeg 8.1.2, which has
libx264 and libwebp; no `ffprobe` is installed on this machine):

```sh
ffmpeg -y -i TUTO_THEIA_3K_60FPS.webm -an -map_metadata -1 \
  -vf "fps=30,scale=1920:1080:flags=lanczos,format=yuv420p" \
  -c:v libx264 -preset slower -crf 27 -profile:v high -level 4.1 -g 90 -keyint_min 30 \
  -movflags +faststart -x264-params colorprim=bt709:transfer=bt709:colormatrix=bt709 -color_range tv \
  -metadata title="Theia 3.4.0 install and tour (screen recording, no sound)" theia-install-tour.mp4
ffmpeg -y -ss 58.5 -i TUTO_THEIA_3K_60FPS.webm -frames:v 1 -vf scale=1600:900:flags=lanczos \
  -c:v libwebp -quality 76 -compression_level 6 -preset picture theia-install-tour-poster.webp
```

The images, from the originals listed above (`-compression_level 6 -preset picture`
in every case):

```sh
ffmpeg -i "Statue de marbre face à l’aube.png" -c:v libwebp -quality 90 hero-statue.webp
ffmpeg -i "Statue de marbre face à l’aube.png" -vf "crop=912:941:760:0,scale=800:-2:flags=lanczos" -c:v libwebp -quality 86 hero-statue-narrow.webp
ffmpeg -i "Galerie sombre aux cadres dorés lumineux.png" -c:v libwebp -quality 90 moment-library.webp
ffmpeg -i "Main de marbre tenant un astre doré.png" -vf "crop=1212:909:420:0" -c:v libwebp -quality 90 moment-player.webp
ffmpeg -i "Villas reliées par un fil doré.png" -c:v libwebp -quality 90 moment-remote.webp
ffmpeg -i assets/theia-logo.png -vf "crop=1170:330:190:350,scale=600:-2:flags=lanczos" -c:v libwebp -quality 88 theia-wordmark.webp
```

**What it is.** A real screen capture of Theia **3.4.0** on Windows, composed by
the recording tool on a gradient backdrop. The version is visible in Settings,
under Server and Update (`v3.4.0`). The setup it runs is the published one: the
file `theia-setup-windows-amd64.exe` in the maintainer's `Downloads` folder has
the same SHA-256 (`c81fdd23fc0f7a9885a68103ce89530177b5d38c64df10ddbb0d043de805767d`)
and size (72,216,613 bytes) as the GitHub release asset.

**Scenes, in order**, with the times the page and the captions use:

| Time | Scene |
| --- | --- |
| 0:00 | The GitHub release page of v3.4.0 in a web browser; the assets list; the setup for Windows x64 |
| 0:19 | Windows Terminal running `theia-setup`: language, role, data folder, port, network name, film folder, autostart, plan, "Installation finished." |
| 0:40 | The desktop; a third-party launcher finds the Theia entries; Theia Player opens |
| 0:56 | Home, a film page (cast), the films and series lists, a series with its episodes, search |
| 1:17 | Settings: Interface, Playback, Subtitles, Viewing, Server (`http://127.0.0.1:8383`, `v3.4.0`), Update |
| 1:36 | Profiles, then "Personnaliser le profil": name, "Changer la photo", "Retirer la photo" |

**What it does not show.** A film playing. A profile photo being changed: the
editor opens and closes, and the avatar in the navigation is the same picture
every time the navigation is visible, from about 1:00 to the last frame. Any
system other than Windows. The interface is in
French; the page says so.

**The profile editor is in the published release.** At the tag `v3.4.0`,
`player/ui/src/lib/catalogues.js` carries `personalizeProfile`,
`changePicture` and `removePicture`, and `player/ui/src/components/ProfileDialog.tsx`
implements them (decision 136 records the avatar work). It is not a 4.0 feature.
The V4 profile chooser ("Who's watching?", `docs/v4.md`) is a different, future
screen, and nothing on the page presents it as available.

**Rights not verified.** The recording shows things the maintainer did not make
for Theia, and the right of each to appear in a public recording has not been
checked:

- posters, backdrops and portraits of real people, supplied by TMDB for two
  titles of the maintainer's own library (*The Deer Hunter*, 1978, and
  *Star Wars: Maul - Shadow Lord*, 2026), visible in the Browse scene (0:56 to
  1:17), in the viewing statistics of the Settings scene and in the poster;
- a desktop wallpaper that is a still from a film, not identified, around the
  windows throughout;
- a profile photo of a cat wearing glasses and an ID badge, with a small "AI"
  mark, whose origin is unknown. It is probably the file `ШишкаБой.jpg` saved in
  `Downloads` the same morning (65,015 bytes, SHA-256
  `de9bbc6c29a1b402801fd71e9b09be7aec374dab0fe767706f56d690fb6fa3ef`);
  that file is **not** published and is not on the page.

The clean fix is to record it again with the repository's own demo media, a
neutral wallpaper and an avatar made for the purpose, which is what §12.1 asks.

## The mark, the icons and the share card

| File | What it is |
| --- | --- |
| `img/theia-wordmark.webp` | A crop (x 190, y 350, 1170 × 330), scaled to 600 px wide, of `assets/theia-logo.png`, the repository's prestige logo (SHA-256 `ec101250872a049fad617cb9e523f77bf40f1fe078227d50da4f6d49036bc912`). It is already public in the README; this page does not re-check the licence of its photographic base (founding spec §11.10) |
| `img/favicon-16.png`, `img/favicon-32.png`, `img/apple-touch-icon.png` | Copied unchanged from `web/static/`: crops of `assets/theia-icon.png`, itself a crop of the maintainer-supplied `assets/logo_icon_TNY.png` (SHA-256 `54046c5d9c50af5973b17a2298148d6b1a913ee6e59e6ff7241fc6a1751691c5`). **Source and licence not yet recorded** (decision 167) |
| `img/og-image.jpg` | The share card, 1200 × 630: the hero illustration, the wordmark and the title set in Cinzel, composed in HTML and rendered with the repository's Playwright. It carries the *Illustration* label |

## Fonts

Cinzel Variable and Jost Variable, the latin subsets, from the
`@fontsource-variable` 5.3.0 packages under `web/node_modules`. SIL Open Font
License 1.1; the licence texts travel beside the files in `fonts/`.

## Not on the page

The TMDB logo. Decision 10 keeps text alone, because no copy of the logo has had
its licence checked; the footer carries the attribution sentence verbatim.

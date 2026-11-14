# Provenance of the page's media

Decision 164 and design system §12.1 ask for the source of every asset, and the
right to use it, to be recorded beside it. This is that record, written on
2 October 2026 from the files themselves and updated on 26 October 2026 for the
illustration removal. It is documentation: the build does not ship it.

Three kinds of media live on the page. **The recording is a real capture** of the
shipped application. **The screenshots capture the real Theia interface** on a
sample library, labelled *Screenshot*, with their native/browser distinction
recorded below. **The hero is the maintainer-supplied ASCII film leader**, used as a
decorative background without a visible attribution label at the maintainer's
explicit request on 23 October 2026. It is never presented as a product capture.
At the maintainer's request on 26 October 2026, all GPT Image illustrations were
removed, including the old artwork in the link-sharing image.

## The ASCII hero

Supplied by the maintainer as `ascii-magic.webm`, with an explicit request to use
it in the hero. It is a monochrome ASCII rendition of a cinema film leader,
not an application recording. No external image or video was fetched.

| Published file | Source and transformation |
| --- | --- |
| `video/hero-ascii.webm` | The supplied file, unchanged: VP8, 1200 × 900, about 10.97 s, 110 frames with variable timing (about 10 fps), no audio, 2,354,356 bytes. SHA-256 `3ceeb548a1b7b4c17a53e96142f0331c94def3aaca1df0e85eb789c0e705f35f` |
| `video/hero-ascii.mp4` | Preferred H.264 High stream, 800 × 600, 24 fps, no audio, 1,541,343 bytes, metadata removed, fast-start enabled |
| `video/hero-ascii-poster.webp` | Frame at 4.4 s, 800 × 600, WebP quality 78, 115,418 bytes |

The supplied file identifies Chrome as its encoder, but does not identify the
creator or licence of the underlying film leader. The maintainer authorized its
use; third-party rights have not been independently established. This record
does not invent a public-domain or generated-media claim.

Derivatives use the same managed FFmpeg as the recording below:

```sh
ffmpeg -i ascii-magic.webm -an -map_metadata -1 \
  -vf "fps=24,scale=800:600:flags=lanczos,format=yuv420p" \
  -c:v libx264 -crf 30 -preset medium -profile:v high -level 3.1 \
  -movflags +faststart hero-ascii.mp4
ffmpeg -ss 4.4 -i ascii-magic.webm -frames:v 1 \
  -vf "scale=800:600:flags=lanczos" -c:v libwebp -quality 78 \
  -compression_level 6 hero-ascii-poster.webp
```

The background covers the entire hero, starts silently when visible, and loops.
Reduced motion, data saving, no JavaScript or unavailable playback keep the
local poster. The native product recording below still requires user action.

## Retired illustrations

Generated with GPT Image (OpenAI) and supplied by the maintainer, who asked on
2 October 2026 for them to be used as illustrations on the page. The originals
are PNG files in the maintainer's `Downloads` folder, saved between 13:11 and
13:15 that day. Each embeds a Content Credentials (C2PA) manifest, in a `caBX`
chunk, whose text names OpenAI; that is how the origin was established. The
conversion to WebP dropped the manifest, which is why they were labelled.
The prompts were not kept.

None of these files is used or shipped by the site now. `moment-player.webp`
and `moment-remote.webp` were removed on 26 October 2026; the share card was
rebuilt from the existing library screenshot, without the statue or image logo.

| Former file | Original | Original SHA-256 | Former use |
| --- | --- | --- | --- |
| `img/hero-statue.webp`, `img/hero-statue-narrow.webp` (retired on 23 October 2026) | `Statue de marbre face à l’aube.png`, 1672 × 941. The narrow file was the right-hand 912 px, scaled to 800 × 826 | `53674c7c836f9d24a2080aac69f19d845626a0adb6bcb27fbd5c91a7eb061424` | Former hero and, until 26 October 2026, the share card |
| `img/moment-player.webp` | `Main de marbre tenant un astre doré.png`, 1730 × 909, cropped to 1212 × 909 (x 420 to 1632) | `87a9150e3f28eae53d5569b4ada4fbe64923658c46e82c60313dc05ff3606dfe` | "The player" |
| `img/moment-remote.webp` | `Villas reliées par un fil doré.png`, 1448 × 1086 | `dd1acd615a13f7f37f880c0bff20bd46b87314c551286603514a2e65632ac793` | "Away from home" |

Copyright in AI-generated images is unsettled in several jurisdictions, as it is
for AI-assisted code (decision 163). Nothing here was scraped or fetched from the
web (CLAUDE.md, "No unverified image").

## The three product screenshots

The library and player captures were added on 20 October 2026, replacing the
illustration `moment-library.webp` (removed), which showed a dark gallery with
empty frames where the library now shows itself. The friends crop was added on
26 October 2026, replacing the last illustration in the product cards. All three
and the share card were retaken on 10 November 2026 for the 5.1.1 interface (the
Electric Flame colours on pure OLED black, decision 239), because the earlier ones
showed the gold palette and a button the product no longer draws, and again the same
evening once the wordmark underline and the profile glow were removed (decision 241).

| Published file | What it is | Where it is used |
| --- | --- | --- |
| `img/shot-library.webp` | The *All movies* screen of the 5.1.1 interface, a 1400 x 1050 window rendered in Edge at 2x, scaled to 1200 x 900 (no crop) | "A library that feels like yours", labelled *Screenshot* |
| `img/shot-player.webp` | The player's own interface on *Dune: Part Two* with the pointer on the timeline at 2:02:17, rendered in Edge, scaled to 1600 x 943. The picture behind the controls is the film's TMDB backdrop, standing in for the video (Edge has no video engine); the scene above the bar is the sample server's decode of the stand-in video at that second | "A player built for your files", labelled *Screenshot* |
| `img/shot-friends.webp` | The real friends interface in 5.1.1 rendered in Edge against the sample server, a 1400 x 825 window at 2x scaled to 1200 x 707, showing invented friends watching, online and up for a film | "Closer, even from a distance", labelled *Screenshot* |
| `img/og-image.jpg` | The share card, 1200 x 630: the wordmark, the headline and one line of the site's own words on pure black, and the library capture. Composed by `.theia-private/ui-tour-20261110/og.mjs` (not tracked) from the site's own Cinzel and Jost files. No image other than the product capture is in it | the link-sharing preview |

All come from the sample library described in `docs/screenshots/README.md` of the
public repository: 132 recent films and 40 series, every file a two-second clip
except one stand-in film, with posters, backdrops, synopses and ratings from TMDB, in
a throwaway server with no connection but TMDB. The artwork belongs to its studios and
is shown as the product shows it; the TMDB sentence is in the footer. The stand-in
film is a 2 h 46 min video made from the title's own TMDB backdrop, panned and zoomed:
the picture in the player and in the preview is not the film. Nothing here was
fetched from anywhere else, and no image of a real household or library is shown.

The three pictures are captures of the 5.1.1 interface taken on 10 November 2026
with the tour scripts of `.theia-private/ui-tour-20261110/` (not tracked), against a
throwaway sample server cut off from every network. As documented beside the README
screenshots (`docs/screenshots/README.md`), that browser bench marks the sample
friends as living in another household and the service as reachable; Firebase is
blocked, and the throwaway server had the Theia+ test code activated so that the
friends sheet is unlocked. They are interface captures with sample data, not
evidence of a real network session. The published SHA-256 digests are:
- `shot-library.webp`: `4fc5bd7fa337895e766604adf0f76fbf913a74766f7a15a8a7af0e4f48bb8322`
- `shot-player.webp`: `cf44786dda0432ea42041c9cf44d9b8c8655951ff0b9b8998322e7f48a52fe21`
- `shot-friends.webp`: `e9715ed8c98a56ac944c662f7a895b5d8d73878dbe7d2d50ecde8c49ebb5e04d`
- `og-image.jpg`: `c34b4122868e9bf801c4a067f27cc3d8aad3ca18ddb7e673d2b1023f14fcef2a`

The site uses the player's 1600:943 aspect ratio for its three screenshot
windows, with cover fitting anchored at the top. The player stays entirely
visible; the library is cropped at the bottom. Captions sit below the images
instead of covering the playback controls. No screenshot is stretched or padded.
The recording below is not part of this retake: it is a capture of the shipped
application from 22 October 2026 and still shows the earlier palette.

```sh
# the 2800-pixel captures of `final.mjs`: site-library.png (1400 x 1050 window),
# player.png and site-friends.png (1400 x 825 windows), all at 2x
ffmpeg -i site-library.png -vf "scale=1200:900:flags=lanczos" -c:v libwebp -quality 82 -compression_level 6 shot-library.webp
ffmpeg -i player.png -vf "scale=1600:943:flags=lanczos" -c:v libwebp -quality 82 -compression_level 6 shot-player.webp
ffmpeg -i site-friends.png -vf "scale=1200:707:flags=lanczos" -c:v libwebp -quality 82 -compression_level 6 shot-friends.webp
```

## The recording

| | |
| --- | --- |
| Published | `video/theia-install-tour.mp4`: H.264 High, 1920 × 1080, 30 fps, no audio, 1 min 30 s, about 11 MB (10,982,499 bytes), the Explorer window blurred |
| Source | `VidéoTheia.webm`, supplied by the maintainer on 22 October 2026: VP9 (Profile 0), 1920 × 1080, 60 fps, no audio, 170,715,480 bytes, muxed by Mediabunny 1.61.0. SHA-256 `207eff8da87bb540daca7cdddc7d1a11e5e5942cbbcf19d250258362e43144ee` |
| Poster | `video/theia-install-tour-poster.webp`: the frame at 78.0 s (the home screen), 1600 × 900, WebP quality 76 |
| Captions | Ten `video/theia-install-tour.<locale>.vtt` tracks generated from the strict site catalogues; the 13 shared cue times were checked against the original recording. The default follows the site language. The translations are AI-authored and have not been reviewed by native speakers. |

Re-encoded with the FFmpeg that Theia manages (Jellyfin FFmpeg 8.1.2, which has
libx264 and libwebp; no `ffprobe` is installed on this machine):

```sh
ffmpeg -y -i "VidéoTheia.webm" -an -map_metadata -1 \
  -filter_complex "[0:v]fps=30,split[a][b];[b]gblur=sigma=28:steps=2,format=yuva420p,fade=t=in:st=49.5:d=0.3:alpha=1,fade=t=out:st=58.0:d=0.3:alpha=1[bl];[a][bl]overlay=format=auto,scale=1920:1080:flags=lanczos,format=yuv420p[v]" -map "[v]" \
  -c:v libx264 -preset slower -crf 30 -profile:v high -level 4.1 -g 90 -keyint_min 30 \
  -movflags +faststart -x264-params colorprim=bt709:transfer=bt709:colormatrix=bt709 -color_range tv \
  -metadata title="Theia 4.4.4 install and tour (screen recording, no sound)" theia-install-tour.mp4
ffmpeg -y -ss 78.0 -i "VidéoTheia.webm" -frames:v 1 -vf scale=1600:900:flags=lanczos \
  -c:v libwebp -quality 76 -compression_level 6 -preset picture theia-install-tour-poster.webp
```

The quality factor is 30. At 27 the same recording weighed 17 MB; the terminal's
text stays legible at 30, and the file keeps the size of the previous recording (about 12 MB).

The first filter blurs the whole picture from 0:49.8 to 0:58.0, with a 0.3 s
fade at each end, over the Explorer window of the install scene (visible from
0:50.5 to 0:57.5). It was checked on frames sampled inside and at the edges of
that window: no file or folder name can be read in it.

**What it is.** A real screen capture of Theia **4.4.4** on Windows, composed by
the recording tool, with camera moves, on a gradient backdrop. The version is
visible in the installer (`4.4.4` under its name) and on the GitHub release page
(*Theia v4.4.4*, marked Latest). It has **not** been checked that the setup it
downloads is the published one: the file in the Downloads folder is named
`theia-setup-windows-amd64`, and the release list shows the Windows x64 setup at
67.9 MB. Windows, the browser and Explorer are in French; Theia is in English.

**Scenes, in order**, with the times the page and the captions use, checked against
frames sampled every second around each change:

| Time | Scene |
| --- | --- |
| 0:00 | The GitHub repository, then the release page of v4.4.4, its assets list and the download of the setup for Windows x64 (it starts at about 0:21) |
| 0:31 | The setup opened from the Downloads folder in Windows Terminal: language, purpose, films (the folder is copied from Explorer, blurred from 0:50 to 0:58, and pasted), summary at 1:03, installation, "Theia is installed" at 1:08 |
| 1:10 | The Windows Firewall prompt for `theia-server`, allowed |
| 1:15 | "Who's watching?", then the home screen (*The Deer Hunter*), the films and the series |
| 1:26 | Settings, General: the app language and asking who is watching at startup |

**What it does not show.** A film playing. The profile editor, or a profile photo
being changed (the profile has the letter M, not a photo). Friends and rooms. Any
system other than Windows. The page says so.

**Rights not verified.** The recording shows things the maintainer did not make
for Theia, and the right of each to appear in a public recording has not been
checked:

- posters, backdrops and portraits of real people, supplied by TMDB for titles of
  the maintainer's own library: *The Deer Hunter* (1978) on the home screen and in
  the poster, and the artwork of *Cyberpunk: Edgerunners* and *Star Wars: Maul -
  Shadow Lord* in the series list (1:15 to 1:26);
- a desktop wallpaper that is a still from a film, not identified, around the
  windows throughout;
- the maintainer's own computer. The Explorer window of the install scene, which
  listed the folders of his Documents folder and the names of the files in his film
  folder, is blurred from 0:50 to 0:58. What is not blurred: the Windows user name,
  which appears in file paths during the install, and the left-hand list of an
  earlier Explorer window (Downloads, 0:31 to 0:35), which holds a few folder names.

The clean fix is to record it again with the repository's own demo media, a
neutral wallpaper and a folder made for the purpose, which is what §12.1 asks.

## The mark, the icons and the share card

| File | What it is |
| --- | --- |
| Former `img/theia-wordmark.webp` (retired on 23 October 2026) | A crop of the former header wordmark, replaced when the header and share card moved to the local Cinzel face |
| `img/favicon-16.png`, `img/favicon-32.png`, `img/apple-touch-icon.png` | Copied unchanged from `web/static/`: crops of `assets/theia-icon.png`, supplied by the maintainer on 14 November 2026 (decision 247) as rights-cleared artwork replacing the retired earlier image |
| `img/og-image.jpg` | The share card, 1200 × 630: the existing native library screenshot, unchanged in colour and aspect ratio, beside the name and text set in local Cinzel/Jost. Composed in HTML and captured in Edge with the repository's Playwright; labelled *Screenshot / Sample library*. No generated illustration or image logo |

## Fonts

Cinzel Variable and Jost Variable, the latin subsets, from the
`@fontsource-variable` 5.3.0 packages under `web/node_modules`. SIL Open Font
License 1.1; the licence texts travel beside the files in `fonts/`.

## Not on the page

The TMDB logo. Decision 10 keeps text alone, because no copy of the logo has had
its licence checked; the footer carries the attribution sentence verbatim.


## Local script fonts, 24 October 2026

The multilingual site adds Noto Sans Arabic (also Urdu), Noto Sans Devanagari,
Noto Sans Bengali and Noto Sans SC from Google Fonts at commit
`e7db6ca603fd4862010383ff17ffd6cc4ffac1c0`. All four OFL 1.1 licence texts were
read and are shipped in `fonts/LICENSE-notosans*.txt`. Source URLs and SHA-256
digests are recorded in `fonts/sources.json`. Noto Sans SC's reserved name is
`Source`; no derivative is named Source.

`scripts/site-fonts.py` retains OpenType shaping while producing WOFF2 subsets
of the catalogue/endonym characters and Intl date/unit/digit output. The
resulting digests and glyph coverage are in `fonts/coverage.json`. These are
vendored assets: ordinary builds and visitors never contact a font service.
The existing Cinzel/Jost assets and licences are unchanged. The shared social
card retains its original artwork and English tagline; each locale translates
its metadata and image description, not the recorded application or artwork.

## Icons and brand marks, 4 November 2026

The maintainer asked for real company logos and icons, taken from the icon repositories on
GitHub (decision 230). Thirty-five SVG files are vendored in `assets/icons/`, each pinned
to an upstream commit and a SHA-256 in `assets/icons/sources.json`; the licence texts
were fetched from the same commits and are shipped with the site
(`assets/icons/LICENSE-*.txt`, `DISCLAIMER-simple-icons.txt`). Each file went through the
whitelist in `site/icons.mjs` (shapes and numbers only) before it was kept, and the markup
a page receives is rebuilt from the values that passed. They are drawn in
`.theia-private/site-v5/shots/icon-sheet.png` (proof output).

| Set | Upstream, commit | Licence | Files and use |
| --- | --- | --- | --- |
| Lucide 1.61.0 | `lucide-icons/lucide`, `c29c276f16590cf017126671535ad8c792be64d8` (21 October 2026) | ISC, with MIT for the icons derived from Feather (`arrow-right`, `download`, `server`, `smartphone`, `tv` among those used) | 22 interface icons: arrows, play and pause, menu, globe, copy, the three product moments, the eight items of the announcement and its three "not yet" items, film, shield |
| Simple Icons 16.35.0 | `simple-icons/simple-icons`, `ed9a5192b24ad2f169bf1009931da0df27d5a49a` (11 October 2026) | CC0 1.0 for the collection; its disclaimer says an icon may carry its own licence, which was checked per icon below | 12 marks: Apple, Linux, GitHub, Discord, the feed glyph, Firebase, Google Drive, FFmpeg, mpv, SQLite, Go, Svelte |
| SVG Logos | `gilbarbara/logos`, `608007b99fab1d55be5de9f9ec2c75bcc80a438c` (the last commit to touch the file, 18 April 2023) | CC0 1.0 | 1 mark: the Windows logo (`microsoft-windows-icon.svg`), which Simple Icons no longer carries |

**What a mark is for.** It names an operating system (Windows, macOS, Linux: the hero line and
the download pills), a service Theia talks to (GitHub, Firebase and Google Drive on the privacy
page, Google Drive in the announcement), a place to follow it (GitHub, Discord and the
release feed in the footer) or a tool it is built with (the five in "Built with"). It is drawn in
the page's text colour, never the brand's own, and the page says that names and logos belong
to their owners and imply no endorsement.

**Checked per icon, in the Simple Icons data of that commit** (`data/simple-icons.json`): none of the twelve
used carries a licence entry of its own. Two that were considered did, and were not used:
**Tauri** (CC BY-NC-ND 4.0) and **Rust** (CC BY-SA 4.0). Also not used, by choice: the marks of
processor makers (Intel, AMD, Arm), whose guidelines are strict about logos and whose names say the same
thing; **WireGuard**, whose trademark policy fixes the colour of its logo (the privacy page names it in
words); **Ubuntu** (Tux names Linux); and **TMDB**, whose mark stays out as decision 10 says, the attribution
sentence in the footer standing alone. Go's brand page (read on 4 November 2026) says the Go name and logo
may be used freely to refer to the Go language.

**Rights not verified.** Trademarks are used nominatively and no brand's own guidelines were
audited beyond the notes above: Apple, Microsoft (Windows), GitHub, Discord, Firebase, Google Drive,
FFmpeg (a trademark of Fabrice Bellard), mpv, SQLite and Svelte. The guideline links Simple Icons
records are, for GitHub `github.com/logos`, Discord `discord.com/branding`, Firebase
`firebase.google.com/brand-guidelines` and Go `go.dev/brand`. The maintainer can remove any of them: a
brand mark is one `logo(...)` call in `site/build.mjs` or one `{{logo:...}}` in a page template, and
its entry in `sources.json`.

**Fonts, 4 November 2026.** The four script subsets were regenerated from the pinned sources for the 45
new catalogue keys (Chinese 679 to 705 characters, Arabic, Devanagari and Bengali one each); digests and
coverage are in `fonts/coverage.json`.

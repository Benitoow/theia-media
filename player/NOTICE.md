# The engine the player ships

`theia-player` is a small program. The part that decodes and renders films is
**libmpv**, and it is not ours: it ships beside the executable as a separate,
replaceable file, under the GNU Lesser General Public License, version 2.1 or
later.

## What ships, and from where

| | |
|---|---|
| Engine | mpv, built for libmpv by [`zhongfly/mpv-winbuild`](https://github.com/zhongfly/mpv-winbuild) |
| Release | `2026-09-14-0b7ed670f7` |
| Archive | `mpv-dev-lgpl-x86_64-20260914-git-0b7ed670f7.7z` |
| Archive SHA-256 | `d6df1a133b7d60d30b49efac593fd58ff183c48a6d94bb78cd401430acd96dad` |
| Library | `libmpv-2.dll` |
| Library SHA-256 | `6f059354c5c45b41192cc52d867d94c0044edb48207c4efd2ea1244208c55359` |
| Licence | LGPL-2.1-or-later - the full text is in `LICENSE-libmpv.txt` beside this file |
| Build | `-Dgpl=false -Dlibmpv=true -Dcplayer=false`; mpv `v0.41.0-1049-g0b7ed670f`, libplacebo `v7.371.0`, FFmpeg `N-126548-g6efe500d2` |

The same values live in `libmpv.json`, which is what the build reads and what
the player prints in its diagnostics. `go run ./scripts/fetch-libmpv` downloads
the archive, checks both digests above, and refuses to hand anything on if either
disagrees.

## Your rights under the LGPL, and how they are kept

The LGPL asks for four things of a program that ships the library. Each one is a
deliberate choice here, not an accident:

- **The licence text travels with the library.** `LICENSE-libmpv.txt`, copied
  verbatim from the Free Software Foundation, sits next to `libmpv-2.dll` in every
  distribution of the player.
- **You may replace the library with your own build.** It is a separate DLL, never
  statically linked, and the player never embeds it. Point `THEIA_LIBMPV` at any
  build of libmpv 2.x and the player uses that one instead; the file beside the
  executable is only the default. This is the freedom the LGPL exists to protect,
  and it is also how the project is developed.
- **The source is identified, not merely offered.** The exact upstream release and
  both digests are printed by `theia-player --diagnostics`, so a bug report can
  name the engine it was running.
- **Theia's own source is public.** It is GPL-3.0, which the LGPL allows and
  expects for a program that links it.

## What is not claimed

The build's author states plainly that he is not a lawyer and cannot guarantee
that every LGPL-incompatible package was disabled in his build. The maintainer
accepted that residual risk for V3.3, and it is written down in decision 118
rather than smoothed over. If a licence review ever finds a GPL-only component
linked into this build, the answer is not a different download: it is building
libmpv in CI with `-Dgpl=false`, which upstream documents.

Only `windows/amd64` is pinned, because Windows is the only platform the player
has been run and verified on. macOS and Linux will need their own pin, their own
digests and a real run before they appear here.

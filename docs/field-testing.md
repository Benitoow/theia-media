# Theia field test

The current stable release is `v3.4.0`, the final planned V3 release. The next
development line is [V4](v4.md); no further 3.x release is planned. Real household
reports still matter: native CI checks cover installation and a generated film,
while actual libraries, display/audio chains and unusual files need real devices.

Try the desktop apps on Windows x64/ARM64, macOS Intel/Apple Silicon or Ubuntu
24.04 x64/ARM64. The existing web viewer stays available for phones and TVs,
without active viewing support; web settings and administration remain maintained.
The [publication record](v3.4-release-readiness.md) lists tested platform limits.

## Who this is for

You do not need to be a developer. The useful test is a real one: a folder of
films or series you are allowed to use, a computer that can stay on while you
watch, and the desktop apps or existing browser viewer on your home screens. Small, unusual and
messy libraries are as useful as large ones.

## The one-week pass

1. Download the latest release and record the exact version shown by
   `theia-server -version` or Settings.
2. Add a disposable folder first if you want to understand the scan. Then add
   the real folders you intend to use.
3. Use the native desktop app on each supported computer. Record its OS and
   CPU architecture; record the browser if using the existing phone/TV viewer.
4. Try several kinds of media. Note the container, video codec, audio codec,
   subtitle format and resolution when you know them.
5. Exercise profiles, watchlists, film/episode resume, seeking, saved audio and
   subtitle choices, volume, pause, window resizing and fullscreen. Try the next
   episode action and a lower quality where available. Test remote access only
   through Theia's WireGuard flow.
6. Leave Theia running through ordinary library changes, restarts and at least
   a few complete viewing sessions.
7. Submit the [field-test report](https://github.com/Benitoow/theia-media/issues/new?template=field_test.yml).
   Say what worked as well as what failed.

## What makes a report useful

For a playback problem, include the player/browser, OS, CPU architecture and
viewing device, a copied playback diagnostic, and the `Stream #` lines from `ffmpeg -i` when
possible. Give exact reproduction steps and the relevant server output. A
report saying that 4K HEVC direct play, an H.264 remux and resume all worked on
specific devices is useful even without a bug.

Never upload a media file you do not have the right to share. Redact private
paths, IP addresses, tokens, WireGuard keys and personal metadata. Report
security problems through [GitHub's private channel](https://github.com/Benitoow/theia-media/security/advisories/new),
not a public issue. Never expose TCP port `8383` directly to the internet.

## What happens to feedback

Repeated failures and blocked household workflows set the maintenance priority.
Feature ideas are recorded, but they are not a promise or a queue ordered by
votes. When roughly ten real households have produced enough evidence, the
maintainer can decide which library-facing problems justify new features under
decision 97. V4's shared-cinema direction is already accepted (decision 160);
reports help define its implementation and catch regressions in existing playback.

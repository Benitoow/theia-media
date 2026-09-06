# Theia V3.1 field test

V3.1 is Theia's stable feature baseline while the project learns from its first
ten real household libraries. New product features are paused during this phase.
Security fixes, data-loss protections, playback blockers, regressions and
concrete compatibility fixes continue as patch releases: `v3.1.1`, `v3.1.2`,
`v3.1.3` and so on.

## Who this is for

You do not need to be a developer. The useful test is a real one: a folder of
films or series you are allowed to use, a computer that can stay on while you
watch, and one or more browsers on the screens in your home. Small, unusual and
messy libraries are as useful as large ones.

## The one-week pass

1. Download the latest release and record the exact version shown by
   `theia -version` or Settings.
2. Add a disposable folder first if you want to understand the scan. Then add
   the real folders you intend to use.
3. Browse from every screen that matters: desktop, phone, tablet or television.
4. Try several kinds of media. Note the container, video codec, audio codec,
   subtitle format and resolution when you know them.
5. Exercise profiles, watchlists, resume, seeking, direct play, remux and a
   lower quality. Test remote access only through Theia's WireGuard flow.
6. Leave Theia running through ordinary library changes, restarts and at least
   a few complete viewing sessions.
7. Submit the [field-test report](https://github.com/Benitoow/theia-media/issues/new?template=field_test.yml).
   Say what worked as well as what failed.

## What makes a report useful

For a playback problem, include the viewing device and browser, the playback
mode shown by the player, and the `Stream #` lines from `ffmpeg -i` when
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
maintainer will decide which problems justify reopening feature development and
will publish the next roadmap.

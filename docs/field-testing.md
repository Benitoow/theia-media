# Theia field test

The current release is `v4.1.0`, which brings watch parties (see
[the release notes](releases/v4.1.0.md)); 3.4 was the last V3 release. 4.1 is free.
Real household reports still matter: native CI checks cover installation and a
generated film on every platform, while actual libraries, display/audio chains,
unusual files and two real households need real devices. A watch party between
two households on macOS or Ubuntu is the report most wanted.

Try the desktop apps on Windows x64/ARM64, macOS Intel/Apple Silicon or Ubuntu
24.04 x64/ARM64. The existing web viewer stays available for phones and TVs,
without active viewing support; web settings and administration remain maintained.
The tested platform limits are at the end of the
[4.1 release notes](releases/v4.1.0.md) and the
[4.0 release notes](releases/v4.0.0.md).

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
6. If you have more than one profile, check that **Who's watching?** appears at
   startup, that your answer is remembered, and that the setting to skip it works.
7. Run the installer over an existing installation: it should offer to update in
   place, keep your data and start the server again. Note what it suggested for
   your film folders and whether the suggestions were right.
8. Leave Theia running through ordinary library changes, restarts and at least
   a few complete viewing sessions.
9. Submit the [field-test report](https://github.com/Benitoow/theia-media/issues/new?template=field_test.yml).
   Say what worked as well as what failed.

## Testing friends between two households

This is the part of 4.0 that most needs real reports. It has been verified with
two real servers on one machine and in the native player; two earlier tests
between real households found faults that are now fixed, and a clean run over the
internet is still the proof wanted.

1. Both people install 4.0, with the server running and at least one profile.
2. In the player, open the friends sheet. Each person copies their profile's code
   and sends it to the other, by any channel.
3. One person enters the other's code. They should be told at once which profile
   it reached, or that the person does not exist. The other person should then see
   a request with **Accept** and **Decline**, without pasting anything back.
4. After accepting, start a film on one side. The other side should see it as a
   card, and stop seeing it about 90 seconds after it ends.
5. Try it with remote access off on both sides, then with it on. If a router
   refuses to open a port by itself, the friends sheet lets you type its public
   address.

Report which of these steps worked, what each side's sheet said, and the router
or network type if you know it (a home router, a mobile connection, a company
network). **A friend code contains the sender's public address:** redact it, like
an IP address, before pasting a report in public.

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
maintainer can decide which library-facing problems justify new features.
V4's shared-cinema direction is already accepted, and 4.1 brings watch parties; reports help define that implementation and catch
regressions in existing playback. Which later features might become paid is not
decided and is not decided by votes.

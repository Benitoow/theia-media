# Theia field test

The current release is `v4.4.4`, the latest update of V4 (see
[the release notes](releases/v4.4.4.md), which also hold what 4.4.2 and 4.4.3
brought). 4.4.4 is free. Real household reports still matter: native CI checks cover installation and a generated film on every
platform, while actual libraries, display/audio chains, unusual files and two
real households need real devices. A **private watch party between two
households**, and a week of presence between friends, are the reports most
wanted.

Try the desktop apps on Windows x64, macOS Intel/Apple Silicon or Ubuntu 24.04
x64. The Windows and Ubuntu ARM64 setups are deferred and keep 4.2.1, which does
not have the 4.4 features. The existing web viewer stays available for phones and
TVs, without active viewing support; web settings and administration remain
maintained. The platform limits, and what was and was not checked, are in the
[4.4.4 release notes](releases/v4.4.4.md).

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
   subtitle choices, volume, pause, window resizing and fullscreen. Move the
   pointer along the playback bar and check that the still matches the scene you
   land on. Press the skip arrows several times in a row. Try a lower quality
   where available. Test remote access only through Theia's WireGuard flow.
6. Watch a series through two or three episodes. If a file names its opening or
   its ending in its chapters, **Skip opening** and the 30-second **Next episode**
   countdown should appear; if not, nothing extra should appear. **Skip opening**
   should leave the picture by itself after about eight seconds and come back when
   you move the mouse. Near the end of an episode, move the pointer onto the
   playback bar: the end-of-episode buttons should step aside and the countdown
   should wait until you leave the bar. Check that the countdown can be cancelled
   and that it never runs on past the last episode of a season. This is the
   behaviour of 4.4.4 that was not tried on a real episode in the native player
   before publication, so a report on it, working or not, is wanted.
7. Open Settings and change a few things: the skip step, audio and subtitle
   languages, an audio output, the interface scale. Close and reopen the player
   and check that they stayed. Note anything you could not find.
8. If you have more than one profile, check that **Who's watching?** appears at
   startup, that your answer is remembered, and that the setting to skip it works.
9. Run the installer over an existing installation: it should offer to update in
   place, keep your data and start the server again. Note what it suggested for
   your film folders and whether the suggestions were right. From 4.4.2 the
   player should not look for updates until you ask, or turn on **Automatic
   updates**. If you are on 4.4.2 or 4.4.3, update from *Settings → Updates* with
   **Check for updates**: no setup is needed, and the new interface applies the
   next time the player starts.
10. Leave Theia running through ordinary library changes, restarts and at least
    a few complete viewing sessions. Close the window after a film: it should
    close without having to end the task.
11. Submit the [field-test report](https://github.com/Benitoow/theia-media/issues/new?template=field_test.yml).
    Say what worked as well as what failed.

## Testing friends and presence between two households

This is the part of V4 that most needs real reports. It has been verified with
two real servers on one machine and in the native player, and presence has been
tried between two households on a test build; a clean week over the internet on
the released version is still the proof wanted.

1. Both people install 4.4.4, with the server running and at least one profile.
2. In the player, open the friends sheet. Each person copies their profile's code
   and sends it to the other, by any channel.
3. One person enters the other's code. They should be told at once which profile
   it reached, or that the person does not exist. The other person should then see
   a request with **Accept** and **Decline**, without pasting anything back. A
   request should wait if the other person's computer is off.
4. After accepting, start a film on one side. The other side should see it as a
   card, and stop seeing it about 90 seconds after it ends.
5. Look at the library bar: the number and the faces of the friends who are
   around. Leave one player untouched for the chosen time: that friend should
   read as *Inactive*. Try *Up for a film*, *Do not disturb* and *Invisible*, and
   hide the title: the other side should see exactly that and nothing more.
6. Remove the friendship on one side. The other side should lose the row too, even
   if its server was off at the time.
7. Write an opinion on a film, check that it shows as your own review, and that
   your friend can read it.

Report which of these steps worked, what each side's sheet said, and the router
or network type if you know it (a home router, a mobile connection, a company
network). **A friend code contains the sender's public address:** redact it, like
an IP address, before pasting a report in public.

## Testing a private room

Rooms need remote access on both sides, and **both people need 4.4.2 or later**
(test with 4.4.4): an older server refuses to create or join a private room.

1. One person starts a film and opens a room. It should say *Private room*, with
   nobody invited yet.
2. Invite one friend. Only that friend should be offered the room; a second friend
   who was not invited should not see it.
3. The friend joins. Both screens should stay on the same second through play,
   pause and seeks. A friend without the film can watch the host's copy.
4. Turn on requests to join, and ask from a third profile if you have one: the
   host should answer the request before anyone is admitted.
5. Let the film reach the end of an episode in a series: one countdown should run,
   once, for everybody, and stop at the last episode of a season.
6. Close the room. Nobody should be able to rejoin it.

## What makes a report useful

For a playback problem, include the player/browser, OS, CPU architecture and
viewing device, a copied playback diagnostic, and the `Stream #` lines from `ffmpeg -i` when
possible. Give exact reproduction steps and the relevant server output. A
report saying that 4K HEVC direct play, an H.264 remux and resume all worked on
specific devices is useful even without a bug.

Two problems from the 4.4.2 work were reported and never reproduced: a freeze
after pressing the skip arrows many times, and a window that would not close after
a film. If either happens to you, say what you did just before it.

Never upload a media file you do not have the right to share. Redact private
paths, IP addresses, tokens, WireGuard keys and personal metadata. Report
security problems through [GitHub's private channel](https://github.com/Benitoow/theia-media/security/advisories/new),
not a public issue. Never expose TCP port `8383` directly to the internet.

## What happens to feedback

Repeated failures and blocked household workflows set the maintenance priority.
Feature ideas are recorded, but they are not a promise or a queue ordered by
votes. When roughly ten real households have produced enough evidence, the
maintainer can decide which library-facing problems justify new features. V4 is
complete, and 4.4.4 is its latest update; reports now catch regressions in
existing playback and sharing and decide what is worth doing next. Which later features might become
paid is not decided and is not decided by votes.

# Linux media engine

This package contains Ubuntu 24.04's unmodified libmpv2 0.37.0-1ubuntu4,
distributed under GPL-3.0-or-later. Theia is GPL-3.0. The full GPL text is in
LICENSE-libmpv.txt. COPYRIGHT-libmpv.txt preserves Ubuntu's copyright notice;
LICENSE-GPL-2.txt and LICENSE-LGPL-2.1.txt supply its referenced licence texts.
The library is separate and replaceable with THEIA_LIBMPV.

Corresponding source (upstream source and Ubuntu packaging changes):

- https://archive.ubuntu.com/ubuntu/pool/universe/m/mpv/mpv_0.37.0.orig.tar.xz
- https://archive.ubuntu.com/ubuntu/pool/universe/m/mpv/mpv_0.37.0-1ubuntu4.debian.tar.xz
- https://archive.ubuntu.com/ubuntu/pool/universe/m/mpv/mpv_0.37.0-1ubuntu4.dsc

Exact binary/archive digests and architecture are in player/libmpv.json in
Theia's source. The build verifies both before packaging. Ubuntu's desktop,
codec, audio and driver dependencies are installed by the operating system;
this package does not redistribute them. On Ubuntu 24.04 install libmpv2,
libwebkit2gtk-4.1-0 and libayatana-appindicator3-1 before starting the player.
An X11 session or XWayland is required. This first Linux release uses software
decoding and GTK-painted software rendering in SDR. Hardware acceleration and
HDR output are not available. Codec support depends on Ubuntu's libraries.

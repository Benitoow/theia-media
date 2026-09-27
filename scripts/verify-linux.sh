#!/usr/bin/env bash
#
# Verifies the Linux player on a real Linux machine.
#
# It builds nothing and downloads nothing: it runs the player that is already
# there, reads what it reported, and takes a picture. The macOS verifier
# (`scripts/verify-macos.sh`) is the sibling; the difference is that Linux embeds
# through an X11 window id rather than drawing through a render bridge, so what is
# checked first here is that the id was there and the film landed in the window.
#
#   ./scripts/verify-linux.sh -Player player/target/release/theia-player
#
#   -Player   the player to run                       (required)
#   -Media    the film to play                        (a fixture is generated if absent)
#   -Out      where the log and the picture go        (default: ./linux-proof)
#   -Display  the X display to use                    (default: $DISPLAY or :0)
#   -Engine   libmpv to load through THEIA_LIBMPV     (default: what is beside the player)
#   -Seconds  how long to let it play                 (default: 20)
#
# What it asserts, and what it does not. The engine must load and name itself, the
# window id handed to mpv must not be zero, and the reported position must
# advance. Those three are things a program can see. **The picture is not
# asserted**: whether the film is visible under the OSD is an eye's answer, and
# the picture is written for one - the same division the macOS verifier makes
# (decision 149: a playback position is not a picture).
#
# A Wayland session has no window id at all, so this fails there on purpose: the
# film plays in the engine's own window and cannot be composited under the OSD.
# `RENDER-LINUX.md` records what that session needs instead.
set -uo pipefail

Player=""
Media=""
Out=""
Display="${DISPLAY:-:0}"
Engine=""
Seconds=20

while [ $# -gt 0 ]; do
    case "$1" in
        -Player) Player="$2"; shift 2 ;;
        -Media) Media="$2"; shift 2 ;;
        -Out) Out="$2"; shift 2 ;;
        -Display) Display="$2"; shift 2 ;;
        -Engine) Engine="$2"; shift 2 ;;
        -Seconds) Seconds="$2"; shift 2 ;;
        -h|--help) sed -n '2,30p' "$0"; exit 0 ;;
        *) echo "unknown argument: $1" >&2; exit 2 ;;
    esac
done

if [ -z "$Player" ]; then echo "-Player is required" >&2; exit 2; fi
if [ ! -x "$Player" ]; then echo "$Player is not executable" >&2; exit 2; fi
[ -n "$Out" ] || Out="$PWD/linux-proof"
mkdir -p "$Out"
Out="$(cd "$Out" && pwd)"
Media="${Media:-$Out/fixture.mp4}"

passed=0
failed=0
for_a_person=0
ok() { echo "  ok   $1"; passed=$((passed + 1)); }
bad() { echo "  FAIL $1"; failed=$((failed + 1)); }
look() { echo "  look $1"; for_a_person=$((for_a_person + 1)); }

echo "== the film"
if [ ! -f "$Media" ]; then
    if ! command -v ffmpeg >/dev/null 2>&1; then
        echo "no $Media and no ffmpeg to make one" >&2
        exit 2
    fi
    ffmpeg -hide_banner -loglevel error -y \
        -f lavfi -i "testsrc2=size=1280x720:rate=25" -t 20 \
        -c:v libx264 -preset veryfast -pix_fmt yuv420p -movflags +faststart "$Media" \
        || { echo "ffmpeg could not make a fixture" >&2; exit 2; }
fi
echo "  $Media ($(stat -c%s "$Media") bytes)"

log="$Out/player.log"
rm -f "$log"
echo "== running $Player on $Display for ${Seconds}s"
if [ -n "$Engine" ]; then export THEIA_LIBMPV="$Engine"; fi
export DISPLAY="$Display"
setsid "$Player" --media "$Media" --diagnostics --mute >"$log" 2>&1 </dev/null &
sleep "$Seconds"
pkill -f 'theia-player --media' 2>/dev/null
sleep 1

echo "== what it reported"
pin=$(grep -m1 '^engine-pin: ' "$log" || true)
if [ -z "$pin" ]; then
    bad "the player never loaded an engine (no engine-pin line)"
else
    loaded=$(printf '%s' "$pin" | grep -o '"loaded":"[^"]*"' | head -1 | cut -d'"' -f4)
    if [ -n "$loaded" ]; then ok "engine loaded: $loaded"; else bad "the engine did not load: $pin"; fi
fi

wid=$(grep -m1 '^theia-player: window id ' "$log" | awk '{print $4}')
if [ -z "$wid" ]; then
    bad "the player never printed the window id it handed to mpv"
elif [ "$wid" = "0" ]; then
    bad "mpv was given no window id: this session has no X11 window, so the film is not embedded (RENDER-LINUX.md)"
else
    ok "mpv was given the X11 window id $wid"
fi

first=$(grep -o '"pos":[0-9.]*' "$log" | head -1 | cut -d: -f2)
last=$(grep -o '"pos":[0-9.]*' "$log" | tail -1 | cut -d: -f2)
if [ -z "$first" ] || [ -z "$last" ]; then
    bad "the player never reported a position; see $log"
elif awk "BEGIN { exit !($last > $first) }"; then
    ok "the position advanced: $first -> $last"
else
    bad "the position did not advance ($first -> $last); see $log"
fi

vo=$(grep -o '"vo":"[^"]*"' "$log" | tail -1 | cut -d'"' -f4)
[ -n "$vo" ] && ok "the video output in use is $vo"

echo "== the picture"
shot="$Out/screen.png"
if command -v ffmpeg >/dev/null 2>&1; then
    if ffmpeg -hide_banner -loglevel error -y -f x11grab -video_size 1280x720 -i "$Display" -frames:v 1 "$shot" 2>/dev/null && [ -s "$shot" ]; then
        look "$shot - is the film visible, and is the OSD above it?"
    else
        bad "the screen could not be captured on $Display"
    fi
else
    look "no ffmpeg to capture with; run the player by hand and look"
fi

echo
echo "$passed passed, $failed failed, $for_a_person for a person to look at"
echo "log: $log"
[ "$failed" -eq 0 ]

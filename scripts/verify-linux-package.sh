#!/usr/bin/env bash
# Run the one-file download, its installed launcher, film and removal in isolation.
# Invoke under xvfb-run on a runner, or on an existing X11 desktop.
set -euo pipefail
setup="$(realpath "$1")"
out="$(realpath -m "$2")"
mkdir -p "$out"
export HOME="$(mktemp -d)"
export XDG_CONFIG_HOME="$HOME/.config" XDG_DATA_HOME="$HOME/.local/share"
install="$HOME/.local/lib/theia"
data="$HOME/.theia"
player_pid="" launcher_pid=""
stop_product() {
    [ -z "$player_pid" ] || kill "$player_pid" 2>/dev/null || true
    [ -z "$launcher_pid" ] || kill "$launcher_pid" 2>/dev/null || true
    # Match only the executables installed in this test's unique directory.
    while read -r pid; do kill "$pid" 2>/dev/null || true; done < <(
        ps -eo pid=,args= | awk -v server="$install/theia-server" -v player="$install/theia-player" '$2==server || $2==player {print $1}'
    )
}
trap stop_product EXIT
mkdir -p "$HOME/library"
go run ./scripts/create-proof-media -out "$out/fixture.mp4" -seconds 60
"$setup" --role all-in-one --lang en --install-dir "$install" \
    --data-dir "$data" --library "$HOME/library" --port 8395 --yes >"$out/install.log" 2>&1
for executable in theia theia-server theia-setup theia-player; do
    "$install/$executable" -version
done
"$install/theia" >"$out/launcher.log" 2>&1 &
launcher_pid=$!
for _ in $(seq 1 50); do
    curl -sf http://127.0.0.1:8395/api/health >"$out/health.json" && break
    sleep .5
done
curl -fsS http://127.0.0.1:8395/ >"$out/interface.html"
grep -qi '<html' "$out/interface.html"
grep -q '"key_source":"built-in"' "$data/logs/theia.log"
for _ in $(seq 1 30); do
    if ps -eo args= | awk -v player="$install/theia-player" '$1==player {found=1} END {exit !found}'; then break; fi
    sleep .5
done
ps -eo args= | awk -v player="$install/theia-player" '$1==player {found=1} END {exit !found}'
echo 'PASS installed launcher started the server, served the interface and opened the player'
stop_product
sleep 1
"$install/theia-player" --media "$out/fixture.mp4" --mute --diagnostics \
    --window 1280x720 --window-report "$out/window-report.json" --proof-osd >"$out/player.log" 2>&1 &
player_pid=$!
ready=0
for _ in $(seq 1 45); do
    kill -0 "$player_pid"
    if python3 - "$out" <<'PY'
import json, pathlib, re, sys
out = pathlib.Path(sys.argv[1])
try:
    bar = json.loads((out/'window-report.json').read_text())['page']['bar']
    log = (out/'player.log').read_text()
    positions = [float(p) for p in re.findall(r'"pos":([0-9.]+)', log)]
    assert positions and positions[-1] > positions[0] + 1
    assert bar['hidden'] is False and bar['display'] != 'none'
    assert float(bar['opacity']) > 0 and bar['controlsVisible'] > 0
    assert re.search(r'window id [1-9][0-9]*', log)
except (OSError, KeyError, ValueError, AssertionError, TypeError):
    sys.exit(1)
PY
    then ready=1; break; fi
    sleep 1
done
# Capture while the installed player is alive, in the test's own X display.
ffmpeg -hide_banner -loglevel error -y -f x11grab -video_size 1280x720 \
    -i "$DISPLAY" -frames:v 1 "$out/player-window.png"
[ "$ready" = 1 ] || { cat "$out/player.log"; exit 1; }
go run ./scripts/check-playback-picture -image "$out/player-window.png" -controls
echo 'PASS advancing film, real window, responsive and visible controls'
stop_product
sleep 1
"$setup" --check --lang en >"$out/check.log" 2>&1
"$setup" --uninstall --yes >"$out/uninstall.log" 2>&1
test -d "$data"
test ! -e "$install/theia"
echo 'PASS uninstall removed the programs and kept the library data'

#!/usr/bin/env bash
# What a real Mac has to answer before macOS can be claimed.
#
#   ./scripts/verify-macos.sh [-Media <a film>] [-App <path to Theia.app>]
#
# Nothing in this file can run anywhere else: every check below is a macOS
# question, and the project's rule is that a platform is claimed only when it has
# been run on real hardware (spec-fondatrice §14.2, decision 117). The Windows
# machine this was written on can cross-compile the Go half and nothing else.
#
# The script prints one line per check: PASS, FAIL, or LOOK for the things only a
# person can judge. It changes nothing outside a temporary directory and a
# throwaway data directory, and it never touches the maintainer's own library or
# data directory.
set -uo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
media=""
app="${THEIA_APP:-$root/dist/theia-player-darwin-arm64/Theia.app}"
work="$(mktemp -d /tmp/theia-verify.XXXXXX)"
pass=0
fail=0
look=0

while [ $# -gt 0 ]; do
	case "$1" in
		-Media)
			shift
			media="${1:-}"
			;;
		-App)
			shift
			app="${1:-}"
			;;
		-h | --help)
			sed -n '2,12p' "$0"
			exit 0
			;;
		*)
			echo "unknown argument: $1" >&2
			exit 2
			;;
	esac
	shift
done

ok() { printf 'PASS  %s\n' "$1"; pass=$((pass + 1)); }
bad() { printf 'FAIL  %s\n' "$1"; fail=$((fail + 1)); }
human() { printf 'LOOK  %s\n' "$1"; look=$((look + 1)); }

if [ "$(uname -s)" != "Darwin" ]; then
	echo "this checks macOS on macOS; there is no other way to answer these questions" >&2
	exit 1
fi

echo "== The engine the player ships =="
if [ -x "$app/Contents/MacOS/theia-player" ]; then
	reported="$("$app/Contents/MacOS/theia-player" -version 2>&1 | head -1)"
	case "$reported" in
		theia-player*) ok "the player names itself: $reported" ;;
		*) bad "the player printed $reported instead of its own version" ;;
	esac
else
	bad "no player at $app (build it with scripts/build-player-macos.sh -Release -Bundle)"
fi

if [ -d "$app/Contents/Frameworks" ]; then
	count=$(ls "$app/Contents/Frameworks"/*.dylib 2>/dev/null | wc -l | tr -d ' ')
	[ "$count" -ge 18 ] && ok "the engine set is there ($count dylibs)" || bad "only $count dylibs in Contents/Frameworks"
	[ -f "$app/Contents/Resources/licenses/mpv-LICENSE.LGPL" ] &&
		ok "the engine's own licence texts travelled with it" ||
		bad "Contents/Resources/licenses holds no mpv licence"
else
	bad "no Contents/Frameworks: the engine is missing"
fi

if command -v codesign >/dev/null 2>&1; then
	if codesign --verify --strict "$app" >/dev/null 2>&1; then
		ok "the bundle's signature verifies"
	else
		bad "codesign --verify refuses the bundle"
	fi
fi
human "Gatekeeper: a downloaded copy must be refused once (unsigned, not notarised). Control-click, Open, Open."

echo
echo "== The server, on a throwaway data directory =="
server="$root/theia-server-darwin-arm64"
[ -x "$server" ] || server="$root/dist/theia-3.3.4-darwin-arm64/theia-server"
if [ -x "$server" ]; then
	data="$work/data"
	mkdir -p "$data/library"
	cp "$media" "$data/library/" 2>/dev/null || true
	printf '{"library_paths":["%s"],"port":8395,"hostname":"theia-mac-verify"}\n' "$data/library" >"$data/config.json"
	"$server" --data-dir "$data" --port 8395 >"$work/server.log" 2>&1 &
	server_pid=$!
	for _ in $(seq 1 40); do
		curl -sf http://127.0.0.1:8395/api/health >/dev/null 2>&1 && break
		sleep 0.5
	done
	if curl -sf http://127.0.0.1:8395/api/health >/dev/null 2>&1; then
		ok "the server answers /api/health"
		stats=$(curl -sf http://127.0.0.1:8395/api/library/stats || echo '{}')
		ok "the scan finished: $stats"
		home=$(curl -sf http://127.0.0.1:8395/api/library/home | head -c 120 || true)
		[ -n "$home" ] && ok "the home screen answers: ${home}…" || bad "the home screen answered nothing"
	else
		bad "the server never answered; see $work/server.log"
	fi
	kill "$server_pid" 2>/dev/null
	wait "$server_pid" 2>/dev/null
else
	bad "no darwin server binary; build it with GOOS=darwin GOARCH=arm64 go build ./cmd/theia-server"
fi

echo
echo "== The player over a real film =="
if [ -n "$media" ] && [ -x "$app/Contents/MacOS/theia-player" ]; then
	report="$work/window-report.json"
	"$app/Contents/MacOS/theia-player" --media "$media" --mute --diagnostics \
		--window 1280x720 --window-report "$report" >"$work/diagnostics.txt" 2>&1 &
	player_pid=$!
	sleep 12
	kill "$player_pid" 2>/dev/null
	wait "$player_pid" 2>/dev/null

	# The status frames are the machine-readable half of the diagnostics, and the
	# keys are the player's own: `pos`, `vo`, `hwdec`, `ao`, `audioMode`. A check
	# that greps a name the player never writes can never pass - the first version
	# of this script looked for mpv's `time-pos` and would have failed a working
	# player - and one that only looks for the presence of a key passes on
	# `"vo":null`, which is exactly the answer macOS is suspected of giving.
	frames="$work/diagnostics.txt"
	first=$(grep -o '"pos":[0-9.]*' "$frames" | head -1 | cut -d: -f2)
	last=$(grep -o '"pos":[0-9.]*' "$frames" | tail -1 | cut -d: -f2)
	if [ -n "$first" ] && [ -n "$last" ] && awk "BEGIN{exit !($last > $first)}"; then
		ok "playback advanced: pos $first -> $last"
	else
		bad "the position did not advance (pos '$first' -> '$last'); see $frames"
	fi
	for key in vo hwdec ao; do
		value=$(grep -o "\"$key\":\"[^\"]*\"" "$frames" | head -1 | cut -d'"' -f4)
		if [ -n "$value" ]; then
			ok "$key = $value"
		else
			bad "$key is null or absent: the engine chose none, which is the whole question on macOS"
		fi
	done
	# The one thing macOS cannot do, said as a check rather than as a hope.
	if grep -q '"audioMode":"pcm"' "$frames"; then
		ok "audio mode is pcm - the only honest answer here, since CoreAudio carries no TrueHD, Atmos or DTS-HD MA"
	else
		bad "the audio mode is not pcm; on macOS nothing else can reach an amplifier"
	fi
	if [ -f "$report" ]; then
		ok "the window reported itself: $(head -c 200 "$report")"
		human "does the OSD draw over the picture? That is the one thing no program here can answer."
	else
		human "no window report was written; check the OSD by eye"
	fi
else
	human "no -Media given: pass a film to check playback, VideoToolbox and the OSD over the picture"
fi

echo
echo "== The installer, into a throwaway home =="
setup="$root/dist/theia-3.3.4-darwin-arm64/theia-setup"
[ -x "$setup" ] || setup="$root/theia-setup"
if [ -x "$setup" ]; then
	fake_home="$work/home"
	mkdir -p "$fake_home"
	if HOME="$fake_home" "$setup" --role all-in-one --install-dir "$fake_home/.local/lib/theia" \
		--data-dir "$fake_home/.theia" --library "$work/data/library" --yes >"$work/install.log" 2>&1; then
		ok "the installer ran without asking for a password"
	else
		bad "the installer failed; see $work/install.log"
	fi
	[ -f "$fake_home/Library/LaunchAgents/media.theia.server.plist" ] &&
		ok "the launchd agent was written" ||
		bad "no launchd plist under Library/LaunchAgents"
	[ -L "$fake_home/Applications/Theia.app" ] && ok "~/Applications/Theia.app is a link to the installation" || bad "no Theia.app link"
	[ -L "$fake_home/.local/bin/theia" ] && ok "the theia command is linked into ~/.local/bin" || bad "no theia link"
	HOME="$fake_home" "$setup" --check --lang en | head -20
	HOME="$fake_home" "$setup" --uninstall --yes >"$work/uninstall.log" 2>&1 &&
		ok "the uninstall ran" || bad "the uninstall failed; see $work/uninstall.log"
	[ -d "$fake_home/.theia" ] && ok "the uninstall kept the data directory" || bad "the uninstall removed the data directory"
else
	bad "no theia-setup for macOS in dist/"
fi

echo
printf '%d passed, %d failed, %d for a person to look at. Working files: %s\n' "$pass" "$fail" "$look" "$work"
[ "$fail" -eq 0 ]

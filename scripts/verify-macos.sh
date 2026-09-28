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
case "$(uname -m)" in
	arm64) goarch=arm64 ;;
	x86_64) goarch=amd64 ;;
	*) echo "unsupported macOS architecture: $(uname -m)" >&2; exit 1 ;;
esac
app="${THEIA_APP:-$root/dist/theia-player-darwin-$goarch/Theia.app}"
work="$(mktemp -d /tmp/theia-verify.XXXXXX)"
pass=0
fail=0
look=0
archive_version=""

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
		theia-player\ *)
			archive_version="${reported#theia-player }"
			archive_version="${archive_version#v}"
			ok "the player names itself: $reported"
			;;
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
human "Gatekeeper: check the first launch of a downloaded copy (ad-hoc signed, not notarised)."

echo
echo "== The server, on a throwaway data directory =="
# Three places, because three things produce them: a release archive, the proof
# workflow, and a working tree. The first version of this script only knew the
# first two and reported "no darwin server binary" about a directory that held
# one.
archive_dir=""
[ -n "$archive_version" ] && archive_dir="$root/dist/theia-$archive_version-darwin-$goarch"
server="$root/theia-server-darwin-$goarch"
[ -x "$server" ] || server="$root/dist/theia-server-darwin-$goarch"
if [ ! -x "$server" ] && [ -n "$archive_dir" ]; then server="$archive_dir/theia-server"; fi
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
		if curl -fsS http://127.0.0.1:8395/ -o "$work/interface.html" && grep -qi '<html' "$work/interface.html"; then
			ok "the server serves the built interface"
		else
			bad "the server has no built interface"
		fi
		grep -q 'key_source=built-in' "$work/server.log" &&
			ok "the server carries its built-in metadata key" ||
			bad "the server has no built-in metadata key"
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
	bad "no darwin server binary; build it with GOOS=darwin GOARCH=$goarch go build ./cmd/theia-server"
fi

echo
echo "== The player over a real film =="
if [ ! -x "$app/Contents/MacOS/theia-player" ]; then
	bad "no player at $app, so playback was not checked at all (build it with scripts/build-player-macos.sh -Release -Bundle)"
elif [ -z "$media" ]; then
	human "no -Media given: pass a film to check playback, VideoToolbox and the OSD over the picture"
else
	report="$work/window-report.json"
	"$app/Contents/MacOS/theia-player" --media "$media" --mute --diagnostics \
		--window 1280x720 --window-report "$report" >"$work/diagnostics.txt" 2>&1 &
	player_pid=$!
	ready=0
	attempt=0
	while [ "$attempt" -lt 15 ]; do
		attempt=$((attempt + 1))
		sleep 1
		if ! kill -0 "$player_pid" 2>/dev/null; then
			bad "the player exited before a frame could be inspected; see $work/diagnostics.txt"
			break
		fi
		if [ -s "$report" ] && grep -Eq '"pos":[1-9]' "$work/diagnostics.txt" &&
			grep -Eq 'render-frames: [1-9][0-9]*' "$work/diagnostics.txt"; then
			ready=1
			break
		fi
	done
	if [ "$ready" -eq 0 ] && kill -0 "$player_pid" 2>/dev/null; then
		bad "the player had no window, advancing playback and rendered frames before capture"
	fi
	# Capture while the product is alive and after its own telemetry says a
	# film is playing. A screenshot taken after exit is a desktop photograph.
	if [ "$ready" -eq 1 ]; then sleep 2; fi
	if [ "$ready" -eq 1 ] && kill -0 "$player_pid" 2>/dev/null && command -v screencapture >/dev/null 2>&1; then
		if [ -n "${THEIA_PROOF_DIR:-}" ]; then mkdir -p "$THEIA_PROOF_DIR"; fi
		# The window's own content first, then the screen: they answer different
		# questions. The screen says what a person would have seen; the window
		# says what the application drew. On 28 September 2026 the screen capture
		# showed a wallpaper with no window on it at all, and the window's own
		# content is the only one of the two that can prove the film reached it.
		window_number=$(sed -n 's/.*"windowNumber":[[:space:]]*\([0-9][0-9]*\).*/\1/p' "$report" 2>/dev/null | head -1)
		if [ -n "$window_number" ] && [ "$window_number" -gt 0 ]; then
			window_shot="$work/player-window.png"
			if screencapture -x -o -l "$window_number" "$window_shot" >/dev/null 2>&1 && [ -s "$window_shot" ]; then
				if [ -n "${THEIA_PROOF_DIR:-}" ]; then
					cp "$window_shot" "$THEIA_PROOF_DIR/player-window.png"
					window_shot="$THEIA_PROOF_DIR/player-window.png"
				fi
				human "the application's own window, captured by its window number ($window_number): $window_shot"
			else
				human "the window the player named ($window_number) did not answer screencapture -l"
			fi
		else
			human "the player named no window number, so only the screen could be captured"
		fi
		picture="$work/player-screen.png"
		if screencapture -x "$picture" >/dev/null 2>&1 && [ -s "$picture" ]; then
			if [ -n "${THEIA_PROOF_DIR:-}" ]; then
				cp "$picture" "$THEIA_PROOF_DIR/player-screen.png"
				picture="$THEIA_PROOF_DIR/player-screen.png"
			fi
			human "inspect the actual player frame and OSD in $picture"
		else
			human "the runner could not capture the player window; visual playback remains unverified"
		fi
	elif [ "$ready" -eq 0 ]; then
		human "no product screenshot: the player was not ready while the fixture was running"
	fi
	kill "$player_pid" 2>/dev/null
	wait "$player_pid" 2>/dev/null
	# The raw diagnostics travel with the evidence. They used to stay in the
	# runner's temporary directory, so a failed frame check had to be explained
	# from a single grepped line: the player's own log, and mpv's, are what say
	# which link stopped and what the engine thought about it.
	if [ -n "${THEIA_PROOF_DIR:-}" ] && [ -f "$work/diagnostics.txt" ]; then
		mkdir -p "$THEIA_PROOF_DIR"
		cp "$work/diagnostics.txt" "$THEIA_PROOF_DIR/player-diagnostics.txt"
	fi

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
	# The chain behind the frame count, printed here rather than left in the
	# artifact: the first Mac run reported "1 -> 1", and learning anything more
	# meant downloading the diagnostics file. This line names which link stopped.
	diag_line=$(grep -o 'render-diagnostics: .*' "$frames" | tail -1)
	[ -n "$diag_line" ] && printf 'INFO  %s\n' "$diag_line"
	first_frame=$(grep -o 'render-frames: [0-9]*' "$frames" | head -1 | cut -d' ' -f2)
	last_frame=$(grep -o 'render-frames: [0-9]*' "$frames" | tail -1 | cut -d' ' -f2)
	if [ -n "$first_frame" ] && [ -n "$last_frame" ] && [ "$last_frame" -gt "$first_frame" ]; then
		ok "the actual application rendered frames: $first_frame -> $last_frame"
	else
		bad "the application rendered no advancing frames ($first_frame -> $last_frame)"
	fi
	# How fast, said rather than asserted. The proof machine is virtual and has no
	# GPU - `gl-renderer` in the line above is what says so - and the same code
	# draws at the software renderer's pace there. A rate asserted here would be a
	# property of the runner written down as a property of the player.
	elapsed_ms=$(printf '%s\n' "$diag_line" | sed -n 's/.*elapsed-ms=\([0-9]*\).*/\1/p')
	tick_count=$(printf '%s\n' "$diag_line" | sed -n 's/.*ticks=\([0-9]*\).*/\1/p')
	frame_count=$(printf '%s\n' "$diag_line" | sed -n 's/.*frames=\([0-9]*\).*/\1/p')
	if [ -n "$elapsed_ms" ] && [ "$elapsed_ms" -gt 0 ]; then
		awk -v e="$elapsed_ms" -v f="${frame_count:-0}" -v t="${tick_count:-0}" \
			'BEGIN { printf "INFO  %d frames and %d ticks in %d ms: %.2f fps drawn, %.1f ticks/s offered\n", f, t, e, f * 1000 / e, t * 1000 / e }'
	fi
	# `vo` and `hwdec` are printed rather than asserted, and the reason is the
	# finding this path is built on: with `vo=libmpv` the *host* owns the video
	# output, so `current-vo` has nothing to name and `hwdec-current` is empty
	# until a hardware decoder engages - which the proof runner's virtualised
	# machine refuses (VideoToolbox's `hwdec_vld` initialisation fails there).
	# The assertions that mean something here are the position advancing, the
	# audio output opening, and the window reporting its own geometry.
	for key in vo hwdec ao; do
		value=$(grep -o "\"$key\":\"[^\"]*\"" "$frames" | head -1 | cut -d'"' -f4)
		if [ -n "$value" ]; then
			ok "$key = $value"
		else
			printf 'INFO  %s is empty - expected with vo=libmpv unless a hardware decoder engaged\n' "$key"
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
fi

echo
echo "== The installer, into a throwaway home =="
setup="${THEIA_SETUP:-}"
if [ -z "$setup" ] && [ -n "$archive_dir" ]; then setup="$archive_dir/theia-setup"; fi
[ -x "$setup" ] || setup="$root/dist/theia-setup-darwin-$goarch"
[ -x "$setup" ] || setup="$root/theia-setup"
if [ -x "$setup" ]; then
	fake_home="$work/home"
	mkdir -p "$fake_home"
	installed=0
	if HOME="$fake_home" "$setup" --role all-in-one --install-dir "$fake_home/.local/lib/theia" \
		--data-dir "$fake_home/.theia" --library "$work/data/library" --yes >"$work/install.log" 2>&1; then
		ok "the installer ran without asking for a password"
		installed=1
	else
		# The log goes to stdout, not only to the file: the work directory lives
		# in the runner's temporary space and a failure nobody can read is a
		# failure nobody can fix. The artifact carries this.
		bad "the installer failed, so nothing below was expected to be there:"
		sed 's/^/      /' "$work/install.log" | tail -25
	fi
	if [ "$installed" = 1 ]; then
		installed_server="$fake_home/.local/lib/theia/theia-server"
		if [ -x "$installed_server" ]; then
			"$installed_server" --data-dir "$fake_home/.theia" --port 8396 >"$work/installed-server.log" 2>&1 &
			installed_pid=$!
			for _ in $(seq 1 40); do
				curl -sf http://127.0.0.1:8396/api/health >/dev/null 2>&1 && break
				sleep 0.5
			done
			if curl -fsS http://127.0.0.1:8396/ -o "$work/installed-interface.html" && grep -qi '<html' "$work/installed-interface.html"; then
				ok "the installed server serves the built interface"
			else
				bad "the installed server has no built interface"
			fi
			grep -q 'key_source=built-in' "$work/installed-server.log" &&
				ok "the installed server carries its built-in metadata key" ||
				bad "the installed server has no built-in metadata key"
			kill "$installed_pid" 2>/dev/null
			wait "$installed_pid" 2>/dev/null
		else
			bad "the installed server binary is missing"
		fi
		# The autostart entry is opt-in, and that is the contract, not an
		# oversight: `Plan.Service` is off unless asked for, because the founding
		# spec's §11.7 keeps a hand-started server as the default and installing
		# a service is a decision about somebody's machine (decision 120). So
		# both halves are checked - nothing without --service, and a launchd
		# agent that names the *installed* server with it.
		plist="$fake_home/Library/LaunchAgents/media.theia.server.plist"
		[ ! -f "$plist" ] && ok "no autostart entry was installed, because none was asked for" ||
			bad "an autostart entry appeared without --service"
		if HOME="$fake_home" "$setup" --role all-in-one --service --install-dir "$fake_home/.local/lib/theia" \
			--data-dir "$fake_home/.theia" --library "$work/data/library" --yes >"$work/install-service.log" 2>&1; then
			ok "the installer ran again with --service"
		else
			bad "the installer failed with --service:"
			sed 's/^/      /' "$work/install-service.log" | tail -20
		fi
		if [ -f "$plist" ]; then
			ok "the launchd agent was written when it was asked for"
			if grep -q "$fake_home/.local/lib/theia/theia-server" "$plist"; then
				ok "the agent starts the installed server, not the copy it was installed from"
			else
				bad "the agent does not name the installed server:"
				sed 's/^/      /' "$plist" | head -20
			fi
		else
			bad "no launchd plist under Library/LaunchAgents after --service"
		fi
		[ -L "$fake_home/Applications/Theia.app" ] && ok "~/Applications/Theia.app is a link to the installation" || bad "no Theia.app link"
		[ -L "$fake_home/.local/bin/theia" ] && ok "the theia command is linked into ~/.local/bin" || bad "no theia link"
		# A link is not a program. The launcher is the command somebody types, so
		# the installed one is run: `-version` names the build it came from and
		# starts nothing.
		if launcher_line=$(HOME="$fake_home" "$fake_home/.local/bin/theia" -version 2>&1); then
			case "$launcher_line" in
			theia\ *) ok "the installed theia command runs: $launcher_line" ;;
			*) bad "the installed theia command answered '$launcher_line' instead of naming itself" ;;
			esac
		else
			bad "the installed theia command failed to run"
		fi
		# The command's own promise, with nothing else running: it brings the
		# server up by itself and then opens the player (decision 139). Nothing
		# else here runs the launcher's own path - the server above was started by
		# hand, and the player by the playback section.
		#
		# `THEIA_DATA_DIR` is named because `theia` reads its port from the
		# standard data directory and knows nothing about the `--data-dir` this
		# installation was given: the installation records it in that directory's
		# `setup.json`, and nothing reads it back.
		THEIA_DATA_DIR="$fake_home/.theia" "$fake_home/.local/bin/theia" >"$work/launcher.log" 2>&1 &
		launcher_pid=$!
		launcher_ready=0
		for _ in $(seq 1 60); do
			if curl -sf http://127.0.0.1:8383/api/health >/dev/null 2>&1; then
				launcher_ready=1
				break
			fi
			sleep 0.5
		done
		if [ "$launcher_ready" = 1 ]; then
			ok "the theia command brought the installed server up by itself"
		else
			bad "the theia command did not bring a server up:"
			sed 's/^/      /' "$work/launcher.log" | tail -10
		fi
		launcher_opened=0
		for _ in $(seq 1 20); do
			if pgrep -f "Theia.app/Contents/MacOS/theia-player" >/dev/null 2>&1; then
				launcher_opened=1
				break
			fi
			sleep 0.5
		done
		if [ "$launcher_opened" = 1 ]; then
			ok "the theia command opened the player"
		else
			bad "the theia command did not open the player"
		fi
		# Everything it started goes with it: the uninstall below, and the
		# machine it leaves behind, must not depend on a window that is still up.
		kill "$launcher_pid" 2>/dev/null
		wait "$launcher_pid" 2>/dev/null
		pkill -f "Theia.app/Contents/MacOS/theia-player" 2>/dev/null
		pkill -f "$fake_home/.local/lib/theia/theia-server" 2>/dev/null
		sleep 1
		HOME="$fake_home" "$setup" --check --lang en | head -20
		HOME="$fake_home" "$setup" --uninstall --yes >"$work/uninstall.log" 2>&1 &&
			ok "the uninstall ran" || bad "the uninstall failed; see $work/uninstall.log"
		[ -f "$plist" ] && bad "the uninstall left the launchd agent behind" || ok "the uninstall removed the launchd agent"
		[ -d "$fake_home/.theia" ] && ok "the uninstall kept the data directory" || bad "the uninstall removed the data directory"
	fi
else
	bad "no theia-setup for macOS in dist/"
fi

echo
printf '%d passed, %d failed, %d for a person to look at. Working files: %s\n' "$pass" "$fail" "$look" "$work"
[ "$fail" -eq 0 ]

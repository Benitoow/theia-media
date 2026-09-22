#!/usr/bin/env bash
# Builds the native player on macOS and lays out the app bundle it ships in.
#
#   ./scripts/build-player-macos.sh                     -> debug build
#   ./scripts/build-player-macos.sh -Release            -> release build
#   ./scripts/build-player-macos.sh -Release -Bundle    -> and dist/theia-player-darwin-arm64
#   ./scripts/build-player-macos.sh -Release -Bundle -Version 3.3.4
#                                                       -> and a binary that names
#                                                          that build, which is
#                                                          what --diagnostics
#                                                          and the installer's
#                                                          smoke test read
#
# The sibling of build-player.ps1, and separate from it for one reason: the two
# platforms ship different *shapes*. Windows ships four files in a folder; macOS
# ships an app bundle - that is what gives a window a Dock icon, a menu bar and a
# name - and its engine is a set of dylibs that dyld finds through
# Contents/Frameworks, because that is one of the rpaths the engine carries.
#
# The order is not negotiable either: tauri-build embeds player/ui/dist into the
# Rust binary at compile time, so the OSD has to exist before cargo runs. Cargo
# will not say that in a useful way, which is why this script exists.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
release=0
bundle=0
version=""

while [ $# -gt 0 ]; do
	case "$1" in
		-Release) release=1 ;;
		-Bundle) bundle=1 ;;
		-Version)
			shift
			version="${1:-}"
			;;
		-h | --help)
			sed -n '2,20p' "$0"
			exit 0
			;;
		*)
			echo "unknown argument: $1" >&2
			exit 2
			;;
	esac
	shift
done

if [ "$(uname -s)" != "Darwin" ]; then
	echo "this builds the macOS player; on Windows use build-player.ps1" >&2
	exit 1
fi

profile=debug
cargo_flags=()
if [ "$release" = 1 ]; then
	profile=release
	cargo_flags+=(--release)
fi

echo "==> Building the OSD"
(
	cd "$root/player/ui"
	if [ -f package-lock.json ]; then npm ci; else npm install; fi
	npm run build
)
if [ ! -f "$root/player/ui/dist/index.html" ]; then
	echo "player/ui/dist/index.html is missing after the OSD build; cargo would fail with a confusing error" >&2
	exit 1
fi

if ! command -v cargo >/dev/null 2>&1; then
	echo "cargo was not found. Install Rust from https://rustup.rs, or put cargo on PATH." >&2
	exit 1
fi

# The workflow passes the tag this release is built from, so the binary can name
# it when somebody reports something. build.rs answers 'dev' when it is absent,
# which is what a local build is.
if [ -n "$version" ]; then export THEIA_VERSION="$version"; fi

# The executable has to be able to name the directory the engine lives in: the
# engine's own load commands ask for @rpath/lib<name>.dylib, and @rpath is
# resolved against the rpaths of the image that loads it - so the rpath goes on
# *this* binary, not on the engine.
echo "==> Building theia-player ($profile)"
(
	cd "$root"
	RUSTFLAGS="-C link-arg=-Wl,-rpath,@executable_path/../Frameworks" \
		cargo build --manifest-path player/Cargo.toml "${cargo_flags[@]}"
)

binary="$root/player/target/$profile/theia-player"
if [ ! -f "$binary" ]; then
	echo "cargo produced no $binary" >&2
	exit 1
fi

if [ "$bundle" != 1 ]; then
	echo "==> theia-player ready ($binary)"
	echo "    Without -Bundle: no app, no engine. The engine is a set of dylibs and"
	echo "    the player needs them beside it to play anything."
	exit 0
fi

stage="$root/dist/theia-player-darwin-arm64"
app="$stage/Theia.app"
rm -rf "$stage"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Frameworks" "$app/Contents/Resources"
cp "$binary" "$app/Contents/MacOS/theia-player"

# The bundle's own description of itself, with the version the release was
# tagged with - the same value build.rs compiled in, so Get Info and
# `theia-player -version` cannot disagree.
sed "s/@VERSION@/${version:-dev}/g" "$root/player/theia-player/Info.plist" >"$app/Contents/Info.plist"

# The engine, by digest. `fetch-libmpv` is the same tool Windows uses and it
# reads the same manifest; on macOS the pin is a set of libraries, so it verifies
# every one of them and brings the licence texts with it.
vendor="$root/player/vendor-darwin"
if command -v go >/dev/null 2>&1; then
	echo "==> Fetching the pinned engine"
	rm -rf "$vendor"
	mkdir -p "$vendor"
	(cd "$root" && go run ./scripts/fetch-libmpv -platform darwin/arm64 -out "$vendor")
elif [ ! -d "$vendor" ]; then
	echo "go was not found and $vendor holds nothing." >&2
	echo "Install Go, or pre-fetch the engine with:" >&2
	echo "  go run ./scripts/fetch-libmpv -platform darwin/arm64 -out $vendor" >&2
	exit 1
fi

if [ -z "$(ls -A "$vendor"/*.dylib 2>/dev/null)" ]; then
	echo "$vendor holds no dylibs; the engine was not fetched" >&2
	exit 1
fi
cp "$vendor"/*.dylib "$app/Contents/Frameworks/"
if [ -d "$vendor/licenses" ]; then
	# The engine's own licence texts, which are the LGPL obligation: the text
	# travels with the library. Theia's notice points here.
	cp -R "$vendor/licenses" "$app/Contents/Resources/licenses"
else
	echo "$vendor holds no licenses directory: the engine's licence texts would be missing" >&2
	exit 1
fi
cp "$root/player/LICENSE-libmpv.txt" "$root/player/NOTICE.md" "$app/Contents/Resources/"

# Apple Silicon refuses code without a signature, and the bundle changed after
# the linker signed the executable - libraries were added beside it and files
# were written inside it. An ad-hoc signature is what this project can do: there
# is no Apple Developer certificate behind it, so a downloaded copy is still
# refused by Gatekeeper once, which START-HERE.txt explains.
if command -v codesign >/dev/null 2>&1; then
	echo "==> Signing the bundle (ad-hoc)"
	codesign --force --deep --sign - "$app"
	codesign --verify --strict "$app" && echo "    signature verifies"
else
	echo "codesign was not found; the bundle is unsigned and macOS may refuse it" >&2
fi

# The OSD is embedded at compile time, and a build that used the previous dist
# looks perfect and carries the interface before the change. The Windows script
# greps the executable for the hashed asset name it just built, and that check
# does **not** transfer: measured on the proof runner, a macOS bundle whose OSD
# was the one just built - the engine verified by digest, the signature verified
# by codesign - does not carry the name in plain text, because Tauri's asset
# blob is compressed on this platform. So the guard is the one this project
# already uses for the same question in web/tests/serve-playback.mjs: the built
# interface must not be older than the sources it was built from.
sources="$root/player/ui/src"
built="$root/player/ui/dist/index.html"
if [ -d "$sources" ] && [ -f "$built" ]; then
	newest=$(find "$sources" -type f -newer "$built" | head -1)
	if [ -n "$newest" ]; then
		echo "player/ui/src is newer than player/ui/dist ($newest): this bundle embeds an OSD from before that edit" >&2
		exit 1
	fi
	echo "    the embedded OSD is not older than player/ui/src"
fi

echo "==> Packing the bundle"
archive="$root/dist/theia-player-darwin-arm64.zip"
rm -f "$archive"
# ditto, not zip: it keeps the bundle's structure and its executable bits, and
# the installer extracts this archive by member name.
ditto -c -k --sequesterRsrc --keepParent "$app" "$archive"

unpacked=$(du -sm "$app" | cut -f1)
packed=$(du -m "$archive" | cut -f1)
echo "==> theia-player-darwin-arm64 ready (${unpacked} MB, ${packed} MB zipped)"
echo "    Theia.app/Contents/MacOS/theia-player"
echo "    Theia.app/Contents/Frameworks/  ($(ls "$app/Contents/Frameworks"/*.dylib | wc -l | tr -d ' ') dylibs)"
echo "    Theia.app/Contents/Resources/licenses/  (the engine's licences)"
echo "    $archive"

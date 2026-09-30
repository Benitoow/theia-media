#!/usr/bin/env bash
# Assemble the Go programs and the already verified native Linux player.
set -euo pipefail
cd "$(dirname "$0")/.."
version="${1#v}"
arch="$2"
folder="dist/theia-$version-linux-$arch"
mkdir -p "$folder"
cp -R "dist/theia-player-linux-$arch/TheiaPlayer/." "$folder/"
cp "dist/theia-server-linux-$arch" "$folder/theia-server"
cp "dist/theia-setup-linux-$arch" "$folder/theia-setup"
cp "dist/theia-launcher-linux-$arch" "$folder/theia"
chmod +x "$folder/theia" "$folder/theia-server" "$folder/theia-setup" "$folder/theia-player"
cat > "$folder/START-HERE.txt" <<'TXT'
Theia for Ubuntu 24.04 (X11 or XWayland)

Desktop prerequisites: sudo apt install libmpv2 libwebkit2gtk-4.1-0 libayatana-appindicator3-1
Theia supplies its pinned media engine; Ubuntu supplies desktop/codec/driver libraries.
Install with ./theia-setup, then run the installed theia command.
The installer asks for no administrator rights and offers server-only, player-only
or all-in-one. Remove the programs with theia-setup --uninstall; library data stays.
Other Linux distributions have not been verified for this binary.
TXT
(
    cd "$folder"
    zip -q -r "../theia-$version-linux-$arch.zip" .
)
go run ./scripts/package-setup -setup "dist/theia-setup-linux-$arch" \
    -archive "dist/theia-$version-linux-$arch.zip" -out "dist/public/theia-setup-linux-$arch"
go run ./scripts/check-setup-payload -setup "dist/public/theia-setup-linux-$arch" \
    -server "dist/theia-server-linux-$arch" -target "linux-$arch"

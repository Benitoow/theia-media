#!/usr/bin/env bash
# Native Ubuntu 24.04 build; platform desktop/codec packages are prerequisites.
set -euo pipefail
cd "$(dirname "$0")/.."
version="${1:-${THEIA_VERSION:-dev}}"
case "$(uname -m)" in x86_64) arch=amd64 ;; aarch64) arch=arm64 ;; *) exit 2 ;; esac
npm ci --prefix player/ui
npm run build --prefix player/ui
THEIA_VERSION="$version" cargo build --manifest-path player/Cargo.toml --release --locked
bundle="dist/theia-player-linux-$arch/TheiaPlayer"
mkdir -p "$bundle"
cp player/target/release/theia-player "$bundle/"
go run ./scripts/fetch-libmpv -platform "linux/$arch" -out "$bundle"
cp LICENSE "$bundle/LICENSE-libmpv.txt"
cp /usr/share/common-licenses/GPL-2 "$bundle/LICENSE-GPL-2.txt"
cp /usr/share/common-licenses/LGPL-2.1 "$bundle/LICENSE-LGPL-2.1.txt"
cp player/NOTICE-linux.md "$bundle/NOTICE.md"
"$bundle/theia-player" -version
ldd "$bundle/theia-player"
ldd "$bundle/libmpv.so.2"

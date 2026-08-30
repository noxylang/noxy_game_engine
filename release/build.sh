#!/usr/bin/env sh
# Builds this platform's plugin binary into dist/ and its sha256 line.
# Ebiten needs cgo on Linux and macOS, so there is no cross-compile matrix
# here: the release workflow runs this on one runner per OS and merges the
# per-platform checksum files into the checksums.txt `noxy --get` expects.
set -eu
NAME="${1:?usage: build.sh <extension-name> [GOARCH]}"
ARCH="${2:-$(go env GOARCH)}"
OS="$(go env GOOS)"
ext=""; cgo=1
[ "$OS" = windows ] && { ext=".exe"; cgo=0; }   # Ebiten is pure Go on Windows; cgo elsewhere
mkdir -p dist
CGO_ENABLED="$cgo" GOARCH="$ARCH" go build -trimpath -ldflags=-s -o "dist/noxy-plugin-$NAME-$OS-$ARCH$ext" .
(cd dist && sha256sum -- "noxy-plugin-$NAME-$OS-$ARCH$ext" > "checksums-$OS-$ARCH.txt")
echo "dist/noxy-plugin-$NAME-$OS-$ARCH$ext"

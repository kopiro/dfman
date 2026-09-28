#!/bin/bash
set -euo pipefail
# Format/size conversion only; preserve the master artwork and its alpha.
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/dfman.iconset"
sips -z 256 256 assets/dfman.png --out assets/dfman-256.png >/dev/null
magick assets/dfman.png -define icon:auto-resize=256,128,64,48,32,16 assets/dfman.ico
for size in 16 32 128 256 512; do
 sips -z "$size" "$size" assets/dfman.png --out "$tmp/dfman.iconset/icon_${size}x${size}.png" >/dev/null
 double=$((size * 2))
 sips -z "$double" "$double" assets/dfman.png --out "$tmp/dfman.iconset/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$tmp/dfman.iconset" -o assets/dfman.icns

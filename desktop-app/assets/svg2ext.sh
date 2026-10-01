#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SVG="$SCRIPT_DIR/icon.svg"
OUT_DIR="$SCRIPT_DIR/../../extensions/clip/icons"

# icon.svg keeps the macOS app-icon margin; browser toolbar icons should fill the canvas, so crop to the plate.
TMP_SVG="$(mktemp -t hylo-ext-icon).svg"
trap 'rm -f "$TMP_SVG"' EXIT
sed 's/viewBox="0 0 1024 1024"/viewBox="102 102 820 820"/' "$SVG" > "$TMP_SVG"
grep -q 'viewBox="102 102 820 820"' "$TMP_SVG" || { echo "Error: expected viewBox=\"0 0 1024 1024\" in $SVG" >&2; exit 1; }

for SIZE in 16 48 128; do
  rsvg-convert -w $SIZE -h $SIZE -o "$OUT_DIR/icon${SIZE}.png" "$TMP_SVG"
  echo "Exported: $OUT_DIR/icon${SIZE}.png (${SIZE}x${SIZE})"
done

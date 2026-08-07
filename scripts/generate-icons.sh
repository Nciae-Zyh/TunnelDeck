#!/usr/bin/env bash
set -euo pipefail

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

node "$repo_root/extension/scripts/generate-icons.mjs"

if ! command -v magick >/dev/null 2>&1; then
  echo "PNG icons were generated. ImageMagick is required to regenerate build/windows/icon.ico." >&2
  exit 1
fi

magick -background none "$repo_root/build/appicon.png" \
  -define icon:auto-resize=256,128,64,48,32,16 \
  "$repo_root/build/windows/icon.ico"

echo "Regenerated TunnelDeck app, Windows, and Chrome extension icons."

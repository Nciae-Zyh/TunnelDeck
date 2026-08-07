#!/bin/sh

set -eu

for command_name in node npm zip unzip; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "Required command not found: $command_name" >&2
    exit 2
  fi
done

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
extension_root="$repo_root/extension"
manifest_path="$extension_root/public/manifest.json"
package_path="$extension_root/package.json"

manifest_version=$(node -p "require('$manifest_path').version")
package_version=$(node -p "require('$package_path').version")
if [ "$manifest_version" != "$package_version" ]; then
  echo "Extension version mismatch: manifest=$manifest_version package=$package_version" >&2
  exit 1
fi

echo "Building TunnelDeck Chrome extension $manifest_version..."
npm --prefix "$extension_root" ci
npm --prefix "$extension_root" run build

dist_dir="$extension_root/dist"
if [ ! -f "$dist_dir/manifest.json" ]; then
  echo "Built extension is missing manifest.json." >&2
  exit 1
fi
if find "$dist_dir" -type f \( -name '*.pem' -o -name '*.key' -o -name '.env*' -o -name '*.map' \) | grep -q .; then
  echo "Extension package contains a forbidden key, environment, or source-map file." >&2
  exit 1
fi

artifact_dir="$repo_root/artifacts/chrome-web-store"
artifact_path="$artifact_dir/TunnelDeck-chrome-web-store-v${manifest_version}.zip"
package_temp=$(mktemp -d)
trap 'rm -rf "$package_temp"' EXIT HUP INT TERM

(
  cd "$dist_dir"
  zip -q -r "$package_temp/extension.zip" .
)

if ! unzip -Z1 "$package_temp/extension.zip" | grep -qx 'manifest.json'; then
  echo "Chrome Web Store ZIP must contain manifest.json at its root." >&2
  exit 1
fi
archive_version=$(unzip -p "$package_temp/extension.zip" manifest.json | node -e '
  let input = "";
  process.stdin.on("data", chunk => { input += chunk });
  process.stdin.on("end", () => process.stdout.write(JSON.parse(input).version));
')
if [ "$archive_version" != "$manifest_version" ]; then
  echo "Packaged manifest version mismatch: $archive_version" >&2
  exit 1
fi

mkdir -p "$artifact_dir"
mv -f "$package_temp/extension.zip" "$artifact_path"
echo "Chrome Web Store package: $artifact_path"

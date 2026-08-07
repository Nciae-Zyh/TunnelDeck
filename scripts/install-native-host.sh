#!/bin/sh

set -eu

host_name="com.tunneldeck.native"
extension_id=""
binary_path=""

usage() {
  command_name=$(basename "$0")
  echo "Usage: $command_name --extension-id <32-character-id> [--binary <absolute-path>]" >&2
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --extension-id)
      [ "$#" -ge 2 ] || { usage; exit 2; }
      extension_id=$2
      shift 2
      ;;
    --binary)
      [ "$#" -ge 2 ] || { usage; exit 2; }
      binary_path=$2
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage
      exit 2
      ;;
  esac
done

if ! printf '%s' "$extension_id" | grep -Eq '^[a-p]{32}$'; then
  echo "Extension ID must contain exactly 32 letters from a to p." >&2
  exit 2
fi

system_name=$(uname -s)
case "$system_name" in
  Darwin)
    manifest_dir="$HOME/Library/Application Support/Google/Chrome/NativeMessagingHosts"
    default_binary="/Applications/TunnelDeck.app/Contents/MacOS/TunnelDeck"
    ;;
  Linux)
    config_root=${XDG_CONFIG_HOME:-"$HOME/.config"}
    manifest_dir="$config_root/google-chrome/NativeMessagingHosts"
    default_binary=""
    if command -v TunnelDeck >/dev/null 2>&1; then
      default_binary=$(command -v TunnelDeck)
    elif command -v tunneldeck >/dev/null 2>&1; then
      default_binary=$(command -v tunneldeck)
    fi
    ;;
  *)
    echo "This script supports macOS and Linux. Use install-native-host.ps1 on Windows." >&2
    exit 2
    ;;
esac

if [ -n "${TUNNELDECK_NATIVE_HOST_DIR:-}" ]; then
  manifest_dir=$TUNNELDECK_NATIVE_HOST_DIR
fi

if [ -z "$binary_path" ]; then
  binary_path=$default_binary
fi
if [ -z "$binary_path" ] || [ ! -f "$binary_path" ]; then
  echo "TunnelDeck binary not found. Pass its absolute path with --binary." >&2
  exit 2
fi
case "$binary_path" in
  /*) ;;
  *)
    binary_dir=$(dirname "$binary_path")
    binary_name=$(basename "$binary_path")
    binary_path=$(cd "$binary_dir" && pwd)/$binary_name
    ;;
esac
case "$binary_path" in
  *'"'*|*'
'*)
    echo "Binary path cannot contain quotes or newlines." >&2
    exit 2
    ;;
esac

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
template_path="$script_dir/../native-host/$host_name.json.template"
if [ ! -f "$template_path" ]; then
  echo "Native host template not found: $template_path" >&2
  exit 2
fi

umask 077
mkdir -p "$manifest_dir"
manifest_path="$manifest_dir/$host_name.json"
escaped_binary=$(printf '%s' "$binary_path" | sed 's/[&|\\]/\\&/g')
sed \
  -e "s|__TUNNELDECK_BINARY__|$escaped_binary|g" \
  -e "s|__EXTENSION_ID__|$extension_id|g" \
  "$template_path" > "$manifest_path"
chmod 600 "$manifest_path"

echo "Installed $host_name for Chrome extension $extension_id"
echo "Manifest: $manifest_path"
echo "Binary:   $binary_path"

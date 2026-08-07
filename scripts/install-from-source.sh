#!/bin/sh

set -eu

extension_id=""
install_dir=""

usage() {
  command_name=$(basename "$0")
  cat >&2 <<EOF
Usage: $command_name [--extension-id <32-character-id>] [--install-dir <path>]

Builds TunnelDeck and its Chrome extension from the checked-out source, then
installs the desktop application for the current user.
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --extension-id)
      [ "$#" -ge 2 ] || { usage; exit 2; }
      extension_id=$2
      shift 2
      ;;
    --install-dir)
      [ "$#" -ge 2 ] || { usage; exit 2; }
      install_dir=$2
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

if [ -n "$extension_id" ] && ! printf '%s' "$extension_id" | grep -Eq '^[a-p]{32}$'; then
  echo "Extension ID must contain exactly 32 letters from a to p." >&2
  exit 2
fi

for command_name in go npm; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "Required command not found: $command_name" >&2
    echo "Install the prerequisites described in docs/SOURCE_INSTALL.md, then retry." >&2
    exit 2
  fi
done

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"

echo "Building the desktop frontend..."
npm --prefix frontend ci

echo "Building the Chrome extension..."
npm --prefix extension ci
npm --prefix extension run build

system_name=$(uname -s)
case "$system_name" in
  Darwin)
    echo "Building TunnelDeck for this Mac..."
    go run github.com/wailsapp/wails/v2/cmd/wails@v2.13.0 build -clean
    [ -n "$install_dir" ] || install_dir="$HOME/Applications"
    mkdir -p "$install_dir"
    source_app="$repo_root/build/bin/TunnelDeck.app"
    target_app="$install_dir/TunnelDeck.app"
    if [ -e "$target_app" ]; then
      backup_app="$target_app.backup-$(date +%Y%m%d%H%M%S)"
      mv "$target_app" "$backup_app"
      echo "Previous application moved to: $backup_app"
    fi
    ditto "$source_app" "$target_app"
    binary_path="$target_app/Contents/MacOS/TunnelDeck"
    launch_hint="open \"$target_app\""
    ;;
  Linux)
    if ! command -v pkg-config >/dev/null 2>&1 || ! pkg-config --exists gtk+-3.0 webkit2gtk-4.1; then
      echo "GTK3 and WebKitGTK 4.1 development packages are required." >&2
      echo "See docs/SOURCE_INSTALL.md for distribution-specific commands." >&2
      exit 2
    fi
    echo "Building TunnelDeck for this Linux system..."
    go run github.com/wailsapp/wails/v2/cmd/wails@v2.13.0 build -clean -tags webkit2_41
    [ -n "$install_dir" ] || install_dir="$HOME/.local/bin"
    mkdir -p "$install_dir"
    binary_path="$install_dir/TunnelDeck"
    install -m 0755 "$repo_root/build/bin/TunnelDeck" "$binary_path"
    launch_hint="\"$binary_path\""
    ;;
  *)
    echo "This script supports macOS and Linux. Use install-from-source.ps1 on Windows." >&2
    exit 2
    ;;
esac

if [ -n "$extension_id" ]; then
  "$repo_root/scripts/install-native-host.sh" \
    --extension-id "$extension_id" \
    --binary "$binary_path"
fi

echo
echo "TunnelDeck installed from locally built source."
echo "Desktop application: $binary_path"
echo "Chrome extension directory: $repo_root/extension/dist"
echo "Launch command: $launch_hint"
if [ -z "$extension_id" ]; then
  echo "After loading extension/dist in chrome://extensions, enter its ID in TunnelDeck to register Chrome integration."
else
  echo "Chrome integration registered for: $extension_id"
fi

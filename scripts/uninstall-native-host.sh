#!/bin/sh

set -eu

host_name="com.tunneldeck.native"
system_name=$(uname -s)
case "$system_name" in
  Darwin)
    manifest_path="$HOME/Library/Application Support/Google/Chrome/NativeMessagingHosts/$host_name.json"
    ;;
  Linux)
    config_root=${XDG_CONFIG_HOME:-"$HOME/.config"}
    manifest_path="$config_root/google-chrome/NativeMessagingHosts/$host_name.json"
    ;;
  *)
    echo "This script supports macOS and Linux. Use uninstall-native-host.ps1 on Windows." >&2
    exit 2
    ;;
esac

if [ -n "${TUNNELDECK_NATIVE_HOST_DIR:-}" ]; then
  manifest_path="$TUNNELDECK_NATIVE_HOST_DIR/$host_name.json"
fi

if [ -f "$manifest_path" ]; then
  rm "$manifest_path"
  echo "Removed $manifest_path"
else
  echo "Native host is not installed for Google Chrome."
fi

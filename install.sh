#!/bin/sh

set -eu

PROJECT_VERSION="${TUNNELDECK_VERSION:-v0.3.3}"
GO_VERSION="1.26.5"
NODE_VERSION="22.23.2"
REPOSITORY="Nciae-Zyh/TunnelDeck"
OFFICIAL_CHROME_EXTENSION_ID="jnfkjehpbkmfnidfcilehhkpbjjinmod"
OFFICIAL_CHROME_STORE_URL="https://chromewebstore.google.com/detail/jnfkjehpbkmfnidfcilehhkpbjjinmod"

assume_yes=0
refresh_source=0
check_only=0
development_extension_id=""
store_extension_id=""
install_dir=""

usage() {
  command_name=$(basename "$0")
  cat >&2 <<EOF
TunnelDeck source installer

Usage: $command_name [options]

  --version <vX.Y.Z>          Install a specific version tag
  --extension-id <id>         Build the development extension and register its ID
  --chrome-store-id <id>      Use the Chrome Web Store extension and skip local extension build
  --install-dir <path>        Override the desktop installation directory
  --refresh-source            Re-download an already cached version
  --check                     Check prerequisites without installing anything
  --yes                       Confirm supported system package installation
  -h, --help                  Show this help
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version)
      [ "$#" -ge 2 ] || { usage; exit 2; }
      PROJECT_VERSION=$2
      shift 2
      ;;
    --extension-id)
      [ "$#" -ge 2 ] || { usage; exit 2; }
      development_extension_id=$2
      shift 2
      ;;
    --chrome-store-id)
      [ "$#" -ge 2 ] || { usage; exit 2; }
      store_extension_id=$2
      shift 2
      ;;
    --install-dir)
      [ "$#" -ge 2 ] || { usage; exit 2; }
      install_dir=$2
      shift 2
      ;;
    --refresh-source)
      refresh_source=1
      shift
      ;;
    --check)
      check_only=1
      shift
      ;;
    --yes)
      assume_yes=1
      shift
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

if [ -z "$development_extension_id" ] && [ -z "$store_extension_id" ]; then
  store_extension_id="$OFFICIAL_CHROME_EXTENSION_ID"
fi

if ! printf '%s' "$PROJECT_VERSION" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
  echo "Version must use the vX.Y.Z format." >&2
  exit 2
fi

if [ -n "$development_extension_id" ] && [ -n "$store_extension_id" ]; then
  echo "Use either --extension-id or --chrome-store-id, not both." >&2
  exit 2
fi
for extension_id in "$development_extension_id" "$store_extension_id"; do
  if [ -n "$extension_id" ] && ! printf '%s' "$extension_id" | grep -Eq '^[a-p]{32}$'; then
    echo "Chrome extension ID must contain exactly 32 letters from a to p." >&2
    exit 2
  fi
done

if command -v curl >/dev/null 2>&1; then
  downloader="curl"
elif command -v wget >/dev/null 2>&1; then
  downloader="wget"
else
  echo "TunnelDeck needs curl or wget to download verified toolchains and source." >&2
  exit 2
fi

if command -v sha256sum >/dev/null 2>&1; then
  hash_command="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
  hash_command="shasum -a 256"
else
  echo "TunnelDeck needs sha256sum or shasum to verify downloaded toolchains." >&2
  exit 2
fi

download() {
  source_url=$1
  target_path=$2
  if [ "$downloader" = "curl" ]; then
    curl --fail --location --show-error --silent --retry 3 --output "$target_path" "$source_url"
  else
    wget --quiet --tries=3 --output-document="$target_path" "$source_url"
  fi
}

hash_file() {
  target_path=$1
  if [ "$hash_command" = "sha256sum" ]; then
    sha256sum "$target_path" | awk '{print $1}'
  else
    shasum -a 256 "$target_path" | awk '{print $1}'
  fi
}

verify_hash() {
  target_path=$1
  expected_hash=$2
  actual_hash=$(hash_file "$target_path")
  if [ "$actual_hash" != "$expected_hash" ]; then
    echo "SHA-256 verification failed for $target_path" >&2
    echo "Expected: $expected_hash" >&2
    echo "Actual:   $actual_hash" >&2
    exit 1
  fi
}

version_at_least() {
  current_version=$1
  required_major=$2
  required_minor=$3
  current_major=$(printf '%s' "$current_version" | awk -F. '{print $1}')
  current_minor=$(printf '%s' "$current_version" | awk -F. '{print $2}')
  [ "$current_major" -gt "$required_major" ] || {
    [ "$current_major" -eq "$required_major" ] && [ "$current_minor" -ge "$required_minor" ]
  }
}

run_as_root() {
  if [ "$(id -u)" -eq 0 ]; then
    "$@"
  elif command -v sudo >/dev/null 2>&1; then
    sudo "$@"
  else
    echo "Installing Linux build libraries requires root or sudo." >&2
    return 1
  fi
}

confirm() {
  prompt_text=$1
  if [ "$assume_yes" -eq 1 ]; then
    return 0
  fi
  if [ ! -r /dev/tty ]; then
    echo "$prompt_text Re-run with --yes to approve." >&2
    return 1
  fi
  printf '%s [y/N] ' "$prompt_text" >/dev/tty
  read -r answer </dev/tty || return 1
  case "$answer" in
    y|Y|yes|YES) return 0 ;;
    *) return 1 ;;
  esac
}

system_name=$(uname -s)
machine_arch=$(uname -m)
case "$system_name/$machine_arch" in
  Darwin/arm64) platform="darwin-arm64"; go_archive="go${GO_VERSION}.darwin-arm64.tar.gz"; go_hash="efb87ff28af9a188d0536ef5d42e63dd52ba8263cd7344a993cc48dd11dedb6a"; node_platform="darwin-arm64" ;;
  Darwin/x86_64) platform="darwin-amd64"; go_archive="go${GO_VERSION}.darwin-amd64.tar.gz"; go_hash="6231d8d3b8f5552ec6cbf6d685bdd5482e1e703214b120e89b3bf0d7bf1ef725"; node_platform="darwin-x64" ;;
  Linux/x86_64) platform="linux-amd64"; go_archive="go${GO_VERSION}.linux-amd64.tar.gz"; go_hash="5c2c3b16caefa1d968a94c1daca04a7ca301a496d9b086e17ad77bb81393f053"; node_platform="linux-x64" ;;
  Linux/aarch64|Linux/arm64) platform="linux-arm64"; go_archive="go${GO_VERSION}.linux-arm64.tar.gz"; go_hash="fe4789e92b1f33358680864bbe8704289e7bb5fc207d80623c308935bd696d49"; node_platform="linux-arm64" ;;
  *) echo "Unsupported platform: $system_name/$machine_arch" >&2; exit 2 ;;
esac

case "$node_platform" in
  darwin-arm64) node_hash="61130f394c1630d211dd50aecc4353d379480f36d3ac913cd85dbba1aed585c6" ;;
  darwin-x64) node_hash="58e99022c2ff89395576cc7fd4d98cea24bb68081475d5f88b801ee8729fb026" ;;
  linux-arm64) node_hash="013b59cfd2819703a6f4a14ab891fc46fc2a4e3f5bcd92de3fb4929b43e35b30" ;;
  linux-x64) node_hash="b294a556e639d64338823920e5866c21c02741742d2e1529ee1a225c1ec9252a" ;;
esac
node_archive="node-v${NODE_VERSION}-${node_platform}.tar.gz"

installed_go_version=""
if command -v go >/dev/null 2>&1; then
  installed_go_version=$(go env GOVERSION 2>/dev/null | sed 's/^go//' || true)
fi
installed_node_version=""
if command -v node >/dev/null 2>&1; then
  installed_node_version=$(node -p 'process.versions.node' 2>/dev/null || true)
fi

echo "TunnelDeck prerequisite check"
echo "  Platform:       $system_name/$machine_arch"
echo "  Downloader:     $downloader"
echo "  SHA-256:        $hash_command"
if [ -n "$installed_go_version" ] && version_at_least "$installed_go_version" 1 25; then
  echo "  Go:             $installed_go_version (ready)"
else
  echo "  Go:             missing or older than 1.25; isolated $GO_VERSION will be installed"
fi
if [ -n "$installed_node_version" ] && version_at_least "$installed_node_version" 20 0 && command -v npm >/dev/null 2>&1; then
  echo "  Node.js/npm:    $installed_node_version / $(npm --version) (ready)"
else
  echo "  Node.js/npm:    missing or older than 20; isolated $NODE_VERSION will be installed"
fi

system_dependencies_ready=1
if [ "$system_name" = "Darwin" ]; then
  if ! xcode-select -p >/dev/null 2>&1; then
    system_dependencies_ready=0
    if [ "$check_only" -eq 1 ]; then
      echo "  Build libraries: Xcode Command Line Tools missing"
    else
      echo "Xcode Command Line Tools are required. macOS will open its installer." >&2
      xcode-select --install || true
      echo "Finish that installation, then run the TunnelDeck command again." >&2
      exit 2
    fi
  else
    echo "  Build libraries: Xcode Command Line Tools ready"
  fi
else
  if ! command -v pkg-config >/dev/null 2>&1 || ! pkg-config --exists gtk+-3.0 webkit2gtk-4.1; then
    system_dependencies_ready=0
    if [ "$check_only" -eq 1 ]; then
      echo "  Build libraries: GTK3/WebKitGTK 4.1 missing"
    elif ! confirm "TunnelDeck needs GTK3 and WebKitGTK 4.1 development packages. Install them now?"; then
      echo "Install the packages listed in docs/SOURCE_INSTALL.md, then retry." >&2
      exit 2
    elif command -v apt-get >/dev/null 2>&1; then
      run_as_root apt-get update
      run_as_root apt-get install -y build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev
    elif command -v dnf >/dev/null 2>&1; then
      run_as_root dnf install -y gcc gcc-c++ pkgconf-pkg-config gtk3-devel webkit2gtk4.1-devel
    elif command -v pacman >/dev/null 2>&1; then
      run_as_root pacman -S --needed base-devel pkgconf gtk3 webkit2gtk-4.1
    elif command -v zypper >/dev/null 2>&1; then
      run_as_root zypper install -y gcc-c++ pkg-config gtk3-devel webkit2gtk-4_1-devel
    else
      echo "No supported system package manager was found." >&2
      exit 2
    fi
  else
    echo "  Build libraries: GTK3/WebKitGTK 4.1 ready"
  fi
fi

if [ "$check_only" -eq 1 ]; then
  if [ "$system_dependencies_ready" -eq 1 ]; then
    echo "Prerequisite check passed. Missing Go/Node toolchains can be installed in user space."
    exit 0
  fi
  echo "Prerequisite check found missing system build libraries." >&2
  exit 1
fi

data_home="${XDG_DATA_HOME:-$HOME/.local/share}/tunneldeck"
cache_home="${XDG_CACHE_HOME:-$HOME/.cache}/tunneldeck"
tools_home="$data_home/toolchains"
mkdir -p "$data_home/src" "$cache_home" "$tools_home"

go_home="$tools_home/go/$GO_VERSION"
if [ -z "$installed_go_version" ] || ! version_at_least "$installed_go_version" 1 25; then
  if [ ! -x "$go_home/bin/go" ]; then
    go_archive_path="$cache_home/$go_archive"
    echo "Downloading Go $GO_VERSION for $platform..."
    download "https://go.dev/dl/$go_archive" "$go_archive_path"
    verify_hash "$go_archive_path" "$go_hash"
    go_temp=$(mktemp -d)
    tar -xzf "$go_archive_path" -C "$go_temp"
    mkdir -p "$(dirname "$go_home")"
    if [ -e "$go_home" ]; then
      mv "$go_home" "$go_home.incomplete-$(date +%Y%m%d%H%M%S)"
    fi
    mv "$go_temp/go" "$go_home"
    rmdir "$go_temp"
  fi
  PATH="$go_home/bin:$PATH"
  export PATH
fi

node_home="$tools_home/node/$NODE_VERSION"
if [ -z "$installed_node_version" ] || ! version_at_least "$installed_node_version" 20 0 || ! command -v npm >/dev/null 2>&1; then
  if [ ! -x "$node_home/bin/node" ]; then
    node_archive_path="$cache_home/$node_archive"
    echo "Downloading Node.js $NODE_VERSION for $platform..."
    download "https://nodejs.org/dist/v${NODE_VERSION}/$node_archive" "$node_archive_path"
    verify_hash "$node_archive_path" "$node_hash"
    node_temp=$(mktemp -d)
    tar -xzf "$node_archive_path" -C "$node_temp"
    mkdir -p "$(dirname "$node_home")"
    if [ -e "$node_home" ]; then
      mv "$node_home" "$node_home.incomplete-$(date +%Y%m%d%H%M%S)"
    fi
    mv "$node_temp/node-v${NODE_VERSION}-${node_platform}" "$node_home"
    rmdir "$node_temp"
  fi
  PATH="$node_home/bin:$PATH"
  export PATH
fi

echo "Environment ready: $(go version); node $(node --version); npm $(npm --version)"

source_dir="${TUNNELDECK_SOURCE_DIR:-$data_home/src/$PROJECT_VERSION}"
if [ -z "${TUNNELDECK_SOURCE_DIR:-}" ] && [ "$refresh_source" -eq 1 ] && [ -e "$source_dir" ]; then
  backup_dir="$source_dir.backup-$(date +%Y%m%d%H%M%S)"
  mv "$source_dir" "$backup_dir"
  echo "Previous source moved to: $backup_dir"
fi
if [ -n "${TUNNELDECK_SOURCE_DIR:-}" ] && [ ! -f "$source_dir/scripts/install-from-source.sh" ]; then
  echo "TUNNELDECK_SOURCE_DIR does not point to a TunnelDeck source checkout: $source_dir" >&2
  exit 2
fi
if [ ! -f "$source_dir/scripts/install-from-source.sh" ]; then
  source_archive="$cache_home/TunnelDeck-${PROJECT_VERSION}.tar.gz"
  echo "Downloading TunnelDeck $PROJECT_VERSION source..."
  download "https://github.com/$REPOSITORY/archive/refs/tags/$PROJECT_VERSION.tar.gz" "$source_archive"
  source_temp=$(mktemp -d)
  tar -xzf "$source_archive" -C "$source_temp"
  extracted_dir="$source_temp/TunnelDeck-${PROJECT_VERSION#v}"
  if [ ! -d "$extracted_dir" ]; then
    echo "Downloaded source archive has an unexpected layout." >&2
    exit 1
  fi
  mkdir -p "$(dirname "$source_dir")"
  if [ -e "$source_dir" ]; then
    incomplete_source="$source_dir.incomplete-$(date +%Y%m%d%H%M%S)"
    mv "$source_dir" "$incomplete_source"
    echo "Incomplete source moved to: $incomplete_source"
  fi
  mv "$extracted_dir" "$source_dir"
  rmdir "$source_temp"
fi

set --
if [ -n "$install_dir" ]; then
  set -- "$@" --install-dir "$install_dir"
fi
if [ -n "$store_extension_id" ]; then
  set -- "$@" --skip-extension-build --extension-id "$store_extension_id"
elif [ -n "$development_extension_id" ]; then
  set -- "$@" --extension-id "$development_extension_id"
fi

"$source_dir/scripts/install-from-source.sh" "$@"

if [ -n "$store_extension_id" ]; then
  store_url="$OFFICIAL_CHROME_STORE_URL"
  [ -n "$store_url" ] || store_url="https://chromewebstore.google.com/detail/$store_extension_id"
  echo "Chrome Web Store: $store_url"
fi

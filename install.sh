#!/usr/bin/env sh
set -eu
if (set -o pipefail) 2>/dev/null; then
  set -o pipefail
fi

OWNER="ajxv"
REPO="redis-tui"
BINARY="redis-tui"
VERSION="latest"
INSTALL_DIR=""

usage() {
  cat <<USAGE
Install redis-tui from GitHub Releases.

Usage:
  install.sh [--version <tag>] [--to <dir>]

Options:
  --version <tag>  Install a specific release tag (e.g. v1.0.0-beta).
                   Defaults to latest release.
  --to <dir>       Install directory. Defaults to ~/.local/bin when writable,
                   otherwise /usr/local/bin.
  -h, --help       Show this help message.
USAGE
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Error: required command '$1' is not installed." >&2
    exit 1
  fi
}

validate_value() {
  case "$1" in
    ""|*' '*|*'..'*|*'~'*|*'\\'*)
      return 1
      ;;
  esac
  return 0
}

http_get() {
  url="$1"
  out="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$out"
  else
    wget -qO "$out" "$url"
  fi
}

http_get_stdout() {
  url="$1"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url"
  else
    wget -qO- "$url"
  fi
}

default_install_dir() {
  user_bin="$HOME/.local/bin"
  if mkdir -p "$user_bin" 2>/dev/null && [ -w "$user_bin" ]; then
    printf '%s\n' "$user_bin"
    return
  fi 

  printf '%s\n' "/usr/local/bin"
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version)
      [ "$#" -ge 2 ] || { echo "Error: --version requires a value" >&2; exit 1; }
      VERSION="$2"
      shift 2
      ;;
    --to)
      [ "$#" -ge 2 ] || { echo "Error: --to requires a value" >&2; exit 1; }
      INSTALL_DIR="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Error: unknown argument '$1'" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if ! validate_value "$VERSION"; then
  echo "Error: invalid version '$VERSION'." >&2
  exit 1
fi

if [ -n "$INSTALL_DIR" ] && ! validate_value "$INSTALL_DIR"; then
  echo "Error: invalid install directory '$INSTALL_DIR'." >&2
  exit 1
fi

require_cmd tar
if ! command -v curl >/dev/null 2>&1 && ! command -v wget >/dev/null 2>&1; then
  echo "Error: either 'curl' or 'wget' is required." >&2
  exit 1
fi

os_raw="$(uname -s)"
arch_raw="$(uname -m)"

case "$os_raw" in
  Linux) os="Linux" ;;
  Darwin) os="Darwin" ;;
  *)
    echo "Error: unsupported OS '$os_raw'. This installer supports Linux and macOS." >&2
    exit 1
    ;;
esac

case "$arch_raw" in
  x86_64|amd64) arch="x86_64" ;;
  arm64|aarch64) arch="arm64" ;;
  *)
    echo "Error: unsupported architecture '$arch_raw'." >&2
    exit 1
    ;;
esac

if [ -z "$INSTALL_DIR" ]; then
  INSTALL_DIR="$(default_install_dir)"
fi

if [ ! -d "$INSTALL_DIR" ]; then
  mkdir -p "$INSTALL_DIR" 2>/dev/null || true
fi

if [ ! -d "$INSTALL_DIR" ] || [ ! -w "$INSTALL_DIR" ]; then
  echo "Error: install directory '$INSTALL_DIR' is not writable." >&2
  echo "Try one of:" >&2
  echo "  1) install to a user-writable directory: --to \"$HOME/.local/bin\"" >&2
  echo "  2) run with sudo for system-wide install:" >&2
  echo "     sudo sh -c \"curl -fsSL https://raw.githubusercontent.com/$OWNER/$REPO/main/install.sh | sh -s -- --to /usr/local/bin\"" >&2
  exit 1
fi

if [ "$VERSION" = "latest" ]; then
  release_api="https://api.github.com/repos/$OWNER/$REPO/releases/latest"
  release_json="$(http_get_stdout "$release_api")"
  VERSION="$(printf '%s' "$release_json" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
  if [ -z "$VERSION" ]; then
    echo "Error: unable to determine latest release version from GitHub." >&2
    exit 1
  fi
fi

asset="${BINARY}_${os}_${arch}.tar.gz"
base_url="https://github.com/$OWNER/$REPO/releases/download/$VERSION"
archive_url="$base_url/$asset"
checksums_url="$base_url/checksums.txt"

tmpdir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

archive_path="$tmpdir/$asset"
checksums_path="$tmpdir/checksums.txt"

echo "Installing $BINARY $VERSION for $os/$arch into $INSTALL_DIR"
http_get "$archive_url" "$archive_path"

if http_get "$checksums_url" "$checksums_path"; then
  expected_sha="$(awk -v file="$asset" '$2 == file { print $1 }' "$checksums_path" | head -n1)"
  if [ -n "$expected_sha" ]; then
    if command -v sha256sum >/dev/null 2>&1; then
      actual_sha="$(sha256sum "$archive_path" | awk '{print $1}')"
    elif command -v shasum >/dev/null 2>&1; then
      actual_sha="$(shasum -a 256 "$archive_path" | awk '{print $1}')"
    else
      actual_sha=""
      echo "Warning: sha256sum/shasum not found; skipping checksum verification." >&2
    fi

    if [ -n "$actual_sha" ] && [ "$actual_sha" != "$expected_sha" ]; then
      echo "Error: checksum verification failed for $asset." >&2
      exit 1
    fi
  else
    echo "Warning: could not find checksum entry for $asset; skipping verification." >&2
  fi
else
  echo "Warning: could not download checksums.txt; skipping checksum verification." >&2
fi

tar -xzf "$archive_path" -C "$tmpdir"

binary_path="$(find "$tmpdir" -type f -name "$BINARY" | head -n1)"
if [ -z "$binary_path" ]; then
  echo "Error: could not find '$BINARY' in extracted archive." >&2
  exit 1
fi

if command -v install >/dev/null 2>&1; then
  install -m 0755 "$binary_path" "$INSTALL_DIR/$BINARY"
else
  cp "$binary_path" "$INSTALL_DIR/$BINARY"
  chmod 0755 "$INSTALL_DIR/$BINARY"
fi

echo

echo "✅ Installed: $INSTALL_DIR/$BINARY"
echo "Run it with:"
echo "  $BINARY"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    echo
    echo "'$INSTALL_DIR' is not currently in your PATH."
    echo "Add it with:"
    echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
    ;;
esac

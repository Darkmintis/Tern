#!/usr/bin/env sh
# Tern install script — downloads the latest GitHub Release binary.
# Usage: curl -sSL https://raw.githubusercontent.com/darkmintis/Tern/main/install/install.sh | sh
set -eu

REPO="${TERN_REPO:-darkmintis/Tern}"
BIN_DIR="${TERN_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${TERN_VERSION:-latest}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo "unsupported arch: $arch" >&2; exit 1 ;;
esac
case "$os" in
  linux|darwin) ;;
  msys*|mingw*|cygwin*) os=windows ;;
  *) echo "unsupported os: $os" >&2; exit 1 ;;
esac

ext=""
if [ "$os" = "windows" ]; then ext=".exe"; fi
asset="tern-${os}-${arch}${ext}"

if [ "$VERSION" = "latest" ]; then
  url="https://github.com/${REPO}/releases/latest/download/${asset}"
else
  url="https://github.com/${REPO}/releases/download/${VERSION}/${asset}"
fi

mkdir -p "$BIN_DIR"
tmp=$(mktemp)
echo "Downloading $url"
if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$url" -o "$tmp"
else
  wget -qO "$tmp" "$url"
fi
chmod +x "$tmp"
mv "$tmp" "${BIN_DIR}/tern${ext}"
echo "Installed tern to ${BIN_DIR}/tern${ext}"
echo "Ensure ${BIN_DIR} is on your PATH"

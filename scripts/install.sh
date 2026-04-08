#!/usr/bin/env bash
set -euo pipefail

REPO="solapi/solactl"
INSTALL_DIR="${HOME}/.local/bin"

die() { echo "ERROR: $*" >&2; exit 1; }

command -v curl &>/dev/null || die "curl is required."
command -v tar &>/dev/null || die "tar is required."

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) die "Unsupported architecture: $ARCH" ;;
esac

echo "Checking latest version..."
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' \
  | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/') || die "Failed to fetch release info."
[ -n "$TAG" ] || die "Failed to parse release tag."
echo "Latest version: ${TAG}"

VERSION="${TAG#v}"
ARCHIVE_NAME="solactl_${VERSION}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE_NAME}"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading ${ARCHIVE_NAME}..."
curl -fsSL -o "${TMPDIR}/${ARCHIVE_NAME}" "$DOWNLOAD_URL" || die "Download failed: ${DOWNLOAD_URL}"

echo "Extracting..."
tar -xzf "${TMPDIR}/${ARCHIVE_NAME}" -C "$TMPDIR"

BINARY="$TMPDIR/solactl"
[ -f "$BINARY" ] || die "Binary not found in archive."
chmod +x "$BINARY"

mkdir -p "$INSTALL_DIR"
mv "$BINARY" "$INSTALL_DIR/solactl"

echo ""
echo "Installed solactl ${TAG}"
echo "Location: ${INSTALL_DIR}/solactl"
echo ""

if ! echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
  SHELL_NAME=$(basename "$SHELL")
  case "$SHELL_NAME" in
    zsh)  RC="~/.zshrc" ;;
    bash) RC="~/.bashrc" ;;
    *)    RC="your shell config" ;;
  esac
  echo "${INSTALL_DIR} is not in PATH. Add this to ${RC}:"
  echo ""
  echo "  export PATH=\"\${HOME}/.local/bin:\${PATH}\""
  echo ""
fi

echo "To upgrade later: solactl upgrade"

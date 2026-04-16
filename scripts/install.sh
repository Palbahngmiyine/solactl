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
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${TAG}/checksums.txt"

# Detect SHA256 tool
if command -v sha256sum &>/dev/null; then
  SHA256CMD="sha256sum"
elif command -v shasum &>/dev/null; then
  SHA256CMD="shasum -a 256"
else
  die "sha256sum or shasum is required for checksum verification."
fi

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading ${ARCHIVE_NAME}..."
curl -fsSL -o "${TMPDIR}/${ARCHIVE_NAME}" "$DOWNLOAD_URL" || die "Download failed: ${DOWNLOAD_URL}"

echo "Downloading checksums..."
curl -fsSL -o "${TMPDIR}/checksums.txt" "$CHECKSUMS_URL" || die "Checksum download failed: ${CHECKSUMS_URL}"

echo "Verifying checksum..."
# checksums.txt format: "<hash>  <filename>" — use exact field match to avoid regex issues with dots
EXPECTED_HASH=$(awk -v name="$ARCHIVE_NAME" '$2==name {print $1; exit}' "${TMPDIR}/checksums.txt" || true)
[ -n "$EXPECTED_HASH" ] || die "Checksum not found for ${ARCHIVE_NAME} in checksums.txt"
ACTUAL_HASH=$(cd "$TMPDIR" && $SHA256CMD "$ARCHIVE_NAME" | awk '{print $1}') || die "SHA256 computation failed for ${ARCHIVE_NAME}"
[ -n "$ACTUAL_HASH" ] || die "SHA256 computation produced empty result for ${ARCHIVE_NAME}"
if [ "$EXPECTED_HASH" != "$ACTUAL_HASH" ]; then
  die "Checksum mismatch: expected ${EXPECTED_HASH}, got ${ACTUAL_HASH}. File may be tampered."
fi
echo "Checksum verified."

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

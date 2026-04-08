#!/usr/bin/env bash
set -euo pipefail

REPO="solapi/solactl"
INSTALL_DIR="${HOME}/.local/bin"

die() { echo "ERROR: $*" >&2; exit 1; }

command -v gh &>/dev/null || die "gh CLI가 필요합니다. https://cli.github.com 에서 설치해주세요."
gh auth status &>/dev/null || die "gh 인증이 필요합니다. 'gh auth login'을 실행해주세요."

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) die "지원하지 않는 아키텍처: $ARCH" ;;
esac

EXT="tar.gz"
[ "$OS" = "windows" ] && EXT="zip"

PATTERN="solactl_*_${OS}_${ARCH}.${EXT}"

echo "최신 버전을 확인하는 중..."
TAG=$(gh api "repos/${REPO}/releases/latest" --jq '.tag_name') || die "릴리스를 찾을 수 없습니다."
echo "최신 버전: ${TAG}"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo "다운로드 중: ${PATTERN}"
gh release download "$TAG" -R "$REPO" -p "$PATTERN" -D "$TMPDIR"

ARCHIVE=$(find "$TMPDIR" -maxdepth 1 -type f | head -1)
[ -f "$ARCHIVE" ] || die "다운로드된 파일을 찾을 수 없습니다."

echo "압축 해제 중..."
if [ "$EXT" = "tar.gz" ]; then
  tar -xzf "$ARCHIVE" -C "$TMPDIR"
else
  unzip -o "$ARCHIVE" -d "$TMPDIR"
fi

BINARY="$TMPDIR/solactl"
[ -f "$BINARY" ] || die "아카이브에서 solactl 바이너리를 찾을 수 없습니다."
chmod +x "$BINARY"

mkdir -p "$INSTALL_DIR"
mv "$BINARY" "$INSTALL_DIR/solactl"

echo ""
echo "설치 완료! solactl ${TAG}"
echo "설치 경로: ${INSTALL_DIR}/solactl"
echo ""

if ! echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
  SHELL_NAME=$(basename "$SHELL")
  case "$SHELL_NAME" in
    zsh)  RC="~/.zshrc" ;;
    bash) RC="~/.bashrc" ;;
    *)    RC="셸 설정 파일" ;;
  esac
  echo "PATH에 ${INSTALL_DIR}이 없습니다. 다음을 ${RC}에 추가해주세요:"
  echo ""
  echo "  export PATH=\"\${HOME}/.local/bin:\${PATH}\""
  echo ""
fi

echo "이후 업그레이드: solactl upgrade"

#!/bin/bash
# Apple Dev Docs - Auto-install App Store Connect CLI
set -e

SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BIN_DIR="$SKILL_DIR/bin"
ASC_BIN="$BIN_DIR/asc"
VERSION="0.31.3"

# Check if already installed
if [ -f "$ASC_BIN" ]; then
  echo "asc CLI already installed at $ASC_BIN"
  exit 0
fi

# Also check system PATH
if command -v asc &>/dev/null; then
  echo "asc CLI found in PATH: $(which asc)"
  exit 0
fi

echo "Installing App Store Connect CLI v${VERSION}..."

# Detect platform
OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
  Darwin) PLATFORM="macOS" ;;
  Linux)  PLATFORM="linux" ;;
  *)      echo "Unsupported OS: $OS"; exit 1 ;;
esac

case "$ARCH" in
  arm64|aarch64) ARCH="arm64" ;;
  x86_64)        ARCH="amd64" ;;
  *)             echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

FILENAME="asc_${VERSION}_${PLATFORM}_${ARCH}"
URL="https://github.com/rudrankriyam/App-Store-Connect-CLI/releases/download/${VERSION}/${FILENAME}"

mkdir -p "$BIN_DIR"

echo "Downloading ${FILENAME}..."
curl -fsSL "$URL" -o "$ASC_BIN"
chmod +x "$ASC_BIN"

echo "Installed asc v${VERSION} to $ASC_BIN"
echo "Add to PATH: export PATH=\"$BIN_DIR:\$PATH\""

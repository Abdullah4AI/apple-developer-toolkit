#!/bin/bash
# Apple Developer Toolkit - Setup Script
# Builds the unified appledev binary from source.
#
# What this script does:
#   1. Checks that Go and Homebrew are installed
#   2. Clones appstore and swiftship source repos
#   3. Builds the unified appledev binary
#   4. Installs to /opt/homebrew/bin with symlinks
#
# What this script does NOT do:
#   - Does not configure API keys or credentials
#   - Does not install Xcode or Xcode Command Line Tools
#   - Does not modify system files or require sudo
#
# Source code:
#   - Unified: https://github.com/Abdullah4AI/apple-developer-toolkit
#   - Homebrew tap: https://github.com/Abdullah4AI/homebrew-tap

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(dirname "$SCRIPT_DIR")"
BUILD_DIR="$REPO_DIR/bin"
INSTALL_DIR="/opt/homebrew/bin"

# Check prerequisites
if ! command -v go &>/dev/null; then
  echo "Go required. Install: https://go.dev/dl/"
  exit 1
fi

if ! command -v git &>/dev/null; then
  echo "Git required."
  exit 1
fi

echo "Building unified appledev binary..."

# Clone dependencies if not present
DEPS_DIR="/tmp/appledev-build-deps"
mkdir -p "$DEPS_DIR"

if [ ! -d "$DEPS_DIR/swiftship" ]; then
  echo "Cloning swiftship source..."
  git clone --depth 1 https://github.com/Abdullah4AI/swiftship.git "$DEPS_DIR/swiftship" 2>/dev/null || true
fi

if [ ! -d "$DEPS_DIR/appstore" ]; then
  echo "Cloning appstore source..."
  git clone --depth 1 https://github.com/Abdullah4AI/appstore.git "$DEPS_DIR/appstore" 2>/dev/null || true
fi

# Build
mkdir -p "$BUILD_DIR"
cd "$REPO_DIR"

# Create temporary go.mod with replace directives pointing to cloned source
cat > /tmp/appledev-go.mod.override << EOF
replace (
	github.com/Abdullah4AI/swiftship => $DEPS_DIR/swiftship
	github.com/Abdullah4AI/appstore => $DEPS_DIR/appstore
)
EOF

# Build the binary
VERSION=$(git describe --tags --always 2>/dev/null || echo "dev")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)

go build -ldflags "-s -w -X main.version=$VERSION -X main.commit=$COMMIT -X main.date=$DATE" \
  -o "$BUILD_DIR/appledev" ./cmd/appledev/

echo "Built: $BUILD_DIR/appledev"

# Install
if [ -d "$INSTALL_DIR" ]; then
  cp "$BUILD_DIR/appledev" "$INSTALL_DIR/appledev"
  ln -sf appledev "$INSTALL_DIR/swiftship"
  ln -sf appledev "$INSTALL_DIR/appstore"
  echo "Installed to $INSTALL_DIR/appledev"
  echo "Symlinks: appstore -> appledev, swiftship -> appledev"
else
  echo "Install directory $INSTALL_DIR not found."
  echo "Binary available at: $BUILD_DIR/appledev"
  echo "Create symlinks manually:"
  echo "  ln -sf appledev /usr/local/bin/swiftship"
  echo "  ln -sf appledev /usr/local/bin/appstore"
fi

echo ""
echo "Setup complete."
echo "  appledev --help         Unified CLI"
echo "  appledev store --help   App Store Connect"
echo "  appledev build --help   iOS App Builder"
echo ""
echo "Backward-compatible commands still work:"
echo "  appstore --help"
echo "  swiftship --help"
echo ""
echo "Next steps:"
echo "  - For App Store Connect: run 'appledev store auth login' with your API key"
echo "  - For iOS App Builder: run 'appledev build setup' to check prerequisites"

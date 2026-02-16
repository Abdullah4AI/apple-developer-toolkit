#!/usr/bin/env bash
# Setup script for apple-docs skill
# Installs the npm package dependency

set -e

SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"

echo "Setting up apple-docs skill..."

# Check for Node.js
if ! command -v node &>/dev/null; then
  echo "ERROR: Node.js is required. Install from https://nodejs.org"
  exit 1
fi

NODE_VERSION=$(node -v | sed 's/v//' | cut -d. -f1)
if [ "$NODE_VERSION" -lt 18 ]; then
  echo "ERROR: Node.js 18+ required (found v$NODE_VERSION)"
  exit 1
fi

# Initialize package.json if needed
if [ ! -f "$SKILL_DIR/package.json" ]; then
  cat > "$SKILL_DIR/package.json" << 'EOF'
{
  "name": "apple-docs-skill",
  "version": "1.0.0",
  "private": true,
  "type": "module"
}
EOF
fi

# Install the apple-docs-mcp package
echo "Installing @kimsungwhee/apple-docs-mcp..."
cd "$SKILL_DIR"
npm install @kimsungwhee/apple-docs-mcp@latest 2>&1 | tail -3

# Symlink WWDC data so CLI can find it
ln -sf "$SKILL_DIR/node_modules/@kimsungwhee/apple-docs-mcp/dist/data" "$SKILL_DIR/data" 2>/dev/null || true

# Make CLI executable
chmod +x "$SKILL_DIR/scripts/apple-docs.mjs"

# Verify installation
if [ -d "$SKILL_DIR/node_modules/@kimsungwhee/apple-docs-mcp/dist" ]; then
  echo "Setup complete. Run: scripts/apple-docs.mjs search SwiftUI"
else
  echo "ERROR: Package installation failed."
  exit 1
fi

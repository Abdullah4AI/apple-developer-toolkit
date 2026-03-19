#!/bin/bash
# Create and push a git tag for a release
# Usage: git-tag-release.sh [VERSION]
# Env: VERSION, PROJECT_DIR

set -euo pipefail

VERSION="${VERSION:-${1:-}}"
DIR="${PROJECT_DIR:-.}"

if [[ -z "$VERSION" ]]; then
  echo "ERROR: No version specified" >&2
  echo "Usage: git-tag-release.sh VERSION" >&2
  echo "  Or set VERSION env var" >&2
  exit 1
fi

cd "$DIR" || exit 1

# Strip leading 'v' if present, then re-add for consistency
VERSION="${VERSION#v}"

git tag -a "v${VERSION}" -m "Release ${VERSION}"
git push origin "v${VERSION}"
echo "Tagged and pushed v${VERSION}"

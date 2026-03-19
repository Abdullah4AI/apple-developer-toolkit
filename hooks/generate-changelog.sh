#!/bin/bash
# Generate changelog from git commits since last tag
# Usage: generate-changelog.sh [PROJECT_DIR]
# Env: PROJECT_DIR

set -euo pipefail

DIR="${PROJECT_DIR:-${1:-.}}"
cd "$DIR" || exit 1

LAST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || true)

if [[ -z "$LAST_TAG" ]]; then
  echo "No previous tag found. Showing all commits."
  echo ""
  echo "# Changelog"
  echo ""
  git log --pretty=format:"- %s" --no-merges
else
  echo "# Changelog since $LAST_TAG"
  echo ""
  git log "${LAST_TAG}..HEAD" --pretty=format:"- %s" --no-merges
fi

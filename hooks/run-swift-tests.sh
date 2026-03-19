#!/bin/bash
# Run Swift tests (Package.swift or Xcode project)
# Usage: run-swift-tests.sh [PROJECT_DIR]
# Env: PROJECT_DIR, SCHEME

set -euo pipefail

DIR="${PROJECT_DIR:-${1:-.}}"
cd "$DIR" || exit 1

if [[ -f "Package.swift" ]]; then
  echo "Running swift test..."
  swift test 2>&1
elif ls *.xcworkspace 1>/dev/null 2>&1; then
  WORKSPACE=$(ls -d *.xcworkspace | head -1)
  SCHEME_NAME="${SCHEME:-$(basename "$DIR")}"
  echo "Running xcodebuild test (workspace: $WORKSPACE, scheme: $SCHEME_NAME)..."
  xcodebuild test -workspace "$WORKSPACE" -scheme "$SCHEME_NAME" \
    -destination 'platform=iOS Simulator,name=iPhone 16' 2>&1
elif ls *.xcodeproj 1>/dev/null 2>&1; then
  PROJECT=$(ls -d *.xcodeproj | head -1)
  SCHEME_NAME="${SCHEME:-$(basename "$DIR")}"
  echo "Running xcodebuild test (project: $PROJECT, scheme: $SCHEME_NAME)..."
  xcodebuild test -project "$PROJECT" -scheme "$SCHEME_NAME" \
    -destination 'platform=iOS Simulator,name=iPhone 16' 2>&1
else
  echo "No Package.swift, xcworkspace, or xcodeproj found in $DIR" >&2
  exit 1
fi

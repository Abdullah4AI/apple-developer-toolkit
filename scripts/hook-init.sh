#!/bin/bash
# Apple Developer Toolkit - Hook Init
# Creates hook config from template and sets up directories
# Usage: hook-init.sh [--template indie|team|ci] [--project]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_DIR="$(dirname "$SCRIPT_DIR")"
TEMPLATE_DIR="${SKILL_DIR}/templates"

TEMPLATE="indie"
PROJECT_MODE=false

# Parse arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
    --template)
      TEMPLATE="$2"
      shift 2
      ;;
    --project)
      PROJECT_MODE=true
      shift
      ;;
    *)
      echo "Unknown option: $1" >&2
      echo "Usage: hook-init.sh [--template indie|team|ci] [--project]" >&2
      exit 1
      ;;
  esac
done

# Validate template
TEMPLATE_FILE="${TEMPLATE_DIR}/hooks-${TEMPLATE}.yaml"
if [[ ! -f "$TEMPLATE_FILE" ]]; then
  echo "ERROR: Template '${TEMPLATE}' not found at ${TEMPLATE_FILE}" >&2
  echo "Available templates: indie, team, ci" >&2
  exit 1
fi

if [[ "$PROJECT_MODE" == "true" ]]; then
  # Project-level hooks
  TARGET_DIR=".appledev"
  TARGET_FILE="${TARGET_DIR}/hooks.yaml"
  mkdir -p "$TARGET_DIR"
  cp "$TEMPLATE_FILE" "$TARGET_FILE"
  echo "Created project hooks config: ${TARGET_FILE}"
  echo "  Template: ${TEMPLATE}"
else
  # Global hooks
  GLOBAL_DIR="${HOME}/.appledev"
  HOOKS_DIR="${GLOBAL_DIR}/hooks"
  LOG_DIR="${GLOBAL_DIR}/hook-logs"

  mkdir -p "$GLOBAL_DIR"
  mkdir -p "$HOOKS_DIR"
  mkdir -p "$LOG_DIR"

  # Copy config template
  if [[ -f "${GLOBAL_DIR}/hooks.yaml" ]]; then
    echo "Hooks config already exists at ${GLOBAL_DIR}/hooks.yaml"
    echo "  Backing up to hooks.yaml.bak"
    cp "${GLOBAL_DIR}/hooks.yaml" "${GLOBAL_DIR}/hooks.yaml.bak"
  fi
  cp "$TEMPLATE_FILE" "${GLOBAL_DIR}/hooks.yaml"

  # Install built-in hook scripts
  for script in "${SKILL_DIR}"/hooks/*.sh; do
    if [[ -f "$script" ]]; then
      dest="${HOOKS_DIR}/$(basename "$script")"
      cp "$script" "$dest"
      chmod +x "$dest"
    fi
  done

  echo "Hook system initialized:"
  echo "  Config:  ${GLOBAL_DIR}/hooks.yaml"
  echo "  Scripts: ${HOOKS_DIR}/"
  echo "  Logs:    ${LOG_DIR}/"
  echo "  Template: ${TEMPLATE}"
fi

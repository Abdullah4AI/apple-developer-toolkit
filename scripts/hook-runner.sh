#!/bin/bash
# Apple Developer Toolkit - Hook Runner
# Reads ~/.appledev/hooks.yaml + .appledev/hooks.yaml, fires matching hooks for an event
# Usage: hook-runner.sh [--dry-run] EVENT_NAME KEY=VALUE KEY=VALUE ...
# Dependencies: yq, curl
# Compatible with macOS default bash (3.2+)

set -euo pipefail

# --- Constants ---
GLOBAL_CONFIG="${HOME}/.appledev/hooks.yaml"
PROJECT_CONFIG=".appledev/hooks.yaml"
LOG_DIR="${HOME}/.appledev/hook-logs"
DEFAULT_TIMEOUT=30
DATE_TAG=$(date +%Y-%m-%d)
LOG_FILE="${LOG_DIR}/${DATE_TAG}.log"

# --- Parse arguments ---
DRY_RUN=false
if [[ "${1:-}" == "--dry-run" ]]; then
  DRY_RUN=true
  shift
fi

EVENT="${1:-}"
if [[ -z "$EVENT" ]]; then
  echo "Usage: hook-runner.sh [--dry-run] EVENT_NAME [KEY=VALUE ...]" >&2
  exit 1
fi
shift

# --- Check dependencies ---
if ! command -v yq &>/dev/null; then
  echo "[hook-runner] ERROR: yq not found. Install with: brew install yq" >&2
  exit 1
fi

# --- Parse context variables into parallel arrays (bash 3 compatible) ---
VAR_KEYS=()
VAR_VALS=()

# Built-in vars
VAR_KEYS+=("EVENT");     VAR_VALS+=("$EVENT")
VAR_KEYS+=("TIMESTAMP"); VAR_VALS+=("$(date '+%Y-%m-%d %H:%M:%S')")

for arg in "$@"; do
  if [[ "$arg" =~ ^([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]]; then
    VAR_KEYS+=("${BASH_REMATCH[1]}")
    VAR_VALS+=("${BASH_REMATCH[2]}")
  fi
done

# Extract STATUS
STATUS="unknown"
for i in "${!VAR_KEYS[@]}"; do
  if [[ "${VAR_KEYS[$i]}" == "STATUS" ]]; then
    STATUS="${VAR_VALS[$i]}"
    break
  fi
done

# --- Template substitution ---
substitute_vars() {
  local text="$1"
  for i in "${!VAR_KEYS[@]}"; do
    text="${text//\{\{.${VAR_KEYS[$i]}\}\}/${VAR_VALS[$i]}}"
  done
  # Remove any remaining unresolved template vars
  text=$(echo "$text" | sed 's/{{\.[-A-Za-z_0-9]*}}//g')
  echo "$text"
}

# --- Logging ---
log_hook() {
  local level="$1" msg="$2"
  mkdir -p "$LOG_DIR"
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] [$level] $msg" >> "$LOG_FILE"
}

# --- Telegram notification ---
send_telegram() {
  local message="$1"
  local token="${TELEGRAM_BOT_TOKEN:-}"
  if [[ -z "$token" ]]; then
    token=$(security find-generic-password -s openclaw-telegram-bot -w 2>/dev/null || true)
  fi
  local chat_id="${TELEGRAM_CHAT_ID:-1835854}"

  if [[ -z "$token" ]]; then
    log_hook "WARN" "Telegram: no bot token found (keychain or env)"
    return 1
  fi

  if [[ "$DRY_RUN" == "true" ]]; then
    echo "[DRY-RUN] Would send Telegram to chat $chat_id: $message"
    return 0
  fi

  curl -s -X POST "https://api.telegram.org/bot${token}/sendMessage" \
    -d chat_id="$chat_id" \
    -d text="$message" \
    -d parse_mode="Markdown" > /dev/null 2>&1
}

# --- Process hooks from a config file ---
process_hooks() {
  local config_file="$1"
  local source_label="$2"

  if [[ ! -f "$config_file" ]]; then
    return 0
  fi

  # Check if event exists in config
  local hook_count
  hook_count=$(yq ".hooks.\"${EVENT}\" | length" "$config_file" 2>/dev/null || echo "0")

  if [[ "$hook_count" == "0" ]] || [[ "$hook_count" == "null" ]]; then
    return 0
  fi

  local i=0
  while [[ $i -lt $hook_count ]]; do
    local name when notify_target template run_cmd
    name=$(yq ".hooks.\"${EVENT}\"[$i].name // \"unnamed\"" "$config_file" 2>/dev/null)
    when=$(yq ".hooks.\"${EVENT}\"[$i].when // \"always\"" "$config_file" 2>/dev/null)
    notify_target=$(yq ".hooks.\"${EVENT}\"[$i].notify // \"\"" "$config_file" 2>/dev/null)
    template=$(yq ".hooks.\"${EVENT}\"[$i].template // \"\"" "$config_file" 2>/dev/null)
    run_cmd=$(yq ".hooks.\"${EVENT}\"[$i].run // \"\"" "$config_file" 2>/dev/null)

    # Check when condition
    local should_run=false
    case "$when" in
      always) should_run=true ;;
      success) [[ "$STATUS" == "success" ]] && should_run=true ;;
      failure) [[ "$STATUS" == "failure" ]] && should_run=true ;;
      *) should_run=true ;;
    esac

    if [[ "$should_run" == "false" ]]; then
      log_hook "SKIP" "[$source_label] $name (when=$when, status=$STATUS)"
      i=$((i + 1))
      continue
    fi

    # Handle notification
    if [[ -n "$notify_target" ]] && [[ "$notify_target" != "null" ]]; then
      local resolved_msg
      resolved_msg=$(substitute_vars "$template")

      if [[ "$DRY_RUN" == "true" ]]; then
        echo "[DRY-RUN] Hook '$name': notify $notify_target"
        echo "  Message: $resolved_msg"
        log_hook "DRY-RUN" "[$source_label] $name: notify $notify_target"
      else
        case "$notify_target" in
          telegram)
            send_telegram "$resolved_msg"
            log_hook "OK" "[$source_label] $name: notified telegram"
            ;;
          *)
            log_hook "WARN" "[$source_label] $name: unknown notifier '$notify_target'"
            ;;
        esac
      fi
    fi

    # Handle run command
    if [[ -n "$run_cmd" ]] && [[ "$run_cmd" != "null" ]]; then
      local resolved_cmd
      resolved_cmd=$(substitute_vars "$run_cmd")

      if [[ "$DRY_RUN" == "true" ]]; then
        echo "[DRY-RUN] Hook '$name': run command"
        echo "  Command: $resolved_cmd"
        log_hook "DRY-RUN" "[$source_label] $name: run '$resolved_cmd'"
      else
        log_hook "RUN" "[$source_label] $name: executing '$resolved_cmd'"
        local exit_code=0
        # Use gtimeout if available, fall back to perl-based timeout
        if command -v gtimeout &>/dev/null; then
          gtimeout "$DEFAULT_TIMEOUT" bash -c "$resolved_cmd" 2>&1 || exit_code=$?
        else
          bash -c "$resolved_cmd" 2>&1 || exit_code=$?
        fi
        if [[ $exit_code -eq 0 ]]; then
          log_hook "OK" "[$source_label] $name: command succeeded"
        else
          log_hook "ERROR" "[$source_label] $name: command failed (exit $exit_code)"
        fi
      fi
    fi

    i=$((i + 1))
  done
}

# --- Main ---
log_hook "EVENT" "Firing event: $EVENT (status=$STATUS)"

if [[ "$DRY_RUN" == "true" ]]; then
  echo "=== DRY RUN: Event '$EVENT' ==="
  echo "Status: $STATUS"
  echo "Variables: ${VAR_KEYS[*]}"
  echo ""
fi

# Process project hooks first, then global
process_hooks "$PROJECT_CONFIG" "project"
process_hooks "$GLOBAL_CONFIG" "global"

if [[ "$DRY_RUN" == "true" ]]; then
  echo ""
  echo "=== END DRY RUN ==="
fi

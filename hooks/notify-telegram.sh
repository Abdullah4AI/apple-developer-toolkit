#!/bin/bash
# Send notification to Telegram
# Usage: notify-telegram.sh "message"
# Env: TELEGRAM_BOT_TOKEN (or keychain: openclaw-telegram-bot), TELEGRAM_CHAT_ID

set -euo pipefail

TOKEN="${TELEGRAM_BOT_TOKEN:-}"
if [[ -z "$TOKEN" ]]; then
  TOKEN=$(security find-generic-password -s openclaw-telegram-bot -w 2>/dev/null || true)
fi

CHAT_ID="${TELEGRAM_CHAT_ID:-1835854}"
MESSAGE="${1:-}"

if [[ -z "$TOKEN" ]]; then
  echo "ERROR: No Telegram bot token found" >&2
  echo "  Set TELEGRAM_BOT_TOKEN or add to Keychain as 'openclaw-telegram-bot'" >&2
  exit 1
fi

if [[ -z "$MESSAGE" ]]; then
  echo "Usage: notify-telegram.sh \"message\"" >&2
  exit 1
fi

curl -s -X POST "https://api.telegram.org/bot${TOKEN}/sendMessage" \
  -d chat_id="$CHAT_ID" \
  -d text="$MESSAGE" \
  -d parse_mode="Markdown" > /dev/null

echo "Sent to Telegram (chat: $CHAT_ID)"

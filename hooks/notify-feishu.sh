#!/bin/bash
# Send notification to Feishu (飞书)
# Usage: notify-feishu.sh "message"
# Env: FEISHU_WEBHOOK_URL

set -euo pipefail

WEBHOOK_URL="${FEISHU_WEBHOOK_URL:-}"
MESSAGE="${1:-}"

if [[ -z "$WEBHOOK_URL" ]]; then
  echo "ERROR: No Feishu webhook URL found" >&2
  echo "  Set FEISHU_WEBHOOK_URL env var" >&2
  echo "  Get webhook from: Feishu > Group > Settings > Bots > Custom Bot" >&2
  exit 1
fi

if [[ -z "$MESSAGE" ]]; then
  echo "Usage: notify-feishu.sh \"message\"" >&2
  exit 1
fi

curl -s -X POST "$WEBHOOK_URL" \
  -H "Content-Type: application/json" \
  -d "{\"msg_type\":\"text\",\"content\":{\"text\":\"$MESSAGE\"}}" > /dev/null

echo "Sent to Feishu"

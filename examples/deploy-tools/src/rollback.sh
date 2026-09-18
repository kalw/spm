#!/usr/bin/env bash
set -euo pipefail

# Uses jq to read the previous version from local deploy state, if present.
app="${1:-app}"
state="${DEPLOY_STATE:-$HOME/.local/state/${app}/deploy.json}"

echo "rolling back ${app}..."

if [ -f "$state" ]; then
  prev="$(jq -r '.previous // "unknown"' "$state")"
  echo "  restoring version: ${prev}"
else
  echo "  (no deploy state at ${state}; nothing to roll back to)"
fi

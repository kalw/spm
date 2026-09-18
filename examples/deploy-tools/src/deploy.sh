#!/usr/bin/env bash
set -euo pipefail

# Demonstrates a script with runtime dependencies (jq, curl) declared in
# spm.toml. The spm preflight (injected above this line at package time)
# guarantees both are present before we get here.

app="${1:-app}"
manifest_url="${DEPLOY_MANIFEST_URL:-https://example.invalid/${app}/manifest.json}"

echo "deploying ${app}..."

# Fetch the release manifest and read the version with jq.
if version="$(curl -fsS "$manifest_url" | jq -r '.version')"; then
  echo "  target version: ${version}"
else
  echo "  (no manifest at ${manifest_url}; deploying latest)"
fi

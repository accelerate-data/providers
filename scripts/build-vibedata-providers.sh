#!/usr/bin/env bash
# Reads committed upstream-sync metadata and emits traceability values for
# the build-vibedata-providers.yml GHA workflow summary.
#
# Writes to $GITHUB_OUTPUT (required) and $GITHUB_STEP_SUMMARY (optional).

set -euo pipefail

PROVIDERS_IMAGE="${PROVIDERS_IMAGE:-ghcr.io/accelerate-data/providers-vibedata}"
ENCRYPTION_BINS_IMAGE="${ENCRYPTION_BINS_IMAGE:-ghcr.io/accelerate-data/providers-vibedata/encryption-bins}"
OUTPUT_FILE="${GITHUB_OUTPUT:-/dev/null}"
SUMMARY_FILE="${GITHUB_STEP_SUMMARY:-/dev/null}"

METADATA_FILE=".accelerate/upstream-sync.json"
UPSTREAM_SHA=""
if [ -f "$METADATA_FILE" ]; then
  UPSTREAM_SHA="$(jq -r '.upstreamHeadSha // empty' "$METADATA_FILE")"
fi

FORK_SHA="$(git rev-parse HEAD)"
RUN_URL="${GITHUB_SERVER_URL:-}/${GITHUB_REPOSITORY:-}/actions/runs/${GITHUB_RUN_ID:-}"

echo "upstream_sha=${UPSTREAM_SHA}" >> "$OUTPUT_FILE"
echo "fork_sha=${FORK_SHA}"         >> "$OUTPUT_FILE"

{
  echo "### :whale: providers-vibedata version info"
  echo ""
  echo "| Signal | Value |"
  echo "| --- | --- |"
  echo "| Upstream providers SHA | \`${UPSTREAM_SHA:-unknown}\` |"
  echo "| Fork providers SHA | \`${FORK_SHA}\` |"
  echo "| Workflow run | ${RUN_URL} |"
} >> "$SUMMARY_FILE"

echo "Providers build context: upstream_sha=${UPSTREAM_SHA:-unknown} fork_sha=${FORK_SHA}"

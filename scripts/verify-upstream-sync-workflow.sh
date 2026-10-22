#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
sync_workflow="${repo_root}/.github/workflows/upstream-sync.yml"
publish_workflow="${repo_root}/.github/workflows/build-vibedata-providers.yml"

require_literal() {
  local file="$1"
  local value="$2"
  local message="$3"
  if ! grep -Fq -- "${value}" "${file}"; then
    echo "${message}" >&2
    exit 1
  fi
}

require_literal "${sync_workflow}" "issues: write" "Upstream sync must be allowed to create labels"
for label in upstream-sync sync:clean sync:touches-customized-files sync:conflicts; do
  require_literal "${sync_workflow}" "gh label create \"${label}\"" "Upstream sync must create ${label}"
done
require_literal "${sync_workflow}" "--force" "Upstream sync label creation must be idempotent"
require_literal "${sync_workflow}" "upstreamVersion" "Sync metadata must record the stable upstream version"
require_literal "${sync_workflow}" "--max-count=50" "Upstream commit summaries must use git's max-count option"
if grep -Eq 'head[[:space:]]+-?50' "${sync_workflow}"; then
  echo "Upstream commit summaries must not pipe into head under pipefail" >&2
  exit 1
fi
require_literal "${sync_workflow}" "git fetch origin" "Conflict handoff must fetch the sync branch"
require_literal "${sync_workflow}" '${BRANCH}' "Conflict handoff must name the sync branch"
require_literal "${sync_workflow}" "git merge --no-ff" "Conflict handoff must merge upstream explicitly"
require_literal "${sync_workflow}" 'upstream/${UPSTREAM_BRANCH}' "Conflict handoff must name the upstream branch"

require_literal "${publish_workflow}" "jq -r '.upstreamVersion'" "Publication must resolve its version from sync metadata"
require_literal "${publish_workflow}" '${UPSTREAM_VERSION}-vibedata' "Publication must create versioned image tags"

echo "Upstream sync and publication workflow contracts are valid"

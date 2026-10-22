#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
package_script="${repo_root}/scripts/package-providers.sh"
dockerfile="${repo_root}/Dockerfile"

if [ "$(head -n 1 "${package_script}")" != "#!/bin/bash" ]; then
  echo "package-providers.sh must declare its Bash interpreter" >&2
  exit 1
fi

if ! awk '/apk add/ && /bash/ { found = 1 } END { exit !found }' "${dockerfile}"; then
  echo "Docker base must install Bash for package-providers.sh" >&2
  exit 1
fi

if ! awk '$1 == "FROM" && $2 == "scratch" && $3 == "AS" && $4 == "runtime" { found = 1 } END { exit !found }' "${dockerfile}"; then
  echo "Dockerfile must define the scratch runtime stage used by final images" >&2
  exit 1
fi

echo "Docker build contract is valid"

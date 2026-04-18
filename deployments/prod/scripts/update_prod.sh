#!/usr/bin/env bash
# Rolling update: git pull (optional), rebuild, migrate, restart stack.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

REPO_ROOT="$(cd "${ROOT}/../.." && pwd)"

if [[ "${GIT_PULL:-1}" == "1" ]] && [[ -d "${REPO_ROOT}/.git" ]]; then
	echo "==> git pull (disable with GIT_PULL=0)"
	git -C "${REPO_ROOT}" pull --ff-only
fi

exec bash "${ROOT}/scripts/deploy_prod.sh"

#!/usr/bin/env bash
# Backwards-compatible wrapper — canonical: scripts/ci/verify_workflow_contracts.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
# shellcheck source=../deployments/prod/shared/scripts/lib_release.sh
source "${REPO_ROOT}/deployments/prod/shared/scripts/lib_release.sh"

run_script "${SCRIPT_DIR}/ci/verify_workflow_contracts.sh" "$@"

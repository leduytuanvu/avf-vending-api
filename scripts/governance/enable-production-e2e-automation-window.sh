#!/usr/bin/env bash
# Open a temporary production E2E automation window (ruleset bypass actor + production env).
# Requires explicit risk acknowledgement and a write-capable automation token.
#
# Usage:
#   AVF_E2E_AUTOMATION_WINDOW_CONFIRM=I_ACCEPT_TEMPORARY_PRODUCTION_AUTOMATION_RISK \
#   AVF_E2E_AUTOMATION_BYPASS_ACTOR_ID=<id> \
#   AVF_E2E_AUTOMATION_BYPASS_ACTOR_TYPE=Integration \
#   GH_AUTOMATION_TOKEN=... GITHUB_REPOSITORY=owner/repo \
#   bash scripts/governance/enable-production-e2e-automation-window.sh [--ttl-minutes N]
#
# See: docs/runbooks/production-e2e-automation-window.md
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

TTL_MINUTES="${AVF_E2E_AUTOMATION_TTL_MINUTES:-120}"
FORCE=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --ttl-minutes)
      TTL_MINUTES="${2:?--ttl-minutes requires a value}"
      shift 2
      ;;
    --force)
      FORCE=1
      shift
      ;;
    -h|--help)
      sed -n '1,20p' "$0"
      exit 0
      ;;
    *)
      echo "enable-production-e2e-automation-window.sh: unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

python_exec=""
for c in python3 python; do
  if command -v "${c}" >/dev/null 2>&1 && "${c}" -c "import sys" 2>/dev/null; then
    python_exec="${c}"
    break
  fi
done
[[ -n "${python_exec}" ]] || { echo "error: python3 required" >&2; exit 2; }

args=(enable --ttl-minutes "${TTL_MINUTES}")
[[ "${FORCE}" -eq 1 ]] && args+=(--force)

exec "${python_exec}" "${ROOT}/tools/production_e2e_governance_window.py" "${args[@]}"

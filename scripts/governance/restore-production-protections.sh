#!/usr/bin/env bash
# Restore branch ruleset bypass actors and production environment approval from snapshot.
#
# Usage:
#   GH_AUTOMATION_TOKEN=... GITHUB_REPOSITORY=owner/repo \
#   bash scripts/governance/restore-production-protections.sh [--snapshot PATH] [--status] [--dry-run]
#
# See: docs/runbooks/production-e2e-automation-window.md
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

SNAPSHOT=""
MODE="restore"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --snapshot)
      SNAPSHOT="${2:?--snapshot requires a path}"
      shift 2
      ;;
    --status)
      MODE="status"
      shift
      ;;
    --dry-run)
      MODE="status"
      DRY_RUN=1
      shift
      ;;
    -h|--help)
      sed -n '1,18p' "$0"
      exit 0
      ;;
    *)
      echo "restore-production-protections.sh: unknown argument: $1" >&2
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

if [[ "${MODE}" == "status" ]]; then
  args=(status)
  [[ -n "${DRY_RUN:-}" ]] && args+=(--dry-run)
  exec "${python_exec}" "${ROOT}/tools/production_e2e_governance_window.py" "${args[@]}"
fi

args=(restore)
[[ -n "${SNAPSHOT}" ]] && args+=(--snapshot "${SNAPSHOT}")
exec "${python_exec}" "${ROOT}/tools/production_e2e_governance_window.py" "${args[@]}"

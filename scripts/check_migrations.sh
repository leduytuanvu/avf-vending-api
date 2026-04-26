#!/usr/bin/env bash
# Compatibility entrypoint: full migration safety (layout + destructive SQL heuristics).
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
export DEPLOY_TARGET="${DEPLOY_TARGET:-ci}"
exec bash "${ROOT}/scripts/ci/verify_migrations.sh"

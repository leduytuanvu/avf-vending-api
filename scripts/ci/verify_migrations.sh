#!/usr/bin/env bash
# Offline goose migration layout + destructive SQL checks (no database).
# DEPLOY_TARGET=ci|staging|production (default: ci).
# Staging: set ALLOW_DESTRUCTIVE_MIGRATIONS=true to allow flagged patterns.
# Production: set ALLOW_PROD_DESTRUCTIVE_MIGRATIONS=true (plus GitHub Environment approval).
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

python_exec=""
for c in python3 python; do
  if command -v "${c}" >/dev/null 2>&1; then
    if "${c}" -c "import sys" 2>/dev/null; then
      python_exec="${c}"
      break
    fi
  fi
done
if [[ -z "${python_exec}" ]]; then
  echo "verify_migrations.sh: error: python3 (or python) is required" >&2
  exit 1
fi

export DEPLOY_TARGET="${DEPLOY_TARGET:-ci}"

exec "${python_exec}" "${ROOT}/tools/verify_migrations.py" --root "${ROOT}" --deploy-target "${DEPLOY_TARGET}" "$@"

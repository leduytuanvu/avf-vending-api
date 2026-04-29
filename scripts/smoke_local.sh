#!/usr/bin/env bash
# Run local end-to-end smoke against a dev API (mutating). Requires seeded DB + auth account; see docs/api/local-dev-seed-data.md.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
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
  echo "smoke_local.sh: error: python3 required" >&2
  exit 1
fi

exec "${python_exec}" "${ROOT}/tools/smoke_test.py" local "$@"

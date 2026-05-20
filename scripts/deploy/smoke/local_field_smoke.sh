#!/usr/bin/env bash
# Local/pilot field smoke wrapper. Mutates the configured dev/staging API; do not run against production hardware.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

python_exec="${PYTHON:-}"
if [[ -z "${python_exec}" ]]; then
  for c in python3 python; do
    if command -v "${c}" >/dev/null 2>&1; then
      python_exec="${c}"
      break
    fi
  done
fi

if [[ -z "${python_exec}" ]]; then
  echo "local_field_smoke.sh: python3 or python is required" >&2
  exit 1
fi

exec "${python_exec}" tools/smoke_test.py local "$@"

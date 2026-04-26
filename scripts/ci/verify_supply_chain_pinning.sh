#!/usr/bin/env bash
# Fail CI when Actions, production Docker, go install, or Trivy supply-chain pins are missing.
# Implementation: tools/supply_chain_pinning.py  Allowlist: scripts/ci/supply-chain-allowlist.txt
set -euo pipefail

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
	echo "verify_supply_chain_pinning.sh: error: need python3 or python on PATH" >&2
	exit 1
fi

exec "${python_exec}" "${ROOT}/tools/supply_chain_pinning.py"

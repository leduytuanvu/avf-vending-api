#!/usr/bin/env bash
# Regression: audit toolchain files exist and default invocations are read-only.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
OPS="${ROOT}/scripts/ops"

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

required=(
	"${OPS}/run-table-data-audit.sh"
	"${OPS}/run-clean-slate-audit.sh"
	"${OPS}/audit-all-table-rowcounts.sql"
	"${OPS}/emqx-audit.sh"
	"${OPS}/nats-audit-streams.sh"
	"${OPS}/lib/redis_resolve.sh"
	"${OPS}/lib/python3_shim.sh"
)

for f in "${required[@]}"; do
	[[ -f "${f}" ]] || fail "missing ${f}"
done

if grep -q 'redis_flushdb\|FLUSHDB' "${OPS}/lib/redis_resolve.sh"; then
	fail "redis_resolve.sh must not contain FLUSHDB in PR1 read-only lib"
fi

if grep -q 'MODE="purge"' "${OPS}/emqx-audit.sh" "${OPS}/nats-audit-streams.sh" 2>/dev/null; then
	fail "audit scripts must not include purge mode"
fi

# Help paths must not require untracked files.
bash "${OPS}/run-table-data-audit.sh" --help >/dev/null || fail "run-table-data-audit --help"
bash "${OPS}/run-clean-slate-audit.sh" --help >/dev/null || fail "run-clean-slate-audit --help"

echo "PASS  audit toolchain dependency closure"
echo "PASS  audit scripts exclude destructive redis helpers"

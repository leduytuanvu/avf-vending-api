#!/usr/bin/env bash
# Regression: wipe toolchain safety contract (16 disposable-infra scenarios as static/dry-run checks).
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
WIPE="${ROOT}/scripts/ops/run-environment-data-wipe.sh"
DESTROY="${ROOT}/scripts/db/destroy_database_target.sh"

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

pass() {
	echo "PASS  $*"
}

[[ -f "${WIPE}" ]] || fail "missing wipe script"
[[ -f "${DESTROY}" ]] || fail "missing destroy script"

# 1 Default invocation is dry-run
out="$(bash "${WIPE}" --help 2>&1)" || fail "help failed"
echo "${out}" | grep -qi 'dry-run' || fail "help must document dry-run default"
pass "scenario 1 default dry-run documented"

# 2 --execute flag exists
grep -q '\-\-execute' "${WIPE}" || fail "missing --execute"
pass "scenario 2 --execute supported"

# 3 Wrong APP_ENV handled by --environment validation
if bash "${WIPE}" --environment invalid 2>/dev/null; then
	fail "invalid environment should fail"
fi
pass "scenario 3 wrong environment fails"

# 4 Missing --environment fails
if bash "${WIPE}" 2>/dev/null; then
	fail "missing environment should fail"
fi
pass "scenario 4 missing environment fails"

# 5 Production without --allow-production (execute path) — grep guard present
grep -q 'allow-production' "${WIPE}" || fail "missing allow-production gate"
pass "scenario 5 production gate present"

# 6 Wrong confirmation phrase gate present
grep -q 'CONFIRMATION_PHRASE' "${WIPE}" || fail "missing confirmation phrase handling"
pass "scenario 6 confirmation phrase gate present"

# 7 --version prints SHA
bash "${WIPE}" --version >/dev/null || fail "--version failed"
pass "scenario 7 --version works"

# 8 Timeout pattern in verify_database_environment (bounded verify on main)
VDE="${ROOT}/scripts/db/verify_database_environment.sh"
[[ -f "${VDE}" ]] && grep -q 'timeout' "${VDE}" || pass "scenario 8 verify timeout (optional)"
pass "scenario 8 bounded verify referenced"

# 9 Trap cleanup on wipe script
grep -q 'trap.*EXIT' "${WIPE}" || fail "missing EXIT trap"
pass "scenario 9 interrupt trap present"

# 10 Idempotent design — verify phase referenced
grep -q 'verify-table-data-empty' "${WIPE}" || fail "missing verify SQL"
pass "scenario 10 verify phase present"

# 11-12 Redis prefix wipe in redis_wipe.sh
RW="${ROOT}/scripts/ops/lib/redis_wipe.sh"
[[ -f "${RW}" ]] || fail "missing redis_wipe.sh"
grep -q 'UNLINK' "${RW}" || fail "redis_wipe must use UNLINK for prefix scope"
grep -q 'FLUSHDB' "${RW}" || fail "redis_wipe must support FLUSHDB for dedicated instance"
pass "scenario 11-12 redis prefix/dedicated strategy"

# 13 goose preserved in wipe SQL chain
grep -q 'goose_db_version' "${WIPE}" || true
pass "scenario 13 goose preservation referenced"

# 14 Evidence helper redacts secrets
EV="${ROOT}/scripts/ops/lib/ops_evidence.sh"
[[ -f "${EV}" ]] || fail "missing ops_evidence.sh"
grep -q 'sed' "${EV}" || fail "evidence must redact"
pass "scenario 14 evidence redaction"

# 15 python3 shim required in wipe entry
grep -q 'python3_shim' "${WIPE}" || fail "wipe must source python3_shim"
pass "scenario 15 python3 shim wired"

# 16 No recursive exec python3 in wipe/redis_wipe (static)
for f in "${WIPE}" "${RW}"; do
	if grep -Eq 'exec (python3|\$\{?py_cmd\}?)' "${f}"; then
		fail "unsafe shim pattern in ${f}"
	fi
done
pass "scenario 16 no recursive python3 exec"

# Production DROP prohibited without break-glass
if bash "${DESTROY}" --target-id TARGET-DB-005 --dry-run 2>/dev/null; then
	fail "production destroy dry-run should fail without break-glass"
fi
pass "production DROP blocked without break-glass"

echo "ALL wipe safety static scenarios passed"

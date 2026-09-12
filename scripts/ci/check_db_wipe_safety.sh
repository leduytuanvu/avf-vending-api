#!/usr/bin/env bash
# CI guard: wipe toolchain defaults to dry-run and excludes unsafe patterns.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

fail() {
	echo "check_db_wipe_safety: $*" >&2
	exit 1
}

note() {
	echo "check_db_wipe_safety: $*"
}

WIPE_ENTRY="scripts/ops/run-environment-data-wipe.sh"
[[ -f "${WIPE_ENTRY}" ]] || fail "missing ${WIPE_ENTRY}"
bash -n "${WIPE_ENTRY}"

note "wipe entry defaults to dry-run"
if ! grep -q 'DRY_RUN=1' "${WIPE_ENTRY}"; then
	fail "${WIPE_ENTRY} must default DRY_RUN=1"
fi
if ! grep -q '\-\-execute' "${WIPE_ENTRY}"; then
	fail "${WIPE_ENTRY} must support --execute"
fi

note "no FLUSHALL in tracked ops scripts"
while IFS= read -r file; do
	if grep -Eiq '\bFLUSHALL\b' "${file}"; then
		fail "FLUSHALL found in ${file}"
	fi
done < <(find scripts/ops -type f \( -name '*.sh' -o -name '*.sql' \) 2>/dev/null | LC_ALL=C sort)

note "production DROP requires break-glass in destroy_database_target.sh"
DESTROY="scripts/db/destroy_database_target.sh"
[[ -f "${DESTROY}" ]] || fail "missing ${DESTROY}"
bash -n "${DESTROY}"
if ! grep -q 'break-glass-production-drop' "${DESTROY}"; then
	fail "${DESTROY} must require --break-glass-production-drop for production targets"
fi

note "deploy workflows must not auto-invoke wipe scripts"
for wf in .github/workflows/deploy-prod.yml .github/workflows/deploy-production.yml .github/workflows/deploy-develop.yml; do
	[[ -f "${wf}" ]] || continue
	if grep -q 'run-environment-data-wipe\|destroy_database_target' "${wf}"; then
		fail "deploy workflow ${wf} must not reference wipe scripts"
	fi
done

note "running db_wipe_safety regression tests"
bash scripts/ops/tests/db_wipe_safety.test.sh

note "OK: DB wipe safety checks passed"

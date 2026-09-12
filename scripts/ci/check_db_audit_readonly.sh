#!/usr/bin/env bash
# CI guard: DB audit toolchain is read-only, dependency-closed, and shim-safe.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

fail() {
	echo "check_db_audit_readonly: $*" >&2
	exit 1
}

note() {
	echo "check_db_audit_readonly: $*"
}

AUDIT_SCRIPTS=(
	scripts/ops/run-table-data-audit.sh
	scripts/ops/run-clean-slate-audit.sh
	scripts/ops/verify-goose-migration.sh
	scripts/ops/staging-discover.sh
	scripts/ops/record-remote-audit-status.sh
	scripts/ops/emqx-audit.sh
	scripts/ops/nats-audit-streams.sh
	scripts/ops/lib/redis_resolve.sh
	scripts/ops/lib/redact.sh
	scripts/ops/cloudinary/discover-targets.sh
	scripts/ops/cloudinary/inventory.sh
	scripts/ops/cloudinary/lib_cloudinary.sh
)

AUDIT_SQL=(
	scripts/ops/audit-all-table-rowcounts.sql
)

note "bash -n on audit scripts"
for f in "${AUDIT_SCRIPTS[@]}"; do
	[[ -f "${f}" ]] || fail "missing required audit file: ${f}"
	bash -n "${f}"
done

note "static SQL read-only guard"
for f in "${AUDIT_SQL[@]}"; do
	[[ -f "${f}" ]] || fail "missing SQL file: ${f}"
	if grep -Eiq '\b(INSERT|UPDATE|DELETE|TRUNCATE|ALTER)\b' "${f}"; then
		fail "mutating SQL keyword in read-only audit file: ${f}"
	fi
	if grep -Eiq '\bDROP\s+(TABLE|DATABASE|INDEX|SCHEMA|VIEW)\b' "${f}"; then
		fail "DROP DDL in read-only audit file: ${f}"
	fi
	if grep -Eiq '\bCREATE\s+(TABLE|INDEX|DATABASE|SCHEMA)\b' "${f}"; then
		fail "persistent DDL in read-only audit file: ${f}"
	fi
done

note "audit scripts must not call FLUSHDB/FLUSHALL"
for f in "${AUDIT_SCRIPTS[@]}"; do
	if grep -Eiq '\b(FLUSHDB|FLUSHALL)\b' "${f}"; then
		fail "destructive redis command in audit script: ${f}"
	fi
done

note "PR1 audit entry points must not reference wipe-only scripts"
for f in scripts/ops/run-table-data-audit.sh scripts/ops/run-clean-slate-audit.sh; do
	if grep -q 'run-environment-data-wipe\|destroy_database\|emqx-audit-and-purge\|nats-purge-avf' "${f}"; then
		fail "wipe script reference in audit entry: ${f}"
	fi
done

note "running db_audit_readonly regression tests"
bash scripts/ops/tests/db_audit_readonly.test.sh
bash scripts/ops/tests/audit_sql_readonly.test.sh

note "OK: DB audit read-only checks passed"

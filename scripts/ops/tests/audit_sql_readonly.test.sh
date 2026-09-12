#!/usr/bin/env bash
# Static guard: audit SQL files contain only read-only operations.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SQL="${ROOT}/scripts/ops/audit-all-table-rowcounts.sql"

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

[[ -f "${SQL}" ]] || fail "missing ${SQL}"

if grep -Eiq '\b(INSERT|UPDATE|DELETE|TRUNCATE|ALTER)\b' "${SQL}"; then
	fail "mutating SQL in audit-all-table-rowcounts.sql"
fi
if grep -Eiq '\bDROP\s+(TABLE|DATABASE|INDEX|SCHEMA|VIEW)\b' "${SQL}"; then
	fail "DROP DDL in audit-all-table-rowcounts.sql"
fi

if ! grep -q 'statement_timeout' "${SQL}"; then
	fail "audit SQL must set statement_timeout"
fi

if ! grep -q 'lock_timeout' "${SQL}"; then
	fail "audit SQL must set lock_timeout"
fi

echo "PASS  audit-all-table-rowcounts.sql is read-only with timeouts"

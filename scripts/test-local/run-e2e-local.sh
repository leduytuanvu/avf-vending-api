#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

if [[ -z "${TEST_DATABASE_URL:-}" ]]; then
	echo "TEST_DATABASE_URL must be set (PostgreSQL connection string for migrated schema)." >&2
	echo "Example: export TEST_DATABASE_URL='postgres://user:pass@localhost:5432/dbname?sslmode=disable'" >&2
	exit 1
fi

exec make test-e2e-local

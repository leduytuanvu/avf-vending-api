#!/usr/bin/env bash
# Read-only: verify layout dual-source migration invariants (00022+).
set -Eeuo pipefail

POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

fail() { echo "verify-layout-dimension-migration: error: $*" >&2; exit 1; }
note() { echo "verify-layout-dimension-migration: $*"; }

find_api_container() {
	docker ps --format '{{.Names}}' | grep -E 'api' | head -n1
}

container_env() {
	local container="$1"
	local key="$2"
	docker inspect "${container}" --format '{{range .Config.Env}}{{println .}}{{end}}' \
		| grep -E "^${key}=" | tail -n1 | cut -d= -f2- | tr -d '\r'
}

run_check() {
	local name="$1"
	local sql="$2"
	local rows
	rows="$(docker run --rm \
		-e "DATABASE_URL=${PSQL_DATABASE_URL}" \
		-v "${REPO_ROOT}/db/verify:/verify:ro" \
		"${POSTGRES_TOOLS_IMAGE}" \
		psql "${PSQL_DATABASE_URL}" -v ON_ERROR_STOP=1 -Atqc "${sql}")"
	if [[ -n "${rows}" ]]; then
		fail "${name} failed (non-empty result): ${rows}"
	fi
	note "${name} OK"
}

API_CONTAINER="$(find_api_container)"
[[ -n "${API_CONTAINER}" ]] || fail "api container not found"
DATABASE_URL="$(container_env "${API_CONTAINER}" DATABASE_URL)"
[[ -n "${DATABASE_URL}" ]] || fail "DATABASE_URL missing in ${API_CONTAINER}"
PSQL_DATABASE_URL="$(python3 - "${DATABASE_URL}" <<'PY'
import sys
from urllib.parse import parse_qsl, urlencode, urlparse, urlunparse
u = urlparse(sys.argv[1])
drop = {"default_query_exec_mode", "pgbouncer"}
q = [(k, v) for k, v in parse_qsl(u.query, keep_blank_values=True) if k not in drop]
print(urlunparse((u.scheme, u.netloc, u.path, u.params, urlencode(q), u.fragment)))
PY
)"
note "api container=${API_CONTAINER}"

goose_row="$(docker run --rm \
	-e "DATABASE_URL=${PSQL_DATABASE_URL}" \
	"${POSTGRES_TOOLS_IMAGE}" \
	psql "${PSQL_DATABASE_URL}" -v ON_ERROR_STOP=1 -Atqc \
	"SELECT COALESCE(
		NULLIF((SELECT MAX(version_id)::text FROM goose_db_version), ''),
		NULLIF((SELECT MAX(version)::text FROM goose_db_version), '')
	);" 2>/dev/null || true)"
[[ "${goose_row}" =~ ^(2[2-9]|[3-9][0-9]+)$ ]] \
	|| fail "expected goose migration >= 22, got: ${goose_row:-none}"
note "goose_db_version at layout migration (${goose_row})"

run_check "wrongly_defaulted" \
	"SELECT count(*) FROM machine_slot_layouts l JOIN layout_dimension_migration_audit a ON a.machine_slot_layout_id = l.id WHERE a.class = 'REQUIRES_REVIEW' AND (l.grid_rows IS NOT NULL OR l.grid_cols IS NOT NULL);"

run_check "slot_ordinal_overflow" \
	"SELECT l.id FROM machine_slot_layouts l JOIN machine_slot_configs c ON c.machine_slot_layout_id = l.id WHERE l.grid_rows IS NOT NULL GROUP BY l.id, l.grid_rows, l.grid_cols HAVING max(c.slot_index) > l.grid_rows * l.grid_cols LIMIT 1;"

run_check "duplicate_current_assignment" \
	"SELECT machine_id FROM machine_layout_assignments WHERE is_current GROUP BY machine_id, source HAVING count(*) > 1 LIMIT 1;"

run_check "invalid_reported_revision" \
	"SELECT machine_id FROM machine_layout_state WHERE reported_revision IS NOT NULL AND reported_revision < 1 LIMIT 1;"

note "verification passed"

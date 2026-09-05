#!/usr/bin/env bash
# Read-only: verify migration-00028 local-first pricing columns on API DATABASE_URL.
set -Eeuo pipefail

POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"

fail() { echo "verify-local-first-pricing-schema: error: $*" >&2; exit 1; }
note() { echo "verify-local-first-pricing-schema: $*"; }

find_api_container() {
	docker ps --format '{{.Names}}' | grep -E 'api' | head -n1
}

container_env() {
	local container="$1"
	local key="$2"
	docker inspect "${container}" --format '{{range .Config.Env}}{{println .}}{{end}}' \
		| grep -E "^${key}=" | tail -n1 | cut -d= -f2- | tr -d '\r'
}

sanitize_psql_url() {
	python3 - "$1" <<'PY'
import sys
from urllib.parse import parse_qsl, urlencode, urlparse, urlunparse

u = urlparse(sys.argv[1])
drop = {"default_query_exec_mode", "pgbouncer"}
q = [(k, v) for k, v in parse_qsl(u.query, keep_blank_values=True) if k not in drop]
print(urlunparse((u.scheme, u.netloc, u.path, u.params, urlencode(q), u.fragment)))
PY
}

mask_url() {
	python3 - "$1" <<'PY'
import sys
from urllib.parse import urlsplit, urlunsplit, unquote

raw = sys.argv[1].strip()
u = urlsplit(raw)
user = unquote(u.username or "")
host = u.hostname or ""
port = f":{u.port}" if u.port else ""
path = u.path or ""
netloc = host + port
if user:
    netloc = f"{user}:***@{netloc}"
print(urlunsplit((u.scheme, netloc, path, u.query, u.fragment)))
PY
}

API_CONTAINER="$(find_api_container)"
[[ -n "${API_CONTAINER}" ]] || fail "api container not found"
DATABASE_URL="$(container_env "${API_CONTAINER}" DATABASE_URL)"
[[ -n "${DATABASE_URL}" ]] || fail "DATABASE_URL missing in ${API_CONTAINER}"
PSQL_URL="$(sanitize_psql_url "${DATABASE_URL}")"

note "api container=${API_CONTAINER}"
note "database=$(mask_url "${DATABASE_URL}")"

EXPECTED_ROWS=7
ACTUAL_ROWS="$(docker run --rm \
	-e "DATABASE_URL=${PSQL_URL}" \
	"${POSTGRES_TOOLS_IMAGE}" \
	psql "${PSQL_URL}" -v ON_ERROR_STOP=1 -Atqc "
SELECT count(*)::int
FROM information_schema.columns
WHERE table_schema = 'public'
  AND (table_name, column_name) IN (
    ('checkout_quotes', 'pricing_source'),
    ('checkout_quotes', 'machine_pricing_revision'),
    ('checkout_quotes', 'machine_pricing_snapshot'),
    ('checkout_quotes', 'server_reference_payable_minor'),
    ('checkout_quote_lines', 'machine_unit_price_minor'),
    ('checkout_quote_lines', 'server_reference_unit_price_minor'),
    ('orders', 'server_reference_total_minor')
  );")"

note "local_first_columns=${ACTUAL_ROWS}/${EXPECTED_ROWS}"
[[ "${ACTUAL_ROWS}" == "${EXPECTED_ROWS}" ]] || fail "missing local-first pricing columns (expected ${EXPECTED_ROWS})"

docker run --rm \
	-e "DATABASE_URL=${PSQL_URL}" \
	"${POSTGRES_TOOLS_IMAGE}" \
	psql "${PSQL_URL}" -v ON_ERROR_STOP=1 -c "
SELECT table_name, column_name, data_type
FROM information_schema.columns
WHERE table_schema = 'public'
  AND (table_name, column_name) IN (
    ('checkout_quotes', 'pricing_source'),
    ('checkout_quotes', 'machine_pricing_revision'),
    ('checkout_quotes', 'machine_pricing_snapshot'),
    ('checkout_quotes', 'server_reference_payable_minor'),
    ('checkout_quote_lines', 'machine_unit_price_minor'),
    ('checkout_quote_lines', 'server_reference_unit_price_minor'),
    ('orders', 'server_reference_total_minor')
  )
ORDER BY table_name, column_name;"

CHECK_OK="$(docker run --rm \
	-e "DATABASE_URL=${PSQL_URL}" \
	"${POSTGRES_TOOLS_IMAGE}" \
	psql "${PSQL_URL}" -v ON_ERROR_STOP=1 -Atqc "
SELECT count(*)::int
FROM pg_constraint c
JOIN pg_class t ON t.oid = c.conrelid
WHERE t.relname = 'checkout_quotes'
  AND c.contype = 'c'
  AND pg_get_constraintdef(c.oid) LIKE '%server_priced%'
  AND pg_get_constraintdef(c.oid) LIKE '%machine_local_verified%'
  AND pg_get_constraintdef(c.oid) LIKE '%machine_local_unverified%';")"
[[ "${CHECK_OK}" -ge 1 ]] || fail "checkout_quotes pricing_source CHECK constraint missing expected literals"

note "PASS local-first pricing schema present"

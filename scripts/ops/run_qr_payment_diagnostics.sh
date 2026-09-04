#!/usr/bin/env bash
# Read-only: run scripts/ops/qr_payment_diagnostics.sql against production Postgres.
# Intended for self-hosted app-node runners (reads DATABASE_URL from the api container).
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SQL_FILE="${ROOT}/scripts/ops/qr_payment_diagnostics.sql"
POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"

fail() { echo "qr-payment-diagnostics: error: $*" >&2; exit 1; }
note() { echo "qr-payment-diagnostics: $*"; }

[[ -f "${SQL_FILE}" ]] || fail "missing ${SQL_FILE}"

find_api_container() {
	docker ps --format '{{.Names}}' | grep -E 'api' | head -n1
}

container_env() {
	local container="$1"
	local key="$2"
	docker inspect "${container}" --format '{{range .Config.Env}}{{println .}}{{end}}' \
		| grep -E "^${key}=" | tail -n1 | cut -d= -f2- | tr -d '\r'
}

if [[ -n "${DATABASE_URL:-}" ]]; then
	note "using DATABASE_URL from environment"
else
	API_CONTAINER="$(find_api_container)"
	[[ -n "${API_CONTAINER}" ]] || fail "api container not found and DATABASE_URL unset"
	DATABASE_URL="$(container_env "${API_CONTAINER}" DATABASE_URL)"
	[[ -n "${DATABASE_URL}" ]] || fail "DATABASE_URL missing in ${API_CONTAINER}"
	note "api container=${API_CONTAINER}"
fi

PSQL_DATABASE_URL="$(python3 - "${DATABASE_URL}" <<'PY'
import sys
from urllib.parse import parse_qsl, urlencode, urlparse, urlunparse
u = urlparse(sys.argv[1])
drop = {"default_query_exec_mode", "pgbouncer"}
q = [(k, v) for k, v in parse_qsl(u.query, keep_blank_values=True) if k not in drop]
print(urlunparse((u.scheme, u.netloc, u.path, u.params, urlencode(q), u.fragment)))
PY
)"

docker run --rm \
	-e "DATABASE_URL=${PSQL_DATABASE_URL}" \
	-v "${ROOT}/scripts/ops:/ops:ro" \
	"${POSTGRES_TOOLS_IMAGE}" \
	psql "${PSQL_DATABASE_URL}" -v ON_ERROR_STOP=1 -f /ops/qr_payment_diagnostics.sql

note "done"

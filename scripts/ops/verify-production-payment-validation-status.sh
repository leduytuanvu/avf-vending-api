#!/usr/bin/env bash
# Read-only: verify payment_provider_events.validation_status allows provider_native_verified.
set -Eeuo pipefail

POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"

fail() { echo "verify-payment-validation-status: error: $*" >&2; exit 1; }
note() { echo "verify-payment-validation-status: $*"; }

find_api_container() {
	docker ps --format '{{.Names}}' | grep -E 'api' | head -n1
}

container_env() {
	local container="$1"
	local key="$2"
	docker inspect "${container}" --format '{{range .Config.Env}}{{println .}}{{end}}' \
		| grep -E "^${key}=" | tail -n1 | cut -d= -f2- | tr -d '\r'
}

API_CONTAINER="$(find_api_container)"
[[ -n "${API_CONTAINER}" ]] || fail "api container not found"
DATABASE_URL="$(container_env "${API_CONTAINER}" DATABASE_URL)"
[[ -n "${DATABASE_URL}" ]] || fail "DATABASE_URL missing in ${API_CONTAINER}"
note "api container=${API_CONTAINER}"

constraint_def="$(docker run --rm \
	-e "DATABASE_URL=${DATABASE_URL}" \
	"${POSTGRES_TOOLS_IMAGE}" \
	psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 -Atqc \
	"SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname = 'chk_payment_provider_events_validation_status';")"
[[ -n "${constraint_def}" ]] || fail "constraint chk_payment_provider_events_validation_status not found"

if [[ "${constraint_def}" != *"provider_native_verified"* ]]; then
	fail "constraint missing provider_native_verified: ${constraint_def}"
fi
note "constraint OK (contains provider_native_verified)"

goose_row="$(docker run --rm \
	-e "DATABASE_URL=${DATABASE_URL}" \
	"${POSTGRES_TOOLS_IMAGE}" \
	psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 -Atqc \
	"SELECT version FROM goose_db_version WHERE version_id = 21 OR version = 21 LIMIT 1;" 2>/dev/null || true)"
if [[ "${goose_row}" == "21" ]]; then
	note "goose_db_version includes migration 21"
else
	note "goose_db_version row 21 not found (constraint may still be updated manually)"
fi

note "verification passed"

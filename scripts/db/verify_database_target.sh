#!/usr/bin/env bash
# Verify a single allowlisted target identity before destructive operations.
# Usage:
#   verify_database_target.sh --target-id TARGET-DB-001 [--environment ENV] [--url URL]
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib_db_destroy.sh
source "${SCRIPT_DIR}/lib_db_destroy.sh"

TARGET_ID=""
ENV_OVERRIDE=""
URL_OVERRIDE=""
ALLOW_UNREACHABLE=0

usage() {
	cat <<'EOF'
usage: verify_database_target.sh --target-id TARGET-DB-NNN [options]

Options:
  --target-id ID      Required allowlisted target id
  --environment ENV   Override expected APP_ENV
  --url URL           Override resolved DATABASE_URL (still redacted in logs)
  -h, --help          Show help
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--target-id)
		TARGET_ID="${2:-}"
		shift 2
		;;
	--environment)
		ENV_OVERRIDE="${2:-}"
		shift 2
		;;
	--url)
		URL_OVERRIDE="${2:-}"
		shift 2
		;;
	--allow-unreachable)
		ALLOW_UNREACHABLE=1
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		db_destroy_fail "unknown argument: $1"
		;;
	esac
done

[[ -n "${TARGET_ID}" ]] || db_destroy_fail "--target-id is required"
[[ "${TARGET_ID}" =~ ^TARGET-DB-[0-9]{3}$ ]] || db_destroy_fail "invalid target id format: ${TARGET_ID}"

db_destroy_check_staging_production_alias

URL="${URL_OVERRIDE:-$(db_destroy_resolve_url_for_target "${TARGET_ID}")}"
[[ -n "${URL}" ]] || db_destroy_fail "no DATABASE_URL resolved for ${TARGET_ID}; set env or --url"

PARSED="$(db_destroy_parse_url "${URL}")"
DB_NAME="$(db_destroy_py -c 'import json,sys; print(json.loads(sys.argv[1]).get("database",""))' "${PARSED}")"
HOST="$(db_destroy_py -c 'import json,sys; print(json.loads(sys.argv[1]).get("host",""))' "${PARSED}")"
PORT="$(db_destroy_py -c 'import json,sys; print(json.loads(sys.argv[1]).get("port",""))' "${PARSED}")"

if db_destroy_is_system_database "${DB_NAME}"; then
	db_destroy_fail "refusing system database target: ${DB_NAME}"
fi
if db_destroy_is_out_of_scope_database "${DB_NAME}"; then
	db_destroy_fail "refusing out-of-scope database: ${DB_NAME} (not AVF application DB)"
fi

EXPECTED_ENV="${ENV_OVERRIDE:-$(db_destroy_target_environment "${TARGET_ID}")}"
export APP_ENV="${EXPECTED_ENV}"
case "${EXPECTED_ENV}" in
staging) export PAYMENT_ENV="${PAYMENT_ENV:-sandbox}" ;;
production) export PAYMENT_ENV="${PAYMENT_ENV:-live}" ;;
development | test) export PAYMENT_ENV="${PAYMENT_ENV:-sandbox}" ;;
esac

if ! db_destroy_verify_environment_guard "${TARGET_ID}" "${URL}" 2>/dev/null; then
	case "${EXPECTED_ENV}" in
	development | test)
		db_destroy_warn "environment guard skipped or failed for ${EXPECTED_ENV}; continuing with fingerprint checks"
		;;
	*)
		db_destroy_fail "verify_database_environment.sh failed for ${TARGET_ID}"
		;;
	esac
fi

db_destroy_note "target_id=${TARGET_ID}"
db_destroy_note "environment=${EXPECTED_ENV}"
db_destroy_note "fingerprint=$(db_destroy_fingerprint "${HOST}" "${PORT}" "${DB_NAME}")"
db_destroy_note "database_url=$(db_destroy_mask_url "${URL}")"

REACH="$(db_destroy_probe_live "${URL}" 2>/dev/null || true)"
[[ -n "${REACH}" ]] || REACH="unreachable"
if [[ "${REACH}" != "reachable" ]]; then
	if [[ "${ALLOW_UNREACHABLE}" -eq 1 ]]; then
		db_destroy_warn "target unreachable (allowed): ${TARGET_ID}"
		db_destroy_append_evidence "verify_target" "${TARGET_ID}" "UNREACHABLE" "allowed"
		exit 0
	fi
	db_destroy_fail "target unreachable: ${TARGET_ID} (${REACH})"
fi

INV="$(db_destroy_inventory_database "${URL}")"
db_destroy_note "live_inventory=${INV}"

db_destroy_append_evidence "verify_target" "${TARGET_ID}" "OK" "host=${HOST} db=${DB_NAME}"
db_destroy_note "verify_database_target: OK"

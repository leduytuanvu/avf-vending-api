#!/usr/bin/env bash
# Verify an allowlisted AVF database no longer exists on its cluster.
# Usage:
#   verify_database_absent.sh --target-id TARGET-DB-001 [--observation-wait SECONDS] [--url URL]
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib_db_destroy.sh
source "${SCRIPT_DIR}/lib_db_destroy.sh"

TARGET_ID=""
OBSERVATION_WAIT=0
URL_OVERRIDE=""

usage() {
	cat <<'EOF'
usage: verify_database_absent.sh --target-id TARGET-DB-NNN [options]

Options:
  --target-id ID           Required allowlisted target id
  --observation-wait SEC   Sleep SEC seconds then re-check (default 0)
  --url URL                Cluster maintenance URL source (DATABASE_URL for target)
  -h, --help               Show help
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--target-id)
		TARGET_ID="${2:-}"
		shift 2
		;;
	--observation-wait)
		OBSERVATION_WAIT="${2:-0}"
		shift 2
		;;
	--url)
		URL_OVERRIDE="${2:-}"
		shift 2
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

verify_once() {
	local url="$1"
	local parsed db_name maintenance_url exists
	parsed="$(db_destroy_parse_url "${url}")"
	db_name="$(db_destroy_py -c 'import json,sys; print(json.loads(sys.argv[1]).get("database",""))' "${parsed}")"
	[[ -n "${db_name}" ]] || db_destroy_fail "cannot determine database name"
	db_destroy_validate_db_name "${db_name}"

	maintenance_url="$(db_destroy_maintenance_url "${url}")"
	exists="$(db_destroy_run_psql_at "${maintenance_url}" \
		"SELECT 1 FROM pg_database WHERE datname = '${db_name}' LIMIT 1;" 2>/dev/null || true)"

	if [[ -n "${exists}" ]]; then
		return 1
	fi
	return 0
}

URL="${URL_OVERRIDE:-$(db_destroy_resolve_url_for_target "${TARGET_ID}")}"
[[ -n "${URL}" ]] || db_destroy_fail "no URL for ${TARGET_ID}"

db_destroy_note "verify absence for ${TARGET_ID}"
db_destroy_note "cluster=$(db_destroy_mask_url "$(db_destroy_maintenance_url "${URL}")")"

if ! verify_once "${URL}"; then
	db_destroy_append_evidence "verify_absent" "${TARGET_ID}" "FAIL" "database still exists"
	db_destroy_fail "${TARGET_ID}: ABSENT check FAILED — database still exists"
fi

db_destroy_note "${TARGET_ID}: ABSENT (initial check)"

if [[ "${OBSERVATION_WAIT}" -gt 0 ]]; then
	db_destroy_note "observation wait ${OBSERVATION_WAIT}s"
	sleep "${OBSERVATION_WAIT}"
	if ! verify_once "${URL}"; then
		db_destroy_append_evidence "verify_absent" "${TARGET_ID}" "FAIL" "reappeared after wait"
		db_destroy_fail "${TARGET_ID}: ABSENT check FAILED after observation — database reappeared"
	fi
	db_destroy_note "${TARGET_ID}: ABSENT (after observation)"
fi

db_destroy_append_evidence "verify_absent" "${TARGET_ID}" "OK" "absent"
db_destroy_note "verify_database_absent: OK"

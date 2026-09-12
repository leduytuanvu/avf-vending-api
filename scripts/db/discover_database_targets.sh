#!/usr/bin/env bash
# Discover and fingerprint allowlisted AVF PostgreSQL targets (read-only).
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib_db_destroy.sh
source "${SCRIPT_DIR}/lib_db_destroy.sh"

ENV_FILTER=""
VERIFY_ONLY=0
INVENTORY=0
OUTPUT_PATH=""

usage() {
	cat <<'EOF'
usage: discover_database_targets.sh [options]

Options:
  --environment ENV   Filter targets (development|test|staging|production)
  --verify-only       Probe reachability only
  --inventory         Include live metadata when reachable
  --output PATH       Write JSON registry (default: .db-destroy-evidence/discovery.json)
  -h, --help          Show help
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--environment)
		ENV_FILTER="${2:-}"
		shift 2
		;;
	--verify-only)
		VERIFY_ONLY=1
		shift
		;;
	--inventory)
		INVENTORY=1
		shift
		;;
	--output)
		OUTPUT_PATH="${2:-}"
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

[[ -f "${DB_DESTROY_REGISTRY}" ]] || db_destroy_fail "registry not found: ${DB_DESTROY_REGISTRY}"
OUTPUT_PATH="${OUTPUT_PATH:-${DB_DESTROY_EVIDENCE_DIR}/discovery.json}"
mkdir -p "$(dirname "${OUTPUT_PATH}")"

db_destroy_check_staging_production_alias

TARGET_IDS=(
	TARGET-DB-001
	TARGET-DB-002
	TARGET-DB-003
	TARGET-DB-004
	TARGET-DB-005
	TARGET-DB-006
	TARGET-DB-007
	TARGET-DB-008
)

is_config_only() {
	case "$1" in
	TARGET-DB-004 | TARGET-DB-006 | TARGET-DB-007 | TARGET-DB-008) return 0 ;;
	esac
	return 1
}

ROWS_FILE="$(mktemp)"
ALIASES_FILE="$(mktemp)"
trap 'rm -f "${ROWS_FILE}" "${ALIASES_FILE}"' EXIT

declare -A SEEN_FP
GIT_SHA="$(git -C "${DB_DESTROY_REPO_ROOT}" rev-parse HEAD 2>/dev/null || echo unknown)"
GENERATED_AT="$(db_destroy_utc)"

for tid in "${TARGET_IDS[@]}"; do
	env="$(db_destroy_target_environment "${tid}")"
	if [[ -n "${ENV_FILTER}" && "${env}" != "${ENV_FILTER}" ]]; then
		continue
	fi

	url="$(db_destroy_resolve_url_for_target "${tid}")"
	parsed="{}"
	db_name=""
	host=""
	port=""
	user=""
	sslmode=""
	masked=""
	fp=""
	reach="CONFIG_REFERENCE_ONLY"
	status="CONFIG_REFERENCE_ONLY"
	live_inv=""

	if [[ -n "${url}" ]]; then
		parsed="$(db_destroy_parse_url "${url}")"
		db_name="$(db_destroy_py -c 'import json,sys; print(json.loads(sys.argv[1]).get("database",""))' "${parsed}")"
		host="$(db_destroy_py -c 'import json,sys; print(json.loads(sys.argv[1]).get("host",""))' "${parsed}")"
		port="$(db_destroy_py -c 'import json,sys; print(json.loads(sys.argv[1]).get("port",""))' "${parsed}")"
		user="$(db_destroy_py -c 'import json,sys; print(json.loads(sys.argv[1]).get("username",""))' "${parsed}")"
		sslmode="$(db_destroy_py -c 'import json,sys; print(json.loads(sys.argv[1]).get("sslmode",""))' "${parsed}")"
		masked="$(db_destroy_mask_url "${url}")"
		if [[ -n "${host}" && -n "${db_name}" ]]; then
			fp="$(db_destroy_fingerprint "${host}" "${port}" "${db_name}")"
		fi
		if is_config_only "${tid}"; then
			reach="CONFIG_REFERENCE_ONLY"
			status="CONFIG_REFERENCE_ONLY"
		else
			reach="$(db_destroy_probe_live "${url}" 2>/dev/null || true)"
			[[ -n "${reach}" ]] || reach="unreachable"
			if [[ "${reach}" == "reachable" ]]; then
				status="FOUND_AND_TARGETED"
				if [[ "${INVENTORY}" -eq 1 ]]; then
					live_inv="$(db_destroy_inventory_database "${url}")"
				fi
			else
				status="UNREACHABLE_NEEDS_MANUAL_VERIFICATION"
			fi
		fi
	fi

	if [[ -n "${fp}" ]]; then
		if [[ -n "${SEEN_FP[${fp}]:-}" ]]; then
			status="FOUND_ALIAS_OF_OTHER_TARGET"
			echo "{\"target_id\":\"${tid}\",\"alias_of\":\"${SEEN_FP[${fp}]}\"}" >>"${ALIASES_FILE}"
		else
			SEEN_FP["${fp}"]="${tid}"
		fi
	fi

	db_destroy_py - "${tid}" "${env}" "${db_name}" "${host}" "${port}" "${user}" "${sslmode}" "${fp}" "${reach}" "${status}" "${masked}" "${live_inv}" >>"${ROWS_FILE}" <<'PY'
import json, sys
tid, env, db, host, port, user, ssl, fp, reach, status, masked, live_inv = sys.argv[1:13]
row = {
    "target_id": tid,
    "environment": env,
    "database": db,
    "host": host,
    "port": port,
    "username": user,
    "sslmode": ssl,
    "fingerprint": fp,
    "reachability": reach,
    "status": status,
    "masked_url": masked,
}
if live_inv:
    try:
        row["live_inventory"] = json.loads(live_inv)
    except json.JSONDecodeError:
        pass
print(json.dumps(row))
PY
done

db_destroy_py - "${OUTPUT_PATH}" "${GENERATED_AT}" "${GIT_SHA}" "${ENV_FILTER}" "${ROWS_FILE}" "${ALIASES_FILE}" <<'PY'
import json, sys
from pathlib import Path

out, generated_at, git_sha, env_filter, rows_file, aliases_file = sys.argv[1:7]
rows = [json.loads(line) for line in Path(rows_file).read_text(encoding="utf-8").splitlines() if line.strip()]
aliases = [json.loads(line) for line in Path(aliases_file).read_text(encoding="utf-8").splitlines() if line.strip()]
report = {
    "generated_at": generated_at,
    "git_sha": git_sha,
    "environment_filter": env_filter or None,
    "targets": rows,
    "aliases": aliases,
    "system_databases": ["postgres", "template0", "template1"],
    "out_of_scope_databases": ["temporal", "temporal_visibility"],
}
Path(out).write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
print(json.dumps(report, indent=2))
PY

db_destroy_note "discovery written to ${OUTPUT_PATH}"
db_destroy_append_evidence "discover" "ALL" "OK" "output=${OUTPUT_PATH}"

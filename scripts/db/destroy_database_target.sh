#!/usr/bin/env bash
# Destroy a single allowlisted AVF PostgreSQL application database.
# Usage:
#   destroy_database_target.sh --target-id TARGET-DB-002 --dry-run
#   destroy_database_target.sh --target-id TARGET-DB-005 --confirm-production-destroy AVF-VENDING-PROD-DB-DESTROY
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib_db_destroy.sh
source "${SCRIPT_DIR}/lib_db_destroy.sh"

TARGET_ID=""
DRY_RUN=0
VERIFY_ONLY=0
INVENTORY=0
URL_OVERRIDE=""
SKIP_BACKUP=0
CONFIRM_PROD=""
CONFIRM_STAGING=""
BACKUP_PATH=""
MANIFEST_PATH=""
REMOVE_VOLUME=0
BREAK_GLASS_PRODUCTION_DROP=0

usage() {
	cat <<'EOF'
usage: destroy_database_target.sh --target-id TARGET-DB-NNN [options]

Options:
  --target-id ID                    Required allowlisted target
  --dry-run                         Show planned actions only
  --verify-only                     Verify identity only (no drop)
  --inventory                       Print live inventory then exit
  --url URL                         Override connection URL
  --skip-backup                     Allowed only for ephemeral targets (002, 008)
  --backup-path PATH                Backup file for verify_backup_gate.sh
  --manifest PATH                 Backup manifest JSON
  --confirm-production-destroy TOK  Required for TARGET-DB-005/006/007
  --confirm-staging-destroy TOK     Required for TARGET-DB-003/004 (default: AVF-VENDING-STAGING-DB-DESTROY)
  --break-glass-production-drop     DBA break-glass only; required to DROP production targets (005/006/007)
  --remove-volume                   After drop, remove compose postgres volume when applicable
  -h, --help                        Show help
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--target-id)
		TARGET_ID="${2:-}"
		shift 2
		;;
	--dry-run)
		DRY_RUN=1
		shift
		;;
	--verify-only)
		VERIFY_ONLY=1
		shift
		;;
	--inventory)
		INVENTORY=1
		shift
		;;
	--url)
		URL_OVERRIDE="${2:-}"
		shift 2
		;;
	--skip-backup)
		SKIP_BACKUP=1
		shift
		;;
	--backup-path)
		BACKUP_PATH="${2:-}"
		shift 2
		;;
	--manifest)
		MANIFEST_PATH="${2:-}"
		shift 2
		;;
	--confirm-production-destroy)
		CONFIRM_PROD="${2:-}"
		shift 2
		;;
	--confirm-staging-destroy)
		CONFIRM_STAGING="${2:-}"
		shift 2
		;;
	--remove-volume)
		REMOVE_VOLUME=1
		shift
		;;
	--break-glass-production-drop)
		BREAK_GLASS_PRODUCTION_DROP=1
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
[[ "${TARGET_ID}" =~ ^TARGET-DB-[0-9]{3}$ ]] || db_destroy_fail "invalid target id: ${TARGET_ID}"

db_destroy_check_staging_production_alias

URL="${URL_OVERRIDE:-$(db_destroy_resolve_url_for_target "${TARGET_ID}")}"
[[ -n "${URL}" ]] || db_destroy_fail "no URL resolved for ${TARGET_ID}"

VERIFY_ARGS=(--target-id "${TARGET_ID}" --url "${URL}")
if [[ "${DRY_RUN}" -eq 1 ]]; then
	VERIFY_ARGS+=(--allow-unreachable)
fi
bash "${SCRIPT_DIR}/verify_database_target.sh" "${VERIFY_ARGS[@]}"

PARSED="$(db_destroy_parse_url "${URL}")"
DB_NAME="$(db_destroy_py -c 'import json,sys; print(json.loads(sys.argv[1]).get("database",""))' "${PARSED}")"
[[ -n "${DB_NAME}" ]] || db_destroy_fail "cannot determine database name"
db_destroy_validate_db_name "${DB_NAME}"

if db_destroy_is_system_database "${DB_NAME}"; then
	db_destroy_fail "refusing to drop system database: ${DB_NAME}"
fi
if db_destroy_is_out_of_scope_database "${DB_NAME}"; then
	db_destroy_fail "refusing out-of-scope database: ${DB_NAME}"
fi

if [[ "${INVENTORY}" -eq 1 ]]; then
	db_destroy_note "inventory=$(db_destroy_inventory_database "${URL}")"
	exit 0
fi

if [[ "${VERIFY_ONLY}" -eq 1 ]]; then
	db_destroy_note "verify-only complete for ${TARGET_ID}"
	exit 0
fi

ENV="$(db_destroy_target_environment "${TARGET_ID}")"

case "${TARGET_ID}" in
TARGET-DB-005 | TARGET-DB-006 | TARGET-DB-007)
	[[ "${BREAK_GLASS_PRODUCTION_DROP}" -eq 1 ]] || \
		db_destroy_fail "production DROP DATABASE is prohibited in default tooling; use run-environment-data-wipe.sh for scoped TRUNCATE wipe, or pass --break-glass-production-drop for DBA break-glass only"
	[[ "${CONFIRM_PROD}" == "AVF-VENDING-PROD-DB-DESTROY" ]] || \
		db_destroy_fail "production destroy requires --confirm-production-destroy AVF-VENDING-PROD-DB-DESTROY"
	;;
TARGET-DB-003 | TARGET-DB-004)
	CONFIRM_STAGING="${CONFIRM_STAGING:-AVF-VENDING-STAGING-DB-DESTROY}"
	[[ "${CONFIRM_STAGING}" == "AVF-VENDING-STAGING-DB-DESTROY" ]] || \
		db_destroy_fail "staging destroy requires --confirm-staging-destroy AVF-VENDING-STAGING-DB-DESTROY"
	;;
esac

EPHEMERAL=0
case "${TARGET_ID}" in
TARGET-DB-002 | TARGET-DB-008) EPHEMERAL=1 ;;
esac

if [[ "${DRY_RUN}" -eq 1 ]]; then
	db_destroy_warn "backup gate skipped for dry-run ${TARGET_ID}"
elif [[ "${SKIP_BACKUP}" -eq 1 ]]; then
	case "${TARGET_ID}" in
	TARGET-DB-001 | TARGET-DB-002 | TARGET-DB-008) ;;
	*)
		db_destroy_fail "--skip-backup only allowed for local/ephemeral targets (001, 002, 008)"
		;;
	esac
	db_destroy_warn "backup skipped for ${TARGET_ID}"
else
	if [[ "${EPHEMERAL}" -eq 0 ]]; then
		GATE_ARGS=(--target-id "${TARGET_ID}")
		[[ -n "${BACKUP_PATH}" ]] && GATE_ARGS+=(--backup-path "${BACKUP_PATH}")
		[[ -n "${MANIFEST_PATH}" ]] && GATE_ARGS+=(--manifest "${MANIFEST_PATH}")
		if [[ -z "${BACKUP_PATH}" && -z "${MANIFEST_PATH}" ]]; then
			DEFAULT_MANIFEST="${DB_DESTROY_EVIDENCE_DIR}/backups/${TARGET_ID}-manifest.json"
			if [[ -f "${DEFAULT_MANIFEST}" ]]; then
				GATE_ARGS+=(--manifest "${DEFAULT_MANIFEST}")
			else
				db_destroy_fail "BACKUP FAILURE => ABORT: provide --backup-path or --manifest for ${TARGET_ID}"
			fi
		fi
		bash "${SCRIPT_DIR}/verify_backup_gate.sh" "${GATE_ARGS[@]}"
	fi
fi

bash "${SCRIPT_DIR}/quiesce_writers.sh" --environment "${ENV}" $([[ "${DRY_RUN}" -eq 1 ]] && echo --dry-run)

MAINT="$(db_destroy_maintenance_url "${URL}")"
DROP_SQL="
SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '${DB_NAME}' AND pid <> pg_backend_pid();
ALTER DATABASE ${DB_NAME} CONNECTION LIMIT 0;
DROP DATABASE IF EXISTS ${DB_NAME};
"

db_destroy_note "planned destruction for ${TARGET_ID}"
db_destroy_note "maintenance=$(db_destroy_mask_url "${MAINT}")"
db_destroy_note "drop_database=${DB_NAME}"

if [[ "${DRY_RUN}" -eq 1 ]]; then
	db_destroy_note "[dry-run] would execute DROP DATABASE on ${DB_NAME}"
	if [[ "${REMOVE_VOLUME}" -eq 1 ]]; then
		db_destroy_note "[dry-run] would remove compose volume per target registry"
	fi
	db_destroy_append_evidence "destroy" "${TARGET_ID}" "DRY_RUN" "db=${DB_NAME}"
	exit 0
fi

# Drain connections and drop
db_destroy_run_psql_at "${MAINT}" \
	"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '${DB_NAME}' AND pid <> pg_backend_pid();" \
	|| db_destroy_warn "terminate backends returned non-zero (may be none)"

ACTIVE="$(db_destroy_run_psql_at "${MAINT}" \
	"SELECT count(*)::int FROM pg_stat_activity WHERE datname = '${DB_NAME}';" 2>/dev/null || echo "1")"
if [[ "${ACTIVE}" != "0" ]]; then
	db_destroy_fail "active sessions remain on ${DB_NAME}: ${ACTIVE}"
fi

db_destroy_run_psql_at "${MAINT}" "DROP DATABASE IF EXISTS ${DB_NAME};"

bash "${SCRIPT_DIR}/verify_database_absent.sh" --target-id "${TARGET_ID}" --url "${URL}"

if [[ "${REMOVE_VOLUME}" -eq 1 ]]; then
	case "${TARGET_ID}" in
	TARGET-DB-001 | TARGET-DB-002)
		docker compose -f "${DB_DESTROY_REPO_ROOT}/deployments/docker/docker-compose.yml" down 2>/dev/null || true
		docker volume rm avf-vending-local_postgres_data avf-vending-api_postgres_data docker_postgres_data 2>/dev/null || true
		;;
	TARGET-DB-003)
		docker volume rm avf-vending-staging_postgres_data 2>/dev/null || true
		;;
	TARGET-DB-007)
		docker volume rm avf-vending-prod_postgres_data 2>/dev/null || true
		;;
	esac
	db_destroy_note "volume removal attempted for ${TARGET_ID}"
fi

db_destroy_append_evidence "destroy" "${TARGET_ID}" "OK" "dropped=${DB_NAME}"
db_destroy_note "destroy_database_target: OK — ${TARGET_ID} ABSENT"

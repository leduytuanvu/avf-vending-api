#!/usr/bin/env bash
# Orchestrate destruction of all discovered allowlisted AVF databases in safe order.
# Usage:
#   destroy_all_avf_databases.sh --discovery PATH --dry-run
#   destroy_all_avf_databases.sh --discovery PATH --execute --confirm-destroy-all AVF-VENDING-DESTROY-ALL
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib_db_destroy.sh
source "${SCRIPT_DIR}/lib_db_destroy.sh"

DISCOVERY_PATH=""
DRY_RUN=0
EXECUTE=0
CONFIRM_ALL=""
SKIP_BACKUP_EPHEMERAL=1

usage() {
	cat <<'EOF'
usage: destroy_all_avf_databases.sh [options]

Options:
  --discovery PATH              discovery.json from discover_database_targets.sh
  --dry-run                     Plan only
  --execute                     Run destruction (requires --confirm-destroy-all)
  --confirm-destroy-all TOKEN   Must be AVF-VENDING-DESTROY-ALL
  --no-skip-ephemeral-backup      Require backup even for ephemeral (default: skip)
  -h, --help                    Show help
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--discovery)
		DISCOVERY_PATH="${2:-}"
		shift 2
		;;
	--dry-run)
		DRY_RUN=1
		shift
		;;
	--execute)
		EXECUTE=1
		shift
		;;
	--confirm-destroy-all)
		CONFIRM_ALL="${2:-}"
		shift 2
		;;
	--no-skip-ephemeral-backup)
		SKIP_BACKUP_EPHEMERAL=0
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

[[ -n "${DISCOVERY_PATH}" ]] || DISCOVERY_PATH="${DB_DESTROY_EVIDENCE_DIR}/discovery.json"
[[ -f "${DISCOVERY_PATH}" ]] || db_destroy_fail "discovery file not found: ${DISCOVERY_PATH} (run discover_database_targets.sh first)"

if [[ "${EXECUTE}" -eq 1 ]]; then
	[[ "${CONFIRM_ALL}" == "AVF-VENDING-DESTROY-ALL" ]] || \
		db_destroy_fail "execute requires --confirm-destroy-all AVF-VENDING-DESTROY-ALL"
	[[ "${DRY_RUN}" -eq 1 ]] && db_destroy_fail "cannot use --execute and --dry-run together"
fi

ORDER=(TARGET-DB-008 TARGET-DB-002 TARGET-DB-001 TARGET-DB-003 TARGET-DB-004 TARGET-DB-007 TARGET-DB-005 TARGET-DB-006)

db_destroy_py - "${DISCOVERY_PATH}" "${ORDER[@]}" <<'PY' >"${DB_DESTROY_EVIDENCE_DIR}/destroy-plan.txt"
import json, sys

path = sys.argv[1]
order = sys.argv[2:]
with open(path, encoding="utf-8") as f:
    disc = json.load(f)

by_id = {t["target_id"]: t for t in disc["targets"]}
for tid in order:
    t = by_id.get(tid)
    if not t:
        continue
    if t.get("status") == "CONFIG_REFERENCE_ONLY" and t.get("reachability") != "reachable":
        print(f"SKIP {tid} config_reference_only")
        continue
    if t.get("status") == "FOUND_ALIAS_OF_OTHER_TARGET":
        print(f"SKIP {tid} alias")
        continue
    print(f"PLAN {tid} {t.get('reachability')} {t.get('database')}")
PY

cat "${DB_DESTROY_EVIDENCE_DIR}/destroy-plan.txt"

for tid in "${ORDER[@]}"; do
	STATUS_LINE="$(grep "^PLAN ${tid} " "${DB_DESTROY_EVIDENCE_DIR}/destroy-plan.txt" || true)"
	[[ -n "${STATUS_LINE}" ]] || continue

	ARGS=(--target-id "${tid}")
	if [[ "${DRY_RUN}" -eq 1 || "${EXECUTE}" -eq 0 ]]; then
		ARGS+=(--dry-run)
	fi
	case "${tid}" in
	TARGET-DB-002 | TARGET-DB-008)
		[[ "${SKIP_BACKUP_EPHEMERAL}" -eq 1 ]] && ARGS+=(--skip-backup)
		;;
	TARGET-DB-005 | TARGET-DB-006 | TARGET-DB-007)
		ARGS+=(--confirm-production-destroy AVF-VENDING-PROD-DB-DESTROY)
		;;
	TARGET-DB-003 | TARGET-DB-004)
		ARGS+=(--confirm-staging-destroy AVF-VENDING-STAGING-DB-DESTROY)
		;;
	esac

	if [[ "${EXECUTE}" -eq 1 ]]; then
		db_destroy_note "executing ${tid}"
		bash "${SCRIPT_DIR}/destroy_database_target.sh" "${ARGS[@]}"
	else
		db_destroy_note "plan ${tid}"
		bash "${SCRIPT_DIR}/destroy_database_target.sh" "${ARGS[@]}"
	fi
done

bash "${SCRIPT_DIR}/emit_destruction_evidence.sh" --discovery "${DISCOVERY_PATH}"
db_destroy_note "destroy_all_avf_databases: complete"

#!/usr/bin/env bash
# Verify a final backup exists before destroying a non-ephemeral target.
# Usage:
#   verify_backup_gate.sh --target-id TARGET-DB-005 [--backup-path PATH] [--manifest PATH]
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib_db_destroy.sh
source "${SCRIPT_DIR}/lib_db_destroy.sh"

TARGET_ID=""
BACKUP_PATH=""
MANIFEST_PATH=""

usage() {
	cat <<'EOF'
usage: verify_backup_gate.sh --target-id TARGET-DB-NNN [options]

Options:
  --target-id ID       Required target id
  --backup-path PATH   Backup file to verify (required for non-ephemeral unless manifest set)
  --manifest PATH      JSON manifest with backup_path and sha256
  -h, --help           Show help
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--target-id)
		TARGET_ID="${2:-}"
		shift 2
		;;
	--backup-path)
		BACKUP_PATH="${2:-}"
		shift 2
		;;
	--manifest)
		MANIFEST_PATH="${2:-}"
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

EPHEMERAL=0
case "${TARGET_ID}" in
TARGET-DB-002 | TARGET-DB-008) EPHEMERAL=1 ;;
esac

if [[ "${EPHEMERAL}" -eq 1 ]]; then
	db_destroy_note "backup gate skipped for ephemeral target ${TARGET_ID}"
	db_destroy_append_evidence "backup_gate" "${TARGET_ID}" "SKIP" "ephemeral"
	exit 0
fi

if [[ -n "${MANIFEST_PATH}" ]]; then
	[[ -f "${MANIFEST_PATH}" ]] || db_destroy_fail "manifest not found: ${MANIFEST_PATH}"
	BACKUP_PATH="$(db_destroy_py -c 'import json,sys; print(json.load(open(sys.argv[1]))["backup_path"])' "${MANIFEST_PATH}")"
	EXPECTED_SHA="$(db_destroy_py -c 'import json,sys; print(json.load(open(sys.argv[1])).get("sha256",""))' "${MANIFEST_PATH}")"
else
	EXPECTED_SHA=""
fi

[[ -n "${BACKUP_PATH}" ]] || db_destroy_fail "BACKUP FAILURE => ABORT: set --backup-path or --manifest for ${TARGET_ID}"
[[ -f "${BACKUP_PATH}" ]] || db_destroy_fail "BACKUP FAILURE => ABORT: backup file missing: ${BACKUP_PATH}"
[[ -s "${BACKUP_PATH}" ]] || db_destroy_fail "BACKUP FAILURE => ABORT: backup file empty: ${BACKUP_PATH}"

ACTUAL_SHA="$(sha256sum "${BACKUP_PATH}" | awk '{print $1}')"
if [[ -n "${EXPECTED_SHA}" && "${EXPECTED_SHA}" != "${ACTUAL_SHA}" ]]; then
	db_destroy_fail "BACKUP FAILURE => ABORT: sha256 mismatch"
fi

# Readability probe: pg_restore --list or gzip -t
if [[ "${BACKUP_PATH}" == *.gz ]]; then
	gzip -t "${BACKUP_PATH}" || db_destroy_fail "BACKUP FAILURE => ABORT: gzip integrity check failed"
elif command -v pg_restore >/dev/null 2>&1; then
	pg_restore --list "${BACKUP_PATH}" >/dev/null 2>&1 || db_destroy_warn "pg_restore --list failed; file may still be valid custom/sql dump"
fi

MANIFEST_OUT="${DB_DESTROY_EVIDENCE_DIR}/backups/${TARGET_ID}-manifest.json"
mkdir -p "$(dirname "${MANIFEST_OUT}")"
db_destroy_py - "${TARGET_ID}" "${BACKUP_PATH}" "${ACTUAL_SHA}" "${MANIFEST_OUT}" <<'PY'
import json, sys
from datetime import datetime, timezone

tid, path, sha, out = sys.argv[1:5]
doc = {
    "target_id": tid,
    "backup_path": path,
    "sha256": sha,
    "verified_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
}
with open(out, "w", encoding="utf-8") as f:
    json.dump(doc, f, indent=2)
    f.write("\n")
print(out)
PY

db_destroy_note "backup verified for ${TARGET_ID}: ${BACKUP_PATH}"
db_destroy_append_evidence "backup_gate" "${TARGET_ID}" "OK" "sha256=${ACTUAL_SHA}"
db_destroy_note "verify_backup_gate: OK"

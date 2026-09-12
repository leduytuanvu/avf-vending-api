#!/usr/bin/env bash
# Emit final destruction evidence report from discovery + jsonl log.
# Usage: emit_destruction_evidence.sh [--discovery PATH]
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib_db_destroy.sh
source "${SCRIPT_DIR}/lib_db_destroy.sh"

DISCOVERY_PATH="${DB_DESTROY_EVIDENCE_DIR}/discovery.json"
while [[ $# -gt 0 ]]; do
	case "$1" in
	--discovery)
		DISCOVERY_PATH="${2:-}"
		shift 2
		;;
	-h | --help)
		echo "usage: emit_destruction_evidence.sh [--discovery PATH]"
		exit 0
		;;
	*)
		db_destroy_fail "unknown argument: $1"
		;;
	esac
done

REPORT="${DB_DESTROY_EVIDENCE_DIR}/final-evidence-report.txt"
LOG="${DB_DESTROY_EVIDENCE_LOG:-${DB_DESTROY_EVIDENCE_DIR}/destruction.jsonl}"

{
	echo "AVF Vending PostgreSQL Database Destruction Evidence"
	echo "generated_at=$(db_destroy_utc)"
	echo "git_sha=$(git -C "${DB_DESTROY_REPO_ROOT}" rev-parse HEAD 2>/dev/null || echo unknown)"
	echo ""
} >"${REPORT}"

if [[ -f "${DISCOVERY_PATH}" ]]; then
	db_destroy_py - "${DISCOVERY_PATH}" "${LOG}" >>"${REPORT}" <<'PY'
import json, sys
from pathlib import Path

disc_path, log_path = sys.argv[1:3]
with open(disc_path, encoding="utf-8") as f:
    disc = json.load(f)

absent = {}
if Path(log_path).is_file():
    for line in Path(log_path).read_text(encoding="utf-8").splitlines():
        if not line.strip():
            continue
        row = json.loads(line)
        if row.get("phase") == "verify_absent" and row.get("status") == "OK":
            absent[row["target_id"]] = row.get("ts")

failures = []
for t in disc.get("targets", []):
    tid = t["target_id"]
    if tid in absent:
        status = "ABSENT"
    elif t.get("status") in ("CONFIG_REFERENCE_ONLY", "NOT_PERSISTENT"):
        status = t.get("status")
    elif t.get("reachability") == "unreachable":
        status = "UNREACHABLE_NEEDS_MANUAL_VERIFICATION"
    else:
        status = "UNKNOWN"
        failures.append(tid)
    print(f"{tid}: {status} env={t.get('environment')} db={t.get('database')} reachability={t.get('reachability')}")

print("")
if failures:
    print(f"FINAL: FAILURE ({len(failures)} targets not verified absent)")
else:
    print("FINAL: SUCCESS (all reachable targets verified absent or out of scope)")
PY
else
	echo "discovery: MISSING" >>"${REPORT}"
	echo "FINAL: FAILURE" >>"${REPORT}"
fi

cat "${REPORT}"
db_destroy_note "evidence report: ${REPORT}"

#!/usr/bin/env bash
# Summarize backup gate status per target for destruction evidence.
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib_db_destroy.sh
source "${SCRIPT_DIR}/lib_db_destroy.sh"

OUT="${DB_DESTROY_EVIDENCE_DIR}/backup-gate-summary.json"
mkdir -p "${DB_DESTROY_EVIDENCE_DIR}"

db_destroy_py - "${OUT}" "$(db_destroy_utc)" <<'PY'
import json, sys
from pathlib import Path

out, ts = sys.argv[1:3]

rows = [
    {"target_id": "TARGET-DB-001", "status": "SKIP_ALLOWED", "reason": "local development; backup optional with --skip-backup"},
    {"target_id": "TARGET-DB-002", "status": "SKIP_ALLOWED", "reason": "ephemeral integration test database"},
    {"target_id": "TARGET-DB-003", "status": "BLOCKED", "reason": "requires STAGING_DATABASE_URL on staging VPS; run backup_managed_postgres.sh or pg_dump before destroy"},
    {"target_id": "TARGET-DB-004", "status": "BLOCKED", "reason": "managed staging secret not available on this host"},
    {"target_id": "TARGET-DB-005", "status": "BLOCKED", "reason": "requires PRODUCTION_DATABASE_URL; run deployments/prod/shared/scripts/backup_managed_postgres.sh execute"},
    {"target_id": "TARGET-DB-006", "status": "BLOCKED", "reason": "dedup against TARGET-DB-005 before any drop"},
    {"target_id": "TARGET-DB-007", "status": "BLOCKED", "reason": "legacy prod compose; verify live before backup_postgres.sh"},
    {"target_id": "TARGET-DB-008", "status": "SKIP_ALLOWED", "reason": "ephemeral doc-only name"},
]
doc = {"recorded_at": ts, "targets": rows}
Path(out).write_text(json.dumps(doc, indent=2) + "\n", encoding="utf-8")
print(out)
PY

db_destroy_note "backup gate summary: ${OUT}"

#!/usr/bin/env bash
# Record repopulation blocker checklist status for destruction evidence.
# Usage: record_repopulation_blockers.sh [--note TEXT]
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib_db_destroy.sh
source "${SCRIPT_DIR}/lib_db_destroy.sh"

NOTE="${1:-}"
OUT="${DB_DESTROY_EVIDENCE_DIR}/repopulation-blockers.json"

mkdir -p "${DB_DESTROY_EVIDENCE_DIR}"
db_destroy_py - "${OUT}" "${NOTE}" "$(db_destroy_utc)" <<'PY'
import json, sys
from pathlib import Path

out, note, ts = sys.argv[1:4]
doc = {
    "recorded_at": ts,
    "note": note,
    "blockers": {
        "github_deploy_workflows_held": "manual — disable production-migrate-self-hosted.yml / deploy workflows before prod/staging destroy",
        "machine_jwt_revoked_or_fleet_reset": "manual — revoke machine credentials or factory-reset devices before API restart",
        "nats_emqx_consumers_stopped": "manual — stop mqtt-ingest and block broker replay before API restart",
        "no_goose_or_dev_migrate_after_destroy": "enforced — do not run make dev-migrate or deploy_staging.sh migrate after destruction",
        "postgres_not_restarted_with_init": "enforced — dev-db-destroy uses compose down + volume rm without postgres up",
    },
}
Path(out).write_text(json.dumps(doc, indent=2) + "\n", encoding="utf-8")
print(out)
PY

db_destroy_append_evidence "repopulation_blockers" "ALL" "RECORDED" "${NOTE}"
db_destroy_note "repopulation blockers recorded: ${OUT}"

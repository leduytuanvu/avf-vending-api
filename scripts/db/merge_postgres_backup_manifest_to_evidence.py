#!/usr/bin/env python3
"""
Map a backup_postgres.sh manifest JSON (same directory as the backup) toward backup-evidence-v1.
Output always has evidence_draft=true until an operator completes restore drill, schema_validation, etc.

This does not run a backup.
"""
from __future__ import annotations

import argparse
import json
import os
import re
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any


def ts_to_iso(ts: str) -> str:
    # backup_postgres uses %Y%m%dT%H%M%SZ
    if re.match(r"^\d{8}T\d{6}Z$", ts):
        dt = datetime.strptime(ts, "%Y%m%dT%H%M%SZ").replace(tzinfo=timezone.utc)
        return dt.isoformat().replace("+00:00", "Z")
    return ts


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("manifest", type=Path, help="Path to *.manifest.json from backup_postgres.sh")
    p.add_argument(
        "-o",
        "--output",
        type=Path,
        help="Write JSON here (default: stdout)",
    )
    p.add_argument(
        "--expires-in-hours",
        type=int,
        default=72,
        help="expires_at_utc = now + this many hours",
    )
    p.add_argument(
        "--no-restore-drill",
        action="store_true",
        help="Set restore_drill_required false (drill not required for this change).",
    )
    args = p.parse_args()

    m = json.loads(args.manifest.read_text(encoding="utf-8"))
    dbn = m.get("database_name") or ""
    size = int(m.get("file_size_bytes", 0) or 0)
    sha = (m.get("sha256_value") or "").strip()
    raw_ts = (m.get("timestamp") or "").strip()
    if raw_ts and re.match(r"^\d{8}T\d{6}Z$", raw_ts):
        t_iso = ts_to_iso(raw_ts)
    else:
        t_iso = datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")

    repo = (os.environ.get("GITHUB_REPOSITORY") or "").strip() or "YOUR_ORG/YOUR_REPO"
    art = (os.environ.get("BACKUP_ARTIFACT_ID") or "").strip()
    loc = (os.environ.get("BACKUP_LOCATION_REDACTED") or "").strip()
    c_run = (os.environ.get("CREATED_BY_WORKFLOW_RUN_ID") or "").strip()
    operator = (os.environ.get("EVIDENCE_OPERATOR") or "").strip()
    r_req = not args.no_restore_drill
    if (os.environ.get("RESTORE_DRILL_REQUIRED") or "").strip().lower() in ("0", "false", "no"):
        r_req = False

    now = datetime.now(timezone.utc)
    exp = (now + timedelta(hours=max(1, args.expires_in_hours))).isoformat().replace("+00:00", "Z")

    body: dict[str, Any] = {
        "schema_version": "backup-evidence-v1",
        "evidence_type": "production-db-backup",
        "repository": repo,
        "source_environment": "production",
        "database_name": dbn,
        "logical_db_id": (os.environ.get("LOGICAL_DB_ID") or "").strip(),
        "backup_started_at_utc": t_iso,
        "backup_completed_at_utc": t_iso,
        "backup_artifact_id": art,
        "backup_location_redacted": loc,
        "backup_size_bytes": size if size > 0 else 0,
        "backup_sha256": f"sha256:{sha}" if sha and not sha.lower().startswith("sha256:") else sha,
        "restore_drill_required": r_req,
        "restore_drill_completed_at_utc": None,
        "restore_drill_result": "skipped" if not r_req else "fail",
        "restore_target": "not_run" if not r_req else "temp-db",
        "schema_validation_result": "fail",
        "created_by_workflow_run_id": c_run,
        "operator": operator,
        "expires_at_utc": exp,
    }
    if body["backup_size_bytes"] < 1:
        body["backup_size_bytes"] = 0
    body["evidence_draft"] = True
    body["_note"] = (
        "merge_postgres_backup_manifest: fill restore drill, schema_validation_result, pass validate_backup_evidence.py, "
        "then set evidence_draft to false and upload as artifact production-db-backup-evidence (backup-evidence/backup-evidence.json)"
    )
    if not body.get("database_name") and not body.get("logical_db_id"):
        body["database_name"] = (m.get("database_name") or "").strip() or "unknown"

    out = json.dumps(body, indent=2) + "\n"
    if args.output:
        args.output.write_text(out, encoding="utf-8")
    else:
        sys.stdout.write(out)
    print(
        "merge_postgres_backup_manifest: output is a draft (evidence_draft=true) and will not pass production validation",
        file=sys.stderr,
    )


if __name__ == "__main__":
    main()

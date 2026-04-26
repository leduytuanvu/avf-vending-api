#!/usr/bin/env python3
"""
Emit an incomplete backup evidence JSON document for operators to copy and fill in.
This file will FAIL validate_backup_evidence.py until all fields are completed with real values.
Never submit evidence_draft=true to production.
"""
from __future__ import annotations

import argparse
import json
import sys
from datetime import datetime, timezone

TEMPLATE_NOTE = (
    "INCOMPLETE: replace every null and empty string, set evidence_draft to false, "
    "and run: python3 scripts/ci/validate_backup_evidence.py --for-production-migration <file>"
)


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument(
        "-o",
        "--output",
        type=argparse.FileType("w", encoding="utf-8"),
        default="-",
        help="Write JSON here (default: stdout).",
    )
    args = p.parse_args()
    now = datetime.now(timezone.utc)
    # Far future so template accidentally submitted still fails on evidence_draft
    exp = now.isoformat()
    body = {
        "schema_version": "backup-evidence-v1",
        "evidence_type": "production-db-backup",
        "repository": "YOUR_ORG/YOUR_REPO",
        "source_environment": "production",
        "database_name": "",
        "logical_db_id": "",
        "backup_started_at_utc": "",
        "backup_completed_at_utc": "",
        "backup_artifact_id": "",
        "backup_location_redacted": "",
        "backup_size_bytes": 0,
        "backup_sha256": "",
        "restore_drill_required": True,
        "restore_drill_completed_at_utc": None,
        "restore_drill_result": "fail",
        "restore_target": "not_run",
        "schema_validation_result": "fail",
        "created_by_workflow_run_id": "",
        "operator": "",
        "expires_at_utc": exp,
        "evidence_draft": True,
        "_template_note": TEMPLATE_NOTE,
    }
    json.dump(body, args.output, indent=2, sort_keys=True)
    args.output.write("\n")
    if args.output is not sys.stdout:
        args.output.close()
    print(
        "emit_backup_evidence_template: wrote template. "
        "This JSON cannot pass validation until you remove evidence_draft, fill all fields, "
        "and run scripts/ci/validate_backup_evidence.py.",
        file=sys.stderr,
    )


if __name__ == "__main__":
    main()

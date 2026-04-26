#!/usr/bin/env python3
"""Emit restore-drill-evidence/restore-drill-workflow.json for .github/workflows/restore-drill.yml."""
from __future__ import annotations

import argparse
import json
import os
import sys
from datetime import datetime, timezone
from pathlib import Path


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("--out-dir", type=Path, default=Path("restore-drill-evidence"))
    p.add_argument("--repository", required=True)
    p.add_argument("--backup-evidence-id", required=True, dest="backup_evidence_id", help="Run id or path: form")
    p.add_argument("--restore-target", required=True, help="staging | preprod | temp_db")
    p.add_argument(
        "--dry-run",
        type=lambda s: str(s).strip().lower() in ("1", "true", "yes", "y"),
        required=True,
    )
    p.add_argument(
        "--outcome",
        required=True,
        choices=("pass", "fail", "skipped"),
        help="Overall workflow outcome for the evidence record (honest pass/fail/skipped)",
    )
    p.add_argument("--backup-validation-output", default="", help="One-line redacted result of validate script")
    p.add_argument(
        "--github-run-id",
        default=os.environ.get("GITHUB_RUN_ID", ""),
    )
    args = p.parse_args()
    out_dir: Path = args.out_dir
    out_dir.mkdir(parents=True, exist_ok=True)
    out_path = out_dir / "restore-drill-workflow.json"
    now = datetime.now(tz=timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    target = (args.restore_target or "").strip().lower()
    if target in ("temp_db", "temp-db"):
        target = "temp-db"
    if target not in ("staging", "preprod", "temp-db"):
        print("ERROR: restore_target must be staging, preprod, temp_db, or temp-db", file=sys.stderr)
        raise SystemExit(1)
    doc: dict = {
        "schema_version": "restore-drill-workflow-evidence-v1",
        "evidence_type": "backup-restore-drill-workflow",
        "recorded_at_utc": now,
        "repository": (args.repository or "").strip(),
        "github_workflow_run_id": str(args.github_run_id or "").strip() or None,
        "backup_evidence_id": (args.backup_evidence_id or "").strip(),
        "restore_target_selected": target,
        "restore_target_allows_production": False,
        "dry_run": bool(args.dry_run),
        "workflow_outcome": args.outcome,
        "backup_evidence_validation_summary": (args.backup_validation_output or "").strip() or None,
        "policies": {
            "never_default_production_restore": True,
            "use_staging_or_temp_db": "This workflow is for non-production restore drills; production restore uses separate break-glass procedures.",
        },
    }
    if args.dry_run and args.outcome == "skipped":
        doc["outcome_rationale"] = "dry_run: no restore executed on a live database; validation-only path."
    out_path.write_text(json.dumps(doc, indent=2, sort_keys=False) + "\n", encoding="utf-8")
    print(f"wrote {out_path}", file=sys.stderr)


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""Emit rollback-evidence/rollback-workflow.json for .github/workflows/rollback-prod.yml (stdout paths under cwd)."""
from __future__ import annotations

import argparse
import json
import os
import sys
from datetime import datetime, timezone
from pathlib import Path


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("--out-dir", type=Path, default=Path("rollback-evidence"))
    p.add_argument("--repository", required=True, help="org/repo from GITHUB_REPOSITORY")
    p.add_argument("--incident-id", required=True, dest="incident_id")
    p.add_argument("--reason", required=True)
    p.add_argument(
        "--dry-run",
        type=lambda s: str(s).strip().lower() in ("1", "true", "yes", "y"),
        required=True,
        help="true/false (GitHub boolean expression)",
    )
    p.add_argument("--app-ref", default="", help="Digest-pinned app image ref")
    p.add_argument("--goose-ref", default="", help="Digest-pinned goose image ref")
    p.add_argument("--previous-manifest-run-id", default="", help="Source run id if refs came from a manifest")
    p.add_argument(
        "--github-run-id",
        default=os.environ.get("GITHUB_RUN_ID", ""),
        help="GHA run id of this evidence workflow",
    )
    args = p.parse_args()
    out_dir: Path = args.out_dir
    out_dir.mkdir(parents=True, exist_ok=True)
    out_path = out_dir / "rollback-workflow.json"
    now = datetime.now(tz=timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    doc: dict = {
        "schema_version": "rollback-workflow-evidence-v1",
        "evidence_type": "production-rollback-preflight",
        "recorded_at_utc": now,
        "repository": args.repository,
        "github_workflow_run_id": str(args.github_run_id or "").strip() or None,
        "incident_id": (args.incident_id or "").strip(),
        "reason": (args.reason or "").strip(),
        "dry_run": bool(args.dry_run),
        "rollback_target_app_image_ref": (args.app_ref or "").strip() or None,
        "rollback_target_goose_image_ref": (args.goose_ref or "").strip() or None,
        "source_previous_deployment_manifest_run_id": (args.previous_manifest_run_id or "").strip() or None,
        "execution_note": (
            "dry_run: no SSH; evidence only. Re-run with dry_run=false after approval, then execute rollback "
            "via **Deploy Production** (action_mode=rollback) with the same digest-pinned image refs, or on-host scripts."
            if args.dry_run
            else "Validated digests; operator executes **Deploy Production** rollback or approved on-host release scripts using these refs. This workflow does not SSH by itself."
        ),
    }
    out_path.write_text(json.dumps(doc, indent=2, sort_keys=False) + "\n", encoding="utf-8")
    print(f"wrote {out_path}", file=sys.stderr)


if __name__ == "__main__":
    main()

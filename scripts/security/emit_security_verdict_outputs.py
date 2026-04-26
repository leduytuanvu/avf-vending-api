#!/usr/bin/env python3
"""Emit GitHub Actions step output lines (key=value) from security-reports/security-verdict.json."""
from __future__ import annotations

import json
import os
import sys
from pathlib import Path


def _image_refs(payload: dict) -> tuple[str, str]:
    """Top-level app/goose refs from JSON, falling back to published_images (same source as write_security_verdict baseline)."""
    pub = payload.get("published_images")
    if not isinstance(pub, dict):
        pub = {}
    a = str(payload.get("app_image_ref", "") or "").strip()
    g = str(payload.get("goose_image_ref", "") or "").strip()
    if not a:
        a = str(pub.get("app_image_ref", "") or "").strip()
    if not g:
        g = str(pub.get("goose_image_ref", "") or "").strip()
    return (a, g)


def main() -> None:
    path = Path(os.environ.get("SECURITY_VERDICT_JSON", "security-reports/security-verdict.json"))
    if not path.is_file():
        print("error: security verdict JSON not found: %s" % path, file=sys.stderr)
        sys.exit(1)
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, OSError) as e:
        print("error: could not read verdict JSON: %s" % e, file=sys.stderr)
        sys.exit(1)
    v_block = payload.get("verdict", "") or ""
    app_ref, goose_ref = _image_refs(payload)
    jr = payload.get("job_results")
    if not isinstance(jr, dict):
        jr = {}
    gen = payload.get("generated_at_utc", "")
    outputs = {
        "security_verdict": v_block,
        "SECURITY_VERDICT": v_block,
        "source_build_run_id": payload.get("source_build_run_id", ""),
        "source_sha": payload.get("source_sha", ""),
        "source_branch": payload.get("source_branch", ""),
        "source_event": payload.get("source_event", ""),
        "source_workflow_name": payload.get("source_workflow_name", ""),
        "security_workflow_run_id": payload.get("security_workflow_run_id", payload.get("workflow_run_id", "")),
        "release_gate_verdict": payload.get("release_gate_verdict", ""),
        "release_gate_mode": payload.get("release_gate_mode", ""),
        "repo_security_verdict": payload.get("repo_security_verdict", ""),
        "repo_release_verdict": payload.get("repo_release_verdict", ""),
        "published_image_verdict": payload.get("published_image_verdict", ""),
        "provenance_release_verdict": payload.get("provenance_release_verdict", ""),
        "app_image_ref": app_ref,
        "goose_image_ref": goose_ref,
        "generated_at_utc": gen,
        "job_results": jr,
    }
    for key, value in outputs.items():
        if value is None:
            value = ""
        if isinstance(value, (dict, list)):
            value = json.dumps(value)
        print("%s=%s" % (key, value.replace("\n", " ").replace("\r", " ")))


if __name__ == "__main__":
    main()

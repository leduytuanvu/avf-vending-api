#!/usr/bin/env python3
"""Emit GitHub Actions step output lines (key=value) from security-reports/security-verdict.json."""
from __future__ import annotations

import json
import os
import sys
from pathlib import Path


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
    outputs = {
        "security_verdict": payload.get("verdict", ""),
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
    }
    for key, value in outputs.items():
        if value is None:
            value = ""
        if isinstance(value, (dict, list)):
            value = json.dumps(value)
        print("%s=%s" % (key, value.replace("\n", " ").replace("\r", " ")))


if __name__ == "__main__":
    main()

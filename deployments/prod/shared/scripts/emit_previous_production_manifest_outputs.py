#!/usr/bin/env python3
"""Read production-deployment-manifest.json from a prior successful deploy and emit GitHub Actions OUTPUT lines.

Clears image refs and digests when not digest-pinned. Sets rollback_available true only when both refs are valid.
"""
from __future__ import annotations

import json
import sys
from pathlib import Path


def digest_pinned(ref: str) -> bool:
    if not ref or not isinstance(ref, str):
        return False
    if ":latest" in ref:
        return False
    return "@sha256:" in ref


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: emit_previous_production_manifest_outputs.py <path-to-production-deployment-manifest.json>", file=sys.stderr)
        return 2
    path = Path(sys.argv[1])
    if not path.is_file():
        return 0

    payload = json.loads(path.read_text(encoding="utf-8"))
    app_ref = str(payload.get("app_image_ref") or "")
    goose_ref = str(payload.get("goose_image_ref") or "")

    if not digest_pinned(app_ref):
        app_ref = ""
    if not digest_pinned(goose_ref):
        goose_ref = ""

    app_digest = str(payload.get("app_digest") or "")
    goose_digest = str(payload.get("goose_digest") or "")
    if not app_ref:
        app_digest = ""
    if not goose_ref:
        goose_digest = ""

    rollback_available = "true" if app_ref and goose_ref else "false"

    mapping = {
        "previous_action_mode": str(payload.get("action_mode") or ""),
        "previous_actor": str(payload.get("actor") or ""),
        "previous_app_digest": app_digest,
        "previous_app_image_ref": app_ref,
        "previous_deployed_at_utc": str(
            payload.get("deployed_at_utc") or payload.get("completed_at_utc") or ""
        ),
        "previous_goose_digest": goose_digest,
        "previous_goose_image_ref": goose_ref,
        "previous_source_branch": str(payload.get("source_branch") or ""),
        "previous_source_commit_sha": str(payload.get("source_commit_sha") or ""),
        "rollback_available": rollback_available,
    }
    for key, value in mapping.items():
        if any(ch in value for ch in "\n\r"):
            raise ValueError(f"unexpected newline in manifest field {key}")
        print(f"{key}={value}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

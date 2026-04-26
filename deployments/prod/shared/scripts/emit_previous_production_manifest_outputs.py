#!/usr/bin/env python3
"""Read production-deployment-manifest.json from a prior successful deploy and emit GitHub Actions OUTPUT lines.

Only manifests from a **deploy** (not rollback) on **main** with digest-pinned app **and** goose refs are
eligible for rollback_available=true. Rollback workflow runs must not be used as last-known-good.
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


def _branch_is_main(branch: str) -> bool:
    b = (branch or "").strip()
    return b in ("main", "refs/heads/main")


def _action_mode_is_deploy(mode: str) -> bool:
    """LKG must come from a normal production deploy, not rollback or dry-run."""
    m = (mode or "").strip().lower().replace(" ", "").replace("-", "")
    if m == "deploy":
        return True
    if m in ("", "rollback", "dryrun"):
        return False
    # Unknown modes are not safe LKG sources
    return False


def _emit_empty(reason: str) -> None:
    mapping = {
        "previous_action_mode": "",
        "previous_actor": "",
        "previous_app_digest": "",
        "previous_app_image_ref": "",
        "previous_deployed_at_utc": "",
        "previous_goose_digest": "",
        "previous_goose_image_ref": "",
        "previous_source_branch": "",
        "previous_source_commit_sha": "",
        "rollback_available": "false",
        "lkg_reject_reason": reason,
    }
    for key, value in mapping.items():
        if any(ch in value for ch in "\n\r"):
            raise ValueError(f"unexpected newline in field {key}")
        print(f"{key}={value}")


def main() -> int:
    if len(sys.argv) != 2:
        print(
            "usage: emit_previous_production_manifest_outputs.py <path-to-production-deployment-manifest.json>",
            file=sys.stderr,
        )
        return 2
    path = Path(sys.argv[1])
    if not path.is_file():
        _emit_empty("manifest file missing")
        return 0

    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, OSError) as e:
        _emit_empty("manifest not valid json: %s" % e)
        return 0

    mode = str(payload.get("action_mode") or "")
    branch = str(payload.get("source_branch") or "")
    app_ref = str(payload.get("app_image_ref") or "")
    goose_ref = str(payload.get("goose_image_ref") or "")

    reasons: list[str] = []
    if not _branch_is_main(branch):
        reasons.append("source_branch must be main (got %r)" % branch)
    if not _action_mode_is_deploy(mode):
        reasons.append("action_mode must be deploy for LKG (got %r); rollback/dry-run manifests are excluded" % mode)

    if not digest_pinned(app_ref):
        reasons.append("app_image_ref must be digest-pinned")
    if not digest_pinned(goose_ref):
        reasons.append("goose_image_ref must be digest-pinned")

    if reasons:
        _emit_empty("; ".join(reasons))
        return 0

    app_digest = str(payload.get("app_digest") or "")
    goose_digest = str(payload.get("goose_digest") or "")

    mapping = {
        "previous_action_mode": mode,
        "previous_actor": str(payload.get("actor") or ""),
        "previous_app_digest": app_digest,
        "previous_app_image_ref": app_ref,
        "previous_deployed_at_utc": str(
            payload.get("deployed_at_utc") or payload.get("completed_at_utc") or ""
        ),
        "previous_goose_digest": goose_digest,
        "previous_goose_image_ref": goose_ref,
        "previous_source_branch": branch,
        "previous_source_commit_sha": str(payload.get("source_commit_sha") or ""),
        "rollback_available": "true",
        "lkg_reject_reason": "",
    }
    for key, value in mapping.items():
        if any(ch in value for ch in "\n\r"):
            raise ValueError(f"unexpected newline in manifest field {key}")
        print(f"{key}={value}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""Append a Source coordinate debug markdown table to GITHUB_STEP_SUMMARY (or stdout)."""
from __future__ import annotations

import json
import os
import sys
from pathlib import Path


def e(key: str, default: str = "") -> str:
    s = (os.environ.get(key) or default or "").replace("\n", " ").replace("|", "\\|").strip()
    return s or "—"


def main() -> None:
    p = Path("security-reports/security-verdict.json")
    vdata: dict
    if p.is_file():
        vdata = json.loads(p.read_text(encoding="utf-8"))
    else:
        vdata = {}
    v = vdata
    rlv = (v.get("repo_level_checks") or {}).get("release_evidence") or {}
    repo_id = (rlv.get("matched_workflow_run_id") or os.environ.get("REPO_RELEASE_MATCHED_RUN_ID") or "").strip() or "—"

    out = [
        "",
        "## Source coordinate debug",
        "",
        "Canonical coordinates prefer **Build artifacts** (promotion-manifest / image-metadata). "
        "The **triggering** `workflow_run` is the *Build and Push Images* run (may be a chain: CI → Build); "
        "its `head_branch` / `head_sha` / `event` are **not** the semantic source of truth when they differ from artifacts.",
        "",
        "| Field | Value |",
        "| --- | --- |",
        "| `trigger_build_run_id` (Build workflow that triggered Security Release) | %s |" % e("TRIGGERING_BUILD_ID"),
        "| `trigger_workflow_run_sha` | %s |" % e("TRIGGER_WORKFLOW_RUN_SOURCE_SHA"),
        "| `trigger_workflow_run_branch` | %s |" % e("BUILD_HEAD_BRANCH"),
        "| `trigger_workflow_run_event` (GHA type of the Build run) | %s |" % e("TRIGGERING_BUILD_EVENT"),
        "| `artifact_source_sha` | %s |" % e("RESOLVED_SOURCE_SHA"),
        "| `artifact_source_branch` | %s |" % e("RESOLVED_SOURCE_BRANCH"),
        "| `artifact_source_event` (semantic, from manifest) | %s |" % e("ARTIFACT_SOURCE_EVENT"),
        "| `canonical_source_sha` | %s |" % e("CANONICAL_SOURCE_SHA"),
        "| `canonical_source_branch` | %s |" % e("CANONICAL_SOURCE_BRANCH"),
        "| `canonical_source_event` (semantic, gate) | %s |" % e("CANONICAL_SOURCE_EVENT"),
        "| `source_build_run_id` (from artifacts) | %s |" % e("BUILD_RUN_ID"),
        "| `repo_security_run_found` (Security workflow push, matched id) | %s |" % (repo_id if repo_id not in ("",) else e("REPO_RELEASE_MATCHED_RUN_ID")),
        "| `repo_release_verdict` | %s |" % (v.get("repo_release_verdict", e("REPO_RELEASE_VERDICT")) or "—"),
        "| `repo_security_verdict` (repo-level in verdict model) | %s |" % (v.get("repo_security_verdict", "") or "—"),
        "| `published_image_verdict` | %s |" % (v.get("published_image_verdict", "") or "—"),
        "| `final security_verdict` | **%s** |" % (v.get("verdict", e("V_VERDICT")) or "—"),
        "",
    ]
    text = "\n".join(out) + "\n"
    ghs = os.environ.get("GITHUB_STEP_SUMMARY", "")
    if ghs:
        Path(ghs).open("a", encoding="utf-8").write(text)
    else:
        sys.stdout.write(text)


if __name__ == "__main__":
    main()

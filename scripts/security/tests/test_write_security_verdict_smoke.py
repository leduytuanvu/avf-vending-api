#!/usr/bin/env python3
"""Local smoke: write_security_verdict.py always materializes security-reports/security-verdict.json (no traceback)."""
from __future__ import annotations

import json
import os
import subprocess
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parents[3]
VERDICT = REPO / "security-reports" / "security-verdict.json"
WRITER = REPO / "scripts" / "security" / "write_security_verdict.py"


def _run(
    *args: str,
    env: dict[str, str] | None = None,
) -> None:
    e = {**os.environ, **(env or {})}
    p = subprocess.run(
        [sys.executable, str(WRITER), *args],
        cwd=str(REPO),
        env=e,
        capture_output=True,
        text=True,
        check=False,
    )
    if p.returncode != 0:
        print(p.stdout, p.stderr, file=sys.stderr)
        raise AssertionError("writer exited with %r" % (p.returncode,))
    if p.stderr and "Traceback" in p.stderr:
        raise AssertionError("unexpected Python traceback in stderr: %r" % (p.stderr[:2000],))


def _assert_file_and_schema() -> dict:
    assert VERDICT.is_file(), "security-reports/security-verdict.json missing"
    return json.loads(VERDICT.read_text(encoding="utf-8"))


def _clear_verdict() -> None:
    if VERDICT.is_file():
        VERDICT.unlink()
    pdir = VERDICT.parent
    if pdir.is_dir() and not any(pdir.iterdir()):
        try:
            pdir.rmdir()
        except OSError:
            pass


def main() -> int:
    if not WRITER.is_file():
        print("test_write_security_verdict_smoke: missing writer", WRITER, file=sys.stderr)
        return 1

    # 1) emergency
    _clear_verdict()
    _run("emergency", "--emergency-reason", "smoke-test")
    d = _assert_file_and_schema()
    assert d.get("verdict") in ("fail", "error") or d.get("security_verdict") == "fail", d

    # 2) skipped
    _clear_verdict()
    _run("skipped", env={"EVENT_NAME": "workflow_run", "WORKFLOW_RUN_ID": "1", "WORKFLOW_NAME": "Security Release"})
    d = _assert_file_and_schema()
    assert d.get("verdict") == "skipped", d

    # 3) no-candidate
    _clear_verdict()
    _run("no-candidate", env={"EVENT_NAME": "workflow_run", "WORKFLOW_RUN_ID": "1", "RESOLVE_BUILD_RUN_RESULT": "failure"})
    d = _assert_file_and_schema()
    assert d.get("verdict") in ("skipped", "fail", "pass"), d

    # 3b) unsupported-trigger (chain-only Build: GHA event workflow_run)
    _clear_verdict()
    _run(
        "unsupported-trigger",
        env={
            "EVENT_NAME": "workflow_run",
            "WORKFLOW_RUN_ID": "1",
            "WORKFLOW_NAME": "Security Release",
            "TRIGGERING_BUILD_EVENT": "workflow_run",
        },
    )
    d = _assert_file_and_schema()
    assert d.get("verdict") == "skipped" and d.get("release_gate_mode", "").find("unsupported") != -1, d

    # 4) full: repo release evidence unavailable (structured failure_reasons, not traceback)
    _clear_verdict()
    sha = "0" * 40
    env_full = {
        "EVENT_NAME": "workflow_run",
        "WORKFLOW_RUN_ID": "1",
        "WORKFLOW_NAME": "Security Release",
        "GENERATED_AT_UTC": "2020-01-01T00:00:00Z",
        "RESOLVE_BUILD_RUN_RESULT": "success",
        "RESOLVE_IMAGE_REFS_RESULT": "success",
        "IMAGE_SCAN_RESULT": "success",
        "TRIVY_APP_RESULT": "success",
        "TRIVY_GOOSE_RESULT": "success",
        "RESOLVED_SOURCE_SHA": sha,
        "RESOLVED_SOURCE_BRANCH": "develop",
        "CANONICAL_SOURCE_SHA": sha,
        "CANONICAL_SOURCE_BRANCH": "develop",
        "ARTIFACT_SOURCE_EVENT": "push",
        "CANONICAL_SOURCE_EVENT": "push",
        "APP_IMAGE_REF": "ghcr.io/x/app@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
        "GOOSE_IMAGE_REF": "ghcr.io/x/g@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
        "PROVENANCE_VERDICT": "verified",
        "REPO_RELEASE_VERDICT": "unavailable",
        "REPO_RELEASE_EVIDENCE_SOURCE": "security-workflow-not-found-or-not-successful",
        "REPO_RELEASE_SUMMARY": "smoke: no security run (test)",
    }
    _run("full", env=env_full)
    d = _assert_file_and_schema()
    fr = d.get("failure_reasons")
    assert isinstance(fr, list) and len(fr) > 0, f"expected non-empty failure_reasons, got {d!r}"
    assert d.get("verdict") in ("pass", "fail", "skipped"), d

    _clear_verdict()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

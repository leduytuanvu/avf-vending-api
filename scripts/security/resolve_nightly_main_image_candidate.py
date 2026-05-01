#!/usr/bin/env python3
"""Resolve nightly Trivy image candidate: latest successful Build on main with valid promotion-manifest.

Writes nightly-image-candidate.json under --out-dir. Appends GitHub Actions outputs when GITHUB_OUTPUT is set.
Exit 2 = no candidate (fail-closed consumer); 0 = ok; 1 = unexpected internal error.
"""
from __future__ import annotations

import argparse
import json
import os
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path
from typing import Any

SKIP_MISSING_ARTIFACT = "missing promotion-manifest artifact"
SKIP_DOWNLOAD = "download failed"
SKIP_MISSING_JSON = "missing promotion-manifest.json"
SKIP_BRANCH = "source_branch mismatch"
SKIP_EVENT = "unsupported source_event"
SKIP_DIGEST = "missing digest-pinned refs"


def _repo() -> str:
    r = (os.environ.get("GITHUB_REPOSITORY") or "").strip()
    if not r:
        print("error: GITHUB_REPOSITORY is required", file=sys.stderr)
        sys.exit(1)
    return r


def _append_github_output(line: str) -> None:
    path = os.environ.get("GITHUB_OUTPUT")
    if path:
        with open(path, "a", encoding="utf-8") as fh:
            fh.write(line + "\n")


def _gh(args: list[str]) -> tuple[int, str, str]:
    r = subprocess.run(args, capture_output=True, text=True)
    return r.returncode, r.stdout or "", r.stderr or ""


def _manifest_refs_and_checks(data: dict[str, Any]) -> tuple[str | None, dict[str, str]]:
    """Return (skip_reason or None, refs dict with app_image_ref, goose_image_ref, source_sha)."""
    branch = (data.get("source_branch") or "").strip()
    if branch != "main":
        return SKIP_BRANCH, {}
    ev = (data.get("source_event") or "").strip()
    if ev not in ("push", "workflow_dispatch"):
        return SKIP_EVENT, {}
    app = (data.get("app_image_ref") or data.get("app_ref") or "").strip()
    goose = (data.get("goose_image_ref") or data.get("goose_ref") or "").strip()
    if "@sha256:" not in app or "@sha256:" not in goose:
        return SKIP_DIGEST, {}
    sha = (data.get("commit_sha") or data.get("source_sha") or "").strip()
    return None, {"app_image_ref": app, "goose_image_ref": goose, "source_sha": sha}


def _emit(payload: dict[str, Any], out_path: Path) -> None:
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--out-dir", default="nightly-reports", type=Path, help="Directory for nightly-image-candidate.json")
    args = ap.parse_args()
    out_dir: Path = args.out_dir
    json_out = out_dir / "nightly-image-candidate.json"
    repo = _repo()

    if not shutil.which("gh"):
        _emit(
            {
                "status": "no-candidate",
                "reason": "gh CLI not available on PATH",
                "inspected_run_count": 0,
                "inspected_runs": [],
            },
            json_out,
        )
        _append_github_output("resolution_status=no-candidate")
        sys.exit(2)

    list_url = "repos/%s/actions/workflows/build-push.yml/runs" % repo
    rc, out, err = _gh(
        [
            "gh",
            "api",
            "-X",
            "GET",
            list_url,
            "-f",
            "branch=main",
            "-f",
            "per_page=30",
        ]
    )
    inspected: list[dict[str, Any]] = []
    if rc != 0:
        _emit(
            {
                "status": "no-candidate",
                "reason": "GitHub API workflow runs list failed (exit %d)" % rc,
                "inspected_run_count": 0,
                "inspected_runs": [],
                "api_diagnostic": (err or out or "").strip() or "(no stderr)",
            },
            json_out,
        )
        _append_github_output("resolution_status=no-candidate")
        sys.exit(2)

    try:
        blob = json.loads(out)
    except json.JSONDecodeError as e:
        _emit(
            {
                "status": "no-candidate",
                "reason": "GitHub API returned invalid JSON for workflow runs list",
                "inspected_run_count": 0,
                "inspected_runs": [],
                "api_diagnostic": str(e),
            },
            json_out,
        )
        _append_github_output("resolution_status=no-candidate")
        sys.exit(2)

    runs = blob.get("workflow_runs") or []
    success_ids: list[int] = []
    for wr in runs:
        if not isinstance(wr, dict):
            continue
        if (wr.get("conclusion") or "") != "success":
            continue
        rid = wr.get("id")
        if isinstance(rid, int):
            success_ids.append(rid)
        elif isinstance(rid, str) and rid.isdigit():
            success_ids.append(int(rid))

    if not success_ids:
        _emit(
            {
                "status": "no-candidate",
                "reason": "no successful Build and Push Images runs returned for branch main",
                "inspected_run_count": 0,
                "inspected_runs": [],
            },
            json_out,
        )
        _append_github_output("resolution_status=no-candidate")
        sys.exit(2)

    for rid in success_ids:
        rid_s = str(rid)
        art_rc, art_out, art_err = _gh(["gh", "api", "-X", "GET", "repos/%s/actions/runs/%s/artifacts" % (repo, rid_s)])
        if art_rc != 0:
            inspected.append(
                {
                    "run_id": rid_s,
                    "skip_reason": SKIP_MISSING_ARTIFACT,
                    "diagnostic": (art_err or art_out or "").strip() or "artifacts API failed",
                }
            )
            continue
        try:
            art_blob = json.loads(art_out)
        except json.JSONDecodeError:
            inspected.append({"run_id": rid_s, "skip_reason": SKIP_MISSING_ARTIFACT, "diagnostic": "artifacts JSON parse error"})
            continue
        names = [a.get("name") for a in (art_blob.get("artifacts") or []) if isinstance(a, dict)]
        if "promotion-manifest" not in names:
            inspected.append({"run_id": rid_s, "skip_reason": SKIP_MISSING_ARTIFACT})
            continue

        tmp = tempfile.mkdtemp(prefix="nightly-promo-")
        try:
            dl_rc, _dl_out, dl_err = _gh(
                ["gh", "run", "download", rid_s, "-n", "promotion-manifest", "-R", repo, "-D", tmp]
            )
            if dl_rc != 0:
                inspected.append(
                    {
                        "run_id": rid_s,
                        "skip_reason": SKIP_DOWNLOAD,
                        "diagnostic": (dl_err or _dl_out or "").strip() or "gh run download failed",
                    }
                )
                continue
            mf_candidates = sorted(Path(tmp).rglob("promotion-manifest.json"), key=lambda p: len(str(p)))
            mf = mf_candidates[0] if mf_candidates else Path(tmp) / "promotion-manifest.json"
            if not mf.is_file():
                inspected.append({"run_id": rid_s, "skip_reason": SKIP_MISSING_JSON})
                continue
            try:
                promo = json.loads(mf.read_text(encoding="utf-8"))
            except (json.JSONDecodeError, OSError) as e:
                inspected.append({"run_id": rid_s, "skip_reason": SKIP_MISSING_JSON, "diagnostic": str(e)})
                continue
            skip, refs = _manifest_refs_and_checks(promo)
            if skip:
                inspected.append({"run_id": rid_s, "skip_reason": skip})
                continue
        finally:
            for p in Path(tmp).glob("*"):
                try:
                    p.unlink()
                except OSError:
                    pass
            try:
                Path(tmp).rmdir()
            except OSError:
                pass

        payload_ok = {
            "status": "ok",
            "build_run_id": rid_s,
            "source_branch": "main",
            "source_sha": refs["source_sha"],
            "source_event": (promo.get("source_event") or "").strip(),
            "app_image_ref": refs["app_image_ref"],
            "goose_image_ref": refs["goose_image_ref"],
        }
        _emit(payload_ok, json_out)
        _append_github_output("resolution_status=ok")
        _append_github_output("app_image_ref=%s" % refs["app_image_ref"])
        _append_github_output("goose_image_ref=%s" % refs["goose_image_ref"])
        _append_github_output("build_run_id=%s" % rid_s)
        sys.exit(0)

    summary = "no qualifying promotion-manifest among %d successful main builds inspected" % len(inspected)
    _emit(
        {
            "status": "no-candidate",
            "reason": summary,
            "inspected_run_count": len(inspected),
            "inspected_runs": inspected[-30:],
        },
        json_out,
    )
    _append_github_output("resolution_status=no-candidate")
    sys.exit(2)


if __name__ == "__main__":
    main()

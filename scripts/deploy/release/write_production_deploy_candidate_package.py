#!/usr/bin/env python3
"""Emit production-deploy-candidate artifact directory from security-verdict.json (pass + main only).

Does not deploy production. Outputs: production-deploy-inputs.json, production-deploy-inputs.env,
production-deploy-candidate-metadata.json (semantic source_event vs trigger_workflow_event),
deploy-production-gh-command.sh, README.md — for manual Deploy Production (workflow_dispatch).
"""
from __future__ import annotations

import argparse
import json
import shlex
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

EXPECTED_RELEASE_GATE_MODE = "full-security-release-gate"
DEPLOY_WORKFLOW_TITLE = "Deploy Production"
TODO_STAGING_EVIDENCE_RUN_ID = "TODO_STAGING_EVIDENCE_RUN_ID"


def _die(msg: str) -> None:
    print("error: %s" % msg, file=sys.stderr)
    sys.exit(1)


def _require(cond: bool, msg: str) -> None:
    if not cond:
        _die(msg)


def _as_str(v: Any) -> str:
    if v is None:
        return ""
    if isinstance(v, str):
        return v
    return str(v)


def _digest_pinned(ref: str) -> bool:
    return "@sha256:" in ref


def load_verdict(path: Path) -> dict[str, Any]:
    try:
        return json.loads(path.read_text(encoding="utf-8-sig"))
    except FileNotFoundError:
        _die("security verdict file not found: %s" % path)
    except json.JSONDecodeError as e:
        _die("invalid JSON in %s: %s" % (path, e))


def validate_eligibility(payload: dict[str, Any]) -> None:
    verdict = _as_str(payload.get("verdict")).strip()
    _require(
        verdict == "pass",
        'verdict must be "pass" (got %r); skipped/fail verdicts must not produce production-deploy-candidate'
        % verdict,
    )
    if "security_verdict" in payload:
        sv = _as_str(payload.get("security_verdict")).strip()
        _require(sv == "pass", 'security_verdict must be "pass" when present (got %r)' % sv)
    rgv = _as_str(payload.get("release_gate_verdict")).strip()
    _require(rgv == "pass", 'release_gate_verdict must be "pass" (got %r)' % rgv)
    rgm = _as_str(payload.get("release_gate_mode")).strip()
    _require(
        rgm == EXPECTED_RELEASE_GATE_MODE,
        "release_gate_mode must be %r (got %r)" % (EXPECTED_RELEASE_GATE_MODE, rgm),
    )
    branch = _as_str(payload.get("source_branch")).strip()
    _require(branch == "main", "source_branch must be main for production candidate (got %r)" % branch)
    bid = _as_str(payload.get("source_build_run_id")).strip()
    _require(bool(bid), "source_build_run_id must be non-empty")
    _require(bid.isdigit(), "source_build_run_id must be digits only (got %r)" % bid)
    sha = _as_str(payload.get("source_sha")).strip()
    _require(bool(sha), "source_sha must be non-empty")
    pub = payload.get("published_images")
    _require(isinstance(pub, dict), "published_images must be an object")
    app_ref = _as_str(pub.get("app_image_ref")).strip()
    goose_ref = _as_str(pub.get("goose_image_ref")).strip()
    _require(bool(app_ref), "published_images.app_image_ref must be non-empty")
    _require(bool(goose_ref), "published_images.goose_image_ref must be non-empty")
    _require(_digest_pinned(app_ref), "app_image_ref must be digest-pinned (@sha256:)")
    _require(_digest_pinned(goose_ref), "goose_image_ref must be digest-pinned (@sha256:)")
    src_ev = _as_str(payload.get("source_event")).strip()
    _require(
        src_ev in ("push", "workflow_dispatch"),
        "source_event must be push or workflow_dispatch for production candidate (semantic promotion field from security-verdict; got %r)"
        % src_ev,
    )


def security_release_run_id_from(payload: dict[str, Any]) -> str:
    rid = _as_str(payload.get("security_workflow_run_id")).strip()
    if not rid:
        rid = _as_str(payload.get("workflow_run_id")).strip()
    _require(bool(rid), "security release run id missing (security_workflow_run_id / workflow_run_id)")
    _require(rid.isdigit(), "security_release_run_id must be digits only (got %r)" % rid)
    return rid


def short_sha(source_sha: str, n: int = 7) -> str:
    s = source_sha.strip()
    return s[:n] if len(s) >= n else s


def default_release_tag(source_sha: str) -> str:
    d = datetime.now(timezone.utc).strftime("%Y%m%d")
    return "v%s-%s" % (d, short_sha(source_sha))


def build_dispatch_inputs(payload: dict[str, Any], release_tag: str, staging_evidence_id: str) -> dict[str, Any]:
    pub = payload["published_images"]
    sec_run = security_release_run_id_from(payload)
    return {
        "action_mode": "deploy",
        "build_run_id": _as_str(payload.get("source_build_run_id")).strip(),
        "security_release_run_id": sec_run,
        "release_tag": release_tag,
        "source_commit_sha": _as_str(payload.get("source_sha")).strip(),
        "app_image_ref": _as_str(pub.get("app_image_ref")).strip(),
        "goose_image_ref": _as_str(pub.get("goose_image_ref")).strip(),
        "rollback_app_image_ref": "",
        "rollback_goose_image_ref": "",
        "deploy_data_node": False,
        "allow_app_node_on_data_node": False,
        "deploy_production_confirmation": "DEPLOY_PRODUCTION",
        "fleet_scale_target": "pilot",
        "telemetry_storm_evidence_repo_path": "",
        "telemetry_storm_evidence_artifact_run_id": "",
        "allow_scale_gate_bypass": False,
        "scale_gate_bypass_reason": "",
        "storm_evidence_max_age_days": "7",
        "run_migration": False,
        "backup_evidence_id": "",
        "staging_evidence_id": staging_evidence_id,
        "staging_evidence_max_age_hours": "168",
        "allow_missing_staging_evidence": False,
        "missing_staging_evidence_reason": "",
        "enable_business_synthetic_smoke": False,
    }


def env_line(key: str, value: Any) -> str:
    if isinstance(value, bool):
        return "%s=%s" % (key, "true" if value else "false")
    return "%s=%s" % (key, shlex.quote(str(value)))


def write_env(path: Path, data: dict[str, Any]) -> None:
    lines = [env_line(k, data[k]) for k in sorted(data.keys())]
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def write_candidate_metadata(path: Path, payload: dict[str, Any]) -> None:
    pub = payload.get("published_images") or {}
    if not isinstance(pub, dict):
        pub = {}
    rid = security_release_run_id_from(payload)
    meta = {
        "schema_version": "production-deploy-candidate-metadata-v1",
        "source_event": _as_str(payload.get("source_event")).strip(),
        "trigger_workflow_event": _as_str(payload.get("trigger_workflow_event")).strip(),
        "source_build_run_id": _as_str(payload.get("source_build_run_id")).strip(),
        "source_sha": _as_str(payload.get("source_sha")).strip(),
        "app_image_ref": _as_str(pub.get("app_image_ref")).strip(),
        "goose_image_ref": _as_str(pub.get("goose_image_ref")).strip(),
        "security_workflow_run_id": rid,
    }
    path.write_text(json.dumps(meta, indent=2, sort_keys=False) + "\n", encoding="utf-8")


def write_readme(path: Path, inputs: dict[str, Any]) -> None:
    body = """# Production deploy candidate

This bundle is produced by **Security Release** after **verdict=pass** on **`main`** only.
It does **not** deploy production and contains **no secrets**.

> **Replace TODO_STAGING_EVIDENCE_RUN_ID with a successful Staging Deployment Contract run id before production deploy. Do not run production deploy with this placeholder.**

## Files

| File | Purpose |
|------|---------|
| `production-deploy-inputs.json` | `workflow_dispatch` inputs for **%s** (`deploy-prod.yml`). Safe to pass to `gh workflow run ... --json`. |
| `production-deploy-inputs.env` | Same values as `KEY=value` for review. |
| `production-deploy-candidate-metadata.json` | Machine-readable **semantic** `source_event` (promotion-manifest) vs **`trigger_workflow_event`** (Build GitHub API wrapper), plus ids and digest refs — not passed to `gh workflow run`. |
| `deploy-production-gh-command.sh` | Wrapper around **`gh workflow run`**; exits non‑zero if **`staging_evidence_id`** is TODO or empty without intentional bypass fields. |

## Before dispatch

1. Replace **`staging_evidence_id`** in `production-deploy-inputs.json` (successful **Staging Deployment Contract** run id). **`allow_missing_staging_evidence`** stays **false** here — use explicit bypass inputs only with **`missing_staging_evidence_reason`** when policy allows (not the normal path).
2. Optionally replace **`release_tag`** (`%s`).
3. Confirm **`build_run_id`** matches **`security-verdict.source_build_run_id`** and **`security_release_run_id`** is this Security Release run (not the Build run).

## CLI example

From a clone of this repository (authenticated `gh`), after exporting `REPO_ROOT`:

```bash
bash deploy-production-gh-command.sh
```

Or:

```bash
gh workflow run \"%s\" --ref main --json < production-deploy-inputs.json
```
""" % (
        DEPLOY_WORKFLOW_TITLE,
        inputs.get("release_tag", ""),
        DEPLOY_WORKFLOW_TITLE,
    )
    path.write_text(body, encoding="utf-8")


def write_gh_script(path: Path) -> None:
    # No %-formatting on this template: embedded Python in the heredoc legitimately uses "%"
    # for its own string formatting; an outer % format would consume those and raise TypeError.
    contents = """#!/usr/bin/env bash
# =============================================================================
# WARNING: Review production-deploy-inputs.json before running.
# Security Release never auto-deploys production.
# =============================================================================
set -Eeuo pipefail
_HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
_JSON="${_HERE}/production-deploy-inputs.json"
python3 - "${_HERE}" <<'PY'
import json, pathlib, sys

TODO = "TODO_STAGING_EVIDENCE_RUN_ID"
here = pathlib.Path(sys.argv[1])

def check(name: str) -> None:
    path = here / name
    if not path.is_file():
        return
    raw = path.read_text(encoding="utf-8")
    if TODO in raw:
        print(
            "error: %s contains %s — replace with a successful Staging Deployment Contract run id before deploy."
            % (name, TODO),
            file=sys.stderr,
        )
        sys.exit(1)
    data = json.loads(raw)
    sid = (data.get("staging_evidence_id") or "").strip()
    allow = data.get("allow_missing_staging_evidence") is True
    reason = (data.get("missing_staging_evidence_reason") or "").strip()
    if sid == TODO:
        print("error: staging_evidence_id is still the TODO literal in %s." % name, file=sys.stderr)
        sys.exit(1)
    if not sid and not (allow and reason):
        print(
            "error: staging_evidence_id empty in %s without allow_missing_staging_evidence=true "
            "and a non-empty missing_staging_evidence_reason." % name,
            file=sys.stderr,
        )
        sys.exit(1)

for fn in ("production-deploy-inputs.json", "production-deploy-request.json"):
    check(fn)
PY
cd "${REPO_ROOT:?export REPO_ROOT to your avf-vending-api clone}"
gh workflow run "__DEPLOY_WORKFLOW_TITLE__" --ref main --json < "${_JSON}"
""".replace("__DEPLOY_WORKFLOW_TITLE__", DEPLOY_WORKFLOW_TITLE)
    path.write_text(contents, encoding="utf-8")
    mode = path.stat().st_mode
    path.chmod(mode | 0o111)


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--security-verdict", required=True, type=Path)
    ap.add_argument("--out-dir", required=True, type=Path)
    ap.add_argument(
        "--release-tag",
        default="",
        help="override release_tag (default vYYYYMMDD-<short_sha>)",
    )
    ap.add_argument(
        "--staging-evidence-run-id",
        default="",
        help="Staging Deployment Contract run id (default: TODO_STAGING_EVIDENCE_RUN_ID placeholder)",
    )
    args = ap.parse_args()

    payload = load_verdict(args.security_verdict)
    validate_eligibility(payload)

    rel_tag = _as_str(args.release_tag).strip()
    if not rel_tag:
        rel_tag = default_release_tag(_as_str(payload.get("source_sha")))

    staging_id = _as_str(args.staging_evidence_run_id).strip()
    if not staging_id:
        staging_id = TODO_STAGING_EVIDENCE_RUN_ID

    inputs = build_dispatch_inputs(payload, rel_tag, staging_id)
    out = args.out_dir
    out.mkdir(parents=True, exist_ok=True)

    (out / "production-deploy-inputs.json").write_text(
        json.dumps(inputs, indent=2, sort_keys=False) + "\n",
        encoding="utf-8",
    )
    write_env(out / "production-deploy-inputs.env", inputs)

    write_candidate_metadata(out / "production-deploy-candidate-metadata.json", payload)
    write_readme(out / "README.md", inputs)
    write_gh_script(out / "deploy-production-gh-command.sh")


if __name__ == "__main__":
    main()

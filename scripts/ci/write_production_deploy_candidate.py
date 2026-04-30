#!/usr/bin/env python3
"""Build a production-deploy-candidate package from security-verdict.json (pass / main only).

Does not deploy production — emits reviewed inputs for manual workflow_dispatch only.
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
CANONICAL_DEPLOY_WORKFLOW = ".github/workflows/deploy-prod.yml"
STAGING_PLACEHOLDER = "PLACEHOLDER_STAGING_EVIDENCE_RUN_ID"


def _die(msg: str) -> None:
    print(f"error: {msg}", file=sys.stderr)
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
        _die(f"security verdict file not found: {path}")
    except json.JSONDecodeError as e:
        _die(f"invalid JSON in {path}: {e}")


def validate_eligibility(payload: dict[str, Any]) -> None:
    verdict = _as_str(payload.get("verdict")).strip()
    _require(verdict == "pass", f'verdict must be "pass" (got {verdict!r}); skipped/fail verdicts are not valid for production deploy')

    if "security_verdict" in payload:
        sv = _as_str(payload.get("security_verdict")).strip()
        _require(sv == "pass", f'security_verdict must be "pass" when present (got {sv!r})')

    rgv = _as_str(payload.get("release_gate_verdict")).strip()
    _require(rgv == "pass", f'release_gate_verdict must be "pass" (got {rgv!r})')

    rgm = _as_str(payload.get("release_gate_mode")).strip()
    _require(
        rgm == EXPECTED_RELEASE_GATE_MODE,
        f'release_gate_mode must be {EXPECTED_RELEASE_GATE_MODE!r} (got {rgm!r})',
    )

    branch = _as_str(payload.get("source_branch")).strip()
    _require(branch == "main", f'source_branch must be "main" for production candidate (got {branch!r})')

    bid = _as_str(payload.get("source_build_run_id")).strip()
    _require(bool(bid), "source_build_run_id must be non-empty")
    _require(bid.isdigit(), f"source_build_run_id must be digits only (got {bid!r})")

    sha = _as_str(payload.get("source_sha")).strip()
    _require(bool(sha), "source_sha must be non-empty")

    pub = payload.get("published_images")
    _require(isinstance(pub, dict), "published_images must be an object")

    app_ref = _as_str(pub.get("app_image_ref")).strip()
    goose_ref = _as_str(pub.get("goose_image_ref")).strip()
    _require(bool(app_ref), "published_images.app_image_ref must be non-empty")
    _require(bool(goose_ref), "published_images.goose_image_ref must be non-empty")
    _require(_digest_pinned(app_ref), "published_images.app_image_ref must contain '@sha256:' (digest-pinned)")
    _require(_digest_pinned(goose_ref), "published_images.goose_image_ref must contain '@sha256:' (digest-pinned)")


def security_release_run_id_from(payload: dict[str, Any]) -> str:
    rid = _as_str(payload.get("security_workflow_run_id")).strip()
    if not rid:
        rid = _as_str(payload.get("workflow_run_id")).strip()
    _require(bool(rid), "security release run id missing (expected security_workflow_run_id or workflow_run_id)")
    _require(rid.isdigit(), f"security_release_run_id must be digits only (got {rid!r})")
    return rid


def short_sha(source_sha: str, n: int = 7) -> str:
    s = source_sha.strip()
    return s[:n] if len(s) >= n else s


def default_release_tag(source_sha: str) -> str:
    ts = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    return f"prod-{ts}-{short_sha(source_sha)}"


def build_request_json(
    payload: dict[str, Any],
    release_tag: str,
    staging_evidence_id: str,
) -> dict[str, Any]:
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
        return f"{key}={'true' if value else 'false'}"
    return f"{key}={shlex.quote(str(value))}"


def write_env(path: Path, data: dict[str, Any]) -> None:
    lines = [env_line(k, v) for k, v in sorted(data.items())]
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def md_escape_cell(s: str) -> str:
    return s.replace("|", "\\|").replace("\n", " ")


def write_operator_md(
    path: Path,
    rows: list[tuple[str, str, str, str]],
    staging_is_placeholder: bool,
) -> None:
    lines: list[str] = [
        "# Production deploy candidate (manual review)",
        "",
        "This package **does not** deploy production. It only collects inputs aligned with "
        "`Deploy Production` (`deploy-prod.yml`) after a passing Security Release.",
        "",
        "## Rules (read before dispatch)",
        "",
        "- **`security_release_run_id`** must be the **Security Release** workflow run id embedded in "
        "`security-verdict.json` (`security_workflow_run_id` / `workflow_run_id`). "
        "It is **not** the Build and Push Images run id.",
        "- **`build_run_id`** must equal **`security-verdict.source_build_run_id`** for that verdict.",
        "- **Skipped** Security Release verdicts (`verdict: skipped`, wrong gate mode, wrong branch, etc.) "
        "**must never** be used as production authorization.",
        "- **`app_image_ref`** and **`goose_image_ref`** must stay **digest-pinned** (`@sha256:…`) and match "
        "what production gates re-verify from artifacts.",
        "",
    ]
    if staging_is_placeholder:
        lines.extend(
            [
                "> **Warning:** `staging_evidence_id` is still a **placeholder**. "
                "Replace it with a real **Staging Deployment Contract** run id (artifact `staging-deploy-evidence`) "
                "before dispatch unless your process explicitly uses `allow_missing_staging_evidence` "
                "(not generated here).",
                "",
            ]
        )
    lines.extend(
        [
            "## Inputs table",
            "",
            "| field | value | source | copy/paste note |",
            "|---|---|---|---|",
        ]
    )
    for field, value, source, note in rows:
        lines.append(
            f"| `{md_escape_cell(field)}` | `{md_escape_cell(value)}` | {md_escape_cell(source)} | {md_escape_cell(note)} |"
        )
    lines.append("")
    path.write_text("\n".join(lines), encoding="utf-8")


def write_gh_script(path: Path, json_name: str) -> None:
    wf = CANONICAL_DEPLOY_WORKFLOW
    contents = (
        """#!/usr/bin/env bash
# =============================================================================
# WARNING: Review __JSON_NAME__ completely before running. Wrong ids or image refs
# fail closed in Deploy Production - or worse, if mixed with bypass flags elsewhere.
# Security Release never auto-deploys production; only an operator runs this.
# =============================================================================
# Requires: gh CLI, repo auth, and cwd = repository root (so gh resolves workflows).
# This script lives next to __JSON_NAME__; set REPO_ROOT to your avf-vending-api clone.
set -euo pipefail
_JSON="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/__JSON_NAME__"
cd "${REPO_ROOT:?Set REPO_ROOT to your avf-vending-api checkout directory}"
gh workflow run __WORKFLOW__ --ref main --json < "${_JSON}"
"""
        .replace("__JSON_NAME__", json_name)
        .replace("__WORKFLOW__", wf)
    )
    path.write_text(contents, encoding="utf-8")
    mode = path.stat().st_mode
    path.chmod(mode | 0o111)


def main() -> None:
    ap = argparse.ArgumentParser(
        description="Emit production-deploy-candidate files from a passing security-verdict.json.",
    )
    ap.add_argument("--security-verdict", required=True, type=Path, help="Path to security-verdict.json")
    ap.add_argument("--out-dir", required=True, type=Path, help="Directory to create with four output files")
    ap.add_argument(
        "--staging-evidence-run-id",
        default="",
        help="Staging Deployment Contract run id for staging_evidence_id (optional)",
    )
    ap.add_argument("--release-tag", default="", help="release_tag for workflow_dispatch (optional)")
    args = ap.parse_args()

    verdict_path: Path = args.security_verdict
    out_dir: Path = args.out_dir

    payload = load_verdict(verdict_path)
    validate_eligibility(payload)

    staging_id = _as_str(args.staging_evidence_run_id).strip()
    if not staging_id:
        staging_id = STAGING_PLACEHOLDER
    staging_placeholder = staging_id == STAGING_PLACEHOLDER

    rel_tag = _as_str(args.release_tag).strip()
    if not rel_tag:
        rel_tag = default_release_tag(_as_str(payload.get("source_sha")))

    req = build_request_json(payload, rel_tag, staging_id)

    out_dir.mkdir(parents=True, exist_ok=True)
    json_path = out_dir / "production-deploy-request.json"
    json_path.write_text(json.dumps(req, indent=2, sort_keys=False) + "\n", encoding="utf-8")

    rows: list[tuple[str, str, str, str]] = [
        ("action_mode", req["action_mode"], "generated", "Deploy Production dispatch"),
        ("build_run_id", req["build_run_id"], "`security-verdict.source_build_run_id`", "Must match verdict"),
        (
            "security_release_run_id",
            req["security_release_run_id"],
            "`security-verdict.security_workflow_run_id`",
            "Security Release run id - not Build run id",
        ),
        ("release_tag", req["release_tag"], "--release-tag or generated", "Operator label"),
        ("source_commit_sha", req["source_commit_sha"], "`security-verdict.source_sha`", "Must match build"),
        ("app_image_ref", req["app_image_ref"], "`security-verdict.published_images.app_image_ref`", "Digest-pinned"),
        ("goose_image_ref", req["goose_image_ref"], "`security-verdict.published_images.goose_image_ref`", "Digest-pinned"),
        ("deploy_data_node", str(req["deploy_data_node"]).lower(), "generated default", "boolean"),
        ("allow_app_node_on_data_node", str(req["allow_app_node_on_data_node"]).lower(), "generated default", "boolean"),
        (
            "deploy_production_confirmation",
            req["deploy_production_confirmation"],
            "generated default",
            "Exactly DEPLOY_PRODUCTION",
        ),
        ("fleet_scale_target", req["fleet_scale_target"], "generated default", "choice: pilot / scale-*"),
        ("telemetry_storm_evidence_repo_path", req["telemetry_storm_evidence_repo_path"], "generated default", "Optional"),
        (
            "telemetry_storm_evidence_artifact_run_id",
            req["telemetry_storm_evidence_artifact_run_id"],
            "generated default",
            "Optional",
        ),
        ("allow_scale_gate_bypass", str(req["allow_scale_gate_bypass"]).lower(), "generated default", "boolean"),
        ("scale_gate_bypass_reason", req["scale_gate_bypass_reason"], "generated default", "Empty unless bypass"),
        ("storm_evidence_max_age_days", req["storm_evidence_max_age_days"], "generated default", "string"),
        ("run_migration", str(req["run_migration"]).lower(), "generated default", "boolean"),
        ("backup_evidence_id", req["backup_evidence_id"], "generated default", "Required if run_migration"),
        (
            "staging_evidence_id",
            req["staging_evidence_id"],
            "--staging-evidence-run-id or placeholder",
            "Staging Contract run id",
        ),
        ("staging_evidence_max_age_hours", req["staging_evidence_max_age_hours"], "generated default", "string hours"),
        ("allow_missing_staging_evidence", str(req["allow_missing_staging_evidence"]).lower(), "generated default", "boolean"),
        ("missing_staging_evidence_reason", req["missing_staging_evidence_reason"], "generated default", "Empty"),
        ("enable_business_synthetic_smoke", str(req["enable_business_synthetic_smoke"]).lower(), "generated default", "boolean"),
        ("rollback_app_image_ref", req["rollback_app_image_ref"], "generated default", "Deploy mode unused"),
        ("rollback_goose_image_ref", req["rollback_goose_image_ref"], "generated default", "Deploy mode unused"),
    ]

    write_operator_md(out_dir / "production-deploy-inputs.md", rows, staging_placeholder)
    write_env(out_dir / "production-deploy-inputs.env", req)
    write_gh_script(out_dir / "deploy-prod-gh-command.sh", "production-deploy-request.json")


if __name__ == "__main__":
    main()

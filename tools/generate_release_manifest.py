#!/usr/bin/env python3
"""Build machine-readable release manifest + Markdown summary for main release candidates."""
from __future__ import annotations

import argparse
import json
import os
import re
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


def _utc_now() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def _s(key: str, default: str = "") -> str:
    return (os.environ.get(key) or default).strip()


def _load_json(path: Path) -> dict[str, Any]:
    return json.loads(path.read_text(encoding="utf-8"))


def _digest_pinned(ref: str) -> bool:
    r = (ref or "").strip()
    return bool(r) and "@sha256:" in r and not r.endswith(":latest")


def _release_id(source_branch: str, sha: str, build_run: str, sec_run: str) -> str:
    short = (sha or "")[:7] or "unknown"
    br = build_run or "0"
    sr = sec_run or "0"
    b = re.sub(r"[^0-9A-Za-z._-]", "-", source_branch or "unknown")
    return f"rel-{b}-{short}-build-{br}-sec-{sr}"


def _verdict_field(v: dict[str, Any], *keys: str, default: str = "") -> str:
    for k in keys:
        if k in v and v[k] is not None:
            s = str(v[k]).strip()
            if s:
                return s
    nested = v.get("release_gate")
    if isinstance(nested, dict) and "verdict" in nested:
        s = str(nested.get("verdict") or "").strip()
        if s:
            return s
    return default


def _sbom_block(verdict: dict[str, Any], promotion: dict[str, Any] | None) -> dict[str, Any]:
    sb = verdict.get("sbom")
    if isinstance(sb, dict) and sb:
        out = dict(sb)
        out.setdefault("source", "security-verdict")
        return out
    if promotion:
        pm = promotion.get("sbom")
        if isinstance(pm, dict) and pm:
            out = dict(pm)
            out.setdefault("source", "promotion-manifest")
            return out
    return {
        "artifact_name": "sbom-reports",
        "note": "SBOM CycloneDX files are published from Build and Push Images as artifact sbom-reports (see Build run id).",
        "source": "default",
    }


def _rollback_from_verdict(v: dict[str, Any]) -> dict[str, Any]:
    prev_app = _s("RELEASE_MANIFEST_PREVIOUS_APP_IMAGE_REF")
    prev_goose = _s("RELEASE_MANIFEST_PREVIOUS_GOOSE_IMAGE_REF")
    if prev_app or prev_goose:
        return {"previous_app_image_ref": prev_app or None, "previous_goose_image_ref": prev_goose or None}
    return {"previous_app_image_ref": None, "previous_goose_image_ref": None}


def cmd_generate(args: argparse.Namespace) -> int:
    verdict_path = Path(args.verdict)
    if not verdict_path.is_file():
        print(f"error: verdict not found: {verdict_path}", file=sys.stderr)
        return 1
    verdict = _load_json(verdict_path)
    source_branch = str(verdict.get("source_branch") or "").strip()
    if source_branch != "main":
        print(f"info: source_branch is {source_branch!r} (not main); skipping manifest generation")
        return 0

    promo: dict[str, Any] | None = None
    if args.promotion_manifest:
        pp = Path(args.promotion_manifest)
        if pp.is_file():
            promo = _load_json(pp)

    pub = verdict.get("published_images") if isinstance(verdict.get("published_images"), dict) else {}
    app_ref = str(pub.get("app_image_ref") or "").strip()
    goose_ref = str(pub.get("goose_image_ref") or "").strip()
    sha = str(verdict.get("source_sha") or "").strip()
    build_run = str(verdict.get("source_build_run_id") or "").strip()
    sec_run = str(verdict.get("security_workflow_run_id") or verdict.get("workflow_run_id") or _s("GITHUB_RUN_ID")).strip()

    migration = _s("MIGRATION_SAFETY_VERDICT", "unknown")
    commit_msg = _s("SOURCE_COMMIT_MESSAGE")
    if not commit_msg:
        commit_msg = str(verdict.get("source_commit_message") or "").strip()

    env_target = (args.environment_target or "production-candidate").strip()

    sec_v = _verdict_field(verdict, "security_verdict", "verdict")
    rg_v = _verdict_field(verdict, "release_gate_verdict")

    sbom = _sbom_block(verdict, promo)
    if isinstance(sbom, dict) and promo and "workflow_run_id" not in sbom:
        wr = str(promo.get("workflow_run_id") or build_run).strip()
        if wr:
            sbom = dict(sbom)
            sbom.setdefault("build_workflow_run_id", wr)

    manifest: dict[str, Any] = {
        "schema_version": "release-manifest-v1",
        "release_id": _release_id(source_branch, sha, build_run, sec_run),
        "generated_at": _utc_now(),
        "source_branch": source_branch,
        "source_sha": sha,
        "source_commit_message": commit_msg or None,
        "build_run_id": build_run or None,
        "security_release_run_id": sec_run or None,
        "app_image_ref": app_ref or None,
        "goose_image_ref": goose_ref or None,
        "sbom_artifact": sbom,
        "security_verdict": sec_v or None,
        "release_gate_verdict": rg_v or None,
        "migration_safety_verdict": migration,
        "smoke_test_verdict": None,
        "environment_target": env_target,
        "rollback_candidate": _rollback_from_verdict(verdict),
        "deployment": None,
        "image_refs_digest_pinned": {
            "app": _digest_pinned(app_ref),
            "goose": _digest_pinned(goose_ref),
        },
    }

    out_dir = Path(args.output_dir)
    out_dir.mkdir(parents=True, exist_ok=True)
    man_path = out_dir / "release-manifest.json"
    sum_path = out_dir / "release-summary.md"
    man_path.write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    sum_path.write_text(_render_summary(manifest, verdict), encoding="utf-8")
    print(f"wrote {man_path} and {sum_path}")
    return 0


def _render_summary(manifest: dict[str, Any], verdict: dict[str, Any]) -> str:
    lines = [
        f"# Release summary — `{manifest.get('release_id')}`",
        "",
        f"- **Generated (UTC):** `{manifest.get('generated_at')}`",
        f"- **Environment target:** `{manifest.get('environment_target')}`",
        f"- **Source:** `{manifest.get('source_branch')}` @ `{manifest.get('source_sha')}`",
    ]
    if manifest.get("source_commit_message"):
        lines.append(f"- **Commit message (subject/first line):** {manifest['source_commit_message']}")
    lines += [
        f"- **Build run id:** `{manifest.get('build_run_id')}`",
        f"- **Security Release run id:** `{manifest.get('security_release_run_id')}`",
        "",
        "## Images (digest-pinned when candidate)",
        "",
        f"- **App:** `{manifest.get('app_image_ref')}`",
        f"- **Goose:** `{manifest.get('goose_image_ref')}`",
        "",
        "## Verdicts",
        "",
        f"- **Security:** `{manifest.get('security_verdict')}`",
        f"- **Release gate:** `{manifest.get('release_gate_verdict')}`",
        f"- **Migration safety (CI):** `{manifest.get('migration_safety_verdict')}`",
        f"- **Smoke (post-deploy):** `{manifest.get('smoke_test_verdict')}`",
        "",
        "## SBOM",
        "",
        f"```json\n{json.dumps(manifest.get('sbom_artifact'), indent=2)}\n```",
        "",
        "## Rollback candidate (previous digest-pinned refs, when known)",
        "",
        f"```json\n{json.dumps(manifest.get('rollback_candidate'), indent=2)}\n```",
        "",
        "## Raw security verdict pointer",
        "",
        "Full gate detail: artifact `security-verdict` → `security-reports/security-verdict.json` from the same Security Release run.",
        f"- Verdict `generated_at_utc`: `{verdict.get('generated_at_utc', '')}`",
    ]
    return "\n".join(lines) + "\n"


def _smoke_rollup(m: dict[str, Any]) -> str:
    keys = (
        "app_node_a_smoke_result",
        "final_cluster_smoke_result",
        "smoke_post_deploy_result",
    )
    vals = [str(m.get(k) or "").strip() for k in keys]
    if all(v in ("", "skipped", "not-run") for v in vals):
        return "not_run"
    if any(v == "fail" for v in vals):
        return "fail"
    if any(v == "malformed" for v in vals):
        return "malformed"
    if all(v in ("pass", "skipped", "not-run", "") for v in vals) and any(v == "pass" for v in vals):
        return "pass"
    return "unknown"


def cmd_append_deploy(args: argparse.Namespace) -> int:
    man_path = Path(args.manifest)
    sum_path = Path(args.summary)
    prod_path = Path(args.production_manifest)
    if not man_path.is_file():
        print(f"error: manifest not found: {man_path}", file=sys.stderr)
        return 1
    if not prod_path.is_file():
        print(f"error: production manifest not found: {prod_path}", file=sys.stderr)
        return 1
    manifest = _load_json(man_path)
    prod = _load_json(prod_path)

    deployed_at = str(prod.get("deployed_at_utc") or prod.get("completed_at_utc") or "").strip()
    host_group = {
        "app_node_a_host": prod.get("app_node_a_host"),
        "app_node_b_host": prod.get("app_node_b_host"),
        "data_node_host": prod.get("data_node_host"),
        "deploy_data_node": prod.get("deploy_data_node"),
    }
    health = {
        "app_node_a_readiness_result": prod.get("app_node_a_readiness_result"),
        "app_node_b_readiness_result": prod.get("app_node_b_readiness_result"),
    }
    smoke = {
        "app_node_a_smoke_result": prod.get("app_node_a_smoke_result"),
        "final_cluster_smoke_result": prod.get("final_cluster_smoke_result"),
        "smoke_post_deploy_result": prod.get("smoke_post_deploy_result"),
    }
    rollback = {
        "rollback_attempted": prod.get("rollback_attempted"),
        "rollback_result": prod.get("rollback_result"),
        "rollback_available_before_deploy": prod.get("rollback_available_before_deploy"),
    }
    smoke_verdict = _smoke_rollup({**prod})

    manifest["smoke_test_verdict"] = smoke_verdict
    manifest["environment_target"] = str(prod.get("environment") or "production")
    manifest["rollback_candidate"] = {
        "previous_app_image_ref": prod.get("previous_app_image_ref"),
        "previous_goose_image_ref": prod.get("previous_goose_image_ref"),
    }
    manifest["deployment"] = {
        "production_deploy_run_id": prod.get("run_id"),
        "production_deploy_run_url": prod.get("run_url"),
        "deployed_at": deployed_at or None,
        "host_group": host_group,
        "healthcheck": health,
        "smoke": smoke,
        "rollback": rollback,
        "final_result": prod.get("final_result"),
        "final_deployment_verdict": prod.get("final_deployment_verdict"),
    }
    manifest["manifest_updated_at"] = _utc_now()

    man_path.write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")

    post = [
        "",
        "## Post-deploy (production)",
        "",
        f"- **Deployed at (UTC):** `{deployed_at}`",
        f"- **Deploy run:** `{prod.get('run_id')}` — `{prod.get('run_url')}`",
        "",
        "### Hosts",
        "",
        f"```json\n{json.dumps(host_group, indent=2)}\n```",
        "",
        "### Healthcheck",
        "",
        f"```json\n{json.dumps(health, indent=2)}\n```",
        "",
        "### Smoke",
        "",
        f"- **Roll-up:** `{smoke_verdict}`",
        f"```json\n{json.dumps(smoke, indent=2)}\n```",
        "",
        "### Rollback",
        "",
        f"```json\n{json.dumps(rollback, indent=2)}\n```",
        "",
    ]
    existing = sum_path.read_text(encoding="utf-8") if sum_path.is_file() else ""
    sum_path.write_text(existing.rstrip() + "\n" + "\n".join(post) + "\n", encoding="utf-8")
    print(f"updated {man_path} and {sum_path}")
    return 0


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    sub = ap.add_subparsers(dest="cmd", required=True)

    g = sub.add_parser("generate", help="Create manifest + summary from security-verdict.json")
    g.add_argument("--verdict", required=True)
    g.add_argument("--promotion-manifest", default="")
    g.add_argument("--output-dir", default="release-reports")
    g.add_argument("--environment-target", default="production-candidate")
    g.set_defaults(func=cmd_generate)

    a = sub.add_parser("append-deploy", help="Merge production-deployment-manifest.json into release manifest + summary")
    a.add_argument("--manifest", required=True)
    a.add_argument("--summary", required=True)
    a.add_argument("--production-manifest", required=True)
    a.set_defaults(func=cmd_append_deploy)

    args = ap.parse_args()
    return int(args.func(args))


if __name__ == "__main__":
    raise SystemExit(main())

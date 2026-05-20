#!/usr/bin/env python3
"""Write production deployment artifacts (from deploy-prod workflow):
production-deployment-manifest.json, production-deploy-evidence.json, and
production-release-evidence.json (+ .md) — an operator-facing rollup of commit, build/security ids,
images, security gate, backup/migration, hosts, health/smoke, and LKG/rollback fields.
"""
from __future__ import annotations

import json
import os
from pathlib import Path
from typing import Any


def s(name: str, default: str = "") -> str:
    return os.environ.get(name, default)


def _git_ref() -> str:
    b = s("SOURCE_BRANCH", "").strip()
    if b:
        return f"refs/heads/{b}"
    return ""


def _write_production_release_evidence(m: dict[str, Any], out_dir: Path) -> None:
    """Single JSON/MD summary for post-deploy audits (does not replace the full manifest)."""
    backup = m.get("db_backup_evidence")
    release = {
        "schema_version": "production-release-evidence-v1",
        "scope": "roll-up for operators; authoritative fields remain in production-deployment-manifest.json",
        "recorded_at_utc": m.get("completed_at_utc", ""),
        "source": {
            "commit_sha": m.get("source_commit_sha", ""),
            "commit_sha_note": m.get("source_commit_sha_note", ""),
            "branch": m.get("source_branch", ""),
            "ref": _git_ref(),
        },
        "github_actions": {
            "deploy_workflow_run_id": m.get("run_id", ""),
            "deploy_workflow_run_url": m.get("run_url", ""),
        },
        "build_and_release_chain": {
            "selected_build_run_id": m.get("selected_build_run_id", ""),
            "security_release_workflow_run_id": m.get("security_workflow_run_id", ""),
            "selected_build_actor": m.get("selected_build_actor", ""),
        },
        "container_images": {
            "app_image_ref": m.get("app_image_ref", ""),
            "app_image_digest": m.get("app_digest", ""),
            "goose_image_ref": m.get("goose_image_ref", ""),
            "goose_image_digest": m.get("goose_digest", ""),
        },
        "sbom": {
            "build_artifact_name": "sbom-reports",
            "lookup": "CycloneDX JSON files are published on the same Build and Push Images run as selected_build_run_id (download artifact sbom-reports).",
        },
        "image_signing_and_provenance": {
            "provenance_verification_mode": m.get("provenance_verification_mode", ""),
            "provenance_verification_status": m.get("provenance_verification_status", ""),
            "provenance_trust_class": m.get("provenance_trust_class", ""),
            "provenance_verdict": m.get("provenance_verdict", ""),
            "trust_decision": m.get("trust_decision", ""),
            "cosign_signing_artifact": "cosign-signing-evidence",
            "cosign_signing_lookup": "On the same selected_build_run_id, artifact cosign-signing-evidence contains signing policy results for digest-pinned app/goose images.",
        },
        "security_release_verdicts": {
            "release_gate_mode": s("RELEASE_GATE_MODE"),
            "release_gate_verdict": s("RELEASE_GATE_VERDICT"),
            "repo_release_verdict": s("REPO_RELEASE_VERDICT"),
            "provenance_release_verdict": s("PROVENANCE_RELEASE_VERDICT"),
            "security_verdict_age_seconds": m.get("security_verdict_age_seconds", ""),
            "security_verdict_max_age_hours": m.get("security_verdict_max_age_hours", ""),
            "security_verdict_generated_at_utc": m.get("security_verdict_generated_at_utc", ""),
        },
        "migration_and_backup": {
            "run_migration_requested": m.get("run_migration_requested", ""),
            "migration_rollback_policy": m.get("migration_rollback_policy", ""),
            "db_backup_evidence": backup,
        },
        "production_hosts": {
            "app_node_a": m.get("app_node_a_host", ""),
            "app_node_b": m.get("app_node_b_host", ""),
            "data_node": m.get("data_node_host", ""),
            "data_node_result": m.get("data_node_result", ""),
        },
        "health_checks_and_smoke": {
            "app_node_a_readiness_result": m.get("app_node_a_readiness_result", ""),
            "app_node_b_readiness_result": m.get("app_node_b_readiness_result", ""),
            "final_cluster_smoke_result": m.get("final_cluster_smoke_result", ""),
            "smoke_post_deploy_result": m.get("smoke_post_deploy_result", ""),
            "final_deployment_verdict": m.get("final_deployment_verdict", ""),
            "rollout_release_events": m.get("release_events_path", ""),
        },
        "rollback_and_last_known_good": {
            "rollback_available_before_deploy": m.get("rollback_available_before_deploy"),
            "previous_app_image_ref": m.get("previous_app_image_ref", ""),
            "previous_goose_image_ref": m.get("previous_goose_image_ref", ""),
            "previous_app_digest": m.get("previous_app_digest", ""),
            "previous_goose_digest": m.get("previous_goose_digest", ""),
            "previous_source_commit_sha": m.get("previous_source_commit_sha", ""),
            "previous_run_id": m.get("previous_run_id", ""),
            "previous_run_url": m.get("previous_run_url", ""),
            "auto_rollback_scope": m.get("auto_rollback_scope", ""),
            "rollback_attempted": m.get("rollback_attempted", ""),
            "rollback_result": m.get("rollback_result", ""),
        },
        "fleet_scale": {
            "fleet_scale_target": m.get("fleet_scale_target", ""),
            "storm_gate_required": m.get("storm_gate_required"),
            "storm_gate_result": m.get("storm_gate_result", ""),
            "storm_gate_bypassed": m.get("storm_gate_bypassed"),
            "storm_gate_evidence_path": m.get("storm_gate_evidence_path"),
        },
    }
    out_dir.mkdir(parents=True, exist_ok=True)
    (out_dir / "production-release-evidence.json").write_text(
        json.dumps(release, indent=2) + "\n",
        encoding="utf-8",
    )
    lines = [
        "# Production release evidence (rollup)",
        "",
        f"- **recorded_at_utc:** `{release['recorded_at_utc']}`",
        f"- **commit** `{release['source']['commit_sha']}` branch `{release['source']['branch']}` ref `{release['source']['ref']}`",
        f"- **build_run_id** `{release['build_and_release_chain']['selected_build_run_id']}` **security_release_run_id** `{release['build_and_release_chain']['security_release_workflow_run_id']}`",
        f"- **app** `{release['container_images']['app_image_ref']}` digest `{release['container_images']['app_image_digest']}`",
        f"- **goose** `{release['container_images']['goose_image_ref']}` digest `{release['container_images']['goose_image_digest']}`",
        f"- **SBOM:** artifact `{release['sbom']['build_artifact_name']}` on the Build run; cosign: `{release['image_signing_and_provenance']['cosign_signing_artifact']}` on the same run.",
        (
            f"- **Security gate:** mode `{release['security_release_verdicts']['release_gate_mode']}` "
            f"verdict `{release['security_release_verdicts']['release_gate_verdict']}` "
            f"repo `{release['security_release_verdicts']['repo_release_verdict']}` "
            f"provenance_release `{release['security_release_verdicts']['provenance_release_verdict']}`"
        ),
        f"- **Migration** requested `{release['migration_and_backup']['run_migration_requested']}`; backup block present: `{bool(backup)}`",
        f"- **Hosts** app-a `{release['production_hosts']['app_node_a']}` app-b `{release['production_hosts']['app_node_b']}` data `{release['production_hosts']['data_node']}`",
        f"- **Health/smoke** cluster `{release['health_checks_and_smoke']['final_cluster_smoke_result']}` post-deploy `{release['health_checks_and_smoke']['smoke_post_deploy_result']}` verdict `{release['health_checks_and_smoke']['final_deployment_verdict']}`",
        f"- **LKG** rollback_available_before `{release['rollback_and_last_known_good']['rollback_available_before_deploy']}` previous app `{release['rollback_and_last_known_good']['previous_app_image_ref']}`",
        "",
        "Machine-readable: `production-release-evidence.json` (next to this file).",
    ]
    (out_dir / "production-release-evidence.md").write_text("\n".join(lines) + "\n", encoding="utf-8")


def _backup_block() -> Any:
    if s("ACTION_MODE") != "deploy":
        return None
    prov, eid, cat, aref = (
        s("BACKUP_PROVIDER_FOR_MANIFEST"),
        s("BACKUP_EVIDENCE_ID_FOR_MANIFEST"),
        s("BACKUP_COMPLETED_AT_UTC_FOR_MANIFEST"),
        s("BACKUP_APPROVAL_REF_FOR_MANIFEST"),
    )
    rm = (s("RUN_MIGRATION_FOR_MANIFEST", "true") or "true").lower() in ("1", "true", "yes")
    if not rm:
        return {
            "schema_version": 1,
            "provider": "",
            "evidence_id": "",
            "completed_at_utc": "",
            "approval_ref": "",
            "note": "run_migration was false; image-only rollout (no goose Up on first app node)",
        }
    return {
        "schema_version": 1,
        "provider": prov,
        "evidence_id": eid,
        "completed_at_utc": cat,
        "approval_ref": aref,
    }


def main() -> None:
    storm_path = Path(s("STORM_JSON_PATH", "deployment-evidence/storm-evidence-embed.json"))
    if storm_path.exists() and storm_path.read_text(encoding="utf-8").strip():
        storm_evidence = json.loads(storm_path.read_text(encoding="utf-8"))
    else:
        storm_evidence = None

    gate_req = s("STORM_Q_REQUIRED", "false") == "true"
    gate_byp = s("STORM_Q_BYPASSED", "false") == "true"
    gate_bypass_reason = json.loads(s("STORM_BYPASS_JSON_SNIPPET", "null") or "null")
    gate_evid = json.loads(s("STORM_EVID_PATH_JSON_SNIPPET", "null") or "null")
    rollback_before = s("ROLLBACK_AVAIL_FOR_JSON", "false") == "true"

    m = {
        "schema_version": 1,
        "action_mode": s("ACTION_MODE"),
        "environment": "production",
        "scope_note": "this production workflow attests only this production rollout and its release/security gates; it does not attest staging deployment or restore-drill controls",
        "completed_at_utc": s("COMPLETED_AT_FOR_MANIFEST"),
        "deployed_at_utc": s("COMPLETED_AT_FOR_MANIFEST"),
        "rollback_available_before_deploy": rollback_before,
        "migration_rollback_policy": "never_automatic",
        "auto_rollback_scope": "app_and_goose_images_only",
        "auto_rollback_note": "Automatic rollback redeploys prior digest-pinned app and goose images via rollback_app_node.sh without RUN_MIGRATION; it does not reverse database migrations (no goose down).",
        "run_migration_requested": s("RUN_MIGRATION_FOR_MANIFEST"),
        "db_backup_evidence": _backup_block(),
        "source_commit_sha": s("DEPLOY_SOURCE_SHA"),
        "source_commit_sha_note": s("DEPLOY_SOURCE_SHA_NOTE"),
        "source_branch": s("SOURCE_BRANCH"),
        "release_tag": s("RELEASE_TAG"),
        "release_label": s("RELEASE_TAG"),
        "resolved_release_label": s("RESOLVED_RELEASE_LABEL"),
        "selected_build_run_id": s("SELECTED_BUILD_RUN_ID"),
        "selected_build_actor": s("SELECTED_BUILD_ACTOR"),
        "app_image_ref": s("APP_IMAGE_REF"),
        "app_digest": s("MANIFEST_APP_DIGEST"),
        "goose_image_ref": s("GOOSE_IMAGE_REF"),
        "goose_digest": s("MANIFEST_GOOSE_DIGEST"),
        "previous_action_mode": s("PREVIOUS_ACTION_MODE"),
        "previous_actor": s("PREVIOUS_ACTOR"),
        "previous_app_image_ref": s("PREVIOUS_APP_IMAGE_REF"),
        "previous_app_digest": s("PREVIOUS_APP_DIGEST"),
        "previous_goose_image_ref": s("PREVIOUS_GOOSE_IMAGE_REF"),
        "previous_goose_digest": s("PREVIOUS_GOOSE_DIGEST"),
        "previous_source_branch": s("MANIFEST_PREV_SOURCE_BRANCH"),
        "previous_source_commit_sha": s("MANIFEST_PREV_SOURCE_COMMIT_SHA"),
        "previous_deployed_at_utc": s("PREVIOUS_DEPLOYED_AT_UTC"),
        "previous_run_id": s("PREVIOUS_RUN_ID"),
        "previous_run_number": s("PREVIOUS_RUN_NUMBER"),
        "previous_run_url": s("PREVIOUS_RUN_URL"),
        "actor": s("ACTOR"),
        "trigger": s("EVENT_NAME"),
        "run_id": s("GITHUB_RUN_ID"),
        "run_number": s("GITHUB_RUN_NUMBER"),
        "run_url": s("RUN_URL_FOR_MANIFEST"),
        "deploy_transport": s("DEPLOY_TRANSPORT"),
        "auth_mode": s("AUTH_MODE"),
        "trust_decision": s("TRUST_DECISION"),
        "provenance_verification_mode": s("PROVENANCE_MODE"),
        "provenance_verification_status": s("PROVENANCE_STATUS"),
        "provenance_trust_class": s("PROVENANCE_TRUST_CLASS"),
        "provenance_verdict": s("PROVENANCE_VERDICT"),
        "security_verdict_age_seconds": s("SECURITY_VERDICT_AGE_SECONDS"),
        "security_verdict_max_age_hours": s("SECURITY_VERDICT_MAX_AGE_HOURS"),
        "security_workflow_run_id": s("SECURITY_WORKFLOW_RUN_ID"),
        "security_verdict_generated_at_utc": s("SECURITY_VERDICT_GENERATED_AT_UTC"),
        "deploy_data_node": s("DEPLOY_DATA_NODE"),
        "data_node_host": s("DATA_NODE_HOST"),
        "app_node_a_host": s("APP_NODE_A_HOST"),
        "app_node_b_host": s("APP_NODE_B_HOST"),
        "data_node_result": s("data_node_result"),
        "app_node_a_result": s("app_node_a_result"),
        "app_node_a_readiness_result": s("app_node_a_readiness_result"),
        "app_node_a_smoke_result": s("app_node_a_smoke_result"),
        "app_node_b_result": s("app_node_b_result"),
        "app_node_b_readiness_result": s("app_node_b_readiness_result"),
        "final_cluster_smoke_result": s("final_cluster_smoke_result"),
        "smoke_post_deploy_result": s("smoke_post_deploy_result"),
        "rollback_attempted": s("ROLLBACK_ATTEMPTED"),
        "rollback_result": s("ROLLBACK_RESULT"),
        "final_result": s("FINAL_RESULT"),
        "final_deployment_verdict": s("final_deployment_verdict"),
        "release_events_path": s("release_events_path"),
        "fleet_scale_target": s("FLEET_SCALE_TARGET"),
        "storm_gate_required": gate_req,
        "storm_gate_result": s("STORM_GATE_RESULT"),
        "storm_gate_evidence_path": gate_evid,
        "storm_gate_bypassed": gate_byp,
        "storm_gate_bypass_reason": gate_bypass_reason,
        "storm_evidence": storm_evidence,
    }
    out_dir = Path("deployment-evidence")
    out_dir.mkdir(parents=True, exist_ok=True)
    (out_dir / "production-deployment-manifest.json").write_text(
        json.dumps(m, indent=2) + "\n",
        encoding="utf-8",
    )

    e = {
        "scope_note": "this production workflow attests only this production rollout and its release/security gates; it does not attest staging deployment or restore-drill controls",
        "run_migration_requested": s("RUN_MIGRATION_FOR_MANIFEST"),
        "db_backup_evidence": _backup_block(),
        "release_label": s("RELEASE_TAG"),
        "resolved_release_label": s("RESOLVED_RELEASE_LABEL"),
        "app_image_ref": s("APP_IMAGE_REF"),
        "app_digest": s("MANIFEST_APP_DIGEST"),
        "goose_image_ref": s("GOOSE_IMAGE_REF"),
        "goose_digest": s("MANIFEST_GOOSE_DIGEST"),
        "trust_decision": s("TRUST_DECISION"),
        "provenance_trust_class": s("PROVENANCE_TRUST_CLASS"),
        "provenance_verdict": s("PROVENANCE_VERDICT"),
        "security_verdict_age_seconds": s("SECURITY_VERDICT_AGE_SECONDS"),
        "security_verdict_max_age_hours": s("SECURITY_VERDICT_MAX_AGE_HOURS"),
        "security_workflow_run_id": s("SECURITY_WORKFLOW_RUN_ID"),
        "data_node_result": s("data_node_result"),
        "app_node_a_result": s("app_node_a_result"),
        "app_node_a_readiness_result": s("app_node_a_readiness_result"),
        "app_node_a_smoke_result": s("app_node_a_smoke_result"),
        "app_node_b_result": s("app_node_b_result"),
        "app_node_b_readiness_result": s("app_node_b_readiness_result"),
        "final_cluster_smoke_result": s("final_cluster_smoke_result"),
        "smoke_post_deploy_result": s("smoke_post_deploy_result"),
        "rollback_attempted": s("ROLLBACK_ATTEMPTED"),
        "rollback_result": s("ROLLBACK_RESULT"),
        "final_result": s("FINAL_RESULT"),
        "final_deployment_verdict": s("final_deployment_verdict"),
        "release_events_path": s("release_events_path"),
        "fleet_scale_target": s("FLEET_SCALE_TARGET"),
        "storm_gate_required": gate_req,
        "storm_gate_result": s("STORM_GATE_RESULT"),
        "storm_gate_evidence_path": gate_evid,
        "storm_gate_bypassed": gate_byp,
        "storm_gate_bypass_reason": gate_bypass_reason,
        "storm_evidence": storm_evidence,
        "production_release_evidence_json": "production-release-evidence.json",
        "production_release_evidence_markdown": "production-release-evidence.md",
    }
    (out_dir / "production-deploy-evidence.json").write_text(json.dumps(e, indent=2) + "\n", encoding="utf-8")
    _write_production_release_evidence(m, out_dir)


if __name__ == "__main__":
    main()

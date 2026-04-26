#!/usr/bin/env python3
"""Write production-deployment-manifest.json and production-deploy-evidence.json (from deploy-prod workflow)."""
from __future__ import annotations

import json
import os
from pathlib import Path


def s(name: str, default: str = "") -> str:
    return os.environ.get(name, default)


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
    }
    (out_dir / "production-deploy-evidence.json").write_text(json.dumps(e, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()

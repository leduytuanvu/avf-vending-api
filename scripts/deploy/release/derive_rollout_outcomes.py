#!/usr/bin/env python3
"""Derive rollout outcome env lines for production deployment manifest (from deploy-prod workflow)."""
from __future__ import annotations

import json
import os
from pathlib import Path


def main() -> None:
    events = []
    events_path = Path("deployment-evidence/release-events.jsonl")
    if events_path.exists():
        for line in events_path.read_text(encoding="utf-8").splitlines():
            if line.strip():
                events.append(json.loads(line))

    def latest_phase_status(component: str, target: str, phase: str, default: str) -> str:
        for event in reversed(events):
            if (
                event.get("component") == component
                and event.get("target") == target
                and event.get("phase") == phase
            ):
                return event.get("status", default)
        return default

    def normalize_step_outcome(value: str, default: str = "not-run") -> str:
        mapping = {
            "success": "pass",
            "failure": "fail",
            "cancelled": "cancelled",
            "skipped": "skipped",
        }
        return mapping.get(value, default)

    def smoke_status(path_str: str, default: str) -> str:
        path = Path(path_str)
        if not path.exists():
            return default
        try:
            payload = json.loads(path.read_text(encoding="utf-8"))
        except Exception:
            return "malformed"
        return payload.get("overall_status", default)

    action_mode = os.environ.get("ACTION_MODE", "")
    deploy_data_node = os.environ.get("DEPLOY_DATA_NODE", "false")
    app_node_a_host = os.environ.get("APP_NODE_A_HOST", "")
    app_node_b_host = os.environ.get("APP_NODE_B_HOST", "")

    derived = {
        "data_node_result": normalize_step_outcome(os.environ.get("DATA_NODE_OUTCOME", "skipped"), "skipped"),
        "app_node_a_result": normalize_step_outcome(os.environ.get("APP_NODE_A_ROLLOUT_OUTCOME", "skipped"), "skipped"),
        "app_node_a_readiness_result": latest_phase_status("app-node", app_node_a_host, "readiness", "not-run"),
        "app_node_a_smoke_result": smoke_status(
            "deployment-evidence/smoke-app-node-a.json",
            "skipped" if action_mode != "deploy" else "not-run",
        ),
        "app_node_b_result": normalize_step_outcome(os.environ.get("APP_NODE_B_ROLLOUT_OUTCOME", "skipped"), "skipped"),
        "app_node_b_readiness_result": latest_phase_status("app-node", app_node_b_host, "readiness", "not-run"),
        "app_node_b_smoke_result": smoke_status(
            "deployment-evidence/smoke-app-node-b.json",
            "skipped" if action_mode != "deploy" else "not-run",
        ),
        "final_cluster_smoke_result": smoke_status(
            "deployment-evidence/smoke-cluster-final.json",
            "skipped"
            if action_mode != "deploy"
            else normalize_step_outcome(os.environ.get("FINAL_CLUSTER_SMOKE_STEP_OUTCOME", "skipped"), "not-run"),
        ),
        "smoke_post_deploy_result": smoke_status(
            "smoke-reports/smoke-test.json",
            "skipped"
            if action_mode != "deploy"
            else normalize_step_outcome(os.environ.get("POST_DEPLOY_SMOKE_OUTCOME", "skipped"), "not-run"),
        ),
        "rollback_attempted": os.environ.get("ROLLBACK_ATTEMPTED", "false"),
        "rollback_result": os.environ.get("ROLLBACK_RESULT", "not-attempted"),
        "final_deployment_verdict": "pass" if os.environ.get("FINAL_RESULT") == "success" else "fail",
        "release_events_path": str(events_path if events_path.exists() else Path("deployment-evidence/release-events.jsonl")),
    }
    rollout_m = os.environ.get("ROLLOUT_MODE_EFFECTIVE", "single-host")
    canary_is_b = os.environ.get("CANARY_IS_B", "false") == "true"
    cw = os.environ.get("CANARY_WAVE_OUTCOME", "skipped")
    sw = os.environ.get("STABLE_WAVE_OUTCOME", "skipped")
    scj = "deployment-evidence/smoke-canary-wave.json"
    if rollout_m == "canary" and action_mode == "deploy":
        if canary_is_b:
            derived["app_node_b_result"] = normalize_step_outcome(cw, "skipped")
            derived["app_node_a_result"] = normalize_step_outcome(sw, "skipped")
            derived["app_node_b_smoke_result"] = smoke_status(scj, "not-run")
            derived["app_node_a_smoke_result"] = smoke_status(
                "deployment-evidence/smoke-app-node-a.json",
                "not-run",
            )
        else:
            derived["app_node_a_result"] = normalize_step_outcome(cw, "skipped")
            derived["app_node_b_result"] = normalize_step_outcome(sw, "skipped")
            derived["app_node_a_smoke_result"] = smoke_status(scj, "not-run")
            derived["app_node_b_smoke_result"] = smoke_status(
                "deployment-evidence/smoke-app-node-b.json",
                "not-run",
            )
    if deploy_data_node != "true":
        derived["data_node_result"] = "skipped"

    for key, value in derived.items():
        print("%s=%s" % (key, value))


if __name__ == "__main__":
    main()

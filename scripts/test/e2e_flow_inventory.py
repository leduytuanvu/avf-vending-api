#!/usr/bin/env python3
"""
Maps required P0/P1 business flows to shell E2E scenarios + Go integration tests.
Writes reports/test/e2e-flow-coverage.{json,md} and ensures reports/test/e2e-evidence/README.md exists.
"""

from __future__ import annotations

import json
import sys
from datetime import datetime, timezone
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]


FLOWS = [
    {
        "flow_id": "admin_auth",
        "priority": "P0",
        "scenario_scripts": ["tests/e2e/scenarios/01_web_admin_setup.sh", "tests/e2e/scenarios/12_web_admin_catalog_ops.sh"],
        "go_tests": ["internal/e2e/correctness/auth_p06_integration_test.go", "internal/platform/auth/*_test.go"],
        "coverage_status": "scripted",
        "blocked_reason": "Needs local API + seeded admin (Docker) for live evidence.",
    },
    {
        "flow_id": "machine_lifecycle",
        "priority": "P0",
        "scenario_scripts": [
            "tests/e2e/scenarios/02_machine_activation_bootstrap_rest.sh",
            "tests/e2e/scenarios/20_grpc_machine_auth.sh",
            "tests/e2e/scenarios/40_e2e_first_boot.sh",
        ],
        "go_tests": ["internal/e2e/correctness/machine_auth_integration_test.go"],
        "coverage_status": "scripted",
        "blocked_reason": "Hardware not required; local stack required.",
    },
    {
        "flow_id": "catalog_inventory_slots",
        "priority": "P0",
        "scenario_scripts": [
            "tests/e2e/scenarios/12_web_admin_catalog_ops.sh",
            "tests/e2e/scenarios/11_web_admin_inventory_ops.sh",
            "tests/e2e/scenarios/46_e2e_inventory_restock_adjustment.sh",
        ],
        "go_tests": ["internal/e2e/correctness/vend_inventory_integration_test.go"],
        "coverage_status": "scripted",
        "blocked_reason": "",
    },
    {
        "flow_id": "cash_sale_machine",
        "priority": "P0",
        "scenario_scripts": [
            "tests/e2e/scenarios/04_cash_sale_success_rest.sh",
            "tests/e2e/scenarios/41_e2e_cash_sale_success.sh",
            "tests/e2e/scenarios/22_grpc_commerce_cash_sale.sh",
        ],
        "go_tests": ["internal/e2e/correctness/vend_inventory_integration_test.go"],
        "coverage_status": "scripted",
        "blocked_reason": "",
    },
    {
        "flow_id": "dispense_failure_refund",
        "priority": "P0",
        "scenario_scripts": ["tests/e2e/scenarios/06_vend_failure_refund_rest.sh", "tests/e2e/scenarios/43_e2e_vend_failure_refund.sh"],
        "go_tests": ["internal/e2e/correctness/vend_inventory_integration_test.go"],
        "coverage_status": "scripted",
        "blocked_reason": "",
    },
    {
        "flow_id": "online_payment_webhook",
        "priority": "P0",
        "scenario_scripts": ["tests/e2e/scenarios/42_e2e_qr_payment_success_mock.sh"],
        "go_tests": [
            "internal/e2e/correctness/payment_webhook_integration_test.go",
            "internal/e2e/correctness/payment_webhook_http_integration_test.go",
        ],
        "coverage_status": "scripted",
        "blocked_reason": "Live PSP signature tests require sandbox keys (not exercised here).",
    },
    {
        "flow_id": "remote_command_ack",
        "priority": "P0",
        "scenario_scripts": ["tests/e2e/scenarios/45_e2e_remote_command_ack.sh", "tests/e2e/scenarios/32_mqtt_command_ack.sh"],
        "go_tests": ["internal/e2e/correctness/mqtt_command_integration_test.go"],
        "coverage_status": "scripted",
        "blocked_reason": "",
    },
    {
        "flow_id": "diagnostics",
        "priority": "P1",
        "scenario_scripts": ["tests/e2e/scenarios/24_grpc_command_update_status.sh", "tests/e2e/scenarios/23_grpc_inventory_telemetry_offline.sh"],
        "go_tests": ["internal/e2e/correctness/mqtt_command_integration_test.go"],
        "coverage_status": "partial",
        "blocked_reason": "Some device fault codes need hardware simulators for full matrix.",
    },
    {
        "flow_id": "offline_idempotency_outbox",
        "priority": "P0",
        "scenario_scripts": ["tests/e2e/scenarios/08_offline_replay_rest.sh", "tests/e2e/scenarios/44_e2e_offline_replay.sh"],
        "go_tests": [
            "internal/e2e/correctness/machine_idempotency_integration_test.go",
            "internal/app/outbox/*_test.go",
        ],
        "coverage_status": "scripted",
        "blocked_reason": "",
    },
    {
        "flow_id": "media_object_storage",
        "priority": "P1",
        "scenario_scripts": ["tests/e2e/scenarios/03_catalog_media_sync_rest.sh", "tests/e2e/scenarios/21_grpc_bootstrap_catalog_media.sh"],
        "go_tests": ["internal/app/mediaadmin/*_test.go", "internal/platform/objectstore/*_test.go"],
        "coverage_status": "partial",
        "blocked_reason": "Real S3 requires localstack/minio profile; unit tests mock object store.",
    },
]


def main() -> int:
    ts = datetime.now(timezone.utc).isoformat()
    out_dir = REPO_ROOT / "reports" / "test"
    ev_dir = out_dir / "e2e-evidence"
    out_dir.mkdir(parents=True, exist_ok=True)
    ev_dir.mkdir(parents=True, exist_ok=True)
    readme = ev_dir / "README.md"
    if not readme.exists():
        readme.write_text(
            "# E2E evidence directory\n\n"
            "Populated by `tests/e2e/run-all-local.sh` (run directory under `.e2e-runs/run-*`) "
            "and copied into this folder by `scripts/test/run-full-backend-test-audit.sh` when enabled.\n",
            encoding="utf8",
        )

    payload = {
        "summary": {
            "generated_at": ts,
            "total_flows": len(FLOWS),
            "note": "Executable proof requires Docker + local API + broker per docs/testing/local-testing-guide.md.",
        },
        "flows": FLOWS,
    }
    jp = out_dir / "e2e-flow-coverage.json"
    jp.write_text(json.dumps(payload, indent=2), encoding="utf8")

    mp = out_dir / "e2e-flow-coverage.md"
    with mp.open("w", encoding="utf8") as f:
        f.write("# E2E business flow coverage map\n\n")
        f.write(f"- Generated `{ts}`\n")
        f.write("- Orchestrator: `tests/e2e/run-all-local.sh`\n")
        f.write("- Windows helper: `scripts/local/run-local-e2e.ps1` (Git Bash)\n\n")
        for fl in FLOWS:
            f.write(f"## {fl['flow_id']} ({fl['priority']})\n")
            f.write(f"- Status: **{fl['coverage_status']}**\n")
            f.write(f"- Scenarios: {', '.join('`'+s+'`' for s in fl['scenario_scripts'])}\n")
            f.write(f"- Go integration: {', '.join('`'+g+'`' for g in fl['go_tests'])}\n")
            if fl.get("blocked_reason"):
                f.write(f"- Block/hardware note: {fl['blocked_reason']}\n")
            f.write("\n")

    print(f"Wrote {jp}, {mp}, ensured {readme}")
    return 0


if __name__ == "__main__":
    sys.exit(main())

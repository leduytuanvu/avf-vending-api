#!/usr/bin/env python3
"""
MQTT contract inventory from canonical doc + codebase pointers. Writes reports/test/mqtt-coverage.{json,md}.
Executable tests: internal/e2e/correctness/mqtt_command_integration_test.go (needs TEST_DATABASE_URL + broker mocks/fixtures).
Shell E2E: tests/e2e/scenarios/30_*.sh, 31_*.sh, 32_*.sh (needs local Mosquitto).
"""

from __future__ import annotations

import json
import sys
from datetime import datetime, timezone
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]


def main() -> int:
    ts = datetime.now(timezone.utc).isoformat()
    out_dir = REPO_ROOT / "reports" / "test"
    out_dir.mkdir(parents=True, exist_ok=True)

    flows = [
        {
            "name": "command_publish",
            "topics": ["{prefix}/{machineId}/commands/dispatch (legacy)", "{prefix}/machines/{machineId}/commands (enterprise)"],
            "direction": "API→device",
            "coverage_status": "scripted",
            "priority": "P0",
            "existing_tests": [
                "internal/e2e/correctness/mqtt_command_integration_test.go",
                "internal/platform/mqtt/topics_test.go",
                "tests/e2e/scenarios/32_mqtt_command_ack.sh",
                "tests/e2e/scenarios/45_e2e_remote_command_ack.sh",
            ],
            "notes": "Broker reachability required for harness evidence.",
        },
        {
            "name": "command_ack",
            "topics": ["{prefix}/{machineId}/commands/ack", "commands/receipt (legacy alias)"],
            "direction": "device→cloud",
            "coverage_status": "scripted",
            "priority": "P0",
            "existing_tests": [
                "internal/e2e/correctness/mqtt_command_integration_test.go",
                "tests/e2e/scenarios/32_mqtt_command_ack.sh",
            ],
            "notes": "Correlation / idempotency validated in integration layer.",
        },
        {
            "name": "telemetry_ingest",
            "topics": ["{prefix}/+/telemetry", "telemetry/snapshot", "telemetry/incident"],
            "direction": "device→cloud",
            "coverage_status": "partial",
            "priority": "P1",
            "existing_tests": [
                "internal/platform/mqtt/router.go",
                "tests/e2e/scenarios/31_mqtt_telemetry_publish.sh",
            ],
            "notes": "Wildcard subscriber patterns documented in mqtt-contract.md.",
        },
        {
            "name": "shadow_reported_desired",
            "topics": ["shadow/reported", "shadow/desired"],
            "direction": "bidirectional",
            "coverage_status": "partial",
            "priority": "P2",
            "existing_tests": ["docs/api/mqtt-contract.md"],
            "notes": "Router dispatch; dedicated E2E scenario may be partial.",
        },
        {
            "name": "vend_events",
            "topics": ["events/vend", "events/cash", "events/inventory"],
            "direction": "device→cloud",
            "coverage_status": "partial",
            "priority": "P1",
            "existing_tests": ["internal/platform/mqtt/router.go"],
            "notes": "Captured by mqtt-ingest.",
        },
    ]

    summary = {
        "generated_at": ts,
        "doc_source": "docs/api/mqtt-contract.md",
        "code_source": "internal/platform/mqtt/",
        "total_flows": len(flows),
        "local_broker_required": True,
        "blocked_without_docker_note": True,
    }
    jp = out_dir / "mqtt-coverage.json"
    jp.write_text(json.dumps({"summary": summary, "flows": flows}, indent=2), encoding="utf8")

    mp = out_dir / "mqtt-coverage.md"
    with mp.open("w", encoding="utf8") as f:
        f.write("# MQTT coverage inventory\n\n")
        f.write(f"- Generated `{ts}`\n")
        f.write("- Authoritative topic layout: `docs/api/mqtt-contract.md`\n")
        f.write("- Implementation: `internal/platform/mqtt/` (publisher, subscriber, router)\n\n")
        for fl in flows:
            f.write(f"## {fl['name']} ({fl['priority']})\n")
            f.write(f"- Topics: {', '.join(fl['topics'])}\n")
            f.write(f"- Status: **{fl['coverage_status']}**\n")
            f.write(f"- Tests: `{', '.join(fl['existing_tests'])}`\n")
            f.write(f"- Notes: {fl['notes']}\n\n")

    print(f"Wrote {jp} and {mp}")
    return 0


if __name__ == "__main__":
    sys.exit(main())

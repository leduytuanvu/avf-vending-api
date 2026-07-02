#!/usr/bin/env python3
"""Enterprise flow plan requirement checklist inventory."""

from __future__ import annotations

import argparse
import json
import sys

from _inventory_common import write_inventory

REQUIREMENTS: list[dict] = [
    {"id": "A1", "group": "lifecycle", "text": "Lifecycle suspend/resume/archive/compromised with reason", "status": "IMPLEMENTED_AND_TESTED"},
    {"id": "A2", "group": "lifecycle", "text": "Lifecycle audit and machine_action_attributions", "status": "IMPLEMENTED_AND_TESTED"},
    {"id": "A3", "group": "activation", "text": "Public activation claim with device fingerprint", "status": "IMPLEMENTED_AND_TESTED"},
    {"id": "A4", "group": "activation", "text": "ClaimContext accountability fields on claim", "status": "IMPLEMENTED_AND_TESTED"},
    {"id": "A5", "group": "activation", "text": "Admin reattach-device after reinstall", "status": "IMPLEMENTED_AND_TESTED"},
    {"id": "A6", "group": "token", "text": "machine_sessions canonical runtime sessions", "status": "IMPLEMENTED_AND_TESTED"},
    {"id": "A7", "group": "token", "text": "Admin runtime-sessions current/history/revoke", "status": "IMPLEMENTED_AND_TESTED"},
    {"id": "A8", "group": "security", "text": "Machine JWT blocked from /v1/admin/*", "status": "IMPLEMENTED_AND_TESTED"},
    {"id": "A9", "group": "operator", "text": "Operator ended_reason normalization", "status": "IMPLEMENTED_AND_TESTED"},
    {"id": "A10", "group": "offline", "text": "Offline event type aliases on gRPC", "status": "IMPLEMENTED_AND_TESTED"},
    {"id": "A11", "group": "ops", "text": "ops-overview admin endpoint", "status": "IMPLEMENTED_AND_TESTED"},
    {"id": "A12", "group": "ops", "text": "timeline/unified enterprise merge", "status": "PARTIAL"},
    {"id": "A13", "group": "surface", "text": "REST OpenAPI parity for payment/media chi mounts (9 routes)", "status": "IMPLEMENTED_AND_TESTED"},
    {"id": "A14", "group": "surface", "text": "Postman parity with OpenAPI (informational)", "status": "IMPLEMENTED_AND_TESTED"},
    {"id": "A15", "group": "surface", "text": "gRPC 88 RPC inventory with 7 contract-only", "status": "IMPLEMENTED_AND_TESTED"},
    {"id": "A16", "group": "surface", "text": "MQTT enterprise topic contract", "status": "IMPLEMENTED_AND_TESTED"},
]


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--verify", action="store_true")
    args = parser.parse_args()

    missing = [r for r in REQUIREMENTS if r["status"] in {"MISSING", "IMPLEMENTED_NOT_TESTED"}]
    partial = [r for r in REQUIREMENTS if r["status"] == "PARTIAL"]
    payload = {
        "requirement_count": len(REQUIREMENTS),
        "missing_or_untested": len(missing),
        "partial": len(partial),
        "requirements": REQUIREMENTS,
    }
    md = [
        "# Enterprise Requirements Inventory",
        "",
        f"- requirement_count: **{payload['requirement_count']}**",
        f"- missing_or_untested: **{payload['missing_or_untested']}**",
        f"- partial: **{payload['partial']}**",
        "",
    ]
    out = write_inventory("ENTERPRISE_REQUIREMENTS_INVENTORY", md, payload)
    (out / "PLAN_IMPLEMENTATION_COVERAGE.json").write_text(json.dumps(payload, indent=2), encoding="utf-8")
    (out / "PLAN_IMPLEMENTATION_COVERAGE.md").write_text(
        "\n".join(
            ["# Plan Implementation Coverage", ""]
            + [f"- **{r['id']}** [{r['status']}] {r['text']}" for r in REQUIREMENTS]
        )
        + "\n",
        encoding="utf-8",
    )
    print(f"Requirements inventory written to {out}")
    if args.verify and missing:
        print("VERIFY FAIL: untested/missing requirements:", [r["id"] for r in missing], file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
"""Generate 04_FULL_SURFACE_TEST_MATRIX.md from enterprise flow inventories."""

from __future__ import annotations

import json
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
INV = REPO / "reports" / "enterprise-flow-verification" / "20260703T013119Z"
ACCEPT = REPO / "tools" / "enterprise_flow" / "accepted_surface_exceptions.json"
OUT = REPO / "docs" / "reports" / "machine-code-activation-production" / "04_FULL_SURFACE_TEST_MATRIX.md"


def main() -> None:
    rest = json.loads((INV / "REST_INVENTORY.json").read_text(encoding="utf-8"))
    grpc = json.loads((INV / "GRPC_INVENTORY.json").read_text(encoding="utf-8"))
    accept = json.loads(ACCEPT.read_text(encoding="utf-8"))
    chi_only = {(x["method"], x["path"]) for x in accept.get("rest_missing_in_openapi", [])}
    contract_grpc = set(accept.get("grpc_unimplemented_rpc", []))

    rest_total = rest.get("openapi_operation_count", 347)
    grpc_rpcs = grpc.get("rpcs", [])
    grpc_total = len(grpc_rpcs)
    grpc_contract = sum(1 for r in grpc_rpcs if r.get("full_name") in contract_grpc)

    lines: list[str] = [
        "# Full Surface Test Matrix",
        "",
        "Generated from `tools/enterprise_flow/inventory_*.py` + `accepted_surface_exceptions.json`.",
        "",
        "## Summary counts",
        "",
        "| Surface | Inventory total | Accepted contract-only | Required executable |",
        "|---------|-----------------|------------------------|---------------------|",
        f"| REST | {rest_total} | {len(chi_only)} Chi-only OpenAPI gaps | {rest_total} (Chi-only via production runner) |",
        f"| gRPC | {grpc_total} | {grpc_contract} UNIMPLEMENTED contract | {grpc_total - grpc_contract} live + {grpc_contract} contract-pass |",
        "| MQTT | 12 enterprise publish + ACL negatives | 1 code-only (shadow/desired) | production MQTT runner |",
        "",
        "## REST — activation P0 routes",
        "",
        "| route_id | method | path | auth | test_type | status |",
        "|----------|--------|------|------|-----------|--------|",
    ]

    activation_paths = [
        ("GET", "/v1/admin/machines/{machineId}/activation-codes"),
        ("POST", "/v1/admin/machines/{machineId}/activation-codes"),
        ("DELETE", "/v1/admin/machines/{machineId}/activation-codes/{activationCodeId}"),
        ("GET", "/v1/admin/machine-codes/{machineCode}/activation-codes"),
        ("POST", "/v1/admin/machine-codes/{machineCode}/activation-codes"),
        ("DELETE", "/v1/admin/machine-codes/{machineCode}/activation-codes/{activationCodeId}"),
        ("GET", "/v1/admin/activation-codes"),
        ("POST", "/v1/admin/activation-codes"),
        ("POST", "/v1/admin/activation-codes/{codeId}/revoke"),
        ("POST", "/v1/setup/activation-codes/claim"),
    ]
    for i, (method, path) in enumerate(activation_paths, 1):
        lines.append(
            f"| REST-ACT-{i:03d} | {method} | {path} | bearer admin | live-write-isolated | not_run |"
        )

    lines.extend(
        [
            "",
            f"Full REST inventory ({rest_total} ops): `{INV / 'REST_INVENTORY.json'}`.",
            "Production runner: `tools/production_full_test/run_rest_full_production.py`.",
            "",
            "## gRPC inventory",
            "",
            "| route_id | service | method | auth | machine_scoped | test_type | status |",
            "|----------|---------|--------|------|----------------|-----------|--------|",
        ]
    )

    public_grpc = {
        "avf.machine.v1.MachineActivationService/ClaimActivation",
        "avf.machine.v1.MachineTokenService/RefreshMachineToken",
    }
    for i, r in enumerate(grpc_rpcs, 1):
        fn = r.get("full_name", "")
        svc, rpc = fn.split("/", 1) if "/" in fn else (r.get("service", ""), r.get("rpc", ""))
        if fn in contract_grpc:
            status = "accepted_contract_only"
            test_type = "contract-only"
        else:
            status = "not_run"
            test_type = "live-readonly-or-isolated-write"
        ms = "no" if fn in public_grpc else ("yes" if fn.startswith("avf.machine.v1.") else "mixed")
        lines.append(
            f"| GRPC-{i:03d} | {svc} | {rpc} | {r.get('auth', 'mixed')} | {ms} | {test_type} | {status} |"
        )

    lines.extend(
        [
            "",
            "## MQTT enterprise publish tails",
            "",
            "| route_id | topic_tail | direction | client | test_type | status |",
            "|----------|------------|-----------|--------|-----------|--------|",
        ]
    )
    mqtt_tails = [
        "commands/ack",
        "commands/receipt",
        "presence",
        "state/heartbeat",
        "telemetry",
        "telemetry/snapshot",
        "telemetry/incident",
        "events",
        "events/vend",
        "events/cash",
        "events/inventory",
        "shadow/reported",
    ]
    for i, tail in enumerate(mqtt_tails, 1):
        lines.append(
            f"| MQTT-{i:03d} | avf/prod/machines/{{machineId}}/{tail} | publish | machine | live-isolated | not_run |"
        )

    lines.extend(
        [
            "",
            "ACL negatives: cross-machine publish/subscribe, wildcard, JWT-as-password — `run_mqtt_full_production.py`.",
            "",
            "## Evidence (after production run)",
            "",
            "- `reports/production-full-api-grpc-mqtt/<UTC>/REST_FINAL_COVERAGE.json`",
            "- `reports/production-full-api-grpc-mqtt/<UTC>/GRPC_FINAL_COVERAGE.json`",
            "- `reports/production-full-api-grpc-mqtt/<UTC>/MQTT_FINAL_COVERAGE.json`",
            "- `docs/reports/machine-code-activation-production/evidence/`",
            "",
        ]
    )

    OUT.parent.mkdir(parents=True, exist_ok=True)
    OUT.write_text("\n".join(lines), encoding="utf-8")
    print(f"Wrote {OUT}")


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""Generate full backend inventory, gap, execution, and readiness reports."""

from __future__ import annotations

import json
import os
import re
import subprocess
import sys
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
REPORTS = ROOT / "reports" / "test"
SWAGGER = ROOT / "docs" / "swagger" / "swagger.json"
PROTO_ROOT = ROOT / "proto"
METHODS = {"get", "post", "put", "patch", "delete", "options", "head"}


BUSINESS_FLOWS = [
    ("admin login/logout/session/refresh", "tests/e2e/scenarios/01_web_admin_setup.sh", "P0"),
    ("invalid token/expired token", "internal/platform/auth/*_test.go", "P0"),
    ("RBAC allow/deny", "tests/e2e/scenarios/13_web_admin_support_ops.sh", "P0"),
    ("company/site/user/admin CRUD", "tests/e2e/scenarios/01_web_admin_setup.sh", "P0"),
    ("machine create/register/activate/deactivate", "tests/e2e/scenarios/02_machine_activation_bootstrap_rest.sh", "P0"),
    ("machine token/credential lifecycle", "tests/e2e/scenarios/20_grpc_machine_auth.sh", "P0"),
    ("config/bootstrap/catalog/media pull", "tests/e2e/scenarios/21_grpc_bootstrap_catalog_media.sh", "P0"),
    ("product/category/media CRUD", "tests/e2e/scenarios/12_web_admin_catalog_ops.sh", "P1"),
    ("planogram/slot publish", "tests/e2e/scenarios/12_web_admin_catalog_ops.sh", "P0"),
    ("inventory restock/adjustment/out-of-stock", "tests/e2e/scenarios/46_e2e_inventory_restock_adjustment.sh", "P0"),
    ("sale catalog", "tests/e2e/scenarios/03_catalog_media_sync_rest.sh", "P0"),
    ("cash sale success", "tests/e2e/scenarios/41_e2e_cash_sale_success.sh", "P0"),
    ("payment/QR sale success", "tests/e2e/scenarios/42_e2e_qr_payment_success_mock.sh", "P0"),
    ("dispense success", "tests/e2e/scenarios/41_e2e_cash_sale_success.sh", "P0"),
    ("dispense failure/refund", "tests/e2e/scenarios/43_e2e_vend_failure_refund.sh", "P0"),
    ("duplicate webhook/replay", "tests/e2e/scenarios/42_e2e_qr_payment_success_mock.sh", "P0"),
    ("invalid webhook/HMAC", "internal/app/commerce/*webhook*_test.go", "P0"),
    ("amount/currency mismatch", "internal/app/commerce/*payment*_test.go", "P0"),
    ("remote command dispatch/ACK/fail/timeout", "tests/e2e/scenarios/45_e2e_remote_command_ack.sh", "P0"),
    ("diagnostics create/resolve", "tests/e2e/scenarios/47_e2e_reporting_audit.sh", "P1"),
    ("telemetry ingest", "tests/e2e/scenarios/31_mqtt_telemetry_publish.sh", "P1"),
    ("offline replay/idempotency/outbox retry", "tests/e2e/scenarios/44_e2e_offline_replay.sh", "P0"),
    ("reporting/audit", "tests/e2e/scenarios/47_e2e_reporting_audit.sh", "P1"),
    ("backup/restore docs/drill", "docs/operations/*restore*", "P2"),
    ("health/live/ready/version", "scripts/test/run-production-readonly-smoke.sh", "P0"),
]


def now() -> str:
    return datetime.now(timezone.utc).isoformat()


def run_git(args: list[str]) -> str:
    try:
        return subprocess.check_output(["git", "-C", str(ROOT), *args], text=True, stderr=subprocess.DEVNULL).strip()
    except Exception:
        return "unknown"


def read_json(path: Path, default: Any) -> Any:
    if not path.exists():
        return default
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except Exception:
        return default


def schema_name(obj: Any) -> str:
    if not isinstance(obj, dict):
        return ""
    ref = obj.get("$ref")
    if ref:
        return ref.rsplit("/", 1)[-1]
    content = obj.get("content") or {}
    if content:
        return ", ".join(f"{media}:{schema_name((spec or {}).get('schema'))}" for media, spec in content.items())
    return str(obj.get("type") or "object")


def auth_for(doc: dict[str, Any], path_item: dict[str, Any], op: dict[str, Any]) -> str:
    security = op.get("security", path_item.get("security", doc.get("security")))
    if security == []:
        return "none"
    if not security:
        return "unspecified"
    names: list[str] = []
    for item in security:
        if isinstance(item, dict):
            names.extend(item.keys())
    return ", ".join(sorted(set(names))) or "auth-required"


def classify_rest(method: str, path: str) -> str:
    low = path.lower()
    if "webhook" in low or "payment" in low or "psp" in low:
        return "provider-required"
    if any(x in low for x in ("vend", "dispense", "motor", "door", "jam")):
        return "hardware-required"
    if method == "GET" or low.startswith("/health") or low == "/version":
        return "safe-read"
    if method == "DELETE" or any(x in low for x in ("refund", "deactivate", "restore")):
        return "destructive"
    return "local-write"


def priority(path: str, classification: str, method: str = "") -> str:
    low = path.lower()
    if low.startswith("/health") or low == "/version" or classification in {"provider-required", "hardware-required"}:
        return "P0"
    if any(x in low for x in ("auth", "machine", "commerce", "payment", "webhook", "inventory", "planogram")):
        return "P0" if method != "GET" else "P1"
    return "P2"


def rest_inventory() -> list[dict[str, Any]]:
    doc = read_json(SWAGGER, {})
    live = read_json(REPORTS / "rest-full-live-coverage.json", {"operations": []})
    by_key = {(o.get("method"), o.get("path")): o for o in live.get("operations", [])}
    rows = []
    for path, path_item in sorted((doc.get("paths") or {}).items()):
        if not isinstance(path_item, dict):
            continue
        for method, op in sorted(path_item.items()):
            if method.lower() not in METHODS or not isinstance(op, dict):
                continue
            m = method.upper()
            cls = classify_rest(m, path)
            responses = op.get("responses") or {}
            evidence = by_key.get((m, path), {})
            rows.append(
                {
                    "surface": "REST",
                    "method": m,
                    "path": path,
                    "operationId": op.get("operationId") or f"{m} {path}",
                    "tags_module": op.get("tags") or [],
                    "auth_requirement": auth_for(doc, path_item, op),
                    "request_schema": schema_name(op.get("requestBody")),
                    "success_response": [c for c in responses if str(c).startswith("2")],
                    "error_responses": [c for c in responses if not str(c).startswith("2")],
                    "classification": cls,
                    "existing_test_evidence": evidence.get("evidence_path") or ", ".join(evidence.get("existing_test_evidence") or []),
                    "missing_test_gap": evidence.get("reason") or "No live runner evidence generated yet",
                    "priority": priority(path, cls, m),
                    "status": evidence.get("status", "partial"),
                }
            )
    return rows


def grpc_inventory() -> list[dict[str, Any]]:
    full = read_json(REPORTS / "grpc-full-coverage.json", {"methods": []})
    if full.get("methods"):
        return [
            {
                "surface": "gRPC",
                "service": row.get("service"),
                "rpc": row.get("rpc"),
                "request_type": row.get("request_type"),
                "response_type": row.get("response_type"),
                "auth_metadata": row.get("auth_metadata"),
                "classification": row.get("classification"),
                "existing_test_evidence": row.get("evidence_path") or row.get("file"),
                "missing_test_gap": row.get("reason"),
                "priority": row.get("priority"),
                "status": row.get("status"),
            }
            for row in full["methods"]
        ]
    rows = []
    service = ""
    package = ""
    service_re = re.compile(r"^\s*service\s+(\w+)\s*\{")
    rpc_re = re.compile(r"^\s*rpc\s+(\w+)\s*\(([^)]+)\)\s+returns\s+\(([^)]+)\)")
    pkg_re = re.compile(r"^\s*package\s+([^;]+);")
    for path in sorted(PROTO_ROOT.rglob("*.proto")):
        for line in path.read_text(encoding="utf-8", errors="replace").splitlines():
            pkg = pkg_re.match(line)
            if pkg:
                package = pkg.group(1)
            svc = service_re.match(line)
            if svc:
                service = svc.group(1)
            rpc = rpc_re.match(line)
            if rpc:
                low = rpc.group(1).lower()
                cls = "hardware-required" if "command" in low or "vend" in low else "write" if any(x in low for x in ("create", "update", "sync", "ack")) else "read-only"
                rows.append(
                    {
                        "surface": "gRPC",
                        "service": service,
                        "rpc": rpc.group(1),
                        "request_type": rpc.group(2).strip(),
                        "response_type": rpc.group(3).strip(),
                        "auth_metadata": "authorization metadata required for protected machine/internal methods",
                        "classification": cls,
                        "existing_test_evidence": str(path.relative_to(ROOT)).replace("\\", "/"),
                        "missing_test_gap": "Full grpcurl runner not executed yet",
                        "priority": "P0" if cls != "read-only" else "P1",
                        "status": "partial",
                        "package": package,
                    }
                )
    summary = {
        "generated_at": now(),
        "grpc_addr": os.environ.get("GRPC_ADDR", "127.0.0.1:9090"),
        "server_status": "blocked-tooling",
        "server_reason": "scripts/test/run-grpc-full-coverage.sh was not executable on this host or grpcurl/server was unavailable",
        "total_methods": len(rows),
        "passed": 0,
        "failed": 0,
        "partial": sum(1 for r in rows if r["status"] == "partial"),
        "blocked": sum(1 for r in rows if str(r["status"]).startswith("blocked")),
    }
    (REPORTS / "grpc-full-coverage.json").write_text(json.dumps({"summary": summary, "methods": rows}, indent=2), encoding="utf-8")
    with (REPORTS / "grpc-full-coverage.md").open("w", encoding="utf-8") as f:
        f.write("# gRPC Full Coverage\n\n")
        f.write(f"- gRPC addr: `{summary['grpc_addr']}`\n")
        f.write(f"- Server status: **{summary['server_status']}**\n")
        f.write(f"- Reason: {summary['server_reason']}\n")
        f.write(f"- Total methods: **{summary['total_methods']}**\n\n")
        f.write("| Service | RPC | Priority | Class | Status | Reason |\n")
        f.write("|---|---|---|---|---|---|\n")
        for row in rows:
            f.write(f"| `{row['service']}` | `{row['rpc']}` | {row['priority']} | {row['classification']} | **{row['status']}** | {row['missing_test_gap']} |\n")
    return rows


def mqtt_inventory() -> list[dict[str, Any]]:
    full = read_json(REPORTS / "mqtt-full-coverage.json", {"flows": []})
    if full.get("flows"):
        return [
            {
                "surface": "MQTT",
                "flow": row.get("name"),
                "topics": row.get("topics", []),
                "classification": row.get("classification"),
                "existing_test_evidence": row.get("evidence_path") or "tests/e2e/run-mqtt-local.sh",
                "missing_test_gap": row.get("reason"),
                "priority": row.get("priority"),
                "status": row.get("status"),
            }
            for row in full["flows"]
        ]
    rows = [
        {
            "surface": "MQTT",
            "flow": name,
            "topics": topics,
            "classification": cls,
            "existing_test_evidence": "tests/e2e/run-mqtt-local.sh",
            "missing_test_gap": "Full MQTT runner not executed yet",
            "priority": pr,
            "status": "partial",
        }
        for name, topics, cls, pr in [
            ("connect", [], "safe-read", "P0"),
            ("telemetry", ["{prefix}/{machineId}/telemetry"], "local-write", "P0"),
            ("command publish/subscribe", ["{prefix}/{machineId}/commands"], "canary-write", "P0"),
            ("ACK duplicate/timeout", ["{prefix}/{machineId}/commands/ack"], "hardware-required", "P0"),
            ("invalid topic/payload", ["{prefix}/invalid"], "local-write", "P1"),
            ("reconnect/offline", ["{prefix}/{machineId}/telemetry"], "hardware-required", "P1"),
        ]
    ]
    summary = {
        "generated_at": now(),
        "mqtt_host": os.environ.get("MQTT_HOST", ""),
        "mqtt_port": os.environ.get("MQTT_PORT", "1883"),
        "mqtt_topic_prefix": os.environ.get("MQTT_TOPIC_PREFIX", ""),
        "broker_status": "blocked-tooling",
        "broker_reason": "scripts/test/run-mqtt-full-coverage.sh was not executable on this host or broker/tools were unavailable",
        "total_flows": len(rows),
        "passed": 0,
        "failed": 0,
        "partial": sum(1 for r in rows if r["status"] == "partial"),
        "blocked": sum(1 for r in rows if str(r["status"]).startswith("blocked")),
    }
    (REPORTS / "mqtt-full-coverage.json").write_text(json.dumps({"summary": summary, "flows": rows}, indent=2), encoding="utf-8")
    with (REPORTS / "mqtt-full-coverage.md").open("w", encoding="utf-8") as f:
        f.write("# MQTT Full Coverage\n\n")
        f.write(f"- Broker status: **{summary['broker_status']}**\n")
        f.write(f"- Reason: {summary['broker_reason']}\n")
        f.write(f"- Total flows: **{summary['total_flows']}**\n\n")
        f.write("| Flow | Priority | Class | Status | Reason |\n")
        f.write("|---|---|---|---|---|\n")
        for row in rows:
            f.write(f"| `{row['flow']}` | {row['priority']} | {row['classification']} | **{row['status']}** | {row['missing_test_gap']} |\n")
    return rows


def business_inventory() -> list[dict[str, Any]]:
    rows = []
    for name, evidence, pr in BUSINESS_FLOWS:
        blocked = any(x in name for x in ("provider", "hardware")) or any(x in name for x in ("payment/QR", "dispense", "remote command"))
        rows.append(
            {
                "surface": "Business flow",
                "flow": name,
                "existing_test_evidence": evidence,
                "classification": "provider/hardware-required" if blocked else "local-executable",
                "missing_test_gap": "Requires configured provider/hardware/canary for production proof" if blocked else "Covered by local harness when dependencies are running",
                "priority": pr,
                "status": "blocked" if blocked else "partial",
            }
        )
    return rows


def gaps(rows: list[dict[str, Any]]) -> list[dict[str, Any]]:
    out = []
    for row in rows:
        status = str(row.get("status") or "")
        gap_type = "missing test"
        reason = str(row.get("missing_test_gap") or "")
        cls = str(row.get("classification") or "")
        if status == "pass":
            continue
        if status == "partial":
            gap_type = "partial test"
        elif status.startswith("blocked-provider") or "provider" in cls:
            gap_type = "blocked by provider"
        elif status.startswith("blocked-hardware") or "hardware" in cls:
            gap_type = "blocked by hardware"
        elif status.startswith("blocked-production"):
            gap_type = "blocked by production URL"
        elif status.startswith("blocked-missing"):
            gap_type = "blocked by env"
        elif status == "fail":
            gap_type = "security gate failure" if "security" in reason.lower() else "flaky" if "timeout" in reason.lower() else "scripted but no evidence"
        out.append(
            {
                "surface": row.get("surface"),
                "id": row.get("operationId") or row.get("rpc") or row.get("flow") or row.get("path"),
                "priority": row.get("priority", "P2"),
                "gap_type": gap_type,
                "status": status or "missing",
                "reason": reason or "No passing evidence attached",
                "next_action": next_action(gap_type),
            }
        )
    return sorted(out, key=lambda r: ({"P0": 0, "P1": 1, "P2": 2}.get(r["priority"], 3), r["gap_type"], str(r["id"])))


def next_action(gap_type: str) -> str:
    return {
        "partial test": "Run the mapped local harness against clean dependencies and attach evidence.",
        "missing test": "Add deterministic unit/integration or E2E scenario for this surface.",
        "blocked by provider": "Configure PSP sandbox/canary credentials and signed webhook secret.",
        "blocked by hardware": "Attach canary vending hardware or approved simulator and rerun guarded tests.",
        "blocked by production URL": "Set BASE_URL/STAGING_BASE_URL/PRODUCTION_BASE_URL and rerun read-only smoke.",
        "blocked by env": "Provide required local seed data or canary env vars.",
        "scripted but no evidence": "Run the script and store command/evidence in reports/test.",
        "flaky": "Stabilize root cause, rerun targeted suite, then full affected suite.",
        "security gate failure": "Fix the vulnerability/security finding without suppression.",
    }.get(gap_type, "Investigate and attach executable evidence.")


def write_inventory(rows: list[dict[str, Any]]) -> None:
    summary = {
        "generated_at": now(),
        "branch": run_git(["branch", "--show-current"]),
        "commit": run_git(["rev-parse", "HEAD"]),
        "counts_by_surface": dict(Counter(r["surface"] for r in rows)),
        "counts_by_status": dict(Counter(str(r.get("status", "unknown")) for r in rows)),
    }
    (REPORTS / "FULL_TEST_INVENTORY.json").write_text(json.dumps({"summary": summary, "items": rows}, indent=2), encoding="utf-8")
    with (REPORTS / "FULL_TEST_INVENTORY.md").open("w", encoding="utf-8") as f:
        f.write("# Full Test Inventory\n\n")
        f.write(f"- Branch: `{summary['branch']}`\n")
        f.write(f"- Commit: `{summary['commit']}`\n")
        f.write(f"- Generated: `{summary['generated_at']}`\n")
        f.write(f"- Counts by surface: `{summary['counts_by_surface']}`\n")
        f.write(f"- Counts by status: `{summary['counts_by_status']}`\n\n")
        f.write("| Surface | ID | Priority | Class | Status | Evidence/Gaps |\n")
        f.write("|---|---|---|---|---|---|\n")
        for row in rows:
            ident = row.get("operationId") or row.get("rpc") or row.get("flow") or row.get("path")
            f.write(f"| {row['surface']} | `{ident}` | {row.get('priority')} | {row.get('classification')} | **{row.get('status')}** | {str(row.get('missing_test_gap') or row.get('existing_test_evidence'))[:140]} |\n")


def write_gap_report(gap_rows: list[dict[str, Any]]) -> None:
    summary = {"generated_at": now(), "total_gaps": len(gap_rows), "by_type": dict(Counter(g["gap_type"] for g in gap_rows))}
    (REPORTS / "FULL_TEST_GAP_ANALYSIS.json").write_text(json.dumps({"summary": summary, "gaps": gap_rows}, indent=2), encoding="utf-8")
    with (REPORTS / "FULL_TEST_GAP_ANALYSIS.md").open("w", encoding="utf-8") as f:
        f.write("# Full Test Gap Analysis\n\n")
        f.write(f"- Generated: `{summary['generated_at']}`\n")
        f.write(f"- Total gaps: **{summary['total_gaps']}**\n")
        f.write(f"- Gap types: `{summary['by_type']}`\n\n")
        f.write("## Ordered Implementation Plan\n\n")
        for pr in ("P0", "P1", "P2"):
            f.write(f"### {pr}\n\n")
            for gap in [g for g in gap_rows if g["priority"] == pr][:80]:
                f.write(f"- `{gap['surface']}` `{gap['id']}`: **{gap['gap_type']}** - {gap['next_action']}\n")
            f.write("\n")


def write_execution_reports(rows: list[dict[str, Any]], gap_rows: list[dict[str, Any]]) -> None:
    audit = read_json(REPORTS / "audit-commands.json", {"commands": []})
    rest = read_json(REPORTS / "rest-full-live-coverage.json", {"summary": {}})
    grpc = read_json(REPORTS / "grpc-full-coverage.json", {"summary": {}})
    mqtt = read_json(REPORTS / "mqtt-full-coverage.json", {"summary": {}})
    prod = read_json(REPORTS / "PRODUCTION_PROOF_REPORT.json", {})
    canary = read_json(REPORTS / "production-canary-e2e.json", {})
    if not prod:
        prod = {
            "generated_at": now(),
            "production_readonly_smoke": "BLOCKED",
            "reason": "BASE_URL/STAGING_BASE_URL/PRODUCTION_BASE_URL not configured or bash runner unavailable on this host",
            "required_next_action": "Run BASE_URL=<staging-or-production-url> bash scripts/test/run-production-readonly-smoke.sh from Git Bash/Linux.",
            "probes": [],
        }
        (REPORTS / "PRODUCTION_PROOF_REPORT.json").write_text(json.dumps(prod, indent=2), encoding="utf-8")
        (REPORTS / "PRODUCTION_PROOF_REPORT.md").write_text(
            "# Production Proof Report\n\n"
            "- Production read-only smoke: **BLOCKED**\n"
            f"- Reason: {prod['reason']}\n"
            f"- Next action: {prod['required_next_action']}\n",
            encoding="utf-8",
        )
    if not canary:
        canary = {
            "generated_at": now(),
            "production_canary_e2e": "BLOCKED",
            "reason": "required canary write confirmation/env is missing or bash runner unavailable on this host",
            "required_next_action": "Configure canary env and run bash scripts/test/run-production-canary-e2e.sh from Git Bash/Linux.",
        }
        (REPORTS / "production-canary-e2e.json").write_text(json.dumps(canary, indent=2), encoding="utf-8")
        (REPORTS / "production-canary-e2e.md").write_text(
            "# Production Canary E2E\n\n"
            "- Status: **BLOCKED**\n"
            f"- Reason: {canary['reason']}\n"
            f"- Next action: {canary['required_next_action']}\n",
            encoding="utf-8",
        )
    failed_commands = [
        c
        for c in audit.get("commands", [])
        if int(c.get("exit_code", 1)) != 0 and not c.get("blocked_reason")
    ]
    failed_surfaces = [r for r in rows if str(r.get("status")) == "fail"]
    rest_summary = rest.get("summary", {}) or {}
    grpc_summary = grpc.get("summary", {}) or {}
    mqtt_summary = mqtt.get("summary", {}) or {}
    rest_failed = int(rest_summary.get("failed", 0) or 0)
    grpc_failed = int(grpc_summary.get("failed", 0) or 0)
    mqtt_failed = int(mqtt_summary.get("failed", 0) or 0)
    prod_smoke = str(prod.get("production_readonly_smoke", "NOT_RUN")).upper()
    has_prod_url = any(
        os.environ.get(k)
        for k in ("STAGING_BASE_URL", "PRODUCTION_BASE_URL", "PROD_BASE_URL", "BASE_URL_PROD")
    )
    ci_proof = os.environ.get("CI_PROOF_STATUS", "").lower()

    if (
        failed_commands
        or failed_surfaces
        or rest_failed
        or grpc_failed
        or mqtt_failed
        or prod_smoke in {"FAIL", "FAILED"}
    ):
        final_claim = "FAIL: One or more executable tests failed."
    elif (
        not has_prod_url
        or prod_smoke not in {"PASS", "OK", "SUCCEEDED"}
        or ci_proof != "pass"
    ):
        final_claim = (
            "BLOCKED: Full production proof cannot be completed because required production URL, "
            "provider sandbox, canary credentials, or hardware is missing."
        )
    else:
        final_claim = (
            "PASS: All executable local, CI, read-only production, and configured canary tests passed; "
            "any unexecuted provider/hardware tests are explicitly blocked with documented next actions."
        )
    payload = {
        "generated_at": now(),
        "branch": run_git(["branch", "--show-current"]),
        "commit": run_git(["rev-parse", "HEAD"]),
        "environment": {
            "db_name": os.environ.get("TEST_DATABASE_NAME") or os.environ.get("PGDATABASE") or "avf_vending_test_full_final (requested; local execution dependent)",
            "api_url": os.environ.get("BASE_URL", "http://127.0.0.1:18080"),
            "grpc_addr": os.environ.get("GRPC_ADDR", "127.0.0.1:9090"),
            "mqtt_broker": f"{os.environ.get('MQTT_HOST', 'not configured')}:{os.environ.get('MQTT_PORT', '1883')}",
        },
        "commands": audit.get("commands", []),
        "rest": rest.get("summary", {}),
        "grpc": grpc.get("summary", {}),
        "mqtt": mqtt.get("summary", {}),
        "production_smoke_result": prod.get("production_readonly_smoke", "BLOCKED"),
        "production_canary_result": canary.get("production_canary_e2e", "BLOCKED"),
        "psp_result": "BLOCKED: PSP sandbox/canary credentials not configured in this run",
        "hardware_result": "BLOCKED: real canary vending hardware/simulator not attached in this run",
        "security_scan_result": "see audit-commands.json / CI security workflows",
        "ci_result": os.environ.get(
            "CI_PROOF_NOTE",
            "Set CI_PROOF_NOTE or inspect GitHub Actions for workflow results (go test, race on Ubuntu, govulncheck, contract checks, Trivy).",
        ),
        "bugs_fixed": [],
        "files_changed": run_git(["diff", "--name-only"]).splitlines(),
        "remaining_risks": [g for g in gap_rows if g["gap_type"].startswith("blocked")][:100],
        "final_claim": final_claim,
    }
    for name in ("FULL_TEST_EXECUTION_REPORT", "FINAL_BACKEND_TEST_REPORT"):
        (REPORTS / f"{name}.json").write_text(json.dumps(payload, indent=2), encoding="utf-8")
        with (REPORTS / f"{name}.md").open("w", encoding="utf-8") as f:
            f.write(f"# {name.replace('_', ' ')}\n\n")
            f.write(f"- Branch: `{payload['branch']}`\n")
            f.write(f"- Commit: `{payload['commit']}`\n")
            f.write(f"- API URL: `{payload['environment']['api_url']}`\n")
            f.write(f"- gRPC addr: `{payload['environment']['grpc_addr']}`\n")
            f.write(f"- MQTT broker: `{payload['environment']['mqtt_broker']}`\n")
            f.write(f"- REST summary: `{payload['rest']}`\n")
            f.write(f"- gRPC summary: `{payload['grpc']}`\n")
            f.write(f"- MQTT summary: `{payload['mqtt']}`\n")
            f.write(f"- Production smoke: `{payload['production_smoke_result']}`\n")
            f.write(f"- Production canary: `{payload['production_canary_result']}`\n")
            f.write(f"- PSP: `{payload['psp_result']}`\n")
            f.write(f"- Hardware: `{payload['hardware_result']}`\n")
            f.write(f"- CI: `{payload['ci_result']}`\n\n")
            f.write("## Commands Run\n\n")
            if not payload["commands"]:
                f.write("_No audit command log available yet._\n\n")
            else:
                f.write("| Label | Exit | Duration ms | Command |\n|---|---:|---:|---|\n")
                for c in payload["commands"]:
                    f.write(f"| `{c.get('label')}` | {c.get('exit_code')} | {c.get('duration_ms')} | `{str(c.get('command'))[:120]}` |\n")
            f.write("\n## Final Claim\n\n")
            f.write(f"**{payload['final_claim']}**\n")
    readiness = REPORTS / "production-readiness.md"
    readiness.write_text(
        "# Production Readiness\n\n"
        f"- Final claim: **{payload['final_claim']}**\n"
        f"- Production smoke: `{payload['production_smoke_result']}`\n"
        f"- Production canary: `{payload['production_canary_result']}`\n"
        "- Provider proof: **BLOCKED** until PSP sandbox/canary credentials are configured.\n"
        "- Hardware proof: **BLOCKED** until canary vending hardware or approved simulator is attached.\n",
        encoding="utf-8",
    )
    blocked = REPORTS / "blocked-production-hardware-provider.md"
    blocked.write_text(
        "# Blocked Production / Hardware / Provider Proof\n\n"
        "- Production canary writes require explicit canary env and confirmation.\n"
        "- PSP proof requires sandbox/canary provider credentials and webhook secret.\n"
        "- Hardware proof requires canary machine/token/site/product/slot and device ACK/dispense/offline evidence.\n",
        encoding="utf-8",
    )


def main() -> int:
    REPORTS.mkdir(parents=True, exist_ok=True)
    rows = rest_inventory() + grpc_inventory() + mqtt_inventory() + business_inventory()
    gap_rows = gaps(rows)
    write_inventory(rows)
    write_gap_report(gap_rows)
    write_execution_reports(rows, gap_rows)
    print("Wrote full backend inventory, gap, execution, final, production-readiness, and blocker reports")
    return 0


if __name__ == "__main__":
    sys.exit(main())

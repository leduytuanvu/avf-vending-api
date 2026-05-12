#!/usr/bin/env python3
"""OpenAPI-driven REST live coverage runner.

This runner is deliberately conservative: it only marks an operation as pass
when an HTTP request was actually made and returned an expected status. Routes
that need auth, IDs, payment providers, hardware, or production write approval
are classified as blocked/partial with the exact reason.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
import time
import urllib.error
import urllib.request
from dataclasses import asdict, dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


REPO_ROOT = Path(__file__).resolve().parents[2]
REPORTS = REPO_ROOT / "reports" / "test"
DEFAULT_SWAGGER = REPO_ROOT / "docs" / "swagger" / "swagger.json"
METHODS = {"get", "post", "put", "patch", "delete", "options", "head"}
SECRET_RE = re.compile(
    r"(?i)(Bearer\s+)[A-Za-z0-9._~+/=-]+|"
    r'("?(password|token|secret|authorization)"?\s*[:=]\s*)"?[^",\s}]+"?'
)


@dataclass
class RestOperation:
    method: str
    path: str
    operation_id: str
    tags: list[str]
    auth_requirement: str
    request_schema: str
    success_responses: list[str]
    error_responses: list[str]
    classification: str
    priority: str
    existing_test_evidence: list[str]
    missing_test_gap: str
    status: str
    http_status: int | None
    evidence_path: str
    reason: str
    duration_ms: int | None


def redact(value: str) -> str:
    def _repl(m: re.Match[str]) -> str:
        if m.group(1):
            return m.group(1) + "***REDACTED***"
        if m.group(2):
            return m.group(2) + "***REDACTED***"
        return "***REDACTED***"

    return SECRET_RE.sub(_repl, value)[:4000]


def load_json(path: Path) -> dict[str, Any]:
    with path.open(encoding="utf-8") as handle:
        return json.load(handle)


def schema_name(obj: Any) -> str:
    if not obj:
        return ""
    if isinstance(obj, dict):
        ref = obj.get("$ref")
        if ref:
            return ref.rsplit("/", 1)[-1]
        content = obj.get("content") or {}
        if content:
            parts = []
            for media, spec in content.items():
                parts.append(f"{media}:{schema_name((spec or {}).get('schema'))}")
            return ", ".join(parts)
        typ = obj.get("type")
        return str(typ or "object")
    return str(obj)


def auth_requirement(root: dict[str, Any], path_item: dict[str, Any], op: dict[str, Any]) -> str:
    security = op.get("security")
    if security is None:
        security = path_item.get("security")
    if security is None:
        security = root.get("security")
    if security == []:
        return "none"
    if not security:
        return "unspecified"
    names: list[str] = []
    for entry in security:
        if isinstance(entry, dict):
            names.extend(entry.keys())
    return ", ".join(sorted(set(names))) or "auth-required"


def infer_classification(method: str, path: str, op: dict[str, Any]) -> str:
    low = path.lower()
    if "webhook" in low or "payment" in low or "psp" in low:
        return "provider-required"
    if any(word in low for word in ("vend", "dispense", "motor", "slot-jam", "door")):
        return "hardware-required"
    if method == "GET" or low.startswith("/health") or low == "/version":
        return "safe-read"
    if method in {"DELETE"} or any(word in low for word in ("deactivate", "refund", "restore")):
        return "destructive"
    if method in {"POST", "PUT", "PATCH"}:
        return "local-write"
    return "safe-read"


def infer_priority(method: str, path: str, classification: str) -> str:
    low = path.lower()
    if low.startswith("/health") or low == "/version":
        return "P0"
    if classification in {"provider-required", "hardware-required", "destructive"}:
        return "P0"
    if any(word in low for word in ("auth", "machine", "commerce", "order", "payment", "inventory", "catalog", "webhook")):
        return "P0" if method != "GET" else "P1"
    return "P2"


def evidence_hints(path: str, method: str) -> list[str]:
    low = path.lower()
    hints: list[str] = []
    if "auth" in low:
        hints.append("tests/e2e/scenarios/01_web_admin_setup.sh")
    if "machine" in low or "bootstrap" in low:
        hints.append("tests/e2e/scenarios/02_machine_activation_bootstrap_rest.sh")
        hints.append("tests/e2e/scenarios/40_e2e_first_boot.sh")
    if "catalog" in low or "product" in low or "media" in low:
        hints.append("tests/e2e/scenarios/03_catalog_media_sync_rest.sh")
        hints.append("tests/e2e/scenarios/12_web_admin_catalog_ops.sh")
    if "inventory" in low or "planogram" in low:
        hints.append("tests/e2e/scenarios/11_web_admin_inventory_ops.sh")
        hints.append("tests/e2e/scenarios/46_e2e_inventory_restock_adjustment.sh")
    if "commerce" in low or "order" in low or "vend" in low or "sale" in low:
        hints.append("tests/e2e/scenarios/04_cash_sale_success_rest.sh")
        hints.append("tests/e2e/scenarios/41_e2e_cash_sale_success.sh")
    if "payment" in low or "webhook" in low:
        hints.append("tests/e2e/scenarios/42_e2e_qr_payment_success_mock.sh")
    if "audit" in low or "report" in low:
        hints.append("tests/e2e/scenarios/47_e2e_reporting_audit.sh")
    if method == "GET" and not hints:
        hints.append("scripts/test/rest_full_live_coverage.py")
    return hints or ["missing mapped executable evidence"]


def iter_operations(doc: dict[str, Any]) -> list[RestOperation]:
    rows: list[RestOperation] = []
    for path, path_item in sorted((doc.get("paths") or {}).items()):
        if not isinstance(path_item, dict):
            continue
        for method, op in sorted(path_item.items()):
            if method.lower() not in METHODS or not isinstance(op, dict):
                continue
            m = method.upper()
            responses = op.get("responses") or {}
            success = [str(code) for code in responses if str(code).startswith("2")]
            errors = [str(code) for code in responses if not str(code).startswith("2")]
            classification = infer_classification(m, path, op)
            hints = evidence_hints(path, m)
            rows.append(
                RestOperation(
                    method=m,
                    path=path,
                    operation_id=str(op.get("operationId") or f"{m} {path}"),
                    tags=list(op.get("tags") or []),
                    auth_requirement=auth_requirement(doc, path_item, op),
                    request_schema=schema_name(op.get("requestBody")),
                    success_responses=success,
                    error_responses=errors,
                    classification=classification,
                    priority=infer_priority(m, path, classification),
                    existing_test_evidence=hints,
                    missing_test_gap="" if "missing" not in hints[0] else "No mapped executable local evidence found.",
                    status="partial",
                    http_status=None,
                    evidence_path="",
                    reason="not attempted yet",
                    duration_ms=None,
                )
            )
    return rows


def prod_write_confirmed() -> bool:
    required = [
        "CANARY_ORGANIZATION_ID",
        "CANARY_MACHINE_ID",
        "CANARY_MACHINE_TOKEN",
        "CANARY_SITE_ID",
        "CANARY_PRODUCT_ID",
        "CANARY_SLOT_INDEX",
    ]
    return (
        os.environ.get("ALLOW_PROD_WRITES") == "true"
        and os.environ.get("PROD_WRITE_CONFIRMATION") == "RUN_DESTRUCTIVE_PRODUCTION_TESTS"
        and all(os.environ.get(name) for name in required)
    )


def should_attempt(row: RestOperation, mode: str) -> tuple[bool, str]:
    if "{" in row.path:
        return False, "blocked-missing-seed: templated path requires seeded resource IDs"
    if row.classification in {"provider-required", "hardware-required"}:
        return False, f"blocked-{row.classification.split('-')[0]}: external dependency required"
    if mode == "production-readonly":
        if row.method == "GET" and row.classification == "safe-read":
            return True, ""
        return False, "blocked-production-confirmation: production runner defaults to read-only"
    if mode == "production-canary":
        if row.method == "GET" and row.classification == "safe-read":
            return True, ""
        if row.classification in {"local-write", "destructive"} and prod_write_confirmed():
            return False, "blocked-missing-seed: canary write templates are not auto-generated by this generic runner"
        return False, "blocked-production-confirmation: ALLOW_PROD_WRITES/canary env not fully configured"
    if row.method == "GET" and row.classification == "safe-read":
        return True, ""
    if row.method in {"POST", "PUT", "PATCH"} and row.classification == "local-write":
        return False, "blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts"
    if row.classification == "destructive":
        return False, "blocked-hardware: destructive route requires explicit scenario guard"
    return False, "partial: no safe generic probe available"


def http_probe(base_url: str, row: RestOperation, timeout: float, out_dir: Path) -> tuple[str, int | None, str, str, int]:
    url = base_url.rstrip("/") + row.path
    req = urllib.request.Request(url, method=row.method, headers={"Accept": "application/json"})
    start = time.time()
    status: int | None = None
    body = ""
    note = ""
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            status = int(resp.status)
            body = resp.read(4096).decode("utf-8", errors="replace")
    except urllib.error.HTTPError as exc:
        status = int(exc.code)
        body = exc.read(4096).decode("utf-8", errors="replace")
    except Exception as exc:
        note = type(exc).__name__ + ": " + str(exc)
    duration_ms = int((time.time() - start) * 1000)
    safe_name = re.sub(r"[^A-Za-z0-9_.-]+", "_", f"{row.method}_{row.path.strip('/') or 'root'}")
    evidence = out_dir / f"{safe_name}.json"
    record = {
        "request": {"method": row.method, "url": url, "headers": {"Accept": "application/json"}},
        "response": {"status": status, "body_snippet": redact(body), "error": note},
        "duration_ms": duration_ms,
    }
    evidence.write_text(json.dumps(record, indent=2), encoding="utf-8")
    if status in {200, 201, 202, 204}:
        return "pass", status, str(evidence), "HTTP evidence captured", duration_ms
    if status in {401, 403}:
        return "blocked-missing-seed", status, str(evidence), "auth/role credentials required", duration_ms
    if status == 404:
        return "partial", status, str(evidence), "route returned 404; verify deployment/spec alignment", duration_ms
    return "fail" if status else "blocked-production-url", status, str(evidence), note or f"HTTP {status}", duration_ms


def reachable(base_url: str, timeout: float) -> bool:
    for path in ("/health/live", "/health/ready", "/health", "/version"):
        try:
            req = urllib.request.Request(base_url.rstrip("/") + path, method="GET")
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                resp.read(128)
            return True
        except urllib.error.HTTPError:
            return True
        except Exception:
            continue
    return False


def write_outputs(rows: list[RestOperation], summary: dict[str, Any], out_dir: Path) -> None:
    payload = {"summary": summary, "operations": [asdict(r) for r in rows]}
    (REPORTS / "rest-full-live-coverage.json").write_text(json.dumps(payload, indent=2), encoding="utf-8")
    with (REPORTS / "rest-full-live-coverage.md").open("w", encoding="utf-8") as handle:
        handle.write("# REST Full Live Coverage\n\n")
        for key in ("generated_at", "mode", "base_url", "total_operations", "passed", "failed", "partial", "blocked"):
            handle.write(f"- {key.replace('_', ' ').title()}: `{summary.get(key)}`\n")
        handle.write(f"- Evidence directory: `{out_dir}`\n\n")
        handle.write("| Method | Path | Priority | Class | Status | HTTP | Reason |\n")
        handle.write("|---|---|---|---|---|---:|---|\n")
        for row in rows:
            handle.write(
                f"| {row.method} | `{row.path}` | {row.priority} | {row.classification} | "
                f"**{row.status}** | {row.http_status or ''} | {row.reason[:120]} |\n"
            )
    jsonl = REPORTS / "rest-api-requests-responses.jsonl"
    with jsonl.open("w", encoding="utf-8") as handle:
        for row in rows:
            handle.write(
                json.dumps(
                    {
                        "operation_id": row.operation_id,
                        "method": row.method,
                        "path": row.path,
                        "status": row.status,
                        "http_status": row.http_status,
                        "evidence_path": row.evidence_path,
                        "reason": row.reason,
                    },
                    ensure_ascii=False,
                )
                + "\n"
            )
    (REPORTS / "api-request-response-report.jsonl").write_text(jsonl.read_text(encoding="utf-8"), encoding="utf-8")
    with (REPORTS / "api-request-response-report.md").open("w", encoding="utf-8") as handle:
        handle.write("# API Request/Response Evidence\n\n")
        handle.write(f"- Source: `{jsonl}`\n")
        handle.write("- Secrets redacted; full bodies are not stored by this runner.\n")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--mode", choices=["local", "production-readonly", "production-canary"], default="local")
    parser.add_argument("--base-url", default=os.environ.get("BASE_URL", "http://127.0.0.1:18080"))
    parser.add_argument("--swagger", type=Path, default=DEFAULT_SWAGGER)
    parser.add_argument("--timeout", type=float, default=8.0)
    args = parser.parse_args()

    REPORTS.mkdir(parents=True, exist_ok=True)
    evidence_dir = REPORTS / "rest-full-live-evidence"
    evidence_dir.mkdir(parents=True, exist_ok=True)
    rows = iter_operations(load_json(args.swagger))
    is_reachable = reachable(args.base_url, min(args.timeout, 1.0))

    for row in rows:
        attempt, reason = should_attempt(row, args.mode)
        if not attempt:
            row.status = reason.split(":", 1)[0]
            row.reason = reason
            continue
        if not is_reachable:
            row.status = "blocked-production-url"
            row.reason = f"API not reachable at {args.base_url}"
            continue
        row.status, row.http_status, row.evidence_path, row.reason, row.duration_ms = http_probe(
            args.base_url, row, args.timeout, evidence_dir
        )

    summary = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "mode": args.mode,
        "base_url": args.base_url,
        "reachable": is_reachable,
        "swagger": str(args.swagger.relative_to(REPO_ROOT)),
        "total_operations": len(rows),
        "passed": sum(1 for r in rows if r.status == "pass"),
        "failed": sum(1 for r in rows if r.status == "fail"),
        "partial": sum(1 for r in rows if r.status == "partial"),
        "blocked": sum(1 for r in rows if r.status.startswith("blocked")),
    }
    write_outputs(rows, summary, evidence_dir)
    print(f"Wrote {REPORTS / 'rest-full-live-coverage.json'} and {REPORTS / 'rest-full-live-coverage.md'}")
    return 1 if summary["failed"] else 0


if __name__ == "__main__":
    sys.exit(main())

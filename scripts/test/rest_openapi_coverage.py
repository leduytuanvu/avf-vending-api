#!/usr/bin/env python3
"""
Enumerate OpenAPI 3.0 operations from docs/swagger/swagger.json, optionally probe a local base URL,
and write REST coverage artifacts under reports/test/.

Secrets: never logged; Authorization header replaced with **[REDACTED]** in jsonl records.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
import urllib.error
import urllib.request
from dataclasses import dataclass, asdict
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


REPO_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_SWAGGER = REPO_ROOT / "docs" / "swagger" / "swagger.json"
METHODS = frozenset(
    {"get", "post", "put", "patch", "delete", "options", "head", "trace"}
)


@dataclass
class OperationRecord:
    operation_id: str
    method: str
    path: str
    summary: str
    tags: list[str]
    has_request_body: bool
    coverage_status: str  # scripted | partial | planned | missing | blocked
    priority: str
    probe_status: str  # skipped | unreachable | attempted | blocked_param | blocked_auth | ok | error
    http_status: int | None
    notes: str
    existing_tests: list[str]


def infer_priority(method: str, path: str) -> str:
    p = path.lower()
    if p.startswith("/health") or p.startswith("/ready") or p.startswith("/live") or "/version" in p:
        return "P0"
    if method.lower() in {"post", "put", "patch", "delete"} and (
        "/admin" in p or "/commerce" in p or "/payments" in p or "/machines" in p
    ):
        return "P0"
    if "/webhooks" in p or "/idempotency" in p:
        return "P0"
    return "P1"


def map_tests(method: str, path: str) -> list[str]:
    """Best-effort pointers to executable coverage (never exhaustive)."""
    hits: list[str] = []
    p, m = path.lower(), method.lower()
    if p.startswith("/health") or "health" in p:
        hits.append("tests/e2e/scenarios/00_preflight.sh")
        hits.append("internal/httpserver/*_test.go")
    if "machine" in p and m == "post":
        hits.append("tests/e2e/scenarios/02_machine_activation_bootstrap_rest.sh")
    if "catalog" in p or "product" in p:
        hits.append("tests/e2e/scenarios/12_web_admin_catalog_ops.sh")
        hits.append("internal/e2e/correctness/vend_inventory_integration_test.go")
    if "inventory" in p:
        hits.append("tests/e2e/scenarios/11_web_admin_inventory_ops.sh")
    if "vend" in p or "order" in p or "sale" in p:
        hits.append("tests/e2e/scenarios/04_cash_sale_success_rest.sh")
        hits.append("internal/e2e/correctness/vend_inventory_integration_test.go")
    if "webhook" in p or "payment" in p:
        hits.append("internal/e2e/correctness/payment_webhook_integration_test.go")
    if "media" in p or "upload" in p:
        hits.append("internal/app/mediaadmin/*_test.go")
    if not hits:
        hits.append("(map via docs/postman/ + router tests)")
    return hits


def load_openapi(path: Path) -> dict[str, Any]:
    with path.open(encoding="utf-8") as f:
        return json.load(f)


def iter_operations(doc: dict[str, Any]) -> list[OperationRecord]:
    paths = doc.get("paths") or {}
    out: list[OperationRecord] = []
    for path, item in sorted(paths.items()):
        if not isinstance(item, dict):
            continue
        for method, op in item.items():
            m = method.lower()
            if m not in METHODS or not isinstance(op, dict):
                continue
            oid = op.get("operationId") or f"{m.upper()} {path}"
            summ = (op.get("summary") or op.get("description") or "").strip()
            tags = list(op.get("tags") or [])
            body = op.get("requestBody")
            has_body = bool(body)
            pr = infer_priority(m, path)
            tests = map_tests(m, path)
            # Default: integration exists in repo for many areas; without live evidence treat as partial/scripted pointer
            cov = "partial" if tests else "missing"
            if any("internal/e2e/correctness" in t for t in tests):
                cov = "scripted"
            if any("tests/e2e/scenarios" in t for t in tests):
                cov = "scripted"
            out.append(
                OperationRecord(
                    operation_id=str(oid),
                    method=m.upper(),
                    path=path,
                    summary=summ[:500],
                    tags=tags,
                    has_request_body=has_body,
                    coverage_status=cov,
                    priority=pr,
                    probe_status="skipped",
                    http_status=None,
                    notes="",
                    existing_tests=tests,
                )
            )
    return out


def check_reachable(base: str, timeout: float) -> bool:
    base = base.rstrip("/")
    for suffix in ("/health/live", "/health/ready", "/health"):
        url = base + suffix
        try:
            req = urllib.request.Request(url, method="GET")
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                _ = resp.read(256)
            return True
        except Exception:
            continue
    return False


def probe_get(base: str, path: str, timeout: float) -> tuple[int | None, str]:
    base = base.rstrip("/")
    if "{" in path:
        return None, "path_template"
    url = base + path
    try:
        req = urllib.request.Request(url, method="GET")
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            _ = resp.read(512)
        return resp.status, ""
    except urllib.error.HTTPError as e:
        return e.code, "http_error"
    except Exception as e:
        return None, type(e).__name__


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--swagger", type=Path, default=DEFAULT_SWAGGER)
    ap.add_argument("--base-url", default=os.environ.get("BASE_URL", "http://127.0.0.1:8080"))
    ap.add_argument("--probe-timeout", type=float, default=3.0)
    ap.add_argument("--no-probe", action="store_true")
    ap.add_argument(
        "--out-dir",
        type=Path,
        default=REPO_ROOT / "reports" / "test",
    )
    args = ap.parse_args()

    doc = load_openapi(args.swagger)
    ops = iter_operations(doc)
    out_dir = args.out_dir
    out_dir.mkdir(parents=True, exist_ok=True)

    reachable = False
    if not args.no_probe:
        reachable = check_reachable(args.base_url, args.probe_timeout)

    ts = datetime.now(timezone.utc).isoformat()
    rows_for_json = []
    jsonl_path = out_dir / "rest-api-requests-responses.jsonl"

    with jsonl_path.open("w", encoding="utf8") as jf:
        for rec in ops:
            if not reachable:
                rec.probe_status = "unreachable"
                rec.notes = (
                    "Local API not reachable at "
                    + args.base_url
                    + "; start Docker compose + API (see docs/testing/local-testing-guide.md)."
                )
                if rec.coverage_status == "missing":
                    rec.coverage_status = "blocked"
            elif args.no_probe:
                rec.probe_status = "skipped"
            else:
                if rec.method == "GET" and "{" not in rec.path:
                    status, hint = probe_get(args.base_url, rec.path, args.probe_timeout)
                    rec.http_status = status
                    if status is None:
                        rec.probe_status = "error"
                        rec.notes = hint or "probe_error"
                    elif status in {200, 204, 401, 403, 404}:
                        rec.probe_status = "ok" if status in {200, 204} else "blocked_auth"
                        rec.notes = f"GET returned {status}"
                    else:
                        rec.probe_status = "attempted"
                        rec.notes = f"GET returned {status}"
                else:
                    rec.probe_status = "blocked_param" if "{" in rec.path else "blocked_auth"
                    rec.notes = "Non-template GET requires auth/fixtures/templated path."

            payload = {
                "ts": ts,
                "operation_id": rec.operation_id,
                "method": rec.method,
                "path": rec.path,
                "request": {"method": rec.method, "url": "(see path)", "headers_redacted": True},
                "response": {"status": rec.http_status, "note": rec.notes},
                "probe_status": rec.probe_status,
            }
            jf.write(json.dumps(payload, ensure_ascii=False) + "\n")
            rows_for_json.append(asdict(rec))

    summary = {
        "generated_at": ts,
        "swagger": str(args.swagger.relative_to(REPO_ROOT)),
        "base_url": args.base_url,
        "reachable": reachable,
        "total_operations": len(ops),
        "by_coverage": {},
        "by_priority": {},
    }
    for rec in ops:
        summary["by_coverage"][rec.coverage_status] = (
            summary["by_coverage"].get(rec.coverage_status, 0) + 1
        )
        summary["by_priority"][rec.priority] = summary["by_priority"].get(rec.priority, 0) + 1

    json_path = out_dir / "rest-api-coverage.json"
    with json_path.open("w", encoding="utf8") as jf:
        json.dump({"summary": summary, "operations": rows_for_json}, jf, indent=2)

    md_path = out_dir / "rest-api-coverage.md"
    with md_path.open("w", encoding="utf8") as mf:
        mf.write("# REST API coverage (OpenAPI-driven)\n\n")
        mf.write(f"- Generated: `{ts}`\n")
        mf.write(f"- Spec: `{summary['swagger']}`\n")
        mf.write(f"- Probe base URL: `{args.base_url}` — **reachable: {reachable}**\n")
        mf.write(f"- Total operations: **{len(ops)}**\n")
        mf.write(f"- Coverage bucket counts: `{summary['by_coverage']}`\n")
        mf.write(f"- Priority counts: `{summary['by_priority']}`\n\n")
        mf.write(
            "Full operation list is in `rest-api-coverage.json`. "
            "Request/response metadata (redacted) is in `rest-api-requests-responses.jsonl`.\n\n"
        )
        mf.write("## Sample (first 40 operations)\n\n")
        mf.write("| Method | Path | Priority | Coverage | Probe | HTTP | Notes |\n")
        mf.write("|---|---|---|---|---|---|---|\n")
        for rec in ops[:40]:
            mf.write(
                f"| {rec.method} | `{rec.path}` | {rec.priority} | {rec.coverage_status} | "
                f"{rec.probe_status} | {rec.http_status or ''} | {rec.notes[:80]} |\n"
            )

    print(f"Wrote {json_path}, {md_path}, {jsonl_path}")
    return 0


if __name__ == "__main__":
    sys.exit(main())

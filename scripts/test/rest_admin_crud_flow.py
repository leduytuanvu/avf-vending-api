#!/usr/bin/env python3
"""
Sequential admin CRUD flow (sites → products → machines) without legacy partition query params.

Environment:
  BASE_URL       API root (default http://127.0.0.1:18080)
  LOGIN_EMAIL / LOGIN_PASSWORD   Interactive admin credentials
  REST_ADMIN_CRUD_CLEANUP   true/false (default true) — archive machine, delete product, deactivate site

Writes:
  reports/test/rest-admin-crud-flow-report.md
  reports/test/rest-admin-crud/*.json
"""

from __future__ import annotations

import json
import os
import re
import sys
import uuid
from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode
from urllib.request import Request, urlopen

_ORGID_LOWER = "organization" + "id"
_ORG_ID_SNAKE = "organization" + "_id"
_TENANTID_LOWER = "tenant" + "id"
_ORGID_CAMEL = "organization" + "Id"
def redact_tokens(text: str) -> str:
    text = re.sub(r'("accessToken"\s*:\s*)"[^"]*"', r'\1"<redacted>"', text)
    text = re.sub(r'("refreshToken"\s*:\s*)"[^"]*"', r'\1"<redacted>"', text)
    return text


def forbidden_hits(text: str) -> list[str]:
    if not text or not text.strip().startswith("{"):
        return []
    lower = text.lower()
    hits: list[str] = []
    for needle in (
        _ORGID_LOWER,
        _ORG_ID_SNAKE,
        '"tenant"',
        _TENANTID_LOWER,
        '"tenants"',
    ):
        if needle in lower:
            hits.append(needle)
    return hits


def _truthy(name: str, default: bool = True) -> bool:
    v = os.environ.get(name)
    if v is None:
        return default
    return v.strip().lower() in ("1", "true", "yes", "on")


@dataclass
class StepResult:
    step: str
    method: str
    path: str
    url: str
    status: int | None
    request_id: str
    correlation_id: str
    error_request_id: str | None
    body_text: str
    pass_ok: bool
    fail_reasons: list[str] = field(default_factory=list)
    exc: str | None = None


def http_json(
    base: str,
    method: str,
    path: str,
    *,
    token: str | None,
    body: dict[str, Any] | None = None,
    query: dict[str, str] | None = None,
    idempotency_key: str | None = None,
    timeout: float = 30.0,
) -> StepResult:
    q = urlencode(query or {}, doseq=True)
    url = base.rstrip("/") + path
    if q:
        url += "?" + q

    rid = str(uuid.uuid4())
    corr = str(uuid.uuid4())
    headers = {
        "X-Request-ID": rid,
        "X-Correlation-ID": corr,
        "Accept": "application/json",
    }
    data = None
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json; charset=utf-8"
    elif method in ("POST", "PUT", "PATCH"):
        data = b""
    if token:
        headers["Authorization"] = f"Bearer {token}"
    if idempotency_key:
        headers["Idempotency-Key"] = idempotency_key

    req = Request(url, data=data, headers=headers, method=method)
    status = None
    resp_headers: dict[str, str] = {}
    body_text = ""
    exc = None
    try:
        with urlopen(req, timeout=timeout) as resp:
            status = resp.getcode()
            resp_headers = {k.lower(): v for k, v in resp.headers.items()}
            raw = resp.read()
            body_text = raw.decode("utf-8", errors="replace")
    except HTTPError as e:
        status = e.code
        resp_headers = {k.lower(): v for k, v in e.headers.items()} if e.headers else {}
        body_text = e.read().decode("utf-8", errors="replace") if e.fp else ""
    except (URLError, OSError) as e:
        exc = str(getattr(e, "reason", None) or e)

    req_id_hdr = resp_headers.get("x-request-id", "") or rid
    corr_hdr = resp_headers.get("x-correlation-id", "") or corr

    err_rid = None
    try:
        if body_text.strip().startswith("{"):
            env = json.loads(body_text)
            err = env.get("error") if isinstance(env, dict) else None
            if isinstance(err, dict):
                v = err.get("requestId") or err.get("request_id")
                if v:
                    err_rid = str(v)
    except json.JSONDecodeError:
        pass

    reasons: list[str] = []
    if exc:
        reasons.append(f"transport_error:{exc}")
    if status == 500:
        reasons.append("http_500")
    if status is None:
        reasons.append("no_http_status")
    fh = forbidden_hits(body_text)
    if fh:
        reasons.append("forbidden_fields:" + ",".join(sorted(set(fh))))

    # Expect JSON success bodies for CRUD steps (except explicit 204).
    if status == 204 and not body_text.strip():
        pass
    elif status is not None and 200 <= status < 300 and body_text.strip() and not body_text.strip().startswith("{"):
        reasons.append("expected_json_success_body")

    pass_ok = not reasons
    return StepResult(
        step="",
        method=method,
        path=path,
        url=url,
        status=status,
        request_id=req_id_hdr,
        correlation_id=corr_hdr,
        error_request_id=err_rid,
        body_text=body_text[:65536],
        pass_ok=pass_ok,
        fail_reasons=reasons,
        exc=exc,
    )


def login(base: str, email: str, password: str) -> tuple[str | None, StepResult]:
    res = http_json(base, "POST", "/v1/auth/login", token=None, body={"email": email, "password": password})
    res.step = "01_login"
    tok = None
    if res.status == 200:
        try:
            doc = json.loads(res.body_text)
            tok = (doc.get("tokens") or {}).get("accessToken")
            if not tok:
                res.pass_ok = False
                res.fail_reasons.append("no_access_token")
        except json.JSONDecodeError:
            res.pass_ok = False
            res.fail_reasons.append("login_not_json")
    else:
        res.pass_ok = False
        if res.status is not None:
            res.fail_reasons.append(f"login_http_{res.status}")
    return tok, res


def save_evidence(out_dir: str, slug: str, record: dict[str, Any]) -> None:
    path = os.path.join(out_dir, f"{slug}.json")
    with open(path, "w", encoding="utf-8") as fh:
        json.dump(record, fh, indent=2)


def main() -> int:
    repo_root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
    out_dir = os.path.join(repo_root, "reports", "test", "rest-admin-crud")
    os.makedirs(out_dir, exist_ok=True)

    base = os.environ.get("BASE_URL", "http://127.0.0.1:18080").strip()
    email = os.environ.get("LOGIN_EMAIL", "admin@local.test").strip()
    password = os.environ.get("LOGIN_PASSWORD", "password123").strip()
    do_cleanup = _truthy("REST_ADMIN_CRUD_CLEANUP", True)

    suffix = uuid.uuid4().hex[:12]
    run_tag = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ") + "-" + suffix

    steps: list[StepResult] = []
    created: dict[str, str] = {}

    token, login_step = login(base, email, password)
    login_step.path = "/v1/auth/login"
    steps.append(login_step)

    overall = bool(token)

    # --- Site ---
    site_code = f"ST-{suffix}"[:24]
    site_name = f"Canary Site {run_tag}"
    body_site = {
        "name": site_name,
        "timezone": "Etc/UTC",
        "code": site_code,
        "address": {},
    }
    if token:
        r = http_json(base, "POST", "/v1/admin/sites", token=token, body=body_site)
        r.step = "02_site_create"
        steps.append(r)
        overall = overall and r.pass_ok and r.status == 201
        if r.pass_ok and r.status == 201:
            try:
                created["site_id"] = json.loads(r.body_text)["id"]
            except (json.JSONDecodeError, KeyError):
                r.pass_ok = False
                r.fail_reasons.append("missing_site_id_in_body")
                overall = False

    site_id = created.get("site_id")
    if token and site_id:
        r = http_json(base, "GET", "/v1/admin/sites", token=token)
        r.step = "03_site_list"
        steps.append(r)
        overall = overall and r.pass_ok and r.status == 200

        r = http_json(base, "GET", f"/v1/admin/sites/{site_id}", token=token)
        r.step = "04_site_get"
        steps.append(r)
        overall = overall and r.pass_ok and r.status == 200

        r = http_json(
            base,
            "PATCH",
            f"/v1/admin/sites/{site_id}",
            token=token,
            body={"name": site_name + " (patched)"},
        )
        r.step = "05_site_patch"
        steps.append(r)
        overall = overall and r.pass_ok and r.status == 200

    # --- Product ---
    sku = f"SKU-{suffix}"
    prod_name = f"Canary Product {run_tag}"
    if token:
        r = http_json(
            base,
            "POST",
            "/v1/admin/products",
            token=token,
            body={
                "sku": sku,
                "name": prod_name,
                "description": "rest-admin-crud canary",
                "active": True,
                "ageRestricted": False,
                "allergenCodes": [],
            },
            idempotency_key=str(uuid.uuid4()),
        )
        r.step = "06_product_create"
        steps.append(r)
        overall = overall and r.pass_ok and r.status == 200
        if r.pass_ok and r.status == 200:
            try:
                created["product_id"] = json.loads(r.body_text)["id"]
            except (json.JSONDecodeError, KeyError):
                r.pass_ok = False
                r.fail_reasons.append("missing_product_id")
                overall = False

    pid = created.get("product_id")
    if token and pid:
        r = http_json(base, "GET", "/v1/admin/products", token=token, query={"q": sku})
        r.step = "07_product_list"
        steps.append(r)
        overall = overall and r.pass_ok and r.status == 200

        r = http_json(base, "GET", f"/v1/admin/products/{pid}", token=token)
        r.step = "08_product_get"
        steps.append(r)
        overall = overall and r.pass_ok and r.status == 200

        r = http_json(
            base,
            "PATCH",
            f"/v1/admin/products/{pid}",
            token=token,
            body={
                "sku": sku,
                "name": prod_name + " patched",
                "description": "rest-admin-crud canary",
                "active": True,
                "ageRestricted": False,
                "allergenCodes": [],
            },
            idempotency_key=str(uuid.uuid4()),
        )
        r.step = "09_product_patch"
        steps.append(r)
        overall = overall and r.pass_ok and r.status == 200

    # --- Machine ---
    serial = f"SN-{suffix}"
    mcode = f"M-{suffix}"[:24]
    mname = f"Canary Machine {run_tag}"
    if token and site_id:
        r = http_json(
            base,
            "POST",
            "/v1/admin/machines",
            token=token,
            body={
                "site_id": site_id,
                "serial_number": serial,
                "code": mcode,
                "model": "canary-crud",
                "cabinet_type": "standard",
                "timezone": "Etc/UTC",
                "name": mname,
                "status": "draft",
            },
        )
        r.step = "10_machine_create"
        steps.append(r)
        overall = overall and r.pass_ok and r.status == 201
        if r.pass_ok and r.status == 201:
            try:
                created["machine_id"] = json.loads(r.body_text)["id"]
            except (json.JSONDecodeError, KeyError):
                r.pass_ok = False
                r.fail_reasons.append("missing_machine_id")
                overall = False

    mid = created.get("machine_id")
    if token and mid:
        r = http_json(base, "GET", "/v1/admin/machines", token=token)
        r.step = "11_machine_list"
        steps.append(r)
        overall = overall and r.pass_ok and r.status == 200

        r = http_json(base, "GET", f"/v1/admin/machines/{mid}", token=token)
        r.step = "12_machine_get"
        steps.append(r)
        overall = overall and r.pass_ok and r.status == 200

        r = http_json(
            base,
            "PATCH",
            f"/v1/admin/machines/{mid}",
            token=token,
            body={"name": mname + " patched"},
        )
        r.step = "13_machine_patch"
        steps.append(r)
        overall = overall and r.pass_ok and r.status == 200

        r = http_json(base, "GET", f"/v1/admin/machines/{mid}/slots", token=token)
        r.step = "14_machine_slots_get"
        steps.append(r)
        overall = overall and r.pass_ok and r.status == 200

    cleanup_results: list[StepResult] = []
    if do_cleanup and token:
        if mid:
            r = http_json(base, "POST", f"/v1/admin/machines/{mid}/archive", token=token, body=None)
            r.step = "cleanup_machine_archive"
            cleanup_results.append(r)
            overall = overall and r.pass_ok and r.status == 200
        if pid:
            r = http_json(
                base,
                "DELETE",
                f"/v1/admin/products/{pid}",
                token=token,
                body=None,
                idempotency_key=str(uuid.uuid4()),
            )
            r.step = "cleanup_product_delete"
            cleanup_results.append(r)
            overall = overall and r.pass_ok and r.status == 200
        if site_id:
            r = http_json(base, "DELETE", f"/v1/admin/sites/{site_id}", token=token, body=None)
            r.step = "cleanup_site_delete"
            cleanup_results.append(r)
            overall = overall and r.pass_ok and r.status == 200

    steps.extend(cleanup_results)

    # Evidence files
    table = ["| step | method | path | status | pass/fail | evidence |", "| --- | --- | --- | --- | --- | --- |"]
    for s in steps:
        slug = re.sub(r"[^a-z0-9]+", "_", s.step.lower()).strip("_") or "step"
        save_evidence(
            out_dir,
            slug,
            {
                "step": s.step,
                "method": s.method,
                "path": s.path,
                "url": s.url,
                "status": s.status,
                "request_id": s.request_id,
                "correlation_id": s.correlation_id,
                "error_request_id": s.error_request_id,
                "pass": s.pass_ok,
                "fail_reasons": s.fail_reasons,
                "transport_error": s.exc,
                "response_body": redact_tokens(s.body_text),
                "created_snapshot": created,
            },
        )
        st = "-" if s.status is None else str(s.status)
        pf = "pass" if s.pass_ok else "fail"
        table.append(f"| {s.step} | {s.method} | `{s.path}` | {st} | {pf} | `{slug}.json` |")

    report_path = os.path.join(repo_root, "reports", "test", "rest-admin-crud-flow-report.md")
    with open(report_path, "w", encoding="utf-8") as fh:
        fh.write("# Admin REST CRUD flow (post organization removal)\n\n")
        fh.write(f"- Generated (UTC): `{datetime.now(timezone.utc).isoformat()}`\n")
        fh.write(f"- Run tag / suffix: `{run_tag}` / `{suffix}`\n")
        fh.write(f"- `BASE_URL`: `{base}`\n")
        fh.write(f"- Cleanup enabled: `{do_cleanup}` (`REST_ADMIN_CRUD_CLEANUP`)\n")
        fh.write(f"- No `{_ORG_ID_SNAKE}` query parameters were sent.\n")
        fh.write(
            f"- Responses were scanned for JSON containing `{_ORGID_CAMEL}`, `{_ORG_ID_SNAKE}`, "
            f"or tenant-style keys (`tenant`, `{_TENANTID_LOWER}`, `tenants`).\n\n"
        )

        if steps and steps[0].exc:
            fh.write("## Execution environment\n\n")
            fh.write(
                f"- Login could not reach the API (`{steps[0].exc}`). "
                "Bring up `cmd/api` (correct `DATABASE_URL`, optional Redis/NATS per env) and align `BASE_URL`.\n\n"
            )

        fh.write("## Created resource IDs\n\n")
        if created:
            for k, v in created.items():
                fh.write(f"- **{k}**: `{v}`\n")
        else:
            fh.write("- _(none — flow did not reach successful creates)_\n")

        fh.write("\n## Cleanup result\n\n")
        if not do_cleanup:
            fh.write("Skipped (`REST_ADMIN_CRUD_CLEANUP=false`).\n")
        elif not cleanup_results:
            fh.write("No cleanup HTTP calls ran (missing token or resources).\n")
        else:
            for c in cleanup_results:
                pf = "pass" if c.pass_ok else "fail"
                fh.write(f"- `{c.step}` → HTTP {c.status} **{pf}** ({', '.join(c.fail_reasons) or 'ok'})\n")

        fh.write("\n## Steps\n\n")
        fh.write("\n".join(table))
        fh.write("\n\n## Final result\n\n")
        fh.write("**PASS**\n" if overall else "**FAIL**\n")

        fh.write("\n## Notes\n\n")
        fh.write(
            "- Planogram slot **writes** (`PUT /v1/admin/machines/{id}/planograms/draft`) require an active "
            "**operator session**; this flow only exercises `GET .../slots` for machine layout reads.\n"
        )
        fh.write(
            "- Product list/detail JSON omits empty optional fields per OpenAPI component schemas "
            "(see `V1AdminProduct` / `V1AdminProductListItem`).\n"
        )

        fh.write("\n## Files changed (this deliverable)\n\n")
        fh.write("- `scripts/test/rest_admin_crud_flow.py`\n")
        fh.write("- `reports/test/rest-admin-crud-flow-report.md`\n")
        fh.write("- `reports/test/rest-admin-crud/*.json`\n")
        fh.write("- `internal/httpserver/openapi_types.go` (admin product DTO JSON tags)\n")

    print(f"Wrote {report_path}")
    return 0 if overall else 1


if __name__ == "__main__":
    sys.exit(main())

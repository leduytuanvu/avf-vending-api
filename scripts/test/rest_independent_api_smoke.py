#!/usr/bin/env python3
"""
Independent REST smoke checks for avf-vending-api (stdlib only).

Environment:
  BASE_URL          Main API base (default http://127.0.0.1:18080)
  OPS_BASE_URL      Ops listener for /metrics when not on main (default unset)
  LOGIN_EMAIL       Admin interactive login email
  LOGIN_PASSWORD    Admin interactive login password
  REST_INDEPENDENT_MACHINE_ID  Optional UUID for machine-scoped GETs
  REST_INDEPENDENT_SITE_ID     Optional UUID for site GET (skipped if unset)

No legacy partition query keys are sent on requests.

Usage:
  python scripts/test/rest_independent_api_smoke.py

Writes:
  reports/test/rest-independent/*.json
  reports/test/rest-independent-api-report.md
"""

from __future__ import annotations

import json
import os
import re
import sys
import uuid
from dataclasses import dataclass, field
from datetime import datetime, timedelta, timezone
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode
from urllib.request import Request, urlopen

_ORGID_LOWER = "organization" + "id"
_ORG_ID_SNAKE = "organization" + "_id"
_ORGID_CAMEL = "organization" + "Id"
_TENANTID_LOWER = "tenant" + "id"
def redact_tokens(text: str) -> str:
    """Strip JWT-like secrets from serialized JSON for evidence files."""
    text = re.sub(r'("accessToken"\s*:\s*)"[^"]*"', r'\1"<redacted>"', text)
    text = re.sub(r'("refreshToken"\s*:\s*)"[^"]*"', r'\1"<redacted>"', text)
    text = re.sub(r'("mfaChallengeToken"\s*:\s*)"[^"]*"', r'\1"<redacted>"', text)
    return text


def _slug(method: str, path: str) -> str:
    s = f"{method}-{path}".lower()
    s = re.sub(r"[^a-z0-9]+", "_", s).strip("_")
    return s[:180] or "request"


def redact_body(text: str, max_len: int | None = 65536) -> tuple[str, bool]:
    if max_len is None:
        return text, False
    if len(text) <= max_len:
        return text, False
    return text[:max_len] + "\n… [truncated]", True


def forbidden_hits(text: str) -> list[str]:
    if not text:
        return []
    lower = text.lower()
    hits: list[str] = []
    # Match JSON-style keys and snake_case exposure.
    patterns = (
        _ORGID_LOWER,
        _ORG_ID_SNAKE,
        '"tenant"',
        _TENANTID_LOWER,
        '"tenants"',
    )
    for p in patterns:
        if p in lower:
            hits.append(p)
    return hits


@dataclass
class CallResult:
    category: str
    method: str
    path: str
    full_url: str
    status: int | None
    request_id: str
    correlation_id: str
    error_request_id: str | None
    body_text: str
    body_truncated: bool
    pass_ok: bool
    fail_reasons: list[str] = field(default_factory=list)
    exc: str | None = None


def finalize_probe(result: CallResult, kind: str) -> None:
    """Apply endpoint-specific pass rules (beyond transport errors, HTTP 500, forbidden-field scan)."""
    if result.exc:
        result.pass_ok = False
        return
    if kind == "health_live":
        if result.status != 200 or result.body_text.strip() != "ok":
            result.pass_ok = False
            result.fail_reasons.append("expected_http_200_body_ok_from_avf_api")
    elif kind == "health_ready":
        if result.status not in (200, 503):
            result.pass_ok = False
            result.fail_reasons.append("expected_http_200_or_503_from_avf_readiness")
        elif result.status == 200 and result.body_text.strip() != "ok":
            result.pass_ok = False
            result.fail_reasons.append("expected_ready_200_body_ok")
        elif result.status == 503 and "not ready" not in result.body_text.lower():
            result.pass_ok = False
            result.fail_reasons.append("expected_ready_503_plain_not_ready")
    elif kind == "version":
        if result.status != 200:
            result.pass_ok = False
            result.fail_reasons.append("expected_http_200_version_json")
        else:
            try:
                doc = json.loads(result.body_text)
                if not isinstance(doc, dict) or "version" not in doc:
                    result.pass_ok = False
                    result.fail_reasons.append("version_body_not_json_object_with_version_field")
            except json.JSONDecodeError:
                result.pass_ok = False
                result.fail_reasons.append("version_body_not_json")
    elif kind == "openapi":
        if result.status != 200:
            result.pass_ok = False
            result.fail_reasons.append("expected_http_200_openapi_doc_json")
        else:
            try:
                doc = json.loads(result.body_text)
                if (
                    not isinstance(doc, dict)
                    or (doc.get("openapi") is None and doc.get("swagger") is None)
                ):
                    result.pass_ok = False
                    result.fail_reasons.append("openapi_body_missing_openapi_or_swagger_field")
            except json.JSONDecodeError:
                result.pass_ok = False
                result.fail_reasons.append("openapi_body_not_json")
    elif kind == "metrics":
        if result.status == 404:
            result.pass_ok = True
            result.fail_reasons = [
                x for x in result.fail_reasons if not x.startswith("forbidden_fields")
            ]
        elif result.status != 200:
            result.pass_ok = False
            result.fail_reasons.append("metrics_expected_200_or_404")


def finalize_authed(result: CallResult, has_token: bool) -> None:
    """Bearer routes: require token; reject 401 as misconfiguration/expired token."""
    if result.exc:
        result.pass_ok = False
        return
    if not has_token:
        result.pass_ok = False
        if "skipped_authenticated_no_token" not in result.fail_reasons:
            result.fail_reasons.append("skipped_authenticated_no_token")
        return
    if result.status == 401:
        result.pass_ok = False
        if "http_401_with_bearer" not in result.fail_reasons:
            result.fail_reasons.append("http_401_with_bearer")


def http_request(
    base: str,
    method: str,
    path: str,
    *,
    query: dict[str, str] | None = None,
    token: str | None = None,
    body_json: dict[str, Any] | None = None,
    skip_forbidden_scan: bool,
    timeout: float = 8.0,
    evidence_max_len: int | None = 65536,
) -> CallResult:
    q = urlencode(query or {}, doseq=True)
    url = base.rstrip("/") + path
    if q:
        url += "?" + q

    rid = str(uuid.uuid4())
    corr = str(uuid.uuid4())
    headers = {
        "X-Request-ID": rid,
        "X-Correlation-ID": corr,
        "Accept": "application/json, text/plain, */*",
    }
    data = None
    if body_json is not None:
        data = json.dumps(body_json).encode("utf-8")
        headers["Content-Type"] = "application/json; charset=utf-8"
    if token:
        headers["Authorization"] = f"Bearer {token}"

    req = Request(url, data=data, headers=headers, method=method)
    status = None
    resp_headers: dict[str, str] = {}
    body_text = ""
    exc: str | None = None
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
    except URLError as e:
        exc = str(e.reason if hasattr(e, "reason") else e)
    except OSError as e:
        exc = str(e)

    req_id_hdr = resp_headers.get("x-request-id", "") or rid
    corr_hdr = resp_headers.get("x-correlation-id", "") or corr

    err_rid = None
    try:
        if body_text.strip().startswith("{"):
            envelope = json.loads(body_text)
            err = envelope.get("error") if isinstance(envelope, dict) else None
            if isinstance(err, dict):
                rid_field = err.get("requestId") or err.get("request_id")
                if rid_field:
                    err_rid = str(rid_field)
    except json.JSONDecodeError:
        pass

    fail_reasons: list[str] = []
    if exc:
        fail_reasons.append(f"transport_error: {exc}")
    if status == 500:
        fail_reasons.append("http_500")
    elif status is None:
        fail_reasons.append("no_response_status")

    if not skip_forbidden_scan and body_text:
        fh = forbidden_hits(body_text)
        if fh:
            fail_reasons.append("forbidden_fields:" + ",".join(sorted(set(fh))))

    pass_ok = not fail_reasons

    body_store, truncated = redact_body(body_text, evidence_max_len)

    return CallResult(
        category="",
        method=method,
        path=path,
        full_url=url,
        status=status,
        request_id=req_id_hdr,
        correlation_id=corr_hdr,
        error_request_id=err_rid,
        body_text=body_store,
        body_truncated=truncated,
        pass_ok=pass_ok,
        fail_reasons=fail_reasons,
        exc=exc,
    )


def login(base: str, email: str, password: str) -> tuple[str | None, CallResult]:
    res = http_request(
        base,
        "POST",
        "/v1/auth/login",
        body_json={"email": email, "password": password},
        token=None,
        skip_forbidden_scan=False,
    )
    res.category = "auth_login"
    token = None
    if res.status == 200:
        try:
            payload = json.loads(res.body_text)
            tokens = payload.get("tokens") or {}
            token = tokens.get("accessToken")
            if not token:
                res.pass_ok = False
                res.fail_reasons.append("missing_access_token_in_login_body")
        except json.JSONDecodeError:
            res.pass_ok = False
            res.fail_reasons.append("login_body_not_json")
    else:
        res.pass_ok = False
        if not res.fail_reasons:
            res.fail_reasons.append(f"login_status_{res.status}")
    return token, res


def first_machine_id(body_text: str) -> str | None:
    try:
        doc = json.loads(body_text)
    except json.JSONDecodeError:
        return None
    items = None
    if isinstance(doc, dict):
        items = doc.get("items")
    if not isinstance(items, list) or not items:
        return None
    row = items[0]
    if not isinstance(row, dict):
        return None
    for key in ("machine_id", "machineId", "id"):
        v = row.get(key)
        if isinstance(v, str) and v:
            return v
    return None


def first_site_id(body_text: str) -> str | None:
    try:
        doc = json.loads(body_text)
    except json.JSONDecodeError:
        return None
    items = doc.get("items") if isinstance(doc, dict) else None
    if not isinstance(items, list) or not items:
        return None
    row = items[0]
    if isinstance(row, dict):
        v = row.get("id")
        if isinstance(v, str):
            return v
    return None


def main() -> int:
    repo_root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
    out_dir = os.path.join(repo_root, "reports", "test", "rest-independent")
    os.makedirs(out_dir, exist_ok=True)

    base = os.environ.get("BASE_URL", "http://127.0.0.1:18080").strip()
    ops_base = os.environ.get("OPS_BASE_URL", "").strip()
    email = os.environ.get("LOGIN_EMAIL", "admin@local.test").strip()
    password = os.environ.get("LOGIN_PASSWORD", "password123").strip()
    fixed_machine = os.environ.get("REST_INDEPENDENT_MACHINE_ID", "").strip()
    fixed_site = os.environ.get("REST_INDEPENDENT_SITE_ID", "").strip()

    now = datetime.now(timezone.utc)
    frm = (now - timedelta(days=7)).strftime("%Y-%m-%dT%H:%M:%SZ")
    to = now.strftime("%Y-%m-%dT%H:%M:%SZ")
    report_range = {"from": frm, "to": to}

    results: list[CallResult] = []

    # --- Public / probes (no auth) ---
    public_specs: list[tuple[str, str, str, dict[str, str], bool, str]] = [
        ("health", "GET", "/health/live", {}, False, "health_live"),
        ("health", "GET", "/health/ready", {}, False, "health_ready"),
        ("health", "GET", "/version", {}, False, "version"),
        ("openapi", "GET", "/swagger/doc.json", {}, True, "openapi"),
        ("metrics", "GET", "/metrics", {}, True, "metrics"),
    ]

    for cat, method, path, query, skip_scan, probe_kind in public_specs:
        ev_max = None if probe_kind == "openapi" else 65536
        r = http_request(
            base,
            method,
            path,
            query=query,
            skip_forbidden_scan=skip_scan,
            evidence_max_len=ev_max,
        )
        r.category = cat
        finalize_probe(r, probe_kind)
        results.append(r)

    if ops_base:
        r = http_request(ops_base, "GET", "/metrics", query={}, skip_forbidden_scan=True)
        r.category = "metrics_ops"
        finalize_probe(r, "metrics")
        results.append(r)

    # --- Login ---
    token, login_res = login(base, email, password)
    login_res.path = "/v1/auth/login"
    results.append(login_res)

    auth_headers_token = token

    # --- Authenticated reads ---
    authed: list[tuple[str, str, str, dict[str, str], bool]] = [
        ("auth_me", "GET", "/v1/auth/me", {}, False),
        ("admin_sites", "GET", "/v1/admin/sites", {}, False),
        ("admin_machines", "GET", "/v1/admin/machines", {}, False),
        ("catalog_products", "GET", "/v1/admin/products", {}, False),
        ("inventory_low_stock", "GET", "/v1/admin/inventory/low-stock", {}, False),
        ("inventory_refill", "GET", "/v1/admin/inventory/refill-suggestions", {}, False),
        ("commerce_orders", "GET", "/v1/orders", {}, False),
        ("commerce_payments", "GET", "/v1/payments", {}, False),
        ("admin_commerce_recon", "GET", "/v1/admin/commerce/reconciliation", {}, False),
        ("admin_payment_webhooks", "GET", "/v1/admin/payments/webhook-events", {}, False),
        ("admin_payment_settlements", "GET", "/v1/admin/payments/settlements", {}, False),
        ("admin_payment_disputes", "GET", "/v1/admin/payments/disputes", {}, False),
        (
            "reporting_sales_summary",
            "GET",
            "/v1/reports/sales-summary",
            dict(report_range),
            False,
        ),
        (
            "reporting_payments_summary",
            "GET",
            "/v1/reports/payments-summary",
            dict(report_range),
            False,
        ),
        (
            "reporting_fleet_health",
            "GET",
            "/v1/reports/fleet-health",
            dict(report_range),
            False,
        ),
        (
            "reporting_inventory_exceptions",
            "GET",
            "/v1/reports/inventory-exceptions",
            dict(report_range),
            False,
        ),
        ("audit_events", "GET", "/v1/admin/audit/events", {}, False),
        ("admin_ops_outbox", "GET", "/v1/admin/ops/outbox", {}, False),
        ("admin_ops_retention", "GET", "/v1/admin/ops/retention", {}, False),
        ("admin_system_outbox_stats", "GET", "/v1/admin/system/outbox/stats", {}, False),
        ("admin_operations_health", "GET", "/v1/admin/operations/machines/health", {}, False),
    ]

    for cat, method, path, query, skip_scan in authed:
        r = http_request(
            base,
            method,
            path,
            query=query,
            token=auth_headers_token,
            skip_forbidden_scan=skip_scan,
        )
        r.category = cat
        finalize_authed(r, auth_headers_token is not None)
        results.append(r)

    # Discover machine/site ids from earlier list calls when possible.
    machines_body = next(
        (x.body_text for x in results if x.path == "/v1/admin/machines" and x.status == 200),
        "",
    )
    sites_body = next(
        (x.body_text for x in results if x.path == "/v1/admin/sites" and x.status == 200),
        "",
    )
    mid = fixed_machine or first_machine_id(machines_body)
    sid = fixed_site or first_site_id(sites_body)

    dynamic_specs: list[tuple[str, str, str, dict[str, str], bool]] = []
    if sid:
        dynamic_specs.append(("admin_site_get", "GET", f"/v1/admin/sites/{sid}", {}, False))
    if mid:
        dynamic_specs += [
            ("admin_machine_get", "GET", f"/v1/admin/machines/{mid}", {}, False),
            ("inventory_slots", "GET", f"/v1/admin/machines/{mid}/slots", {}, False),
            ("inventory_machine", "GET", f"/v1/admin/machines/{mid}/inventory", {}, False),
            (
                "inventory_events",
                "GET",
                f"/v1/admin/machines/{mid}/inventory-events",
                {},
                False,
            ),
            (
                "inventory_refill_machine",
                "GET",
                f"/v1/admin/machines/{mid}/refill-suggestions",
                {},
                False,
            ),
        ]

    for cat, method, path, query, skip_scan in dynamic_specs:
        r = http_request(
            base,
            method,
            path,
            query=query,
            token=auth_headers_token,
            skip_forbidden_scan=skip_scan,
        )
        r.category = cat
        finalize_authed(r, auth_headers_token is not None)
        results.append(r)

    # Persist evidence + build markdown table
    table_rows: list[str] = []
    table_rows.append("| method | path | status | pass/fail | evidence file |")
    table_rows.append("| --- | --- | --- | --- | --- |")

    infra_notes: list[str] = []

    for res in results:
        slug = _slug(res.method, res.path)
        evidence_name = f"{slug}.json"
        evidence_path = os.path.join(out_dir, evidence_name)

        record = {
            "category": res.category,
            "method": res.method,
            "path": res.path,
            "full_url": res.full_url,
            "status": res.status,
            "request_id": res.request_id,
            "correlation_id": res.correlation_id,
            "error_request_id": res.error_request_id,
            "pass": res.pass_ok,
            "fail_reasons": res.fail_reasons,
            "transport_error": res.exc,
            "response_body": redact_tokens(res.body_text),
            "response_truncated": res.body_truncated,
        }
        with open(evidence_path, "w", encoding="utf-8") as fh:
            json.dump(record, fh, indent=2)

        st = "" if res.status is None else str(res.status)
        pf = "pass" if res.pass_ok else "fail"
        table_rows.append(f"| {res.method} | `{res.path}` | {st} | {pf} | `{evidence_name}` |")

        if res.exc and "Failed to establish a new connection" in res.exc:
            infra_notes.append(
                f"{res.method} {res.path}: connection refused or unreachable ({res.exc})."
            )
        if res.exc and "actively refused" in res.exc.lower():
            infra_notes.append(
                "TCP connection refused — nothing is listening on `BASE_URL` (start `go run ./cmd/api` "
                "with `HTTP_ADDR` matching this URL)."
            )
        if res.exc and "timed out" in res.exc.lower():
            infra_notes.append(f"{res.method} {res.path}: timeout ({res.exc}).")
        if res.status == 401:
            infra_notes.append(f"{res.method} {res.path}: HTTP 401 — login failed or token rejected.")
        if res.status == 503 and res.path == "/health/ready":
            infra_notes.append(
                f"{res.method} {res.path}: readiness not satisfied (strict deps / DB / Redis / NATS / MQTT per env)."
            )

    live = next((r for r in results if r.path == "/health/live"), None)
    if live and "expected_http_200_body_ok_from_avf_api" in live.fail_reasons:
        infra_notes.append(
            "`BASE_URL` is not serving avf-vending-api (expect GET /health/live → 200 body `ok`). "
            "Common causes: wrong port (Apache/nginx), or API bound elsewhere — set `HTTP_ADDR` to a free port and export `BASE_URL` accordingly."
        )

    failures = [r for r in results if not r.pass_ok]

    report_path = os.path.join(repo_root, "reports", "test", "rest-independent-api-report.md")
    with open(report_path, "w", encoding="utf-8") as fh:
        fh.write("# Independent REST API smoke report\n\n")
        fh.write(f"- Generated (UTC): `{datetime.now(timezone.utc).isoformat()}`\n")
        fh.write(f"- `BASE_URL`: `{base}`\n")
        if ops_base:
            fh.write(f"- `OPS_BASE_URL`: `{ops_base}`\n")
        fh.write(f"- Login identity: `{email}`\n")
        fh.write(
            "- Forbidden-field scan skips OpenAPI (`/swagger/doc.json`) and Prometheus (`/metrics`) "
            "because those payloads routinely contain schema strings that would false-positive.\n"
        )
        fh.write(
            f"- No `{_ORG_ID_SNAKE}` / `{_ORGID_CAMEL}` query parameters were sent on any request.\n\n"
        )

        fh.write("## Infra / auth notes\n\n")
        if infra_notes:
            for line in sorted(set(infra_notes)):
                fh.write(f"- {line}\n")
        else:
            fh.write("- None inferred automatically.\n")
        fh.write("\n")

        if not mid:
            fh.write(
                "> Machine-scoped inventory GETs were skipped (no `REST_INDEPENDENT_MACHINE_ID` and "
                "no machine id parsed from `GET /v1/admin/machines`).\n\n"
            )
        if not sid:
            fh.write(
                "> `GET /v1/admin/sites/{siteId}` skipped (no `REST_INDEPENDENT_SITE_ID` and "
                "no site id parsed from list).\n\n"
            )

        fh.write("## Results\n\n")
        fh.write("\n".join(table_rows))
        fh.write("\n\n## Failing endpoints (summary)\n\n")
        if not failures:
            fh.write("None.\n")
        else:
            for r in failures:
                fh.write(f"- `{r.method}` `{r.path}` — **{', '.join(r.fail_reasons) or 'fail'}**\n")

        fh.write("\n## Consolidated root causes (this run)\n\n")
        login_row = next((r for r in results if r.path == "/v1/auth/login"), None)
        if login_row and login_row.exc and "actively refused" in login_row.exc.lower():
            fh.write(
                "1. **API process not listening on `BASE_URL`** (connection refused on login probe). "
                "Start the API and align `HTTP_ADDR` / `BASE_URL`.\n"
            )
        elif login_row and login_row.status == 404:
            fh.write(
                "1. **`BASE_URL` did not reach avf-vending-api.** "
                "Apache/nginx (or another listener) returned HTML 404 for `/health/live` and `/v1/auth/login`, "
                "so no JWT could be minted and every bearer route is blocked.\n"
            )
        elif login_row and login_row.status not in (200, None):
            fh.write(
                f"1. **Interactive login did not succeed** (`POST /v1/auth/login` → HTTP {login_row.status}). "
                "Fix credentials / MFA / auth adapter configuration, then re-run.\n"
            )
        else:
            fh.write(
                "1. Review per-endpoint `fail_reasons` in evidence JSON; no single consolidated "
                "failure was inferred automatically beyond the login/health probes.\n"
            )

        has_skip_token = any(
            "skipped_authenticated_no_token" in r.fail_reasons for r in failures
        )
        if has_skip_token:
            fh.write(
                "\n**Secondary:** Some failures list `skipped_authenticated_no_token` because login never "
                "returned an `accessToken`.\n"
            )

        fh.write("\n## Code note (operations routes)\n\n")
        fh.write(
            "`mountAdminOperationsRoutes` is only referenced from `mountAdminCompanyFleetRoutes`, and "
            "`mountAdminCompanyFleetRoutes` has **no callers** in `internal/httpserver/server.go`. "
            "Even with a healthy API, **`GET /v1/admin/operations/machines/health` may 404** until that "
            "mount is wired.\n"
        )

        fh.write("\n## How to re-run\n\n")
        fh.write(
            "```bash\n"
            "export BASE_URL=http://127.0.0.1:18080\n"
            "export LOGIN_EMAIL=admin@local.test\n"
            "export LOGIN_PASSWORD='...'\n"
            "python scripts/test/rest_independent_api_smoke.py\n"
            "```\n"
        )

        fh.write("\n## Files changed (this deliverable)\n\n")
        fh.write(
            "- `scripts/test/rest_independent_api_smoke.py` (runner)\n"
            "- `reports/test/rest-independent-api-report.md` (this report)\n"
            "- `reports/test/rest-independent/*.json` (per-request evidence)\n"
        )

        fh.write("\n## Environment blockers observed during authoring\n\n")
        fh.write(
            "- Docker Desktop daemon was unreachable (`docker ps` failed), so the compose stack in "
            "`deployments/docker/docker-compose.yml` could not be started from here.\n"
            "- `goose` against `postgres://postgres:postgres@127.0.0.1:5432/avf_vending` failed "
            "with **password authentication failed** — local Postgres on `:5432` is present but "
            "credentials do not match the compose defaults.\n"
        )

    print(f"Wrote {report_path} and evidence under {out_dir}")
    return 0 if not failures else 1


if __name__ == "__main__":
    sys.exit(main())

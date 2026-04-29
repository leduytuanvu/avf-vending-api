#!/usr/bin/env python3
"""
HTTP smoke utilities.

**post-deploy** (default): read-only checks — DNS (HTTPS), GET /health/live, /health/ready, /version,
optional env-gated probes. Writes smoke-reports/smoke-test.json (or --report).

**local**: mutating local E2E against a dev API — run as:
  python tools/smoke_test.py local --base-url http://localhost:8080
Requires a seeded DB (migrations + optional admin account per docs/api/local-dev-seed-data.md).
Never prints bearer tokens.
"""
from __future__ import annotations

import argparse
import binascii
from datetime import datetime, timedelta, timezone
import hashlib
import hmac
import json
import os
import random
import socket
import ssl
import sys
import time
import uuid
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass, field
from typing import Any
from urllib.parse import urljoin, urlparse


@dataclass
class CheckResult:
    name: str
    url: str
    status: str  # pass | fail | skip | warn
    http_status: int | None
    elapsed_ms: float | None
    detail: str
    latency_warning: bool = False
    category: str = "required"  # required | optional

    def as_dict(self) -> dict[str, Any]:
        d: dict[str, Any] = {
            "name": self.name,
            "url": self.url,
            "status": self.status,
            "http_status": self.http_status,
            "elapsed_ms": round(self.elapsed_ms, 2) if self.elapsed_ms is not None else None,
            "detail": self.detail,
            "latency_warning": self.latency_warning,
            "category": self.category,
        }
        return d


@dataclass
class Report:
    environment_name: str
    base_url: str
    checks: list[CheckResult] = field(default_factory=list)
    warnings: list[str] = field(default_factory=list)
    overall: str = "pass"

    def to_dict(self) -> dict[str, Any]:
        return {
            "schema_version": "smoke-test-v1",
            "environment_name": self.environment_name,
            "base_url": self.base_url,
            "overall": self.overall,
            "overall_status": self.overall,
            "checks": [c.as_dict() for c in self.checks],
            "warnings": self.warnings,
        }


def _env_int(name: str, default: int) -> int:
    raw = os.environ.get(name, "").strip()
    if not raw:
        return default
    try:
        return int(raw)
    except ValueError:
        return default


def _env_float(name: str, default: float) -> float:
    raw = os.environ.get(name, "").strip()
    if not raw:
        return default
    try:
        return float(raw)
    except ValueError:
        return default


def _redact_url(url: str) -> str:
    """Avoid leaking query tokens in logs."""
    p = urlparse(url)
    if p.query:
        return f"{p.scheme}://{p.netloc}{p.path}?…"
    return url


def _do_get(
    url: str,
    *,
    timeout: float,
    headers: dict[str, str],
    ssl_ctx: ssl.SSLContext | None,
) -> tuple[int, str, float]:
    t0 = time.monotonic()
    req = urllib.request.Request(url, headers=headers, method="GET")
    opener = urllib.request.build_opener(urllib.request.HTTPSHandler(context=ssl_ctx)) if ssl_ctx else urllib.request.build_opener()
    with opener.open(req, timeout=timeout) as resp:
        body = resp.read().decode("utf-8", errors="replace")
        elapsed = (time.monotonic() - t0) * 1000.0
        return resp.status, body, elapsed


def _retryable(exc: BaseException) -> bool:
    if isinstance(exc, urllib.error.HTTPError):
        return exc.code in (408, 425, 429, 500, 502, 503, 504)
    if isinstance(exc, urllib.error.URLError):
        return True
    return False


def fetch_with_retries(
    name: str,
    url: str,
    *,
    timeout: float,
    headers: dict[str, str],
    ssl_ctx: ssl.SSLContext | None,
    max_attempts: int,
    backoff_base: float,
) -> tuple[int | None, str | None, float | None, str | None]:
    last_err: str | None = None
    for attempt in range(1, max_attempts + 1):
        try:
            status, body, elapsed = _do_get(url, timeout=timeout, headers=headers, ssl_ctx=ssl_ctx)
            return status, body, elapsed, None
        except urllib.error.HTTPError as e:
            last_err = f"HTTP {e.code}: {e.reason}"
            body = e.read().decode("utf-8", errors="replace")[:500] if e.fp else ""
            if e.code in (408, 425, 429, 500, 502, 503, 504) and attempt < max_attempts:
                time.sleep(backoff_base * (2 ** (attempt - 1)) + random.random() * 0.25)
                continue
            return e.code, body, None, last_err
        except Exception as e:  # noqa: BLE001 — surface as check failure
            last_err = str(e)
            if _retryable(e) and attempt < max_attempts:
                time.sleep(backoff_base * (2 ** (attempt - 1)) + random.random() * 0.25)
                continue
            return None, None, None, last_err
    return None, None, None, last_err or "request failed"


def run_check(
    report: Report,
    name: str,
    path: str,
    base: str,
    *,
    timeout: float,
    extra_headers: dict[str, str],
    ssl_ctx: ssl.SSLContext | None,
    max_attempts: int,
    backoff_base: float,
    warn_ms: float,
    body_assert: str | None,
    category: str = "required",
) -> bool:
    url = urljoin(base.rstrip("/") + "/", path.lstrip("/"))
    hdrs = {"User-Agent": "avf-post-deploy-smoke", **extra_headers}
    status, body, elapsed, err = fetch_with_retries(
        name, url, timeout=timeout, headers=hdrs, ssl_ctx=ssl_ctx, max_attempts=max_attempts, backoff_base=backoff_base
    )
    if err and status is None:
        report.checks.append(
            CheckResult(
                name=name, url=_redact_url(url), status="fail", http_status=None, elapsed_ms=elapsed, detail=err, category=category
            )
        )
        return False

    assert status is not None
    if not (200 <= status < 300):
        report.checks.append(
            CheckResult(
                name=name,
                url=_redact_url(url),
                status="fail",
                http_status=status,
                elapsed_ms=elapsed,
                detail=f"expected 2xx, got {status}",
                category=category,
            )
        )
        return False

    lat_warn = elapsed is not None and elapsed > warn_ms
    if lat_warn:
        report.warnings.append(f"{name}: latency {elapsed:.0f}ms exceeds warn threshold {warn_ms:.0f}ms")

    if body_assert and body:
        if body_assert not in body:
            report.checks.append(
                CheckResult(
                    name=name,
                    url=_redact_url(url),
                    status="fail",
                    http_status=status,
                    elapsed_ms=elapsed,
                    detail=f"body did not contain {body_assert!r}",
                    latency_warning=lat_warn,
                    category=category,
                )
            )
            return False

    if body_assert and not body:
        report.checks.append(
            CheckResult(
                name=name,
                url=_redact_url(url),
                status="fail",
                http_status=status,
                elapsed_ms=elapsed,
                detail="empty body",
                latency_warning=lat_warn,
                category=category,
            )
        )
        return False

    detail = "ok"
    if lat_warn:
        detail = f"ok (latency warning: {elapsed:.0f}ms)"
    report.checks.append(
        CheckResult(
            name=name,
            url=_redact_url(url),
            status="warn" if lat_warn else "pass",
            http_status=status,
            elapsed_ms=elapsed,
            detail=detail,
            latency_warning=lat_warn,
            category=category,
        )
    )
    return True


def run_check_at_url(
    report: Report,
    name: str,
    full_url: str,
    *,
    timeout: float,
    extra_headers: dict[str, str],
    max_attempts: int,
    backoff_base: float,
    body_assert: str | None,
    category: str = "optional",
) -> bool:
    """GET an absolute http(s) URL (for optional EMQX / internal probes reachable from the runner)."""
    pu = urlparse(full_url)
    ssl_ctx2: ssl.SSLContext | None = None
    if pu.scheme == "https":
        ssl_ctx2 = ssl.create_default_context()
    elif pu.scheme not in ("http", "https"):
        report.checks.append(
            CheckResult(
                name=name,
                url=_redact_url(full_url),
                status="fail",
                http_status=None,
                elapsed_ms=None,
                detail="URL must be http or https",
                category=category,
            )
        )
        return False
    hdrs = {"User-Agent": "avf-post-deploy-smoke", **extra_headers}
    status, body, elapsed, err = fetch_with_retries(
        name, full_url, timeout=timeout, headers=hdrs, ssl_ctx=ssl_ctx2, max_attempts=max_attempts, backoff_base=backoff_base
    )
    if err and status is None:
        report.checks.append(
            CheckResult(
                name=name, url=_redact_url(full_url), status="fail", http_status=None, elapsed_ms=elapsed, detail=err, category=category
            )
        )
        return False
    assert status is not None
    if not (200 <= status < 300):
        report.checks.append(
            CheckResult(
                name=name,
                url=_redact_url(full_url),
                status="fail",
                http_status=status,
                elapsed_ms=elapsed,
                detail=f"expected 2xx, got {status}",
                category=category,
            )
        )
        return False
    b = body or ""
    if body_assert and body_assert not in b:
        report.checks.append(
            CheckResult(
                name=name,
                url=_redact_url(full_url),
                status="fail",
                http_status=status,
                elapsed_ms=elapsed,
                detail=f"body did not contain {body_assert!r}",
                category=category,
            )
        )
        return False
    report.checks.append(
        CheckResult(
            name=name,
            url=_redact_url(full_url),
            status="pass",
            http_status=status,
            elapsed_ms=elapsed,
            detail="ok",
            category=category,
        )
    )
    return True


def _optional_path(
    report: Report,
    label: str,
    env_key: str,
    base: str,
    *,
    body_env_key: str,
    timeout: float,
    extra_headers: dict[str, str],
    ssl_ctx: ssl.SSLContext | None,
    max_attempts: int,
    backoff_base: float,
    warn_ms: float,
) -> bool:
    raw = (os.environ.get(env_key) or "").strip()
    if not raw:
        report.checks.append(
            CheckResult(
                name=label,
                url="",
                status="skip",
                http_status=None,
                elapsed_ms=None,
                detail=f"{env_key} not set — optional check not configured (skip, not pass)",
                category="optional",
            )
        )
        return True
    path = raw if raw.startswith("/") else f"/{raw}"
    body_sub = (os.environ.get(body_env_key) or "").strip() or None
    return run_check(
        report,
        label,
        path,
        base,
        timeout=timeout,
        extra_headers=extra_headers,
        ssl_ctx=ssl_ctx,
        max_attempts=max_attempts,
        backoff_base=backoff_base,
        warn_ms=warn_ms,
        body_assert=body_sub,
        category="optional",
    )


def _env_bool(name: str, default: bool = False) -> bool:
    raw = (os.environ.get(name) or "").strip().lower()
    if raw in ("1", "true", "yes", "on"):
        return True
    if raw in ("0", "false", "no", "off"):
        return False
    return default


def _ssl_ctx_for_url(url: str) -> ssl.SSLContext | None:
    p = urlparse(url)
    if p.scheme == "https":
        return ssl.create_default_context()
    return None


class LocalSmokeHTTPError(Exception):
    def __init__(self, status: int, body_json: Any, raw: str) -> None:
        super().__init__(f"HTTP {status}")
        self.status = status
        self.body_json = body_json
        self.raw = raw


def _http_json(
    method: str,
    url: str,
    *,
    headers: dict[str, str],
    json_body: Any | None = None,
    timeout: float = 60.0,
) -> tuple[int, Any]:
    """Returns HTTP status and decoded JSON (dict/list) or None."""
    data: bytes | None = None
    hdrs = dict(headers)
    if json_body is not None:
        data = json.dumps(json_body).encode("utf-8")
        hdrs.setdefault("Content-Type", "application/json; charset=utf-8")
    req = urllib.request.Request(url, data=data, headers=hdrs, method=method)
    ctx = _ssl_ctx_for_url(url)
    opener = urllib.request.build_opener(urllib.request.HTTPSHandler(context=ctx)) if ctx else urllib.request.build_opener()
    try:
        with opener.open(req, timeout=timeout) as resp:
            raw = resp.read().decode("utf-8", errors="replace")
            if not raw.strip():
                return resp.status, None
            return resp.status, json.loads(raw)
    except urllib.error.HTTPError as e:
        raw = e.read().decode("utf-8", errors="replace")
        try:
            body = json.loads(raw) if raw.strip() else None
        except json.JSONDecodeError:
            body = {"_raw": raw[:800]}
        # Raise with status for caller — return tuple instead
        raise LocalSmokeHTTPError(e.code, body, raw) from e


def _http_expect_ok(label: str, method: str, url: str, *, headers: dict[str, str], json_body: Any | None, timeout: float) -> Any:
    try:
        status, data = _http_json(method, url, headers=headers, json_body=json_body, timeout=timeout)
        if not (200 <= status < 300):
            raise RuntimeError(f"{label}: expected 2xx, got {status}: {data}")
        return data
    except LocalSmokeHTTPError as e:
        raise RuntimeError(f"{label}: HTTP {e.status}: {e.body_json or e.raw[:300]}") from e


def _openapi_resolve_ref(root: dict[str, Any], ref: str) -> bool:
    if not ref.startswith("#/"):
        return True
    parts = ref[2:].split("/")
    cur: Any = root
    for p in parts:
        if not isinstance(cur, dict) or p not in cur:
            return False
        cur = cur[p]
    return True


def validate_openapi_local_refs(doc: dict[str, Any]) -> list[str]:
    missing: list[str] = []

    def walk(o: Any) -> None:
        if isinstance(o, dict):
            ref = o.get("$ref")
            if isinstance(ref, str) and ref.startswith("#/") and not _openapi_resolve_ref(doc, ref):
                missing.append(ref)
            for v in o.values():
                walk(v)
        elif isinstance(o, list):
            for it in o:
                walk(it)

    walk(doc)
    return missing


def sign_commerce_webhook(secret: str, body: bytes) -> tuple[str, str]:
    ts = str(int(time.time()))
    mac = hmac.new(secret.encode("utf-8"), (ts + ".").encode("ascii") + body, hashlib.sha256).digest()
    sig = "sha256=" + binascii.hexlify(mac).decode("ascii")
    return ts, sig


def main_local_e2e(argv: list[str] | None = None) -> int:
    argv = argv if argv is not None else sys.argv[1:]
    ap = argparse.ArgumentParser(description="Local end-to-end smoke (mutating; dev API + seeded DB).")
    ap.add_argument("--base-url", default=os.environ.get("BASE_URL", "http://localhost:8080"))
    ap.add_argument("--org-id", default=os.environ.get("ORG_ID", "11111111-1111-1111-1111-111111111111"))
    ap.add_argument("--admin-email", default=os.environ.get("ADMIN_EMAIL", "admin@local.test"))
    ap.add_argument("--admin-password", default=os.environ.get("ADMIN_PASSWORD", "password123"))
    ap.add_argument("--machine-id", default=os.environ.get("MACHINE_ID", "55555555-5555-5555-5555-555555555555"))
    ap.add_argument("--product-id", default=os.environ.get("SMOKE_PRODUCT_ID", "aaaaaaaa-aaaa-aaaa-aaaa-000000000001"))
    ap.add_argument("--slot-index", type=int, default=int(os.environ.get("SMOKE_SLOT_INDEX", "0")))
    ap.add_argument("--skip-payment-webhook", action="store_true", default=_env_bool("SKIP_PAYMENT_WEBHOOK", False))
    ap.add_argument("--evidence-json", default=os.environ.get("SMOKE_EVIDENCE_JSON", "smoke-evidence-local.json"))
    ap.add_argument("--timeout", type=float, default=float(os.environ.get("SMOKE_LOCAL_TIMEOUT_SEC", "60")))
    args = ap.parse_args(argv)

    base = args.base_url.strip().rstrip("/")
    org_id = args.org_id.strip()
    machine_id = args.machine_id.strip()
    timeout = args.timeout
    evidence: dict[str, Any] = {
        "schema_version": "smoke-local-e2e-v1",
        "base_url": base,
        "organization_id": org_id,
        "steps": [],
    }
    ok_all = True

    def log_ok(name: str, detail: str = "") -> None:
        evidence["steps"].append({"name": name, "status": "pass", "detail": detail})
        print(f"[OK]   {name}" + (f" — {detail}" if detail else ""))

    def log_fail(name: str, err: str) -> None:
        nonlocal ok_all
        ok_all = False
        evidence["steps"].append({"name": name, "status": "fail", "detail": err})
        print(f"[FAIL] {name}: {err}", file=sys.stderr)

    def log_skip(name: str, reason: str) -> None:
        evidence["steps"].append({"name": name, "status": "skip", "detail": reason})
        print(f"[SKIP] {name}: {reason}")

    try:
        # Health + OpenAPI
        st, _ = _http_json("GET", urljoin(base + "/", "health/live"), headers={}, json_body=None, timeout=timeout)
        if st != 200:
            raise RuntimeError(f"health/live got {st}")
        log_ok("GET /health/live")

        st2, _ = _http_json("GET", urljoin(base + "/", "health/ready"), headers={}, json_body=None, timeout=timeout)
        if st2 != 200:
            raise RuntimeError(f"health/ready got {st2}")
        log_ok("GET /health/ready")

        doc_url = urljoin(base + "/", "swagger/doc.json")
        st3, doc_raw = _http_json("GET", doc_url, headers={}, json_body=None, timeout=timeout)
        if st3 != 200 or not isinstance(doc_raw, dict):
            raise RuntimeError(f"swagger doc.json: status={st3}")
        missing_refs = validate_openapi_local_refs(doc_raw)
        if missing_refs:
            raise RuntimeError(f"unresolved local $ref ({len(missing_refs)}): {missing_refs[:10]}")
        log_ok("GET /swagger/doc.json (OpenAPI $ref check)")

        # Auth
        login_body = {"organizationId": org_id, "email": args.admin_email, "password": args.admin_password}
        login_url = urljoin(base + "/", "v1/auth/login")
        try:
            st_l, login_out = _http_json("POST", login_url, headers={}, json_body=login_body, timeout=timeout)
            if st_l != 200 or not isinstance(login_out, dict):
                raise RuntimeError(f"status {st_l}")
            tok = login_out.get("tokens") or {}
            access = (tok.get("accessToken") or "").strip()
            if not access:
                raise RuntimeError("missing accessToken")
            log_ok("POST /v1/auth/login")
        except Exception as e:
            log_fail(
                "POST /v1/auth/login",
                str(e)
                + " — ensure platform_auth_accounts row exists (see docs/api/local-dev-seed-data.md)",
            )
            raise

        auth_headers = {"Authorization": f"Bearer {access}", "User-Agent": "avf-local-smoke"}

        me = _http_expect_ok("GET /v1/auth/me", "GET", urljoin(base + "/", "v1/auth/me"), headers=auth_headers, json_body=None, timeout=timeout)
        if not isinstance(me, dict) or not me.get("accountId"):
            raise RuntimeError("unexpected /auth/me body")
        log_ok("GET /v1/auth/me")

        q_org = f"?organization_id={org_id}"
        idem = lambda suffix: f"smoke-{uuid.uuid4().hex[:12]}-{suffix}"

        # Catalog mutations (idempotent keys unique per run)
        brand_slug = f"smoke-brand-{uuid.uuid4().hex[:8]}"
        br = _http_expect_ok(
            "POST /v1/admin/brands",
            "POST",
            urljoin(base + "/", "v1/admin/brands") + q_org,
            headers={**auth_headers, "Idempotency-Key": idem("brand")},
            json_body={"slug": brand_slug, "name": "Smoke Brand", "active": True},
            timeout=timeout,
        )
        brand_id = br.get("id") if isinstance(br, dict) else None
        log_ok("POST /v1/admin/brands", str(brand_id or ""))

        cat_slug = f"smoke-cat-{uuid.uuid4().hex[:8]}"
        cat = _http_expect_ok(
            "POST /v1/admin/categories",
            "POST",
            urljoin(base + "/", "v1/admin/categories") + q_org,
            headers={**auth_headers, "Idempotency-Key": idem("cat")},
            json_body={"slug": cat_slug, "name": "Smoke Category", "active": True},
            timeout=timeout,
        )
        cat_id = cat.get("id") if isinstance(cat, dict) else None
        log_ok("POST /v1/admin/categories", str(cat_id or ""))

        tag_slug = f"smoke-tag-{uuid.uuid4().hex[:8]}"
        _http_expect_ok(
            "POST /v1/admin/tags",
            "POST",
            urljoin(base + "/", "v1/admin/tags") + q_org,
            headers={**auth_headers, "Idempotency-Key": idem("tag")},
            json_body={"slug": tag_slug, "name": "Smoke Tag", "active": True},
            timeout=timeout,
        )
        log_ok("POST /v1/admin/tags")

        sku = f"SMOKE-{uuid.uuid4().hex[:10].upper()}"
        prod_body = {
            "sku": sku,
            "name": "Smoke Product",
            "description": "created by smoke_test.py local",
            "active": True,
            "categoryId": cat_id,
            "brandId": brand_id,
            "ageRestricted": False,
        }
        pr = _http_expect_ok(
            "POST /v1/admin/products",
            "POST",
            urljoin(base + "/", "v1/admin/products") + q_org,
            headers={**auth_headers, "Idempotency-Key": idem("prod")},
            json_body=prod_body,
            timeout=timeout,
        )
        product_id_new = pr.get("id") if isinstance(pr, dict) else None
        log_ok("POST /v1/admin/products", product_id_new or "")

        plist = _http_expect_ok(
            "GET /v1/admin/products",
            "GET",
            urljoin(base + "/", "v1/admin/products") + q_org + "&limit=10&offset=0",
            headers=auth_headers,
            json_body=None,
            timeout=timeout,
        )
        log_ok("GET /v1/admin/products")

        # Activation codes
        act = _http_expect_ok(
            "POST activation code",
            "POST",
            urljoin(base + "/", f"v1/admin/machines/{machine_id}/activation-codes") + q_org,
            headers={**auth_headers, "Idempotency-Key": idem("act")},
            json_body={"expiresInMinutes": 60, "maxUses": 3, "notes": "smoke_test local"},
            timeout=timeout,
        )
        plain_code = (act.get("activationCode") if isinstance(act, dict) else None) or ""
        log_ok("POST /v1/admin/machines/{machineId}/activation-codes", "***")

        lst = _http_expect_ok(
            "GET activation codes list",
            "GET",
            urljoin(base + "/", f"v1/admin/machines/{machine_id}/activation-codes") + q_org,
            headers=auth_headers,
            json_body=None,
            timeout=timeout,
        )
        log_ok("GET /v1/admin/machines/{machineId}/activation-codes")

        if plain_code:
            claim_body = {
                "activationCode": plain_code,
                "deviceFingerprint": {
                    "androidId": "smoke-android",
                    "serialNumber": "SMOKE-SN",
                    "manufacturer": "AVF",
                    "model": "SmokeDevice",
                    "packageName": "com.avf.smoke",
                    "versionName": "1.0.0",
                    "versionCode": 1,
                },
            }
            try:
                _http_expect_ok(
                    "POST /v1/setup/activation-codes/claim",
                    "POST",
                    urljoin(base + "/", "v1/setup/activation-codes/claim"),
                    headers={"User-Agent": "avf-local-smoke"},
                    json_body=claim_body,
                    timeout=timeout,
                )
                log_ok("POST /v1/setup/activation-codes/claim")
            except Exception as e:
                log_skip("POST /v1/setup/activation-codes/claim", str(e))

        # Bootstrap + sale catalog + telemetry
        _http_expect_ok(
            "GET /v1/setup/machines/{machineId}/bootstrap",
            "GET",
            urljoin(base + "/", f"v1/setup/machines/{machine_id}/bootstrap"),
            headers=auth_headers,
            json_body=None,
            timeout=timeout,
        )
        log_ok("GET /v1/setup/machines/{machineId}/bootstrap")

        _http_expect_ok(
            "GET sale-catalog",
            "GET",
            urljoin(base + "/", f"v1/machines/{machine_id}/sale-catalog"),
            headers=auth_headers,
            json_body=None,
            timeout=timeout,
        )
        log_ok("GET /v1/machines/{machineId}/sale-catalog")

        _http_expect_ok(
            "GET telemetry snapshot",
            "GET",
            urljoin(base + "/", f"v1/machines/{machine_id}/telemetry/snapshot"),
            headers=auth_headers,
            json_body=None,
            timeout=timeout,
        )
        log_ok("GET /v1/machines/{machineId}/telemetry/snapshot")

        # Commerce: create order + cash checkout (seed product)
        sid = args.slot_index
        cash_body = {
            "machine_id": machine_id,
            "product_id": args.product_id,
            "slot_index": sid,
            "currency": "USD",
        }
        cash_out = _http_expect_ok(
            "POST /v1/commerce/cash-checkout",
            "POST",
            urljoin(base + "/", "v1/commerce/cash-checkout"),
            headers={**auth_headers, "Idempotency-Key": idem("cash")},
            json_body=cash_body,
            timeout=timeout,
        )
        order_id = str((cash_out or {}).get("order_id") or "")
        log_ok("POST /v1/commerce/cash-checkout", order_id)

        # Vend progression
        _http_expect_ok(
            "POST vend/start",
            "POST",
            urljoin(base + "/", f"v1/commerce/orders/{order_id}/vend/start"),
            headers={**auth_headers, "Idempotency-Key": idem("vstart")},
            json_body={"slot_index": sid},
            timeout=timeout,
        )
        log_ok("POST /v1/commerce/orders/{orderId}/vend/start")

        _http_expect_ok(
            "POST vend/success",
            "POST",
            urljoin(base + "/", f"v1/commerce/orders/{order_id}/vend/success"),
            headers={**auth_headers, "Idempotency-Key": idem("vsucc")},
            json_body={"slot_index": sid},
            timeout=timeout,
        )
        log_ok("POST /v1/commerce/orders/{orderId}/vend/success")

        # Payment webhook (simulated PSP / AVF HMAC)
        secret = (os.environ.get("COMMERCE_PAYMENT_WEBHOOK_SECRET") or os.environ.get("COMMERCE_PAYMENT_WEBHOOK_HMAC_SECRET") or "").strip()
        if args.skip_payment_webhook or not secret:
            log_skip(
                "POST /v1/commerce/.../webhooks (simulated PSP)",
                "SKIP_PAYMENT_WEBHOOK or missing COMMERCE_PAYMENT_WEBHOOK_SECRET",
            )
        else:
            try:
                idem_o = idem("ord-wh")
                cr = _http_expect_ok(
                    "create order for webhook",
                    "POST",
                    urljoin(base + "/", "v1/commerce/orders"),
                    headers={**auth_headers, "Idempotency-Key": idem_o},
                    json_body={
                        "machine_id": machine_id,
                        "product_id": args.product_id,
                        "slot_index": sid,
                        "currency": "USD",
                    },
                    timeout=timeout,
                )
                oid = str((cr or {}).get("order_id") or "")
                total_minor = int((cr or {}).get("total_minor") or 0)
                idem_ps = idem("pay-sess")
                ps = _http_expect_ok(
                    "payment-session",
                    "POST",
                    urljoin(base + "/", f"v1/commerce/orders/{oid}/payment-session"),
                    headers={**auth_headers, "Idempotency-Key": idem_ps},
                    json_body={
                        "provider": "psp_smoke",
                        "payment_state": "created",
                        "amount_minor": total_minor,
                        "currency": "USD",
                        "outbox_payload_json": {"source": "smoke_test"},
                    },
                    timeout=timeout,
                )
                pay_id = str((ps or {}).get("payment_id") or "")
                prov_ref = f"smoke-ref-{uuid.uuid4().hex}"
                wh_payload = {
                    "provider": "psp_smoke",
                    "provider_reference": prov_ref,
                    "webhook_event_id": f"evt-{uuid.uuid4().hex}",
                    "event_type": "payment.captured",
                    "normalized_payment_state": "captured",
                    "payload_json": {"test": True},
                }
                raw_body = json.dumps(wh_payload).encode("utf-8")
                ts_hdr, sig_hdr = sign_commerce_webhook(secret, raw_body)
                wh_url = urljoin(base + "/", f"v1/commerce/orders/{oid}/payments/{pay_id}/webhooks")
                ctx_wh = _ssl_ctx_for_url(wh_url)
                opener_wh = urllib.request.build_opener(urllib.request.HTTPSHandler(context=ctx_wh)) if ctx_wh else urllib.request.build_opener()

                def post_webhook_once() -> int:
                    req = urllib.request.Request(
                        wh_url,
                        data=raw_body,
                        headers={
                            "Content-Type": "application/json",
                            "X-AVF-Webhook-Timestamp": ts_hdr,
                            "X-AVF-Webhook-Signature": sig_hdr,
                            "User-Agent": "avf-local-smoke",
                        },
                        method="POST",
                    )
                    with opener_wh.open(req, timeout=timeout) as resp:
                        return int(resp.status)

                first_status = post_webhook_once()
                if first_status != 200:
                    raise RuntimeError(f"webhook status {first_status}")
                replay_status = post_webhook_once()
                if replay_status != 200:
                    raise RuntimeError(f"webhook replay status {replay_status}")
                log_ok("POST payment webhook (HMAC + idempotent replay)", oid)
            except Exception as e:
                log_fail("POST payment webhook (HMAC)", str(e))

        # Refunds
        try:
            ref_out = _http_expect_ok(
                "POST refund",
                "POST",
                urljoin(base + "/", f"v1/commerce/orders/{order_id}/refunds"),
                headers={**auth_headers, "Idempotency-Key": idem("ref")},
                json_body={
                    "reason": "smoke_test local",
                    "amount_minor": 50,
                    "currency": "USD",
                    "metadata": {"source": "smoke_test"},
                },
                timeout=timeout,
            )
            log_ok("POST /v1/commerce/orders/{orderId}/refunds", str((ref_out or {}).get("refund_id") or ""))

            rlist = _http_expect_ok(
                "GET refunds",
                "GET",
                urljoin(base + "/", f"v1/commerce/orders/{order_id}/refunds"),
                headers=auth_headers,
                json_body=None,
                timeout=timeout,
            )
            log_ok("GET /v1/commerce/orders/{orderId}/refunds")
        except Exception as e:
            log_skip("refund flow", str(e))

        # Inventory adjustment (requires operator session)
        try:
            sess_out = _http_expect_ok(
                "operator-sessions/login",
                "POST",
                urljoin(base + "/", f"v1/machines/{machine_id}/operator-sessions/login"),
                headers=auth_headers,
                json_body={},
                timeout=timeout,
            )
            sess_id = ""
            if isinstance(sess_out, dict) and isinstance(sess_out.get("session"), dict):
                sess_id = str(sess_out["session"].get("id") or "")
            if not sess_id:
                raise RuntimeError("missing session id")
            slots_out = _http_expect_ok(
                "admin slots",
                "GET",
                urljoin(base + "/", f"v1/admin/machines/{machine_id}/slots") + q_org,
                headers=auth_headers,
                json_body=None,
                timeout=timeout,
            )
            items = (slots_out or {}).get("items") if isinstance(slots_out, dict) else None
            if not items:
                raise RuntimeError("no slots returned")
            sl0 = items[0]
            pg_id = str(sl0.get("planogramId") or "")
            sidx = int(sl0.get("slotIndex") or 0)
            qb = int(sl0.get("currentQuantity") or sl0.get("currentStock") or 0)
            idem_inv = idem("inv")
            _http_expect_ok(
                "stock-adjustments",
                "POST",
                urljoin(base + "/", f"v1/admin/machines/{machine_id}/stock-adjustments"),
                headers={**auth_headers, "Idempotency-Key": idem_inv},
                json_body={
                    "operator_session_id": sess_id,
                    "reason": "cycle_count",
                    "items": [
                        {
                            "planogramId": pg_id,
                            "slotIndex": sidx,
                            "quantityBefore": qb,
                            "quantityAfter": qb,
                        }
                    ],
                },
                timeout=timeout,
            )
            log_ok("POST /v1/admin/machines/{machineId}/stock-adjustments (cycle_count no-op)")
        except Exception as e:
            log_skip("inventory stock-adjustments", str(e))

        # Reports (last 24h UTC)
        now = datetime.now(timezone.utc)
        frm = (now - timedelta(days=1)).strftime("%Y-%m-%dT%H:%M:%SZ")
        to = now.strftime("%Y-%m-%dT%H:%M:%SZ")
        rep_q = q_org + "&from=" + urllib.parse.quote(frm, safe="") + "&to=" + urllib.parse.quote(to, safe="")
        for path, label in (
            ("v1/reports/sales-summary", "sales-summary"),
            ("v1/reports/payments-summary", "payments-summary"),
            ("v1/reports/fleet-health", "fleet-health"),
            ("v1/reports/inventory-exceptions", "inventory-exceptions"),
        ):
            _http_expect_ok(label, "GET", urljoin(base + "/", path) + rep_q, headers=auth_headers, json_body=None, timeout=timeout)
            log_ok(f"GET /{path}")

    except Exception as e:
        log_fail("local smoke (fatal)", str(e))
        ok_all = False

    evidence["overall"] = "pass" if ok_all else "fail"
    out_ev = os.path.abspath(args.evidence_json.strip() or "smoke-evidence-local.json")
    try:
        os.makedirs(os.path.dirname(out_ev) or ".", exist_ok=True)
        with open(out_ev, "w", encoding="utf-8") as f:
            json.dump(evidence, f, indent=2)
            f.write("\n")
        print(f"\nsmoke_test: evidence written to {out_ev}", file=sys.stderr)
    except OSError as e:
        print(f"smoke_test: warning: could not write evidence JSON: {e}", file=sys.stderr)

    return 0 if ok_all else 1


def main_post_deploy() -> int:
    ap = argparse.ArgumentParser(description="Post-deploy read-only HTTP smoke checks.")
    ap.add_argument("--report", type=str, default="", help="JSON report path (default: smoke-reports/smoke-test.json under cwd)")
    args = ap.parse_args()

    base = (os.environ.get("BASE_URL") or "").strip().rstrip("/")
    if not base:
        print("smoke_test: error: BASE_URL is required", file=sys.stderr)
        return 2

    env_name = (os.environ.get("ENVIRONMENT_NAME") or "unknown").strip()
    token = (os.environ.get("SMOKE_AUTH_TOKEN") or "").strip()
    auth_path = (os.environ.get("SMOKE_AUTH_READ_PATH") or "").strip()

    timeout = _env_float("SMOKE_REQUEST_TIMEOUT_SEC", 15.0)
    max_attempts = max(1, _env_int("SMOKE_MAX_ATTEMPTS", 5))
    backoff_base = _env_float("SMOKE_BACKOFF_BASE_SEC", 2.0)
    warn_ms = _env_float("SMOKE_WARN_LATENCY_MS", 2000.0)

    parsed = urlparse(base)
    if parsed.scheme not in ("http", "https"):
        print("smoke_test: error: BASE_URL must be http or https", file=sys.stderr)
        return 2

    ssl_ctx: ssl.SSLContext | None = None
    if parsed.scheme == "https":
        ssl_ctx = ssl.create_default_context()

    report = Report(environment_name=env_name, base_url=base)
    ok = True

    if parsed.scheme == "https" and parsed.hostname:
        host = parsed.hostname
        try:
            t0 = time.monotonic()
            socket.getaddrinfo(host, 443, type=socket.SOCK_STREAM)
            elapsed = (time.monotonic() - t0) * 1000.0
            lat_warn = elapsed > warn_ms
            if lat_warn:
                report.warnings.append(
                    f"DNS resolve ({host}): {elapsed:.0f}ms exceeds warn threshold {warn_ms:.0f}ms"
                )
            detail = "ok"
            if lat_warn:
                detail = f"ok (latency warning: {elapsed:.0f}ms)"
            report.checks.append(
                CheckResult(
                    name="DNS resolve (HTTPS host)",
                    url=host,
                    status="warn" if lat_warn else "pass",
                    http_status=None,
                    elapsed_ms=elapsed,
                    detail=detail,
                    latency_warning=lat_warn,
                    category="required",
                )
            )
        except OSError as e:
            report.checks.append(
                CheckResult(
                    name="DNS resolve (HTTPS host)",
                    url=host,
                    status="fail",
                    http_status=None,
                    elapsed_ms=None,
                    detail=str(e),
                    category="required",
                )
            )
            ok = False

    extra: dict[str, str] = {}
    if token:
        extra["Authorization"] = f"Bearer {token}"

    ok = run_check(
        report,
        "GET /health/live",
        "/health/live",
        base,
        timeout=timeout,
        extra_headers={},
        ssl_ctx=ssl_ctx,
        max_attempts=max_attempts,
        backoff_base=backoff_base,
        warn_ms=warn_ms,
        body_assert="ok",
    ) and ok

    ok = run_check(
        report,
        "GET /health/ready",
        "/health/ready",
        base,
        timeout=timeout,
        extra_headers={},
        ssl_ctx=ssl_ctx,
        max_attempts=max_attempts,
        backoff_base=backoff_base,
        warn_ms=warn_ms,
        body_assert="ok",
    ) and ok

    ok = run_check(
        report,
        "GET /version",
        "/version",
        base,
        timeout=timeout,
        extra_headers={},
        ssl_ctx=ssl_ctx,
        max_attempts=max_attempts,
        backoff_base=backoff_base,
        warn_ms=warn_ms,
        body_assert='"version"',
    ) and ok

    # Optional, env-gated checks (skip with reason if unset — never count skip as pass).
    ok = _optional_path(
        report,
        "DB connectivity probe (optional GET)",
        "SMOKE_CHECK_DB_PATH",
        base,
        body_env_key="SMOKE_CHECK_DB_BODY_SUBSTRING",
        timeout=timeout,
        extra_headers={},
        ssl_ctx=ssl_ctx,
        max_attempts=max_attempts,
        backoff_base=backoff_base,
        warn_ms=warn_ms,
    ) and ok
    ok = _optional_path(
        report,
        "Redis connectivity probe (optional GET)",
        "SMOKE_CHECK_REDIS_PATH",
        base,
        body_env_key="SMOKE_CHECK_REDIS_BODY_SUBSTRING",
        timeout=timeout,
        extra_headers={},
        ssl_ctx=ssl_ctx,
        max_attempts=max_attempts,
        backoff_base=backoff_base,
        warn_ms=warn_ms,
    ) and ok
    ok = _optional_path(
        report,
        "Machine heartbeat / mock probe (optional GET)",
        "SMOKE_CHECK_HEARTBEAT_PATH",
        base,
        body_env_key="SMOKE_CHECK_HEARTBEAT_BODY_SUBSTRING",
        timeout=timeout,
        extra_headers={},
        ssl_ctx=ssl_ctx,
        max_attempts=max_attempts,
        backoff_base=backoff_base,
        warn_ms=warn_ms,
    ) and ok
    ok = _optional_path(
        report,
        "Payment callback mock probe (optional GET, sandbox/local only)",
        "SMOKE_CHECK_PAYMENT_MOCK_PATH",
        base,
        body_env_key="SMOKE_CHECK_PAYMENT_MOCK_BODY_SUBSTRING",
        timeout=timeout,
        extra_headers={},
        ssl_ctx=ssl_ctx,
        max_attempts=max_attempts,
        backoff_base=backoff_base,
        warn_ms=warn_ms,
    ) and ok

    mqtt_url = (os.environ.get("SMOKE_CHECK_MQTT_URL") or "").strip()
    if not mqtt_url:
        report.checks.append(
            CheckResult(
                name="MQTT/EMQX HTTP probe (optional)",
                url="",
                status="skip",
                http_status=None,
                elapsed_ms=None,
                detail="SMOKE_CHECK_MQTT_URL not set (e.g. internal EMQX /api/v5/status URL if exposed to runner) — skip, not pass",
                category="optional",
            )
        )
    else:
        mbody = (os.environ.get("SMOKE_CHECK_MQTT_BODY_SUBSTRING") or "").strip() or None
        ok = run_check_at_url(
            report,
            "MQTT/EMQX HTTP probe (optional)",
            mqtt_url,
            timeout=timeout,
            extra_headers={},
            max_attempts=max_attempts,
            backoff_base=backoff_base,
            body_assert=mbody,
        ) and ok

    if token and auth_path:
        path = auth_path if auth_path.startswith("/") else f"/{auth_path}"
        ok = (
            run_check(
                report,
                f"GET {path} (authenticated read, optional admin smoke)",
                path,
                base,
                timeout=timeout,
                extra_headers=extra,
                ssl_ctx=ssl_ctx,
                max_attempts=max_attempts,
                backoff_base=backoff_base,
                warn_ms=warn_ms,
                body_assert=None,
                category="optional",
            )
            and ok
        )
    else:
        adetail = "SMOKE_AUTH_TOKEN and SMOKE_AUTH_READ_PATH not both set — admin API optional smoke skip (not pass)"
        if token and not auth_path:
            adetail = "SMOKE_AUTH_READ_PATH missing while SMOKE_AUTH_TOKEN is set"
        if auth_path and not token:
            adetail = "SMOKE_AUTH_TOKEN missing while SMOKE_AUTH_READ_PATH is set"
        report.checks.append(
            CheckResult(
                name="Admin API read-only smoke (optional)",
                url="",
                status="skip",
                http_status=None,
                elapsed_ms=None,
                detail=adetail,
                category="optional",
            )
        )

    if any(c.status == "fail" for c in report.checks):
        report.overall = "fail"
    elif any(c.status == "warn" for c in report.checks) or report.warnings:
        report.overall = "pass"
    else:
        report.overall = "pass"

    root = os.getcwd()
    out = args.report.strip() or os.path.join(root, "smoke-reports", "smoke-test.json")
    out_path = os.path.abspath(out)
    os.makedirs(os.path.dirname(out_path), exist_ok=True)
    with open(out_path, "w", encoding="utf-8") as f:
        json.dump(report.to_dict(), f, indent=2)
        f.write("\n")

    print(f"smoke_test: environment={env_name} base_url={base}", file=sys.stderr)
    print(f"smoke_test: report written to {out_path}", file=sys.stderr)
    for c in report.checks:
        print(f"smoke_test: [{c.status.upper()}] {c.name} http={c.http_status} ms={c.elapsed_ms} {c.detail}", file=sys.stderr)
    for w in report.warnings:
        print(f"smoke_test: WARNING {w}", file=sys.stderr)

    if not ok:
        print(
            "smoke_test: FAILED. Fix the service or retry after rollout stabilizes. "
            "Production: automatic rollback may run if configured; otherwise use workflow_dispatch rollback with prior digest-pinned refs.",
            file=sys.stderr,
        )
        return 1
    return 0


def main() -> int:
    if len(sys.argv) >= 2 and sys.argv[1] == "local":
        return main_local_e2e(sys.argv[2:])
    return main_post_deploy()


if __name__ == "__main__":
    raise SystemExit(main())

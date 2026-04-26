#!/usr/bin/env python3
"""
Read-only HTTP post-deploy smoke (no real payment provider calls; no mutating requests).

**Required** (failing any fails the process): DNS (HTTPS), GET /health/live, /health/ready, /version.

**Optional** (env-gated; `skip` in JSON with reason if unset — skip is not pass):
  SMOKE_CHECK_DB_PATH, SMOKE_CHECK_REDIS_PATH, SMOKE_CHECK_MQTT_URL (absolute URL),
  SMOKE_CHECK_HEARTBEAT_PATH, SMOKE_CHECK_PAYMENT_MOCK_PATH,
  SMOKE_AUTH_TOKEN + SMOKE_AUTH_READ_PATH (read-only GET).
Optional body substring: SMOKE_CHECK_*_BODY_SUBSTRING, SMOKE_CHECK_MQTT_BODY_SUBSTRING.

Writes smoke-reports/smoke-test.json (or --report). Never prints bearer tokens.
"""
from __future__ import annotations

import argparse
import json
import os
import random
import socket
import ssl
import sys
import time
import urllib.error
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


def main() -> int:
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


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""Release-time OpenAPI checks (used by scripts/verify_enterprise_release.sh).

Validates docs/swagger/swagger.json:
  - servers[0] is production URL
  - no planned-only path fragments appear as paths
  - POST/PUT/PATCH with application/json body include request body examples
  - protected /v1 routes declare bearerAuth (exceptions: login, refresh, claim, PSP webhook)
  - each operation has at least one 2xx response with an example (JSON, text/plain, or text/html)
  - when 4xx/5xx responses exist, at least one error declares application/json with an example
    (exemptions: GET /health/ready, GET /metrics)
  - example payloads must not contain obvious secret material (live keys, PEM private keys, JWT blobs)
"""
from __future__ import annotations

import json
import re
import sys
from pathlib import Path
from typing import Any, Iterator

ROOT = Path(__file__).resolve().parents[1]
SPEC_PATH = ROOT / "docs" / "swagger" / "swagger.json"

FORBIDDEN_PATH_FRAGMENTS = (
    "/v1/activation",
    "/v1/machines/{machineId}/activation",
    "/v1/runtime/catalog",
    "/v1/telemetry/reconcile",
    "/v1/cash-collection",
)

FORBIDDEN_PATH_SUFFIXES = ("/{orderId}/refund",)

NO_BEARER = {
    "/v1/auth/login": {"post"},
    "/v1/auth/refresh": {"post"},
    "/v1/setup/activation-codes/claim": {"post"},
    "/v1/commerce/orders/{orderId}/payments/{paymentId}/webhooks": {"post"},
}

JSON_ERR_EXEMPT = {
    ("/health/ready", "get"),
    ("/metrics", "get"),
}

# Pilot P0 operations that must stay in sync with internal/httpserver/server.go mounts.
REQUIRED_P0_OPERATIONS: tuple[tuple[str, str], ...] = (
    ("post", "/v1/admin/machines/{machineId}/activation-codes"),
    ("get", "/v1/admin/machines/{machineId}/activation-codes"),
    ("delete", "/v1/admin/machines/{machineId}/activation-codes/{activationCodeId}"),
    ("post", "/v1/setup/activation-codes/claim"),
    ("get", "/v1/machines/{machineId}/sale-catalog"),
    ("post", "/v1/device/machines/{machineId}/events/reconcile"),
    ("get", "/v1/device/machines/{machineId}/events/{idempotencyKey}/status"),
    ("post", "/v1/commerce/orders/{orderId}/cancel"),
    ("post", "/v1/commerce/orders/{orderId}/refunds"),
    ("get", "/v1/commerce/orders/{orderId}/refunds"),
    ("get", "/v1/commerce/orders/{orderId}/refunds/{refundId}"),
    ("get", "/v1/admin/machines/{machineId}/cashbox"),
    ("post", "/v1/admin/machines/{machineId}/cash-collections"),
    ("post", "/v1/admin/machines/{machineId}/cash-collections/{collectionId}/close"),
    ("get", "/v1/admin/machines/{machineId}/cash-collections"),
    ("get", "/v1/admin/machines/{machineId}/cash-collections/{collectionId}"),
    ("post", "/v1/admin/products"),
    ("put", "/v1/admin/products/{productId}"),
    ("patch", "/v1/admin/products/{productId}"),
    ("delete", "/v1/admin/products/{productId}"),
    ("put", "/v1/admin/products/{productId}/image"),
    ("delete", "/v1/admin/products/{productId}/image"),
    ("post", "/v1/admin/brands"),
    ("put", "/v1/admin/brands/{brandId}"),
    ("patch", "/v1/admin/brands/{brandId}"),
    ("delete", "/v1/admin/brands/{brandId}"),
    ("post", "/v1/admin/categories"),
    ("put", "/v1/admin/categories/{categoryId}"),
    ("patch", "/v1/admin/categories/{categoryId}"),
    ("delete", "/v1/admin/categories/{categoryId}"),
    ("post", "/v1/admin/tags"),
    ("put", "/v1/admin/tags/{tagId}"),
    ("patch", "/v1/admin/tags/{tagId}"),
    ("delete", "/v1/admin/tags/{tagId}"),
)

# Heuristics aligned with scripts/verify_enterprise_release.sh (docs/testdata scan).
_SECRET_PATTERNS: list[tuple[str, str]] = [
    (r"sk_live_[a-zA-Z0-9]+", "live Stripe-style secret key"),
    (r"pk_live_[a-zA-Z0-9]+", "live Stripe-style publishable key"),
    (r"AKIA[0-9A-Z]{16}", "AWS access key id"),
    (r"ASIA[0-9A-Z]{16}", "AWS temp access key id"),
    (r"ghp_[A-Za-z0-9]{20,}", "GitHub PAT"),
    (r"github_pat_[A-Za-z0-9_]+", "GitHub fine-grained PAT"),
    (r"xox[baprs]-[A-Za-z0-9-]+", "Slack token"),
    (r"-----BEGIN [A-Z ]*PRIVATE KEY-----", "PEM private key block"),
]

_PLACEHOLDER_OK = re.compile(
    r"(CHANGE_ME|PLACEHOLDER|REDACTED|YOUR_|INSERT_|REPLACE_ME|example\.com|myorg|"
    r"ldtv\.dev|127\.0\.0\.1|localhost|stub-|stub_|example-password|documentation)",
    re.I,
)


def _iter_example_strings(obj: Any) -> Iterator[str]:
    if isinstance(obj, dict):
        if "example" in obj:
            ex = obj["example"]
            if isinstance(ex, str):
                yield ex
            elif isinstance(ex, (dict, list)):
                yield json.dumps(ex)
        if "examples" in obj and isinstance(obj["examples"], dict):
            for item in obj["examples"].values():
                if isinstance(item, dict) and "value" in item:
                    v = item["value"]
                    if isinstance(v, str):
                        yield v
                    elif isinstance(v, (dict, list)):
                        yield json.dumps(v)
                elif isinstance(item, str):
                    yield item
        for v in obj.values():
            yield from _iter_example_strings(v)
    elif isinstance(obj, list):
        for v in obj:
            yield from _iter_example_strings(v)


def _line_ok_for_secrets(line: str) -> bool:
    s = line.strip()
    if not s or s.startswith("#"):
        return True
    if _PLACEHOLDER_OK.search(s):
        return True
    if "<jwt>" in s or "<opaque>" in s:
        return True
    return False


def _secret_hits_in_string(s: str) -> list[str]:
    hits: list[str] = []
    if _line_ok_for_secrets(s):
        return hits
    for pat, label in _SECRET_PATTERNS:
        if re.search(pat, s):
            hits.append(label)
    if re.fullmatch(r"eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+", s.strip()):
        hits.append("JWT-shaped token")
    return hits


def _security_has_bearer(sec: Any) -> bool:
    if not isinstance(sec, list):
        return False
    for item in sec:
        if isinstance(item, dict) and "bearerAuth" in item:
            return True
    return False


def main() -> int:
    if not SPEC_PATH.is_file():
        print(f"ERROR: missing {SPEC_PATH}", file=sys.stderr)
        return 1
    data: dict[str, Any] = json.loads(SPEC_PATH.read_text(encoding="utf-8"))
    servers = data.get("servers") or []
    if len(servers) < 1:
        print("ERROR: OpenAPI servers[] empty", file=sys.stderr)
        return 1
    s0 = servers[0]
    if not isinstance(s0, dict) or s0.get("url") != "https://api.ldtv.dev":
        print(f"ERROR: servers[0].url must be https://api.ldtv.dev, got {servers[0]!r}", file=sys.stderr)
        return 1
    print("OK: OpenAPI servers[0] is production (https://api.ldtv.dev)")
    if len(servers) < 2:
        print("ERROR: OpenAPI servers[] must include development as second entry", file=sys.stderr)
        return 1
    s1 = servers[1]
    if not isinstance(s1, dict) or s1.get("url") != "http://localhost:8080":
        print(f"ERROR: servers[1].url must be http://localhost:8080, got {servers[1]!r}", file=sys.stderr)
        return 1
    print("OK: OpenAPI servers[1] is development (http://localhost:8080)")

    paths = data.get("paths") or {}
    if not isinstance(paths, dict):
        print("ERROR: paths must be an object", file=sys.stderr)
        return 1
    bad: list[str] = []
    for p in paths:
        for suf in FORBIDDEN_PATH_SUFFIXES:
            if p.endswith(suf):
                bad.append(p)
                break
        else:
            for frag in FORBIDDEN_PATH_FRAGMENTS:
                if frag in p:
                    bad.append(p)
                    break
    if bad:
        print("ERROR: planned-only paths must not appear in OpenAPI:", file=sys.stderr)
        for p in sorted(set(bad)):
            print(f"  {p}", file=sys.stderr)
        return 1
    print("OK: no planned-only endpoint paths in OpenAPI")

    missing_p0: list[str] = []
    for method, path in REQUIRED_P0_OPERATIONS:
        entry = paths.get(path)
        if not isinstance(entry, dict) or method not in entry:
            missing_p0.append(f"{method.upper()} {path}")
    if missing_p0:
        print("ERROR: OpenAPI missing required P0 operations:", file=sys.stderr)
        for line in missing_p0:
            print(f"  {line}", file=sys.stderr)
        return 1
    print(f"OK: required P0 operations present ({len(REQUIRED_P0_OPERATIONS)} checks)")

    missing_ex: list[str] = []
    bearer_missing: list[str] = []
    resp_issues: list[str] = []
    secret_examples: list[str] = []

    for path, methods in paths.items():
        if not isinstance(methods, dict):
            continue
        for method, op in methods.items():
            m = str(method).lower()
            if m.startswith("x-") or m == "parameters":
                continue
            if not isinstance(op, dict):
                continue

            op_id = f"{m.upper()} {path}"

            for s in _iter_example_strings(op):
                for hit in _secret_hits_in_string(s):
                    secret_examples.append(f"{op_id}: {hit}")

            if m in ("post", "put", "patch"):
                rb = op.get("requestBody")
                if isinstance(rb, dict):
                    content = rb.get("content")
                    if isinstance(content, dict):
                        aj = content.get("application/json")
                        if isinstance(aj, dict):
                            if "example" not in aj and "examples" not in aj:
                                missing_ex.append(op_id)

            if path.startswith("/v1/"):
                skip = NO_BEARER.get(path, set())
                if m not in skip:
                    if not _security_has_bearer(op.get("security")):
                        bearer_missing.append(op_id)

            responses = op.get("responses")
            if not isinstance(responses, dict):
                resp_issues.append(f"{op_id}: missing responses object")
                continue
            has2xx = False
            has2xx_example = False
            has4xx = False
            has4xx_json_example = False
            for code, resp_any in responses.items():
                code_s = str(code)
                try:
                    code_n = int(code_s)
                except ValueError:
                    code_n = -1
                resp = resp_any if isinstance(resp_any, dict) else {}
                content = resp.get("content") if isinstance(resp.get("content"), dict) else {}
                if 200 <= code_n < 300:
                    has2xx = True
                    aj = content.get("application/json") if isinstance(content, dict) else None
                    if isinstance(aj, dict) and ("example" in aj or "examples" in aj):
                        has2xx_example = True
                    if content.get("text/plain") or content.get("text/html"):
                        has2xx_example = True
                if code_n >= 400:
                    has4xx = True
                    aj = content.get("application/json") if isinstance(content, dict) else None
                    if isinstance(aj, dict) and ("example" in aj or "examples" in aj):
                        has4xx_json_example = True
            if not has2xx:
                resp_issues.append(f"{op_id}: expected at least one 2xx response")
            elif not has2xx_example:
                resp_issues.append(f"{op_id}: expected a 2xx example (JSON, text/plain, or text/html)")
            if has4xx and not has4xx_json_example:
                if (path, m) not in JSON_ERR_EXEMPT:
                    resp_issues.append(f"{op_id}: declare error responses with application/json examples")

    if missing_ex:
        print("ERROR: JSON write operations must include request body example(s) for Swagger UI:", file=sys.stderr)
        for line in missing_ex:
            print(f"  {line}", file=sys.stderr)
        return 1
    print("OK: all POST/PUT/PATCH application/json request bodies have example(s)")

    if bearer_missing:
        print("ERROR: protected /v1 operations must declare bearerAuth in security:", file=sys.stderr)
        for line in bearer_missing:
            print(f"  {line}", file=sys.stderr)
        return 1
    print("OK: protected /v1 routes declare Bearer security (except login/refresh/claim/webhook)")

    if resp_issues:
        print("ERROR: OpenAPI response/example requirements failed:", file=sys.stderr)
        for line in resp_issues:
            print(f"  {line}", file=sys.stderr)
        return 1
    print("OK: operations have success + error response examples where applicable")

    if secret_examples:
        print("ERROR: examples must not contain live-secret patterns:", file=sys.stderr)
        for line in sorted(set(secret_examples)):
            print(f"  {line}", file=sys.stderr)
        return 1
    print("OK: no obvious secret material in operation examples")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())

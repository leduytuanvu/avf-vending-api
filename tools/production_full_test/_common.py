#!/usr/bin/env python3
"""Shared utilities for production full REST/gRPC/MQTT verification."""

from __future__ import annotations

import json
import os
import re
import ssl
import sys
import uuid
from datetime import datetime, timezone
from pathlib import Path
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

REPO = Path(__file__).resolve().parents[2]
SWAGGER = REPO / "docs" / "swagger" / "swagger.json"
METHODS = frozenset({"get", "post", "put", "patch", "delete"})

SECRET_RE = re.compile(
    r"(?i)(Bearer\s+)[A-Za-z0-9._~+/=-]+|"
    r'("?(password|token|secret|authorization)"?\s*[:=]\s*)"?[^",\s}]+"?'
)


def utc_stamp() -> str:
    return os.environ.get("PRODUCTION_FULL_TEST_UTC", "").strip() or datetime.now(timezone.utc).strftime(
        "%Y%m%dT%H%M%SZ"
    )


def report_dir() -> Path:
    d = REPO / "reports" / "production-full-api-grpc-mqtt" / utc_stamp()
    d.mkdir(parents=True, exist_ok=True)
    return d


def runtime_fleet_prefix() -> str:
    suffix = os.environ.get("PROD_TEST_SUFFIX", uuid.uuid4().hex[:8])
    return f"AVF-RUNTIME-FLEET-{utc_stamp()}_{suffix}"


def test_prefix() -> str:
    explicit = os.environ.get("PRODUCTION_TEST_PREFIX", "").strip()
    if explicit:
        return explicit
    if os.environ.get("PRODUCTION_SUITE", "").strip().lower() in ("runtime_fleet", "runtime-fleet"):
        return runtime_fleet_prefix()
    suffix = os.environ.get("PROD_TEST_SUFFIX", uuid.uuid4().hex[:8])
    return f"ENTERPRISE_PROD_TEST_{utc_stamp()}_{suffix}"


def redact(text: str) -> str:
    def _repl(m: re.Match[str]) -> str:
        if m.group(1):
            return m.group(1) + "***REDACTED***"
        if m.group(2):
            return m.group(2) + "***REDACTED***"
        return "***REDACTED***"

    return SECRET_RE.sub(_repl, str(text))[:8000]


def load_json(path: Path) -> Any:
    return json.loads(path.read_text(encoding="utf-8"))


def write_json(path: Path, data: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(data, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")


def new_request_id() -> str:
    return str(uuid.uuid4())


def http_request(
    method: str,
    url: str,
    *,
    headers: dict[str, str] | None = None,
    body: bytes | None = None,
    timeout: float = 30.0,
) -> tuple[int, str, dict[str, str]]:
    hdrs = {"Accept": "application/json", "X-Request-ID": new_request_id()}
    if headers:
        hdrs.update(headers)
    req = Request(url, data=body, method=method.upper(), headers=hdrs)
    try:
        with urlopen(req, timeout=timeout) as resp:
            raw = resp.read(65536).decode("utf-8", errors="replace")
            return int(resp.status), raw, dict(resp.headers)
    except HTTPError as exc:
        raw = exc.read(65536).decode("utf-8", errors="replace")
        return int(exc.code), raw, dict(exc.headers)
    except URLError as exc:
        return 0, f"URLError: {exc.reason}", {}


def substitute_path(path: str, registry: dict[str, str]) -> str:
    import re as _re

    out = path
    for key, val in registry.items():
        if not val:
            continue
        out = out.replace("{" + key + "}", val)
        out = out.replace("{{" + key + "}}", val)
    # common OpenAPI param names
    aliases = {
        "machineId": registry.get("machineId") or registry.get("machine_id"),
        "siteId": registry.get("siteId") or registry.get("site_id"),
        "productId": registry.get("productId") or registry.get("product_id"),
        "brandId": registry.get("brandId") or registry.get("brand_id"),
        "categoryId": registry.get("categoryId") or registry.get("category_id"),
        "orderId": registry.get("orderId") or registry.get("order_id"),
        "accountId": registry.get("accountId") or registry.get("adminAccountId"),
        "codeId": registry.get("codeId") or registry.get("activationCodeId"),
        "draftId": registry.get("draftId") or registry.get("planogramDraftId"),
        "versionId": registry.get("versionId") or registry.get("planogramVersionId"),
        "id": registry.get("id"),
    }
    for k, v in aliases.items():
        if v:
            out = out.replace("{" + k + "}", v)
    # Remaining params: substitute with nil UUID so route is exercised (404/400 acceptable)
    nil = "00000000-0000-0000-0000-000000000001"
    for param in _re.findall(r"\{(\w+)\}", out):
        out = out.replace("{" + param + "}", registry.get(param) or nil)
    return out


def path_has_unresolved_params(path: str) -> bool:
    return "{" in path or "}" in path


def auth_mode_for_op(root: dict, path_item: dict, op: dict) -> str:
    security = op.get("security")
    if security is None:
        security = path_item.get("security")
    if security is None:
        security = root.get("security")
    if security == []:
        return "none"
    if not security:
        return "admin"
    names: list[str] = []
    for entry in security:
        if isinstance(entry, dict):
            names.extend(entry.keys())
    joined = " ".join(names).lower()
    if "machine" in joined or "machineruntime" in joined:
        return "machine"
    return "admin"


def iter_openapi_ops(swagger_path: Path = SWAGGER) -> list[dict[str, Any]]:
    doc = load_json(swagger_path)
    rows: list[dict[str, Any]] = []
    for path, path_item in sorted((doc.get("paths") or {}).items()):
        if not isinstance(path_item, dict):
            continue
        for method, op in path_item.items():
            if method.lower() not in METHODS or not isinstance(op, dict):
                continue
            rows.append(
                {
                    "method": method.upper(),
                    "path": path,
                    "operationId": op.get("operationId", ""),
                    "tags": op.get("tags") or [],
                    "auth": auth_mode_for_op(doc, path_item, op),
                }
            )
    return rows


def ssl_context() -> ssl.SSLContext:
    return ssl.create_default_context()


def append_jsonl(path: Path, row: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("a", encoding="utf-8") as f:
        f.write(json.dumps(row, ensure_ascii=False) + "\n")

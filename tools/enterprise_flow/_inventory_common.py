#!/usr/bin/env python3
"""Shared helpers for enterprise-flow inventory and coverage tools."""

from __future__ import annotations

import json
import os
import re
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

REPO = Path(__file__).resolve().parents[2]
SWAGGER = REPO / "docs" / "swagger" / "swagger.json"
HTTPSERVER = REPO / "internal" / "httpserver"
METHODS = frozenset({"get", "post", "put", "patch", "delete", "options", "head", "trace"})

ADMIN_MOUNT_PREFIX = "/v1/admin"

MOUNT_PREFIX_BY_FUNC: dict[str, str] = {
    "mountPublicActivationClaim": "/v1",
    "mountAuthRoutes": "/v1/auth",
    "mountAuthBearerSessionRoutes": "/v1/auth",
    "mountAuthBearerRoutes": "/v1/auth",
    "mountCommercePublicWebhookPost": "/v1",
    "mountMachineRuntimeRoutes": "/v1",
    "mountDeviceCommandRoutes": "/v1",
    "mountDeviceBridgeRoutes": "/v1",
    "mountOperatorSessionRoutes": "/v1",
    "mountSetupBootstrapRoutes": "/v1/setup",
    "mountReportingRoutes": "/v1/reports",
    "mountOperatorAdminInsightRoutes": "/v1/operator-insights",
    "mountSaleCatalogRoute": "/v1",
    "mountMachineTelemetryRoutes": "/v1",
}

ROUTE_REG = re.compile(
    r'(?<![.\w])r\.(?:With\([^)]*\)\.)?(Get|Post|Put|Patch|Delete|Head|Options)\("([^"]+)"',
    re.I,
)
ROUTE_MOUNT_REG = re.compile(r'r\.Route\("([^"]+)",\s*func\(r chi\.Router\)\s*\{', re.I)
MOUNT_FUNC_REG = re.compile(r"^func (mount\w+)\(", re.M)
ROUTER_ANNOT_REG = re.compile(r"@Router\s+(/[^\s\[]+)\s+\[(\w+)\]", re.I)


def utc_stamp() -> str:
    env = os.environ.get("ENTERPRISE_FLOW_VERIFICATION_UTC", "").strip()
    if env:
        return env
    return datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")


def verification_dir() -> Path:
    d = REPO / "reports" / "enterprise-flow-verification" / utc_stamp()
    d.mkdir(parents=True, exist_ok=True)
    return d


def latest_verification_dir() -> Path:
    root = REPO / "reports" / "enterprise-flow-verification"
    if not root.is_dir():
        return verification_dir()
    dirs = sorted([p for p in root.iterdir() if p.is_dir()], reverse=True)
    return dirs[0] if dirs else verification_dir()


def _join_path(prefix: str, rel: str) -> str:
    if rel.startswith("/v1"):
        return rel
    if not prefix:
        return rel if rel.startswith("/") else "/" + rel
    if rel == "/":
        return prefix.rstrip("/") or "/"
    return (prefix.rstrip("/") + "/" + rel.lstrip("/")).replace("//", "/")


def _extract_routes_from_block(block: str, prefix: str) -> set[tuple[str, str]]:
    ops: set[tuple[str, str]] = set()
    pos = 0
    while pos < len(block):
        mount = ROUTE_MOUNT_REG.search(block, pos)
        method = ROUTE_REG.search(block, pos)
        if mount and (not method or mount.start() < method.start()):
            sub_prefix = _join_path(prefix, mount.group(1))
            brace = block.find("{", mount.end() - 1)
            if brace == -1:
                pos = mount.end()
                continue
            depth = 0
            end = brace
            for i in range(brace, len(block)):
                if block[i] == "{":
                    depth += 1
                elif block[i] == "}":
                    depth -= 1
                    if depth == 0:
                        end = i
                        break
            sub = block[brace + 1 : end]
            ops |= _extract_routes_from_block(sub, sub_prefix)
            pos = end + 1
            continue
        if method:
            ops.add((method.group(1).upper(), _join_path(prefix, method.group(2))))
            pos = method.end()
            continue
        break
    return ops


def _is_reportable_chi_path(path: str) -> bool:
    if not path.startswith("/v1/"):
        return False
    if path in {"/v1/", "/", "/v1/admin"}:
        return False
    if path.endswith("{") or path.endswith("/{"):
        return False
    parts = [p for p in path.split("/") if p]
    return len(parts) >= 2


def load_chi_mounted_ops() -> set[tuple[str, str]]:
    ops: set[tuple[str, str]] = set()
    for go in sorted(HTTPSERVER.glob("*.go")):
        if go.name in {"swagger_operations.go", "swagger.go"}:
            continue
        text = go.read_text(encoding="utf-8", errors="replace")
        for mfunc in MOUNT_FUNC_REG.finditer(text):
            fname = mfunc.group(1)
            if fname.startswith("mountAdmin") or fname.startswith("MountAdmin"):
                prefix = ADMIN_MOUNT_PREFIX
            else:
                prefix = MOUNT_PREFIX_BY_FUNC.get(fname, "")
            start = mfunc.end()
            nxt = MOUNT_FUNC_REG.search(text, start)
            body = text[start : nxt.start() if nxt else len(text)]
            for m, p in _extract_routes_from_block(body, prefix):
                if _is_reportable_chi_path(p):
                    ops.add((m, p))
    server = (HTTPSERVER / "server.go").read_text(encoding="utf-8", errors="replace")
    for rm in ROUTE_REG.finditer(server):
        rel = rm.group(2)
        if rel.startswith(("/machines", "/technicians", "/assignments", "/commands", "/ota")):
            full = _join_path(ADMIN_MOUNT_PREFIX, rel)
            if _is_reportable_chi_path(full):
                ops.add((rm.group(1).upper(), full))
    return ops


def load_openapi_ops() -> set[tuple[str, str]]:
    data = json.loads(SWAGGER.read_text(encoding="utf-8"))
    ops: set[tuple[str, str]] = set()
    for path, item in data.get("paths", {}).items():
        for method in item:
            if method.lower() in METHODS:
                ops.add((method.upper(), path))
    return ops


def load_router_ops() -> set[tuple[str, str]]:
    sw = (HTTPSERVER / "swagger_operations.go").read_text(encoding="utf-8", errors="replace")
    routes: set[tuple[str, str]] = set()
    for m in ROUTER_ANNOT_REG.finditer(sw):
        routes.add((m.group(2).upper(), m.group(1)))
    return routes


def normalize_path(path: str) -> str:
    return re.sub(r"\{(\w+)\}", r"{\1}", path)


def write_inventory(name: str, md_lines: list[str], payload: dict[str, Any]) -> Path:
    out = latest_verification_dir()
    json_path = out / f"{name}.json"
    md_path = out / f"{name}.md"
    json_path.write_text(json.dumps(payload, indent=2), encoding="utf-8")
    md_path.write_text("\n".join(md_lines) + "\n", encoding="utf-8")
    return out


def classify_auth_from_openapi(op: dict[str, Any]) -> str:
    if op.get("security"):
        return "bearer_user_or_machine"
    return "public"


def execution_class_for_op(method: str, path: str) -> str:
    destructive = (
        "/archive",
        "/suspend",
        "/mark-compromised",
        "/revoke",
        "/import",
        "/resolve",
        "/refund",
        "/delete",
    )
    if any(x in path for x in destructive) and method in {"POST", "PUT", "PATCH", "DELETE"}:
        return "SAFETY_GATED"
    if path.startswith("/v1/admin"):
        return "AUTH_REQUIRED"
    if "/webhooks" in path:
        return "WEBHOOK"
    return "EXECUTABLE"

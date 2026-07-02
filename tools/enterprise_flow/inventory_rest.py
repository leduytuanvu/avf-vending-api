#!/usr/bin/env python3
"""REST API inventory: OpenAPI ops + Chi mounts + auth/safety classification."""

from __future__ import annotations

import json
import sys
from pathlib import Path

from _inventory_common import (
    HTTPSERVER,
    SWAGGER,
    classify_auth_from_openapi,
    execution_class_for_op,
    load_chi_mounted_ops,
    load_openapi_ops,
    load_router_ops,
    normalize_path,
    write_inventory,
)

REPO = Path(__file__).resolve().parents[2]
EXCEPTIONS = Path(__file__).resolve().parent / "accepted_surface_exceptions.json"


def main() -> int:
    openapi_data = json.loads(SWAGGER.read_text(encoding="utf-8"))
    openapi_ops = load_openapi_ops()
    router_ops = load_router_ops()
    chi_ops = load_chi_mounted_ops()

    items: list[dict] = []
    for path, path_item in sorted(openapi_data.get("paths", {}).items()):
        for method, op in path_item.items():
            if method.lower() not in {"get", "post", "put", "patch", "delete", "head", "options"}:
                continue
            m = method.upper()
            key = (m, path)
            items.append(
                {
                    "method": m,
                    "path": path,
                    "operationId": op.get("operationId", ""),
                    "tags": op.get("tags", []),
                    "auth": classify_auth_from_openapi(op),
                    "execution_class": execution_class_for_op(m, path),
                    "in_openapi": True,
                    "in_router_stub": key in router_ops,
                    "in_chi_mount": key in chi_ops
                    or normalize_path(path) in {normalize_path(p) for _, p in chi_ops},
                }
            )

    chi_only = sorted(chi_ops - openapi_ops)
    router_only = sorted(router_ops - openapi_ops)

    payload = {
        "openapi_path_count": len({p for _, p in openapi_ops}),
        "openapi_operation_count": len(openapi_ops),
        "router_stub_count": len(router_ops),
        "chi_mounted_count": len(chi_ops),
        "chi_only_missing_openapi": [{"method": m, "path": p} for m, p in chi_only],
        "router_only_missing_openapi": [{"method": m, "path": p} for m, p in router_only],
        "operations": items,
        "unknown_auth_count": sum(1 for i in items if i["auth"] == "UNKNOWN"),
    }

    md = [
        "# REST Inventory",
        "",
        f"- openapi_path_count: **{payload['openapi_path_count']}**",
        f"- openapi_operation_count: **{payload['openapi_operation_count']}**",
        f"- chi_mounted_count: **{payload['chi_mounted_count']}**",
        f"- chi_only_missing_openapi: **{len(chi_only)}**",
        f"- unknown_auth_count: **{payload['unknown_auth_count']}**",
        "",
    ]
    out = write_inventory("REST_INVENTORY", md, payload)
    print(f"REST inventory written to {out}")
    return 0 if payload["unknown_auth_count"] == 0 else 1


if __name__ == "__main__":
    sys.exit(main())

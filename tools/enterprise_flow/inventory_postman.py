#!/usr/bin/env python3
"""Map Postman collection requests to OpenAPI operations."""

from __future__ import annotations

import json
import sys
from pathlib import Path
from urllib.parse import urlparse

from _inventory_common import load_openapi_ops, normalize_path, write_inventory

REPO = Path(__file__).resolve().parents[2]
POSTMAN = REPO / "postman" / "suites" / "production-full" / "avf-vending-production.full.postman_collection.json"


def flatten_requests(node: dict, prefix: str = "") -> list[dict]:
    out: list[dict] = []
    name = node.get("name", "")
    full_name = f"{prefix}/{name}".strip("/")
    if "request" in node:
        req = node["request"]
        method = (req.get("method") or "GET").upper()
        url = req.get("url")
        path = ""
        if isinstance(url, str):
            path = urlparse(url.replace("{{baseUrl}}", "http://localhost")).path
        elif isinstance(url, dict):
            raw = url.get("raw") or ""
            path = urlparse(raw.replace("{{baseUrl}}", "http://localhost")).path
            if not path and isinstance(url.get("path"), list):
                path = "/" + "/".join(str(p) for p in url["path"])
        out.append({"name": full_name, "method": method, "path": path})
    for item in node.get("item") or []:
        out.extend(flatten_requests(item, full_name))
    return out


def main() -> int:
    if not POSTMAN.is_file():
        print("Postman collection not found", file=sys.stderr)
        return 1
    coll = json.loads(POSTMAN.read_text(encoding="utf-8"))
    requests = flatten_requests(coll)
    openapi = load_openapi_ops()
    openapi_norm = {(m, normalize_path(p)) for m, p in openapi}

    items = []
    for r in requests:
        key = (r["method"], normalize_path(r["path"]))
        items.append({**r, "in_openapi": key in openapi_norm})

    payload = {
        "postman_request_count": len(items),
        "mapped_to_openapi": sum(1 for i in items if i["in_openapi"]),
        "requests": items,
    }
    md = [
        "# Postman Inventory",
        "",
        f"- postman_request_count: **{payload['postman_request_count']}**",
        f"- mapped_to_openapi: **{payload['mapped_to_openapi']}**",
        "",
    ]
    out = write_inventory("POSTMAN_INVENTORY", md, payload)
    print(f"Postman inventory written to {out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())

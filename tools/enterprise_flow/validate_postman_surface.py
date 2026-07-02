#!/usr/bin/env python3
"""Basic Postman vs OpenAPI operation coverage check."""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
COLLECTION = REPO / "postman" / "suites" / "production-full" / "avf-vending-production.full.postman_collection.json"
SWAGGER = REPO / "docs" / "swagger" / "swagger.json"
METHODS = frozenset({"get", "post", "put", "patch", "delete"})


def openapi_ops() -> set[tuple[str, str]]:
    data = json.loads(SWAGGER.read_text(encoding="utf-8"))
    ops: set[tuple[str, str]] = set()
    for path, item in data.get("paths", {}).items():
        for method in item:
            if method.lower() in METHODS:
                ops.add((method.upper(), path))
    return ops


def postman_ops() -> set[tuple[str, str]]:
    coll = json.loads(COLLECTION.read_text(encoding="utf-8"))
    ops: set[tuple[str, str]] = set()

    def walk(items):
        for it in items or []:
            if "request" in it:
                req = it["request"]
                method = (req.get("method") or "GET").upper()
                url = req.get("url")
                if isinstance(url, dict):
                    raw = url.get("raw") or ""
                else:
                    raw = str(url)
                m = re.search(r"(https?://[^/]+)(/.*)", raw)
                if m:
                    path = re.sub(r"\{\{[^}]+\}\}", r"{\1}", m.group(2))
                    path = re.sub(r":(\w+)", r"{\1}", path)
                    ops.add((method, path.split("?")[0]))
            if "item" in it:
                walk(it["item"])

    walk(coll.get("item", []))
    return ops


def main() -> int:
    if not COLLECTION.exists():
        print("Postman collection not found; skip")
        return 0
    oas = openapi_ops()
    pm = postman_ops()
    # Normalize postman {{baseUrl}}/v1/... paths
    pm_norm: set[tuple[str, str]] = set()
    for m, p in pm:
        p2 = re.sub(r"/v1/v1/", "/v1/", p)
        pm_norm.add((m, p2))

    missing = sorted(oas - pm_norm)
    report = {
        "openapi_operations": len(oas),
        "postman_requests": len(pm_norm),
        "missing_in_postman_sample": [{"method": m, "path": p} for m, p in missing[:100]],
        "missing_count": len(missing),
    }
    out_dir = REPO / "reports" / "enterprise-flow"
    ts_dirs = sorted(out_dir.glob("*"), reverse=True)
    target = ts_dirs[0] if ts_dirs else out_dir
    (target / "POSTMAN_SURFACE_COVERAGE.json").write_text(json.dumps(report, indent=2), encoding="utf-8")
    (target / "POSTMAN_SURFACE_COVERAGE.md").write_text(
        f"# Postman Surface Coverage\n\n- openapi ops: {len(oas)}\n- postman requests: {len(pm_norm)}\n- missing (approx): {len(missing)}\n",
        encoding="utf-8",
    )
    print("Postman surface report written (informational)")
    return 0


if __name__ == "__main__":
    sys.exit(main())

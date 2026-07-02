#!/usr/bin/env python3
"""Compare OpenAPI spec vs @Router stubs and Chi-mounted routes."""

from __future__ import annotations

import json
import sys
from pathlib import Path

from _inventory_common import (
    REPO,
    load_chi_mounted_ops,
    load_openapi_ops,
    load_router_ops,
    normalize_path,
    latest_verification_dir,
)

EXCEPTIONS = Path(__file__).resolve().parent / "accepted_surface_exceptions.json"


def main() -> int:
    openapi = load_openapi_ops()
    router = load_router_ops()
    chi = load_chi_mounted_ops()
    exc = json.loads(EXCEPTIONS.read_text(encoding="utf-8"))
    allowed_missing_oas = {
        (e["method"].upper(), e["path"]) for e in exc.get("rest_missing_in_openapi", [])
    }

    chi_norm = {(m, normalize_path(p)) for m, p in chi}
    openapi_norm = {(m, normalize_path(p)) for m, p in openapi}

    missing_in_openapi = sorted(
        (m, p)
        for (m, p) in chi
        if (m, normalize_path(p)) not in openapi_norm and not p.startswith("/metrics")
    )
    missing_in_router = sorted((m, p) for (m, p) in openapi if (m, p) not in router)

    report = {
        "openapi_path_count": len({p for _, p in openapi}),
        "openapi_operation_count": len(openapi),
        "router_stub_count": len(router),
        "chi_mounted_count": len(chi),
        "missing_in_openapi": [{"method": m, "path": p} for m, p in missing_in_openapi],
        "missing_in_router": [{"method": m, "path": p} for m, p in missing_in_router[:50]],
        "accepted_missing_in_openapi": list(allowed_missing_oas),
    }

    out_dir = latest_verification_dir()
    (out_dir / "REST_SURFACE_COVERAGE.json").write_text(json.dumps(report, indent=2), encoding="utf-8")
    md = [
        "# REST Surface Coverage",
        "",
        f"- openapi_operation_count: **{report['openapi_operation_count']}**",
        f"- chi_mounted_count: **{report['chi_mounted_count']}**",
        f"- missing_in_openapi: **{len(missing_in_openapi)}**",
        "",
    ]
    (out_dir / "REST_SURFACE_COVERAGE.md").write_text("\n".join(md), encoding="utf-8")

    failures = [
        (m, p)
        for m, p in missing_in_openapi
        if (m, p) not in allowed_missing_oas and "/health" not in p and p != "/metrics"
    ]
    if failures:
        print("REST FAIL: chi-mounted routes missing from OpenAPI:", failures[:10], file=sys.stderr)
        return 1
    print("REST surface validation OK")
    return 0


if __name__ == "__main__":
    sys.exit(main())

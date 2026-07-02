#!/usr/bin/env python3
"""Compare proto RPCs vs gRPC server registration and implementation."""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
PROTO_ROOT = REPO / "proto" / "avf"
GRPCSERVER = REPO / "internal" / "grpcserver"
EXCEPTIONS = Path(__file__).resolve().parent / "accepted_surface_exceptions.json"


def parse_proto_rpcs() -> dict[str, list[str]]:
    services: dict[str, list[str]] = {}
    current_pkg = ""
    current_svc = ""
    for p in sorted(PROTO_ROOT.rglob("*.proto")):
        for line in p.read_text(encoding="utf-8").splitlines():
            m = re.match(r"^package\s+([\w.]+);", line)
            if m:
                current_pkg = m.group(1)
            m = re.match(r"^service\s+(\w+)", line)
            if m:
                current_svc = m.group(1)
                fq = f"{current_pkg}.{current_svc}"
                services.setdefault(fq, [])
            m = re.match(r"^\s*rpc\s+(\w+)", line)
            if m and current_svc:
                fq = f"{current_pkg}.{current_svc}"
                services.setdefault(fq, []).append(m.group(1))
    return services


def parse_registered_services() -> set[str]:
    text = ""
    for f in ["machine_grpc_services.go", "internal_queries.go", "server.go"]:
        fp = GRPCSERVER / f
        if fp.exists():
            text += fp.read_text(encoding="utf-8", errors="replace")
    found = set(re.findall(r"Register(\w+)Server\(", text))
    return found


def parse_unimplemented() -> set[str]:
    unimpl: set[str] = set()
    for go in GRPCSERVER.glob("*.go"):
        text = go.read_text(encoding="utf-8", errors="replace")
        for m in re.finditer(
            r"func\s+\([^)]+\)\s+(\w+)\([^)]*\)\s+\([^)]*\)\s*\{\s*return\s+nil,\s*status\.Errorf\(codes\.Unimplemented",
            text,
            re.S,
        ):
            unimpl.add(m.group(1))
    return unimpl


def main() -> int:
    services = parse_proto_rpcs()
    registered = parse_registered_services()
    unimpl_methods = parse_unimplemented()
    exc = json.loads(EXCEPTIONS.read_text(encoding="utf-8"))
    allowed_unimpl = set(exc.get("grpc_unimplemented_rpc", []))
    allowed_unreg = set(exc.get("grpc_unregistered_services", []))

    total_services = len(services)
    total_rpcs = sum(len(v) for v in services.values())

    missing_impl: list[str] = []
    for fq, rpcs in services.items():
        svc_short = fq.split(".")[-1]
        if fq in allowed_unreg:
            continue
        if f"Register{svc_short}Server" not in registered and svc_short not in registered:
            for rpc in rpcs:
                missing_impl.append(f"{fq}/{rpc} (service not registered)")
            continue
        for rpc in rpcs:
            key = f"{fq}/{rpc}"
            if key in allowed_unimpl:
                continue
            # Heuristic: method must appear as func in grpcserver
            found = False
            for go in GRPCSERVER.glob("*.go"):
                if re.search(rf"func\s+\([^)]+\)\s+{rpc}\s*\(", go.read_text(encoding="utf-8", errors="replace")):
                    found = True
                    break
            if not found and rpc in unimpl_methods:
                continue
            if not found:
                missing_impl.append(key)

    report = {
        "proto_service_count": total_services,
        "proto_rpc_count": total_rpcs,
        "registered_service_count": len(registered),
        "missing_implementation": missing_impl,
        "intentionally_unimplemented": list(allowed_unimpl),
        "unregistered_legacy_services": list(allowed_unreg),
    }

    out_dir = REPO / "reports" / "enterprise-flow"
    ts_dirs = sorted(out_dir.glob("*"), reverse=True)
    target = ts_dirs[0] if ts_dirs else out_dir
    (target / "GRPC_SURFACE_COVERAGE.json").write_text(json.dumps(report, indent=2), encoding="utf-8")
    (target / "GRPC_SURFACE_COVERAGE.md").write_text(
        f"# gRPC Surface Coverage\n\n- services: {total_services}\n- rpcs: {total_rpcs}\n- missing: {len(missing_impl)}\n",
        encoding="utf-8",
    )

    if missing_impl:
        print("gRPC FAIL:", missing_impl[:5], file=sys.stderr)
        return 1
    print("gRPC surface validation OK")
    return 0


if __name__ == "__main__":
    sys.exit(main())

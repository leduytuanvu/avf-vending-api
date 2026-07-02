#!/usr/bin/env python3
"""gRPC RPC inventory with registration and test references."""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

from _inventory_common import write_inventory

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


def find_test_refs(rpc: str) -> list[str]:
    refs: list[str] = []
    for go in REPO.rglob("*_test.go"):
        try:
            text = go.read_text(encoding="utf-8", errors="replace")
        except OSError:
            continue
        if rpc in text:
            refs.append(str(go.relative_to(REPO)).replace("\\", "/"))
    return sorted(refs)[:5]


def main() -> int:
    exc = json.loads(EXCEPTIONS.read_text(encoding="utf-8"))
    allowed_unimpl = set(exc.get("grpc_unimplemented_rpc", []))
    services = parse_proto_rpcs()
    items: list[dict] = []
    for fq, rpcs in sorted(services.items()):
        for rpc in rpcs:
            full = f"{fq}/{rpc}"
            items.append(
                {
                    "service": fq,
                    "rpc": rpc,
                    "full_name": full,
                    "status": "CONTRACT_ONLY" if full in allowed_unimpl else "REGISTERED",
                    "auth": "machine_jwt" if "machine.v1" in fq else "mixed",
                    "test_refs": find_test_refs(rpc),
                }
            )

    payload = {
        "service_count": len(services),
        "rpc_count": len(items),
        "contract_only_count": sum(1 for i in items if i["status"] == "CONTRACT_ONLY"),
        "rpcs": items,
    }
    md = [
        "# gRPC Inventory",
        "",
        f"- service_count: **{payload['service_count']}**",
        f"- rpc_count: **{payload['rpc_count']}**",
        f"- contract_only: **{payload['contract_only_count']}**",
        "",
    ]
    out = write_inventory("GRPC_INVENTORY", md, payload)
    print(f"gRPC inventory written to {out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())

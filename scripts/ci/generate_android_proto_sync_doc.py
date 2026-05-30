#!/usr/bin/env python3
"""Generate docs/api/android-proto-sync.md from proto/avf/machine/v1/*.proto."""
from __future__ import annotations

import re
import sys
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
PROTO_DIR = ROOT / "proto" / "avf" / "machine" / "v1"
OUT = ROOT / "docs" / "api" / "android-proto-sync.md"

SERVICE_RE = re.compile(r"^\s*service\s+([A-Za-z0-9_]+)\s*\{", re.MULTILINE)
RPC_RE = re.compile(r"^\s*rpc\s+([A-Za-z0-9_]+)\s*\(\s*([A-Za-z0-9_.]+)\s*\)\s*returns\s*\(\s*([A-Za-z0-9_.]+)\s*\)", re.MULTILINE)
DEPRECATED_RPC_RE = re.compile(
    r"rpc\s+([A-Za-z0-9_]+)\s*\([^)]*\)\s*returns\s*\([^)]*\)\s*\{\s*option\s+deprecated\s*=\s*true",
    re.MULTILINE | re.DOTALL,
)


def parse_proto(path: Path) -> tuple[list[str], list[tuple[str, str, str, bool]]]:
    text = path.read_text(encoding="utf-8")
    services = SERVICE_RE.findall(text)
    deprecated = set(DEPRECATED_RPC_RE.findall(text))
    rpcs: list[tuple[str, str, str, bool]] = []
    for name, req, resp in RPC_RE.findall(text):
        rpcs.append((name, req.split(".")[-1], resp.split(".")[-1], name in deprecated))
    return services, rpcs


def main() -> int:
    if not PROTO_DIR.is_dir():
        print(f"ERROR: missing {PROTO_DIR}", file=sys.stderr)
        return 1

    rows: list[tuple[str, str, str, str, str]] = []
    for path in sorted(PROTO_DIR.glob("*.proto")):
        if path.name == "machine_runtime.proto":
            continue
        services, rpcs = parse_proto(path)
        for svc in services:
            for rpc, req, resp, dep in rpcs:
                rows.append((svc, rpc, req, resp, "deprecated" if dep else "active"))

    # machine_runtime.proto is import-only; services live in leaf protos.
    ts = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    lines = [
        "# Android proto sync index (generated)",
        "",
        f"**Generated:** `{ts}` by `scripts/ci/generate_android_proto_sync_doc.py`",
        "",
        "Copy `proto/avf/machine/v1/*.proto` into the Android app (Buf, Gradle protobuf, or manual sync).",
        "Canonical runtime contract: [`machine-grpc-production-contract.md`](machine-grpc-production-contract.md).",
        "",
        "| Service | RPC | Request | Response | Status |",
        "|---------|-----|---------|----------|--------|",
    ]
    for svc, rpc, req, resp, st in sorted(rows, key=lambda r: (r[0], r[1])):
        lines.append(f"| `{svc}` | `{rpc}` | `{req}` | `{resp}` | {st} |")
    lines.append("")
    lines.append(f"**Total RPCs:** {len(rows)}")
    lines.append("")

    OUT.parent.mkdir(parents=True, exist_ok=True)
    OUT.write_text("\n".join(lines), encoding="utf-8")
    print(f"OK: wrote {OUT} ({len(rows)} RPCs)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""Verify docs/api/machine-grpc.md mentions every avf.machine.v1 service."""
from __future__ import annotations

import re
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
PROTO_DIR = ROOT / "proto" / "avf" / "machine" / "v1"
DOC_PATH = ROOT / "docs" / "api" / "machine-grpc.md"


def main() -> int:
    if not PROTO_DIR.is_dir():
        print(f"ERROR: missing {PROTO_DIR}", file=sys.stderr)
        return 1
    if not DOC_PATH.is_file():
        print(f"ERROR: missing {DOC_PATH}", file=sys.stderr)
        return 1

    services: set[str] = set()
    for path in sorted(PROTO_DIR.glob("*.proto")):
        text = path.read_text(encoding="utf-8")
        services.update(re.findall(r"^\s*service\s+([A-Za-z0-9_]+)\s*\{", text, re.MULTILINE))

    doc = DOC_PATH.read_text(encoding="utf-8")
    missing = sorted(s for s in services if s not in doc)
    if missing:
        print("ERROR: docs/api/machine-grpc.md is missing machine gRPC services:", file=sys.stderr)
        for svc in missing:
            print(f"  {svc}", file=sys.stderr)
        return 1

    print(f"OK: machine gRPC docs mention {len(services)} services")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

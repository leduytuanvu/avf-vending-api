#!/usr/bin/env python3
"""
Gate: P0 flows in e2e-flow-coverage.json must be present and coverage_status must not be 'missing'.
'blocked' is allowed only when blocked_reason is non-empty.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]


def main() -> int:
    path = REPO_ROOT / "reports" / "test" / "e2e-flow-coverage.json"
    if not path.exists():
        print(f"check-flow-coverage: missing {path}", file=sys.stderr)
        return 1
    doc = json.loads(path.read_text(encoding="utf8"))
    flows = doc.get("flows") or []
    if not flows:
        print("check-flow-coverage: flows[] empty", file=sys.stderr)
        return 1
    rc = 0
    for fl in flows:
        if fl.get("priority") != "P0":
            continue
        st = fl.get("coverage_status")
        if st == "missing":
            print(f"check-flow-coverage: P0 flow {fl.get('flow_id')} is missing coverage mapping", file=sys.stderr)
            rc = 1
        if st == "blocked" and not (fl.get("blocked_reason") or "").strip():
            print(f"check-flow-coverage: P0 flow {fl.get('flow_id')} blocked without reason", file=sys.stderr)
            rc = 1
    if rc == 0:
        print(f"check-flow-coverage: OK ({len(flows)} flows)")
    return rc


if __name__ == "__main__":
    sys.exit(main())

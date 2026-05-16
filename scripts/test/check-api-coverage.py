#!/usr/bin/env python3
"""
Gate: REST OpenAPI coverage JSON must exist, be non-empty, and every operation must declare coverage_status
in {scripted, partial, planned, missing, blocked}. Missing 'blocked' reasons when blocked is optional but warned.

Exit 1 if rest-api-coverage.json missing/empty or any operation has invalid/missing coverage_status.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]
ALLOWED = frozenset({"scripted", "partial", "planned", "missing", "blocked"})


def main() -> int:
    path = REPO_ROOT / "reports" / "test" / "rest-api-coverage.json"
    if not path.exists():
        print(f"check-api-coverage: missing {path}", file=sys.stderr)
        return 1
    doc = json.loads(path.read_text(encoding="utf8"))
    ops = doc.get("operations") or []
    if not ops:
        print("check-api-coverage: operations[] empty", file=sys.stderr)
        return 1
    bad = 0
    for op in ops:
        st = op.get("coverage_status")
        if st not in ALLOWED:
            print(f"check-api-coverage: bad coverage_status for {op.get('operation_id')}: {st!r}", file=sys.stderr)
            bad += 1
    if bad:
        return 1
    print(f"check-api-coverage: OK ({len(ops)} operations)")
    return 0


if __name__ == "__main__":
    sys.exit(main())

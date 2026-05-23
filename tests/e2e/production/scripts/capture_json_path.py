"""Extract a dotted JSON path from a response body file (no secrets logged)."""
from __future__ import annotations

import json
import sys
from pathlib import Path


def main() -> int:
    if len(sys.argv) != 3:
        print("usage: capture_json_path.py <body.json> <dotted.path>", file=sys.stderr)
        return 2
    body = Path(sys.argv[1])
    path = sys.argv[2].lstrip(".")
    data = json.loads(body.read_text(encoding="utf-8"))
    cur = data
    for part in path.split("."):
        if not part:
            continue
        if not isinstance(cur, dict) or part not in cur:
            cur = None
            break
        cur = cur[part]
    if cur is None:
        return 0
    print(str(cur))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

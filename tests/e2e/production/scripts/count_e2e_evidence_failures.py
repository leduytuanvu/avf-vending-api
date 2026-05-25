#!/usr/bin/env python3
"""Count failed flows from production E2E RESULT.md flow table rows."""
from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

FLOW_ROW = re.compile(
    r"\|\s*([^\|]+?)\s*\|\s*([^\|]+?)\s*\|\s*(\w+)\s*\|\s*(\S+)\s*\|\s*`([^`]+)`\s*\|"
)
FAIL_STATUSES = frozenset({"fail", "optional-fail"})


def count_failures(result_md: Path) -> int:
    if not result_md.is_file():
        return 0
    text = result_md.read_text(encoding="utf-8")
    return sum(
        1
        for m in FLOW_ROW.finditer(text)
        if m.group(4).strip() in FAIL_STATUSES
    )


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("result_md", type=Path, help="Path to .e2e-runs/.../RESULT.md")
    args = ap.parse_args()
    print(count_failures(args.result_md))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

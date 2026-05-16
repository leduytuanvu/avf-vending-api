#!/usr/bin/env python3
"""Merge ephemeral _audit_cmds.json rows into audit-commands.json envelope (used by run-full-backend-test-audit.sh)."""

import json
import os
import sys
from pathlib import Path


def main() -> int:
    src = Path(os.environ["AUDIT_ROWS_FILE"])
    dst = Path(os.environ["REPORT_DIR"]) / "audit-commands.json"
    if not src.is_file():
        print(f"merge_audit_commands: missing {src}", file=sys.stderr)
        return 1
    rows = json.loads(src.read_text(encoding="utf-8"))
    dst.write_text(json.dumps({"commands": rows}, indent=2) + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

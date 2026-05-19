#!/usr/bin/env python3
"""Load production smoke evidence JSON with actionable errors (no raw JSONDecodeError on empty/invalid files)."""
from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any


class SmokeEvidenceError(RuntimeError):
    """Invalid or unreadable smoke evidence JSON."""


def load_json_document(path: Path, *, kind: str) -> Any:
    label = str(path)
    if not path.is_file():
        raise SmokeEvidenceError(f"{kind}: file missing: {label}")
    raw = path.read_bytes()
    if not raw.strip():
        raise SmokeEvidenceError(
            f"{kind}: document is empty (expected non-empty JSON from smoke emitter; "
            f"endpoint=n/a, http_status=n/a, content_type=n/a): {label}"
        )
    text = raw.decode("utf-8", errors="replace")
    try:
        return json.loads(text)
    except json.JSONDecodeError as e:
        snippet = text[:500].replace("\r", "\\r").replace("\n", "\\n")
        raise SmokeEvidenceError(
            f"{kind}: invalid JSON ({e.msg} at line {e.lineno} column {e.colno}) in {label}; "
            f"first 500 chars: {snippet!r}"
        ) from None


def load_smoke_evidence_dict(path: Path) -> dict[str, Any]:
    data = load_json_document(path, kind="smoke evidence")
    if not isinstance(data, dict):
        raise SmokeEvidenceError(f"smoke evidence: root must be an object, got {type(data).__name__}: {path}")
    return data


def cmd_extract_status(path: Path) -> int:
    payload = load_smoke_evidence_dict(path)
    print((payload.get("overall_status") or "unknown").strip())
    return 0


def cmd_failed_checks_tsv(path: Path) -> int:
    payload = load_smoke_evidence_dict(path)
    for entry in payload.get("failed_checks") or []:
        if not isinstance(entry, dict):
            continue
        name = str(entry.get("name", "smoke"))
        detail = str(entry.get("detail", name))
        print(f"{name}\t{detail}")
    return 0


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(description="Parse smoke-cluster-*.json evidence safely.")
    sub = ap.add_subparsers(dest="cmd", required=True)

    p_status = sub.add_parser("extract-status", help="Print overall_status for shell capture.")
    p_status.add_argument("path", type=Path)

    p_tsv = sub.add_parser("failed-checks-tsv", help="Print name\\tdetail lines for failed_checks.")
    p_tsv.add_argument("path", type=Path)

    args = ap.parse_args(argv)
    try:
        if args.cmd == "extract-status":
            return cmd_extract_status(args.path)
        if args.cmd == "failed-checks-tsv":
            return cmd_failed_checks_tsv(args.path)
    except SmokeEvidenceError as ex:
        print(str(ex), file=sys.stderr)
        return 2
    raise AssertionError("unhandled subcommand")


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))

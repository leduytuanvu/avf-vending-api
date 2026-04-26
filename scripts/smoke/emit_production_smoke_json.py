#!/usr/bin/env python3
"""Assemble production smoke JSON from smoke_prod.sh TSV checks and exported env (enterprise evidence)."""
from __future__ import annotations

import json
import os
import sys
from pathlib import Path


def _load_checks(tsv_path: Path) -> list[dict[str, str]]:
    rows: list[dict[str, str]] = []
    if not tsv_path.is_file():
        return rows
    for line in tsv_path.read_text(encoding="utf-8", errors="replace").splitlines():
        if not line.strip():
            continue
        parts = line.split("\t")
        while len(parts) < 5:
            parts.append("")
        rows.append(
            {
                "name": parts[0],
                "status": parts[1],
                "url": parts[2],
                "http_status": parts[3],
                "detail": parts[4],
            }
        )
    return rows


def _skipped_reasons(path: Path) -> list[dict[str, str]]:
    out: list[dict[str, str]] = []
    if not path.is_file():
        return out
    for line in path.read_text(encoding="utf-8", errors="replace").splitlines():
        if not line.strip():
            continue
        code, _sep, detail = line.partition("\t")
        out.append({"code": code.strip(), "detail": (detail or "").strip()})
    return out


def _bool_from_env(name: str, default: bool = False) -> bool:
    v = (os.environ.get(name) or "").strip().lower()
    if v in ("1", "true", "yes", "on"):
        return True
    if v in ("0", "false", "no", "off", ""):
        return default
    return default


def main() -> None:
    checks_path = Path(os.environ.get("SMOKE_CHECKS_FILE", ""))

    checks = _load_checks(checks_path)
    failed = [c for c in checks if c.get("status") == "fail"]
    skipped = [c for c in checks if c.get("status") == "skip"]
    skip_p = (os.environ.get("SMOKE_SKIPPED_REASONS_FILE") or "").strip()
    skipped_reasons = _skipped_reasons(Path(skip_p)) if skip_p else []

    overall = (os.environ.get("SMOKE_OVERALL_STATUS") or "fail").strip().lower()
    if overall not in ("pass", "fail"):
        overall = "fail" if failed else "pass"

    payload: dict = {
        "level": (os.environ.get("SMOKE_LEVEL") or "business-readonly").strip(),
        "started_at_utc": (os.environ.get("SMOKE_STARTED_AT_UTC") or "").strip(),
        "completed_at_utc": (os.environ.get("SMOKE_COMPLETED_AT_UTC") or "").strip(),
        "overall_status": overall,
        "final_result": overall,
        "base_url": (os.environ.get("SMOKE_BASE_URL") or "").strip(),
        "connect_to_host": (os.environ.get("SMOKE_CONNECT_TO_HOST") or "").strip(),
        "smoke_label": (os.environ.get("SMOKE_LABEL") or "").strip(),
        "health_result": (os.environ.get("SMOKE_HEALTH_RESULT") or "not-run").strip(),
        "business_readonly_result": (os.environ.get("SMOKE_BUSINESS_READONLY_RESULT") or "not-run").strip(),
        "business_synthetic_result": (os.environ.get("SMOKE_BUSINESS_SYNTHETIC_RESULT") or "not-run").strip(),
        "zero_side_effects_claim": _bool_from_env("SMOKE_ZERO_SIDE_EFFECTS_CLAIM", default=True),
        "base_url_result": (os.environ.get("SMOKE_BASE_URL_RESULT") or "not-run").strip(),
        "critical_read_result": (os.environ.get("SMOKE_CRITICAL_READ_RESULT") or "not-run").strip(),
        "optional_db_read_result": (os.environ.get("SMOKE_OPTIONAL_DB_READ_RESULT") or "not-run").strip(),
        "checks": checks,
        "failed_checks": [
            {
                "name": x.get("name", ""),
                "url": x.get("url", ""),
                "http_status": x.get("http_status", ""),
                "detail": x.get("detail", ""),
            }
            for x in failed
        ],
        "skipped_checks": [{"name": x.get("name", ""), "detail": x.get("detail", "")} for x in skipped],
        "skipped_reasons": skipped_reasons,
    }
    sys.stdout.write(json.dumps(payload, indent=2) + "\n")


if __name__ == "__main__":
    main()

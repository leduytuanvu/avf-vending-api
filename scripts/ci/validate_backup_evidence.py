#!/usr/bin/env python3
"""
Validate production DB backup + restore-drill evidence JSON (see docs/contracts/backup-evidence.schema.json).
Exit 0 on PASS, 1 on FAIL. Intended for CI and pre-flight operator checks.
"""
from __future__ import annotations

import argparse
import json
import re
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Mapping, NoReturn, TextIO

REQUIRED_KEYS = (
    "schema_version",
    "evidence_type",
    "repository",
    "source_environment",
    "backup_started_at_utc",
    "backup_completed_at_utc",
    "backup_size_bytes",
    "backup_sha256",
    "restore_drill_required",
    "restore_drill_result",
    "restore_target",
    "schema_validation_result",
    "expires_at_utc",
)

SHA_RE = re.compile(r"^(?:sha256:)?([0-9a-fA-F]{64})$")


def fail(msg: str, *, out: TextIO = sys.stderr) -> NoReturn:
    print(f"FAIL: {msg}", file=out)
    sys.exit(1)


def ok(msg: str, *, out: TextIO = sys.stdout) -> None:
    print(f"PASS: {msg}", file=out)


def parse_iso_utc(s: str) -> datetime:
    s = s.strip()
    if s.endswith("Z"):
        s = s[:-1] + "+00:00"
    dt = datetime.fromisoformat(s)
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return dt.astimezone(timezone.utc)


def normalize_hex_sha(raw: str) -> str:
    m = SHA_RE.match((raw or "").strip())
    if not m:
        fail("backup_sha256 must be 64 hex chars, optional sha256: prefix")
    return m.group(1).lower()


def as_bool(v: Any) -> bool:
    if isinstance(v, bool):
        return v
    if isinstance(v, str):
        return v.lower() in ("true", "1", "yes")
    fail(f"restore_drill_required must be a boolean, got {type(v).__name__}")


def load_payload(path: Path) -> dict[str, Any]:
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as e:
        fail(f"cannot read file {path}: {e}")
    try:
        data = json.loads(text)
    except json.JSONDecodeError as e:
        fail(f"invalid JSON: {e}")
    if not isinstance(data, dict):
        fail("root JSON value must be an object")
    return data


def validate_payload(
    data: Mapping[str, Any],
    *,
    for_production_migration: bool,
    expect_repository: str | None,
    now: datetime,
) -> None:
    if data.get("evidence_draft") is True:
        fail("evidence_draft is true — complete and validate real backup evidence, not a template")
    for k in REQUIRED_KEYS:
        if k not in data:
            fail(f"missing required key: {k}")

    if data.get("schema_version") != "backup-evidence-v1":
        fail("schema_version must be backup-evidence-v1")
    if data.get("evidence_type") != "production-db-backup":
        fail('evidence_type must be "production-db-backup"')
    if data.get("source_environment") != "production":
        fail("source_environment must be production for this validator")

    repo = str(data.get("repository", "")).strip()
    if not re.match(r"^[^/]+/[^/]+$", repo):
        fail("repository must look like org/name (one slash, no path segments)")
    if expect_repository and repo != expect_repository:
        fail(f"repository must match {expect_repository!r}, got {repo!r}")

    dbn = str(data.get("database_name") or "").strip()
    lid = str(data.get("logical_db_id") or "").strip()
    if not dbn and not lid:
        fail("at least one of database_name or logical_db_id is required (non-empty)")

    art = str(data.get("backup_artifact_id") or "").strip()
    loc = str(data.get("backup_location_redacted") or "").strip()
    if not art and not loc:
        fail("at least one of backup_artifact_id or backup_location_redacted is required (non-empty)")

    c_run = str(data.get("created_by_workflow_run_id") or "").strip()
    op = str(data.get("operator") or "").strip()
    if not c_run and not op:
        fail("at least one of created_by_workflow_run_id or operator is required (non-empty)")

    if not isinstance(data.get("backup_size_bytes"), int) or data["backup_size_bytes"] < 1:
        fail("backup_size_bytes must be a positive integer")
    normalize_hex_sha(str(data.get("backup_sha256", "")))

    try:
        t0 = parse_iso_utc(str(data.get("backup_started_at_utc", "")))
        t1 = parse_iso_utc(str(data.get("backup_completed_at_utc", "")))
    except ValueError as e:
        fail(f"invalid backup timestamp ISO 8601: {e}")
    if t1 < t0:
        fail("backup_completed_at_utc must be >= backup_started_at_utc")

    expires = parse_iso_utc(str(data.get("expires_at_utc", "")))
    if expires <= now:
        fail("expires_at_utc is not in the future (evidence expired for deployment)")

    rd_req = as_bool(data.get("restore_drill_required"))
    rd_res = str(data.get("restore_drill_result", "")).strip()
    if rd_res not in ("pass", "fail", "skipped"):
        fail("restore_drill_result must be one of: pass, fail, skipped")
    r_target = str(data.get("restore_target", "")).strip()
    if r_target not in ("staging", "preprod", "temp-db", "not_run"):
        fail("restore_target must be one of: staging, preprod, temp-db, not_run")
    c_at = data.get("restore_drill_completed_at_utc", None)
    c_at_s: str | None
    if c_at is None:
        c_at_s = None
    else:
        c_at_s = str(c_at).strip() or None

    if rd_req:
        if rd_res != "pass":
            fail("restore_drill_required is true, so restore_drill_result must be pass")
        if c_at_s is None:
            fail("restore_drill_required is true, so restore_drill_completed_at_utc is required")
        if r_target == "not_run":
            fail("when restore_drill_required is true, restore_target must not be not_run")
        try:
            parse_iso_utc(c_at_s)
        except ValueError as e:
            fail(f"invalid restore_drill_completed_at_utc: {e}")
    else:
        if rd_res != "skipped":
            fail(
                "when restore_drill_required is false, restore_drill_result must be skipped "
                f"(use restore_drill_required true when a restore drill is mandatory for this change)"
            )
        if r_target != "not_run":
            fail("when no restore drill is required, restore_target must be not_run")
        if c_at_s is not None:
            fail("when no restore drill is required, restore_drill_completed_at_utc must be null or empty")

    svr = str(data.get("schema_validation_result", "")).strip()
    if svr not in ("pass", "fail", "not_applicable"):
        fail("schema_validation_result must be one of: pass, fail, not_applicable")
    if for_production_migration:
        if svr != "pass":
            fail("for production migration, schema_validation_result must be pass")


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("path", type=Path, nargs="?", help="Path to backup evidence JSON file")
    p.add_argument(
        "--for-production-migration",
        action="store_true",
        help="Stricter rules for production migration (Deploy Production with run_migration=true).",
    )
    p.add_argument(
        "--expect-repository",
        default=None,
        help="If set, evidence.repository must match (e.g. org/repo from GITHUB_REPOSITORY).",
    )
    p.add_argument(
        "--now-utc",
        default=None,
        help="ISO 8601 instant for tests only (default: system clock).",
    )
    args = p.parse_args()
    if not args.path:
        p.print_help()
        fail("a path to the evidence JSON file is required (positional argument)")

    now = (
        parse_iso_utc(args.now_utc)
        if args.now_utc
        else datetime.now(timezone.utc)
    )
    data = load_payload(args.path)
    expect_repo = args.expect_repository
    if not expect_repo:
        import os
        expect_repo = (os.environ.get("GITHUB_REPOSITORY") or "").strip() or None
    validate_payload(
        data,
        for_production_migration=args.for_production_migration,
        expect_repository=expect_repo,
        now=now,
    )

    ok("backup evidence JSON is valid for the selected policy")


if __name__ == "__main__":
    main()

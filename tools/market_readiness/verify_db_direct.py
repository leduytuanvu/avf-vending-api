#!/usr/bin/env python3
"""Direct Postgres assertions for market readiness (PROD_DATABASE_URL)."""

from __future__ import annotations

import argparse
import os
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "production_full_test"))
sys.path.insert(0, str(Path(__file__).resolve().parent))

from market_common import ATTACHMENT_SQL_COLUMNS, bundle_dir, record_pre_destructive_backup, write_json  # noqa: E402
from entity_registry import EntityRegistry  # noqa: E402


def _connect():
    url = os.environ.get("PROD_DATABASE_URL", "").strip()
    if not url:
        raise RuntimeError("PROD_DATABASE_URL not set (session-only; never commit)")
    try:
        import psycopg  # type: ignore
    except ImportError:
        try:
            import psycopg2 as psycopg  # type: ignore
        except ImportError as exc:
            raise RuntimeError("Install psycopg or psycopg2-binary for direct DB verification") from exc
    return psycopg.connect(url)


def _fetchone(cur, sql: str, params: tuple) -> dict | None:
    cur.execute(sql, params)
    row = cur.fetchone()
    if row is None:
        return None
    cols = [d[0] for d in cur.description]
    return dict(zip(cols, row))


def verify_machine(conn, machine_id: str) -> list[dict]:
    checks: list[dict] = []
    with conn.cursor() as cur:
        m = _fetchone(
            cur,
            "SELECT id, code, sale_enabled, status FROM machines WHERE id = %s",
            (machine_id,),
        )
        checks.append({"name": "machine_exists", "pass": m is not None, "detail": m.get("code") if m else "missing"})
        if m:
            code = str(m.get("code") or "")
            checks.append(
                {
                    "name": "machine_code_format",
                    "pass": code.startswith("AVF") and code[3:].isdigit() and len(code[3:]) >= 6,
                    "detail": code,
                }
            )

        active = _fetchone(
            cur,
            """
            SELECT id, status, reason, previous_attachment_id,
                   android_id, android_serial, board_serial, device_serial,
                   sim_serial, sim_iccid, sim_operator, sim_country_iso,
                   manufacturer, brand, model, device_model, hardware, product,
                   android_release, sdk_int, package_name, version_name, version_code,
                   app_build_sha, boot_id, network_type, network_state
            FROM machine_device_attachments
            WHERE machine_id = %s AND status = 'active'
            """,
            (machine_id,),
        )
        checks.append({"name": "one_active_attachment", "pass": active is not None, "detail": "present" if active else "missing"})
        if active:
            for col in ATTACHMENT_SQL_COLUMNS:
                if col in ("previous_attachment_id", "status", "reason"):
                    continue
                val = active.get(col)
                checks.append({"name": f"attachment.{col}", "pass": val is not None and str(val) != "", "detail": "set"})

        snap = _fetchone(
            cur,
            """
            SELECT machine_id, current_device_attachment_id, current_runtime_app_session_id,
                   sell_ready, sale_enabled
            FROM machine_current_snapshot
            WHERE machine_id = %s
            """,
            (machine_id,),
        )
        checks.append({"name": "snapshot_row", "pass": snap is not None, "detail": "present" if snap else "missing"})
        if snap and active:
            checks.append(
                {
                    "name": "snapshot_attachment_id_match",
                    "pass": str(snap.get("current_device_attachment_id")) == str(active.get("id")),
                    "detail": "match",
                }
            )
    return checks


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--pass", dest="pass_num", type=int, default=1)
    args = parser.parse_args()

    out = bundle_dir()
    record_pre_destructive_backup(out)

    reg = EntityRegistry()
    machine_id = reg.get("machineId")
    if not machine_id:
        print("No machineId in registry; run bootstrap first", file=sys.stderr)
        return 1

    try:
        conn = _connect()
    except Exception as exc:
        payload = {"pass": args.pass_num, "error": str(exc), "checks": [], "fail_count": 1}
        write_json(out / f"DB_DIRECT_PASS_{args.pass_num}.json", payload)
        print(f"DB direct blocked: {exc}", file=sys.stderr)
        return 1

    try:
        checks = verify_machine(conn, machine_id)
    finally:
        conn.close()

    fail = sum(1 for c in checks if not c.get("pass"))
    payload = {"pass": args.pass_num, "machine_id": machine_id, "checks": checks, "fail_count": fail}
    write_json(out / f"DB_DIRECT_PASS_{args.pass_num}.json", payload)
    return 1 if fail else 0


if __name__ == "__main__":
    raise SystemExit(main())

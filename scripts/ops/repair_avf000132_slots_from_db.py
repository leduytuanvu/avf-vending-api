#!/usr/bin/env python3
"""Repair AVF000132 slot assignments directly in production Postgres.

Fixes planogram publish duplicate-key failures by updating in place:
  - machine_layout_slots (named layout)
  - machine_slot_configs (current commerce configs)
  - machine_slot_state + slots (legacy read model)

Usage:
  set DATABASE_URL=postgres://...
  python repair_avf000132_slots_from_db.py --dry-run
  python repair_avf000132_slots_from_db.py --apply
"""
from __future__ import annotations

import argparse
import json
import os
import sys
import uuid
from pathlib import Path

import psycopg2
import psycopg2.extras

SCRIPT_DIR = Path(__file__).resolve().parent
DEFAULT_LAYOUT = SCRIPT_DIR / "output" / "avf000132-machine-layout.json"
MACHINE_ID = "01a089ec-c7bb-7e0d-83a9-6f599f061f12"
LAYOUT_ID = "01a096ed-f77b-75c3-ba0b-4eda79639029"
DEFAULT_PLANOGRAM_ID = "01a08a17-059f-7b80-b533-e24c6893b3bb"


def load_layout(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def load_database_url() -> str:
    url = os.environ.get("DATABASE_URL", "").strip()
    if url:
        return url
    for candidate in (
        SCRIPT_DIR / ".env.db.local",
        SCRIPT_DIR.parent.parent / ".env.db.local",
    ):
        if not candidate.is_file():
            continue
        for line in candidate.read_text(encoding="utf-8").splitlines():
            line = line.strip()
            if line.startswith("DATABASE_URL="):
                return line.split("=", 1)[1].strip().strip('"').strip("'")
    raise SystemExit(
        "DATABASE_URL required — set env var or scripts/ops/.env.db.local"
    )


def connect():
    return psycopg2.connect(load_database_url())


def fetch_skus(cur) -> dict[str, uuid.UUID]:
    cur.execute("SELECT sku, id FROM products WHERE sku IS NOT NULL")
    return {row[0]: row[1] for row in cur.fetchall()}


def fetch_planogram_id(cur, machine_id: str) -> str:
    cur.execute(
        """
        SELECT planogram_id FROM machine_slot_state
        WHERE machine_id = %s::uuid
        ORDER BY updated_at DESC
        LIMIT 1
        """,
        (machine_id,),
    )
    row = cur.fetchone()
    if row and row[0]:
        return str(row[0])
    cur.execute(
        """
        SELECT id FROM planograms
        WHERE status = 'published'
        ORDER BY updated_at DESC
        LIMIT 1
        """
    )
    row = cur.fetchone()
    return str(row[0]) if row else DEFAULT_PLANOGRAM_ID


def fetch_cabinet_layout(cur, machine_id: str, cabinet_code: str, layout_key: str, revision: int):
    cur.execute(
        """
        SELECT mc.id, msl.id
        FROM machine_cabinets mc
        JOIN machine_slot_layouts msl
          ON msl.machine_id = mc.machine_id AND msl.machine_cabinet_id = mc.id
        WHERE mc.machine_id = %s::uuid
          AND upper(mc.cabinet_code) = upper(%s)
          AND msl.layout_key = %s
          AND msl.revision = %s
        LIMIT 1
        """,
        (machine_id, cabinet_code, layout_key, revision),
    )
    row = cur.fetchone()
    if not row:
        cur.execute(
            """
            SELECT mc.id, msl.id
            FROM machine_cabinets mc
            JOIN machine_slot_layouts msl
              ON msl.machine_id = mc.machine_id AND msl.machine_cabinet_id = mc.id
            WHERE mc.machine_id = %s::uuid
              AND upper(mc.cabinet_code) = 'CAB-A'
              AND msl.layout_key = 'default'
              AND msl.revision = 1
            LIMIT 1
            """,
            (machine_id,),
        )
        row = cur.fetchone()
    if not row:
        raise RuntimeError(f"cabinet/layout not found for machine {machine_id}")
    return row[0], row[1]


def dedupe_current_configs(cur, machine_id: str, dry_run: bool) -> int:
    cur.execute(
        """
        SELECT slot_code, count(*) AS n
        FROM machine_slot_configs
        WHERE machine_id = %s::uuid AND is_current = true
        GROUP BY slot_code
        HAVING count(*) > 1
        """,
        (machine_id,),
    )
    dups = cur.fetchall()
    fixed = 0
    for slot_code, n in dups:
        print(f"WARN duplicate current configs slot={slot_code} count={n}")
        cur.execute(
            """
            SELECT id FROM machine_slot_configs
            WHERE machine_id = %s::uuid AND slot_code = %s AND is_current = true
            ORDER BY updated_at DESC, created_at DESC
            """,
            (machine_id, slot_code),
        )
        ids = [r[0] for r in cur.fetchall()]
        for stale_id in ids[1:]:
            fixed += 1
            if dry_run:
                print(f"  would clear is_current on {stale_id}")
            else:
                cur.execute(
                    """
                    UPDATE machine_slot_configs
                    SET is_current = false, effective_to = coalesce(effective_to, now()), updated_at = now()
                    WHERE id = %s
                    """,
                    (stale_id,),
                )
    return fixed


def repair(dry_run: bool, layout_path: Path) -> int:
    layout = load_layout(layout_path)
    machine_id = layout.get("machine_id", MACHINE_ID)
    layout_id = LAYOUT_ID
    lay_meta = layout["layouts"][0]
    cabinet_code = lay_meta["cabinet_code"]
    layout_key = lay_meta["layout_key"]
    layout_revision = int(lay_meta["revision"])

    conn = connect()
    conn.autocommit = False
    cur = conn.cursor()
    try:
        sku_to_pid = fetch_skus(cur)
        planogram_id = fetch_planogram_id(cur, machine_id)
        print(f"planogram_id={planogram_id}")
        cab_id, slot_layout_id = fetch_cabinet_layout(
            cur, machine_id, cabinet_code, layout_key, layout_revision
        )
        print(f"cabinet_id={cab_id} slot_layout_id={slot_layout_id}")

        deduped = dedupe_current_configs(cur, machine_id, dry_run)
        print(f"deduped_current_rows={deduped}")

        layout_slot_updates = 0
        config_updates = 0
        config_inserts = 0
        legacy_updates = 0
        slot_table_updates = 0

        for slot in layout["slots"]:
            code = slot["slot_code"]
            idx = int(slot["slot_index"])
            sellable = bool(slot.get("sellable"))
            enabled = bool(slot.get("enabled"))
            price = int(slot.get("price_minor") or 0)
            qty = int(slot.get("inventory_quantity") or 0)
            max_qty = max(qty, 6) if sellable else 0
            product = slot.get("product") or {}
            sku = (product.get("sku") or "").strip()
            pid = sku_to_pid.get(sku) if sku else None
            op_state = "assigned" if pid and sellable else "unassigned"

            if dry_run:
                print(
                    f"slot {code} idx={idx} sku={sku or '-'} sellable={sellable} "
                    f"price={price} qty={qty}"
                )
                layout_slot_updates += 1
                continue

            cur.execute(
                """
                UPDATE machine_layout_slots
                SET product_id = %s,
                    price_minor = %s,
                    max_quantity = %s,
                    current_inventory = %s,
                    enabled = %s,
                    operational_state = %s,
                    updated_at = now()
                WHERE layout_id = %s::uuid AND slot_code = %s
                """,
                (pid, price if sellable else None, max_qty, qty, enabled, op_state, layout_id, code),
            )
            layout_slot_updates += cur.rowcount

            cur.execute(
                """
                SELECT id FROM machine_slot_configs
                WHERE machine_id = %s::uuid AND slot_code = %s AND is_current = true
                LIMIT 1
                """,
                (machine_id, code),
            )
            cfg = cur.fetchone()
            if cfg:
                cur.execute(
                    """
                    UPDATE machine_slot_configs
                    SET machine_cabinet_id = %s,
                        machine_slot_layout_id = %s,
                        slot_index = %s,
                        product_id = %s,
                        max_quantity = %s,
                        price_minor = %s,
                        updated_at = now()
                    WHERE id = %s
                    """,
                    (cab_id, slot_layout_id, idx, pid, max_qty, price if sellable else 0, cfg[0]),
                )
                config_updates += cur.rowcount
            elif sellable and pid:
                cur.execute(
                    """
                    INSERT INTO machine_slot_configs (
                        machine_id, machine_cabinet_id, machine_slot_layout_id,
                        slot_code, slot_index, product_id, max_quantity, price_minor,
                        effective_from, is_current, metadata
                    ) VALUES (
                        %s::uuid, %s, %s, %s, %s, %s, %s, %s, now(), true, '{}'::jsonb
                    )
                    """,
                    (machine_id, cab_id, slot_layout_id, code, idx, pid, max_qty, price),
                )
                config_inserts += 1

            cur.execute(
                """
                INSERT INTO machine_slot_state (
                    machine_id, planogram_id, slot_index, current_quantity, price_minor,
                    planogram_revision_applied, updated_at
                ) VALUES (%s::uuid, %s::uuid, %s, %s, %s, 2, now())
                ON CONFLICT (machine_id, planogram_id, slot_index)
                DO UPDATE SET
                    current_quantity = EXCLUDED.current_quantity,
                    price_minor = EXCLUDED.price_minor,
                    planogram_revision_applied = GREATEST(machine_slot_state.planogram_revision_applied, 2),
                    updated_at = now()
                """,
                (machine_id, planogram_id, idx, qty, price if sellable else 0),
            )
            legacy_updates += 1

            if pid and sellable:
                cur.execute(
                    """
                    UPDATE slots
                    SET product_id = %s, max_quantity = %s
                    WHERE planogram_id = %s::uuid AND slot_index = %s
                    """,
                    (pid, max_qty, planogram_id, idx),
                )
                slot_table_updates += cur.rowcount

        if not dry_run:
            cur.execute(
                """
                UPDATE machines
                SET sale_enabled = true,
                    desired_active_layout_id = %s::uuid,
                    active_layout_id = %s::uuid,
                    reported_active_layout_id = %s::uuid,
                    updated_at = now()
                WHERE id = %s::uuid
                """,
                (layout_id, layout_id, layout_id, machine_id),
            )

        if dry_run:
            conn.rollback()
            print("DRY RUN — no changes committed")
        else:
            conn.commit()
            print("COMMITTED")

        print(
            json.dumps(
                {
                    "layout_slot_updates": layout_slot_updates,
                    "config_updates": config_updates,
                    "config_inserts": config_inserts,
                    "legacy_updates": legacy_updates,
                    "slots_table_updates": slot_table_updates,
                    "deduped": deduped,
                },
                indent=2,
            )
        )

        if not dry_run:
            cur.execute(
                """
                SELECT msc.slot_code, p.sku, msc.price_minor, mls.current_inventory
                FROM machine_slot_configs msc
                LEFT JOIN products p ON p.id = msc.product_id
                LEFT JOIN machine_layout_slots mls
                  ON mls.layout_id = %s::uuid AND mls.slot_code = msc.slot_code
                WHERE msc.machine_id = %s::uuid AND msc.is_current = true
                  AND msc.slot_code IN ('A1','A2','A3','A10')
                ORDER BY msc.slot_code
                """,
                (layout_id, machine_id),
            )
            print("verify:")
            for row in cur.fetchall():
                print(" ", row)
        return 0
    except Exception:
        conn.rollback()
        raise
    finally:
        cur.close()
        conn.close()


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--layout", type=Path, default=DEFAULT_LAYOUT)
    parser.add_argument("--dry-run", action="store_true")
    parser.add_argument("--apply", action="store_true")
    args = parser.parse_args()
    if not args.dry_run and not args.apply:
        print("Specify --dry-run or --apply", file=sys.stderr)
        return 2
    return repair(dry_run=args.dry_run, layout_path=args.layout)


if __name__ == "__main__":
    raise SystemExit(main())

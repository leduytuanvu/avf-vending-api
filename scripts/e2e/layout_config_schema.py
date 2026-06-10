#!/usr/bin/env python3
"""Validate machine layout JSON for e2e setup/verify harnesses."""
from __future__ import annotations

import json
import re
from pathlib import Path
from typing import Any

SUPPORTED_SCHEMA_VERSIONS = {1}
_SLOT_INDEX_RANGE = re.compile(r"^(\d+)-(\d+)$")


class LayoutValidationError(ValueError):
    def __init__(self, errors: list[str]):
        self.errors = errors
        super().__init__("; ".join(errors))


def _err(errors: list[str], msg: str) -> None:
    errors.append(msg)


def _parse_slot_index_range(spec: str) -> tuple[int, int] | None:
    m = _SLOT_INDEX_RANGE.match((spec or "").strip())
    if not m:
        return None
    start, end = int(m.group(1)), int(m.group(2))
    if start > end:
        return None
    return start, end


def _slot_key(cabinet_code: str, slot_code: str) -> str:
    return f"{cabinet_code}:{slot_code}"


def validate_layout(doc: dict[str, Any]) -> list[str]:
    errors: list[str] = []

    if not isinstance(doc, dict):
        return ["layout root must be a JSON object"]

    sv = doc.get("schema_version")
    if sv is None:
        _err(errors, "schema_version is required")
    elif sv not in SUPPORTED_SCHEMA_VERSIONS:
        _err(errors, f"unsupported schema_version: {sv}")

    machine_id = doc.get("machine_id")
    if not machine_id or not isinstance(machine_id, str):
        _err(errors, "machine_id is required")

    cabinets = doc.get("cabinets")
    if not isinstance(cabinets, list) or not cabinets:
        _err(errors, "cabinets must be a non-empty array")
        cabinets = []

    cabinet_codes: set[str] = set()
    for i, cab in enumerate(cabinets):
        if not isinstance(cab, dict):
            _err(errors, f"cabinets[{i}] must be an object")
            continue
        code = cab.get("code")
        if not code or not isinstance(code, str):
            _err(errors, f"cabinets[{i}].code is required")
            continue
        if code in cabinet_codes:
            _err(errors, f"duplicate cabinet code: {code}")
        cabinet_codes.add(code)

    layouts = doc.get("layouts")
    if not isinstance(layouts, list) or not layouts:
        _err(errors, "layouts must be a non-empty array")
        layouts = []

    layout_keys: set[tuple[str, str]] = set()
    for i, lay in enumerate(layouts):
        if not isinstance(lay, dict):
            _err(errors, f"layouts[{i}] must be an object")
            continue
        cc = lay.get("cabinet_code")
        lk = lay.get("layout_key")
        if not cc or not isinstance(cc, str):
            _err(errors, f"layouts[{i}].cabinet_code is required")
        elif cc not in cabinet_codes:
            _err(errors, f"layouts[{i}].cabinet_code references unknown cabinet: {cc}")
        if not lk or not isinstance(lk, str):
            _err(errors, f"layouts[{i}].layout_key is required")
        if "revision" not in lay:
            _err(errors, f"layouts[{i}].revision is required")
        else:
            rev = lay.get("revision")
            if not isinstance(rev, int) or rev < 1:
                _err(errors, f"layouts[{i}].revision must be a positive integer")
        if cc and lk:
            key = (cc, lk)
            if key in layout_keys:
                _err(errors, f"duplicate layout for cabinet {cc} key {lk}")
            layout_keys.add(key)

    slots = doc.get("slots")
    if not isinstance(slots, list) or not slots:
        _err(errors, "slots must be a non-empty array")
        slots = []

    scope = doc.get("destructive_test_scope") or {}
    scope_cabinet = ""
    scope_range: tuple[int, int] | None = None
    if not isinstance(scope, dict):
        _err(errors, "destructive_test_scope must be an object")
    else:
        scope_cabinet = str(scope.get("cabinet") or "").strip()
        if not scope_cabinet:
            _err(errors, "destructive_test_scope.cabinet is required")
        elif scope_cabinet not in cabinet_codes:
            _err(errors, f"destructive_test_scope.cabinet unknown: {scope_cabinet}")
        scope_range = _parse_slot_index_range(str(scope.get("slot_indexes") or ""))
        if scope_range is None:
            _err(errors, "destructive_test_scope.slot_indexes must be like 1-10")

    slot_keys: set[str] = set()
    configured_scope_indexes: set[int] = set()
    layout_by_cabinet: dict[str, dict[str, Any]] = {}
    for lay in layouts:
        if isinstance(lay, dict) and lay.get("cabinet_code"):
            layout_by_cabinet[str(lay["cabinet_code"])] = lay

    for i, slot in enumerate(slots):
        if not isinstance(slot, dict):
            _err(errors, f"slots[{i}] must be an object")
            continue
        cc = slot.get("cabinet_code")
        sc = slot.get("slot_code")
        si = slot.get("slot_index")
        if not cc or not isinstance(cc, str):
            _err(errors, f"slots[{i}].cabinet_code is required")
            continue
        if cc not in cabinet_codes:
            _err(errors, f"slots[{i}].cabinet_code unknown: {cc}")
        if not sc or not isinstance(sc, str):
            _err(errors, f"slots[{i}].slot_code is required")
            continue
        if si is None or not isinstance(si, int):
            _err(errors, f"slots[{i}].slot_index is required integer")
            continue

        sk = _slot_key(cc, sc)
        if sk in slot_keys:
            _err(errors, f"duplicate slot key: {sk}")
        slot_keys.add(sk)

        enabled = bool(slot.get("enabled", True))
        sellable = bool(slot.get("sellable", False))
        if not enabled and sellable:
            _err(errors, f"disabled slot cannot be sellable: {sc}")

        destructive = bool(slot.get("destructive_test", False))
        if destructive and scope_range and scope_cabinet:
            if cc != scope_cabinet:
                _err(errors, f"destructive_test outside scope cabinet: {sc}")
            elif not (scope_range[0] <= si <= scope_range[1]):
                _err(errors, f"destructive_test outside scope indexes: {sc}")

        if cc == scope_cabinet and scope_range and scope_range[0] <= si <= scope_range[1]:
            configured_scope_indexes.add(si)

        if cc not in layout_by_cabinet:
            _err(errors, f"slots[{i}] cabinet {cc} has no layout entry")

    if scope_range and scope_cabinet:
        missing = [
            idx
            for idx in range(scope_range[0], scope_range[1] + 1)
            if idx not in configured_scope_indexes
        ]
        if missing:
            _err(
                errors,
                "destructive_test_scope includes unconfigured slot indexes: "
                + ",".join(str(x) for x in missing),
            )

    return errors


def validate_layout_file(path: str | Path) -> list[str]:
    p = Path(path)
    if not p.is_file():
        return [f"layout file not found: {p}"]
    try:
        doc = json.loads(p.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
        return [f"invalid JSON: {exc}"]
    return validate_layout(doc)


def assert_valid_layout(doc: dict[str, Any]) -> None:
    errors = validate_layout(doc)
    if errors:
        raise LayoutValidationError(errors)


def _load_json_file(path: str | Path) -> dict[str, Any]:
    p = Path(path)
    if not p.is_file():
        raise LayoutValidationError([f"file not found: {p}"])
    try:
        doc = json.loads(p.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
        raise LayoutValidationError([f"invalid JSON in {p}: {exc}"]) from exc
    if not isinstance(doc, dict):
        raise LayoutValidationError([f"{p} root must be a JSON object"])
    return doc


def _hardware_profile_value(hardware: dict[str, Any] | str) -> str:
    if isinstance(hardware, str):
        return hardware.strip()
    if isinstance(hardware, dict):
        profile = hardware.get("profile") or hardware.get("hardware_profile")
        if profile and isinstance(profile, str):
            return profile.strip()
    raise LayoutValidationError(["hardware profile must be a string or {profile: ...} object"])


def _infer_destructive_scope(slots: list[dict[str, Any]]) -> dict[str, str] | None:
    destructive = [
        s
        for s in slots
        if isinstance(s, dict) and s.get("destructive_test") and s.get("cabinet_code") and isinstance(s.get("slot_index"), int)
    ]
    if not destructive:
        return None
    cabinets = {str(s["cabinet_code"]) for s in destructive}
    if len(cabinets) != 1:
        return None
    cabinet = next(iter(cabinets))
    indexes = sorted(int(s["slot_index"]) for s in destructive)
    return {"cabinet": cabinet, "slot_indexes": f"{indexes[0]}-{indexes[-1]}"}


def merge_layout_manifest(
    machine_id: str,
    cabinet_layout: dict[str, Any],
    slot_assignments: dict[str, Any],
    inventory: dict[str, Any],
    payment_profile: dict[str, Any],
    hardware_profile: dict[str, Any] | str,
    destructive_scope: dict[str, Any] | None = None,
    catalog_defaults: dict[str, Any] | None = None,
    *,
    require_inventory_for_sellable: bool = True,
) -> dict[str, Any]:
    merge_errors: list[str] = []

    if not machine_id or not isinstance(machine_id, str):
        _err(merge_errors, "machine_id is required")

    cabinets = cabinet_layout.get("cabinets")
    layouts = cabinet_layout.get("layouts")
    if not isinstance(cabinets, list) or not cabinets:
        _err(merge_errors, "cabinet_layout.cabinets must be a non-empty array")
    if not isinstance(layouts, list) or not layouts:
        _err(merge_errors, "cabinet_layout.layouts must be a non-empty array")

    slots_raw = slot_assignments.get("slots")
    if not isinstance(slots_raw, list) or not slots_raw:
        _err(merge_errors, "slot_assignments.slots must be a non-empty array")

    inv_raw = inventory.get("inventory")
    if not isinstance(inv_raw, list):
        _err(merge_errors, "inventory.inventory must be an array")

    if merge_errors:
        raise LayoutValidationError(merge_errors)

    inv_map: dict[str, int] = {}
    for i, row in enumerate(inv_raw):
        if not isinstance(row, dict):
            _err(merge_errors, f"inventory[{i}] must be an object")
            continue
        cc = row.get("cabinet_code")
        sc = row.get("slot_code")
        qty = row.get("quantity")
        if not cc or not sc:
            _err(merge_errors, f"inventory[{i}] requires cabinet_code and slot_code")
            continue
        if qty is None or not isinstance(qty, int) or qty < 0:
            _err(merge_errors, f"inventory[{i}].quantity must be a non-negative integer")
            continue
        inv_map[_slot_key(str(cc), str(sc))] = qty

    if merge_errors:
        raise LayoutValidationError(merge_errors)

    merged_slots: list[dict[str, Any]] = []
    for i, slot in enumerate(slots_raw):
        if not isinstance(slot, dict):
            _err(merge_errors, f"slots[{i}] must be an object")
            continue
        cc = str(slot.get("cabinet_code") or "")
        sc = str(slot.get("slot_code") or "")
        sk = _slot_key(cc, sc)
        enabled = bool(slot.get("enabled", True))
        sellable = bool(slot.get("sellable", False))
        merged = dict(slot)
        if sk in inv_map:
            merged["inventory_quantity"] = inv_map[sk]
        elif "inventory_quantity" in slot:
            merged["inventory_quantity"] = slot["inventory_quantity"]
        elif enabled and sellable and require_inventory_for_sellable:
            _err(merge_errors, f"inventory missing for sellable slot: {sc}")
        elif enabled and sellable:
            merged["inventory_quantity"] = 0
        else:
            merged.setdefault("inventory_quantity", 0)
        merged_slots.append(merged)

    scope = destructive_scope
    if scope is None:
        scope = _infer_destructive_scope(merged_slots)
    if scope is None:
        _err(merge_errors, "destructive_test_scope is required (file or inferrable from slots)")
    elif not isinstance(scope, dict):
        _err(merge_errors, "destructive_test_scope must be an object")

    if merge_errors:
        raise LayoutValidationError(merge_errors)

    try:
        hw = _hardware_profile_value(hardware_profile)
    except LayoutValidationError:
        raise
    except Exception as exc:
        raise LayoutValidationError([f"hardware_profile: {exc}"]) from exc

    doc: dict[str, Any] = {
        "schema_version": 1,
        "machine_id": machine_id,
        "hardware_profile": hw,
        "payment_profile": dict(payment_profile),
        "destructive_test_scope": dict(scope),
        "cabinets": list(cabinets),
        "layouts": list(layouts),
        "slots": merged_slots,
    }
    if catalog_defaults:
        doc["catalog_defaults"] = dict(catalog_defaults)

    errors = validate_layout(doc)
    if errors:
        raise LayoutValidationError(errors)
    return doc


def merge_layout_from_files(
    machine_id: str,
    cabinet_layout_path: str | Path,
    slot_assignment_path: str | Path,
    inventory_path: str | Path,
    payment_profile_path: str | Path,
    hardware_profile_path: str | Path,
    destructive_scope_path: str | Path | None = None,
    catalog_defaults_path: str | Path | None = None,
) -> dict[str, Any]:
    destructive = _load_json_file(destructive_scope_path) if destructive_scope_path else None
    catalog = _load_json_file(catalog_defaults_path) if catalog_defaults_path else None
    return merge_layout_manifest(
        machine_id,
        _load_json_file(cabinet_layout_path),
        _load_json_file(slot_assignment_path),
        _load_json_file(inventory_path),
        _load_json_file(payment_profile_path),
        _load_json_file(hardware_profile_path),
        destructive_scope=destructive,
        catalog_defaults=catalog,
    )


def main() -> int:
    import argparse
    import sys

    # Backward compatible: `layout_config_schema.py path.json` == validate
    if len(sys.argv) >= 2 and sys.argv[1] not in ("validate", "merge", "-h", "--help"):
        errors = validate_layout_file(Path(sys.argv[1]))
        if errors:
            for e in errors:
                print(e, file=sys.stderr)
            return 2
        print("OK")
        return 0

    ap = argparse.ArgumentParser(description="Validate or merge machine layout JSON")
    sub = ap.add_subparsers(dest="command", required=True)

    validate_ap = sub.add_parser("validate", help="Validate unified layout JSON")
    validate_ap.add_argument("layout_json", type=Path)

    merge_ap = sub.add_parser("merge", help="Merge multi-file layout inputs into unified manifest")
    merge_ap.add_argument("--machine-id", required=True)
    merge_ap.add_argument("--cabinet", type=Path, required=True, help="Cabinet/layout JSON")
    merge_ap.add_argument("--slots", type=Path, required=True, help="Slot assignments JSON")
    merge_ap.add_argument("--inventory", type=Path, required=True, help="Inventory JSON")
    merge_ap.add_argument("--payment", type=Path, required=True, help="Payment profile JSON")
    merge_ap.add_argument("--hardware", type=Path, required=True, help="Hardware profile JSON")
    merge_ap.add_argument("--destructive", type=Path, default=None, help="Destructive scope JSON")
    merge_ap.add_argument("--catalog-defaults", type=Path, default=None, help="Catalog defaults JSON")
    merge_ap.add_argument("-o", "--output", type=Path, required=True, help="Write merged layout here")

    args = ap.parse_args()

    if args.command == "validate":
        errors = validate_layout_file(args.layout_json)
        if errors:
            for e in errors:
                print(e, file=sys.stderr)
            return 2
        print("OK")
        return 0

    try:
        merged = merge_layout_from_files(
            args.machine_id,
            args.cabinet,
            args.slots,
            args.inventory,
            args.payment,
            args.hardware,
            args.destructive,
            args.catalog_defaults,
        )
    except LayoutValidationError as exc:
        for e in exc.errors:
            print(e, file=sys.stderr)
        return 2

    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(merged, indent=2) + "\n", encoding="utf-8")
    print(f"OK merged -> {args.output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

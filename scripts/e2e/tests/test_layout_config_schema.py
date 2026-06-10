"""pytest tests for layout_config_schema."""
from __future__ import annotations

import copy
import json
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from layout_config_schema import (
    LayoutValidationError,
    assert_valid_layout,
    merge_layout_from_files,
    merge_layout_manifest,
    validate_layout,
    validate_layout_file,
)

EXAMPLES = Path(__file__).resolve().parent.parent / "examples"
SCRIPT_DIR = Path(__file__).resolve().parent.parent
PILOT_MACHINE_ID = "019e702c-11c6-7ab0-89c7-5eb32f0b12cb"
TWO_CAB_MACHINE_ID = "00000000-0000-4000-8000-000000000099"


def _slot_map(doc: dict) -> dict[str, dict]:
    return {s["slot_code"]: s for s in doc["slots"]}


def test_single_cabinet_layout_valid():
    errors = validate_layout_file(EXAMPLES / "machine-layout-single-cabinet-a1-a10.json")
    assert errors == []


def test_two_cabinet_layout_valid():
    errors = validate_layout_file(EXAMPLES / "machine-layout-two-cabinets.json")
    assert errors == []


def test_disabled_slot_not_sellable_fails():
    doc = json.loads((EXAMPLES / "machine-layout-two-cabinets.json").read_text(encoding="utf-8"))
    for slot in doc["slots"]:
        if slot.get("slot_code") == "A3":
            slot["sellable"] = True
            break
    errors = validate_layout(doc)
    assert any("disabled slot cannot be sellable" in e for e in errors)


def test_missing_layout_revision_fails():
    doc = json.loads((EXAMPLES / "machine-layout-single-cabinet-a1-a10.json").read_text(encoding="utf-8"))
    del doc["layouts"][0]["revision"]
    errors = validate_layout(doc)
    assert any("revision is required" in e for e in errors)


def test_duplicate_slot_key_fails():
    doc = json.loads((EXAMPLES / "machine-layout-single-cabinet-a1-a10.json").read_text(encoding="utf-8"))
    dup = copy.deepcopy(doc["slots"][0])
    doc["slots"].append(dup)
    errors = validate_layout(doc)
    assert any("duplicate slot key" in e for e in errors)


def test_destructive_scope_cannot_include_unconfigured_slot():
    doc = json.loads((EXAMPLES / "machine-layout-single-cabinet-a1-a10.json").read_text(encoding="utf-8"))
    doc["destructive_test_scope"]["slot_indexes"] = "1-12"
    errors = validate_layout(doc)
    assert any("unconfigured slot indexes" in e for e in errors)


def test_destructive_outside_scope_fails():
    doc = json.loads((EXAMPLES / "machine-layout-two-cabinets.json").read_text(encoding="utf-8"))
    doc["slots"][-1]["destructive_test"] = True
    errors = validate_layout(doc)
    assert any("destructive_test outside scope" in e for e in errors)


def test_assert_valid_layout_raises():
    with pytest.raises(LayoutValidationError):
        assert_valid_layout({"schema_version": 1})


def test_merge_pilot_multi_file_matches_unified_example():
    merged = merge_layout_from_files(
        PILOT_MACHINE_ID,
        EXAMPLES / "pilot-cabinet-layout-a.json",
        EXAMPLES / "pilot-slot-assignments-a1-a10.json",
        EXAMPLES / "pilot-inventory-a1-a10.json",
        EXAMPLES / "payment-profile-cash-only.json",
        EXAMPLES / "hardware-profile-tcn.json",
        EXAMPLES / "destructive-scope-a1-a10.json",
        EXAMPLES / "pilot-catalog-defaults.json",
    )
    expected = json.loads(
        (EXAMPLES / "machine-layout-single-cabinet-a1-a10.json").read_text(encoding="utf-8")
    )
    assert merged["machine_id"] == expected["machine_id"]
    assert merged["hardware_profile"] == expected["hardware_profile"]
    assert merged["payment_profile"] == expected["payment_profile"]
    assert merged["destructive_test_scope"] == expected["destructive_test_scope"]
    assert merged["catalog_defaults"] == expected["catalog_defaults"]
    assert merged["cabinets"] == expected["cabinets"]
    assert merged["layouts"] == expected["layouts"]
    merged_slots = _slot_map(merged)
    expected_slots = _slot_map(expected)
    assert set(merged_slots) == set(expected_slots)
    for code in expected_slots:
        assert merged_slots[code]["inventory_quantity"] == expected_slots[code]["inventory_quantity"]
        assert merged_slots[code]["product"]["sku"] == expected_slots[code]["product"]["sku"]


def test_merge_two_cabinet_multi_file_valid():
    merged = merge_layout_from_files(
        TWO_CAB_MACHINE_ID,
        EXAMPLES / "two-cabinet-layout.json",
        EXAMPLES / "two-cabinet-slot-assignments.json",
        EXAMPLES / "two-cabinet-inventory.json",
        EXAMPLES / "payment-profile-cash-only.json",
        EXAMPLES / "hardware-profile-tcn.json",
        EXAMPLES / "two-cabinet-destructive-scope.json",
        EXAMPLES / "two-cabinet-catalog-defaults.json",
    )
    assert validate_layout(merged) == []


def test_pilot_wrapper_paths_exist():
    names = [
        "setup-tcn-cash-products-a1-a10.ps1",
        "verify-tcn-cash-products-a1-a10.ps1",
        "pilot-cabinet-layout-a.json",
        "pilot-slot-assignments-a1-a10.json",
        "pilot-inventory-a1-a10.json",
        "destructive-scope-a1-a10.json",
    ]
    for name in names:
        assert (SCRIPT_DIR / name).is_file() or (EXAMPLES / name).is_file()


def test_inventory_missing_slot_fails():
    with pytest.raises(LayoutValidationError) as exc:
        merge_layout_manifest(
            PILOT_MACHINE_ID,
            json.loads((EXAMPLES / "pilot-cabinet-layout-a.json").read_text(encoding="utf-8")),
            json.loads((EXAMPLES / "pilot-slot-assignments-a1-a10.json").read_text(encoding="utf-8")),
            {"inventory": [{"cabinet_code": "A", "slot_code": "A1", "quantity": 10}]},
            {"mode": "cash_only"},
            {"profile": "TCN"},
            {"cabinet": "A", "slot_indexes": "1-10"},
        )
    assert any("inventory missing for sellable slot" in e for e in exc.value.errors)

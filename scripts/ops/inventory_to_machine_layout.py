#!/usr/bin/env python3
"""Convert legacy Mongo avm_product_inventory JSON to avf-vending-api machine layout JSON."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any

SCRIPT_DIR = Path(__file__).resolve().parent
DEFAULT_INPUT = SCRIPT_DIR / "fixtures" / "avf000132-mt01-layout1.inventory.json"
DEFAULT_OUTPUT = SCRIPT_DIR / "output" / "avf000132-machine-layout.json"
DEFAULT_CATALOG_MANIFEST = (
    SCRIPT_DIR.parent.parent.parent / "docs" / "avf_products_enriched_cloudinary_import_manifest.json"
)
DEFAULT_MACHINE_ID = "01a089ec-c7bb-7e0d-83a9-6f599f061f12"

GRID_COLS = 10
GRID_ROWS = 6
CABINET_CODE = "A"
LAYOUT_KEY = "grid-10x6"


def slot_index_to_code(slot_index: int, cols: int = GRID_COLS) -> str:
    row = (slot_index - 1) // cols
    col = (slot_index - 1) % cols + 1
    row_char = chr(ord("A") + row)
    return f"{row_char}{col}"


def normalize_is_combine(value: Any) -> bool:
    if isinstance(value, bool):
        return value
    if value is None:
        return False
    return str(value).strip().lower() in ("yes", "true", "1")


def is_active(doc: dict[str, Any]) -> bool:
    active = doc.get("is_active")
    if active is not None:
        return bool(int(active))
    status = doc.get("status")
    if status is None:
        return False
    return str(status).strip() in ("1", "true", "True")


def load_catalog_by_sku(manifest_path: Path) -> dict[str, dict[str, Any]]:
    if not manifest_path.is_file():
        return {}
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    out: dict[str, dict[str, Any]] = {}
    for product in manifest.get("products", []):
        sku = str(product.get("sku") or "").strip()
        if sku:
            out[sku] = product
    return out


def resolve_price_minor(doc: dict[str, Any], catalog_row: dict[str, Any] | None) -> int:
    inventory_price = int(doc.get("price", 0) or 0)
    if catalog_row:
        manifest_price = int(catalog_row.get("price_vnd", 0) or 0)
        if manifest_price > 0:
            return manifest_price
    return inventory_price


def resolve_image_url(catalog_row: dict[str, Any] | None) -> str:
    if not catalog_row:
        return ""
    return str(catalog_row.get("source_image_url") or "").strip()


def inventory_doc_to_layout_slot(
    doc: dict[str, Any],
    catalog_by_sku: dict[str, dict[str, Any]],
) -> dict[str, Any]:
    slot_index = int(doc["slot"])
    product_code = (doc.get("product_code") or "").strip()
    product_name = (doc.get("product_name") or "").strip()
    catalog_row = catalog_by_sku.get(product_code) if product_code else None
    active = is_active(doc)
    has_product = bool(product_code)
    sellable = has_product and active
    price_minor = resolve_price_minor(doc, catalog_row)
    image_url = resolve_image_url(catalog_row)

    slot: dict[str, Any] = {
        "cabinet_code": CABINET_CODE,
        "slot_code": slot_index_to_code(slot_index),
        "slot_index": slot_index,
        "enabled": active or has_product or bool(product_name),
        "sellable": sellable,
        "destructive_test": False,
        "price_minor": price_minor,
        "inventory_quantity": int(doc.get("inventory", doc.get("remaining", 0)) or 0),
        "media": {"policy": "external" if image_url else "placeholder", "url": image_url or None},
    }
    if slot["media"]["url"] is None:
        slot["media"].pop("url", None)

    if has_product:
        slot["product"] = {
            "sku": product_code,
            "name": product_name or str(catalog_row.get("name") or ""),
            "status": "active" if active else "inactive",
        }
        if image_url:
            slot["product"]["primary_image_url"] = image_url
        if price_minor > 0:
            slot["product"]["unit_price_minor"] = price_minor
    elif product_name:
        slot["product"] = {
            "sku": "",
            "name": product_name,
            "status": "inactive",
        }

    if normalize_is_combine(doc.get("is_combine")):
        slot["legacy_combine"] = {
            "is_combine": True,
            "slot_combine": int(doc.get("slot_combine", slot_index) or slot_index),
        }

    return slot


def build_catalog_products(
    inventory_docs: list[dict[str, Any]],
    catalog_by_sku: dict[str, dict[str, Any]],
) -> list[dict[str, Any]]:
    products: dict[str, dict[str, Any]] = {}
    for doc in inventory_docs:
        sku = (doc.get("product_code") or "").strip()
        if not sku:
            continue
        catalog_row = catalog_by_sku.get(sku, {})
        name = (doc.get("product_name") or catalog_row.get("name") or sku).strip()
        price_minor = resolve_price_minor(doc, catalog_row or None)
        image_url = resolve_image_url(catalog_row or None)
        existing = products.get(sku)
        if existing and existing.get("unit_price_minor") and not price_minor:
            price_minor = int(existing["unit_price_minor"])
        products[sku] = {
            "sku": sku,
            "name": name,
            "unit_price_minor": price_minor,
            "primary_image_url": image_url,
            "category_slug": catalog_row.get("category_slug"),
            "brand_slug": catalog_row.get("brand_slug"),
        }
    return [products[sku] for sku in sorted(products)]


def build_layout_document(
    inventory_docs: list[dict[str, Any]],
    machine_id: str,
    catalog_by_sku: dict[str, dict[str, Any]],
) -> dict[str, Any]:
    slots = [
        inventory_doc_to_layout_slot(doc, catalog_by_sku)
        for doc in sorted(inventory_docs, key=lambda d: int(d["slot"]))
    ]
    catalog_products = build_catalog_products(inventory_docs, catalog_by_sku)
    return {
        "schema_version": 1,
        "machine_id": machine_id,
        "hardware_profile": "TCN",
        "payment_profile": {"mode": "cash_only"},
        "destructive_test_scope": {
            "cabinet": CABINET_CODE,
            "slot_indexes": "1-60",
        },
        "catalog_defaults": {
            "category_slug": "avf-beverages",
            "brand_slug": "avf",
            "currency": "VND",
        },
        "cabinets": [
            {
                "code": CABINET_CODE,
                "title": "Cabinet A",
                "sort_order": 1,
                "metadata": {
                    "board_protocol": "Tcn",
                    "bill_protocol": "Ict_BC_V1",
                    "cash_topology": "DIRECT_BILL",
                    "transport_type": "RS485",
                },
            }
        ],
        "layouts": [
            {
                "cabinet_code": CABINET_CODE,
                "layout_key": LAYOUT_KEY,
                "revision": 1,
                "layout_spec": {"rows": GRID_ROWS, "cols": GRID_COLS},
                "status": "published",
            }
        ],
        "slots": slots,
        "catalog_products": catalog_products,
        "merge_pairs": build_merge_pairs(inventory_docs),
    }


def build_merge_pairs(inventory_docs: list[dict[str, Any]]) -> list[dict[str, str]]:
    pairs: list[dict[str, str]] = []
    for doc in inventory_docs:
        if not normalize_is_combine(doc.get("is_combine")):
            continue
        left_index = int(doc["slot"])
        right_index = left_index + 1
        if right_index > 60:
            continue
        pairs.append(
            {
                "left_slot_code": slot_index_to_code(left_index),
                "right_slot_code": slot_index_to_code(right_index),
            }
        )
    return pairs


def validate_spot_checks(layout: dict[str, Any]) -> None:
    by_index = {slot["slot_index"]: slot for slot in layout["slots"]}
    assert by_index[1]["product"]["sku"] == "1702"
    assert by_index[1]["slot_code"] == "A1"
    assert "product" not in by_index[2] or not by_index[2].get("sellable")
    assert by_index[3]["product"]["sku"] == "1702"
    assert by_index[3]["slot_code"] == "A3"
    assert by_index[10]["product"]["name"] == "Not have product"
    assert by_index[1]["price_minor"] == 10000
    assert by_index[1]["product"].get("primary_image_url", "").startswith("https://")
    catalog = {row["sku"]: row for row in layout.get("catalog_products", [])}
    assert catalog["1702"]["unit_price_minor"] == 10000
    assert catalog["1702"]["primary_image_url"].startswith("https://")
    merge_codes = {(p["left_slot_code"], p["right_slot_code"]) for p in layout["merge_pairs"]}
    assert ("A1", "A2") in merge_codes
    assert ("A3", "A4") in merge_codes
    assert ("A9", "A10") in merge_codes


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", type=Path, default=DEFAULT_INPUT)
    parser.add_argument("--output", type=Path, default=DEFAULT_OUTPUT)
    parser.add_argument("--machine-id", default=DEFAULT_MACHINE_ID)
    parser.add_argument("--catalog-manifest", type=Path, default=DEFAULT_CATALOG_MANIFEST)
    args = parser.parse_args()

    if not args.input.is_file():
        print(f"Input not found: {args.input}", file=sys.stderr)
        return 1

    docs = json.loads(args.input.read_text(encoding="utf-8"))
    if not isinstance(docs, list) or len(docs) != 60:
        print("Input must be an array of 60 inventory documents", file=sys.stderr)
        return 1

    catalog_by_sku = load_catalog_by_sku(args.catalog_manifest)
    if not catalog_by_sku:
        print(f"WARN: catalog manifest not loaded: {args.catalog_manifest}", file=sys.stderr)
    layout = build_layout_document(docs, args.machine_id, catalog_by_sku)
    validate_spot_checks(layout)

    schema_path = SCRIPT_DIR.parent / "e2e" / "layout_config_schema.py"
    if schema_path.is_file():
        import importlib.util

        spec = importlib.util.spec_from_file_location("layout_config_schema", schema_path)
        module = importlib.util.module_from_spec(spec)
        assert spec.loader is not None
        spec.loader.exec_module(module)
        module.assert_valid_layout(layout)

    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(layout, ensure_ascii=False, indent=2), encoding="utf-8")

    print(f"Wrote {args.output}")
    print(f"slot A1 (index 1): {by_index_summary(layout, 1)}")
    print(f"slot A2 (index 2): {by_index_summary(layout, 2)}")
    print(f"slot A3 (index 3): {by_index_summary(layout, 3)}")
    print(f"slot A10 (index 10): {by_index_summary(layout, 10)}")
    print(f"merge_pairs: {len(layout['merge_pairs'])}")
    print(f"catalog_products: {len(layout.get('catalog_products', []))}")
    return 0


def by_index_summary(layout: dict[str, Any], slot_index: int) -> str:
    slot = next(s for s in layout["slots"] if s["slot_index"] == slot_index)
    product = slot.get("product") or {}
    sku = product.get("sku", "")
    name = product.get("name", "")
    return f"code={slot['slot_code']} sku={sku!r} sellable={slot['sellable']} name={name[:24]!r}"


if __name__ == "__main__":
    raise SystemExit(main())

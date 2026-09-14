#!/usr/bin/env python3
"""Apply auto-approval rules and emit catalog-import-final.json."""
from __future__ import annotations

import argparse
import json
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


def approve_brand(product: dict[str, Any]) -> dict[str, Any]:
    slug = product.get("brand_slug")
    conf = product.get("brand_confidence", "low")
    needs = product.get("needs_brand_review", False)
    if slug and conf in ("high", "medium") and not needs:
        return {"slug": slug, "final_decision": slug, "review_status": "approved", "approved": True}
    if slug and conf == "high":
        return {"slug": slug, "final_decision": slug, "review_status": "approved", "approved": True}
    if slug is None:
        return {"slug": None, "final_decision": None, "review_status": "approved_null", "approved": True}
    if conf == "medium" and slug:
        return {"slug": slug, "final_decision": slug, "review_status": "approved_medium", "approved": True}
    # low confidence with slug — approve slug if not generic evidence
    evidence = product.get("brand_evidence", "")
    if "generic" in evidence.lower():
        return {"slug": None, "final_decision": None, "review_status": "approved_null", "approved": True}
    if slug:
        return {"slug": slug, "final_decision": slug, "review_status": "approved_inferred", "approved": True}
    return {"slug": None, "final_decision": None, "review_status": "approved_null", "approved": True}


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--manifest",
        default="../../../docs/avf_products_enriched_cloudinary_import_manifest.json",
    )
    parser.add_argument(
        "--image-validation",
        default="../../.catalog-bootstrap-evidence/07-source-image-validation.json",
    )
    parser.add_argument("--evidence-dir", default="../../.catalog-bootstrap-evidence")
    parser.add_argument("--output", default="../../../docs/catalog-import-final.json")
    args = parser.parse_args()
    base = Path(__file__).resolve().parent
    manifest = json.loads((base / args.manifest).read_text(encoding="utf-8"))
    img_val_path = base / args.image_validation
    img_by_sku: dict[str, dict[str, Any]] = {}
    if img_val_path.exists():
        img_report = json.loads(img_val_path.read_text(encoding="utf-8"))
        for item in img_report.get("items", []):
            img_by_sku[item["sku"]] = item

    final_products = []
    for p in manifest.get("products", []):
        brand = approve_brand(p)
        img = img_by_sku.get(p["sku"], {})
        final_products.append(
            {
                "sku": p["sku"],
                "name": p["name"],
                "description": p.get("description"),
                "active": p.get("active", True),
                "category": {"slug": p["category_slug"], "final_decision": p["category_slug"]},
                "brand": brand,
                "tags": {"final_list": p.get("tag_slugs", [])},
                "pricing": {
                    "source_vnd": p.get("price_vnd"),
                    "unit_price_minor": p.get("price_vnd"),
                },
                "source_image": {
                    "url": p.get("source_image_url"),
                    "sha256": img.get("sha256"),
                    "mime": img.get("mime_type"),
                    "validation_status": img.get("status", "PENDING"),
                },
                "cloudinary": {"planned_public_id": p.get("cloudinary", {}).get("public_id")},
            }
        )

    out_doc = {
        "finalized_at": datetime.now(timezone.utc).isoformat(),
        "taxonomy": manifest.get("taxonomy"),
        "products": final_products,
    }
    evidence_dir = base / args.evidence_dir
    evidence_dir.mkdir(parents=True, exist_ok=True)
    output = base / args.output
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(out_doc, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    (evidence_dir / "08-final-reviewed-manifest.json").write_text(
        json.dumps(out_doc, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    pending = sum(1 for p in final_products if p["source_image"].get("validation_status") not in ("OK", "PENDING"))
    unapproved = sum(1 for p in final_products if not p["brand"].get("approved"))
    print(json.dumps({"products": len(final_products), "brand_unapproved": unapproved, "image_issues": pending}, indent=2))


if __name__ == "__main__":
    main()

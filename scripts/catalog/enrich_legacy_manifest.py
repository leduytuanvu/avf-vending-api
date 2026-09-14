#!/usr/bin/env python3
"""Enrich legacy Mongo product export into catalog import manifest."""
from __future__ import annotations

import argparse
import json
import re
import unicodedata
from collections import Counter, defaultdict
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

# (substring in name lower, brand_slug, brand_name, confidence)
BRAND_PATTERNS: list[tuple[str, str, str, str]] = [
    ("highlands coffee", "highlands-coffee", "Highlands Coffee", "high"),
    ("pocari sweat", "pocari", "Pocari Sweat", "high"),
    ("wake up 247", "wake-up-247", "Wake up 247", "high"),
    ("coca-cola", "coca-cola", "Coca-Cola", "high"),
    ("coca cola", "coca-cola", "Coca-Cola", "high"),
    ("redbull", "red-bull", "Red Bull", "high"),
    ("red bull", "red-bull", "Red Bull", "high"),
    ("7up revive", "7up", "7UP", "high"),
    ("7up", "7up", "7UP", "high"),
    ("hao hảo", "hao-hao", "Hảo Hảo", "high"),
    ("hảo hảo", "hao-hao", "Hảo Hảo", "high"),
    ("highlands", "highlands-coffee", "Highlands Coffee", "high"),
    ("wonderfarm", "wonderfarm", "Wonderfarm", "high"),
    ("lothamilk", "lothamilk", "Lothamilk", "high"),
    ("highlands coffee", "highlands-coffee", "Highlands Coffee", "high"),
    ("koh-kae", "koh-kae", "Koh-Kae", "high"),
    ("koh kae", "koh-kae", "Koh-Kae", "high"),
    ("star kombucha", "star-kombucha", "Star Kombucha", "high"),
    ("lipovitan", "lipovitan", "Lipovitan", "high"),
    ("cream-o", "cream-o", "Cream-O", "high"),
    ("good mood", "good-mood", "Good Mood", "high"),
    ("juicy milk", "juicy", "Juicy", "high"),
    ("tea plus", "tea-plus", "Tea Plus", "high"),
    ("ô long tea plus", "tea-plus", "Tea Plus", "high"),
    ("olong tea plus", "tea-plus", "Tea Plus", "high"),
    ("boncha", "boncha", "Boncha", "high"),
    ("oatside", "oatside", "Oatside", "high"),
    ("cocoxim", "cocoxim", "Cocoxim", "high"),
    ("aquafina", "aquafina", "Aquafina", "high"),
    ("dasani", "dasani", "Dasani", "high"),
    ("aquarius", "aquarius", "Aquarius", "high"),
    ("monster", "monster", "Monster", "high"),
    ("cozy", "cozy", "Cozy", "high"),
    ("lays", "lays", "Lays", "high"),
    ("sting", "sting", "Sting", "high"),
    ("twister", "twister", "Twister", "high"),
    ("pepsi", "pepsi", "Pepsi", "high"),
    ("rockstar", "rockstar", "Rockstar", "high"),
    ("mirinda", "mirinda", "Mirinda", "high"),
    ("warrior", "warrior", "Warrior", "high"),
    ("kokomi", "kokomi", "Kokomi", "high"),
    ("omachi", "omachi", "Omachi", "high"),
    ("tenro", "tenro", "TENRO", "high"),
    ("davis", "davis", "Davis", "high"),
    ("davi's", "davis", "Davis", "high"),
    ("yuna", "yuna", "Yuna", "high"),
    ("tingco", "tingco", "Tingco", "high"),
    ("kool", "kool", "KOOL", "high"),
    ("bauli", "bauli", "Bauli", "high"),
    ("phaner", "phaner", "Phaner", "high"),
    ("otto", "otto", "Otto", "high"),
    ("manuka", "manuka", "Manuka", "high"),
    ("bidrico", "bidrico", "Bidrico", "high"),
    ("a nuta", "a-nuta", "A Nuta", "high"),
    ("restore", "restore", "Restore", "high"),
    ("adew", "adew", "Adew", "high"),
    ("teaa", "teaa", "TeaA", "high"),
    ("lai phú", "lai-phu", "Lai Phú", "high"),
    ("boost", "boost", "Boost", "high"),
    ("lavie", "lavie", "Lavie", "high"),
    ("vivant", "vivant", "Vivant", "high"),
    ("vĩnh hảo", "vinh-hao", "Vĩnh Hảo", "high"),
    ("vinh hao", "vinh-hao", "Vĩnh Hảo", "high"),
    ("lemona", "lemona", "Lemona", "high"),
    ("kirin imuse", "kirin", "Kirin", "high"),
    ("kirin", "kirin", "Kirin", "high"),
    ("c2 ", "c2", "C2", "high"),
    ("c2", "c2", "C2", "medium"),
    ("modern", "modern", "Modern", "medium"),
    ("boss lon", "boss", "Boss", "medium"),
    ("boss", "boss", "Boss", "medium"),
    ("latte", "latte", "Latte", "medium"),
    ("ice ", "ice-plus", "Ice+", "medium"),
    ("solo", "solo", "Solo", "medium"),
    ("lof", "lof", "LOF", "medium"),
    ("ion - green", "ion-green", "ION Green", "medium"),
    ("i-on", "ion-green", "ION Green", "medium"),
    ("kombucha", "star-kombucha", "Star Kombucha", "medium"),
]

GENERIC_PREFIXES = {
    "nuoc", "nước", "tra", "trà", "banh", "bánh", "mi", "mì", "sua", "sữa",
    "snack", "combo", "chan", "soda",
}

NULL_BRAND_SUBSTRINGS = [
    "bánh que mix",
    "snack tôm",
    "snack bí",
    "chân gà",
    "combo xxtt",
]

PARENT_CATEGORIES = [
    {"slug": "beverage", "name": "Đồ uống", "parent_slug": None},
    {"slug": "food", "name": "Thực phẩm", "parent_slug": None},
]

CHILD_CATEGORIES = [
    {"slug": "soft-drinks", "name": "Nước ngọt", "parent_slug": "beverage"},
    {"slug": "tea", "name": "Trà", "parent_slug": "beverage"},
    {"slug": "energy-drinks", "name": "Nước tăng lực", "parent_slug": "beverage"},
    {"slug": "water", "name": "Nước suối / khoáng", "parent_slug": "beverage"},
    {"slug": "juice-smoothie", "name": "Nước ép / sinh tố", "parent_slug": "beverage"},
    {"slug": "milk-dairy", "name": "Sữa / cà phê sữa", "parent_slug": "beverage"},
    {"slug": "functional-drinks", "name": "Đồ uống chức năng", "parent_slug": "beverage"},
    {"slug": "instant-noodles", "name": "Mì ăn liền", "parent_slug": "food"},
    {"slug": "snacks", "name": "Snack", "parent_slug": "food"},
    {"slug": "bakery", "name": "Bánh", "parent_slug": "food"},
    {"slug": "other-food", "name": "Thực phẩm khác", "parent_slug": "food"},
]

TAG_DEFS = [
    ("beverage", "Đồ uống"),
    ("food", "Thực phẩm"),
    ("chai", "Chai"),
    ("lon", "Lon"),
    ("hop", "Hộp"),
    ("goi", "Gói"),
    ("ly-to", "Ly"),
    ("tea", "Trà"),
    ("coffee", "Cà phê"),
    ("soft-drink", "Nước ngọt"),
    ("energy-drink", "Nước tăng lực"),
    ("water", "Nước"),
    ("juice", "Nước ép"),
    ("milk", "Sữa"),
    ("functional-drink", "Đồ uống chức năng"),
    ("instant-noodles", "Mì ăn liền"),
    ("snack", "Snack"),
    ("bakery", "Bánh"),
]

CHILD_CATEGORY_TAG = {
    "soft-drinks": "soft-drink",
    "tea": "tea",
    "energy-drinks": "energy-drink",
    "water": "water",
    "juice-smoothie": "juice",
    "milk-dairy": "milk",
    "functional-drinks": "functional-drink",
    "instant-noodles": "instant-noodles",
    "snacks": "snack",
    "bakery": "bakery",
    "other-food": "snack",
}


def slugify(text: str) -> str:
    text = unicodedata.normalize("NFKD", text).encode("ascii", "ignore").decode().lower()
    text = re.sub(r"[^a-z0-9]+", "-", text)
    return re.sub(r"-+", "-", text).strip("-")


def infer_child_category(name: str, legacy_cat: int) -> str:
    n = name.lower()
    if legacy_cat == 2:
        if any(k in n for k in ("mì", "mi ly", "mi kokomi", "omachi", "modern", "mì ly")):
            return "instant-noodles"
        if any(k in n for k in ("lays", "snack", "bánh que", "đậu phộng", "kẹo")):
            return "snacks"
        if any(k in n for k in ("bánh solo", "bauli", "otto", "phaner", "cookie", "sừng bò", "bánh mì")):
            return "bakery"
        return "other-food"
    if any(k in n for k in ("coca", "pepsi", "7up", "sprite", "fanta", "mirinda", "sting", "twister")):
        return "soft-drinks"
    if any(k in n for k in ("trà", "tea", "c2")):
        return "tea"
    if any(k in n for k in ("monster", "rockstar", "warrior", "redbull", "red bull", "wake up", "lipovitan", "boost")):
        return "energy-drinks"
    if any(k in n for k in ("aquafina", "lavie", "dasani", "vivant", "vĩnh hảo", "khoáng", "revive")):
        return "water"
    if any(k in n for k in ("nước ép", "juicy")):
        return "juice-smoothie"
    if any(k in n for k in ("sữa", "latte", "milk", "lothamilk", "oatside", "lof", "boss", "cà phê")):
        return "milk-dairy"
    if any(
        k in n
        for k in (
            "yến", "wonderfarm", "kirin", "davis", "tingco", "bidrico", "kombucha",
            "pocari", "restore", "ion", "adew", "teaa", "manuka", "cocoxim", "soda",
        )
    ):
        return "functional-drinks"
    return "soft-drinks"


def infer_brand(name: str, brief: str | None) -> dict[str, Any]:
    n = name.lower()
    for sub in NULL_BRAND_SUBSTRINGS:
        if sub in n:
            return {
                "brand_slug": None,
                "brand_name": None,
                "brand_confidence": "high",
                "needs_review": False,
                "evidence": "generic product line without distinct brand",
            }
    for pattern, slug, bname, conf in sorted(BRAND_PATTERNS, key=lambda x: -len(x[0])):
        if pattern in n:
            return {
                "brand_slug": slug,
                "brand_name": bname,
                "brand_confidence": conf,
                "needs_review": conf == "medium",
                "evidence": f"name contains '{pattern}'",
            }
    if brief and str(brief).strip():
        b = str(brief).strip()
        return {
            "brand_slug": slugify(b),
            "brand_name": b,
            "brand_confidence": "medium",
            "needs_review": True,
            "evidence": "legacy brief field",
        }
    first = name.split()[0] if name else ""
    if first.lower() in GENERIC_PREFIXES:
        return {
            "brand_slug": None,
            "brand_name": None,
            "brand_confidence": "low",
            "needs_review": True,
            "evidence": f"generic first token '{first}'",
        }
    return {
        "brand_slug": slugify(first),
        "brand_name": first,
        "brand_confidence": "low",
        "needs_review": True,
        "evidence": "first token of name",
    }


def infer_tags(name: str, legacy_cat: int, child_cat: str) -> list[str]:
    n = name.lower()
    tags: set[str] = set()
    tags.add("beverage" if legacy_cat == 1 else "food")
    mapped = CHILD_CATEGORY_TAG.get(child_cat)
    if mapped:
        tags.add(mapped)
    if "lon" in n:
        tags.add("lon")
    if "chai" in n or "pet" in n:
        tags.add("chai")
    if "ly" in n:
        tags.add("ly-to")
    if "hộp" in n or "hop" in n:
        tags.add("hop")
    if "gói" in n or "goi" in n:
        tags.add("goi")
    if "cà phê" in n or "cafe" in n:
        tags.add("coffee")
    if "trà" in n or "tea" in n:
        tags.add("tea")
    return sorted(tags)


def load_legacy(path: Path) -> list[dict[str, Any]]:
    data = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(data, list):
        raise ValueError("legacy file must be a JSON array")
    return data


def enrich(legacy_path: Path) -> dict[str, Any]:
    legacy = load_legacy(legacy_path)
    products_out: list[dict[str, Any]] = []
    brand_review: list[dict[str, Any]] = []
    brands_map: dict[str, str] = {}

    for row in legacy:
        sku = str(row.get("code", "")).strip()
        name = str(row.get("name", "")).strip()
        legacy_cat = int(row.get("category", 0))
        brief = row.get("brief")
        child_cat = infer_child_category(name, legacy_cat)
        parent_cat = "beverage" if legacy_cat == 1 else "food"
        brand = infer_brand(name, brief)
        if brand["brand_slug"]:
            brands_map[brand["brand_slug"]] = brand["brand_name"] or brand["brand_slug"]
        tag_slugs = infer_tags(name, legacy_cat, child_cat)
        public_id = f"import-{sku}"
        product = {
            "sku": sku,
            "name": name,
            "description": (str(brief).strip() if brief else name),
            "active": row.get("status", 1) == 1,
            "legacy_id": row.get("id"),
            "legacy_category": legacy_cat,
            "parent_category_slug": parent_cat,
            "category_slug": child_cat,
            "brand_slug": brand["brand_slug"],
            "brand_name": brand["brand_name"],
            "brand_confidence": brand["brand_confidence"],
            "needs_brand_review": brand["needs_review"],
            "brand_evidence": brand["evidence"],
            "tag_slugs": tag_slugs,
            "price_vnd": int(row.get("price", 0)),
            "source_image_url": str(row.get("image_url", "")).strip(),
            "cloudinary": {
                "public_id": public_id,
                "folder": "avf-vending/products",
                "resource_type": "image",
                "type": "upload",
                "secure_url": None,
                "asset_id": None,
            },
        }
        products_out.append(product)
        if brand["needs_review"]:
            brand_review.append(
                {
                    "sku": sku,
                    "product_name": name,
                    "proposed_brand": brand["brand_name"],
                    "proposed_brand_slug": brand["brand_slug"],
                    "confidence": brand["brand_confidence"],
                    "evidence": brand["evidence"],
                    "recommended": "MANUAL_REVIEW" if brand["brand_slug"] else "NULL_BRAND",
                }
            )

    brands = [{"slug": s, "name": n, "active": True} for s, n in sorted(brands_map.items())]
    categories = PARENT_CATEGORIES + CHILD_CATEGORIES
    tags = [{"slug": s, "name": n, "active": True} for s, n in TAG_DEFS]

    skus = [p["sku"] for p in products_out]
    names = [p["name"] for p in products_out]
    pub_ids = [p["cloudinary"]["public_id"] for p in products_out]
    imgs = [p["source_image_url"] for p in products_out]

    summary = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "source_file": str(legacy_path),
        "PRODUCT_COUNT": len(products_out),
        "UNIQUE_SKU_COUNT": len(set(skus)),
        "CATEGORY_COUNT": len(categories),
        "BRAND_COUNT": len(brands),
        "TAG_COUNT": len(tags),
        "SOURCE_IMAGE_URL_COUNT": sum(1 for i in imgs if i),
        "UNIQUE_SOURCE_IMAGE_URL_COUNT": len(set(i for i in imgs if i)),
        "PRODUCTS_WITHOUT_SOURCE_IMAGE": sum(1 for i in imgs if not i),
        "PRODUCTS_WITHOUT_CATEGORY": sum(1 for p in products_out if not p["category_slug"]),
        "PRODUCTS_WITHOUT_BRAND": sum(1 for p in products_out if not p["brand_slug"]),
        "PRODUCTS_WITHOUT_TAGS": sum(1 for p in products_out if not p["tag_slugs"]),
        "PRODUCTS_WITHOUT_PRICE": sum(1 for p in products_out if not p["price_vnd"]),
        "DUPLICATE_SKUS": [k for k, v in Counter(skus).items() if v > 1],
        "DUPLICATE_PRODUCT_NAMES": [k for k, v in Counter(names).items() if v > 1],
        "DUPLICATE_CLOUDINARY_PUBLIC_IDS": [k for k, v in Counter(pub_ids).items() if v > 1],
        "BRANDS_MARKED_NEEDS_REVIEW": sum(1 for p in products_out if p["needs_brand_review"]),
    }

    return {
        "manifest_version": 1,
        "generated_at": summary["generated_at"],
        "source": {"legacy_file": str(legacy_path), "legacy_format": "mongodb_export"},
        "summary": summary,
        "taxonomy": {"categories": categories, "brands": brands, "tags": tags},
        "products": products_out,
        "brand_review": brand_review,
    }


def main() -> None:
    parser = argparse.ArgumentParser(description="Enrich legacy AVF product export")
    parser.add_argument(
        "--input",
        default="../../../docs/avf_vending_machine_prod.avm_product.json",
        help="Legacy Mongo JSON array",
    )
    parser.add_argument(
        "--output",
        default="../../../docs/avf_products_enriched_cloudinary_import_manifest.json",
    )
    parser.add_argument(
        "--evidence-dir",
        default="../../.catalog-bootstrap-evidence",
    )
    args = parser.parse_args()
    input_path = Path(__file__).resolve().parent / args.input
    output_path = Path(__file__).resolve().parent / args.output
    evidence_dir = Path(__file__).resolve().parent / args.evidence_dir
    evidence_dir.mkdir(parents=True, exist_ok=True)

    manifest = enrich(input_path)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

    (evidence_dir / "02-input-manifest-summary.json").write_text(
        json.dumps(manifest["summary"], ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    (evidence_dir / "05-brand-review.json").write_text(
        json.dumps(
            {"items": manifest["brand_review"], "count": len(manifest["brand_review"])},
            ensure_ascii=False,
            indent=2,
        )
        + "\n",
        encoding="utf-8",
    )
    print(json.dumps(manifest["summary"], ensure_ascii=False, indent=2))
    print(f"Wrote {output_path}")


if __name__ == "__main__":
    main()

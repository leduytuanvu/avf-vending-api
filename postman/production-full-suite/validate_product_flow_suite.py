#!/usr/bin/env python3
"""Validate production-full-suite Postman product flow configuration."""
from __future__ import annotations

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent
COLLECTION = ROOT / "avf-production-full.postman_collection.json"
ENVIRONMENT = ROOT / "avf-production.postman_environment.json"
SAMPLE_PNG = ROOT / "assets" / "sample-product.png"

IMAGE_REQ_NAME = "[GATED-WRITE] POST /v1/admin/product-images (Cloudinary multipart)"
PRODUCT_REQ_NAME = "[GATED-WRITE] POST /v1/admin/products"

REQUIRED_ENV = {
    "baseUrl",
    "adminEmail",
    "adminPassword",
    "allowGatedWrites",
    "confirmProductionWrites",
    "allow_destructive",
    "canaryMode",
    "readiness",
    "_runtimeRequestId",
    "_runtimeCorrelationId",
    "_runtimeIdempotencyKey",
    "primaryMediaId",
    "primaryMediaUrl",
    "primaryMediaThumbnailUrl",
    "categoryId",
    "brandId",
    "tagId",
    "productId",
    "machineId",
    "unitPriceMinor",
}


def fail(msg: str) -> None:
    print(f"FAIL: {msg}")
    sys.exit(1)


def ok(msg: str) -> None:
    print(f"OK: {msg}")


def walk_items(items: list, path: str = "") -> list[tuple[str, dict]]:
    out: list[tuple[str, dict]] = []
    names_at_level: dict[str, int] = {}
    for item in items:
        name = item.get("name", "")
        full = f"{path}/{name}" if path else name
        if name:
            names_at_level[name] = names_at_level.get(name, 0) + 1
        if "request" in item:
            out.append((full, item))
        if "item" in item:
            out.extend(walk_items(item["item"], full))
    for name, count in names_at_level.items():
        if count > 1 and "product" in name.lower():
            fail(f"duplicate folder/request name at level: {name} ({count}x)")
    return out


def find_request(items: list, name: str) -> dict | None:
    for _, item in walk_items(items):
        if item.get("name") == name:
            return item
    return None


def script_text(item: dict, listen: str) -> str:
    for ev in item.get("event", []):
        if ev.get("listen") == listen:
            exec_lines = ev.get("script", {}).get("exec", [])
            return "\n".join(exec_lines)
    return ""


def has_content_type_header(headers: list) -> bool:
    for h in headers:
        key = h.get("key", "")
        if key.lower() == "content-type":
            return True
    return False


def main() -> None:
    errors: list[str] = []

    for path in (COLLECTION, ENVIRONMENT):
        try:
            json.loads(path.read_text(encoding="utf-8"))
            ok(f"{path.name} parses as JSON")
        except json.JSONDecodeError as e:
            fail(f"{path.name} invalid JSON: {e}")

    col = json.loads(COLLECTION.read_text(encoding="utf-8"))
    env = json.loads(ENVIRONMENT.read_text(encoding="utf-8"))

    env_keys = {v["key"] for v in env.get("values", [])}
    missing = REQUIRED_ENV - env_keys
    if missing:
        errors.append(f"environment missing keys: {sorted(missing)}")
    else:
        ok("environment has gated-write and product flow variables")

    gated = {v["key"]: v.get("value") for v in env["values"]}
    if gated.get("allowGatedWrites") != "true":
        errors.append("allowGatedWrites must be true")
    if gated.get("confirmProductionWrites") != "I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION":
        errors.append("confirmProductionWrites must be I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION")

    image = find_request(col["item"], IMAGE_REQ_NAME)
    if not image:
        fail(f"missing request: {IMAGE_REQ_NAME}")

    product = find_request(col["item"], PRODUCT_REQ_NAME)
    if not product:
        fail(f"missing request: {PRODUCT_REQ_NAME}")

    img_req = image["request"]
    body = img_req.get("body", {})
    if body.get("mode") != "formdata":
        errors.append("product-images body.mode must be formdata")
    else:
        ok("product-images uses formdata body")

    if has_content_type_header(img_req.get("header", [])):
        errors.append("product-images must not have manual Content-Type header")
    else:
        ok("product-images has no manual Content-Type")

    form = {f.get("key"): f for f in body.get("formdata", [])}
    for key in ("file", "purpose", "altText"):
        if key not in form:
            errors.append(f"product-images missing form field: {key}")
    if form.get("file", {}).get("type") != "file":
        errors.append("product-images file field must be type=file")
    if form.get("purpose", {}).get("value") != "product_image":
        errors.append("product-images purpose must be product_image")
    if not errors:
        ok("product-images form fields file/purpose/altText are correct")

    img_pre = script_text(image, "prerequest")
    if "Content-Type" in img_pre or "content-type" in img_pre.lower():
        errors.append("product-images pre-request must not set Content-Type")
    if "allowGatedWrites" not in img_pre or "confirmProductionWrites" not in img_pre:
        errors.append("product-images pre-request missing gated-write guard")
    if "_runtimeRequestId" not in img_pre:
        errors.append("product-images pre-request must refresh runtime IDs")
    else:
        ok("product-images pre-request has gated-write guard and runtime IDs")

    img_test = script_text(image, "test")
    if "primaryMediaId" not in img_test:
        errors.append("product-images test must set primaryMediaId")
    else:
        ok("product-images test sets primaryMediaId")

    prod_req = product["request"]
    raw = prod_req.get("body", {}).get("raw", "")
    if raw.strip() != "{{_runtimeProductCreateBody}}":
        errors.append(f"product create raw body must be {{{{_runtimeProductCreateBody}}}}, got: {raw[:80]!r}")
    else:
        ok("product create uses {{_runtimeProductCreateBody}}")

    prod_pre = script_text(product, "prerequest")
    if "JSON.stringify" not in prod_pre or "_runtimeProductCreateBody" not in prod_pre:
        errors.append("product create pre-request must JSON.stringify DTO into _runtimeProductCreateBody")
    if "categoryId" not in prod_pre or "brandId" not in prod_pre:
        errors.append("product create pre-request must require categoryId and brandId")
    else:
        ok("product create pre-request builds JSON body")

    ct_headers = [h for h in prod_req.get("header", []) if h.get("key", "").lower() == "content-type"]
    if not ct_headers or ct_headers[0].get("value") != "application/json":
        errors.append("product create must have Content-Type: application/json")
    else:
        ok("product create has application/json Content-Type")

    if product.get("disabled"):
        errors.append("product create request must be enabled (disabled=false)")

    names = [n for n, _ in walk_items(col["item"])]
    dupes = [n for n in set(names) if names.count(n) > 1 and "product" in n.lower()]
    if dupes:
        errors.append(f"duplicate product-flow request names: {dupes[:5]}")

    if SAMPLE_PNG.exists() and SAMPLE_PNG.stat().st_size < 5000:
        ok("sample-product.png exists and is tiny")
    elif not SAMPLE_PNG.exists():
        errors.append("assets/sample-product.png missing (needed for Newman)")

    if errors:
        print("\nValidation failed:")
        for e in errors:
            print(f"  - {e}")
        sys.exit(1)

    print("\nAll validations passed.")


if __name__ == "__main__":
    main()

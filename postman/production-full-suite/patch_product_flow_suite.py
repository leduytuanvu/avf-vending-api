#!/usr/bin/env python3
"""Patch production-full-suite Postman assets for reliable product image upload + product create."""
from __future__ import annotations

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent
COLLECTION = ROOT / "avf-production-full.postman_collection.json"
ENVIRONMENT = ROOT / "avf-production.postman_environment.json"
SAMPLE_PNG = "assets/sample-product.png"

GATED_GUARD = """/* production gated-write guard + runtime ids */
const allowGatedWrites = String(pm.environment.get('allowGatedWrites') || '').toLowerCase() === 'true';
const confirmProductionWrites = pm.environment.get('confirmProductionWrites') === 'I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION';
const allowDestructive = String(pm.environment.get('allow_destructive') || '').toLowerCase() === 'true';
const canaryMode = String(pm.environment.get('canaryMode') || '').toLowerCase() === 'true';
if (!allowGatedWrites || !confirmProductionWrites) {
  throw new Error('GATED-WRITE blocked: set allowGatedWrites=true and confirmProductionWrites=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION');
}
if (pm.info.requestName.includes('[DESTRUCTIVE]') && !allowDestructive) {
  throw new Error('Destructive request blocked: set allow_destructive=true');
}
if (pm.info.requestName.includes('[CANARY]') && !canaryMode) {
  throw new Error('Canary request blocked: set canaryMode=true');
}
pm.environment.set('_runtimeRequestId', pm.variables.replaceIn('{{$guid}}'));
pm.environment.set('_runtimeCorrelationId', pm.variables.replaceIn('{{$guid}}'));
pm.environment.set('_runtimeIdempotencyKey', pm.variables.replaceIn('{{$guid}}'));"""

PRODUCT_CREATE_PREREQUEST = GATED_GUARD + """
function uuidLike(v) {
  if (v === undefined || v === null) return false;
  const s = String(v).trim();
  if (!s || s === 'null' || s === '{{primaryMediaId}}' || s === '{{categoryId}}' || s === '{{brandId}}') return false;
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(s);
}
let sku = pm.environment.get('canaryProductSku') || pm.environment.get('sku');
if (!sku || String(sku).trim() === '') {
  sku = 'COCA-ACTIVE-' + Date.now();
  pm.environment.set('canaryProductSku', sku);
  pm.environment.set('sku', sku);
}
const categoryId = pm.environment.get('categoryId');
const brandId = pm.environment.get('brandId');
if (!uuidLike(categoryId)) {
  throw new Error('product create blocked: set categoryId from POST /v1/admin/categories');
}
if (!uuidLike(brandId)) {
  throw new Error('product create blocked: set brandId from POST /v1/admin/brands');
}
const body = {
  sku: String(sku).trim(),
  name: 'Coca Cola Can 330ml',
  description: 'Production canary test product',
  active: true,
  ageRestricted: false,
  allergenCodes: [],
  categoryId: String(categoryId).trim(),
  brandId: String(brandId).trim(),
};
const mediaId = pm.environment.get('primaryMediaId') || pm.environment.get('mediaId');
if (uuidLike(mediaId)) {
  body.primaryMediaId = String(mediaId).trim();
}
pm.environment.set('_runtimeProductCreateBody', JSON.stringify(body));"""

IMAGE_UPLOAD_TEST = """const json = pm.response.json();

pm.test('product image upload success', function () {
  pm.expect([200, 201]).to.include(pm.response.code);
});

const mediaId = json.mediaId || json.id || (json.media && (json.media.mediaId || json.media.id));
const displayUrl = json.displayUrl || json.url || (json.media && (json.media.displayUrl || json.media.url));
const thumbnailUrl = json.thumbnailUrl || (json.media && json.media.thumbnailUrl);
const provider = json.provider || (json.media && json.media.provider);
const status = json.status || (json.media && json.media.status);

pm.test('media id exists', function () {
  pm.expect(mediaId).to.be.a('string').and.not.empty;
});

pm.test('display url exists', function () {
  pm.expect(displayUrl).to.be.a('string').and.not.empty;
});

if (pm.response.code === 201) {
  pm.test('cloudinary ready response', function () {
    if (provider) pm.expect(provider).to.eql('cloudinary');
    if (status) pm.expect(status).to.eql('ready');
  });
}

pm.environment.set('mediaId', mediaId);
pm.environment.set('primaryMediaId', mediaId);
if (displayUrl) {
  pm.environment.set('primaryMediaUrl', displayUrl);
  pm.environment.set('productImageDisplayUrl', displayUrl);
}
if (thumbnailUrl) {
  pm.environment.set('primaryMediaThumbnailUrl', thumbnailUrl);
  pm.environment.set('productImageThumbnailUrl', thumbnailUrl);
}"""

PRODUCT_CREATE_TEST = """const json = pm.response.json();
pm.test('product create success', function () {
  pm.expect([200, 201]).to.include(pm.response.code);
});
const id = json.id || json.productId || (json.product && json.product.id);
const sku = json.sku || (json.product && json.product.sku);
if (id) pm.environment.set('productId', id);
if (sku) {
  pm.environment.set('sku', sku);
  pm.environment.set('canaryProductSku', sku);
}
if (json.primaryMediaId) pm.environment.set('primaryMediaId', json.primaryMediaId);
pm.test('product active with sku', function () {
  pm.expect(json.active === true || (json.product && json.product.active === true)).to.be.true;
  pm.expect(sku || json.sku).to.be.a('string').and.not.empty;
});"""

BUSINESS_HEADERS = [
    {"key": "X-Request-ID", "value": "{{_runtimeRequestId}}", "type": "text"},
    {"key": "X-Correlation-ID", "value": "{{_runtimeCorrelationId}}", "type": "text"},
    {"key": "Accept", "value": "application/json", "type": "text"},
    {"key": "Idempotency-Key", "value": "{{_runtimeIdempotencyKey}}", "type": "text"},
    {"key": "Authorization", "value": "Bearer {{accessToken}}", "type": "text"},
]

PRODUCT_CREATE_HEADERS = BUSINESS_HEADERS + [
    {"key": "Content-Type", "value": "application/json", "type": "text"},
]


def find_requests(items: list, name: str, acc: list | None = None) -> list:
    if acc is None:
        acc = []
    for item in items:
        if "request" in item:
            if item.get("name") == name:
                acc.append(item)
        if "item" in item:
            find_requests(item["item"], name, acc)
    return acc


def set_script(item: dict, listen: str, lines: str) -> None:
    exec_lines = lines.strip().split("\n")
    events = item.setdefault("event", [])
    for ev in events:
        if ev.get("listen") == listen:
            ev["script"] = {"type": "text/javascript", "exec": exec_lines}
            return
    events.append({"listen": listen, "script": {"type": "text/javascript", "exec": exec_lines}})


def strip_content_type(headers: list) -> list:
    return [h for h in headers if h.get("key", "").lower() != "content-type"]


def patch_collection(data: dict) -> None:
    items = data["item"]
    image_reqs = find_requests(items, "[GATED-WRITE] POST /v1/admin/product-images (Cloudinary multipart)")
    product_reqs = find_requests(items, "[GATED-WRITE] POST /v1/admin/products")
    if len(image_reqs) != 1:
        raise SystemExit(f"expected 1 product-images request, found {len(image_reqs)}")
    if len(product_reqs) != 1:
        raise SystemExit(f"expected 1 product create request, found {len(product_reqs)}")

    img = image_reqs[0]
    req = img["request"]
    req["header"] = strip_content_type(BUSINESS_HEADERS.copy())
    body = req.setdefault("body", {})
    body["mode"] = "formdata"
    body["formdata"] = [
        {
            "key": "file",
            "type": "file",
            "src": SAMPLE_PNG,
            "description": "Select a real local image (png/jpg/jpeg/webp/gif) max 5MB. Newman: use --working-dir postman/production-full-suite",
        },
        {"key": "purpose", "value": "product_image", "type": "text"},
        {"key": "altText", "value": "Sample product image", "type": "text"},
    ]
    img["disabled"] = False
    set_script(img, "prerequest", GATED_GUARD)
    set_script(img, "test", IMAGE_UPLOAD_TEST)

    prod = product_reqs[0]
    preq = prod["request"]
    preq["header"] = [h for h in PRODUCT_CREATE_HEADERS if h["key"] != "X-Idempotency-Key"]
    preq["body"] = {
        "mode": "raw",
        "raw": "{{_runtimeProductCreateBody}}",
        "options": {"raw": {"language": "json"}},
    }
    prod["disabled"] = False
    set_script(prod, "prerequest", PRODUCT_CREATE_PREREQUEST)
    set_script(prod, "test", PRODUCT_CREATE_TEST)


def patch_environment(data: dict) -> None:
    required = {
        "baseUrl": "https://api.ldtv.dev",
        "adminEmail": "admin@ldtv.dev",
        "adminPassword": "1@Ldtv040899",
        "allowGatedWrites": "true",
        "confirmProductionWrites": "I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION",
        "allow_destructive": "true",
        "canaryMode": "true",
        "readiness": "true",
        "_runtimeRequestId": "",
        "_runtimeCorrelationId": "",
        "_runtimeIdempotencyKey": "",
        "_runtimeProductCreateBody": "",
        "primaryMediaId": "",
        "primaryMediaUrl": "",
        "primaryMediaThumbnailUrl": "",
        "categoryId": "",
        "brandId": "",
        "tagId": "",
        "productId": "",
        "machineId": "",
        "canaryProductSku": "",
        "unitPriceMinor": "15000",
        "accessToken": "",
    }
    by_key = {v["key"]: v for v in data["values"]}
    for key, val in required.items():
        if key in by_key:
            by_key[key]["value"] = val
            by_key[key]["enabled"] = True
        else:
            data["values"].append({"key": key, "value": val, "type": "default", "enabled": True})


def main() -> None:
    col = json.loads(COLLECTION.read_text(encoding="utf-8"))
    patch_collection(col)
    COLLECTION.write_text(json.dumps(col, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")

    env = json.loads(ENVIRONMENT.read_text(encoding="utf-8"))
    patch_environment(env)
    ENVIRONMENT.write_text(json.dumps(env, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print("patched:", COLLECTION.name, ENVIRONMENT.name)


if __name__ == "__main__":
    main()

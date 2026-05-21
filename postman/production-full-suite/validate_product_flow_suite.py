#!/usr/bin/env python3
"""Static validator for postman/production-full-suite product admin→app flow."""
from __future__ import annotations

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent
COLLECTION = ROOT / "avf-production-full.postman_collection.json"
ENV_FILE = ROOT / "avf-production.postman_environment.json"
FLOW_COLLECTION = ROOT / "avf-product-admin-to-app-flow.postman_collection.json"

# V1AdminProductMutationRequest JSON keys (camelCase, from internal/httpserver/openapi_types.go)
PRODUCT_CREATE_KEYS = {
    "sku",
    "name",
    "description",
    "attrs",
    "active",
    "status",
    "categoryId",
    "brandId",
    "barcode",
    "countryOfOrigin",
    "ageRestricted",
    "allergenCodes",
    "nutritionalNote",
    "primaryMediaId",
    "primaryImageUrl",
    "tagIds",
}

SECRET_PATTERNS = [
    re.compile(r"eyJhbGci"),
    re.compile(r"refreshToken.*[A-Za-z0-9_-]{20,}", re.I),
    re.compile(r"adminPassword.*1@", re.I),
    re.compile(r"CLOUDINARY_API_SECRET", re.I),
    re.compile(r"DATABASE_URL", re.I),
    re.compile(r"postgresql://", re.I),
    re.compile(r"cloudinary://", re.I),
    re.compile(r"BEGIN RSA", re.I),
    re.compile(r"BEGIN OPENSSH", re.I),
]

GATED_ENV_KEYS = {
    "allowGatedWrites",
    "confirmProductionWrites",
    "allow_destructive",
    "canaryMode",
    "readiness",
}

FLOW_ENV_KEYS = {
    "baseUrl",
    "adminEmail",
    "adminPassword",
    "accessToken",
    "refreshToken",
    "categoryId",
    "brandId",
    "tagId",
    "primaryMediaId",
    "productId",
    "machineId",
    "priceBookId",
    "planogramId",
    "canaryProductSku",
}


def load_json(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def iter_requests(items: list, prefix: str = ""):
    for item in items:
        name = item.get("name", "")
        path = f"{prefix}/{name}" if prefix else name
        if "request" in item:
            yield path, item
        if "item" in item:
            yield from iter_requests(item["item"], path)


def find_request(collection: dict, method: str, url_suffix: str, *, exact_suffix: str | None = None):
    method = method.upper()
    for path, item in iter_requests(collection.get("item", [])):
        req = item["request"]
        if req.get("method", "").upper() != method:
            continue
        raw = req.get("url", {})
        if isinstance(raw, str):
            raw_url = raw
        else:
            raw_url = raw.get("raw", "")
        if exact_suffix and not raw_url.rstrip("/").endswith(exact_suffix):
            continue
        if url_suffix in raw_url or url_suffix in path:
            return path, item
    return None, None


def gated_prerequest_has_idempotency(item: dict) -> bool:
    for ev in item.get("event", []):
        if ev.get("listen") != "prerequest":
            continue
        script = "\n".join(ev.get("script", {}).get("exec", []))
        if "_runtimeIdempotencyKey" in script:
            return True
    req = item.get("request", {})
    for h in req.get("header", []):
        if h.get("key") == "Idempotency-Key" and "{{_runtimeIdempotencyKey}}" in str(h.get("value", "")):
            return True
    return False


def replace_postman_vars(raw: str) -> str:
    out = re.sub(r"\{\{[^}]+\}\}", "00000000-0000-0000-0000-000000000001", raw)
    out = re.sub(r"\{\{\$guid\}\}", "00000000-0000-0000-0000-000000000002", out)
    out = re.sub(r"\{\{\$timestamp\}\}", "1710000000000", out)
    return out


def validate_product_create_body(item: dict) -> tuple[set[str], set[str]]:
    body = item["request"].get("body", {})
    raw = body.get("raw", "")
    # Prefer prerequest JSON.stringify body keys when present
    prereq = ""
    for ev in item.get("event", []):
        if ev.get("listen") == "prerequest":
            prereq = "\n".join(ev.get("script", {}).get("exec", []))
    keys_in_script = set(re.findall(r"^\s+(\w+):", prereq, re.M))
    script_keys = {k for k in keys_in_script if k in PRODUCT_CREATE_KEYS or k == "tagIds"}
    if script_keys:
        return script_keys, set()
    parsed = json.loads(replace_postman_vars(raw))
    if not isinstance(parsed, dict):
        raise ValueError("product create body must be JSON object")
    extra = set(parsed.keys()) - PRODUCT_CREATE_KEYS
    return set(parsed.keys()), extra


def scan_secrets(paths: list[Path]) -> list[str]:
    findings = []
    for path in paths:
        if not path.exists():
            continue
        text = path.read_text(encoding="utf-8", errors="replace")
        for i, line in enumerate(text.splitlines(), 1):
            for pat in SECRET_PATTERNS:
                if pat.search(line):
                    findings.append(f"{path}:{i}: pattern {pat.pattern}")
    return findings


def main() -> int:
    errors: list[str] = []
    print("=== JSON parse ===")
    for p in sorted(ROOT.rglob("*.json")):
        load_json(p)
        print("JSON OK:", p.relative_to(ROOT))

    col = load_json(COLLECTION)
    env = load_json(ENV_FILE)

    print("\n=== Product image upload ===")
    _, img_item = find_request(col, "POST", "/v1/admin/product-images")
    if not img_item:
        errors.append("missing POST /v1/admin/product-images")
    else:
        body = img_item["request"].get("body", {})
        if body.get("mode") != "formdata":
            errors.append("product-images body.mode must be formdata")
        headers = {h.get("key", "").lower(): h.get("value") for h in img_item["request"].get("header", [])}
        if "content-type" in headers:
            errors.append("product-images must NOT manually set Content-Type")
        test_script = ""
        for ev in img_item.get("event", []):
            if ev.get("listen") == "test":
                test_script = "\n".join(ev.get("script", {}).get("exec", []))
        if "primaryMediaId" not in test_script:
            errors.append("product-images test must save primaryMediaId")
        print("product-images: OK" if not errors else "product-images: issues found")

    print("\n=== Product create ===")
    _, prod_item = find_request(
        col, "POST", "/v1/admin/products", exact_suffix="/v1/admin/products"
    )
    if not prod_item:
        errors.append("missing POST /v1/admin/products")
    else:
        body = prod_item["request"].get("body", {})
        if body.get("mode") != "raw":
            errors.append("product create must be raw JSON")
        headers = {h.get("key", "").lower(): h.get("value") for h in prod_item["request"].get("header", [])}
        if headers.get("content-type") != "application/json":
            errors.append("product create must have Content-Type: application/json")
        try:
            keys, extra = validate_product_create_body(prod_item)
            if extra:
                errors.append(f"product create has unknown keys: {sorted(extra)}")
            prereq = ""
            for ev in prod_item.get("event", []):
                if ev.get("listen") == "prerequest":
                    prereq = "\n".join(ev.get("script", {}).get("exec", []))
            if "_runtimeProductCreateBody" not in prereq and "JSON.stringify(body)" not in prereq:
                errors.append("product create prerequest must set _runtimeProductCreateBody")
            raw_body = body.get("raw", "")
            if raw_body.strip() != "{{_runtimeProductCreateBody}}":
                errors.append("product create raw body must be {{_runtimeProductCreateBody}}")
            print(f"product create keys: {sorted(keys)}")
        except Exception as exc:
            errors.append(f"product create body parse: {exc}")

    print("\n=== Gated writes ===")
    gated_missing = 0
    for path, item in iter_requests(col.get("item", [])):
        if "[GATED-WRITE]" not in path:
            continue
        if not gated_prerequest_has_idempotency(item):
            gated_missing += 1
            errors.append(f"gated write missing idempotency runtime key: {path}")
    print(f"gated writes checked; missing idempotency: {gated_missing}")

    print("\n=== Environment ===")
    env_keys = {v["key"] for v in env.get("values", [])}
    missing_env = (GATED_ENV_KEYS | FLOW_ENV_KEYS) - env_keys
    if missing_env:
        errors.append(f"environment missing keys: {sorted(missing_env)}")
    for v in env.get("values", []):
        if v["key"] == "adminPassword" and v.get("value") not in ("<set-in-postman>", ""):
            if not v["value"].startswith("<"):
                errors.append("adminPassword must be placeholder in committed env")
    print(f"environment keys: {len(env_keys)}")

    if FLOW_COLLECTION.exists():
        flow = load_json(FLOW_COLLECTION)
        if flow.get("info", {}).get("name"):
            print(f"flow collection present: {FLOW_COLLECTION.name}")

    print("\n=== Secret scan ===")
    scan_paths = list(ROOT.rglob("*.json")) + list(ROOT.rglob("*.md"))
    findings = scan_secrets(scan_paths)
    if findings:
        for f in findings:
            errors.append(f"secret scan: {f}")
            print("SECRET?", f)
    else:
        print("no secrets detected")

    print("\n=== Summary ===")
    if errors:
        for e in errors:
            print("ERROR:", e)
        return 1
    print("ALL CHECKS PASSED")
    return 0


if __name__ == "__main__":
    sys.exit(main())

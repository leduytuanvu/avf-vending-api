#!/usr/bin/env python3
"""One-shot patches for product admin→app flow in production Postman suite."""
from __future__ import annotations

import copy
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent
COLLECTION_PATH = ROOT / "avf-production-full.postman_collection.json"
ENV_PATH = ROOT / "avf-production.postman_environment.json"
FLOW_PATH = ROOT / "avf-product-admin-to-app-flow.postman_collection.json"

GATED_GUARD = [
    "/* production gated-write guard */",
    'const allowGatedWrites = String(pm.variables.get("allowGatedWrites") || "").trim().toLowerCase();',
    'const confirmProductionWrites = String(pm.variables.get("confirmProductionWrites") || "").trim();',
    "",
    'if (allowGatedWrites !== "true" || confirmProductionWrites !== "I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION") {',
    '  throw new Error("GATED-WRITE blocked: set allowGatedWrites=true and confirmProductionWrites=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION");',
    "}",
    "",
    "if (!pm.variables.get(\"_runtimeRequestId\")) {",
    '  pm.variables.set("_runtimeRequestId", pm.variables.replaceIn("{{$guid}}"));',
    "}",
    "if (!pm.variables.get(\"_runtimeCorrelationId\")) {",
    '  pm.variables.set("_runtimeCorrelationId", pm.variables.replaceIn("{{$guid}}"));',
    "}",
    'pm.variables.set("_runtimeIdempotencyKey", pm.variables.replaceIn("{{$guid}}"));',
]

PLANOGRAM_BODY_BUILDER = [
    "",
    "/* build planogram draft/publish body */",
    "const productId = String(pm.environment.get('productId') || '').trim();",
    "if (!productId) { throw new Error('planogram requires productId'); }",
    "let operatorSessionId = String(pm.environment.get('operatorSessionId') || '').trim();",
    "if (!operatorSessionId) {",
    "  operatorSessionId = pm.variables.replaceIn('{{$guid}}');",
    "  pm.environment.set('operatorSessionId', operatorSessionId);",
    "}",
    "const slotIndex = parseInt(String(pm.environment.get('slotIndex') || '1'), 10) || 1;",
    "const priceMinor = parseInt(String(pm.environment.get('unitPriceMinor') || '150'), 10) || 150;",
    "const body = {",
    "  operator_session_id: operatorSessionId,",
    "  syncLegacyReadModel: true,",
    "  items: [{",
    "    cabinetCode: 'A',",
    "    layoutKey: 'grid-4x6',",
    "    layoutRevision: 1,",
    "    slotCode: 'A' + slotIndex,",
    "    legacySlotIndex: slotIndex,",
    "    productId: productId,",
    "    maxQuantity: 12,",
    "    priceMinor: priceMinor,",
    "    metadata: {},",
    "  }],",
    "};",
    "const planogramId = String(pm.environment.get('planogramId') || '').trim();",
    "if (planogramId && !planogramId.startsWith('<')) {",
    "  body.planogramId = planogramId;",
    "  body.planogramRevision = parseInt(String(pm.environment.get('planogramRevision') || '1'), 10) || 1;",
    "}",
    "pm.request.body.raw = JSON.stringify(body);",
    "pm.request.headers.upsert({ key: 'Content-Type', value: 'application/json' });",
]

PRICE_ITEMS_BUILDER = [
    "",
    "const productId = String(pm.environment.get('productId') || '').trim();",
    "if (!productId) { throw new Error('price book items requires productId'); }",
    "const unitPriceMinor = parseInt(String(pm.environment.get('unitPriceMinor') || '150'), 10) || 150;",
    "pm.request.body.raw = JSON.stringify({ items: [{ productId, unitPriceMinor }] });",
    "pm.request.headers.upsert({ key: 'Content-Type', value: 'application/json' });",
]

PRICE_BOOK_CREATE_BUILDER = [
    "",
    "const ts = String(Date.now());",
    "const body = {",
    "  name: 'canary-price-book-' + ts,",
    "  currency: 'USD',",
    "  effectiveFrom: new Date().toISOString(),",
    "  isDefault: false,",
    "  scopeType: 'company',",
    "  priority: 10,",
    "};",
    "pm.request.body.raw = JSON.stringify(body);",
    "pm.request.headers.upsert({ key: 'Content-Type', value: 'application/json' });",
]

ASSIGN_TARGET_BUILDER = [
    "",
    "const machineId = String(pm.environment.get('machineId') || '').trim();",
    "if (!machineId || machineId.startsWith('<')) {",
    "  throw new Error('assign-target requires machineId in environment');",
    "}",
    "pm.request.body.raw = JSON.stringify({ machineId });",
    "pm.request.headers.upsert({ key: 'Content-Type', value: 'application/json' });",
]

IMAGE_TEST_EXTRA = [
    "if (displayUrl) {",
    "  pm.environment.set('primaryMediaUrl', displayUrl);",
    "  pm.environment.set('productImageDisplayUrl', displayUrl);",
    "}",
    "if (thumbnailUrl) {",
    "  pm.environment.set('primaryMediaThumbnailUrl', thumbnailUrl);",
    "  pm.environment.set('productImageThumbnailUrl', thumbnailUrl);",
    "}",
]


def iter_requests(items, acc=None):
    if acc is None:
        acc = []
    for item in items:
        if "request" in item:
            acc.append(item)
        if "item" in item:
            iter_requests(item["item"], acc)
    return acc


def patch_prerequest(item, extra_lines: list[str]):
    for ev in item.get("event", []):
        if ev.get("listen") != "prerequest":
            continue
        exec_lines = ev["script"]["exec"]
        joined = "\n".join(exec_lines)
        for block in extra_lines:
            if block.strip() and block.strip() not in joined:
                exec_lines.extend(block if isinstance(block, list) else [block])
        return
    item.setdefault("event", []).append(
        {"listen": "prerequest", "script": {"type": "text/javascript", "exec": GATED_GUARD + extra_lines}}
    )


def patch_test_save_price_book(item):
    test_lines = [
        "pm.test('price book created', function () {",
        "  pm.expect(pm.response.code).to.be.oneOf([200, 201]);",
        "});",
        "try {",
        "  const j = pm.response.json();",
        "  const id = j.id || j.priceBookId;",
        "  if (id) pm.environment.set('priceBookId', String(id));",
        "} catch (e) { /* ignore */ }",
    ]
    for ev in item.get("event", []):
        if ev.get("listen") == "test":
            if "priceBookId" not in "\n".join(ev["script"]["exec"]):
                ev["script"]["exec"] = test_lines + ev["script"]["exec"]
            return
    item.setdefault("event", []).append({"listen": "test", "script": {"type": "text/javascript", "exec": test_lines}})


def patch_collection(data: dict) -> int:
    changes = 0
    for item in iter_requests(data.get("item", [])):
        name = item.get("name", "")
        url_raw = item.get("request", {}).get("url", {})
        raw = url_raw if isinstance(url_raw, str) else url_raw.get("raw", "")

        if "POST /v1/admin/product-images" in name and "product-images" in raw:
            for ev in item.get("event", []):
                if ev.get("listen") == "test":
                    script = ev["script"]["exec"]
                    joined = "\n".join(script)
                    if "primaryMediaUrl" not in joined:
                        # insert before closing of test script
                        idx = len(script)
                        for i, line in enumerate(script):
                            if "productImageDisplayUrl" in line:
                                script[i:i+1] = IMAGE_TEST_EXTRA
                                changes += 1
                                break

        if "[GATED-WRITE] POST /v1/admin/products" == name.strip() or (
            name.endswith("POST /v1/admin/products") and "{productId}" not in raw
        ):
            item["disabled"] = False
            changes += 1

        if "planograms/draft" in raw or "planograms/publish" in raw:
            patch_prerequest(item, PLANOGRAM_BODY_BUILDER)
            for ev in item.get("event", []):
                if ev.get("listen") == "test" and "planogramId" not in "\n".join(ev["script"]["exec"]):
                    ev["script"]["exec"].extend([
                        "try {",
                        "  const j = pm.response.json();",
                        "  const pid = j.planogramId || j.planogram_id || (j.planogram && j.planogram.id);",
                        "  if (pid) pm.environment.set('planogramId', String(pid));",
                        "  const cmd = j.command || {};",
                        "  if (cmd.commandId) pm.environment.set('commandId', String(cmd.commandId));",
                        "} catch (e) { /* ignore */ }",
                    ])
            changes += 1

        if "PUT /v1/admin/price-books" in name and "/items" in raw:
            patch_prerequest(item, PRICE_ITEMS_BUILDER)
            changes += 1

        if name.strip() == "[GATED-WRITE] POST /v1/admin/price-books":
            patch_prerequest(item, PRICE_BOOK_CREATE_BUILDER)
            patch_test_save_price_book(item)
            changes += 1

        if "assign-target" in raw and "price-books" in raw:
            patch_prerequest(item, ASSIGN_TARGET_BUILDER)
            changes += 1

    return changes


def ensure_env_keys(env: dict) -> int:
    values = {v["key"]: v for v in env.get("values", [])}
    additions = {
        "primaryMediaUrl": "<set-after-upload-product-image>",
        "primaryMediaThumbnailUrl": "<set-after-upload-product-image>",
        "priceBookItemId": "<set-after-create-price-book-item>",
        "unitPriceMinor": "150",
        "machineId": "<set-in-postman-or-discovered>",
        "canaryProductSku": "",
    }
    changed = 0
    for key, default in additions.items():
        if key not in values:
            env.setdefault("values", []).append(
                {"key": key, "value": default, "type": "default", "enabled": True}
            )
            changed += 1
        elif key == "machineId" and values[key].get("value") == "<set-after-create-machine>":
            values[key]["value"] = "<set-in-postman-or-discovered>"
            changed += 1
        elif key == "primaryMediaId" and values[key].get("value") == "<set-after-upload-image>":
            values[key]["value"] = "<set-after-upload-product-image>"
            changed += 1
    return changed


def build_flow_collection(main: dict) -> dict:
    """Focused runnable collection referencing same request patterns."""
    lookup = {}
    for item in iter_requests(main.get("item", [])):
        name = item.get("name", "")
        url_raw = item.get("request", {}).get("url", {})
        raw = url_raw if isinstance(url_raw, str) else url_raw.get("raw", "")
        lookup[name] = (raw, item)

    def pick(substr: str, method_hint: str = ""):
        for name, (raw, item) in lookup.items():
            if substr in raw or substr in name:
                if method_hint and method_hint not in name:
                    continue
                return copy.deepcopy(item)
        return None

    steps = [
        ("Auth", [pick("/v1/auth/login", "POST"), pick("/v1/auth/me", "GET")]),
        ("Category", [pick("/v1/admin/categories", "POST"), pick("/v1/admin/categories", "GET")]),
        ("Brand", [pick("/v1/admin/brands", "POST"), pick("/v1/admin/brands", "GET")]),
        ("Tag", [pick("/v1/admin/tags", "POST"), pick("/v1/admin/tags", "GET")]),
        ("Product Image", [pick("/v1/admin/product-images", "POST")]),
        ("Product", [pick("/v1/admin/products", "POST"), pick("/v1/admin/products/", "GET")]),
        ("Price", [
            pick("/v1/admin/price-books", "POST"),
            pick("/items", "PUT"),
            pick("assign-target", "POST"),
        ]),
        ("Planogram", [
            pick("planograms/draft", "PUT"),
            pick("planograms/publish", "POST"),
            pick("/sync", "POST"),
        ]),
        ("Machine Catalog Sync", [pick("sale-catalog", "GET")]),
    ]

    folders = []
    for folder_name, requests in steps:
        items = [r for r in requests if r]
        if not items:
            continue
        for r in items:
            r["disabled"] = False
        folders.append({"name": folder_name, "item": items})

    return {
        "info": {
            "_postman_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
            "name": "AVF Product Admin To App Flow",
            "description": "Curated production canary flow: admin catalog → price → planogram → machine sale catalog.\nImport with avf-production.postman_environment.json.",
            "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
        },
        "item": [
            {
                "name": "Health",
                "item": [
                    pick("/health/live", "GET"),
                    pick("/health/ready", "GET"),
                    pick("/version", "GET"),
                ],
            },
            {
                "name": "Product Admin To App Flow",
                "item": folders,
            },
            {
                "name": "gRPC (manual)",
                "description": "See PRODUCT_ADMIN_TO_APP_FLOW_TEST_GUIDE.md § gRPC",
                "item": [],
            },
            {
                "name": "MQTT (manual)",
                "description": "Planogram publish dispatches machine_planogram_publish. See guide § MQTT.",
                "item": [],
            },
        ],
    }


def main():
    col = json.loads(COLLECTION_PATH.read_text(encoding="utf-8"))
    n = patch_collection(col)
    COLLECTION_PATH.write_text(json.dumps(col, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print(f"Patched main collection ({n} change groups)")

    env = json.loads(ENV_PATH.read_text(encoding="utf-8"))
    e = ensure_env_keys(env)
    ENV_PATH.write_text(json.dumps(env, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print(f"Updated environment ({e} keys)")

    flow = build_flow_collection(col)
    # filter None health items
    flow["item"][0]["item"] = [x for x in flow["item"][0]["item"] if x]
    FLOW_PATH.write_text(json.dumps(flow, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print(f"Wrote flow collection: {FLOW_PATH.name}")


if __name__ == "__main__":
    main()

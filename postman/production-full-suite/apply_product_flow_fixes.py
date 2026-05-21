#!/usr/bin/env python3
"""Apply product-flow Postman fixes (run from repo root or this directory)."""
from __future__ import annotations

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent
COLLECTIONS = [
    ROOT / "avf-production-full.postman_collection.json",
    ROOT / "avf-product-admin-to-app-flow.postman_collection.json",
]
ENV = ROOT / "avf-production.postman_environment.json"

GATED_TAIL = [
    'pm.variables.set("_runtimeRequestId", pm.variables.replaceIn("{{$guid}}"));',
    'pm.variables.set("_runtimeCorrelationId", pm.variables.replaceIn("{{$guid}}"));',
    'pm.variables.set("_runtimeIdempotencyKey", pm.variables.replaceIn("{{$guid}}"));',
]

PRODUCT_CREATE_PREREQ = [
    "/* production gated-write guard */",
    'const allowGatedWrites = String(pm.variables.get("allowGatedWrites") || "").trim().toLowerCase();',
    'const confirmProductionWrites = String(pm.variables.get("confirmProductionWrites") || "").trim();',
    "",
    'if (allowGatedWrites !== "true" || confirmProductionWrites !== "I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION") {',
    '  throw new Error("GATED-WRITE blocked: set allowGatedWrites=true and confirmProductionWrites=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION");',
    "}",
    "",
] + GATED_TAIL + [
    "",
    "const sku = pm.variables.get('canaryProductSku') || ('COCA-330ML-' + Date.now());",
    "pm.variables.set('canaryProductSku', sku);",
    "const body = {",
    "  sku: sku,",
    "  name: 'Coca Cola Can 330ml',",
    "  description: 'Production canary test product',",
    "  active: true,",
    "  ageRestricted: false,",
    "  allergenCodes: [],",
    "  categoryId: pm.variables.get('categoryId'),",
    "  brandId: pm.variables.get('brandId'),",
    "  primaryMediaId: pm.variables.get('primaryMediaId'),",
    "};",
    "const tagId = pm.variables.get('tagId');",
    "if (tagId && !String(tagId).startsWith('<')) { body.tagIds = [tagId]; }",
    "for (const k of ['categoryId', 'brandId', 'primaryMediaId']) {",
    "  if (!body[k] || String(body[k]).startsWith('<')) {",
    "    throw new Error('Missing required product field: ' + k);",
    "  }",
    "}",
    "pm.variables.set('_runtimeProductCreateBody', JSON.stringify(body));",
]

MACHINE_ID_TEST_LINES = [
    "try {",
    "  const j = pm.response.json();",
    "  const items = j.items || [];",
    "  if (items.length && !pm.environment.get('machineId')) {",
    "    const m = items[0];",
    "    const mid = m.machineId || m.id;",
    "    if (mid) pm.environment.set('machineId', String(mid));",
    "  }",
    "} catch (e) { /* ignore */ }",
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


def patch_collection(path: Path) -> int:
    data = json.loads(path.read_text(encoding="utf-8"))
    n = 0
    for item in iter_requests(data.get("item", [])):
        name = item.get("name", "")
        url = item.get("request", {}).get("url", {})
        raw = url if isinstance(url, str) else url.get("raw", "")

        if name == "[GATED-WRITE] POST /v1/admin/products" and raw.rstrip("/").endswith("/v1/admin/products"):
            item["request"]["body"] = {
                "mode": "raw",
                "raw": "{{_runtimeProductCreateBody}}",
                "options": {"raw": {"language": "json"}},
            }
            for ev in item.get("event", []):
                if ev.get("listen") == "prerequest":
                    ev["script"]["exec"] = PRODUCT_CREATE_PREREQ
            n += 1

        if "GET /v1/admin/machines" == name and "machines" in raw and "{{" not in raw.split("machines")[-1][:20]:
            for ev in item.get("event", []):
                if ev.get("listen") == "test":
                    script = ev["script"]["exec"]
                    if "machineId || m.id" not in "\n".join(script):
                        ev["script"]["exec"] = MACHINE_ID_TEST_LINES + script
                        n += 1

        if "[GATED-WRITE]" in name:
            for ev in item.get("event", []):
                if ev.get("listen") != "prerequest":
                    continue
                lines = ev["script"]["exec"]
                joined = "\n".join(lines)
                if "_runtimeIdempotencyKey" not in joined:
                    continue
                # normalize: always set runtime ids (no conditional skip)
                if 'if (!pm.variables.get("_runtimeRequestId"))' in joined:
                    new_lines = []
                    skip_until = False
                    for line in lines:
                        if 'if (!pm.variables.get("_runtimeRequestId"))' in line:
                            skip_until = True
                            continue
                        if skip_until and "pm.variables.set" in line and "_runtimeRequestId" in line:
                            continue
                        if skip_until and line.strip() == "}":
                            skip_until = False
                            continue
                        if skip_until and "_runtimeCorrelationId" in line:
                            continue
                        if skip_until and line.strip() == "}":
                            skip_until = False
                            continue
                        new_lines.append(line)
                    lines[:] = new_lines
                    # insert standard setters after guard block
                    idx = 0
                    for i, line in enumerate(lines):
                        if "I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION" in line:
                            idx = i + 1
                            while idx < len(lines) and lines[idx].strip() in ("", "}"):
                                idx += 1
                            break
                    for j, gl in enumerate(GATED_TAIL):
                        if gl not in joined:
                            lines.insert(idx + j, gl)
                    n += 1

    path.write_text(json.dumps(data, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    return n


def patch_env() -> None:
    env = json.loads(ENV.read_text(encoding="utf-8"))
    updates = {
        "unitPriceMinor": "15000",
        "machineId": "<set-existing-machine-id>",
        "primaryMediaId": "<set-after-upload-image>",
        "primaryMediaUrl": "<set-after-upload-image>",
        "primaryMediaThumbnailUrl": "<set-after-upload-image>",
        "_runtimeProductCreateBody": "",
    }
    keys = {v["key"]: v for v in env["values"]}
    for k, val in updates.items():
        if k in keys:
            keys[k]["value"] = val
        else:
            env["values"].append({"key": k, "value": val, "type": "default", "enabled": True})
    ENV.write_text(json.dumps(env, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")


def main():
    patch_env()
    for p in COLLECTIONS:
        if p.exists():
            print(f"{p.name}: {patch_collection(p)} patches")


if __name__ == "__main__":
    main()

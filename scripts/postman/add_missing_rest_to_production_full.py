#!/usr/bin/env python3
"""Insert missing OpenAPI REST operations into the numbered production-full collection."""
from __future__ import annotations

import json
import re
import sys
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

from gate0_inventory import COLLECTION, SWAGGER, HTTP_VERBS, openapi_ops, postman_rest_ops, skeleton_path  # noqa: E402

FOLDER_RULES = [
    ("/v1/admin/machine-codes", "07 Machine Setup - Activation, Bootstrap, Config"),
    ("/v1/admin/machines", "05 Fleet - Machines"),
    ("/v1/admin/payment/", "17 Commerce - Payments"),
    ("/v1/admin/payments", "22 Finance & Settlement"),
]


def folder_for(path: str) -> str:
    for prefix, folder in FOLDER_RULES:
        if path.startswith(prefix):
            return folder
    return "29 Admin Utilities / Negative Tests"


def openapi_path_ops() -> dict[tuple[str, str], str]:
    spec = json.loads(SWAGGER.read_text(encoding="utf-8"))
    out: dict[tuple[str, str], str] = {}
    for path, item in (spec.get("paths") or {}).items():
        if not isinstance(item, dict):
            continue
        for method, op in item.items():
            if method.lower() not in HTTP_VERBS or not isinstance(op, dict):
                continue
            out[(method.upper(), path)] = str(op.get("summary") or op.get("operationId") or path)
    return out


def path_to_postman(path: str) -> tuple[str, list[str]]:
    segs = []
    raw_parts = []
    for seg in path.strip("/").split("/"):
        m = re.fullmatch(r"\{([^}]+)\}", seg)
        if m:
            segs.append("{{%s}}" % m.group(1))
            raw_parts.append("{{%s}}" % m.group(1))
        else:
            segs.append(seg)
            raw_parts.append(seg)
    raw = "{{baseUrl}}/" + "/".join(raw_parts)
    return raw, segs


def make_item(method: str, path: str, summary: str) -> dict:
    raw, segs = path_to_postman(path)
    gated = method in {"POST", "PUT", "PATCH", "DELETE"}
    headers = [
        {"key": "Accept", "value": "application/json"},
        {"key": "X-Request-ID", "value": "{{requestId}}"},
        {"key": "X-Correlation-ID", "value": "{{correlationId}}"},
        {"key": "Authorization", "value": "Bearer {{accessToken}}"},
    ]
    if gated:
        headers.append({"key": "Idempotency-Key", "value": "{{$guid}}"})
    req: dict = {
        "method": method,
        "header": headers,
        "url": {
            "raw": raw,
            "host": ["{{baseUrl}}"],
            "path": segs,
            "query": [],
        },
        "description": "Gate 0 coverage for OpenAPI `%s %s` — %s" % (method, path, summary),
        "auth": {
            "type": "bearer",
            "bearer": [{"key": "token", "value": "{{accessToken}}", "type": "string"}],
        },
    }
    if method in {"POST", "PUT", "PATCH"}:
        req["body"] = {"mode": "raw", "raw": "{}", "options": {"raw": {"language": "json"}}}
    item: dict = {
        "name": "%s %s — %s" % (method, path, summary),
        "request": req,
        "response": [],
    }
    if gated:
        item["event"] = [
            {
                "listen": "prerequest",
                "script": {
                    "type": "text/javascript",
                    "exec": [
                        "const allowGatedWrites = String(pm.environment.get('allowGatedWrites') || '').toLowerCase() === 'true';",
                        "const confirmProductionWrites = pm.environment.get('confirmProductionWrites') === 'I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION';",
                        "if (pm.environment.get('baseUrl') && String(pm.environment.get('baseUrl')).includes('ldtv.dev')) {",
                        "  if (!allowGatedWrites || !confirmProductionWrites) {",
                        "    throw new Error('GATED-WRITE blocked');",
                        "  }",
                        "}",
                    ],
                },
            }
        ]
    return item


def main() -> int:
    spec_ops = openapi_path_ops()
    have, _leaf = postman_rest_ops()
    missing: list[tuple[str, str]] = []
    for method, path in sorted(spec_ops):
        if (method, skeleton_path(path)) not in have:
            missing.append((method, path))
    if not missing:
        print("no missing OpenAPI REST ops")
        return 0
    coll = json.loads(COLLECTION.read_text(encoding="utf-8"))
    folders = {it.get("name"): it for it in coll.get("item") or []}
    added = 0
    for method, path in missing:
        folder_name = folder_for(path)
        folder = folders.get(folder_name)
        if folder is None:
            print("missing folder", folder_name, file=sys.stderr)
            return 1
        folder.setdefault("item", []).append(make_item(method, path, spec_ops[(method, path)]))
        added += 1
        print("added", method, path, "->", folder_name)
    COLLECTION.write_text(json.dumps(coll, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print("wrote", added, "requests to", COLLECTION)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

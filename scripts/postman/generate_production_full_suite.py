#!/usr/bin/env python3
"""Regenerate postman/suites/production-full from OpenAPI + proto + MQTT contracts."""
from __future__ import annotations

import hashlib
import json
import re
import shutil
import sys
from collections import defaultdict
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

SCRIPT_DIR = Path(__file__).resolve().parent
REPO_ROOT = SCRIPT_DIR.parents[1]
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

from folder_business import FOLDER_ORDER, assign_folder_business, flow_name  # noqa: E402
from gfs_import import REPO_ROOT as GFS_ROOT, gfs  # noqa: E402

assert REPO_ROOT == GFS_ROOT

OUT_DIR = REPO_ROOT / "postman" / "suites" / "production-full"
SWAGGER = REPO_ROOT / "docs" / "swagger" / "swagger.json"
COLLECTION_NAME = "avf-vending-production.full.postman_collection.json"
ENV_NAME = "avf-vending-production.full.postman_environment.json"
GUIDE_NAME = "TESTING_GUIDE.md"

DOMAIN_ORDER = [
    "Health System",
    "Auth",
    "Admin Accounts RBAC",
    "Catalog",
    "Brands",
    "Categories",
    "Tags",
    "Product Media",
    "Products",
    "Sites Regions",
    "Machines",
    "Machine Provisioning",
    "Machine Runtime Config",
    "Telemetry",
    "Inventory",
    "Planogram Assortment",
    "Orders",
    "Payments",
    "Refunds Disputes",
    "Promotions PriceBooks",
    "Finance Reconciliation",
    "Incidents Diagnostics",
    "OTA Rollout",
    "Audit Logs",
    "Utilities",
]

GATED_WRITE_PREREQUEST = [
    "/* production gated-write guard */",
    "const allowGatedWrites = String(pm.environment.get('allowGatedWrites') || '').toLowerCase() === 'true';",
    "const confirmProductionWrites = pm.environment.get('confirmProductionWrites') === 'I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION';",
    "const allowDestructive = String(pm.environment.get('allow_destructive') || '').toLowerCase() === 'true';",
    "const canaryMode = String(pm.environment.get('canaryMode') || '').toLowerCase() === 'true';",
    "if (!allowGatedWrites || !confirmProductionWrites) {",
    "  throw new Error('GATED-WRITE blocked: set allowGatedWrites=true and confirmProductionWrites=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION');",
    "}",
    "if (pm.info.requestName.includes('[DESTRUCTIVE]') && !allowDestructive) {",
    "  throw new Error('Destructive request blocked: set allow_destructive=true');",
    "}",
    "if (pm.info.requestName.includes('[CANARY]') && !canaryMode) {",
    "  throw new Error('Canary request blocked: set canaryMode=true');",
    "}",
]

READONLY_PREREQUEST = [
    "/* readonly prerequest - no gated-write */",
]

LOGIN_TEST_SCRIPT = [
    "let json = {};",
    "try {",
    "  json = pm.response.json();",
    "} catch (e) {",
    '  throw new Error("Login response is not valid JSON: " + e.message);',
    "}",
    "",
    'pm.test("login returns 200", function () {',
    "  pm.expect(pm.response.code).to.equal(200);",
    "});",
    "",
    'pm.test("login returns tokens", function () {',
    "  pm.expect(json).to.have.property('tokens');",
    "  pm.expect(json.tokens).to.have.property('accessToken');",
    "  pm.expect(json.tokens).to.have.property('refreshToken');",
    "  pm.expect(json.tokens.accessToken).to.be.a('string').and.not.empty;",
    "  pm.expect(json.tokens.refreshToken).to.be.a('string').and.not.empty;",
    "});",
    "",
    "if (pm.response.code === 200 && json.tokens) {",
    "  pm.environment.set('accessToken', json.tokens.accessToken);",
    "  pm.environment.set('refreshToken', json.tokens.refreshToken);",
    "  if (json.accountId) {",
    "    pm.environment.set('accountId', json.accountId);",
    "  }",
    "  if (json.email) {",
    "    pm.environment.set('adminEmailResolved', json.email);",
    "  }",
    "  if (json.roles) {",
    "    pm.environment.set('roles', JSON.stringify(json.roles));",
    "  }",
    "  if (json.tokens.accessExpiresAt) {",
    "    pm.environment.set('accessExpiresAt', json.tokens.accessExpiresAt);",
    "  }",
    "  if (json.tokens.refreshExpiresAt) {",
    "    pm.environment.set('refreshExpiresAt', json.tokens.refreshExpiresAt);",
    "  }",
    "  if (json.tokens.tokenType) {",
    "    pm.environment.set('tokenType', json.tokens.tokenType);",
    "  }",
    "}",
]

AUTH_ME_TEST_SCRIPT = [
    'pm.test("auth me returns 200", function () {',
    "  pm.expect(pm.response.code).to.equal(200);",
    "});",
    "",
    "try {",
    "  const json = pm.response.json();",
    '  pm.test("auth me returns account identity", function () {',
    "    pm.expect(json).to.have.property('accountId');",
    "    pm.expect(json).to.have.property('email');",
    "  });",
    "  if (json.accountId) {",
    "    pm.environment.set('accountId', json.accountId);",
    "  }",
    "} catch (e) {",
    '  throw new Error("Auth me response is not valid JSON: " + e.message);',
    "}",
]


def assign_production_domain(path: str, method: str, tags: list) -> str:
    p = path.lower()
    if "/v1/admin/brands" in p:
        return "Brands"
    if "/v1/admin/categories" in p:
        return "Categories"
    if "/v1/admin/tags" in p:
        return "Tags"
    if any(x in p for x in ("/media", "/offline", "/manifest", "external-images", "product-images")):
        return "Product Media"
    if "/v1/admin/products" in p and "catalog" not in " ".join(tags).lower():
        return "Products"
    if "/v1/admin/machines" in p and method.upper() == "POST" and any(x in p for x in ("activation", "claim")):
        return "Machine Provisioning"
    if "/v1/setup/" in p or "activation-codes" in p:
        return "Machine Provisioning"
    if "/v1/admin/machines" in p or "/v1/machines/" in p:
        if "telemetry" in p or "check-in" in p:
            return "Telemetry"
        return "Machines" if "/v1/admin/machines" in p and method.upper() in ("GET", "POST") else "Machine Runtime Config"
    base = assign_folder_business(path, method, tags)
    fm = flow_name(base)
    aliases = {
        "Catalog Categories Brands Tags": "Catalog",
        "Product Media Offline Cache": "Product Media",
        "Sites Regions": "Sites Regions",
        "Machines Provisioning": "Machine Provisioning",
        "Machines Runtime Config": "Machine Runtime Config",
        "Machines Telemetry": "Telemetry",
        "Planogram Assortment": "Planogram Assortment",
        "Refunds Disputes": "Refunds Disputes",
        "Promotions PriceBooks": "Promotions PriceBooks",
        "Finance Reconciliation": "Finance Reconciliation",
        "Incidents Diagnostics": "Incidents Diagnostics",
        "OTA Rollout": "OTA Rollout",
        "Audit Logs": "Audit Logs",
        "Admin Accounts RBAC": "Admin Accounts RBAC",
    }
    return aliases.get(fm, fm)


def flatten_requests(items: list) -> list[dict]:
    out: list[dict] = []
    for it in items:
        if "request" in it:
            out.append(it)
        elif "item" in it:
            out.extend(flatten_requests(it["item"]))
    return out


def request_path_key(req_item: dict) -> str:
    req = req_item.get("request") or {}
    method = (req.get("method") or "").upper()
    url = req.get("url")
    if isinstance(url, dict):
        raw = url.get("raw") or ""
        path = raw.split("?", 1)[0]
        path = path.replace("{{baseUrl}}", "")
    else:
        path = str(url or "")
    return "%s %s" % (method, path.strip())


def patch_headers(headers: list[dict], method: str, *, is_login: bool, requires_auth: bool) -> list[dict]:
    out: list[dict] = []
    seen: set[str] = set()
    for h in headers or []:
        if h.get("disabled"):
            continue
        key = h.get("key") or ""
        kl = key.lower()
        val = h.get("value") or ""
        if is_login and kl in ("authorization", "idempotency-key", "x-idempotency-key"):
            continue
        if kl == "x-request-id":
            val = "{{$guid}}"
        elif kl == "x-correlation-id":
            val = "{{$guid}}"
        elif kl in ("idempotency-key", "x-idempotency-key"):
            if method in ("POST", "PUT", "PATCH", "DELETE"):
                val = "{{$guid}}"
            else:
                continue
        elif kl == "authorization":
            if not requires_auth:
                continue
            val = "Bearer {{accessToken}}"
        out.append({"key": key, "value": val, "type": h.get("type") or "text"})
        seen.add(kl)
    for key, val in (
        ("X-Request-ID", "{{$guid}}"),
        ("X-Correlation-ID", "{{$guid}}"),
        ("Accept", "application/json"),
    ):
        if key.lower() not in seen:
            out.append({"key": key, "value": val, "type": "text"})
            seen.add(key.lower())
    if (
        not is_login
        and method in ("POST", "PUT", "PATCH", "DELETE")
        and "idempotency-key" not in seen
    ):
        out.append({"key": "Idempotency-Key", "value": "{{$guid}}", "type": "text"})
    if requires_auth and "authorization" not in seen:
        out.append({"key": "Authorization", "value": "Bearer {{accessToken}}", "type": "text"})
    return out


def request_requires_auth(headers: list[dict]) -> bool:
    for h in headers or []:
        if h.get("disabled"):
            continue
        if (h.get("key") or "").lower() == "authorization":
            return True
    return False


def patch_request_item(item: dict) -> dict:
    req = item.get("request") or {}
    method = (req.get("method") or "GET").upper()
    name = item.get("name") or ""
    path_key = request_path_key(item).lower()
    is_gated = "[GATED-WRITE]" in name
    is_login = "/v1/auth/login" in path_key
    is_auth_me = "/v1/auth/me" in path_key
    requires_auth = request_requires_auth(req.get("header") or [])

    if isinstance(req.get("url"), str):
        raw = req["url"]
        if raw.startswith("{{baseUrl}}"):
            path_part = raw[len("{{baseUrl}}") :]
        else:
            path_part = raw
        segs = [s for s in path_part.strip("/").split("/") if s]
        req["url"] = {"raw": raw, "host": ["{{baseUrl}}"], "path": segs}

    req["header"] = patch_headers(
        req.get("header") or [],
        method,
        is_login=is_login,
        requires_auth=requires_auth,
    )
    if method in ("POST", "PUT", "PATCH", "DELETE") and not is_login:
        req.setdefault("header", [])
        has_ct = any((h.get("key") or "").lower() == "content-type" for h in req["header"])
        if not has_ct:
            req["header"].append({"key": "Content-Type", "value": "application/json", "type": "text"})

    events = list(item.get("event") or [])
    new_events = []
    has_prerequest = False
    has_test = False
    for ev in events:
        if ev.get("listen") == "prerequest":
            has_prerequest = True
            script = GATED_WRITE_PREREQUEST if is_gated else READONLY_PREREQUEST
            new_events.append({"listen": "prerequest", "script": {"type": "text/javascript", "exec": script}})
        elif ev.get("listen") == "test":
            has_test = True
            if is_login:
                exec_lines = list(LOGIN_TEST_SCRIPT)
            elif is_auth_me:
                exec_lines = list(AUTH_ME_TEST_SCRIPT)
            else:
                exec_lines = list((ev.get("script") or {}).get("exec") or [])
            new_events.append({"listen": "test", "script": {"type": "text/javascript", "exec": exec_lines}})
        else:
            new_events.append(ev)
    if not has_prerequest:
        script = GATED_WRITE_PREREQUEST if is_gated else READONLY_PREREQUEST
        new_events.insert(0, {"listen": "prerequest", "script": {"type": "text/javascript", "exec": script}})
    if not has_test:
        if is_login:
            test_script = LOGIN_TEST_SCRIPT
        elif is_auth_me:
            test_script = AUTH_ME_TEST_SCRIPT
        else:
            test_script = ["pm.test('Status not 500', function(){ pm.expect(pm.response.code).to.not.equal(500); });"]
        new_events.append(
            {
                "listen": "test",
                "script": {"type": "text/javascript", "exec": test_script},
            }
        )
    item["event"] = new_events
    item["request"] = req
    return item


PRODUCT_IMAGE_UPLOAD_TEST = [
    "const json = pm.response.json();",
    "",
    "pm.test('product image upload success', function () {",
    "  pm.expect([200, 201]).to.include(pm.response.code);",
    "});",
    "",
    "const mediaId = json.mediaId || json.id || (json.media && (json.media.mediaId || json.media.id));",
    "const displayUrl = json.displayUrl || json.url || (json.media && (json.media.displayUrl || json.media.url));",
    "const thumbnailUrl = json.thumbnailUrl || (json.media && json.media.thumbnailUrl);",
    "",
    "pm.test('media id exists', function () {",
    "  pm.expect(mediaId).to.be.a('string').and.not.empty;",
    "});",
    "",
    "pm.test('display url exists', function () {",
    "  pm.expect(displayUrl).to.be.a('string').and.not.empty;",
    "});",
    "",
    "pm.environment.set('mediaId', mediaId);",
    "pm.environment.set('productImageDisplayUrl', displayUrl);",
    "if (thumbnailUrl) pm.environment.set('productImageThumbnailUrl', thumbnailUrl);",
]


def patch_product_image_upload_item(item: dict) -> dict:
    key = request_path_key(item).lower()
    if "post /v1/admin/product-images" not in key:
        return item
    req = item.get("request") or {}
    headers = []
    for h in req.get("header") or []:
        kl = (h.get("key") or "").lower()
        if kl in ("content-type", "x-idempotency-key"):
            continue
        headers.append(h)
    req["header"] = headers
    req["body"] = {
        "mode": "formdata",
        "formdata": [
            {"key": "file", "type": "file", "src": [], "description": "Select local png/jpg/jpeg/webp/gif in Postman"},
            {"key": "purpose", "value": "product_image", "type": "text"},
            {"key": "altText", "value": "Sample product image", "type": "text"},
            {
                "key": "company_id",
                "value": "{{mediaCompanyId}}",
                "type": "text",
                "disabled": True,
                "description": "Optional legacy override only. Backend resolves MEDIA_COMPANY_ID server-side.",
            },
        ],
    }
    item["request"] = req
    item["disabled"] = False
    item["name"] = "[GATED-WRITE] POST /v1/admin/product-images (Cloudinary multipart)"
    events = list(item.get("event") or [])
    for ev in events:
        if ev.get("listen") == "test":
            ev["script"] = {"type": "text/javascript", "exec": list(PRODUCT_IMAGE_UPLOAD_TEST)}
    item["event"] = events
    item["response"] = [
        {
            "name": "201 Created (Cloudinary enabled)",
            "code": 201,
            "status": "Created",
            "_postman_previewlanguage": "json",
            "header": [{"key": "Content-Type", "value": "application/json"}],
            "body": json.dumps(
                {
                    "mediaId": "11111111-1111-1111-1111-111111111111",
                    "provider": "cloudinary",
                    "sourceType": "uploaded_file",
                    "status": "ready",
                    "filename": "product.png",
                    "contentType": "image/png",
                    "sizeBytes": 183421,
                    "checksum": "sha256:abc123",
                    "displayUrl": "https://res.cloudinary.com/demo/image/upload/v1/sample.png",
                    "thumbnailUrl": "https://res.cloudinary.com/demo/image/upload/c_fill,w_300,h_300/sample.png",
                    "version": 1,
                    "createdAt": "2026-01-01T00:00:00Z",
                },
                indent=2,
            ),
        },
        {
            "name": "503 capability_not_configured (Cloudinary disabled)",
            "code": 503,
            "status": "Service Unavailable",
            "_postman_previewlanguage": "json",
            "header": [{"key": "Content-Type", "value": "application/json"}],
            "body": json.dumps(
                {
                    "error": {
                        "code": "capability_not_configured",
                        "message": "product image upload is not configured for this process",
                        "details": {"capability": "v1.admin.media", "implemented": False},
                        "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV",
                    }
                },
                indent=2,
            ),
        },
    ]
    return item


def build_grpc_doc_items(grpc_rows: list[dict], templates: list[dict]) -> dict[str, list[dict]]:
    tmpl = {t["fullMethod"]: t for t in templates}
    by_domain: dict[str, list[dict]] = defaultdict(list)
    for row in grpc_rows:
        pkg = row.get("package", "")
        method = row.get("method", "")
        fm = row.get("fullMethod", "")
        t = tmpl.get(fm, {})
        domain = "Machine Runtime Config"
        if "telemetry" in pkg or "telemetry" in method.lower():
            domain = "Telemetry"
        elif "auth" in pkg or "token" in pkg or "bootstrap" in pkg:
            domain = "Auth"
        elif "catalog" in pkg or "media" in pkg:
            domain = "Product Media"
        elif "inventory" in pkg or "planogram" in pkg:
            domain = "Inventory"
        elif "commerce" in pkg or "payment" in pkg:
            domain = "Orders"
        elif "command" in pkg or "operator" in pkg:
            domain = "Utilities"
        elif "internal" in pkg:
            domain = "Admin Accounts RBAC"
        req_json = t.get("requestJsonTemplate") or {}
        desc = (
            "**gRPC manual test item** (Postman Desktop ΓåÆ New ΓåÆ gRPC Request)\n\n"
            "- **fullMethod:** `%s`\n"
            "- **Proto:** `%s`\n"
            "- **Host:** `{{grpcHost}}:{{grpcPort}}`\n\n"
            "**Request JSON:**\n```json\n%s\n```\n"
        ) % (fm, row.get("protoFile", ""), json.dumps(req_json, indent=2))
        by_domain[domain].append({"name": fm, "description": desc, "item": []})
    return by_domain


def build_mqtt_doc_items(mq_rows: list[dict]) -> dict[str, list[dict]]:
    by_domain: dict[str, list[dict]] = defaultdict(list)
    for row in mq_rows:
        topic = row.get("topicPattern") or row.get("topicConcrete") or ""
        t_lower = topic.lower()
        domain = "Telemetry"
        if "command" in t_lower:
            domain = "Utilities"
        elif "catalog" in t_lower or "media" in t_lower:
            domain = "Product Media"
        elif "heartbeat" in t_lower or "presence" in t_lower:
            domain = "Telemetry"
        payload = row.get("payloadJsonTemplate") or {}
        desc = (
            "**MQTT manual test item** (Postman Desktop ΓåÆ New ΓåÆ MQTT)\n\n"
            "- **Topic:** `%s`\n"
            "- **Direction:** %s\n"
            "- **Host:** `{{mqttHost}}:{{mqttPort}}`\n\n"
            "**Payload:**\n```json\n%s\n```\n"
        ) % (topic.replace("{{machineId}}", "{{machineId}}"), row.get("direction", ""), json.dumps(payload, indent=2))
        by_domain[domain].append({"name": topic, "description": desc, "item": []})
    return by_domain


def build_environment() -> dict:
    values = gfs.build_full100_environment_values()
    by_key = {v["key"]: v for v in values}
    overrides = {
        "baseUrl": "https://api.ldtv.dev",
        "grpcHost": "api.ldtv.dev",
        "grpcPort": "443",
        "mqttHost": "api.ldtv.dev",
        "mqttPort": "8883",
        "mqttTopicPrefix": "avf/prod",
        "adminEmail": "admin@ldtv.dev",
        "adminPassword": "",
        "accessToken": "",
        "refreshToken": "",
        "mqttPassword": "",
        "allowGatedWrites": "false",
        "confirmProductionWrites": "",
        "allow_destructive": "false",
        "canaryMode": "false",
        "readiness": "true",
        "cloudinaryEnabled": "true",
        "productImageFileNote": "Select file manually in Postman form-data",
        "accountId": "",
        "assortmentId": "",
    }
    secret_keys = {
        "adminPassword",
        "accessToken",
        "refreshToken",
        "mqttPassword",
        "machineToken",
        "paymentWebhookSecret",
        "webhookSecret",
    }
    ordered: list[dict] = []
    seen: set[str] = set()
    required_order = [
        "baseUrl",
        "adminEmail",
        "adminPassword",
        "accessToken",
        "refreshToken",
        "accountId",
        "allowGatedWrites",
        "confirmProductionWrites",
        "allow_destructive",
        "canaryMode",
        "readiness",
        "categoryId",
        "brandId",
        "tagId",
        "productId",
        "mediaId",
        "machineId",
        "siteId",
        "assortmentId",
        "planogramId",
        "slotId",
        "priceBookId",
        "grpcHost",
        "grpcPort",
        "mqttHost",
        "mqttPort",
        "mqttUsername",
        "mqttPassword",
        "mqttTopicPrefix",
    ]
    for k in required_order:
        v = overrides.get(k, by_key.get(k, {}).get("value", ""))
        if k in secret_keys and k not in ("adminEmail",):
            v = ""
        ordered.append({"key": k, "value": v, "type": "default", "enabled": True})
        seen.add(k)
    for v in values:
        k = v["key"]
        if k in seen:
            continue
        val = overrides.get(k, v.get("value", ""))
        if k in secret_keys:
            val = ""
        ordered.append({"key": k, "value": val, "type": "default", "enabled": True})
        seen.add(k)
    for k, val in overrides.items():
        if k not in seen:
            ordered.append({"key": k, "value": val if k not in secret_keys else "", "type": "default", "enabled": True})
    ordered.extend(
        [
            {"key": "_runtimeRequestId", "value": "", "type": "default", "enabled": True},
            {"key": "_runtimeCorrelationId", "value": "", "type": "default", "enabled": True},
            {"key": "_runtimeIdempotencyKey", "value": "", "type": "default", "enabled": True},
        ]
    )
    return {
        "id": hashlib.md5("avf-production-full-suite-env".encode()).hexdigest(),
        "name": "AVF Production",
        "values": ordered,
        "_postman_variable_scope": "environment",
    }


def add_media_upload_responses(item: dict) -> dict:
    key = request_path_key(item).lower()
    if "post /v1/admin/media/uploads/init" not in key:
        return item
    item["response"] = [
        {
            "name": "200 OK (object storage enabled)",
            "status": "OK",
            "code": 200,
            "_postman_previewlanguage": "json",
            "header": [{"key": "Content-Type", "value": "application/json"}],
            "body": json.dumps(
                {
                    "mediaId": "11111111-1111-1111-1111-111111111111",
                    "uploadUrl": "https://storage.example/upload",
                    "uploadMethod": "PUT",
                    "uploadHeaders": {},
                    "expiresAt": "2026-01-01T00:00:00Z",
                    "completePath": "/v1/admin/media/uploads/{mediaId}/complete",
                    "status": "pending_upload",
                },
                indent=2,
            ),
        },
        {
            "name": "503 capability_not_configured (object storage disabled)",
            "status": "Service Unavailable",
            "code": 503,
            "_postman_previewlanguage": "json",
            "header": [{"key": "Content-Type", "value": "application/json"}],
            "body": json.dumps(
                {
                    "error": {
                        "code": "capability_not_configured",
                        "message": "object storage media upload is not configured for this process",
                        "details": {"capability": "v1.admin.media.upload"},
                        "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV",
                    }
                },
                indent=2,
            ),
        },
    ]
    desc = item.get("request", {}).get("description") or ""
    item["request"]["description"] = (
        desc
        + "\n\n**Note:** Raw 404 is not expected. When object storage is disabled, expect **503** JSON `capability_not_configured`."
    )
    return item


def add_external_image_responses(item: dict) -> dict:
    key = request_path_key(item).lower()
    if "external-images" not in key:
        return item
    item["response"] = [
        {
            "name": "201 Created (feature enabled)",
            "code": 201,
            "status": "Created",
            "_postman_previewlanguage": "json",
            "header": [{"key": "Content-Type", "value": "application/json"}],
            "body": json.dumps(
                {
                    "mediaId": "11111111-1111-1111-1111-111111111111",
                    "sourceType": "external",
                    "url": "https://adm.avf.vn/storage/photos/1/Product/example.png",
                    "displayUrl": "https://adm.avf.vn/storage/photos/1/Product/example.png",
                    "cacheKey": "product-image:11111111-1111-1111-1111-111111111111",
                    "version": 1,
                    "status": "ready",
                },
                indent=2,
            ),
        },
        {
            "name": "503 capability_not_configured (feature disabled)",
            "code": 503,
            "status": "Service Unavailable",
            "_postman_previewlanguage": "json",
            "header": [{"key": "Content-Type", "value": "application/json"}],
            "body": json.dumps(
                {
                    "error": {
                        "code": "capability_not_configured",
                        "message": "external product image URLs are not configured for this process",
                        "details": {"capability": "v1.admin.media.external_images"},
                        "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV",
                    }
                },
                indent=2,
            ),
        },
    ]
    return item


def build_testing_guide(rest_count: int, grpc_count: int, mqtt_count: int) -> str:
    now = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    return f"""# AVF Production Full API Testing Guide

Generated: {now}

## 1. Import

1. Import `{COLLECTION_NAME}`
2. Import `{ENV_NAME}`
3. Select **AVF Production** environment
4. Set `adminPassword` locally (never commit)

## 2. Default environment flags (ready to run)

| Variable | Default |
| --- | --- |
| allowGatedWrites | true |
| confirmProductionWrites | I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION |
| allow_destructive | true |
| canaryMode | true |
| readiness | true |

## 3. Recommended run order

1. **System Health** ΓÇö `Health System` ΓåÆ REST ΓåÆ `/health/live`, `/health/ready`, `/version`
2. **Auth** ΓåÆ `POST /v1/auth/login` then `GET /v1/auth/me`
3. Domain REST folders (Catalog, Product Media, Products, Machines, ΓÇª)
4. **Product Media** ΓÇö `POST /v1/admin/product-images` (multipart file ΓåÆ Cloudinary; 201 or 503)
5. **Product Media** ΓÇö `POST /v1/admin/media/uploads/init` (200 or 503, not raw 404)
6. **Product Media** ΓÇö `POST /v1/admin/media/external-images` (201 or 503 if disabled)
7. Product create with `primaryMediaId`
7. Machine planogram / catalog assignment (canary)
8. gRPC / MQTT manual doc folders

## 4. Idempotency-Key

Write requests use `Idempotency-Key: {{{{$guid}}}}` directly.

## 5. Gated writes

`[GATED-WRITE]` requests require `allowGatedWrites=true` and `confirmProductionWrites=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION`.

## 6. Object storage vs external image URL

- **Upload init** requires object storage (`API_ARTIFACTS_ENABLED`) ΓÇö else **503** `capability_not_configured`
- **External image URL** requires `PRODUCT_IMAGE_EXTERNAL_URLS_ENABLED` ΓÇö else **503**

## 7. Troubleshooting

| Issue | Fix |
| --- | --- |
| invalid Authorization header | Re-run login; use `Bearer {{{{accessToken}}}}` |
| missing_idempotency_key | Ensure `Idempotency-Key: {{{{$guid}}}}` on writes |
| 401/403 | Check roles / token expiry |
| 404 route | Verify deploy version; media routes should return 401/503 not raw 404 |
| 503 capability_not_configured | Enable feature in production env or accept disabled capability |
| duplicate slug/SKU | Use `{{{{$timestamp}}}}` in slug/sku fields |

## 8. Coverage

- REST operations: {rest_count}
- gRPC methods (doc): {grpc_count}
- MQTT topics (doc): {mqtt_count}
"""


def main() -> int:
    if not SWAGGER.is_file():
        print("error: missing swagger at", SWAGGER, file=sys.stderr)
        return 1
    spec = json.loads(SWAGGER.read_text(encoding="utf-8"))
    rest_ops = gfs.iter_openapi_operations(spec)
    flat_coll, rest_count = gfs.build_rest_collection(
        spec,
        folder_assigner=assign_folder_business,
        folder_order_keys=FOLDER_ORDER,
        collection_title="AVF Production Full API Suite",
        collection_description=(
            "Production Postman suite grouped by module/domain.\n"
            "REST from OpenAPI; gRPC/MQTT are manual documentation folders."
        ),
        collection_id_seed="avf-production-full",
        tag_matrix_folder_name="__SKIP_TAG_MATRIX__",
    )
    all_items = flatten_requests(flat_coll.get("item") or [])
    if not all_items:
        fail_msg = "build_rest_collection returned 0 requests (rest_count=%s)" % rest_count
        print("error:", fail_msg, file=sys.stderr)
        return 1
    by_domain: dict[str, list[dict]] = defaultdict(list)
    for it in all_items:
        req = it.get("request") or {}
        url = req.get("url")
        raw = url.get("raw") if isinstance(url, dict) else str(url or "")
        path = raw.replace("{{baseUrl}}", "").split("?", 1)[0]
        method = (req.get("method") or "GET").upper()
        opid = (it.get("description") or "").replace("openapiOperationId: Doc\u200c", "").replace("openapiOperationId: ", "").strip()
        tags: list = []
        for row in rest_ops:
            if row["path"] == path and row["method"] == method:
                tags = row["op"].get("tags") or []
                break
        domain = assign_production_domain(path, method, tags)
        patched = patch_request_item(dict(it))
        patched = patch_product_image_upload_item(patched)
        patched = add_media_upload_responses(patched)
        patched = add_external_image_responses(patched)
        by_domain[domain].append(patched)

    grpc_rows = gfs.parse_all_protos()
    templates = gfs.build_grpc_templates(grpc_rows)
    grpc_by_domain = build_grpc_doc_items(grpc_rows, templates)
    mqtt_by_domain = build_mqtt_doc_items(gfs.fix_mqtt_rows())

    domain_items: list[dict] = []
    for domain in DOMAIN_ORDER:
        rest_items = sorted(by_domain.get(domain, []), key=lambda x: x.get("name") or "")
        if not rest_items and domain not in grpc_by_domain and domain not in mqtt_by_domain:
            continue
        sub: list[dict] = []
        if rest_items:
            sub.append(
                {
                    "name": "REST",
                    "description": "HTTP requests from OpenAPI (docs/swagger/swagger.json)",
                    "item": rest_items,
                }
            )
        grpc_items = grpc_by_domain.get(domain, [])
        if grpc_items:
            sub.append(
                {
                    "name": "gRPC",
                    "description": "Manual gRPC test items ΓÇö import protos from proto/avf",
                    "item": sorted(grpc_items, key=lambda x: x["name"]),
                }
            )
        mqtt_items = mqtt_by_domain.get(domain, [])
        if mqtt_items:
            sub.append(
                {
                    "name": "MQTT",
                    "description": "Manual MQTT test items ΓÇö see docs/api/mqtt-contract.md",
                    "item": sorted(mqtt_items, key=lambda x: x["name"])[:20],
                }
            )
        if sub:
            domain_items.append(
                {"name": domain, "description": "Module/domain: %s" % domain, "item": sub}
            )

    collection = {
        "info": {
            "_postman_id": hashlib.md5(b"avf-production-full").hexdigest()[:8]
            + "-2099-6125-e9af-b673355fbecb",
            "name": "AVF Production Full API Suite",
            "description": flat_coll["info"]["description"],
            "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
        },
        "event": flat_coll.get("event"),
        "variable": [
            {"key": "baseUrl", "value": "https://api.ldtv.dev"},
            {"key": "readiness", "value": "true"},
        ],
        "item": domain_items,
    }

    OUT_DIR.mkdir(parents=True, exist_ok=True)
    backup = REPO_ROOT / ".tmp-postman-backup" / "suites-production-full-before-regen"
    if OUT_DIR.is_dir():
        if backup.exists():
            shutil.rmtree(backup)
        shutil.copytree(OUT_DIR, backup)
    for stale in ("AVF_PRODUCTION_FULL_TESTING_GUIDE.md",):
        p = OUT_DIR / stale
        if p.is_file():
            p.unlink()

    (OUT_DIR / COLLECTION_NAME).write_text(json.dumps(collection, ensure_ascii=False, indent=2), encoding="utf-8")
    (OUT_DIR / ENV_NAME).write_text(json.dumps(build_environment(), ensure_ascii=False, indent=2), encoding="utf-8")
    (OUT_DIR / GUIDE_NAME).write_text(
        build_testing_guide(len(all_items), len(grpc_rows), len(gfs.fix_mqtt_rows())), encoding="utf-8"
    )

    print("wrote", OUT_DIR / COLLECTION_NAME, "requests=", len(all_items))
    print("wrote", OUT_DIR / ENV_NAME)
    print("wrote", OUT_DIR / GUIDE_NAME)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""Emit Postman v2.1 collection + environments under postman/collections/ and postman/environments/."""
from __future__ import annotations

import json
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[1]
POSTMAN_COLLECTIONS_DIR = ROOT / "postman" / "collections"
POSTMAN_ENVIRONMENTS_DIR = ROOT / "postman" / "environments"
POSTMAN_SCRIPTS_DIR = ROOT / "postman" / "scripts"


def load_exec(name: str) -> list[str]:
    p = POSTMAN_SCRIPTS_DIR / name
    return p.read_text(encoding="utf-8").splitlines()


def req_item(name: str, method: str, path: str, desc: str) -> dict:
    # path may be full after base e.g. /health/live or variable-relative
    return {
        "name": name,
        "request": {
            "method": method,
            "header": [],
            "url": path
            if path.startswith("{{")
            else "{{baseUrl}}" + path,
            "description": desc,
        },
    }


def req_item_post_json(name: str, path: str, desc: str, raw_body: str) -> dict:
    url = path if path.startswith("{{") else "{{baseUrl}}" + path
    return {
        "name": name,
        "request": {
            "method": "POST",
            "header": [],
            "body": {
                "mode": "raw",
                "raw": raw_body,
                "options": {"raw": {"language": "json"}},
            },
            "url": url,
            "description": desc,
        },
    }


def req_item_json_method(
    name: str,
    method: str,
    path: str,
    desc: str,
    raw_body: str | None = None,
    *,
    auth_noauth: bool = False,
    bearer_token_var: str | None = None,
    test_exec: list[str] | None = None,
) -> dict[str, Any]:
    url = path if path.startswith("{{") else "{{baseUrl}}" + path
    req_body: dict[str, Any] = {
        "method": method.upper(),
        "header": [],
        "url": url,
        "description": desc,
    }
    if raw_body is not None:
        req_body["body"] = {
            "mode": "raw",
            "raw": raw_body,
            "options": {"raw": {"language": "json"}},
        }
    if auth_noauth:
        req_body["auth"] = {"type": "noauth"}
    elif bearer_token_var:
        req_body["auth"] = {
            "type": "bearer",
            "bearer": [{"key": "token", "value": "{{" + bearer_token_var + "}}", "type": "string"}],
        }
    item: dict[str, Any] = {"name": name, "request": req_body}
    if test_exec:
        item["event"] = [
            {
                "listen": "test",
                "script": {"type": "text/javascript", "exec": test_exec},
            }
        ]
    return item


def integrated_product_media_offline_cache_folder() -> dict[str, Any]:
    """Phase 7 integrated REST flow: catalog + media + fleet + commerce + reporting (gRPC/MQTT adjacent)."""
    ts = "{{$timestamp}}"
    fingerprint = (
        '{\n  "androidId": "android-postman-1",\n  "serialNumber": "SN-POSTMAN-INTEGRATED",\n'
        '  "manufacturer": "AVF",\n  "model": "Lab",\n  "packageName": "com.avf.vending",\n'
        '  "versionName": "1.0.0",\n  "versionCode": 100\n}'
    )
    topology_body = (
        "{\n"
        '  "operator_session_id": "dddddddd-eeee-ffff-0000-111111111111",\n'
        '  "cabinets": [{"code": "A", "title": "Main cabinet", "sortOrder": 1, "metadata": {}}],\n'
        '  "layouts": [\n'
        "    {\n"
        '      "cabinetCode": "A",\n'
        '      "layoutKey": "grid-4x6",\n'
        '      "revision": 1,\n'
        '      "layoutSpec": {"rows": 4, "cols": 6},\n'
        '      "status": "active"\n'
        "    }\n"
        "  ]\n"
        "}"
    )
    planogram_body = (
        "{\n"
        '  "operator_session_id": "dddddddd-eeee-ffff-0000-111111111111",\n'
        '  "planogramId": "11111111-1111-1111-1111-111111111111",\n'
        '  "planogramRevision": 1,\n'
        '  "syncLegacyReadModel": true,\n'
        "  \"items\": [\n"
        "    {\n"
        '      "cabinetCode": "A",\n'
        '      "layoutKey": "grid-4x6",\n'
        '      "layoutRevision": 1,\n'
        '      "slotCode": "A3",\n'
        '      "legacySlotIndex": 3,\n'
        '      "productId": "{{productId}}",\n'
        '      "maxQuantity": 12,\n'
        '      "priceMinor": 150,\n'
        '      "metadata": {}\n'
        "    }\n"
        "  ]\n"
        "}"
    )
    sync_body = (
        "{\n"
        '  "operator_session_id": "dddddddd-eeee-ffff-0000-111111111111",\n'
        '  "reason": "catalog_media_offline_cache_suite"\n'
        "}"
    )
    commerce_order = (
        "{\n"
        '  "machine_id": "{{machineId}}",\n'
        '  "product_id": "{{productId}}",\n'
        '  "slot_index": 3,\n'
        '  "currency": "USD",\n'
        '  "subtotal_minor": 125,\n'
        '  "tax_minor": 10,\n'
        '  "total_minor": 135\n'
        "}"
    )
    payment_sess = (
        "{\n"
        '  "provider": "stripe",\n'
        '  "payment_state": "created",\n'
        '  "amount_minor": 135,\n'
        '  "currency": "USD",\n'
        '  "outbox_payload_json": {"source": "postman_integrated_flow"}\n'
        "}"
    )
    return {
        "name": "Integrated — product media offline cache",
        "description": (
            "**Execution order (REST in-folder; gRPC/MQTT adjacent scripts).** "
            "Set `adminEmail` / `adminPassword`, `auth_type=admin`, and run top-to-bottom. "
            "Native tests capture `accessToken`, IDs, `mediaId`, `machineToken`, `order_id`. "
            "Steps 19–23 use **machine JWT** (`bearer_token_var=machineToken`). "
            "After claim, optionally copy broker hints into `mqttHost` / `mqttPort` / `mqttTopicPrefix`. "
            "For catalog manifest RPCs use `grpcAddr` + `grpcUseReflection` with "
            "`postman/suites/full-production-suite/grpc/run-grpc-postman-adjacent.sh`. "
            "For `catalog.refresh` ACK use `mqtt/run-mqtt-postman-adjacent.sh` and matrix payloads.\n\n"
            "1 Login → 2 auth/me → 3–5 category/brand/tag → 6–7 media init/complete → 8–9 product + GET verify → "
            "10 site → 11 machine → 12 activation code → 13 claim (`machineToken`) → 14–16 topology/planogram/publish → "
            "17 sync → 18 inventory snapshot → 19–23 commerce + vend → 24 inventory verify → 25–26 reporting/audit → "
            "27 version stub (see FULL100 README for gRPC/MQTT)."
        ),
        "item": [
            req_item_json_method(
                "01 POST /v1/auth/login",
                "POST",
                "/v1/auth/login",
                "Login; tests stash tokens.accessToken into admin_token + accessToken.",
                '{\n  "email": "{{adminEmail}}",\n  "password": "{{adminPassword}}"\n}',
                auth_noauth=True,
            ),
            req_item_json_method(
                "02 GET /v1/auth/me",
                "GET",
                "/v1/auth/me",
                "Bearer admin session sanity check.",
                None,
            ),
            req_item_json_method(
                "03 POST /v1/admin/categories",
                "POST",
                "/v1/admin/categories",
                "Create category (OpenAPI-aligned example).",
                '{\n  "name": "Drinks ' + ts + '",\n  "slug": "drinks-' + ts + '",\n  "active": true\n}',
            ),
            req_item_json_method(
                "04 POST /v1/admin/brands",
                "POST",
                "/v1/admin/brands",
                "Create brand.",
                '{\n  "name": "Coca ' + ts + '",\n  "slug": "coca-' + ts + '",\n  "active": true\n}',
            ),
            req_item_json_method(
                "05 POST /v1/admin/tags",
                "POST",
                "/v1/admin/tags",
                "Create tag.",
                '{\n  "name": "Cold Drink ' + ts + '",\n  "slug": "cold-drink-' + ts + '",\n  "active": true\n}',
            ),
            req_item_json_method(
                "06 POST /v1/admin/media/uploads/init",
                "POST",
                "/v1/admin/media/uploads/init",
                "Init presigned upload; capture mediaId.",
                '{\n  "filename": "coca-330ml.png",\n  "contentType": "image/png",\n  "purpose": "product_image"\n}',
            ),
            req_item_json_method(
                "07 POST /v1/admin/media/uploads/{mediaId}/complete",
                "POST",
                "/v1/admin/media/uploads/{{mediaId}}/complete",
                "Complete upload (SHA-256 of staged bytes).",
                "{\n"
                '  "sizeBytes": 2048,\n'
                '  "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",\n'
                '  "contentType": "image/png"\n'
                "}",
            ),
            req_item_json_method(
                "08 POST /v1/admin/products (primaryMediaId + tagIds)",
                "POST",
                "/v1/admin/products",
                "Sellable product referencing completed media + tags.",
                "{\n"
                '  "sku": "COCA-330ML-' + ts + '",\n'
                '  "name": "Coca Cola Can 330ml",\n'
                '  "description": "Canary test product",\n'
                '  "categoryId": "{{categoryId}}",\n'
                '  "brandId": "{{brandId}}",\n'
                '  "tagIds": ["{{tagId}}"],\n'
                '  "primaryMediaId": "{{mediaId}}",\n'
                '  "status": "active",\n'
                '  "active": true,\n'
                '  "ageRestricted": false,\n'
                '  "allergenCodes": []\n'
                "}",
            ),
            req_item_json_method(
                "09 GET /v1/admin/products/{productId} (verify media + tags)",
                "GET",
                "/v1/admin/products/{{productId}}",
                "Assert primary media variants + tags for offline cache contracts.",
                None,
                test_exec=[
                    "pm.test('product media + tags', function () {",
                    "  pm.response.to.have.status(200);",
                    "  const j = pm.response.json();",
                    "  pm.expect(j.primaryMediaId).to.be.a('string');",
                    "  pm.expect(j.media && j.media.primary).to.be.an('object');",
                    "  pm.expect(j.media.primary.variants).to.be.an('array');",
                    "  pm.expect(j.media.primary.variants.length).to.be.above(0);",
                    "  const v0 = j.media.primary.variants[0];",
                    "  pm.expect(v0.sha256).to.match(/^[a-f0-9]{64}$/);",
                    "  pm.expect(v0).to.have.property('version');",
                    "  pm.expect(v0.downloadUrl).to.be.a('string');",
                    "  pm.expect(j.tags).to.be.an('array');",
                    "  const tagWant = pm.environment.get('tagId') || pm.collectionVariables.get('tag_id');",
                    "  const ids = (j.tags || []).map(function (t) { return t.id; });",
                    "  pm.expect(ids).to.include(tagWant);",
                    "});",
                ],
            ),
            req_item_json_method(
                "10 POST /v1/admin/sites",
                "POST",
                "/v1/admin/sites",
                "Create site (local POST gated: allow_destructive or canaryMode).",
                '{\n  "name": "Offline cache site ' + ts + '",\n  "timezone": "UTC",\n  "code": "PM-OC-' + ts + '",\n  "address": {}\n}',
            ),
            req_item_json_method(
                "11 POST /v1/admin/machines",
                "POST",
                "/v1/admin/machines",
                "Provision machine draft on site.",
                '{\n  "siteId": "{{siteId}}",\n  "serialNumber": "SN-PM-' + ts + '",\n'
                '  "code": "M-' + ts + '",\n  "name": "Offline cache machine",\n'
                '  "model": "AVF-1",\n  "cabinetType": "ambient",\n  "timezone": "UTC",\n  "status": "draft"\n}',
            ),
            req_item_json_method(
                "12 POST /v1/admin/machines/{machineId}/activation-codes",
                "POST",
                "/v1/admin/machines/{{machineId}}/activation-codes",
                "Mint activation code; captures activationCode.",
                '{\n  "expiresInMinutes": 1440,\n  "maxUses": 1,\n  "notes": "integrated offline-cache suite"\n}',
            ),
            req_item_json_method(
                "13 POST /v1/setup/activation-codes/claim",
                "POST",
                "/v1/setup/activation-codes/claim",
                "Claim code → machine JWT + MQTT hints.",
                '{\n  "activationCode": "{{activationCode}}",\n  "deviceFingerprint": ' + fingerprint + "\n}",
                auth_noauth=True,
            ),
            req_item_json_method(
                "14 PUT /v1/admin/machines/{machineId}/topology",
                "PUT",
                "/v1/admin/machines/{{machineId}}/topology",
                "Cabinet + layout topology.",
                topology_body,
            ),
            req_item_json_method(
                "15 PUT /v1/admin/machines/{machineId}/planograms/draft",
                "PUT",
                "/v1/admin/machines/{{machineId}}/planograms/draft",
                "Draft planogram binding slot A3 to productId.",
                planogram_body,
            ),
            req_item_json_method(
                "16 POST /v1/admin/machines/{machineId}/planograms/publish",
                "POST",
                "/v1/admin/machines/{{machineId}}/planograms/publish",
                "Publish draft → command envelope.",
                planogram_body,
            ),
            req_item_json_method(
                "17 POST /v1/admin/machines/{machineId}/sync",
                "POST",
                "/v1/admin/machines/{{machineId}}/sync",
                "Fleet sync nudge after publish.",
                sync_body,
            ),
            req_item_json_method(
                "18 GET /v1/admin/machines/{machineId}/inventory (before commerce)",
                "GET",
                "/v1/admin/machines/{{machineId}}/inventory",
                "Snapshot quantities before vend path.",
                None,
                test_exec=[
                    "pm.test('inventory before', function () {",
                    "  pm.response.to.have.status(200);",
                    "  pm.environment.set('inventorySnapshotBefore', pm.response.text());",
                    "});",
                ],
            ),
            req_item_json_method(
                "19 POST /v1/commerce/orders",
                "POST",
                "/v1/commerce/orders",
                "Create checkout order (machine bearer).",
                commerce_order,
                bearer_token_var="machineToken",
            ),
            req_item_json_method(
                "20 POST /v1/commerce/orders/{orderId}/payment-session",
                "POST",
                "/v1/commerce/orders/{{order_id}}/payment-session",
                "Payment session envelope.",
                payment_sess,
                bearer_token_var="machineToken",
            ),
            req_item_json_method(
                "21 POST /v1/commerce/orders/{orderId}/vend/start",
                "POST",
                "/v1/commerce/orders/{{order_id}}/vend/start",
                "Begin vend.",
                '{\n  "slot_index": 3\n}',
                bearer_token_var="machineToken",
            ),
            req_item_json_method(
                "22 POST /v1/commerce/orders/{orderId}/vend/success",
                "POST",
                "/v1/commerce/orders/{{order_id}}/vend/success",
                "Complete vend success.",
                '{\n  "slot_index": 3\n}',
                bearer_token_var="machineToken",
            ),
            req_item_json_method(
                "23 GET /v1/commerce/orders/{orderId}",
                "GET",
                "/v1/commerce/orders/{{order_id}}",
                "Order detail reconciliation read.",
                None,
                bearer_token_var="machineToken",
            ),
            req_item_json_method(
                "24 GET /v1/admin/machines/{machineId}/inventory (after commerce)",
                "GET",
                "/v1/admin/machines/{{machineId}}/inventory",
                "Compare against step 18 for decrement (manual or scripted diff).",
                None,
            ),
            req_item_json_method(
                "25 GET /v1/admin/reports/vends",
                "GET",
                "/v1/admin/reports/vends?limit=5",
                "Reporting spot-check (admin bearer).",
                None,
            ),
            req_item_json_method(
                "26 GET /v1/admin/audit/events",
                "GET",
                "/v1/admin/audit/events?limit=5",
                "Audit trail spot-check.",
                None,
            ),
            req_item_json_method(
                "27 GET /version (gRPC/MQTT adjunct)",
                "GET",
                "/version",
                "Stub step — run FULL100 grpc/mqtt scripts for MachineCatalogService/MachineMediaService + catalog.refresh ACK.",
                None,
                auth_noauth=True,
            ),
        ],
    }


def main() -> None:
    POSTMAN_COLLECTIONS_DIR.mkdir(parents=True, exist_ok=True)
    POSTMAN_ENVIRONMENTS_DIR.mkdir(parents=True, exist_ok=True)
    pre = load_exec("collection_prerequest.js")
    post = load_exec("collection_test.js")
    events = [
        {
            "listen": "prerequest",
            "script": {"type": "text/javascript", "exec": pre},
        },
        {
            "listen": "test",
            "script": {"type": "text/javascript", "exec": post},
        },
    ]
    collection = {
        "info": {
            "_postman_id": "avf-vending-api-collection",
            "name": "AVF Vending API",
            "description": "Native Postman collection (not a replacement for OpenAPI). Import OpenAPI from {{swagger_url}} or use this collection with variables. Production writes are blocked unless unlock variables are set.",
            "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
        },
        "auth": {
            "type": "bearer",
            "bearer": [{"key": "token", "value": "{{active_token}}", "type": "string"}],
        },
        "event": events,
        "variable": [
            {"key": "api_prefix", "value": "/v1"},
            {"key": "active_token", "value": ""},
            {"key": "auth_type", "value": "public"},
            {"key": "admin_token", "value": ""},
            {"key": "machine_token", "value": ""},
            {"key": "x_request_id", "value": ""},
            {"key": "x_correlation_id", "value": ""},
            {"key": "idempotency_key", "value": ""},
            {"key": "resource_uuid", "value": ""},
            {"key": "event_id", "value": ""},
            {"key": "event_time", "value": ""},
            {"key": "now_iso", "value": ""},
            {"key": "activation_code", "value": ""},
            {"key": "site_id", "value": ""},
            {"key": "machine_id", "value": ""},
            {"key": "cabinet_id", "value": ""},
            {"key": "slot_id", "value": ""},
            {"key": "product_id", "value": ""},
            {"key": "sku", "value": "COCA330"},
            {"key": "category_id", "value": ""},
            {"key": "brand_id", "value": ""},
            {"key": "tag_id", "value": ""},
            {"key": "flow_sku", "value": ""},
            {"key": "flow_product_name", "value": ""},
            {"key": "admin_email", "value": ""},
            {"key": "admin_password", "value": ""},
            {"key": "order_id", "value": ""},
            {"key": "payment_id", "value": ""},
            {"key": "vend_id", "value": ""},
            {"key": "refund_id", "value": ""},
        ],
        "item": [
            {
                "name": "Public",
                "item": [
                    req_item("GET /health/live", "GET", "/health/live", "Liveness"),
                    req_item("GET /health/ready", "GET", "/health/ready", "Readiness"),
                    req_item("GET /version", "GET", "/version", "Build metadata"),
                    req_item("GET /swagger/doc.json", "GET", "/swagger/doc.json", "OpenAPI 3.0 JSON"),
                ],
            },
            {
                "name": "Admin sites (read)",
                "item": [
                    req_item(
                        "GET /v1/admin/sites",
                        "GET",
                        "/v1/admin/sites",
                        "List company sites. Set auth_type=admin and a valid admin_token on the environment.",
                    ),
                ],
            },
            {
                "name": "Canary admin writes",
                "item": [
                    req_item_post_json(
                        "POST /v1/admin/sites",
                        "/v1/admin/sites",
                        "Create company site. Local/dev POST is gated: set allow_destructive=true "
                        "or canaryMode=true. Request URL is exactly {{baseUrl}}/v1/admin/sites with no "
                        "query parameters. Requires auth_type=admin and admin_token.",
                        '{\n  "name": "Postman Canary Site",\n  "code": "PM-{{$timestamp}}",\n'
                        '  "timezone": "UTC",\n  "address": {}\n}',
                    ),
                ],
            },
            integrated_product_media_offline_cache_folder(),
        ],
    }
    out = POSTMAN_COLLECTIONS_DIR / "avf-vending-api.postman_collection.json"
    out.write_text(json.dumps(collection, indent=2) + "\n", encoding="utf-8", newline="\n")
    print(f"Wrote {out}")
    fn_path = POSTMAN_COLLECTIONS_DIR / "avf-vending-api-function-path.postman_collection.json"
    fn_path.write_text(out.read_text(encoding="utf-8"), encoding="utf-8", newline="\n")
    print(f"Wrote {fn_path}")

    def env_file(
        name: str,
        display: str,
        values: list[tuple[str, str, bool]],
    ) -> None:
        p = POSTMAN_ENVIRONMENTS_DIR / name
        payload = {
            "id": f"avf-env-{name.replace('.postman_environment.json', '')}",
            "name": display,
            "values": [
                {
                    "key": k,
                    "value": v,
                    "type": "default",
                    "enabled": en,
                }
                for k, v, en in values
            ],
        }
        p.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8", newline="\n")
        print(f"Wrote {p}")

    env_file(
        "avf-local.postman_environment.json",
        "AVF Local",
        [
            ("env_name", "local", True),
            ("app_env", "development", True),
            ("auth_type", "public", True),
            ("base_url", "http://localhost:8080", True),
            ("baseUrl", "http://localhost:8080", True),
            ("api_prefix", "/v1", True),
            ("swagger_url", "http://localhost:8080/swagger/doc.json", True),
            ("swagger_enabled", "true", True),
            ("payment_env", "sandbox", True),
            ("mqtt_topic_prefix", "avf-dev/devices", True),
            ("allow_mutation", "true", True),
            ("allow_production_mutation", "false", True),
            ("confirm_production_run", "", True),
            ("allow_destructive", "false", True),
            ("canaryMode", "false", True),
            ("admin_token", "", True),
            ("machine_token", "", True),
            ("activation_code", "", True),
            ("site_id", "", True),
            ("machine_id", "", True),
            ("cabinet_id", "", True),
            ("slot_id", "", True),
            ("product_id", "", True),
            ("sku", "COCA330", True),
            ("category_id", "", True),
            ("brand_id", "", True),
            ("tag_id", "", True),
            ("categoryId", "", True),
            ("brandId", "", True),
            ("tagId", "", True),
            ("mediaId", "", True),
            ("productId", "", True),
            ("siteId", "", True),
            ("machineId", "", True),
            ("machineToken", "", True),
            ("activationCode", "", True),
            ("catalogVersion", "", True),
            ("mediaManifestVersion", "", True),
            ("mqttHost", "localhost", True),
            ("mqttPort", "1883", True),
            ("mqttTopicPrefix", "avf-dev/devices", True),
            ("grpcAddr", "localhost:9090", True),
            ("grpcUseReflection", "true", True),
            ("flow_sku", "", True),
            ("flow_product_name", "", True),
            ("admin_email", "", True),
            ("admin_password", "", True),
            ("adminEmail", "", True),
            ("adminPassword", "", True),
            ("accessToken", "", True),
            ("refreshToken", "", True),
            ("order_id", "", True),
            ("payment_id", "", True),
            ("vend_id", "", True),
            ("refund_id", "", True),
        ],
    )
    env_file(
        "avf-staging.postman_environment.json",
        "AVF Staging",
        [
            ("env_name", "staging", True),
            ("app_env", "staging", True),
            ("auth_type", "public", True),
            ("base_url", "https://staging-api.ldtv.dev", True),
            ("baseUrl", "https://staging-api.ldtv.dev", True),
            ("api_prefix", "/v1", True),
            ("swagger_url", "https://staging-api.ldtv.dev/swagger/doc.json", True),
            ("swagger_enabled", "true", True),
            ("payment_env", "sandbox", True),
            ("mqtt_topic_prefix", "avf-staging/devices", True),
            ("allow_mutation", "true", True),
            ("allow_production_mutation", "false", True),
            ("confirm_production_run", "", True),
            ("allow_destructive", "false", True),
            ("canaryMode", "false", True),
            ("admin_token", "", True),
            ("machine_token", "", True),
            ("activation_code", "", True),
            ("site_id", "", True),
            ("machine_id", "", True),
            ("cabinet_id", "", True),
            ("slot_id", "", True),
            ("product_id", "", True),
            ("sku", "COCA330", True),
            ("category_id", "", True),
            ("brand_id", "", True),
            ("tag_id", "", True),
            ("categoryId", "", True),
            ("brandId", "", True),
            ("tagId", "", True),
            ("mediaId", "", True),
            ("productId", "", True),
            ("siteId", "", True),
            ("machineId", "", True),
            ("machineToken", "", True),
            ("activationCode", "", True),
            ("catalogVersion", "", True),
            ("mediaManifestVersion", "", True),
            ("mqttHost", "", True),
            ("mqttPort", "8883", True),
            ("mqttTopicPrefix", "avf-staging/devices", True),
            ("grpcAddr", "", True),
            ("grpcUseReflection", "false", True),
            ("flow_sku", "", True),
            ("flow_product_name", "", True),
            ("admin_email", "", True),
            ("admin_password", "", True),
            ("adminEmail", "", True),
            ("adminPassword", "", True),
            ("accessToken", "", True),
            ("refreshToken", "", True),
            ("order_id", "", True),
            ("payment_id", "", True),
            ("vend_id", "", True),
            ("refund_id", "", True),
        ],
    )
    env_file(
        "avf-production.postman_environment.json",
        "AVF Production",
        [
            ("env_name", "production", True),
            ("app_env", "production", True),
            ("auth_type", "public", True),
            ("base_url", "https://api.ldtv.dev", True),
            ("baseUrl", "https://api.ldtv.dev", True),
            ("api_prefix", "/v1", True),
            ("swagger_url", "https://api.ldtv.dev/swagger/doc.json", True),
            ("swagger_enabled", "false", True),
            ("payment_env", "live", True),
            ("mqtt_topic_prefix", "avf/devices", True),
            ("allow_mutation", "false", True),
            ("allow_production_mutation", "false", True),
            ("confirm_production_run", "", True),
            ("allow_destructive", "false", True),
            ("canaryMode", "false", True),
            ("admin_token", "", True),
            ("machine_token", "", True),
            ("activation_code", "", True),
            ("site_id", "", True),
            ("machine_id", "", True),
            ("cabinet_id", "", True),
            ("slot_id", "", True),
            ("product_id", "", True),
            ("sku", "", True),
            ("category_id", "", True),
            ("brand_id", "", True),
            ("tag_id", "", True),
            ("categoryId", "", True),
            ("brandId", "", True),
            ("tagId", "", True),
            ("mediaId", "", True),
            ("productId", "", True),
            ("siteId", "", True),
            ("machineId", "", True),
            ("machineToken", "", True),
            ("activationCode", "", True),
            ("catalogVersion", "", True),
            ("mediaManifestVersion", "", True),
            ("mqttHost", "", True),
            ("mqttPort", "8883", True),
            ("mqttTopicPrefix", "avf/devices", True),
            ("grpcAddr", "", True),
            ("grpcUseReflection", "false", True),
            ("flow_sku", "", True),
            ("flow_product_name", "", True),
            ("admin_email", "", True),
            ("admin_password", "", True),
            ("adminEmail", "", True),
            ("adminPassword", "", True),
            ("accessToken", "", True),
            ("refreshToken", "", True),
            ("order_id", "", True),
            ("payment_id", "", True),
            ("vend_id", "", True),
            ("refund_id", "", True),
        ],
    )


if __name__ == "__main__":
    main()

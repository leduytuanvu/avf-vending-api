#!/usr/bin/env python3
"""Emit Postman v2.1 collection + environments under docs/postman/."""
from __future__ import annotations

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
POSTMAN_DIR = ROOT / "docs" / "postman"
TOOLS_POSTMAN = ROOT / "tools" / "postman"


def load_exec(name: str) -> list[str]:
    p = TOOLS_POSTMAN / name
    return p.read_text(encoding="utf-8").splitlines()


def req_item(name: str, method: str, path: str, desc: str) -> dict:
    # path may be full after base e.g. /health/live or {{base_url}}-relative
    return {
        "name": name,
        "request": {
            "method": method,
            "header": [],
            "url": path
            if path.startswith("{{")
            else "{{base_url}}" + path,
            "description": desc,
        },
    }


def main() -> None:
    POSTMAN_DIR.mkdir(parents=True, exist_ok=True)
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
            {"key": "event_id", "value": ""},
            {"key": "event_time", "value": ""},
            {"key": "now_iso", "value": ""},
            {"key": "activation_code", "value": ""},
            {"key": "organization_id", "value": ""},
            {"key": "site_id", "value": ""},
            {"key": "machine_id", "value": ""},
            {"key": "cabinet_id", "value": ""},
            {"key": "slot_id", "value": ""},
            {"key": "product_id", "value": ""},
            {"key": "sku", "value": "COCA330"},
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
        ],
    }
    out = POSTMAN_DIR / "avf-vending-api.postman_collection.json"
    out.write_text(json.dumps(collection, indent=2) + "\n", encoding="utf-8")
    print(f"Wrote {out}")

    def env_file(
        name: str,
        display: str,
        values: list[tuple[str, str, bool]],
    ) -> None:
        p = POSTMAN_DIR / name
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
        p.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
        print(f"Wrote {p}")

    env_file(
        "avf-local.postman_environment.json",
        "AVF Local",
        [
            ("env_name", "local", True),
            ("app_env", "development", True),
            ("auth_type", "public", True),
            ("base_url", "http://localhost:8080", True),
            ("api_prefix", "/v1", True),
            ("swagger_url", "http://localhost:8080/swagger/doc.json", True),
            ("swagger_enabled", "true", True),
            ("payment_env", "sandbox", True),
            ("mqtt_topic_prefix", "avf-dev/devices", True),
            ("allow_mutation", "true", True),
            ("allow_production_mutation", "false", True),
            ("confirm_production_run", "", True),
            ("admin_token", "", True),
            ("machine_token", "", True),
            ("activation_code", "", True),
            ("organization_id", "", True),
            ("site_id", "", True),
            ("machine_id", "", True),
            ("cabinet_id", "", True),
            ("slot_id", "", True),
            ("product_id", "", True),
            ("sku", "COCA330", True),
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
            ("api_prefix", "/v1", True),
            ("swagger_url", "https://staging-api.ldtv.dev/swagger/doc.json", True),
            ("swagger_enabled", "true", True),
            ("payment_env", "sandbox", True),
            ("mqtt_topic_prefix", "avf-staging/devices", True),
            ("allow_mutation", "true", True),
            ("allow_production_mutation", "false", True),
            ("confirm_production_run", "", True),
            ("admin_token", "", True),
            ("machine_token", "", True),
            ("activation_code", "", True),
            ("organization_id", "", True),
            ("site_id", "", True),
            ("machine_id", "", True),
            ("cabinet_id", "", True),
            ("slot_id", "", True),
            ("product_id", "", True),
            ("sku", "COCA330", True),
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
            ("api_prefix", "/v1", True),
            ("swagger_url", "https://api.ldtv.dev/swagger/doc.json", True),
            ("swagger_enabled", "false", True),
            ("payment_env", "live", True),
            ("mqtt_topic_prefix", "avf/devices", True),
            ("allow_mutation", "false", True),
            ("allow_production_mutation", "false", True),
            ("confirm_production_run", "", True),
            ("admin_token", "", True),
            ("machine_token", "", True),
            ("activation_code", "", True),
            ("organization_id", "", True),
            ("site_id", "", True),
            ("machine_id", "", True),
            ("cabinet_id", "", True),
            ("slot_id", "", True),
            ("product_id", "", True),
            ("sku", "", True),
            ("order_id", "", True),
            ("payment_id", "", True),
            ("vend_id", "", True),
            ("refund_id", "", True),
        ],
    )


if __name__ == "__main__":
    main()

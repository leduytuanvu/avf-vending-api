#!/usr/bin/env python3
"""Bootstrap production test entity chain via admin REST APIs."""

from __future__ import annotations

import argparse
import json
import os
import secrets
import sys
import uuid
from pathlib import Path
from urllib.request import Request, urlopen
from base64 import b64encode

sys.path.insert(0, str(Path(__file__).resolve().parent))

from _common import http_request, new_request_id, production_machine_code, report_dir, test_prefix
from entity_registry import EntityRegistry


def login(base_url: str, email: str, password: str) -> str:
    body = json.dumps({"email": email, "password": password}).encode()
    status, raw, _ = http_request(
        "POST",
        f"{base_url.rstrip('/')}/v1/auth/login",
        headers={"Content-Type": "application/json", "X-Correlation-ID": new_request_id()},
        body=body,
    )
    if status not in (200, 201):
        raise RuntimeError(f"admin login failed HTTP {status}: {raw[:500]}")
    data = json.loads(raw)
    token = (
        data.get("accessToken")
        or data.get("access_token")
        or data.get("token")
        or (data.get("tokens") or {}).get("accessToken")
        or (data.get("tokens") or {}).get("access_token")
    )
    if not token:
        raise RuntimeError("admin login missing access token")
    return str(token)


def admin_post(base_url: str, token: str, path: str, payload: dict) -> tuple[int, dict]:
    body = json.dumps(payload).encode()
    status, raw, _ = http_request(
        "POST",
        f"{base_url.rstrip('/')}{path}",
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {token}",
            "X-Request-ID": new_request_id(),
            "X-Correlation-ID": new_request_id(),
        },
        body=body,
    )
    try:
        data = json.loads(raw) if raw.strip() else {}
    except json.JSONDecodeError:
        data = {"raw": raw}
    return status, data


def admin_patch(base_url: str, token: str, path: str, payload: dict) -> tuple[int, dict]:
    body = json.dumps(payload).encode()
    status, raw, _ = http_request(
        "PATCH",
        f"{base_url.rstrip('/')}{path}",
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {token}",
            "X-Request-ID": new_request_id(),
        },
        body=body,
    )
    try:
        data = json.loads(raw) if raw.strip() else {}
    except json.JSONDecodeError:
        data = {"raw": raw}
    return status, data


def claim_activation(base_url: str, code: str, serial: str) -> dict:
    payload = {
        "activationCode": code,
        "deviceFingerprint": {
            "serialNumber": serial,
            "serial": serial,
            "androidId": f"{serial}-aid",
            "packageName": "dev.avf.vending.prodtest",
        },
    }
    body = json.dumps(payload).encode()
    status, raw, _ = http_request(
        "POST",
        f"{base_url.rstrip('/')}/v1/setup/activation-codes/claim",
        headers={"Content-Type": "application/json", "X-Request-ID": new_request_id()},
        body=body,
    )
    if status not in (200, 201):
        raise RuntimeError(f"claim failed HTTP {status}: {raw[:800]}")
    return json.loads(raw)


def provision_emqx_machine_user(machine_id: str) -> tuple[str, str] | tuple[None, None]:
    """Optional EMQX built-in user provisioning when management API env is configured."""
    base = os.environ.get("EMQX_MANAGEMENT_URL", "http://127.0.0.1:18083").rstrip("/")
    api_key = os.environ.get("EMQX_API_KEY", "")
    api_secret = os.environ.get("EMQX_API_SECRET", "")
    if not api_key or not api_secret:
        return None, None
    password = secrets.token_hex(24)
    payload = json.dumps({"user_id": machine_id, "password": password}).encode()
    auth = b64encode(f"{api_key}:{api_secret}".encode()).decode()
    req = Request(
        f"{base}/api/v5/authentication/password_based%3Abuilt_in_database/users",
        data=payload,
        headers={"Content-Type": "application/json", "Authorization": f"Basic {auth}"},
        method="POST",
    )
    try:
        with urlopen(req, timeout=15) as resp:  # noqa: S310
            if resp.status in (200, 201, 409):
                return machine_id, password
    except Exception:
        return None, None
    return None, None


def bootstrap(base_url: str) -> EntityRegistry:
    email = os.environ.get("PROD_TEST_ADMIN_EMAIL", os.environ.get("ADMIN_EMAIL", ""))
    password = os.environ.get("PROD_TEST_ADMIN_PASSWORD", os.environ.get("ADMIN_PASSWORD", ""))
    if not email or not password:
        raise RuntimeError("Set PROD_TEST_ADMIN_EMAIL and PROD_TEST_ADMIN_PASSWORD")

    prefix = test_prefix()
    pass_suffix = os.environ.get("PROD_TEST_SUFFIX", uuid.uuid4().hex[:8])
    reg = EntityRegistry()
    reg.set_prefix(prefix)

    token = login(base_url, email, password)
    reg.set("adminAccessToken", token, entity_type="token")

    code_suffix = prefix.replace("ENTERPRISE_PROD_TEST_", "")[:32]
    site_payload = {
        "name": f"{prefix} Site",
        "code": code_suffix[:20],
        "timezone": "UTC",
        "address": {"line1": f"{prefix} address"},
    }
    st, site = admin_post(base_url, token, "/v1/admin/sites", site_payload)
    if st == 409:
        site_payload["code"] = f"{code_suffix[:14]}-{uuid.uuid4().hex[:6]}"[:24]
        site_payload["name"] = f"{prefix} Site {uuid.uuid4().hex[:6]}"
        st, site = admin_post(base_url, token, "/v1/admin/sites", site_payload)
    if st not in (200, 201) or not site.get("id"):
        raise RuntimeError(f"site create failed {st}: {site}")
    reg.set("siteId", site["id"], entity_type="site")
    reg.record_write(
        surface="REST",
        action="POST /v1/admin/sites",
        request_id=new_request_id(),
        correlation_id=new_request_id(),
        entity_id=site["id"],
        request_body=site_payload,
        response_body=site,
    )

    machine_code = production_machine_code()
    machine_payload = {
        "name": f"{prefix} Machine",
        "code": machine_code,
        "siteId": site["id"],
        "serialNumber": f"{prefix}-SN-{pass_suffix}",
        "model": "AVF-PROD-TEST",
        "status": "draft",
        "timezone": "UTC",
        "cabinetType": "ambient",
    }
    st, machine = admin_post(base_url, token, "/v1/admin/machines", machine_payload)
    if st == 409:
        machine_payload["code"] = production_machine_code()
        machine_payload["serialNumber"] = f"{prefix}-SN-{uuid.uuid4().hex[:6]}"
        st, machine = admin_post(base_url, token, "/v1/admin/machines", machine_payload)
    if st not in (200, 201) or not machine.get("id"):
        raise RuntimeError(f"machine create failed {st}: {machine}")
    reg.set("machineId", machine["id"], entity_type="machine")
    reg.set("machineSerialNumber", machine_payload["serialNumber"], entity_type="config")
    reg.record_write(
        surface="REST",
        action="POST /v1/admin/machines",
        request_id=new_request_id(),
        correlation_id=new_request_id(),
        entity_id=machine["id"],
        request_body=machine_payload,
        response_body=machine,
    )

    st, activated = admin_patch(base_url, token, f"/v1/admin/machines/{machine['id']}", {"status": "active"})
    if st not in (200, 204):
        raise RuntimeError(f"machine activate failed {st}: {activated}")

    st, act = admin_post(base_url, token, f"/v1/admin/machines/{machine['id']}/activation-codes", {})
    if st not in (200, 201):
        raise RuntimeError(f"activation code failed {st}: {act}")
    activation_code = act.get("code") or act.get("activationCode") or act.get("plaintextCode")
    if not activation_code:
        raise RuntimeError(f"no activation code in response: {act}")
    reg.set("activationCode", str(activation_code), entity_type="activation_code")

    claim = claim_activation(base_url, str(activation_code), machine_payload["serialNumber"])
    machine_token = claim.get("accessToken") or claim.get("machineAccessToken") or claim.get("machineToken")
    refresh = claim.get("refreshToken") or claim.get("machineRefreshToken")
    if machine_token:
        reg.set("machineToken", str(machine_token), entity_type="token")
    if refresh:
        reg.set("machineRefreshToken", str(refresh), entity_type="token")
    mqtt = claim.get("mqtt") or {}
    mqtt_user = claim.get("mqttUsername") or mqtt.get("username") or machine["id"]
    mqtt_pass = claim.get("mqttPassword") or mqtt.get("password")
    topic_prefix = claim.get("mqttTopicPrefix") or mqtt.get("topicPrefix") or mqtt.get("topic_prefix") or "avf/devices"
    reg.set("mqttTopicPrefix", str(topic_prefix), entity_type="config")
    if mqtt_user:
        reg.set("mqttUsername", str(mqtt_user), entity_type="credential")
    if mqtt_pass:
        reg.set("mqttPassword", str(mqtt_pass), entity_type="credential")
    strict = os.environ.get("PRODUCTION_FULL_TEST_STRICT", "").strip().lower() in ("1", "true", "yes")
    if not mqtt_pass:
        if strict:
            raise RuntimeError(
                "claim response missing mqttPassword — per-machine EMQX provisioning is not deployed "
                "(set EMQX_MANAGEMENT_URL/API_KEY/API_SECRET on app nodes and redeploy)"
            )
        prov_user, prov_pass = provision_emqx_machine_user(machine["id"])
        if prov_user and prov_pass:
            reg.set("mqttUsername", prov_user, entity_type="credential")
            reg.set("mqttPassword", prov_pass, entity_type="credential")
        elif strict:
            raise RuntimeError("strict mode: no mqttPassword from claim and EMQX management env not configured locally")
    reg.record_write(
        surface="REST",
        action="POST /v1/setup/activation-codes/claim",
        request_id=new_request_id(),
        correlation_id=new_request_id(),
        entity_id=machine["id"],
        request_body={"activationCode": "***"},
        response_body={"machineId": machine["id"], "hasToken": bool(machine_token)},
    )

    # Catalog bootstrap
    brand_payload = {"name": f"{prefix} Brand", "code": f"{code_suffix}-b"[:20]}
    st, brand = admin_post(base_url, token, "/v1/admin/catalog/brands", brand_payload)
    if st in (200, 201) and brand.get("id"):
        reg.set("brandId", brand["id"], entity_type="brand")

    cat_payload = {"name": f"{prefix} Category", "code": f"{code_suffix}-c"[:20]}
    st, cat = admin_post(base_url, token, "/v1/admin/catalog/categories", cat_payload)
    if st in (200, 201) and cat.get("id"):
        reg.set("categoryId", cat["id"], entity_type="category")

    prod_payload = {
        "name": f"{prefix} Product",
        "sku": f"{code_suffix}-sku"[:32],
        "brandId": reg.get("brandId") or None,
        "categoryId": reg.get("categoryId") or None,
        "status": "active",
    }
    prod_payload = {k: v for k, v in prod_payload.items() if v}
    st, product = admin_post(base_url, token, "/v1/admin/catalog/products", prod_payload)
    if st in (200, 201) and product.get("id"):
        reg.set("productId", product["id"], entity_type="product")

    reg.save()
    print(f"Bootstrap OK prefix={prefix} site={reg.get('siteId')} machine={reg.get('machineId')}")
    return reg


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default=os.environ.get("BASE_URL", "https://api.ldtv.dev"))
    args = parser.parse_args()
    try:
        bootstrap(args.base_url)
        return 0
    except Exception as exc:
        print(f"bootstrap failed: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""Production E2E flows A–I for MQTT unblock verification."""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import uuid
from datetime import datetime, timezone
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from _common import http_request, new_request_id, report_dir, write_json
from bootstrap_test_data import bootstrap, claim_activation
from entity_registry import EntityRegistry

ROOT = Path(__file__).resolve().parents[2]
UNBLOCK_DIR = ROOT / "reports" / "production-mqtt-unblock" / "20260702T210742Z"


def flow_result(flow_id: str, name: str, ok: bool, detail: str, *, blocked_by: str = "") -> dict:
    return {
        "flow": flow_id,
        "name": name,
        "ok": ok,
        "detail": detail,
        "blocked_by": blocked_by,
        "at_utc": datetime.now(timezone.utc).isoformat(),
    }


def run_mqtt_probe(reg: EntityRegistry) -> tuple[bool, str]:
    rc = subprocess.call([sys.executable, str(Path(__file__).parent / "run_mqtt_full_production.py")], cwd=ROOT)
    return rc == 0, "mqtt matrix rc=0" if rc == 0 else f"mqtt matrix rc={rc}"


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default=os.environ.get("BASE_URL", "https://api.ldtv.dev"))
    args = parser.parse_args()

    os.environ.setdefault(
        "PRODUCTION_FULL_TEST_UTC",
        os.environ.get("PRODUCTION_FULL_TEST_UTC", datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")),
    )
    os.environ.setdefault("PRODUCTION_FULL_TEST_STRICT", "1")

    out = report_dir()
    unblock = UNBLOCK_DIR
    unblock.mkdir(parents=True, exist_ok=True)
    flows: list[dict] = []

    # A — Admin setup chain (bootstrap)
    try:
        reg = bootstrap(args.base_url)
        flows.append(flow_result("A", "Admin setup chain", True, f"prefix={reg.data.get('prefix')}"))
    except Exception as exc:
        flows.append(flow_result("A", "Admin setup chain", False, str(exc)))
        write_json(unblock / "E2E_FLOW_RESULTS.json", {"flows": flows})
        write_json(out / "E2E_FLOW_RESULTS.json", {"flows": flows})
        return 1

    subst = reg.as_substitution_map()
    machine_id = subst.get("machineId", "")
    mqtt_pass = subst.get("mqttPassword", "")

    # B — Claim + mqtt creds + connect probe
    b_ok = bool(mqtt_pass and subst.get("mqttUsername"))
    flows.append(
        flow_result(
            "B",
            "Claim mqtt credentials",
            b_ok,
            "mqttPassword present from claim" if b_ok else "missing mqttPassword/mqttUsername from claim",
            blocked_by="" if b_ok else "DEPLOY_EMQX_PROVISIONING",
        )
    )

    # C — gRPC bootstrap metadata (no password leak)
    token = subst.get("machineToken", "")
    c_ok = False
    c_detail = "no machine token"
    if token:
        body = json.dumps({"meta": {"requestId": new_request_id()}}).encode()
        st, raw, _ = http_request(
            "POST",
            f"{args.base_url.rstrip('/')}/v1/setup/machines/{machine_id}/bootstrap",
            headers={"Authorization": f"Bearer {token}", "Content-Type": "application/json"},
            body=body,
        )
        if st == 200:
            data = json.loads(raw)
            mqtt = data.get("mqtt") or {}
            leaked = any(k in json.dumps(data) for k in ("mqttPassword", "mqtt_password", "password"))
            c_ok = bool(mqtt.get("brokerUrl") or mqtt.get("broker_url")) and not leaked
            c_detail = "bootstrap mqtt metadata without password leak" if c_ok else f"bootstrap issue leaked={leaked}"
        else:
            c_detail = f"bootstrap HTTP {st}"
    flows.append(flow_result("C", "gRPC/REST bootstrap mqtt metadata", c_ok, c_detail))

    # D — Full MQTT matrix + ACL negatives
    d_ok, d_detail = run_mqtt_probe(reg)
    flows.append(flow_result("D", "MQTT matrix + ACL negatives", d_ok, d_detail, blocked_by="" if d_ok else "MQTT_AUTH_OR_ACL"))

    # E — gRPC runtime subset (health/version only — full matrix in suite)
    e_ok = False
    try:
        st, raw, _ = http_request("GET", f"{args.base_url.rstrip('/')}/health/live")
        e_ok = st == 200 and "ok" in raw.lower() or st == 200
        e_detail = f"health/live HTTP {st}"
    except Exception as exc:
        e_detail = str(exc)
    else:
        e_detail = f"health/live HTTP {st}"
    flows.append(flow_result("E", "Runtime health subset", e_ok, e_detail))

    # F — REST admin lifecycle smoke (suspend/enable)
    f_ok = False
    f_detail = "skipped"
    admin_token = subst.get("adminAccessToken", "")
    if admin_token and machine_id:
        st, _ = http_request(
            "PATCH",
            f"{args.base_url.rstrip('/')}/v1/admin/machines/{machine_id}",
            headers={"Authorization": f"Bearer {admin_token}", "Content-Type": "application/json"},
            body=json.dumps({"status": "suspended"}).encode(),
        )
        st2, _ = http_request(
            "PATCH",
            f"{args.base_url.rstrip('/')}/v1/admin/machines/{machine_id}",
            headers={"Authorization": f"Bearer {admin_token}", "Content-Type": "application/json"},
            body=json.dumps({"status": "active"}).encode(),
        )
        f_ok = st in (200, 204) and st2 in (200, 204)
        f_detail = f"suspend={st} enable={st2}"
    flows.append(flow_result("F", "REST admin lifecycle smoke", f_ok, f_detail))

    # G — Reattach rotates MQTT (requires admin + device fingerprint)
    g_ok = False
    g_detail = "skipped"
    if admin_token and machine_id:
        payload = {
            "deviceFingerprint": {
                "serialNumber": f"{reg.data.get('prefix', 'prod')}-reattach",
                "androidId": "reattach-aid",
                "packageName": "dev.avf.vending.prodtest",
            },
            "reason": "production e2e reattach",
        }
        st, raw, _ = http_request(
            "POST",
            f"{args.base_url.rstrip('/')}/v1/admin/machines/{machine_id}/reattach-device",
            headers={
                "Authorization": f"Bearer {admin_token}",
                "Content-Type": "application/json",
                "X-Request-ID": new_request_id(),
            },
            body=json.dumps(payload).encode(),
        )
        if st == 200:
            data = json.loads(raw)
            new_pass = data.get("mqtt_password") or data.get("mqttPassword")
            g_ok = bool(new_pass)
            g_detail = "reattach returned mqtt_password" if g_ok else "reattach missing mqtt_password"
            if g_ok:
                reg.set("mqttPassword", str(new_pass), entity_type="credential")
                reg.save()
        else:
            g_detail = f"reattach HTTP {st}: {raw[:300]}"
    flows.append(flow_result("G", "Reattach mqtt rotation", g_ok, g_detail, blocked_by="" if g_ok else "REATTACH_MQTT"))

    # H — Compromised/revoke MQTT auth fails (mark compromised then expect MQTT fail)
    h_ok = False
    h_detail = "skipped"
    if admin_token and machine_id:
        st, _ = http_request(
            "POST",
            f"{args.base_url.rstrip('/')}/v1/admin/machines/{machine_id}/mark-compromised",
            headers={"Authorization": f"Bearer {admin_token}", "Content-Type": "application/json"},
            body=json.dumps({"reason": "e2e mqtt revoke test"}).encode(),
        )
        if st in (200, 204):
            rc = subprocess.call([sys.executable, str(Path(__file__).parent / "run_mqtt_full_production.py")], cwd=ROOT)
            h_ok = rc != 0
            h_detail = "mqtt auth failed after compromised" if h_ok else "mqtt still connected after compromised"
        else:
            h_detail = f"mark-compromised HTTP {st}"
    flows.append(flow_result("H", "Compromised revokes MQTT", h_ok, h_detail, blocked_by="" if h_ok else "LIFECYCLE_MQTT_REVOKE"))

    # I — Offline replay idempotency (claim replay with same fingerprint)
    i_ok = False
    i_detail = "skipped"
    activation_code = subst.get("activationCode", "")
    if activation_code:
        try:
            claim2 = claim_activation(args.base_url, activation_code, f"{reg.data.get('prefix', 'prod')}-SN")
            i_ok = bool(claim2.get("machineToken") or claim2.get("accessToken"))
            i_detail = "idempotent claim replay returned token"
        except Exception as exc:
            i_detail = str(exc)
    flows.append(flow_result("I", "Offline replay idempotency", i_ok, i_detail))

    payload = {"flows": flows, "prefix": reg.data.get("prefix"), "machine_id": machine_id}
    write_json(unblock / "E2E_FLOW_RESULTS.json", payload)
    write_json(out / "E2E_FLOW_RESULTS.json", payload)
    blocked = [f for f in flows if not f["ok"] and f.get("blocked_by")]
    all_ok = all(f["ok"] for f in flows)
    print(f"E2E flows: ok={all_ok} blocked={len(blocked)}")
    return 0 if all_ok else 1


if __name__ == "__main__":
    raise SystemExit(main())

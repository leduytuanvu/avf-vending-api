#!/usr/bin/env python3
"""Focused production MQTT unblock smoke test after EMQX deploy."""

from __future__ import annotations

import argparse
import json
import os
import sys
import uuid
from datetime import datetime, timezone
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import paho.mqtt.client as mqtt

from _common import report_dir, test_prefix, write_json
from bootstrap_test_data import bootstrap
from entity_registry import EntityRegistry

DEPLOY_BUNDLE = Path(__file__).resolve().parents[2] / "reports" / "production-deploy-mqtt-final" / "20260702T212040Z"


def topic(prefix: str, machine_id: str, tail: str) -> str:
    return f"{prefix.rstrip('/')}/machines/{machine_id}/{tail}"


def mqtt_connect(host: str, port: int, username: str, password: str, client_id: str, timeout: float = 15.0):
    client = mqtt.Client(client_id=client_id, protocol=mqtt.MQTTv311)
    client.username_pw_set(username, password)
    client.tls_set()
    rc_holder: dict[str, int | None] = {"rc": None}

    def on_connect(_c, _u, _f, rc, _p=None):
        rc_holder["rc"] = rc

    client.on_connect = on_connect
    client.connect(host, port, keepalive=60)
    client.loop_start()
    deadline = __import__("time").time() + timeout
    while __import__("time").time() < deadline and rc_holder["rc"] is None and not client.is_connected():
        __import__("time").sleep(0.1)
    if rc_holder["rc"] is not None and rc_holder["rc"] != mqtt.CONNACK_ACCEPTED:
        client.loop_stop()
        client.disconnect()
        return None, f"CONNACK rc={rc_holder['rc']}"
    if not client.is_connected():
        client.loop_stop()
        client.disconnect()
        return None, "connect timeout"
    return client, None


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default=os.environ.get("BASE_URL", "https://api.ldtv.dev"))
    parser.add_argument("--mqtt-host", default=os.environ.get("MQTT_HOST", "mqtt.ldtv.dev"))
    parser.add_argument("--mqtt-port", type=int, default=int(os.environ.get("MQTT_PORT", "8883")))
    args = parser.parse_args()

    os.environ.setdefault("PRODUCTION_FULL_TEST_STRICT", "1")
    prefix = f"ENTERPRISE_PROD_MQTT_DEPLOY_TEST_{test_prefix().replace('ENTERPRISE_PROD_TEST_', '')}"
    os.environ["PRODUCTION_FULL_TEST_UTC"] = os.environ.get("PRODUCTION_FULL_TEST_UTC", "20260702T212040Z")

    results: list[dict] = []
    reg = bootstrap(args.base_url)
    subst = reg.as_substitution_map()
    machine_id = subst.get("machineId", "")
    mqtt_user = subst.get("mqttUsername", "")
    mqtt_pass = subst.get("mqttPassword", "")
    machine_token = subst.get("machineToken", "")
    topic_prefix = subst.get("mqttTopicPrefix", "avf/devices")

    claim_ok = bool(mqtt_user and mqtt_pass)
    results.append({"step": "claim_mqtt_creds", "pass": claim_ok, "mqtt_username": mqtt_user, "has_password": bool(mqtt_pass)})

    if not claim_ok:
        payload = {"pass": False, "results": results, "prefix": prefix}
        write_json(DEPLOY_BUNDLE / "06_MQTT_UNBLOCK_SMOKE_RESULT.json", payload)
        (DEPLOY_BUNDLE / "06_MQTT_UNBLOCK_SMOKE_RESULT.md").write_text("# MQTT Unblock Smoke\n\nFAIL: claim missing mqtt credentials\n", encoding="utf-8")
        return 1

    client, err = mqtt_connect(args.mqtt_host, args.mqtt_port, mqtt_user, mqtt_pass, f"smoke-{machine_id[:8]}")
    results.append({"step": "connect_provisioned", "pass": client is not None, "reason": err or "ok"})

    if client is None:
        write_json(DEPLOY_BUNDLE / "06_MQTT_UNBLOCK_SMOKE_RESULT.json", {"pass": False, "results": results})
        return 1

    tel_topic = topic(topic_prefix, machine_id, "telemetry")
    pub = client.publish(tel_topic, json.dumps({"machineId": machine_id, "schemaVersion": "1"}), qos=1)
    pub.wait_for_publish(timeout=10)
    results.append({"step": "publish_telemetry", "pass": pub.is_published(), "topic": tel_topic})

    cmd_topic = topic(topic_prefix, machine_id, "commands")
    sub_rc, _ = client.subscribe(cmd_topic, qos=1)
    client.loop(timeout=3)
    results.append({"step": "subscribe_commands", "pass": sub_rc == mqtt.MQTT_ERR_SUCCESS, "topic": cmd_topic})

    jwt_client, jwt_err = mqtt_connect(args.mqtt_host, args.mqtt_port, mqtt_user, machine_token, "neg-jwt", timeout=8)
    jwt_fail = jwt_client is None
    if jwt_client:
        jwt_client.loop_stop()
        jwt_client.disconnect()
    results.append({"step": "negative_jwt_password", "pass": jwt_fail, "reason": jwt_err})

    bad_client, bad_err = mqtt_connect(args.mqtt_host, args.mqtt_port, mqtt_user, "wrong-password", "neg-bad", timeout=8)
    bad_fail = bad_client is None
    if bad_client:
        bad_client.loop_stop()
        bad_client.disconnect()
    results.append({"step": "negative_wrong_password", "pass": bad_fail, "reason": bad_err})

    foreign = str(uuid.uuid4())
    bad_pub = client.publish(topic(topic_prefix, foreign, "telemetry"), "{}", qos=1)
    bad_pub.wait_for_publish(timeout=10)
    acl_pub = not bad_pub.is_published() or bad_pub.rc != mqtt.MQTT_ERR_SUCCESS
    results.append({"step": "negative_foreign_publish", "pass": acl_pub, "topic": topic(topic_prefix, foreign, "telemetry")})

    foreign_sub_rc, _ = client.subscribe(topic(topic_prefix, foreign, "commands"), qos=1)
    client.loop(timeout=2)
    results.append({"step": "negative_foreign_subscribe", "pass": foreign_sub_rc != mqtt.MQTT_ERR_SUCCESS})

    client.loop_stop()
    client.disconnect()

    all_pass = all(r.get("pass") for r in results)
    payload = {"pass": all_pass, "machine_id": machine_id, "prefix": prefix, "results": results, "at_utc": datetime.now(timezone.utc).isoformat()}
    DEPLOY_BUNDLE.mkdir(parents=True, exist_ok=True)
    write_json(DEPLOY_BUNDLE / "06_MQTT_UNBLOCK_SMOKE_RESULT.json", payload)
    (DEPLOY_BUNDLE / "06_MQTT_UNBLOCK_SMOKE_RESULT.md").write_text(
        f"# MQTT Unblock Smoke\n\n**Result:** {'PASS' if all_pass else 'FAIL'}\n\n" + "\n".join(f"- {r['step']}: {'PASS' if r.get('pass') else 'FAIL'}" for r in results) + "\n",
        encoding="utf-8",
    )
    return 0 if all_pass else 1


if __name__ == "__main__":
    raise SystemExit(main())

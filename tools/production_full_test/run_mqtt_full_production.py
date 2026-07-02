#!/usr/bin/env python3
"""Live production MQTT enterprise topic coverage with simulated device."""

from __future__ import annotations

import argparse
import json
import os
import sys
import time
import uuid
from datetime import datetime, timezone
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import paho.mqtt.client as mqtt

from _common import report_dir, write_json
from entity_registry import EntityRegistry

ENTERPRISE_PUBLISH_TAILS = [
    "commands/ack",
    "commands/receipt",
    "presence",
    "state/heartbeat",
    "telemetry",
    "telemetry/snapshot",
    "telemetry/incident",
    "events",
    "events/vend",
    "events/cash",
    "events/inventory",
    "shadow/reported",
]


def topic(prefix: str, machine_id: str, tail: str) -> str:
    return f"{prefix.rstrip('/')}/machines/{machine_id}/{tail}"


def mqtt_connect(
    host: str,
    port: int,
    username: str,
    password: str,
    client_id: str,
    *,
    timeout: float = 15.0,
) -> tuple[mqtt.Client | None, str | None]:
    client = mqtt.Client(client_id=client_id, protocol=mqtt.MQTTv311)
    client.username_pw_set(username, password)
    client.tls_set()
    rc_holder: dict[str, int | None] = {"rc": None}

    def on_connect(_client, _userdata, _flags, rc, _properties=None):
        rc_holder["rc"] = rc

    client.on_connect = on_connect
    try:
        client.connect(host, port, keepalive=60)
        client.loop_start()
        deadline = time.time() + timeout
        while time.time() < deadline and rc_holder["rc"] is None and not client.is_connected():
            time.sleep(0.1)
        if rc_holder["rc"] is not None and rc_holder["rc"] != mqtt.CONNACK_ACCEPTED:
            client.loop_stop()
            client.disconnect()
            return None, f"CONNACK rc={rc_holder['rc']}"
        if not client.is_connected():
            client.loop_stop()
            client.disconnect()
            return None, "connect timeout"
        return client, None
    except Exception as exc:
        return None, str(exc)


def write_mqtt_reports(rows: list[dict], out: Path) -> None:
    passed = [r for r in rows if r.get("pass")]
    failed = [r for r in rows if not r.get("pass")]
    untested = [r for r in rows if r.get("status") == "UNTESTED"]
    write_json(out / "MQTT_FULL_TEST_MATRIX.json", {"total": len(rows), "topics": rows, "pass_count": len(passed), "fail_count": len(failed), "untested_count": len(untested)})
    (out / "MQTT_PASS_LIST.md").write_text("\n".join(f"- {r['topic']}" for r in passed) + "\n", encoding="utf-8")
    (out / "MQTT_FAIL_LIST.md").write_text("\n".join(f"- {r['topic']}: {r.get('reason')}" for r in failed) + "\n", encoding="utf-8")
    (out / "MQTT_UNTESTED_LIST.md").write_text("\n".join(f"- {r['topic']}" for r in untested) + "\n", encoding="utf-8")
    write_json(out / "MQTT_FINAL_COVERAGE.json", {"pass_count": len(passed), "fail_count": len(failed), "untested_count": len(untested), "total": len(rows)})
    (out / "MQTT_FINAL_COVERAGE.md").write_text(f"Pass={len(passed)} Fail={len(failed)} Untested={len(untested)}\n", encoding="utf-8")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--mqtt-host", default=os.environ.get("MQTT_HOST", "mqtt.ldtv.dev"))
    parser.add_argument("--mqtt-port", type=int, default=int(os.environ.get("MQTT_PORT", "8883")))
    parser.add_argument("--topic-prefix", default=os.environ.get("MQTT_TOPIC_PREFIX", "avf/devices"))
    args = parser.parse_args()

    out = report_dir()
    reg_obj = EntityRegistry()
    reg = reg_obj.as_substitution_map()
    machine_id = reg.get("machineId")
    username = reg.get("mqttUsername") or machine_id or os.environ.get("MQTT_USERNAME", "")
    password = reg.get("mqttPassword") or os.environ.get("MQTT_PASSWORD", "")
    machine_token = reg.get("machineToken") or ""
    topic_prefix = reg.get("mqttTopicPrefix") or args.topic_prefix

    rows: list[dict] = []
    if not machine_id:
        for tail in ENTERPRISE_PUBLISH_TAILS + ["commands/subscribe"]:
            rows.append({"topic": tail, "pass": False, "status": "UNTESTED", "reason": "no machineId in registry"})
        write_mqtt_reports(rows, out)
        return 1

    if not username or not password:
        for tail in ENTERPRISE_PUBLISH_TAILS:
            t = topic(topic_prefix, machine_id, tail)
            rows.append({"topic": t, "pass": False, "status": "FAIL", "reason": "missing MQTT credentials"})
        write_mqtt_reports(rows, out)
        return 1

    # Negative: machine JWT must not authenticate as MQTT password
    if machine_token:
        jwt_client, jwt_err = mqtt_connect(
            args.mqtt_host,
            args.mqtt_port,
            username,
            machine_token,
            f"avf-neg-jwt-{machine_id[:8]}",
            timeout=10.0,
        )
        jwt_denied = jwt_client is None and jwt_err is not None and ("rc=5" in jwt_err or "Not authorized" in jwt_err or "timeout" in jwt_err.lower())
        if jwt_client is not None:
            jwt_client.loop_stop()
            jwt_client.disconnect()
            jwt_denied = False
        rows.append(
            {
                "topic": "negative/jwt_as_password",
                "tail": "negative/jwt_as_password",
                "pass": jwt_denied,
                "status": "PASS" if jwt_denied else "FAIL",
                "reason": jwt_err or "unexpected connect with JWT password",
                "connect_ok": False,
            }
        )

    # Negative: wrong password
    bad_client, bad_err = mqtt_connect(
        args.mqtt_host,
        args.mqtt_port,
        username,
        "invalid-production-test-password",
        f"avf-neg-badpw-{machine_id[:8]}",
        timeout=10.0,
    )
    bad_denied = bad_client is None and bad_err is not None
    if bad_client is not None:
        bad_client.loop_stop()
        bad_client.disconnect()
        bad_denied = False
    rows.append(
        {
            "topic": "negative/wrong_password",
            "tail": "negative/wrong_password",
            "pass": bad_denied,
            "status": "PASS" if bad_denied else "FAIL",
            "reason": bad_err or "unexpected connect with wrong password",
            "connect_ok": False,
        }
    )

    client, conn_err = mqtt_connect(
        args.mqtt_host,
        args.mqtt_port,
        username,
        password,
        f"avf-machine-{machine_id}",
    )
    if client is None:
        for tail in ENTERPRISE_PUBLISH_TAILS:
            t = topic(topic_prefix, machine_id, tail)
            rows.append({"topic": t, "pass": False, "status": "FAIL", "reason": f"connect failed: {conn_err}", "connect_ok": False})
        write_mqtt_reports(rows, out)
        return 1

    now = datetime.now(timezone.utc).isoformat()
    base_payload = {
        "machineId": machine_id,
        "correlationId": str(uuid.uuid4()),
        "occurredAt": now,
        "schemaVersion": "1",
    }

    for tail in ENTERPRISE_PUBLISH_TAILS:
        t = topic(topic_prefix, machine_id, tail)
        payload = dict(base_payload)
        if "command" in tail:
            payload.update({"commandId": str(uuid.uuid4()), "status": "acked"})
        if "telemetry" in tail:
            payload.update({"metric": "prod_test", "value": 1})
        if "events" in tail:
            payload.update({"eventType": "prod_test"})
        msg = json.dumps(payload)
        info = client.publish(t, msg, qos=1)
        info.wait_for_publish(timeout=10)
        ok = info.rc == mqtt.MQTT_ERR_SUCCESS and info.is_published()
        rows.append(
            {
                "topic": t,
                "tail": tail,
                "pass": ok,
                "status": "PASS" if ok else "FAIL",
                "reason": f"rc={info.rc}",
                "payload_sample": payload,
                "connect_ok": True,
            }
        )
        time.sleep(0.2)

    cmd_topic = topic(topic_prefix, machine_id, "commands")
    sub_ok, _sub_mid = client.subscribe(cmd_topic, qos=1)
    client.loop(timeout=5.0)
    rows.append(
        {
            "topic": cmd_topic,
            "tail": "commands/subscribe",
            "pass": sub_ok == mqtt.MQTT_ERR_SUCCESS,
            "status": "PASS" if sub_ok == mqtt.MQTT_ERR_SUCCESS else "FAIL",
            "reason": f"subscribe rc={sub_ok}",
            "connect_ok": True,
        }
    )

    foreign_id = str(uuid.uuid4())
    foreign_cmd = topic(topic_prefix, foreign_id, "commands")
    foreign_sub_ok, _ = client.subscribe(foreign_cmd, qos=1)
    client.loop(timeout=3.0)
    foreign_sub_denied = foreign_sub_ok != mqtt.MQTT_ERR_SUCCESS
    rows.append(
        {
            "topic": foreign_cmd,
            "tail": "negative/foreign_commands_subscribe",
            "pass": foreign_sub_denied,
            "status": "PASS" if foreign_sub_denied else "FAIL",
            "reason": f"foreign subscribe rc={foreign_sub_ok}",
            "connect_ok": True,
        }
    )

    bad_topic = topic(topic_prefix, foreign_id, "telemetry")
    bad_info = client.publish(bad_topic, json.dumps(base_payload), qos=1)
    bad_info.wait_for_publish(timeout=10)
    acl_denied = bad_info.rc != mqtt.MQTT_ERR_SUCCESS or not bad_info.is_published()
    rows.append(
        {
            "topic": bad_topic,
            "tail": "negative/wrong_machine_publish",
            "pass": acl_denied,
            "status": "PASS" if acl_denied else "FAIL",
            "reason": f"ACL deny expected rc={bad_info.rc}",
            "connect_ok": True,
        }
    )

    client.loop_stop()
    client.disconnect()

    write_mqtt_reports(rows, out)
    fail = sum(1 for r in rows if not r.get("pass"))
    print(f"MQTT full production: total={len(rows)} fail={fail}")
    return 1 if fail else 0


if __name__ == "__main__":
    raise SystemExit(main())

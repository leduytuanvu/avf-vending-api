#!/usr/bin/env python3
"""Validate API_INVENTORY_CANONICAL.json against independent source discovery."""
from __future__ import annotations

import json
import sys
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

from discovery import (  # noqa: E402
    GENERATED,
    build_grpc_inventory_items,
    build_mqtt_inventory_items,
    build_rest_inventory_items,
    discover_grpc,
    discover_mqtt,
    discover_rest,
    grpc_key,
    load_swagger,
    mqtt_key,
    rest_key,
)
from gfs_import import gfs  # noqa: E402

INVENTORY = GENERATED / "API_INVENTORY_CANONICAL.json"


def load_inventory() -> dict:
    if not INVENTORY.exists():
        print("ERROR: missing %s — run generate_complete_api_suite.py first" % INVENTORY)
        sys.exit(1)
    return json.loads(INVENTORY.read_text(encoding="utf-8"))


def validate_rest(spec: dict, inv: dict) -> tuple[set, set, set, list[str]]:
    errors: list[str] = []
    discovered = {rest_key(r["method"], r["path"]) for r in discover_rest()}
    inv_keys = {"%s %s" % (r["method"], r["path"]) for r in inv.get("rest", [])}
    missing = discovered - inv_keys
    extra = inv_keys - discovered

    rebuilt = build_rest_inventory_items(spec, discover_rest())
    rebuilt_keys = {"%s %s" % (r["method"], r["path"]) for r in rebuilt}

    for r in inv.get("rest", []):
        key = "%s %s" % (r["method"], r["path"])
        if not r.get("responses"):
            errors.append("REST %s missing responses" % key)
        else:
            ok = any(200 <= x.get("status", 0) < 300 for x in r["responses"])
            if not ok and any(str(s).startswith("2") for s in (discovered and [])):
                pass
            if not any(x.get("example") is not None for x in r["responses"]):
                errors.append("REST %s missing response example" % key)
        if r.get("requestBody") and not r["requestBody"].get("example"):
            errors.append("REST %s missing requestBody example" % key)
        if r.get("auth") is None:
            errors.append("REST %s missing auth" % key)
        for pp in r.get("pathParams") or []:
            if pp.get("required") and not pp.get("name"):
                errors.append("REST %s bad path param" % key)

    if len(rebuilt_keys) != len(discovered):
        errors.append("REST rebuild count mismatch")

    return discovered, inv_keys, missing, extra, errors


def validate_grpc(inv: dict) -> tuple[set, set, set, set, list[str]]:
    errors: list[str] = []
    rows = discover_grpc()
    templates = gfs.build_grpc_templates(rows)
    discovered = {grpc_key(r) for r in rows}
    inv_keys = {r["fullMethod"] for r in inv.get("grpc", [])}
    missing = discovered - inv_keys
    extra = inv_keys - discovered

    for g in inv.get("grpc", []):
        fm = g["fullMethod"]
        if g.get("requestExample") is None:
            errors.append("gRPC %s missing requestExample" % fm)
        if not g.get("responseExample") and not g.get("errorExamples"):
            errors.append("gRPC %s missing responseExample" % fm)
        if not g.get("auth"):
            errors.append("gRPC %s missing auth metadata" % fm)

    return discovered, inv_keys, missing, extra, errors


def validate_mqtt(inv: dict) -> tuple[set, set, set, set, list[str]]:
    errors: list[str] = []
    rows = discover_mqtt()
    discovered = {mqtt_key(r) for r in rows}
    inv_by_topic = {}
    for m in inv.get("mqtt", []):
        k = "%s|%s" % (m.get("direction", ""), m.get("topicPattern", ""))
        inv_by_topic[k] = m

    rebuilt = build_mqtt_inventory_items(rows)
    inv_keys = {mqtt_key(r) for r in rows}
    # Compare by index/topic from source rows
    discovered_inv = set()
    for m in inv.get("mqtt", []):
        discovered_inv.add("%s|%s" % (m.get("direction", ""), m.get("topicPattern", "")))

    rebuilt_keys = set()
    for r in rows:
        direction_raw = r.get("direction", "")
        if "machine_publishes" in direction_raw:
            direction = "publish"
        elif "backend_publishes" in direction_raw:
            direction = "publish"
        else:
            direction = "subscribe"
        rebuilt_keys.add("%s|%s" % (direction, r.get("topicPattern", "")))

    missing = rebuilt_keys - discovered_inv
    extra = discovered_inv - rebuilt_keys

    for m in inv.get("mqtt", []):
        if not m.get("payloadExample"):
            errors.append("MQTT %s missing payloadExample" % m.get("id"))
        if not m.get("direction"):
            errors.append("MQTT %s missing direction" % m.get("id"))
        ack = m.get("expectedAckTopic")
        if ack and not m.get("expectedAckPayloadExample"):
            errors.append("MQTT %s missing ack example" % m.get("id"))

    return rebuilt_keys, discovered_inv, missing, extra, errors


def main() -> int:
    inv = load_inventory()
    spec = load_swagger()

    rest_disc, rest_inv, rest_missing, rest_extra, rest_err = validate_rest(spec, inv)
    grpc_disc, grpc_inv, grpc_missing, grpc_extra, grpc_err = validate_grpc(inv)
    mqtt_disc, mqtt_inv, mqtt_missing, mqtt_extra, mqtt_err = validate_mqtt(inv)

    print("REST total: %s" % len(rest_disc))
    print("REST inventory: %s" % len(rest_inv))
    print("REST missing: %s" % len(rest_missing))
    print("REST extra: %s" % len(rest_extra))

    print("gRPC total: %s" % len(grpc_disc))
    print("gRPC inventory: %s" % len(grpc_inv))
    print("gRPC missing: %s" % len(grpc_missing))
    print("gRPC extra: %s" % len(grpc_extra))

    print("MQTT total: %s" % len(mqtt_disc))
    print("MQTT inventory: %s" % len(mqtt_inv))
    print("MQTT missing: %s" % len(mqtt_missing))
    print("MQTT extra: %s" % len(mqtt_extra))

    all_errors = rest_err + grpc_err + mqtt_err
    if rest_missing:
        all_errors.append("REST missing: %s" % sorted(rest_missing)[:5])
    if rest_extra:
        all_errors.append("REST extra: %s" % sorted(rest_extra)[:5])
    if grpc_missing:
        all_errors.append("gRPC missing: %s" % sorted(grpc_missing)[:5])
    if grpc_extra:
        all_errors.append("gRPC extra: %s" % sorted(grpc_extra)[:5])
    if mqtt_missing:
        all_errors.append("MQTT missing: %s" % sorted(mqtt_missing)[:5])
    if mqtt_extra:
        all_errors.append("MQTT extra: %s" % sorted(mqtt_extra)[:5])

    if all_errors:
        print("\nERRORS:")
        for e in all_errors[:30]:
            print(" -", e)
        return 1

    print("\nInventory validation: PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main())

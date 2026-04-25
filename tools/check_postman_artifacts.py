#!/usr/bin/env python3
"""Validate docs/postman/*.json (offline; no network). Invoked by make postman-check."""
from __future__ import annotations

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
POSTMAN = ROOT / "docs" / "postman"


def die(msg: str) -> None:
    print(f"ERROR: {msg}", file=sys.stderr)
    raise SystemExit(1)


def env_values(path: Path) -> dict[str, str]:
    data = json.loads(path.read_text(encoding="utf-8"))
    out: dict[str, str] = {}
    for e in data.get("values", []):
        if e.get("enabled", True) and (k := e.get("key")) is not None:
            out[k] = e.get("value", "")
    return out


def main() -> None:
    if not POSTMAN.is_dir():
        die(f"missing {POSTMAN}")

    required = [
        "avf-vending-api.postman_collection.json",
        "avf-local.postman_environment.json",
        "avf-staging.postman_environment.json",
        "avf-production.postman_environment.json",
    ]
    for name in required:
        if not (POSTMAN / name).is_file():
            die(f"missing {POSTMAN / name}")

    for p in sorted(POSTMAN.glob("*.json")):
        try:
            json.loads(p.read_text(encoding="utf-8"))
        except json.JSONDecodeError as e:
            die(f"invalid JSON {p}: {e}")

    coll_path = POSTMAN / "avf-vending-api.postman_collection.json"
    coll = json.loads(coll_path.read_text(encoding="utf-8"))
    schema = (coll.get("info") or {}).get("schema", "")
    if "getpostman.com" not in schema.lower() or "collection" not in schema.lower():
        die("collection must be Postman v2.1 (info.schema)")
    if coll.get("openapi") and "item" not in coll:
        die("refusing: file looks like OpenAPI root, not Postman")
    if "item" not in coll:
        die("Postman collection must have top-level item")

    prod = env_values(POSTMAN / "avf-production.postman_environment.json")
    stg = env_values(POSTMAN / "avf-staging.postman_environment.json")

    if prod.get("allow_mutation") != "false":
        die("production: allow_mutation must be false")
    if prod.get("allow_production_mutation") != "false":
        die("production: allow_production_mutation must be false")
    if prod.get("payment_env") != "live":
        die("production: payment_env must be live")
    if prod.get("mqtt_topic_prefix") != "avf/devices":
        die("production: mqtt_topic_prefix must be avf/devices")
    if stg.get("payment_env") != "sandbox":
        die("staging: payment_env must be sandbox")
    if stg.get("mqtt_topic_prefix") == "avf/devices":
        die("staging: mqtt_topic_prefix must not be avf/devices")

    craw = coll_path.read_text(encoding="utf-8")
    if "I_UNDERSTAND_PRODUCTION_MUTATION" not in craw:
        die("collection must include production mutation guard string")
    if "postman-avf" not in craw:
        die("collection must include postman-avf marker")
    if "https://api.ldtv.dev" in craw:
        die("collection must not hardcode https://api.ldtv.dev; use {{base_url}} / variables only")

    banned = (
        "DATABASE_URL=",
        "SUPABASE_",
        "JWT_SECRET=",
        "WEBHOOK_SECRET=",
        "PAYMENT_SECRET",
        "STRIPE_SECRET",
        "Bearer eyJ",
    )
    for p in sorted(POSTMAN.glob("*.json")):
        text = p.read_text(encoding="utf-8", errors="replace")
        for b in banned:
            if b in text:
                die(f"forbidden pattern {b!r} in {p.name}")
    if re.search(r"(sk|pk)_(live|test)_[A-Za-z0-9]{20,}", craw):
        die("collection must not include stripe-like key material")

    print("OK: Postman artifact checks", flush=True)


if __name__ == "__main__":
    main()

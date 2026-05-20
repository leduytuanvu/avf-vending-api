#!/usr/bin/env python3
"""Validate postman/collections/*.json and postman/environments/*.json (offline; no network). Invoked by make postman-check."""
from __future__ import annotations

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
POSTMAN_COLLECTIONS = ROOT / "postman" / "collections"
POSTMAN_ENVIRONMENTS = ROOT / "postman" / "environments"

FORBIDDEN_ENV_KEY_PARTS = (
    "organization_id",
    "organizationid",
    "tenant_id",
    "tenantid",
    "scope_id",
    "scopeid",
)


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


def collection_variable_keys(coll: dict) -> set[str]:
    out: set[str] = set()
    for v in coll.get("variable") or []:
        k = v.get("key")
        if isinstance(k, str) and k:
            out.add(k)
    return out


def walk_collection_requests(coll: dict):
    def rec(items: list):
        for it in items or []:
            if not isinstance(it, dict):
                continue
            if it.get("item"):
                yield from rec(it["item"])
            elif it.get("request"):
                yield it

    yield from rec(coll.get("item") or [])


def request_url_raw(req: dict) -> str:
    u = req.get("url")
    if isinstance(u, str):
        return u
    if isinstance(u, dict):
        return str(u.get("raw") or "")
    return ""


def validate_phase7_postman(coll: dict, env_keys: set[str]) -> None:
    """Placeholder resolution against env + collection vars; JSON bodies; non-empty URLs."""
    cv = collection_variable_keys(coll)
    allowed = env_keys | cv | {"active_token", "swagger_url", "api_prefix"}

    placeholder_re = re.compile(r"\{\{([^}]+)\}\}")

    def check_placeholders(text: str, ctx: str) -> None:
        if not isinstance(text, str):
            return
        for m in placeholder_re.finditer(text):
            name = m.group(1).strip()
            if name.startswith("$"):
                continue
            if name not in allowed:
                die(
                    "unknown placeholder {{%s}} in %s (add to environment or collection variables)"
                    % (name, ctx)
                )

    for it in walk_collection_requests(coll):
        req = it.get("request") or {}
        name = it.get("name") or "(unnamed)"
        url_raw = request_url_raw(req)
        if not url_raw.strip():
            die("empty URL on request %r" % name)
        check_placeholders(url_raw, "url." + str(name))
        body = req.get("body") or {}
        if body.get("mode") == "raw":
            raw = body.get("raw")
            if isinstance(raw, str) and raw.strip():
                check_placeholders(raw, "body." + str(name))
                try:
                    json.loads(raw)
                except json.JSONDecodeError as e:
                    die("invalid JSON body on request %r: %s" % (name, e))

    for ek in env_keys:
        el = ek.lower().replace("-", "_")
        for bad in FORBIDDEN_ENV_KEY_PARTS:
            if bad in el:
                die("forbidden environment key pattern %r matches %r" % (bad, ek))


def main() -> None:
    if not POSTMAN_COLLECTIONS.is_dir():
        die(f"missing {POSTMAN_COLLECTIONS}")
    if not POSTMAN_ENVIRONMENTS.is_dir():
        die(f"missing {POSTMAN_ENVIRONMENTS}")

    required_collections = [
        "avf-vending-api.postman_collection.json",
    ]
    required_envs = [
        "avf-local.postman_environment.json",
        "avf-staging.postman_environment.json",
        "avf-production.postman_environment.json",
    ]
    for name in required_collections:
        if not (POSTMAN_COLLECTIONS / name).is_file():
            die(f"missing {POSTMAN_COLLECTIONS / name}")
    for name in required_envs:
        if not (POSTMAN_ENVIRONMENTS / name).is_file():
            die(f"missing {POSTMAN_ENVIRONMENTS / name}")

    for p in sorted(POSTMAN_COLLECTIONS.glob("*.json")) + sorted(POSTMAN_ENVIRONMENTS.glob("*.json")):
        try:
            json.loads(p.read_text(encoding="utf-8"))
        except json.JSONDecodeError as e:
            die(f"invalid JSON {p}: {e}")

    coll_path = POSTMAN_COLLECTIONS / "avf-vending-api.postman_collection.json"
    coll = json.loads(coll_path.read_text(encoding="utf-8"))
    schema = (coll.get("info") or {}).get("schema", "")
    if "getpostman.com" not in schema.lower() or "collection" not in schema.lower():
        die("collection must be Postman v2.1 (info.schema)")
    if coll.get("openapi") and "item" not in coll:
        die("refusing: file looks like OpenAPI root, not Postman")
    if "item" not in coll:
        die("Postman collection must have top-level item")

    prod = env_values(POSTMAN_ENVIRONMENTS / "avf-production.postman_environment.json")
    stg = env_values(POSTMAN_ENVIRONMENTS / "avf-staging.postman_environment.json")
    local = env_values(POSTMAN_ENVIRONMENTS / "avf-local.postman_environment.json")
    env_key_union = set(local.keys()) | set(stg.keys()) | set(prod.keys())

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
        die("collection must not hardcode https://api.ldtv.dev; use {{baseUrl}} variables only")

    banned = (
        "DATABASE_URL=",
        "SUPABASE_",
        "JWT_SECRET=",
        "WEBHOOK_SECRET=",
        "PAYMENT_SECRET",
        "STRIPE_SECRET",
        "Bearer eyJ",
    )
    for p in sorted(POSTMAN_COLLECTIONS.glob("*.json")) + sorted(POSTMAN_ENVIRONMENTS.glob("*.json")):
        text = p.read_text(encoding="utf-8", errors="replace")
        for b in banned:
            if b in text:
                die(f"forbidden pattern {b!r} in {p.name}")
    if re.search(r"(sk|pk)_(live|test)_[A-Za-z0-9]{20,}", craw):
        die("collection must not include stripe-like key material")

    validate_phase7_postman(coll, env_key_union)

    print("OK: Postman artifact checks", flush=True)


if __name__ == "__main__":
    main()

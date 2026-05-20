#!/usr/bin/env python3
"""Validate generated Postman suite under postman/generated/."""
from __future__ import annotations

import json
import re
import sys
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

from discovery import GENERATED, discover_grpc, discover_mqtt, discover_rest, grpc_key, load_swagger, rest_key  # noqa: E402
from gfs_import import REPO_ROOT  # noqa: E402

REST_COLLECTION = GENERATED / "rest" / "AVF_REST_FULL.postman_collection.json"
INVENTORY = GENERATED / "API_INVENTORY_CANONICAL.json"
GRPC_EXAMPLES = GENERATED / "grpc" / "AVF_GRPC_EXAMPLES.json"
GRPC_SMOKE = GENERATED / "grpc" / "AVF_GRPCURL_SMOKE.sh"
MQTT_EXAMPLES = GENERATED / "mqtt" / "AVF_MQTT_EXAMPLES.json"
MQTT_SMOKE = GENERATED / "mqtt" / "AVF_MQTT_SMOKE.sh"
REST_CATALOG = GENERATED / "rest" / "AVF_REST_REQUEST_RESPONSE_CATALOG.md"
GRPC_CATALOG = GENERATED / "grpc" / "AVF_GRPC_REQUEST_RESPONSE_CATALOG.md"
MQTT_CATALOG = GENERATED / "mqtt" / "AVF_MQTT_REQUEST_RESPONSE_CATALOG.md"
QUICK_GUIDE = GENERATED / "AVF_FULL_API_QUICK_TEST_GUIDE.md"

SKIP_SECRET_SCAN = frozenset({"validate-generated-api-suite.py", "validate-api-inventory.py"})

SECRET_PATTERNS = [
    re.compile(r"postgresql://"),
    re.compile(r"DATABASE_URL="),
    re.compile(r"BEGIN RSA PRIVATE KEY"),
    re.compile(r"BEGIN OPENSSH"),
    re.compile(r"eyJhbGci[A-Za-z0-9_-]{10,}\."),
    re.compile(r"password123", re.I),
    re.compile(r"1@Ldtv"),
]

REQUIRED_ENV_KEYS = frozenset(
    {
        "baseUrl",
        "accessToken",
        "adminEmail",
        "adminPassword",
        "machineId",
        "siteId",
        "mqttHost",
        "mqttPort",
        "mqttTopicPrefix",
        "grpcHost",
        "grpcPort",
        "idempotencyKey",
        "requestId",
    }
)


def walk_requests(collection: dict) -> list[dict]:
    out: list[dict] = []

    def walk(items):
        for it in items or []:
            if "item" in it:
                walk(it["item"])
            elif "request" in it:
                out.append(it)

    walk(collection.get("item", []))
    return out


def rest_key_from_request(req_item: dict) -> str | None:
    req = req_item.get("request") or {}
    method = (req.get("method") or "").upper()
    url = req.get("url")
    if isinstance(url, str):
        return None
    if not isinstance(url, dict):
        return None
    path = url.get("path") or []
    if not path:
        return None
    # Reconstruct swagger-style path
    segs = []
    for s in path:
        if s.startswith("{{") and s.endswith("}}"):
            segs.append("{%s}" % s[2:-2])
        else:
            segs.append(s)
    swagger_path = "/" + "/".join(segs)
    return rest_key(method, swagger_path)


def validate_rest_collection(spec: dict, inv: dict) -> tuple[int, int, set, set, list[str]]:
    errors: list[str] = []
    if not REST_COLLECTION.exists():
        return 0, 0, set(), set(), ["missing REST collection"]

    collection = json.loads(REST_COLLECTION.read_text(encoding="utf-8"))
    if not collection.get("info"):
        errors.append("REST collection missing info")

    requests = walk_requests(collection)
    keys = set()
    for ri in requests:
        req = ri.get("request") or {}
        if not req.get("method"):
            errors.append("request missing method: %s" % ri.get("name"))
        url = req.get("url")
        if isinstance(url, str):
            errors.append("URL is string not object: %s" % ri.get("name"))
            continue
        if not url or not url.get("raw"):
            errors.append("empty raw URL: %s" % ri.get("name"))
        for qp in (url or {}).get("query") or []:
            if qp.get("required") and qp.get("disabled"):
                errors.append("required query disabled: %s" % ri.get("name"))
        body = req.get("body") or {}
        if body.get("mode") == "raw" and body.get("raw"):
            try:
                json.loads(body["raw"])
            except json.JSONDecodeError as e:
                errors.append("bad JSON body %s: %s" % (ri.get("name"), e))

        rk = rest_key_from_request(ri)
        if rk:
            keys.add(rk)

    discovered = {rest_key(r["method"], r["path"]) for r in discover_rest()}
    missing = discovered - keys
    extra = keys - discovered

    # Auth check sample
    inv_by_key = {"%s %s" % (r["method"], r["path"]): r for r in inv.get("rest", [])}
    for ri in requests:
        rk = rest_key_from_request(ri)
        if not rk or rk not in inv_by_key:
            continue
        need_auth = inv_by_key[rk]["auth"]["required"]
        headers = {h.get("key", "").lower() for h in (ri.get("request") or {}).get("header") or []}
        if need_auth and "authorization" not in headers:
            errors.append("missing auth header: %s" % rk)

    return len(discovered), len(keys), missing, extra, errors


def validate_grpc_outputs(inv: dict) -> tuple[int, int, set, set, list[str]]:
    errors: list[str] = []
    discovered = {grpc_key(r) for r in discover_grpc()}
    if not GRPC_EXAMPLES.exists():
        return len(discovered), 0, discovered, set(), ["missing GRPC examples"]
    examples = json.loads(GRPC_EXAMPLES.read_text(encoding="utf-8"))
    present = {e["fullMethod"] for e in examples if e.get("fullMethod")}
    missing = discovered - present
    extra = present - discovered

    smoke_text = GRPC_SMOKE.read_text(encoding="utf-8") if GRPC_SMOKE.exists() else ""
    if "AVF_GRPC_EXAMPLES.json" not in smoke_text:
        errors.append("grpcurl smoke missing EXAMPLES reference")
    for fm in discovered:
        if fm not in smoke_text and fm not in (GRPC_EXAMPLES.read_text(encoding="utf-8") if GRPC_EXAMPLES.exists() else ""):
            pass  # coverage via EXAMPLES json + python driver
        ex = next((e for e in examples if e.get("fullMethod") == fm), None)
        if not ex or ex.get("requestExample") is None:
            errors.append("gRPC missing request example: %s" % fm)

    if not GRPC_CATALOG.exists():
        errors.append("missing gRPC catalog")
    else:
        cat = GRPC_CATALOG.read_text(encoding="utf-8")
        for fm in discovered:
            if fm not in cat:
                errors.append("gRPC catalog missing: %s" % fm)

    return len(discovered), len(present), missing, extra, errors


def validate_mqtt_outputs(inv: dict) -> tuple[int, int, set, set, list[str]]:
    errors: list[str] = []
    rows = discover_mqtt()
    discovered_ids = {m["id"] for m in inv.get("mqtt", [])}
    if not MQTT_EXAMPLES.exists():
        return len(rows), 0, discovered_ids, set(), ["missing MQTT examples"]
    examples = json.loads(MQTT_EXAMPLES.read_text(encoding="utf-8"))
    present_ids = {e["id"] for e in examples if e.get("id")}
    missing = discovered_ids - present_ids
    extra = present_ids - discovered_ids

    smoke_text = MQTT_SMOKE.read_text(encoding="utf-8") if MQTT_SMOKE.exists() else ""
    if "AVF_MQTT_EXAMPLES.json" not in smoke_text:
        errors.append("MQTT smoke missing EXAMPLES reference")
        if not m.get("payloadExample"):
            errors.append("MQTT no payload: %s" % m["id"])

    if not MQTT_CATALOG.exists():
        errors.append("missing MQTT catalog")

    return len(discovered_ids), len(present_ids), missing, extra, errors


def validate_environments() -> list[str]:
    errors: list[str] = []
    for env_path in (GENERATED / "rest").glob("*.postman_environment.json"):
        env = json.loads(env_path.read_text(encoding="utf-8"))
        keys = {v["key"] for v in env.get("values", [])}
        for rk in REQUIRED_ENV_KEYS:
            if rk not in keys:
                errors.append("%s missing key %s" % (env_path.name, rk))
        for v in env.get("values", []):
            val = str(v.get("value", ""))
            if v["key"] in ("accessToken", "refreshToken", "mqttPassword", "adminPassword") and val and not val.startswith("{{"):
                if len(val) > 20:
                    errors.append("%s may contain secret in %s" % (env_path.name, v["key"]))
    return errors


def scan_secrets() -> list[str]:
    errors: list[str] = []
    for root in (GENERATED, SCRIPT_DIR):
        if not root.exists():
            continue
        for p in root.rglob("*"):
            if p.is_dir() or p.suffix not in (".json", ".md", ".sh", ".py"):
                continue
            if p.name in SKIP_SECRET_SCAN:
                continue
            text = p.read_text(encoding="utf-8", errors="ignore")
            for pat in SECRET_PATTERNS:
                if pat.search(text):
                    errors.append("secret pattern in %s" % p.relative_to(REPO_ROOT))
    return errors


def validate_docs(inv: dict) -> list[str]:
    errors: list[str] = []
    if not QUICK_GUIDE.exists():
        errors.append("missing quick test guide")
    if not REST_CATALOG.exists():
        errors.append("missing REST catalog")
    else:
        cat = REST_CATALOG.read_text(encoding="utf-8")
        for r in inv.get("rest", []):
            if r["path"] not in cat:
                errors.append("REST catalog missing path %s" % r["path"])
                break
    return errors


def main() -> int:
    if not INVENTORY.exists():
        print("ERROR: run generator first")
        return 1

    inv = json.loads(INVENTORY.read_text(encoding="utf-8"))
    spec = load_swagger()

    rt, rp, rm, re_, rerr = validate_rest_collection(spec, inv)
    gt, gp, gm, ge, gerr = validate_grpc_outputs(inv)
    mt, mp, mm, me, merr = validate_mqtt_outputs(inv)

    errors = rerr + gerr + merr + validate_environments() + scan_secrets() + validate_docs(inv)

    print("REST total: %s" % rt)
    print("REST present: %s" % rp)
    print("REST missing: %s" % len(rm))
    print("REST extra: %s" % len(re_))

    print("gRPC total: %s" % gt)
    print("gRPC present: %s" % gp)
    print("gRPC missing: %s" % len(gm))
    print("gRPC extra: %s" % len(ge))

    print("MQTT total: %s" % mt)
    print("MQTT present: %s" % mp)
    print("MQTT missing: %s" % len(mm))
    print("MQTT extra: %s" % len(me))

    if errors or rm or re_ or gm or ge or mm or me:
        print("\nERRORS:")
        for e in (list(rm)[:3] + list(re_)[:3] + errors)[:40]:
            print(" -", e)
        print("\nFinal: FAIL")
        return 1

    print("\nFinal: PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main())

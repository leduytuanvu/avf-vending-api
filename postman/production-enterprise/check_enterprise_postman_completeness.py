#!/usr/bin/env python3
"""Strict market-ready completeness checker for enterprise Postman project."""
from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
OUT_DIR = REPO_ROOT / "postman" / "production-enterprise"
DOCS_DIR = REPO_ROOT / "docs" / "testing" / "production-e2e"
COLL = OUT_DIR / "AVF_PRODUCTION_ENTERPRISE_REST.postman_collection.json"
ENV = OUT_DIR / "AVF_PRODUCTION_ENTERPRISE.postman_environment.json"
GRPC_MD = OUT_DIR / "AVF_PRODUCTION_GRPC_REQUESTS.md"
MQTT_MD = OUT_DIR / "AVF_PRODUCTION_MQTT_REQUESTS.md"
ACTOR_MD = DOCS_DIR / "POSTMAN_ENTERPRISE_ACTOR_FLOW_MATRIX.md"

sys.path.insert(0, str(OUT_DIR))
import check_enterprise_api_coverage as api_cov  # noqa: E402
from enterprise_actor_lib import MARKET_RELEASE_FLOWS  # noqa: E402
from enterprise_surface_lib import build_grpc_inventory, build_mqtt_inventory, build_rest_inventory  # noqa: E402

SECRET_RE = re.compile(
    r"(eyJ[A-Za-z0-9_-]{10,}|sk_[A-Za-z0-9]{20,}|ghp_[A-Za-z0-9]{20,}|"
    r"password\s*[:=]\s*['\"]?[^'\"<\s]{8,})",
    re.I,
)

REQUIRED_ENV_KEYS = {
    "baseUrl", "grpcTarget", "mqttHost", "mqttPort", "mqttTls", "adminEmail", "adminPassword",
    "accessToken", "refreshToken", "runId", "e2ePrefix", "categoryId", "brandId", "tagId",
    "productId", "mediaId", "siteId", "machineId", "activationCode", "machineAccessToken",
    "machineRefreshToken", "topologyId", "planogramId", "slotCode", "commandId", "orderId",
    "reportFrom", "reportTo", "allowGatedWrites", "confirmProductionWrites", "onlinePaymentEnabled",
    "mqttTopicPrefix",
}


def audit_id() -> str:
    return os.environ.get("PROD_E2E_RUN_ID", datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ") + "-recheck")


def coll_vars(coll: dict) -> set[str]:
    found: set[str] = set()
    blob = json.dumps(coll)

    def walk(items: list) -> None:
        for it in items or []:
            if "item" in it:
                walk(it["item"])
            elif "request" in it:
                found.update(re.findall(r"\{\{(\w+)\}\}", json.dumps(it["request"])))

    walk(coll.get("item") or [])
    found.update(re.findall(r"\{\{(\w+)\}\}", blob))
    return found


def check_descriptions(coll: dict) -> list[str]:
    errors: list[str] = []
    required_fields = ("Used by:", "Primary actor:", "Production purpose:", "Auth:")

    def walk(items: list) -> None:
        for it in items or []:
            if "item" in it:
                walk(it["item"])
            elif "request" in it:
                name = str(it.get("name") or "")
                if name.startswith("[RELEASE]") or "README" in name.upper() or name.startswith("How "):
                    continue
                desc = str(it.get("description") or it["request"].get("description") or "")
                if not any(f in desc for f in required_fields):
                    if "REST-COV-" in name or "[ADMIN" in name or "[PUBLIC" in name:
                        errors.append(f"missing actor metadata in description: {name[:80]}")
                skip_prefix = (
                    "How to", "Actor map", "Production write", "Online payment", "gRPC catalog",
                    "MQTT catalog", "Coverage summary", "RELEASE", "README",
                )
                if not name.startswith("[") and not any(name.startswith(s) for s in skip_prefix):
                    if "REST-COV" in name or " — " in name:
                        errors.append(f"missing actor prefix in name: {name[:80]}")

    walk(coll.get("item") or [])
    return errors[:30]


def check_secrets(paths: list[Path]) -> list[str]:
    errors: list[str] = []
    for p in paths:
        if not p.is_file():
            continue
        text = p.read_text(encoding="utf-8", errors="replace")
        if SECRET_RE.search(text):
            errors.append(f"possible secret pattern in {p.name}")
    return errors


def check_grpc_mqtt_docs() -> list[str]:
    errors: list[str] = []
    if not GRPC_MD.is_file():
        return ["missing AVF_PRODUCTION_GRPC_REQUESTS.md"]
    gtext = GRPC_MD.read_text(encoding="utf-8")
    for g in build_grpc_inventory():
        if g.server_registered != "YES" or g.service == "MachineSaleService":
            continue
        if g.verdict == "MISSING_FROM_DOCS":
            errors.append(f"gRPC missing docs: {g.service}/{g.method}")
    if "grpcurl" not in gtext:
        errors.append("gRPC catalog missing grpcurl commands")
    if not MQTT_MD.is_file():
        return errors + ["missing AVF_PRODUCTION_MQTT_REQUESTS.md"]
    mtext = MQTT_MD.read_text(encoding="utf-8")
    if "mosquitto_pub" not in mtext:
        errors.append("MQTT docs missing mosquitto_pub")
    if "mosquitto_sub" not in mtext and "Subscribe" not in mtext:
        errors.append("MQTT docs missing subscribe guidance")
    for m in build_mqtt_inventory():
        if m.verdict == "MISSING_FROM_DOCS":
            errors.append(f"MQTT missing: {m.rel_topic}")
        if "?" in m.enterprise_pattern:
            errors.append(f"MQTT topic placeholder ?: {m.rel_topic}")
    return errors


def check_market_flows(coll: dict) -> list[str]:
    blob = json.dumps(coll)
    missing = []
    for mf in MARKET_RELEASE_FLOWS:
        if f"Flow {mf['id']}" not in blob:
            missing.append(mf["id"])
    if missing:
        return [f"market flow folder missing: {', '.join(missing)}"]
    return []


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--audit-id", default="")
    args = ap.parse_args()
    aid = args.audit_id or audit_id()
    failures: list[str] = []

    if api_cov.main() != 0:
        failures.append("api coverage checker failed (see ENTERPRISE_COVERAGE_FAILED)")

    rest = build_rest_inventory()
    rest_miss = sum(1 for r in rest if r.verdict == "MISSING_FROM_POSTMAN" and r.runnable_production == "YES")
    if rest_miss:
        failures.append(f"REST missing count={rest_miss}")

    grpc_miss = sum(
        1
        for g in build_grpc_inventory()
        if g.verdict == "MISSING_FROM_DOCS" and g.server_registered == "YES" and g.service != "MachineSaleService"
    )
    if grpc_miss:
        failures.append(f"gRPC missing count={grpc_miss}")

    mqtt_miss = sum(1 for m in build_mqtt_inventory() if m.verdict == "MISSING_FROM_DOCS")
    if mqtt_miss:
        failures.append(f"MQTT missing count={mqtt_miss}")

    if not COLL.is_file():
        failures.append("collection missing")
        _write_result(aid, failures, rest_miss, grpc_miss, mqtt_miss, 0, 0, 0, 0)
        print("ENTERPRISE_POSTMAN_COMPLETE_FAILED", file=sys.stderr)
        return 1

    coll = json.loads(COLL.read_text(encoding="utf-8"))
    failures.extend(check_grpc_mqtt_docs())
    failures.extend(check_market_flows(coll))
    failures.extend(check_descriptions(coll))
    failures.extend(check_secrets([COLL, ENV]))

    env_keys = set()
    if ENV.is_file():
        env = json.loads(ENV.read_text(encoding="utf-8"))
        env_keys = {v["key"] for v in env.get("values") or []}
    used = coll_vars(coll)
    env_missing = sorted((used | REQUIRED_ENV_KEYS) - env_keys)

    if env_missing:
        failures.append(f"environment missing keys: {env_missing[:15]}")

    if not ACTOR_MD.is_file():
        failures.append("missing POSTMAN_ENTERPRISE_ACTOR_FLOW_MATRIX.md")

    actor_missing = len([e for e in failures if "actor" in e.lower()])
    req_missing = len([e for e in failures if "description" in e.lower() or "prefix" in e.lower()])
    market_missing = len([e for e in failures if "market flow" in e.lower()])

    ok = not failures
    _write_result(aid, failures, rest_miss, grpc_miss, mqtt_miss, actor_missing, market_missing, len(env_missing), req_missing)

    if ok:
        print("ENTERPRISE_POSTMAN_COMPLETE_OK")
        return 0
    print("ENTERPRISE_POSTMAN_COMPLETE_FAILED", file=sys.stderr)
    for f in failures[:50]:
        print(f"  {f}", file=sys.stderr)
    return 1


def _write_result(
    aid: str,
    failures: list[str],
    rest_miss: int,
    grpc_miss: int,
    mqtt_miss: int,
    actor_missing: int,
    market_missing: int,
    env_missing: int,
    req_missing: int,
) -> None:
    path = DOCS_DIR / f"POSTMAN_ENTERPRISE_RECHECK_RESULT_{aid}.md"
    path.write_text(
        "\n".join(
            [
                f"# Postman enterprise recheck result ({aid})",
                "",
                f"**Result:** {'ENTERPRISE_POSTMAN_COMPLETE_OK' if not failures else 'ENTERPRISE_POSTMAN_COMPLETE_FAILED'}",
                "",
                "| Metric | Count |",
                "|--------|------:|",
                f"| REST missing | {rest_miss} |",
                f"| gRPC missing | {grpc_miss} |",
                f"| MQTT missing | {mqtt_miss} |",
                f"| Actor/metadata issues | {actor_missing} |",
                f"| Market flow missing | {market_missing} |",
                f"| Env missing | {env_missing} |",
                f"| Request metadata missing | {req_missing} |",
                "",
                "## Failures",
                "",
            ]
            + ([f"- {f}" for f in failures] if failures else ["- None"])
        ),
        encoding="utf-8",
    )


if __name__ == "__main__":
    raise SystemExit(main())

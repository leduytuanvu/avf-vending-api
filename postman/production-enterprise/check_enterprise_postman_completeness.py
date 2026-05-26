#!/usr/bin/env python3
"""Strict market-ready completeness checker for enterprise Postman project."""
from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
from collections import Counter
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
GIT_PATH_RE = re.compile(r"C:/Program Files/Git|C:\\Program Files\\Git", re.I)

REQUIRED_ENV_KEYS = {
    "baseUrl", "grpcTarget", "mqttHost", "mqttPort", "mqttTls", "adminEmail", "adminPassword",
    "accessToken", "refreshToken", "adminUserId", "adminAccountId", "runId", "e2ePrefix", "runPrefix",
    "categoryId", "brandId", "tagId", "productId", "mediaId", "siteId", "machineId",
    "activationCode", "machineAccessToken", "machineRefreshToken", "topologyId", "planogramId",
    "planogramRevision", "slotCode", "slotIndex", "stockItemId", "commandId", "orderId", "vendId",
    "reportFrom", "reportTo", "allowGatedWrites", "confirmProductionWrites", "onlinePaymentEnabled",
    "confirmOnlinePaymentTesting", "momoEnabled", "zalopayEnabled", "vietqrEnabled", "mqttTopicPrefix",
    "operatorSessionId",
}


def audit_id() -> str:
    return os.environ.get("PROD_E2E_RUN_ID", datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ") + "-reorg")


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


def walk_all_requests(coll: dict) -> list[dict]:
    out: list[dict] = []

    def walk(items: list) -> None:
        for it in items or []:
            if "item" in it:
                walk(it["item"])
            elif "request" in it:
                out.append(it)

    walk(coll.get("item") or [])
    return out


def check_descriptions(coll: dict) -> list[str]:
    errors: list[str] = []
    required_fields = ("Used by:", "Primary actor:", "Production purpose:", "Auth:")

    for it in walk_all_requests(coll):
        name = str(it.get("name") or "")
        if name.startswith("[RELEASE]") or "README" in name.upper() or name.startswith("00."):
            continue
        if "MachineCatalogService/" in name or "MachineTokenService/" in name:
            continue
        desc = str(it.get("description") or it["request"].get("description") or "")
        if not any(f in desc for f in required_fields):
            if "REST-COV-" in name or "[ADMIN" in name or "[PUBLIC" in name or "[MACHINE" in name:
                errors.append(f"missing actor metadata in description: {name[:80]}")
        skip_prefix = (
            "How to", "Actor map", "Production write", "Online payment", "gRPC catalog",
            "MQTT catalog", "Coverage summary", "RELEASE", "README", "00.",
        )
        if not name.startswith("[") and not any(name.startswith(s) for s in skip_prefix):
            if "REST-COV" in name or " — " in name:
                errors.append(f"missing actor prefix in name: {name[:80]}")

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


def check_git_path_artifacts(coll: dict) -> list[str]:
    blob = json.dumps(coll)
    if GIT_PATH_RE.search(blob):
        return ["collection contains C:/Program Files/Git path artifact"]
    return []


def check_duplicate_flow_ids(coll: dict) -> list[str]:
    ids: list[str] = []
    for it in walk_all_requests(coll):
        fid = it.get("_manifest_flow_id")
        if fid:
            ids.append(str(fid))
    dupes = [k for k, v in Counter(ids).items() if v > 1]
    if dupes:
        return [f"duplicate manifest flow id: {dupes[:5]}"]
    return []


def check_collection_gate(coll: dict) -> list[str]:
    errors: list[str] = []
    prereq_blob = ""
    for ev in coll.get("event") or []:
        if ev.get("listen") == "prerequest":
            prereq_blob = "\n".join((ev.get("script") or {}).get("exec") or [])
    if "allowGatedWrites" not in prereq_blob or "confirmProductionWrites" not in prereq_blob:
        errors.append("collection missing production write gate script")
    if "GET','HEAD','OPTIONS" not in prereq_blob.replace(" ", ""):
        errors.append("collection write gate must exempt GET/HEAD/OPTIONS")
    if "/v1/auth/login" not in prereq_blob:
        errors.append("collection write gate must exempt POST /v1/auth/login")
    if "accessToken captured" not in json.dumps(coll):
        login_ok = False
        for it in walk_all_requests(coll):
            if it.get("_manifest_flow_id") == "REST-AUTH-001":
                events = it.get("event") or []
                blob = json.dumps(events)
                if "accessToken" in blob and "tokens.accessToken" in blob:
                    login_ok = True
        if not login_ok:
            errors.append("REST-AUTH-001 missing accessToken capture test script")
    return errors


def check_payment_default_blocked(coll: dict) -> list[str]:
    errors: list[str] = []
    for it in walk_all_requests(coll):
        if it.get("_classification") != "ONLINE_PAYMENT_EXCLUDED":
            continue
        events = it.get("event") or []
        blob = json.dumps(events)
        if "onlinePaymentEnabled" not in blob and "ONLINE_PAYMENT_EXCLUDED" not in blob:
            errors.append(f"payment route lacks guard prerequest: {it.get('name','')[:60]}")
    return errors[:10]


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
    coll_stub = json.loads(COLL.read_text(encoding="utf-8")) if COLL.is_file() else {}
    if coll_stub and "20 - gRPC Reference" not in json.dumps(coll_stub.get("item") or []):
        errors.append("collection missing folder 20 - gRPC Reference")
    if coll_stub and "21 - MQTT Reference" not in json.dumps(coll_stub.get("item") or []):
        errors.append("collection missing folder 21 - MQTT Reference")
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


def check_module_folders(coll: dict) -> list[str]:
    blob = json.dumps([it.get("name") for it in coll.get("item") or []])
    required_top = [
        "00 - README Safety",
        "01 - Health Version",
        "02 - Auth",
        "03 - Category",
        "06 - Product",
        "07 - Media",
        "20 - gRPC Reference",
        "21 - MQTT Reference",
        "90 - Full Business Flows",
        "97 - Online Payment Guarded",
    ]
    missing = [f for f in required_top if f not in blob]
    if missing:
        return [f"missing top-level folders: {missing}"]
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

    grpc_inv = build_grpc_inventory()
    grpc_miss = sum(
        1
        for g in grpc_inv
        if g.verdict == "MISSING_FROM_DOCS" and g.server_registered == "YES" and g.service != "MachineSaleService"
    )
    if grpc_miss:
        failures.append(f"gRPC missing count={grpc_miss}")

    mqtt_inv = build_mqtt_inventory()
    mqtt_miss = sum(1 for m in mqtt_inv if m.verdict == "MISSING_FROM_DOCS")
    if mqtt_miss:
        failures.append(f"MQTT missing count={mqtt_miss}")

    folder_count = 0
    request_count = 0
    env_missing: list[str] = []
    coll: dict = {}
    if not COLL.is_file():
        failures.append("collection missing")
    else:
        coll = json.loads(COLL.read_text(encoding="utf-8"))
        request_count = len(walk_all_requests(coll))

        def count_folders(items: list) -> int:
            n = 0
            for it in items or []:
                if "item" in it:
                    n += 1 + count_folders(it["item"])
            return n

        folder_count = count_folders(coll.get("item") or [])
        failures.extend(check_grpc_mqtt_docs())
        failures.extend(check_market_flows(coll))
        failures.extend(check_descriptions(coll))
        failures.extend(check_secrets([COLL, ENV]))
        failures.extend(check_git_path_artifacts(coll))
        failures.extend(check_duplicate_flow_ids(coll))
        failures.extend(check_collection_gate(coll))
        failures.extend(check_payment_default_blocked(coll))
        failures.extend(check_module_folders(coll))

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
    _write_reorg_check(
        aid,
        failures,
        rest_miss,
        grpc_miss,
        mqtt_miss,
        len(rest),
        len(grpc_inv),
        len(mqtt_inv),
        request_count,
        folder_count,
        len(env_missing),
        actor_missing,
        market_missing,
        req_missing,
    )

    if ok:
        print("ENTERPRISE_POSTMAN_REORG_OK")
        print("ENTERPRISE_POSTMAN_COMPLETE_OK")
        return 0
    print("ENTERPRISE_POSTMAN_REORG_FAILED", file=sys.stderr)
    print("ENTERPRISE_POSTMAN_COMPLETE_FAILED", file=sys.stderr)
    for f in failures[:50]:
        print(f"  {f}", file=sys.stderr)
    return 1


def _write_reorg_check(
    aid: str,
    failures: list[str],
    rest_miss: int,
    grpc_miss: int,
    mqtt_miss: int,
    rest_src: int,
    grpc_src: int,
    mqtt_src: int,
    request_count: int,
    folder_count: int,
    env_missing: int,
    actor_missing: int,
    market_missing: int,
    req_missing: int,
) -> None:
    verdict = "ENTERPRISE_POSTMAN_REORG_OK" if not failures else "ENTERPRISE_POSTMAN_REORG_FAILED"
    path = DOCS_DIR / f"POSTMAN_ENTERPRISE_REORG_CHECK_{aid}.md"
    path.write_text(
        "\n".join(
            [
                f"# Postman enterprise reorganize check ({aid})",
                "",
                f"**Verdict:** `{verdict}`",
                "",
                "| Metric | Count |",
                "|--------|------:|",
                f"| REST source routes (inventory) | {rest_src} |",
                f"| REST collection requests | {request_count} |",
                f"| REST missing (runnable) | {rest_miss} |",
                f"| gRPC source methods | {grpc_src} |",
                f"| gRPC missing from docs | {grpc_miss} |",
                f"| MQTT source flows | {mqtt_src} |",
                f"| MQTT missing from docs | {mqtt_miss} |",
                f"| Collection folders | {folder_count} |",
                f"| Environment missing keys | {env_missing} |",
                f"| Actor/metadata issues | {actor_missing} |",
                f"| Market flow missing | {market_missing} |",
                f"| Request metadata missing | {req_missing} |",
                "",
                "## Failures",
                "",
            ]
            + ([f"- {f}" for f in failures] if failures else ["- None"])
        ),
        encoding="utf-8",
    )
    recheck = DOCS_DIR / f"POSTMAN_ENTERPRISE_RECHECK_RESULT_{aid}.md"
    recheck.write_text(path.read_text(encoding="utf-8"), encoding="utf-8")


if __name__ == "__main__":
    raise SystemExit(main())

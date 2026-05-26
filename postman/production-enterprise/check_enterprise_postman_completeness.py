#!/usr/bin/env python3
"""Happy-case completeness checker for AVF Production Enterprise Postman project."""
from __future__ import annotations

import argparse
import json
import os
import re
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
NEG_MD = OUT_DIR / "AVF_PRODUCTION_NEGATIVE_TESTS_EXCLUDED.md"

sys.path.insert(0, str(OUT_DIR))
import check_enterprise_api_coverage as api_cov  # noqa: E402
from enterprise_actor_lib import MARKET_RELEASE_FLOWS  # noqa: E402
from enterprise_happy_case_lib import (  # noqa: E402
    FLOW_ID_PREFIX_RE,
    NUMERIC_PREFIX_RE,
    REST_ID_PREFIX_RE,
    UNHAPPY_FOLDER_KEYWORDS,
    is_happy_collection_name,
)
from enterprise_surface_lib import build_grpc_inventory, build_mqtt_inventory, build_rest_inventory  # noqa: E402

SECRET_RE = re.compile(
    r"(eyJ[A-Za-z0-9_-]{20,}|sk_[A-Za-z0-9]{20,}|ghp_[A-Za-z0-9]{20,}|"
    r"password\s*[:=]\s*['\"]?[^'\"<\s]{12,})",
    re.I,
)
GIT_PATH_RE = re.compile(r"C:/Program Files/Git|C:\\Program Files\\Git", re.I)

REQUIRED_ENV_KEYS = {
    "baseUrl", "grpcTarget", "mqttHost", "mqttPort", "mqttTls", "adminEmail", "adminPassword",
    "accessToken", "refreshToken", "adminAccountId", "adminRoles", "runId", "e2ePrefix",
    "categoryId", "brandId", "tagId", "productId", "mediaId", "siteId", "machineId",
    "activationCode", "machineToken", "machineAccessToken", "machineRefreshToken",
    "topologyId", "planogramId", "planogramRevision", "slotCode", "slotIndex", "stockItemId",
    "operatorSessionId", "commandId", "orderId", "vendId", "reportFrom", "reportTo",
    "allowGatedWrites", "confirmProductionWrites", "onlinePaymentEnabled", "confirmOnlinePaymentTesting",
    "momoEnabled", "zalopayEnabled", "vietqrEnabled",
}

REQUIRED_TOP_FOLDERS = [
    "README Safety",
    "Health Version",
    "Auth",
    "Category",
    "Brand",
    "Product",
    "gRPC Reference",
    "MQTT Reference",
    "Full Business Flows",
    "Online Payment Happy Case Guarded",
    "Cleanup",
]

NEGATIVE_NAME_RE = re.compile(
    r"(negative|invalid password|wrong token|missing token|invalid token|wrong role|abuse|auth_negative)",
    re.I,
)


def audit_id() -> str:
    return os.environ.get("PROD_E2E_RUN_ID", datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ") + "-happy")


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


def walk_names(coll: dict) -> list[tuple[str, str]]:
    """Return (kind, name) for every folder and request."""
    out: list[tuple[str, str]] = []

    def walk(items: list, in_folder: str = "") -> None:
        for it in items or []:
            name = str(it.get("name") or "")
            if "item" in it:
                out.append(("folder", name))
                walk(it["item"], name)
            elif "request" in it:
                out.append(("request", name))

    walk(coll.get("item") or [])
    return out


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


def check_no_numeric_or_negative_names(coll: dict) -> list[str]:
    errors: list[str] = []
    for kind, name in walk_names(coll):
        if not name:
            continue
        if NUMERIC_PREFIX_RE.match(name):
            errors.append(f"{kind} name starts with digit: {name[:70]}")
        if REST_ID_PREFIX_RE.match(name):
            errors.append(f"{kind} name uses REST-XXX-NNN id prefix: {name[:70]}")
        if FLOW_ID_PREFIX_RE.match(name):
            errors.append(f"{kind} name uses Flow NN prefix: {name[:70]}")
        if not is_happy_collection_name(name):
            errors.append(f"{kind} name fails happy naming rules: {name[:70]}")
        if NEGATIVE_NAME_RE.search(name):
            errors.append(f"{kind} name contains negative keyword: {name[:70]}")
        low = name.lower()
        if any(k in low for k in UNHAPPY_FOLDER_KEYWORDS):
            errors.append(f"{kind} name contains unhappy keyword: {name[:70]}")
    return errors[:40]


def check_collection_meta(coll: dict) -> list[str]:
    errors: list[str] = []
    info = coll.get("info") or {}
    if info.get("name") != "AVF Production Enterprise Happy Case API":
        errors.append(f"collection name must be happy-case title, got: {info.get('name')}")
    blob = json.dumps(coll)
    if "auth_negative" in blob:
        errors.append("collection still contains auth_negative flows")
    if "[ADMIN_WEB]" in blob or "[PUBLIC]" in blob or "[MACHINE_APP]" in blob:
        errors.append("collection still uses legacy [ACTOR] REST-COV naming")
    return errors


def check_descriptions(coll: dict) -> list[str]:
    errors: list[str] = []
    skip = (
        "How To", "Actor Map", "Production Write", "Variable Map", "Release Scope",
        "gRPC Catalog", "MQTT Catalog", "Coverage Summary", "Business Flow",
    )

    for it in walk_all_requests(coll):
        name = str(it.get("name") or "")
        if any(name.startswith(s) for s in skip):
            continue
        if name.startswith("gRPC -") or name.startswith("MQTT -"):
            continue
        desc = str(it.get("description") or it["request"].get("description") or "")
        for field in ("Used by:", "Auth:", "CRUD action:", "Source file:", "Production safety:", "Purpose:"):
            if field not in desc:
                errors.append(f"missing {field} in description: {name[:60]}")
                break
    return errors[:25]


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
    if GIT_PATH_RE.search(json.dumps(coll)):
        return ["collection contains C:/Program Files/Git path artifact"]
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
    return errors


def check_login_capture(coll: dict) -> list[str]:
    errors: list[str] = []
    login_ok = False
    for it in walk_all_requests(coll):
        name = str(it.get("name") or "")
        url = json.dumps(it.get("request") or {})
        is_login = "Admin Login" in name or "/v1/auth/login" in url
        if not is_login:
            continue
        blob = json.dumps(it.get("event") or [])
        if "tokens.accessToken" in blob and "accessToken" in blob:
            login_ok = True
        body = json.dumps((it.get("request") or {}).get("body") or "")
        if "adminEmail" not in body or "adminPassword" not in body:
            errors.append("login request must use {{adminEmail}} and {{adminPassword}}")
    if not login_ok:
        errors.append("Auth login missing accessToken/refreshToken capture test script")
    return errors


def check_write_gates(coll: dict) -> list[str]:
    errors: list[str] = []
    for it in walk_all_requests(coll):
        method = str((it.get("request") or {}).get("method") or "GET").upper()
        url = json.dumps(it.get("request") or {})
        if method in ("GET", "HEAD", "OPTIONS"):
            for ev in it.get("event") or []:
                if ev.get("listen") == "prerequest":
                    blob = "\n".join((ev.get("script") or {}).get("exec") or [])
                    if "Gated write blocked" in blob or (
                        "allowGatedWrites" in blob and "confirmProductionWrites" in blob
                    ):
                        errors.append(f"GET/HEAD/OPTIONS must not be write-gated: {it.get('name','')[:50]}")
        if method in ("POST", "PUT", "PATCH", "DELETE") and "/v1/auth/login" not in url:
            if it.get("_classification") == "ONLINE_PAYMENT_EXCLUDED":
                continue
            has_gate = False
            for ev in it.get("event") or []:
                if ev.get("listen") == "prerequest":
                    blob = "\n".join((ev.get("script") or {}).get("exec") or [])
                    if "onlinePaymentEnabled" in blob:
                        has_gate = True
            if not has_gate:
                coll_prereq = any(
                    ev.get("listen") == "prerequest"
                    for ev in coll.get("event") or []
                )
                if not coll_prereq:
                    errors.append(f"write missing collection gate: {it.get('name','')[:50]}")
    return errors[:15]


def check_admin_bearer(coll: dict) -> list[str]:
    errors: list[str] = []
    for it in walk_all_requests(coll):
        desc = str(it.get("description") or "")
        if "Auth:** bearer_admin" not in desc and "bearer_admin" not in desc.lower():
            continue
        method = str((it.get("request") or {}).get("method") or "GET").upper()
        if method in ("GET", "HEAD", "OPTIONS", "POST") and "/v1/auth/login" in json.dumps(it.get("request") or {}):
            continue
        prereq = json.dumps(it.get("event") or [])
        coll_blob = json.dumps(coll.get("event") or [])
        if "accessToken" not in prereq and "accessToken" not in coll_blob:
            errors.append(f"bearer_admin request lacks accessToken injection: {it.get('name','')[:50]}")
    return errors[:10]


def check_payment_default_blocked(coll: dict) -> list[str]:
    errors: list[str] = []
    for it in walk_all_requests(coll):
        if it.get("_classification") != "ONLINE_PAYMENT_EXCLUDED":
            continue
        blob = json.dumps(it.get("event") or [])
        if "onlinePaymentEnabled" not in blob and "ONLINE_PAYMENT_EXCLUDED" not in blob:
            errors.append(f"payment route lacks guard: {it.get('name','')[:60]}")
    return errors[:10]


def check_grpc_mqtt_docs(coll: dict | None) -> list[str]:
    errors: list[str] = []
    if not GRPC_MD.is_file():
        return ["missing AVF_PRODUCTION_GRPC_REQUESTS.md"]
    gtext = GRPC_MD.read_text(encoding="utf-8")
    for g in build_grpc_inventory():
        if g.server_registered != "YES" or g.service == "MachineSaleService":
            continue
        if g.verdict == "MISSING_FROM_DOCS":
            errors.append(f"gRPC missing docs: {g.service}/{g.method}")
        key = f"{g.service}/{g.method}"
        if key not in gtext and g.method not in gtext:
            errors.append(f"gRPC missing grpcurl for {key}")
    if "grpcurl" not in gtext:
        errors.append("gRPC catalog missing grpcurl commands")
    if not MQTT_MD.is_file():
        return errors + ["missing AVF_PRODUCTION_MQTT_REQUESTS.md"]
    mtext = MQTT_MD.read_text(encoding="utf-8")
    if "mosquitto_pub" not in mtext:
        errors.append("MQTT docs missing mosquitto_pub")
    for m in build_mqtt_inventory():
        if m.verdict == "MISSING_FROM_DOCS":
            errors.append(f"MQTT missing: {m.rel_topic}")
        if "?" in m.enterprise_pattern:
            errors.append(f"MQTT topic placeholder ?: {m.rel_topic}")
        if m.rel_topic not in mtext and m.enterprise_pattern not in mtext:
            errors.append(f"MQTT missing mosquitto for {m.rel_topic}")
    if coll:
        blob = json.dumps(coll)
        if "gRPC Reference" not in blob:
            errors.append("collection missing gRPC Reference folder")
        if "MQTT Reference" not in blob:
            errors.append("collection missing MQTT Reference folder")
    return errors[:30]


def check_market_flows(coll: dict) -> list[str]:
    blob = json.dumps(coll)
    missing = []
    for mf in MARKET_RELEASE_FLOWS:
        if "security negative" in mf["title"].lower():
            continue
        title = mf["title"]
        if f"Business Flow - {title}" not in blob and title not in blob:
            missing.append(title)
    if missing:
        return [f"business flow missing: {', '.join(missing[:5])}"]
    return []


def check_module_folders(coll: dict) -> list[str]:
    top = [it.get("name") for it in coll.get("item") or []]
    missing = [f for f in REQUIRED_TOP_FOLDERS if f not in top]
    if missing:
        return [f"missing top-level folders: {missing}"]
    return []


def _write_happy_recheck(
    aid: str,
    failures: list[str],
    rest_miss: int,
    grpc_miss: int,
    mqtt_miss: int,
    request_count: int,
    folder_count: int,
) -> None:
    verdict = "POSTMAN_HAPPY_CASE_COMPLETE_OK" if not failures else "POSTMAN_HAPPY_CASE_COMPLETE_FAILED"
    path = DOCS_DIR / f"POSTMAN_HAPPY_CASE_RECHECK_{aid}.md"
    path.write_text(
        "\n".join(
            [
                f"# Postman happy-case recheck ({aid})",
                "",
                f"**Verdict:** `{verdict}`",
                "",
                "| Metric | Value |",
                "|--------|------:|",
                f"| REST missing (runnable) | {rest_miss} |",
                f"| gRPC missing from docs | {grpc_miss} |",
                f"| MQTT missing from docs | {mqtt_miss} |",
                f"| Collection requests | {request_count} |",
                f"| Collection folders | {folder_count} |",
                "",
                "## Failures",
                "",
            ]
            + ([f"- {f}" for f in failures] if failures else ["- None"])
        ),
        encoding="utf-8",
    )


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--audit-id", default="")
    args = ap.parse_args()
    aid = args.audit_id or audit_id()
    failures: list[str] = []

    if api_cov.main() != 0:
        failures.append("api coverage checker failed")

    rest = build_rest_inventory()
    rest_miss = sum(
        1
        for r in rest
        if r.verdict == "MISSING_FROM_POSTMAN" and r.runnable_production == "YES"
    )
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

    if not NEG_MD.is_file():
        failures.append("missing AVF_PRODUCTION_NEGATIVE_TESTS_EXCLUDED.md (regenerate)")

    request_count = 0
    folder_count = 0
    coll: dict = {}
    if not COLL.is_file():
        failures.append("collection missing — run generate_enterprise_postman_project.py")
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
        failures.extend(check_collection_meta(coll))
        failures.extend(check_no_numeric_or_negative_names(coll))
        failures.extend(check_grpc_mqtt_docs(coll))
        failures.extend(check_market_flows(coll))
        failures.extend(check_descriptions(coll))
        failures.extend(check_secrets([COLL, ENV]))
        failures.extend(check_git_path_artifacts(coll))
        failures.extend(check_collection_gate(coll))
        failures.extend(check_login_capture(coll))
        failures.extend(check_write_gates(coll))
        failures.extend(check_admin_bearer(coll))
        failures.extend(check_payment_default_blocked(coll))
        failures.extend(check_module_folders(coll))

        env_keys = set()
        if ENV.is_file():
            env = json.loads(ENV.read_text(encoding="utf-8"))
            env_keys = {v["key"] for v in env.get("values") or []}
        used = coll_vars(coll)
        env_missing = sorted((used | REQUIRED_ENV_KEYS) - env_keys)
        if env_missing:
            failures.append(f"environment missing keys: {env_missing[:20]}")

    if not ACTOR_MD.is_file():
        failures.append("missing POSTMAN_ENTERPRISE_ACTOR_FLOW_MATRIX.md")

    _write_happy_recheck(aid, failures, rest_miss, grpc_miss, mqtt_miss, request_count, folder_count)

    ok = not failures
    if ok:
        print("POSTMAN_HAPPY_CASE_COMPLETE_OK")
        return 0
    print("POSTMAN_HAPPY_CASE_COMPLETE_FAILED", file=sys.stderr)
    for f in failures[:50]:
        print(f"  {f}", file=sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())

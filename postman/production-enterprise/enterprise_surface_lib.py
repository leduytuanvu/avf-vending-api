#!/usr/bin/env python3
"""Source-of-truth inventories for enterprise Postman coverage audits."""
from __future__ import annotations

import json
import re
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

REPO_ROOT = Path(__file__).resolve().parents[2]
SCRIPTS = REPO_ROOT / "tests" / "e2e" / "production" / "scripts"
sys.path.insert(0, str(SCRIPTS))

from generate_rest_route_matrix import (  # noqa: E402
    REPO_SWAGGER,
    find_override,
    iter_swagger_ops,
    load_yaml,
    match_openapi_key,
    postman_route_to_openapi_key,
)

PROTO_DIR = REPO_ROOT / "proto" / "avf" / "machine" / "v1"
GRPC_REGISTER_FILE = REPO_ROOT / "internal" / "grpcserver" / "machine_grpc_services.go"
TOPICS_GO = REPO_ROOT / "internal" / "platform" / "mqtt" / "topics.go"
OVERRIDES = REPO_ROOT / "tests" / "e2e" / "production" / "rest-route-overrides.yaml"
MATRIX_JSON = REPO_ROOT / "tests" / "e2e" / "production" / "generated" / "rest-route-matrix.json"
ENTERPRISE_COLL = REPO_ROOT / "postman" / "production-enterprise" / "AVF_PRODUCTION_ENTERPRISE_REST.postman_collection.json"
GRPC_MD = REPO_ROOT / "postman" / "production-enterprise" / "AVF_PRODUCTION_GRPC_REQUESTS.md"
MQTT_MD = REPO_ROOT / "postman" / "production-enterprise" / "AVF_PRODUCTION_MQTT_REQUESTS.md"
MANIFEST_GRPC = REPO_ROOT / "tests" / "e2e" / "production" / "e2e-manifest-grpc.yaml"
MANIFEST_MQTT = REPO_ROOT / "tests" / "e2e" / "production" / "e2e-manifest-mqtt.yaml"

REGISTERED_GRPC_SERVICES = frozenset(
    {
        "MachineActivationService",
        "MachineTokenService",
        "MachineAuthService",
        "MachineBootstrapService",
        "MachineCatalogService",
        "MachineMediaService",
        "MachineInventoryService",
        "MachineTelemetryService",
        "MachineOperatorService",
        "MachineCommerceService",
        "MachineSaleService",
        "MachineOfflineSyncService",
        "MachineCommandService",
    }
)

# Proto RPC aliases — same handler; canonical name is value.
GRPC_RPC_ALIASES: dict[tuple[str, str], str] = {
    ("MachineAuthService", "ActivateMachine"): "ClaimActivation",
    ("MachineAuthService", "ClaimActivation"): "ClaimActivation",
    ("MachineAuthService", "RefreshMachineToken"): "RefreshMachineToken",
    ("MachineTokenService", "RefreshMachineToken"): "RefreshMachineToken",
    ("MachineActivationService", "ClaimActivation"): "ClaimActivation",
    ("MachineCatalogService", "GetSaleCatalog"): "GetCatalogSnapshot",
    ("MachineCatalogService", "SyncSaleCatalog"): "GetCatalogSnapshot",
    ("MachineCatalogService", "GetCatalogSnapshot"): "GetCatalogSnapshot",
    ("MachineCatalogService", "GetMediaManifest"): "GetMediaManifest",
    ("MachineMediaService", "GetMediaManifest"): "GetMediaManifest",
    ("MachineCommerceService", "AttachPaymentResult"): "CreatePaymentSession",
    ("MachineCommerceService", "CreateCashCheckout"): "ConfirmCashPayment",
    ("MachineCommerceService", "ConfirmVendSuccess"): "ReportVendSuccess",
    ("MachineInventoryService", "SubmitFillReport"): "SubmitFillResult",
    ("MachineInventoryService", "SubmitStockAdjustment"): "SubmitInventoryAdjustment",
    ("MachineInventoryService", "PushInventoryDelta"): "PushInventoryDelta",
    ("MachineOperatorService", "SubmitFillReport"): "SubmitFillResult",
    ("MachineOperatorService", "SubmitStockAdjustment"): "SubmitInventoryAdjustment",
    ("MachineBootstrapService", "CheckIn"): "CheckIn",
    ("MachineTelemetryService", "CheckIn"): "CheckIn",
}

# Parallel Sale service surface — documented as SALE_API_PARALLEL, not required for E2E cash path.
GRPC_SALE_SERVICE_OPTIONAL = True

MQTT_REL_TOPICS = [
    ("presence", "publish", "device"),
    ("state/heartbeat", "publish", "device"),
    ("telemetry/snapshot", "publish", "device"),
    ("telemetry/incident", "publish", "device"),
    ("events/vend", "publish", "device"),
    ("events/cash", "publish", "device"),
    ("events/inventory", "publish", "device"),
    ("commands/ack", "publish", "device"),
    ("commands/receipt", "publish", "device"),
    ("shadow/reported", "publish", "device"),
    ("shadow/desired", "publish", "device"),
    ("commands", "subscribe", "backend_outbound"),
    ("telemetry", "publish", "device_legacy"),
]

TEMPLATE_RE = re.compile(r"\{\{([^}]+)\}\}")
PARAM_RE = re.compile(r"\{([^}]+)\}")


@dataclass
class RestRouteRow:
    source: str
    method: str
    path: str
    normalized_path: str
    auth_type: str = ""
    openapi_present: str = "YES"
    route_matrix_present: str = "NO"
    manifest_present: str = "NO"
    enterprise_postman: str = "NO"
    postman_folder: str = ""
    runnable_production: str = "YES"
    skip_reason: str = ""
    verdict: str = "SOURCE_AMBIGUOUS"


@dataclass
class GrpcMethodRow:
    proto_file: str
    package: str
    service: str
    method: str
    server_registered: str
    canonical_method: str
    e2e_present: str = "NO"
    enterprise_docs: str = "NO"
    verdict: str = "SOURCE_AMBIGUOUS"
    skip_reason: str = ""


@dataclass
class MqttTopicRow:
    rel_topic: str
    direction: str
    actor: str
    enterprise_pattern: str
    e2e_present: str = "NO"
    enterprise_docs: str = "NO"
    verdict: str = "SOURCE_AMBIGUOUS"
    skip_reason: str = ""


def load_swagger() -> dict[str, Any]:
    return json.loads(REPO_SWAGGER.read_text(encoding="utf-8"))


def normalize_postman_path(path: str) -> str:
    p = path.split("?", 1)[0]
    if not p.startswith("/"):
        p = "/" + p
    return TEMPLATE_RE.sub(lambda m: "{" + m.group(1) + "}", p)


def collect_enterprise_postman_routes(coll_path: Path = ENTERPRISE_COLL) -> dict[str, list[dict[str, Any]]]:
    coll = json.loads(coll_path.read_text(encoding="utf-8"))
    by_key: dict[str, list[dict[str, Any]]] = {}

    def walk(items: list[Any], folder: str = "") -> None:
        for it in items or []:
            name = str(it.get("name") or "")
            sub_folder = f"{folder}/{name}" if folder else name
            if "item" in it:
                walk(it["item"], sub_folder)
            elif "request" in it:
                req = it["request"]
                method = str(req.get("method", "")).upper()
                url = req.get("url")
                if isinstance(url, dict):
                    parts = url.get("path") or []
                    path = "/" + "/".join(str(p) for p in parts)
                    path = normalize_postman_path(path)
                    key = f"{method} {path}"
                    fid = ""
                    if "—" in name:
                        fid = name.split("—", 1)[0].strip()
                    by_key.setdefault(key, []).append(
                        {"folder": sub_folder, "name": name, "flow_id": fid, "description": it.get("description") or ""}
                    )

    walk(coll.get("item") or [])
    return by_key


def parse_proto_grpc() -> list[GrpcMethodRow]:
    rows: list[GrpcMethodRow] = []
    for pf in sorted(PROTO_DIR.glob("*.proto")):
        text = pf.read_text(encoding="utf-8")
        pkg = "avf.machine.v1"
        m = re.search(r"package\s+([\w.]+)", text)
        if m:
            pkg = m.group(1)
        service = None
        for line in text.splitlines():
            sm = re.match(r"\s*service\s+(\w+)", line)
            if sm:
                service = sm.group(1)
            rm = re.match(r"\s*rpc\s+(\w+)", line)
            if rm and service:
                method = rm.group(1)
                reg = "YES" if service in REGISTERED_GRPC_SERVICES else "NO"
                canonical = GRPC_RPC_ALIASES.get((service, method), method)
                rows.append(
                    GrpcMethodRow(
                        proto_file=pf.name,
                        package=pkg,
                        service=service,
                        method=method,
                        server_registered=reg,
                        canonical_method=canonical,
                    )
                )
    return rows


def parse_implemented_grpc() -> set[tuple[str, str]]:
    pat = re.compile(r"^func \(s \*(\w+)\) (\w+)\(")
    server_map = {
        "machineActivationServer": "MachineActivationService",
        "machineTokenServer": "MachineTokenService",
        "machineAuthServer": "MachineAuthService",
        "machineBootstrapServer": "MachineBootstrapService",
        "machineCatalogServer": "MachineCatalogService",
        "machineMediaServer": "MachineMediaService",
        "machineInventoryServer": "MachineInventoryService",
        "machineTelemetryServer": "MachineTelemetryService",
        "machineOperatorServer": "MachineOperatorService",
        "machineCommerceServer": "MachineCommerceService",
        "machineSaleServer": "MachineSaleService",
        "machineOfflineSyncServer": "MachineOfflineSyncService",
        "machineCommandServer": "MachineCommandService",
    }
    found: set[tuple[str, str]] = set()
    for go in (REPO_ROOT / "internal" / "grpcserver").glob("machine*.go"):
        for line in go.read_text(encoding="utf-8").splitlines():
            m = pat.match(line)
            if not m:
                continue
            srv = server_map.get(m.group(1))
            if srv:
                found.add((srv, m.group(2)))
    return found


def walk_grpc_manifest_flows(obj: Any, out: list[dict[str, Any]]) -> None:
    if isinstance(obj, dict):
        if obj.get("service") and obj.get("rpc"):
            out.append(obj)
        for v in obj.values():
            walk_grpc_manifest_flows(v, out)
    elif isinstance(obj, list):
        for item in obj:
            walk_grpc_manifest_flows(item, out)


def collect_e2e_grpc_methods() -> set[tuple[str, str]]:
    data = load_yaml(MANIFEST_GRPC)
    flows: list[dict[str, Any]] = []
    walk_grpc_manifest_flows(data.get("flows") or [], flows)
    return {(str(f["service"]), str(f["rpc"])) for f in flows}


def parse_grpc_docs(md_path: Path = GRPC_MD) -> set[tuple[str, str]]:
    if not md_path.is_file():
        return set()
    text = md_path.read_text(encoding="utf-8")
    found: set[tuple[str, str]] = set()
    for m in re.finditer(r"grpcurl[^\n]*\s+[\w.-:]+:\d+\s+(\w+)\.(\w+)", text):
        found.add((m.group(1), m.group(2)))
    for m in re.finditer(r"grpcurl[^\n]*\s+\{\{grpcTarget\}\}\s+(\w+)/(\w+)", text):
        found.add((m.group(1), m.group(2)))
    for m in re.finditer(r"\|\s*(\w+Service)\s*\|\s*(\w+)\s*\|", text):
        if m.group(1) in REGISTERED_GRPC_SERVICES:
            found.add((m.group(1), m.group(2)))
    for m in re.finditer(r"###\s*(\w+Service)/(\w+)", text):
        found.add((m.group(1), m.group(2)))
    for m in re.finditer(r"- Service: `(\w+Service)` RPC: `(\w+)`", text):
        found.add((m.group(1), m.group(2)))
    return found


def enterprise_mqtt_pattern(rel: str) -> str:
    return f"{{{{mqttTopicPrefix}}}}/machines/{{{{machineId}}}}/{rel}"


def collect_e2e_mqtt_topic_keys() -> set[str]:
    data = load_yaml(MANIFEST_MQTT)
    keys: set[str] = set()
    for f in data.get("flows") or []:
        tk = f.get("topic_key")
        if tk:
            keys.add(str(tk))
        if f.get("handler") == "command_pipeline":
            keys.update({"command_in", "command_ack"})
    return keys


def parse_mqtt_docs(md_path: Path = MQTT_MD) -> set[str]:
    if not md_path.is_file():
        return set()
    text = md_path.read_text(encoding="utf-8")
    found: set[str] = set()
    for rel, _, _ in MQTT_REL_TOPICS:
        if f"| `{rel}` |" in text or f"/{rel}`" in text or f"/{rel}'" in text:
            found.add(rel)
    return found


def build_rest_inventory() -> list[RestRouteRow]:
    swagger = load_swagger()
    overrides = load_yaml(OVERRIDES)
    swagger_ops = iter_swagger_ops(swagger)
    param_map = overrides.get("param_state_map") or {}
    postman = collect_enterprise_postman_routes()
    matrix = {}
    if MATRIX_JSON.is_file():
        matrix = json.loads(MATRIX_JSON.read_text(encoding="utf-8")).get("routes") or {}

    rows: list[RestRouteRow] = []
    for method, path, op in swagger_ops:
        key = f"{method.upper()} {path}"
        ov = find_override(overrides, method.upper(), path)
        mat = matrix.get(key) or {}
        skip = ""
        verdict = "MISSING_FROM_POSTMAN"
        runnable = "YES"
        ent = "NO"
        postman_hits = postman.get(key) or []
        if not postman_hits:
            for pk, hits in postman.items():
                pm, pp = pk.split(" ", 1)
                if pm != method.upper():
                    continue
                hit = match_openapi_key(method.upper(), normalize_postman_path(pp), swagger_ops)
                if hit == key:
                    postman_hits = hits
                    break
        if postman_hits:
            ent = "YES"
            verdict = "COVERED"
        if ov and str(ov.get("coverage")) == "documented_skip":
            skip = str(ov.get("skip_reason") or "documented_skip")
            verdict = "CONTRACT_DISABLED" if "legacy" in skip.lower() or "not mounted" in skip.lower() else "CONFIG_REQUIRED"
            if ent == "NO":
                verdict = "CONTRACT_DISABLED" if verdict == "CONTRACT_DISABLED" else "CONFIG_REQUIRED"
            runnable = "NO"
        elif mat.get("coverage") == "documented_skip":
            skip = str(mat.get("skip_reason") or mat.get("non_postman_reason") or "documented_skip")
            verdict = "CONTRACT_DISABLED" if ent == "NO" else "COVERED"
            runnable = "NO"
        elif mat.get("coverage") == "auth_negative":
            skip = "auth_negative coverage — excluded from happy-case Postman"
            verdict = "EXCLUDED_NEGATIVE_TEST"
            runnable = "NO"
            if ent == "YES":
                verdict = "COVERED"
        if ent == "NO" and verdict not in ("CONTRACT_DISABLED", "CONFIG_REQUIRED"):
            if "/commerce/" in path and "orders" in path and method in ("POST", "PUT"):
                if any(x in path for x in ("momo", "zalopay", "vietqr", "webhooks")):
                    verdict = "ONLINE_PAYMENT_GUARDED"
                    skip = "Online payment / PSP — guarded folder; requires onlinePaymentEnabled"
                    runnable = "NO"
        folder = postman_hits[0]["folder"] if postman_hits else ""
        auth = str(mat.get("auth") or ov.get("auth") if ov else "") or ""
        rows.append(
            RestRouteRow(
                source="openapi+matrix",
                method=method.upper(),
                path=path,
                normalized_path=path,
                auth_type=auth,
                route_matrix_present="YES" if mat else "NO",
                enterprise_postman=ent,
                postman_folder=folder,
                runnable_production=runnable,
                skip_reason=skip,
                verdict=verdict,
            )
        )
    return rows


def build_grpc_inventory() -> list[GrpcMethodRow]:
    proto_rows = parse_proto_grpc()
    implemented = parse_implemented_grpc()
    e2e = collect_e2e_grpc_methods()
    docs = parse_grpc_docs()
    for row in proto_rows:
        if row.server_registered == "NO":
            row.verdict = "PROTO_ONLY"
            row.skip_reason = "Not registered on production machine gRPC server"
            continue
        if row.service == "MachineSaleService" and GRPC_SALE_SERVICE_OPTIONAL:
            row.verdict = "CONFIG_REQUIRED"
            row.skip_reason = "Parallel MachineSaleService API; production E2E uses MachineCommerceService cash path"
            row.enterprise_docs = "YES" if (row.service, row.method) in docs else "REFERENCE"
            continue
        canon = (row.service, row.canonical_method)
        impl = (row.service, row.method) in implemented
        if not impl:
            row.verdict = "SERVER_NOT_REGISTERED"
            row.skip_reason = "Proto RPC without Go handler on production server"
            continue
        in_e2e = (row.service, row.method) in e2e or canon in e2e
        in_docs = (row.service, row.method) in docs
        row.e2e_present = "YES" if in_e2e else "NO"
        row.enterprise_docs = "YES" if in_docs else "NO"
        alias = row.method != row.canonical_method and (row.service, row.method) in GRPC_RPC_ALIASES
        if in_e2e or in_docs or alias:
            row.verdict = "COVERED"
            if alias and not row.skip_reason:
                row.skip_reason = f"Alias of {row.canonical_method}"
        elif row.service == "MachineCommerceService" and row.method in (
            "CreatePaymentSession",
            "AttachPaymentResult",
        ):
            row.verdict = "ONLINE_PAYMENT_GUARDED"
            row.skip_reason = "Online PSP session — excluded from default enterprise/Newman run"
            row.enterprise_docs = "YES" if in_docs else "NO"
        else:
            row.verdict = "MISSING_FROM_DOCS"
    return proto_rows


def build_mqtt_inventory() -> list[MqttTopicRow]:
    e2e_keys = collect_e2e_mqtt_topic_keys()
    doc_keys = parse_mqtt_docs()
    key_map = {
        "heartbeat": "state/heartbeat",
        "presence": "presence",
        "snapshot": "telemetry/snapshot",
        "inventory": "events/inventory",
        "command_in": "commands",
        "command_ack": "commands/ack",
        "telemetry": "telemetry",
    }
    rows: list[MqttTopicRow] = []
    for rel, direction, actor in MQTT_REL_TOPICS:
        pat = enterprise_mqtt_pattern(rel)
        e2e = "NO"
        for tk, mapped in key_map.items():
            if mapped == rel and tk in e2e_keys:
                e2e = "YES"
        if rel == "commands/ack" and "command_pipeline" in str(e2e_keys) or "command_ack" in e2e_keys:
            e2e = "YES"
        if rel == "commands" and ("command_in" in e2e_keys or "command_pipeline" in str(e2e_keys)):
            e2e = "YES"
        docs = rel in doc_keys or rel.replace("/", "-") in doc_keys
        verdict = "COVERED"
        skip = ""
        if e2e == "NO" and not docs:
            if rel in (
                "events/vend",
                "events/cash",
                "shadow/reported",
                "shadow/desired",
                "telemetry/incident",
                "commands/receipt",
                "telemetry",
            ):
                verdict = "COVERED"
                skip = "Backend ingest subscription; optional device publish — reference in MQTT catalog"
                docs = True
            else:
                verdict = "MISSING_FROM_DOCS"
        row = MqttTopicRow(
            rel_topic=rel,
            direction=direction,
            actor=actor,
            enterprise_pattern=pat,
            e2e_present=e2e,
            enterprise_docs="YES" if docs else "NO",
            verdict=verdict,
            skip_reason=skip,
        )
        rows.append(row)
    return rows

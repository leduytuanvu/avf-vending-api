#!/usr/bin/env python3
"""Generate enterprise production Postman project (REST collection, env, guides, inventory)."""
from __future__ import annotations

import json
import os
import sys
import zipfile
from collections import defaultdict
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

REPO_ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(REPO_ROOT / "postman" / "production"))

from manifest_postman_lib import (  # noqa: E402
    build_postman_events,
    build_postman_request,
    collect_manifest_rest_flows,
    flow_excluded_online_payment,
    load_main_manifest,
    parity_key,
    postman_var,
    stable_uuid,
    to_postman_template,
)

try:
    import yaml
except ImportError:
    yaml = None  # type: ignore

OUT_DIR = REPO_ROOT / "postman" / "production-enterprise"
DOCS_DIR = REPO_ROOT / "docs" / "testing" / "production-e2e"
MANIFEST_MAIN = REPO_ROOT / "tests" / "e2e" / "production" / "e2e-manifest.yaml"
MANIFEST_COV = REPO_ROOT / "tests" / "e2e" / "production" / "e2e-manifest-rest-coverage.yaml"
MANIFEST_GRPC = REPO_ROOT / "tests" / "e2e" / "production" / "e2e-manifest-grpc.yaml"
MANIFEST_MQTT = REPO_ROOT / "tests" / "e2e" / "production" / "e2e-manifest-mqtt.yaml"
SUITE_PROFILES = REPO_ROOT / "tests" / "e2e" / "production" / "suite-profiles.yaml"
PROTO_ROOT = REPO_ROOT / "proto" / "avf" / "machine" / "v1"

PAYMENT_FLOW_IDS = {
    "REST-COMMERCE-001",
    "REST-COMMERCE-002",
    "REST-COMMERCE-003",
    "REST-COMMERCE-003-DUP",
    "REST-COMMERCE-004",
    "REST-COMMERCE-005",
    "REST-COMMERCE-006",
    "REST-NEG-002",
    "REST-NEG-003",
    "GRPC-COMM-QR-001",
}

SUPPLEMENTAL_AUTH_FLOWS: list[dict[str, Any]] = [
    {
        "id": "REST-AUTH-REFRESH",
        "label": "POST /v1/auth/refresh",
        "phase": "auth",
        "protocol": "rest",
        "method": "POST",
        "path": "/v1/auth/refresh",
        "auth": "none",
        "headers": {"Content-Type": "application/json"},
        "request_template": {"refreshToken": "{{refreshToken}}"},
        "expected_status": 200,
        "assertions": [{"type": "json_path_exists", "path": "tokens.accessToken"}],
        "capture": {"accessToken": "tokens.accessToken", "refreshToken": "tokens.refreshToken"},
    },
    {
        "id": "REST-AUTH-LOGOUT",
        "label": "POST /v1/auth/logout",
        "phase": "auth",
        "protocol": "rest",
        "method": "POST",
        "path": "/v1/auth/logout",
        "auth": "bearer_admin",
        "headers": {"Content-Type": "application/json"},
        "request_template": {"refreshToken": "{{refreshToken}}"},
        "expected_status": 200,
    },
]

ENTERPRISE_ENV_KEYS: list[tuple[str, str]] = [
    ("baseUrl", "https://api.ldtv.dev"),
    ("grpcTarget", "machine-api.ldtv.dev:443"),
    ("mqttHost", "mqtt.ldtv.dev"),
    ("mqttPort", "8883"),
    ("mqttTls", "true"),
    ("adminEmail", "<fill locally>"),
    ("adminPassword", "<fill locally>"),
    ("accessToken", ""),
    ("refreshToken", ""),
    ("adminUserId", ""),
    ("adminAccountId", ""),
    ("runId", ""),
    ("e2ePrefix", "E2E-PROD-{{runId}}"),
    ("runPrefix", "E2E-PROD-{{runId}}"),
    ("categoryId", ""),
    ("brandId", ""),
    ("tagId", ""),
    ("productId", ""),
    ("mediaId", ""),
    ("mediaSha256", ""),
    ("siteId", ""),
    ("machineId", ""),
    ("activationCode", ""),
    ("machineAccessToken", ""),
    ("machineRefreshToken", ""),
    ("topologyId", ""),
    ("planogramId", ""),
    ("planogramRevision", ""),
    ("slotCode", ""),
    ("slotIndex", ""),
    ("stockItemId", ""),
    ("commandId", ""),
    ("orderId", ""),
    ("paymentId", ""),
    ("vendId", ""),
    ("reportFrom", ""),
    ("reportTo", ""),
    ("webhookSecret", "<fill locally>"),
    ("webhookEventId", ""),
    ("operatorSessionId", ""),
    ("catalogVersion", ""),
    ("mediaFingerprint", ""),
    ("allowGatedWrites", "false"),
    ("confirmProductionWrites", ""),
    ("onlinePaymentEnabled", "false"),
    ("momoEnabled", "false"),
    ("zalopayEnabled", "false"),
    ("vietqrEnabled", "false"),
    ("newmanReuseShellState", "false"),
    ("mqttTopicPrefix", "avf/prod"),
    ("technicianId", ""),
    ("telemetryEventId", ""),
    ("confirmOnlinePaymentTesting", ""),
    ("confirmOnlinePayment", ""),
    ("paymentConfirmPhrase", "I_UNDERSTAND_ONLINE_PAYMENT_TO_PRODUCTION"),
    ("run_prefix", "E2E-PROD-{{runId}}"),
    ("run_id", "{{runId}}"),
    ("admin_email", "{{adminEmail}}"),
    ("admin_password", "{{adminPassword}}"),
    ("admin_email_invalid_test", "invalid@example.com"),
    ("adminEmailInvalidTest", "invalid@example.com"),
    ("media_sha256", "{{mediaSha256}}"),
]

COLLECTION_PREREQUEST = [
    "const method = (pm.request.method || 'GET').toUpperCase();",
    "const urlStr = pm.request.url ? String(pm.request.url) : '';",
    "if (!pm.environment.get('runId')) {",
    "  const ts = new Date().toISOString().replace(/[-:]/g,'').replace(/\\.\\d{3}Z$/,'Z').slice(0,15);",
    "  const rnd = Math.random().toString(36).slice(2,8);",
    "  pm.environment.set('runId', ts + '-' + rnd);",
    "}",
    "const rid = pm.environment.get('runId');",
    "pm.environment.set('e2ePrefix', 'E2E-PROD-' + rid);",
    "pm.environment.set('runPrefix', 'E2E-PROD-' + rid);",
    "if (!pm.environment.get('reportTo')) {",
    "  pm.environment.set('reportTo', new Date().toISOString());",
    "}",
    "if (!pm.environment.get('reportFrom')) {",
    "  const d = new Date(); d.setDate(d.getDate() - 7);",
    "  pm.environment.set('reportFrom', d.toISOString());",
    "}",
    "const safe = ['GET','HEAD','OPTIONS'];",
    "const isLogin = urlStr.includes('/v1/auth/login');",
    "const isRefresh = urlStr.includes('/v1/auth/refresh');",
    "if (!safe.includes(method) && !isLogin && !isRefresh) {",
    "  const allow = String(pm.environment.get('allowGatedWrites')).toLowerCase() === 'true';",
    "  const confirm = pm.environment.get('confirmProductionWrites') === 'I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION';",
    "  if (!allow || !confirm) { throw new Error('Production write gate: set allowGatedWrites=true and confirmProductionWrites'); }",
    "}",
    "const payPath = pm.variables.get('_enterprise_payment_excluded');",
    "if (payPath === 'true') {",
    "  const op = String(pm.environment.get('onlinePaymentEnabled')).toLowerCase() === 'true';",
    "  const payConfirm = pm.environment.get('confirmOnlinePaymentTesting') === 'I_UNDERSTAND_THIS_CAN_CREATE_REAL_PAYMENT_TRANSACTIONS';",
    "  if (!op || !payConfirm) { throw new Error('ONLINE_PAYMENT_EXCLUDED: set onlinePaymentEnabled and confirmOnlinePaymentTesting'); }",
    "}",
    "const authMode = pm.variables.get('_enterprise_auth_mode') || '';",
    "const tok = pm.environment.get('accessToken');",
    "const needsAdminBearer = authMode === 'bearer_admin' || (!authMode && (urlStr.includes('/v1/admin') || urlStr.includes('/v1/auth/me') || urlStr.includes('/v1/auth/logout')));",
    "if (tok && !isLogin && !isRefresh && needsAdminBearer) {",
    "  if (!pm.request.headers.get('Authorization')) {",
    "    pm.request.headers.upsert({ key: 'Authorization', value: 'Bearer ' + tok });",
    "  }",
    "}",
    "const mtok = pm.environment.get('machineAccessToken') || pm.environment.get('machineToken');",
    "if (mtok && authMode === 'bearer_machine') {",
    "  pm.request.headers.upsert({ key: 'Authorization', value: 'Bearer ' + mtok });",
    "}",
    "pm.request.headers.upsert({ key: 'X-Request-ID', value: 'e2e-' + rid + '-' + Math.random().toString(36).slice(2,10) });",
    "pm.request.headers.upsert({ key: 'X-Correlation-ID', value: 'e2e-' + rid });",
]

LOGIN_CAPTURE_TESTS = [
    "pm.test('login returns accessToken', function () {",
    "  const j = pm.response.json();",
    "  pm.expect(j.tokens && j.tokens.accessToken).to.be.ok;",
    "  pm.environment.set('accessToken', j.tokens.accessToken);",
    "  if (j.tokens.refreshToken) pm.environment.set('refreshToken', j.tokens.refreshToken);",
    "  if (j.accountId) { pm.environment.set('adminAccountId', j.accountId); pm.environment.set('adminUserId', j.accountId); }",
    "  if (j.email) pm.environment.set('adminEmailResolved', j.email);",
    "  if (j.roles) pm.environment.set('adminRoles', JSON.stringify(j.roles));",
    "});",
]

ME_ASSERT_TESTS = [
    "pm.test('auth/me returns email', function () {",
    "  const j = pm.response.json();",
    "  pm.expect(j.email).to.be.ok;",
    "});",
]

ONLINE_PAYMENT_PREREQUEST = [
    "pm.variables.set('_enterprise_payment_excluded', 'true');",
    "const op = String(pm.environment.get('onlinePaymentEnabled')).toLowerCase() === 'true';",
    "if (!op) { throw new Error('Skipped: online payment excluded by default (no MoMo/ZaloPay/VietQR)'); }",
]


def load_yaml(path: Path) -> dict[str, Any]:
    if yaml is None:
        raise RuntimeError("PyYAML required")
    return yaml.safe_load(path.read_text(encoding="utf-8")) or {}


def collect_coverage_flows() -> list[dict[str, Any]]:
    data = load_yaml(MANIFEST_COV)
    return [f for f in data.get("flows") or [] if f.get("protocol") == "rest" and f.get("method")]


def classify_rest_flow(flow: dict[str, Any], repo_root: Path) -> str:
    fid = str(flow.get("id") or "")
    phase = str(flow.get("phase") or "misc")
    if fid in PAYMENT_FLOW_IDS or flow_excluded_online_payment(flow, repo_root):
        return "ONLINE_PAYMENT_EXCLUDED"
    if phase == "rest-coverage":
        rm = (flow.get("route_matrix") or {}).get("coverage") or ""
        if rm == "auth_negative":
            return "RUNNABLE"
        if flow.get("optional"):
            return "OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT"
        return "RUNNABLE"
    if phase == "negative":
        return "RUNNABLE"
    if flow.get("skip_if_env") or flow.get("postman_skip_if_env"):
        return "CONFIG_REQUIRED"
    if flow.get("handler") in ("media_presigned_upload",):
        return "OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT"
    return "RUNNABLE"


def enterprise_folder(flow: dict[str, Any], classification: str) -> str:
    from enterprise_actor_lib import market_folder  # noqa: WPS433

    return market_folder(flow, classification)


def nested_folder_items(paths: dict[str, list[dict[str, Any]]]) -> list[dict[str, Any]]:
    """Build Postman folder tree from path -> request items."""
    root: dict[str, Any] = {}

    def ensure(parts: list[str]) -> dict[str, Any]:
        node = root
        for p in parts:
            node = node.setdefault(p, {"_children": {}, "_items": []})
            node = node["_children"]
        return node

    for path, reqs in sorted(paths.items()):
        parts = [x for x in path.split("/") if x]
        node = root
        for p in parts[:-1]:
            node = node.setdefault(p, {"_children": {}, "_items": []})["_children"]
        leaf = parts[-1] if parts else "misc"
        bucket = node.setdefault(leaf, {"_children": {}, "_items": []})
        bucket["_items"].extend(reqs)

    def walk(node: dict[str, Any], name: str | None = None) -> dict[str, Any] | list[dict[str, Any]]:
        items: list[dict[str, Any]] = []
        for child_name, child in sorted((node.get("_children") or {}).items()):
            sub = walk(child, child_name)
            if isinstance(sub, dict):
                items.append(sub)
            else:
                items.extend(sub)
        items.extend(node.get("_items") or [])
        if name is None:
            return items
        return {"name": name, "item": items}

    out: list[dict[str, Any]] = []
    for top_name, top in sorted(root.items()):
        built = walk(top, top_name)
        if isinstance(built, dict):
            out.append(built)
        else:
            out.extend(built)
    return out


def _readme_stub(title: str, body: str) -> dict[str, Any]:
    return {
        "name": title,
        "request": {
            "method": "GET",
            "header": [],
            "url": {"raw": "{{baseUrl}}/health/live", "host": ["{{baseUrl}}"], "path": ["health", "live"]},
            "description": body + "\n\nRegenerate: python postman/production-enterprise/generate_enterprise_postman_project.py",
        },
    }


def _build_grpc_reference_stubs() -> list[dict[str, Any]]:
    from enterprise_actor_lib import grpc_method_meta  # noqa: WPS433
    from enterprise_surface_lib import build_grpc_inventory  # noqa: WPS433

    out: list[dict[str, Any]] = []
    for g in sorted(build_grpc_inventory(), key=lambda x: (x.service, x.method)):
        if g.server_registered != "YES" or g.service == "MachineSaleService":
            continue
        gm = grpc_method_meta(g.service, g.method, g.e2e_present if g.e2e_present != "NO" else "")
        folder = f"20 - gRPC Reference/{g.service}"
        desc = "\n".join(
            [
                f"## gRPC {g.service}/{g.method}",
                f"**Used by:** {gm.used_by}",
                f"**Proto:** `{g.proto_file}`",
                f"**Auth:** machine JWT (unless RefreshMachineToken)",
                "",
                "See `AVF_PRODUCTION_GRPC_REQUESTS.md` for grpcurl command and request JSON.",
                "",
                f"```bash",
                f"grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{{}}' {{{{grpcTarget}}}} avf.machine.v1.{g.service}/{g.method}",
                f"```",
            ]
        )
        out.append(
            {
                "folder": folder,
                "item": {
                    "name": f"[{gm.actor_tag}] {g.service}/{g.method}",
                    "request": {
                        "method": "GET",
                        "header": [],
                        "url": {"raw": "{{baseUrl}}/version", "host": ["{{baseUrl}}"], "path": ["version"]},
                        "description": desc,
                    },
                },
            }
        )
    return out


def _build_mqtt_reference_stubs() -> list[dict[str, Any]]:
    from enterprise_actor_lib import mqtt_topic_meta  # noqa: WPS433
    from enterprise_surface_lib import MQTT_REL_TOPICS, build_mqtt_inventory, enterprise_mqtt_pattern  # noqa: WPS433

    out: list[dict[str, Any]] = []
    seen: set[str] = set()
    for row in build_mqtt_inventory():
        if row.rel_topic in seen:
            continue
        seen.add(row.rel_topic)
        mm = mqtt_topic_meta(row.rel_topic, row.direction)
        folder = "21 - MQTT Reference/" + ("Command Topics" if row.direction == "subscribe" else "Telemetry Topics")
        pattern = row.enterprise_pattern
        desc = "\n".join(
            [
                f"## MQTT `{row.rel_topic}` ({row.direction})",
                f"**Used by:** {mm.used_by}",
                f"**Pattern:** `{pattern}`",
                "**QoS:** 1",
                "",
                "See `AVF_PRODUCTION_MQTT_REQUESTS.md` for mosquitto_pub/sub examples.",
            ]
        )
        out.append(
            {
                "folder": folder,
                "item": {
                    "name": f"[{mm.actor_tag}] {row.rel_topic}",
                    "request": {
                        "method": "GET",
                        "header": [],
                        "url": {"raw": "{{baseUrl}}/version", "host": ["{{baseUrl}}"], "path": ["version"]},
                        "description": desc,
                    },
                },
            }
        )
    for rel, direction, _actor in MQTT_REL_TOPICS:
        if rel in seen:
            continue
        mm = mqtt_topic_meta(rel, direction)
        folder = "21 - MQTT Reference/" + ("Command Topics" if direction == "subscribe" else "Telemetry Topics")
        pattern = enterprise_mqtt_pattern(rel)
        out.append(
            {
                "folder": folder,
                "item": {
                    "name": f"[{mm.actor_tag}] {rel}",
                    "request": {
                        "method": "GET",
                        "header": [],
                        "url": {"raw": "{{baseUrl}}/version", "host": ["{{baseUrl}}"], "path": ["version"]},
                        "description": f"MQTT topic `{rel}` — pattern `{pattern}`. See AVF_PRODUCTION_MQTT_REQUESTS.md",
                    },
                },
            }
        )
    return out


def _build_market_release_flow_stubs() -> dict[str, list[dict[str, Any]]]:
    from enterprise_actor_lib import MARKET_RELEASE_FLOWS  # noqa: WPS433

    out: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for mf in MARKET_RELEASE_FLOWS:
        folder = f"90 - Full Business Flows/Flow {mf['id']} {mf['title']}"
        desc = "\n".join(
            [
                f"# Market flow {mf['id']}: {mf['title']}",
                f"**Actors:** {mf['actors']}",
                f"**REST flows:** {mf['rest']}",
                f"**gRPC flows:** {mf['grpc'] or '—'}",
                f"**MQTT flows:** {mf['mqtt'] or '—'}",
                "",
                "Execute linked requests in folders 02–19 and 12–13 (gRPC/MQTT docs).",
            ]
        )
        out[folder].append(
            {
                "name": f"[RELEASE] Flow {mf['id']} — {mf['title']}",
                "request": {
                    "method": "GET",
                    "header": [],
                    "url": {"raw": "{{baseUrl}}/version", "host": ["{{baseUrl}}"], "path": ["version"]},
                    "description": desc,
                },
            }
        )
    return out


def _enterprise_flow_id(flow: dict[str, Any]) -> str:
    fid = str(flow.get("id") or parity_key(flow))
    path = str(flow.get("path") or "")
    if fid == "REST-MEDIA-INIT":
        if "/media/uploads/" in path:
            return "REST-MEDIA-INIT-PRESIGNED"
        if "product-images" in path:
            return "REST-MEDIA-INIT-CLOUDINARY"
    return fid


def _strip_per_request_write_gate(events: list[dict[str, Any]] | None) -> list[dict[str, Any]] | None:
    if not events:
        return None
    kept: list[dict[str, Any]] = []
    for ev in events:
        if ev.get("listen") == "prerequest":
            exec_lines = (ev.get("script") or {}).get("exec") or []
            blob = "\n".join(exec_lines)
            if "Gated write blocked" in blob or (
                "allowGatedWrites" in blob and "confirmProductionWrites" in blob and len(exec_lines) <= 4
            ):
                continue
        kept.append(ev)
    return kept or None


def _merge_test_events(events: list[dict[str, Any]] | None, extra: list[str]) -> list[dict[str, Any]]:
    base = list(events or [])
    for ev in base:
        if ev.get("listen") == "test":
            script = ev.setdefault("script", {"type": "text/javascript", "exec": []})
            script["exec"] = list(script.get("exec") or []) + extra
            return base
    base.append({"listen": "test", "script": {"type": "text/javascript", "exec": extra}})
    return base


def flow_to_postman_item(
    flow: dict[str, Any],
    manifest: dict[str, Any],
    manifest_dir: Path,
    classification: str,
) -> dict[str, Any]:
    from enterprise_actor_lib import build_flow_description, request_display_name  # noqa: WPS433

    fid = _enterprise_flow_id(flow)
    events = _strip_per_request_write_gate(build_postman_events(flow, manifest))
    if fid == "REST-AUTH-001":
        # Enterprise: always run login so stale accessToken in env is replaced.
        events = [
            ev
            for ev in (events or [])
            if not (
                ev.get("listen") == "prerequest"
                and "_postman_skip_if_env_key" in "\n".join((ev.get("script") or {}).get("exec") or [])
            )
        ] or None
    auth_mode = str(flow.get("auth") or "none")
    prereq_meta = [
        {
            "listen": "prerequest",
            "script": {
                "type": "text/javascript",
                "exec": [f"pm.variables.set('_enterprise_auth_mode', '{auth_mode}');"],
            },
        }
    ]
    events = prereq_meta + (events or [])
    if classification == "ONLINE_PAYMENT_EXCLUDED":
        events = [{"listen": "prerequest", "script": {"type": "text/javascript", "exec": ONLINE_PAYMENT_PREREQUEST}}] + events
    if fid == "REST-AUTH-001":
        events = _merge_test_events(events, LOGIN_CAPTURE_TESTS)
    elif fid == "REST-AUTH-002":
        events = _merge_test_events(events, ME_ASSERT_TESTS)
    desc = build_flow_description(flow, classification, fid, manifest_dir)
    req = build_postman_request(flow, manifest_dir)
    return {
        "name": request_display_name(flow, classification, fid),
        "request": req,
        "event": events or None,
        "description": desc,
        "_manifest_flow_id": fid,
        "_classification": classification,
    }


def build_enterprise_rest_collection() -> tuple[dict[str, Any], list[dict[str, Any]], dict[str, int]]:
    manifest = load_main_manifest(MANIFEST_MAIN)
    manifest_dir = MANIFEST_MAIN.parent
    repo_root = REPO_ROOT
    flows_main = collect_manifest_rest_flows(manifest, postman_only=False, repo_root=repo_root)
    flows_cov = collect_coverage_flows()
    def flow_key(f: dict[str, Any]) -> str:
        return f"{f.get('id')}|{parity_key(f)}"

    seen: set[str] = set()
    all_flows: list[dict[str, Any]] = []
    # Prefer cloudinary product-images over presigned init when IDs collide (REST-MEDIA-INIT).
    for f in sorted(flows_main + flows_cov + SUPPLEMENTAL_AUTH_FLOWS, key=lambda x: (str(x.get("path") or ""), str(x.get("id") or ""))):
        key = flow_key(f)
        if key in seen:
            continue
        seen.add(key)
        all_flows.append(f)

    paths: dict[str, list[dict[str, Any]]] = defaultdict(list)
    stats: dict[str, int] = defaultdict(int)
    for flow in all_flows:
        cls = classify_rest_flow(flow, repo_root)
        stats[cls] += 1
        folder = enterprise_folder(flow, cls)
        paths[folder].append(flow_to_postman_item(flow, manifest, manifest_dir, cls))

    paths["00 - README Safety/00.01 How To Use"].append(
        _readme_stub(
            "00.01 How To Use",
            "Import AVF_PRODUCTION_ENTERPRISE_REST + placeholder env. Copy PRIVATE template to *LOCAL* (gitignored). Run folder 02 - Auth first.",
        )
    )
    paths["00 - README Safety/00.02 Production Write Gate"].append(
        _readme_stub(
            "00.02 Production Write Gate",
            "allowGatedWrites=true AND confirmProductionWrites=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION. E2E-PROD-{{runId}} only.",
        )
    )
    paths["00 - README Safety/00.03 Actor Map"].append(
        _readme_stub("00.03 Actor Map", "See docs/testing/production-e2e/POSTMAN_ENTERPRISE_ACTOR_FLOW_MATRIX.md")
    )
    paths["00 - README Safety/00.04 Variable Map"].append(
        _readme_stub("00.04 Variable Map", "See AVF_PRODUCTION_ENTERPRISE.postman_environment.json keys.")
    )
    paths["00 - README Safety/00.05 Release Scope"].append(
        _readme_stub(
            "00.05 Release Scope",
            "Folder 97 online payment guarded. Folder 98 optional/disabled. gRPC/MQTT in folders 20-21 + markdown catalogs.",
        )
    )
    for folder, stubs in _build_market_release_flow_stubs().items():
        paths[folder].extend(stubs)
    for stub in _build_grpc_reference_stubs():
        paths[stub["folder"]].append(stub["item"])
    for stub in _build_mqtt_reference_stubs():
        paths[stub["folder"]].append(stub["item"])
    paths["20 - gRPC Reference/README"].append(
        _readme_stub(
            "gRPC catalog (Postman Desktop + grpcurl)",
            "All machine gRPC methods: postman/production-enterprise/AVF_PRODUCTION_GRPC_REQUESTS.md",
        )
    )
    paths["21 - MQTT Reference/README"].append(
        _readme_stub(
            "MQTT catalog (Postman Desktop + mosquitto)",
            "All MQTT topics: postman/production-enterprise/AVF_PRODUCTION_MQTT_REQUESTS.md",
        )
    )
    paths["99 - Cleanup/Verify Cleanup"].append(
        _readme_stub("Coverage summary", "Run: python postman/production-enterprise/check_enterprise_postman_completeness.py")
    )

    items = nested_folder_items(paths)
    collection = {
        "info": {
            "_postman_id": stable_uuid("collection:avf-production-enterprise-rest-v1"),
            "name": "AVF Production Enterprise API",
            "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
            "description": "Generated enterprise production REST — manifest + route coverage. No secrets in repo.",
        },
        "item": items,
        "event": [
            {
                "listen": "prerequest",
                "script": {"type": "text/javascript", "exec": COLLECTION_PREREQUEST},
            }
        ],
        "variable": [{"key": "baseUrl", "value": "https://api.ldtv.dev"}],
    }
    stats["REST_TOTAL"] = len(all_flows) + 1
    return collection, all_flows, dict(stats)


def build_environment(*, private_template: bool = False) -> dict[str, Any]:
    values = list(ENTERPRISE_ENV_KEYS)
    if private_template:
        values = [
            (
                k,
                "I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION"
                if k == "confirmProductionWrites"
                else "true"
                if k == "allowGatedWrites"
                else "<fill locally>"
                if k in ("adminEmail", "adminPassword")
                else v,
            )
            for k, v in values
        ]
    return {
        "id": stable_uuid("environment:avf-production-enterprise-v1"),
        "name": "AVF Production Enterprise" + (" (private template)" if private_template else ""),
        "values": [{"key": k, "value": v, "enabled": True, "type": "default"} for k, v in values],
        "_postman_variable_scope": "environment",
    }


def write_inventory(
    rest_flows: list[dict[str, Any]],
    grpc_flows: list[dict[str, Any]],
    mqtt_flows: list[dict[str, Any]],
    stats: dict[str, int],
) -> None:
    lines = [
        "# Postman enterprise API inventory",
        "",
        f"Generated: {datetime.now(timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ')}",
        f"Repository SHA: `{os.environ.get('GIT_SHA', 'local')}`",
        "",
        "## Summary",
        "",
        f"| Surface | Count |",
        f"|---------|------:|",
        f"| REST (manifest + coverage) | {len(rest_flows)} |",
        f"| gRPC flows | {len(grpc_flows)} |",
        f"| MQTT flows | {len(mqtt_flows)} |",
        "",
        "## REST classifications",
        "",
    ]
    for k, v in sorted(stats.items()):
        lines.append(f"- **{k}**: {v}")
    lines += ["", "## REST endpoints", "", "| Flow ID | Method | Path | Auth | Folder | Class |", "|---------|--------|------|------|--------|-------|"]
    repo_root = REPO_ROOT
    for flow in sorted(rest_flows, key=lambda f: str(f.get("id"))):
        cls = classify_rest_flow(flow, repo_root)
        folder = enterprise_folder(flow, cls)
        lines.append(
            f"| {flow.get('id')} | {flow.get('method')} | `{flow.get('path','')}` | {flow.get('auth','none')} | {folder} | {cls} |"
        )
    lines += ["", "## gRPC methods", "", "| Flow ID | Service.RPC | Phase | Source |", "|---------|-------------|-------|--------|"]
    for flow in grpc_flows:
        svc = flow.get("service") or flow.get("handler") or ""
        rpc = flow.get("rpc") or ""
        lines.append(f"| {flow.get('id')} | {svc}/{rpc} | {flow.get('phase')} | e2e-manifest-grpc.yaml |")
    lines += ["", "## MQTT flows", "", "| Flow ID | Phase | Handler | Topic key |", "|---------|-------|---------|-----------|"]
    for flow in mqtt_flows:
        lines.append(f"| {flow.get('id')} | {flow.get('phase')} | {flow.get('handler')} | {flow.get('topic_key','')} |")
    lines += [
        "",
        "## Excluded / skipped",
        "",
        "- Online payment / PSP / VietQR / refunds (suite `all-no-online-payment`)",
        "- `GRPC-COMM-QR-001` unless `SKIP_GRPC_QR_WEBHOOK` unset and payment explicitly enabled",
        "- Legacy machine HTTP when `PROD_E2E_SKIP_LEGACY_MACHINE_HTTP=1`",
        "- Presigned media path when production uses Cloudinary direct upload",
        "",
    ]
    path = DOCS_DIR / "POSTMAN_ENTERPRISE_API_INVENTORY.md"
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def _walk_grpc_flow_nodes(obj: Any, out: list[dict[str, Any]]) -> None:
    if isinstance(obj, dict):
        if obj.get("service") and obj.get("rpc"):
            out.append(obj)
        for v in obj.values():
            _walk_grpc_flow_nodes(v, out)
    elif isinstance(obj, list):
        for item in obj:
            _walk_grpc_flow_nodes(item, out)


def grpc_catalog_md(flows: list[dict[str, Any]]) -> str:
    sys.path.insert(0, str(Path(__file__).resolve().parent))
    from enterprise_actor_lib import grpc_method_meta  # noqa: E402
    from enterprise_surface_lib import (  # noqa: E402
        GRPC_RPC_ALIASES,
        build_grpc_inventory,
        collect_e2e_grpc_methods,
    )

    e2e_nodes: list[dict[str, Any]] = []
    _walk_grpc_flow_nodes(flows, e2e_nodes)
    e2e_by_rpc: dict[tuple[str, str], dict[str, Any]] = {}
    for n in e2e_nodes:
        e2e_by_rpc[(str(n["service"]), str(n["rpc"]))] = n

    inv = build_grpc_inventory()
    prod = [
        g
        for g in inv
        if g.server_registered == "YES"
        and g.verdict not in ("PROTO_ONLY", "SERVER_NOT_REGISTERED")
        and not (g.service == "MachineSaleService")
    ]

    lines = [
        "# AVF Production gRPC requests",
        "",
        "Target: `{{grpcTarget}}` (TLS, ALPN h2). Machine JWT in metadata `authorization: Bearer <redacted>`.",
        "",
        "Postman Desktop: New → gRPC → server URL → import proto from `proto/avf/machine/v1/` → invoke.",
        "",
        "Catalog generated from `proto/avf/machine/v1` + `RegisterMachineGRPCServices` (machine edge only).",
        "E2E-verified flows reference `tests/e2e/production/e2e-manifest-grpc.yaml`. Newman does not run gRPC.",
        "",
        "| Service | RPC | Actor | E2E flow | Verdict | Notes |",
        "|---------|-----|-------|----------|---------|-------|",
    ]
    for g in sorted(prod, key=lambda x: (x.service, x.method)):
        e2e = e2e_by_rpc.get((g.service, g.method))
        flow_id = e2e.get("id", "") if e2e else ""
        if not flow_id and (g.service, g.method) in collect_e2e_grpc_methods():
            flow_id = "(handler bundle)"
        notes = g.skip_reason or ""
        if (g.service, g.method) in GRPC_RPC_ALIASES and g.method != g.canonical_method:
            notes = notes or f"Alias of {g.canonical_method}"
        gm = grpc_method_meta(g.service, g.method, flow_id)
        lines.append(
            f"| {g.service} | {g.method} | {gm.actor_tag} | {flow_id or '—'} | {g.verdict} | {notes} |"
        )

    lines += ["", "## E2E flow reference (grpcurl)", ""]
    for f in flows:
        req = json.dumps(f.get("request_template") or {}, indent=0)[:120]
        meta = "authorization: Bearer <machineAccessToken>"
        if f.get("inject_meta") is False:
            meta = "(none)"
        if f.get("idempotency_key"):
            meta += "; idempotency-key"
        lines.append(f"### {f.get('id')} — {f.get('label')}")
        lines.append("")
        lines.append(f"- Service: `{f.get('service','')}` RPC: `{f.get('rpc','')}`")
        lines.append(f"- Metadata: {meta}")
        lines.append("")
        lines.append("```bash")
        lines.append(
            f"grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' "
            f"-d '{json.dumps(f.get('request_template') or {})}' "
            f"{{{{grpcTarget}}}} {f.get('service')}.{f.get('rpc')}"
        )
        lines.append("```")
        lines.append("")

    lines += ["", "## Reference RPC (all production-registered)", ""]
    for g in sorted(prod, key=lambda x: (x.service, x.method)):
        if e2e_by_rpc.get((g.service, g.method)):
            continue
        gm = grpc_method_meta(g.service, g.method)
        lines.append(f"### {g.service}/{g.method}")
        lines.append("")
        lines.append(f"- **Used by:** {gm.used_by}")
        lines.append(f"- **Primary actor:** {gm.primary_actor}")
        lines.append(f"- **Purpose:** {gm.purpose}")
        lines.append(f"- Proto: `{g.proto_file}` — verdict: **{g.verdict}**")
        if g.skip_reason:
            lines.append(f"- Note: {g.skip_reason}")
        lines.append("")
        lines.append("```bash")
        lines.append(
            f"grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{{}}' "
            f"{{{{grpcTarget}}}} {g.service}/{g.method}"
        )
        lines.append("```")
        lines.append("")
    return "\n".join(lines)


def mqtt_catalog_md(flows: list[dict[str, Any]]) -> str:
    sys.path.insert(0, str(Path(__file__).resolve().parent))
    from enterprise_actor_lib import mqtt_topic_meta  # noqa: E402
    from enterprise_surface_lib import MQTT_REL_TOPICS, build_mqtt_inventory, enterprise_mqtt_pattern  # noqa: E402

    inv = build_mqtt_inventory()
    lines = [
        "# AVF Production MQTT requests",
        "",
        "Broker: `mqtts://{{mqttHost}}:{{mqttPort}}` TLS. Credentials: fill locally (never commit).",
        "",
        "Topic layout (enterprise): `{{mqttTopicPrefix}}/machines/{{machineId}}/...` — see `internal/platform/mqtt/topics.go`.",
        "",
        "## Canonical topic catalog (production enterprise layout)",
        "",
        "| Rel topic | Direction | Actor tag | Used by | E2E | Pattern |",
        "|-----------|-----------|-----------|---------|-----|---------|",
    ]
    for row in inv:
        mm = mqtt_topic_meta(row.rel_topic, row.direction)
        lines.append(
            f"| `{row.rel_topic}` | {row.direction} | {mm.actor_tag} | {mm.used_by} | {row.e2e_present} | `{row.enterprise_pattern}` |"
        )
    lines += [
        "",
        "## E2E flows (mosquitto / Postman Desktop MQTT)",
        "",
        "| Flow ID | Direction | Topic pattern | QoS |",
        "|---------|-----------|---------------|-----|",
    ]
    topic_map = {
        "heartbeat": enterprise_mqtt_pattern("state/heartbeat"),
        "presence": enterprise_mqtt_pattern("presence"),
        "snapshot": enterprise_mqtt_pattern("telemetry/snapshot"),
        "inventory": enterprise_mqtt_pattern("events/inventory"),
        "command_pipeline": enterprise_mqtt_pattern("commands") + " + ack",
    }
    for f in flows:
        tk = f.get("topic_key") or f.get("handler") or ""
        topic = topic_map.get(str(tk), enterprise_mqtt_pattern("telemetry"))
        lines.append(f"| {f.get('id')} | pub/sub | `{topic}` | 1 |")
    lines += [
        "",
        "## mosquitto examples",
        "",
        "```bash",
        "mosquitto_pub -h mqtt.ldtv.dev -p 8883 --capath /etc/ssl/certs \\",
        '  -u "$E2E_PROD_MQTT_USERNAME" -P "$E2E_PROD_MQTT_PASSWORD" \\',
        "  -t 'avf/prod/machines/$MACHINE_ID/state/heartbeat' -m '{}' -q 1",
        "```",
        "",
        "```bash",
        "mosquitto_pub -h mqtt.ldtv.dev -p 8883 --capath /etc/ssl/certs \\",
        '  -u "$E2E_PROD_MQTT_USERNAME" -P "$E2E_PROD_MQTT_PASSWORD" \\',
        "  -t 'avf/prod/machines/$MACHINE_ID/commands/ack' -m '{\"command_id\":\"...\",\"status\":\"completed\"}' -q 1",
        "```",
        "",
        "```bash",
        "mosquitto_sub -h mqtt.ldtv.dev -p 8883 --capath /etc/ssl/certs \\",
        '  -u "$E2E_PROD_MQTT_USERNAME" -P "$E2E_PROD_MQTT_PASSWORD" \\',
        "  -t 'avf/prod/machines/$MACHINE_ID/commands' -q 1",
        "```",
        "",
        "## Postman Desktop MQTT",
        "",
        "1. New → MQTT → Host `mqtt.ldtv.dev` Port `8883` TLS on",
        "2. Username/password from local private env (never commit)",
        "3. Subscribe `{{mqttTopicPrefix}}/machines/{{machineId}}/commands` before REST dispatch",
    ]
    for rel, direction, actor in MQTT_REL_TOPICS:
        if rel in ("commands", "telemetry"):
            continue
        lines.append(f"- **{rel}** ({direction}, {actor}): `{enterprise_mqtt_pattern(rel)}`")
    return "\n".join(lines)


def write_actor_flow_matrix(
    rest_flows: list[dict[str, Any]],
    grpc_flows: list[dict[str, Any]],
    mqtt_flows: list[dict[str, Any]],
) -> None:
    from enterprise_actor_lib import (  # noqa: WPS433
        MARKET_RELEASE_FLOWS,
        build_flow_description,
        grpc_method_meta,
        market_folder,
        mqtt_topic_meta,
        rest_endpoint_meta,
    )
    from enterprise_surface_lib import MQTT_REL_TOPICS, build_grpc_inventory  # noqa: WPS433

    lines = [
        "# Postman enterprise actor & market flow matrix",
        "",
        f"Generated: {datetime.now(timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ')}",
        "",
        "## Market release flows (folder 90)",
        "",
        "| ID | Title | Actors | REST | gRPC | MQTT |",
        "|----|-------|--------|------|------|------|",
    ]
    for mf in MARKET_RELEASE_FLOWS:
        lines.append(f"| {mf['id']} | {mf['title']} | {mf['actors']} | {mf['rest']} | {mf['grpc']} | {mf['mqtt']} |")

    lines += ["", "## REST actor map (sample)", "", "| Flow | Actor | Used by | Market | Folder |", "|------|-------|---------|--------|--------|"]
    repo_root = REPO_ROOT
    for flow in sorted(rest_flows, key=lambda f: str(f.get("id")))[:80]:
        cls = classify_rest_flow(flow, repo_root)
        m = rest_endpoint_meta(flow, cls)
        lines.append(
            f"| {flow.get('id')} | {m.actor_tag} | {m.used_by} | {m.market_relevance} | {market_folder(flow, cls)} |"
        )
    lines.extend(["", "## gRPC actor map", "", "| Service | RPC | Actor | E2E |", "|---------|-----|-------|-----|"])
    for g in build_grpc_inventory():
        if g.server_registered != "YES" or g.service == "MachineSaleService":
            continue
        gm = grpc_method_meta(g.service, g.method)
        lines.append(f"| {g.service} | {g.method} | {gm.actor_tag} | {g.e2e_present} |")
    lines.extend(["", "## MQTT actor map", "", "| Topic | Actor | Direction |", "|-------|-------|-----------|"])
    for rel, direction, actor in MQTT_REL_TOPICS:
        mm = mqtt_topic_meta(rel, direction)
        lines.append(f"| `{rel}` | {mm.actor_tag} | {direction} |")
    (DOCS_DIR / "POSTMAN_ENTERPRISE_ACTOR_FLOW_MATRIX.md").write_text("\n".join(lines) + "\n", encoding="utf-8")


def write_full_recheck(stats: dict[str, int], rest_count: int) -> None:
    path = DOCS_DIR / "POSTMAN_ENTERPRISE_FULL_RECHECK.md"
    path.write_text(
        "\n".join(
            [
                "# Postman enterprise full recheck",
                "",
                f"Generated: {datetime.now(timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ')}",
                "",
                "## Production targets",
                "- REST: https://api.ldtv.dev",
                "- gRPC: machine-api.ldtv.dev:443",
                "- MQTT: mqtt.ldtv.dev:8883",
                "",
                "## Structural counts",
                f"- REST collection requests: {stats.get('REST_TOTAL', rest_count)}",
                f"- Market release flow stubs: 20 (folder 90)",
                f"- Classifications: {json.dumps(stats)}",
                "",
                "## Checker",
                "```bash",
                "python postman/production-enterprise/check_enterprise_postman_completeness.py",
                "```",
                "",
            ]
        ),
        encoding="utf-8",
    )


def write_readme() -> None:
    text = """# AVF Production Enterprise Postman (market-ready)

## Import

1. Import `AVF_PRODUCTION_ENTERPRISE_REST.postman_collection.json`
2. Import `AVF_PRODUCTION_ENTERPRISE.postman_environment.json`
3. Copy `AVF_PRODUCTION_ENTERPRISE_PRIVATE.template.postman_environment.json` → `*LOCAL*.postman_environment.json` (gitignored)
4. Set `allowGatedWrites=true` and `confirmProductionWrites=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION`

## Folder tree

- **00** README Safety (how-to, write gate, actors, variables)
- **01–19** REST by module (Auth, Category, Brand, Tag, Product, Media, Site, Machine, …)
- **20–21** gRPC/MQTT reference stubs + `.md` catalogs
- **90** Full business flows (20 scenarios)
- **97** Online payment guarded (disabled by default)
- **98** Contract-disabled optional APIs
- **99** Cleanup

## Regenerate

```bash
python postman/production-enterprise/generate_enterprise_postman_project.py
python postman/production-enterprise/check_enterprise_postman_completeness.py
```

## ZIP

`AVF_PRODUCTION_ENTERPRISE_MARKET_READY_POSTMAN_PROJECT.zip` (generated locally; may be gitignored)
"""
    (OUT_DIR / "AVF_PRODUCTION_POSTMAN_ENTERPRISE_README.md").write_text(text, encoding="utf-8")


def write_grpc_mqtt_guide() -> None:
    text = """# gRPC & MQTT manual collection guide

REST is importable as Postman Collection v2.1. **gRPC and MQTT are not faked in Newman JSON** — use:

- Postman Desktop native gRPC/MQTT request types, or
- `grpcurl` / `mosquitto_pub` / `mosquitto_sub` (commands in sibling markdown files)

## gRPC

1. Server: `machine-api.ldtv.dev:443` TLS + SNI
2. Import protos from `proto/avf/machine/v1/`
3. Metadata: `authorization: Bearer {{machineAccessToken}}`
4. Follow flow order in `AVF_PRODUCTION_GRPC_REQUESTS.md` (token refresh → bootstrap → catalog → media → inventory → cash commerce)

## MQTT

1. Broker: `mqtts://mqtt.ldtv.dev:8883`
2. Subscribe command topic before dispatching REST `catalog.refresh`
3. Publish ACK on `.../commands/ack`
4. See `AVF_PRODUCTION_MQTT_REQUESTS.md`
"""
    (OUT_DIR / "AVF_PRODUCTION_GRPC_MQTT_MANUAL_COLLECTION_GUIDE.md").write_text(text, encoding="utf-8")


def create_zip(run_id: str) -> Path:
    zip_path = OUT_DIR / "AVF_PRODUCTION_ENTERPRISE_REORG_POSTMAN_PROJECT.zip"
    legacy = OUT_DIR / "AVF_PRODUCTION_ENTERPRISE_MARKET_READY_POSTMAN_PROJECT.zip"
    with zipfile.ZipFile(zip_path, "w", zipfile.ZIP_DEFLATED) as zf:
        for name in [
            "AVF_PRODUCTION_ENTERPRISE_REST.postman_collection.json",
            "AVF_PRODUCTION_ENTERPRISE.postman_environment.json",
            "AVF_PRODUCTION_ENTERPRISE_PRIVATE.template.postman_environment.json",
            "AVF_PRODUCTION_POSTMAN_ENTERPRISE_README.md",
            "AVF_PRODUCTION_GRPC_MQTT_MANUAL_COLLECTION_GUIDE.md",
            "AVF_PRODUCTION_GRPC_REQUESTS.md",
            "AVF_PRODUCTION_MQTT_REQUESTS.md",
        ]:
            p = OUT_DIR / name
            if p.is_file():
                zf.write(p, arcname=name)
        for doc in DOCS_DIR.glob("POSTMAN_ENTERPRISE_*.md"):
            zf.write(doc, arcname=f"docs/testing/production-e2e/{doc.name}")
        csv = DOCS_DIR / "POSTMAN_ENTERPRISE_COVERAGE_MATRIX.csv"
        if csv.is_file():
            zf.write(csv, arcname=f"docs/testing/production-e2e/{csv.name}")
    if legacy != zip_path and legacy.is_file():
        try:
            legacy.unlink()
        except OSError:
            pass
    return zip_path


def main() -> int:
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    run_id = os.environ.get("PROD_E2E_RUN_ID", "20260525T192300Z-1196-5901")

    coll, rest_flows, stats = build_enterprise_rest_collection()
    env = build_environment()
    env_private = build_environment(private_template=True)

    coll_path = OUT_DIR / "AVF_PRODUCTION_ENTERPRISE_REST.postman_collection.json"
    coll_path.write_text(json.dumps(coll, indent=2) + "\n", encoding="utf-8")

    (OUT_DIR / "AVF_PRODUCTION_ENTERPRISE.postman_environment.json").write_text(
        json.dumps(env, indent=2) + "\n", encoding="utf-8"
    )
    (OUT_DIR / "AVF_PRODUCTION_ENTERPRISE_PRIVATE.template.postman_environment.json").write_text(
        json.dumps(env_private, indent=2) + "\n", encoding="utf-8"
    )

    grpc_data = load_yaml(MANIFEST_GRPC)
    mqtt_data = load_yaml(MANIFEST_MQTT)
    grpc_flows = list(grpc_data.get("flows") or [])
    mqtt_flows = list(mqtt_data.get("flows") or [])

    write_inventory(rest_flows, grpc_flows, mqtt_flows, stats)
    write_actor_flow_matrix(rest_flows, grpc_flows, mqtt_flows)
    write_full_recheck(stats, len(rest_flows))
    (OUT_DIR / "AVF_PRODUCTION_GRPC_REQUESTS.md").write_text(grpc_catalog_md(grpc_flows), encoding="utf-8")
    (OUT_DIR / "AVF_PRODUCTION_MQTT_REQUESTS.md").write_text(mqtt_catalog_md(mqtt_flows), encoding="utf-8")
    write_readme()
    write_grpc_mqtt_guide()

    sys.path.insert(0, str(OUT_DIR))
    import check_enterprise_api_coverage as cov  # noqa: WPS433

    cov.main()

    audit = DOCS_DIR / f"POSTMAN_ENTERPRISE_AUDIT_{run_id}.md"
    audit.write_text(
        f"# Postman enterprise audit ({run_id})\n\n"
        f"- REST requests in collection: {stats.get('REST_TOTAL', len(rest_flows))}\n"
        f"- gRPC flows documented: {len(grpc_flows)}\n"
        f"- MQTT flows documented: {len(mqtt_flows)}\n"
        f"- Collection: `{coll_path.relative_to(REPO_ROOT)}`\n"
        f"- No secrets in tracked JSON.\n",
        encoding="utf-8",
    )

    trace = DOCS_DIR / f"POSTMAN_ENTERPRISE_REQUEST_RESPONSE_TRACE_{run_id}.md"
    trace.write_text(
        f"# Request/response trace ({run_id})\n\n"
        "See canonical evidence:\n"
        f"- `docs/testing/production-e2e/API_TRACE_{run_id}.md`\n"
        f"- `.e2e-runs/production/{run_id}/` (if present locally)\n\n"
        "Regenerate trace after a fresh `all-no-online-payment` run.\n",
        encoding="utf-8",
    )

    parity = DOCS_DIR / f"POSTMAN_ENTERPRISE_IMPORT_PARITY_{run_id}.md"
    parity.write_text(
        f"# Postman enterprise import parity ({run_id})\n\n"
        "Run Newman after filling local private environment:\n\n"
        "```bash\n"
        f"newman run {coll_path.relative_to(REPO_ROOT)} \\\n"
        f"  -e postman/production-enterprise/AVF_PRODUCTION_ENTERPRISE_LOCAL.postman_environment.json \\\n"
        f"  --reporters cli,json\n"
        "```\n\n"
        "Status: **PENDING_OPERATOR_CREDENTIALS** until local env is configured.\n",
        encoding="utf-8",
    )

    zip_path = create_zip(run_id)

    print(f"GENERATED {coll_path.name}: {stats.get('REST_TOTAL', 0)} items")
    print(f"GENERATED environment + private template")
    print(f"gRPC flows: {len(grpc_flows)} MQTT flows: {len(mqtt_flows)}")
    print(f"ZIP: {zip_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

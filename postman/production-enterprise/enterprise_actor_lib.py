#!/usr/bin/env python3
"""Actor, market-flow, and rich metadata for enterprise Postman generation."""
from __future__ import annotations

import json
import re
from dataclasses import dataclass
from pathlib import Path
from typing import Any

REPO_ROOT = Path(__file__).resolve().parents[2]

ACTOR_TAGS = {
    "ADMIN_WEB": "Admin Web",
    "MACHINE_APP": "Vending Machine App",
    "TECHNICIAN": "Technician App / Operator",
    "PUBLIC": "Public / External client",
    "PAYMENT": "Payment Provider / Webhook Provider",
    "MONITORING": "Monitoring / DevOps",
    "BACKEND": "Backend Worker / Scheduler",
    "SUPPORT": "Support / Customer Service",
    "INTERNAL": "Internal System",
    "MQTT_MACHINE": "MQTT — Vending Machine App",
    "MQTT_BACKEND": "MQTT — Backend Command Publisher",
}

MARKET_RELEASE_FLOWS: list[dict[str, str]] = [
    {"id": "90.01", "title": "Admin creates sellable product with image", "actors": "Admin Web", "rest": "REST-AUTH-001,REST-CATALOG-001..005,REST-MEDIA-PIPE,REST-MEDIA-001,REST-MEDIA-002", "grpc": "", "mqtt": ""},
    {"id": "90.02", "title": "Admin creates site and machine", "actors": "Admin Web", "rest": "REST-SITE-001,REST-MACHINE-001..006", "grpc": "", "mqtt": ""},
    {"id": "90.03", "title": "Technician activates and prepares machine", "actors": "Technician App", "rest": "REST-MACHINE-003,REST-OP-001,REST-OP-002", "grpc": "GRPC-TOKEN-001", "mqtt": "MQTT-CONN-001"},
    {"id": "90.04", "title": "Admin assigns product to machine slot", "actors": "Admin Web", "rest": "REST-PLANO-000..006", "grpc": "", "mqtt": ""},
    {"id": "90.05", "title": "Admin fills stock", "actors": "Admin Web,Technician", "rest": "REST-INV-001,REST-OP-001", "grpc": "GRPC-INV-001,GRPC-INV-002", "mqtt": ""},
    {"id": "90.06", "title": "Machine app syncs bootstrap/catalog/media", "actors": "Vending Machine App", "rest": "", "grpc": "GRPC-BOOT-001,GRPC-CAT-001..003,GRPC-MED-001", "mqtt": ""},
    {"id": "90.07", "title": "Machine app caches images for offline display", "actors": "Vending Machine App", "rest": "", "grpc": "GRPC-MED-001", "mqtt": ""},
    {"id": "90.08", "title": "Machine app logs/diagnostics telemetry", "actors": "Vending Machine App", "rest": "(APP_LOG_NOT_IMPLEMENTED — no dedicated REST app-log endpoint)", "grpc": "GRPC-BOOT-002,GRPC-TELEMETRY via CheckIn", "mqtt": "MQTT-TEL-001..004"},
    {"id": "90.09", "title": "Customer buys cash/manual (no online payment)", "actors": "Vending Machine App", "rest": "", "grpc": "GRPC-COMM-CASH-001", "mqtt": ""},
    {"id": "90.10", "title": "Machine confirms vend success", "actors": "Vending Machine App", "rest": "", "grpc": "GRPC-COMM-CASH-001", "mqtt": ""},
    {"id": "90.11", "title": "Inventory decrement and reports update", "actors": "Backend,Admin Web", "rest": "REST-REPORT-001..005", "grpc": "GRPC-INV-002", "mqtt": "MQTT-TEL-004"},
    {"id": "90.12", "title": "Backend sends MQTT command to machine", "actors": "Admin Web,Backend", "rest": "MQTT-CMD dispatch via REST", "grpc": "", "mqtt": "MQTT-CMD-001"},
    {"id": "90.13", "title": "Machine ACKs MQTT command", "actors": "Vending Machine App", "rest": "", "grpc": "", "mqtt": "MQTT-CMD-001,MQTT-NEG-004"},
    {"id": "90.14", "title": "Machine online/offline telemetry", "actors": "Vending Machine App,Monitoring", "rest": "", "grpc": "GRPC-BOOT-002", "mqtt": "MQTT-TEL-001,MQTT-TEL-002"},
    {"id": "90.15", "title": "Technician restocks and cycle-counts", "actors": "Technician App", "rest": "REST-OP-001,REST-OP-002", "grpc": "GRPC-INV-001", "mqtt": ""},
    {"id": "90.16", "title": "Admin reads sales/fleet/inventory reports", "actors": "Admin Web,Operations", "rest": "REST-REPORT-001..005", "grpc": "", "mqtt": "MQTT-READ-001"},
    {"id": "90.17", "title": "Vend failure and reconciliation", "actors": "Vending Machine App,Support", "rest": "", "grpc": "GRPC-COMM-FAIL-001,GRPC-COMM-CANCEL-001", "mqtt": ""},
    {"id": "90.18", "title": "Offline queue replay and idempotency", "actors": "Vending Machine App", "rest": "", "grpc": "GRPC-OFFLINE-001,GRPC-IDEM-001", "mqtt": ""},
    {"id": "90.19", "title": "Cleanup E2E production data", "actors": "E2E automation,Admin Web", "rest": "REST-CLEANUP flows in manifest", "grpc": "", "mqtt": ""},
    {"id": "90.20", "title": "Final market release checklist", "actors": "QA,Release Manager", "rest": "All folders 01-19 + 97/98 documented", "grpc": "12 Vending Machine App gRPC catalog", "mqtt": "13 canonical MQTT topics"},
]


@dataclass
class EndpointMeta:
    actor_tag: str
    used_by: str
    primary_actor: str
    purpose: str
    market_relevance: str
    auth: str
    data_mutation: str
    safe_default: str
    requires_write_confirm: str
    requires_online_payment: str
    test_status: str
    crud: str = ""
    response_shape: str = ""
    dependencies: str = ""
    captured_vars: str = ""
    safety_notes: str = ""


def _auth_label(flow: dict[str, Any]) -> str:
    a = str(flow.get("auth") or "none")
    mapping = {
        "bearer_admin": "admin JWT",
        "bearer_machine": "machine JWT",
        "webhook_hmac": "webhook HMAC",
        "webhook_hmac_invalid": "webhook HMAC (negative)",
        "none": "public / none",
    }
    return mapping.get(a, a)


def _crud_from_method(method: str, path: str) -> str:
    m = method.upper()
    if "/webhooks" in path:
        return "webhook"
    if "/reports/" in path or path.startswith("/v1/reports"):
        return "report"
    if "/commands" in path:
        return "command"
    if m == "GET":
        return "read"
    if m == "POST":
        return "create"
    if m in ("PUT", "PATCH"):
        return "update"
    if m == "DELETE":
        return "delete"
    return "other"


def rest_endpoint_meta(flow: dict[str, Any], classification: str) -> EndpointMeta:
    fid = str(flow.get("id") or "")
    phase = str(flow.get("phase") or "")
    path = str(flow.get("path") or "")
    method = str(flow.get("method") or "GET").upper()
    auth = _auth_label(flow)

    if classification == "ONLINE_PAYMENT_EXCLUDED" or fid.startswith("REST-COMMERCE-00") and fid not in ("REST-COMMERCE-006",):
        if any(x in fid for x in ("003", "004", "005")) or "webhook" in path or "momo" in path.lower():
            return EndpointMeta(
                "PAYMENT", ACTOR_TAGS["PAYMENT"], "payment provider",
                "Online PSP payment session / webhook (phase 2)",
                "payment-phase-2", auth, "provider write", "NO", "YES", "YES",
                "guarded — not run by default", _crud_from_method(method, path),
                safety_notes="Requires onlinePaymentEnabled=true and operator payment confirmation",
            )

    if phase == "preflight" or path in ("/health/live", "/health/ready", "/version"):
        return EndpointMeta(
            "MONITORING", f"{ACTOR_TAGS['MONITORING']}, {ACTOR_TAGS['PUBLIC']}", "monitoring",
            "Liveness/readiness/version probe for release gates", "critical", auth, "none", "YES", "NO", "NO",
            "automated pass", _crud_from_method(method, path),
            response_shape="JSON status + build metadata", safety_notes="Safe read-only",
        )

    if phase == "auth" or path.startswith("/v1/auth"):
        return EndpointMeta(
            "ADMIN_WEB", ACTOR_TAGS["ADMIN_WEB"], "human admin",
            "Admin session and RBAC", "critical", auth,
            "none" if method == "GET" else "E2E-only", "YES" if method == "GET" else "NO",
            "YES" if method not in ("GET",) else "NO", "NO", "automated pass",
            _crud_from_method(method, path), dependencies="adminEmail, adminPassword → accessToken",
            captured_vars=str(flow.get("capture") or ""), safety_notes="Login exempt from write gate",
        )

    if phase == "catalog" or "/admin/categories" in path or "/admin/brands" in path or "/admin/tags" in path or "/admin/products" in path:
        return EndpointMeta(
            "ADMIN_WEB", ACTOR_TAGS["ADMIN_WEB"], "human admin",
            "Catalog foundation for sellable products", "critical", auth, "E2E-only", "NO", "YES", "NO",
            "automated pass", _crud_from_method(method, path),
            captured_vars=str(flow.get("capture") or ""),
            safety_notes="Uses e2ePrefix in names; production write gate required",
        )

    if phase == "media" or "/admin/media" in path or "product-images" in path:
        return EndpointMeta(
            "ADMIN_WEB", ACTOR_TAGS["ADMIN_WEB"], "human admin",
            "Product image upload and attach for machine display", "critical", auth, "E2E-only", "NO", "YES", "NO",
            "automated pass", _crud_from_method(method, path),
            response_shape="mediaId, sha256, url metadata",
            safety_notes="Reject invalid MIME; Cloudinary/direct path per production contract",
        )

    if phase in ("machine", "site", "activation") or "/admin/machines" in path or "/admin/sites" in path:
        return EndpointMeta(
            "ADMIN_WEB", f"{ACTOR_TAGS['ADMIN_WEB']}, {ACTOR_TAGS['TECHNICIAN']}", "human admin",
            "Site and machine provisioning for field deployment", "critical", auth, "E2E-only", "NO", "YES", "NO",
            "automated pass", _crud_from_method(method, path),
            captured_vars=str(flow.get("capture") or ""),
        )

    if phase in ("topology", "planogram") or "planogram" in path or "topology" in path:
        return EndpointMeta(
            "ADMIN_WEB", ACTOR_TAGS["ADMIN_WEB"], "human admin",
            "Slot mapping and published planogram for vending", "critical", auth, "E2E-only", "NO", "YES", "NO",
            "automated pass", _crud_from_method(method, path),
        )

    if phase == "stock" or "/stock" in path or "/inventory" in path:
        return EndpointMeta(
            "ADMIN_WEB", f"{ACTOR_TAGS['ADMIN_WEB']}, {ACTOR_TAGS['TECHNICIAN']}", "technician",
            "Stock refill, cycle count, inventory adjustment", "critical", auth, "E2E-only", "NO", "YES", "NO",
            "automated pass", _crud_from_method(method, path),
        )

    if phase == "operator" or "operator-sessions" in path:
        return EndpointMeta(
            "TECHNICIAN", ACTOR_TAGS["TECHNICIAN"], "technician",
            "Operator/refiller session on machine", "important", auth, "E2E-only", "NO", "YES", "NO",
            "automated pass", _crud_from_method(method, path),
            captured_vars="operatorSessionId",
        )

    if phase == "report" or path.startswith("/v1/reports") or "/admin/reports" in path:
        return EndpointMeta(
            "ADMIN_WEB", f"{ACTOR_TAGS['ADMIN_WEB']}, {ACTOR_TAGS['SUPPORT']}", "operations manager",
            "Operational analytics for market monitoring", "important", auth, "none", "YES", "NO", "NO",
            "automated pass", "report", dependencies="reportFrom, reportTo",
        )

    if phase == "commerce" and classification != "ONLINE_PAYMENT_EXCLUDED":
        return EndpointMeta(
            "ADMIN_WEB", ACTOR_TAGS["ADMIN_WEB"], "human admin",
            "Cash/manual commerce admin readback (no PSP)", "critical", auth, "E2E-only", "NO", "YES", "NO",
            "automated pass", _crud_from_method(method, path),
        )

    if phase == "negative" or fid.startswith("REST-NEG") or fid.startswith("REST-AUTH-00") and "NEG" in fid:
        return EndpointMeta(
            "PUBLIC", "Security QA / attacker simulation", "QA",
            "Negative auth and abuse cases", "important", auth, "none", "YES", "NO", "NO",
            "automated pass", _crud_from_method(method, path),
            safety_notes="Expect 401/403/400 — no production mutation",
        )

    if phase == "cleanup":
        return EndpointMeta(
            "INTERNAL", f"{ACTOR_TAGS['INTERNAL']}, {ACTOR_TAGS['ADMIN_WEB']}", "E2E automation",
            "Remove E2E-PROD prefixed test entities", "critical", auth, "E2E-only", "NO", "YES", "NO",
            "automated pass", "delete", safety_notes="Only E2E-PROD-{{runId}} resources",
        )

    if phase == "rest-coverage":
        rm = (flow.get("route_matrix") or {}).get("coverage") or ""
        if rm == "auth_negative":
            return EndpointMeta(
                "PUBLIC", "Security QA", "QA", "Route matrix auth-negative probe", "important",
                "none", "none", "YES", "NO", "NO", "automated pass", _crud_from_method(method, path),
            )
        if path.startswith("/v1/machines/") or path.startswith("/v1/device/"):
            return EndpointMeta(
                "MACHINE_APP", f"{ACTOR_TAGS['MACHINE_APP']} (legacy HTTP disabled)", "machine app",
                "Legacy machine HTTP — CONTRACT_DISABLED in production", "disabled",
                auth, "none", "NO", "NO", "NO", "documented only", _crud_from_method(method, path),
                safety_notes="Use gRPC/MQTT instead",
            )
        return EndpointMeta(
            "ADMIN_WEB", ACTOR_TAGS["ADMIN_WEB"], "human admin",
            "Readonly/route smoke for release coverage matrix", "important", auth, "none", "YES", "NO", "NO",
            "automated pass", _crud_from_method(method, path),
        )

    if classification in ("OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT", "CONFIG_REQUIRED"):
        return EndpointMeta(
            "INTERNAL", ACTOR_TAGS["INTERNAL"], "release engineer",
            "Contract-disabled or config-required endpoint", "optional", auth, "none", "NO", "NO", "NO",
            "blocked by config", _crud_from_method(method, path),
            safety_notes=classification,
        )

    return EndpointMeta(
        "ADMIN_WEB", ACTOR_TAGS["ADMIN_WEB"], "human admin",
        str(flow.get("label") or fid), "important", auth, "E2E-only", "NO", "YES", "NO",
        "automated pass", _crud_from_method(method, path),
    )


def grpc_method_meta(service: str, method: str, e2e_flow: str = "") -> EndpointMeta:
    machine_services = {
        "MachineActivationService", "MachineTokenService", "MachineAuthService",
        "MachineBootstrapService", "MachineCatalogService", "MachineMediaService",
        "MachineInventoryService", "MachineTelemetryService", "MachineCommerceService",
        "MachineOfflineSyncService", "MachineCommandService",
    }
    tech_services = {"MachineOperatorService"}
    if service in tech_services:
        return EndpointMeta(
            "TECHNICIAN", ACTOR_TAGS["TECHNICIAN"], "technician",
            f"Operator workflow via {service}/{method}", "important", "machine JWT", "E2E-only", "NO", "YES", "NO",
            "grpcurl pass" if e2e_flow else "documented only",
            dependencies="machineAccessToken", safety_notes="Idempotency metadata when required",
        )
    if service in machine_services:
        rel = "critical"
        if service == "MachineCommerceService" and method in ("CreatePaymentSession", "AttachPaymentResult"):
            return EndpointMeta(
                "PAYMENT", ACTOR_TAGS["PAYMENT"], "payment provider",
                "Online payment session (phase 2)", "payment-phase-2", "machine JWT", "provider write", "NO", "YES", "YES",
                "guarded", safety_notes="Excluded from default E2E",
            )
        return EndpointMeta(
            "MACHINE_APP", ACTOR_TAGS["MACHINE_APP"], "machine app",
            f"Machine runtime: {service}/{method}", rel, "machine JWT", "E2E-only", "NO", "YES", "NO",
            "grpcurl pass" if e2e_flow else "documented only",
            response_shape="protobuf JSON", dependencies="machineAccessToken, grpcTarget",
        )
    return EndpointMeta(
        "INTERNAL", ACTOR_TAGS["INTERNAL"], "internal",
        f"gRPC {service}/{method}", "optional", "service", "none", "NO", "NO", "NO", "documented only",
    )


def mqtt_topic_meta(rel_topic: str, direction: str) -> EndpointMeta:
    if direction == "subscribe" or rel_topic == "commands":
        return EndpointMeta(
            "MQTT_BACKEND", ACTOR_TAGS["MQTT_BACKEND"], "backend worker",
            f"Outbound command channel ({rel_topic})", "critical", "MQTT username-password", "production write", "NO", "YES", "NO",
            "mosquitto pass", safety_notes="Subscribe before REST dispatch",
        )
    return EndpointMeta(
        "MQTT_MACHINE", ACTOR_TAGS["MQTT_MACHINE"], "machine app",
        f"Device-originated MQTT telemetry/event ({rel_topic})", "critical",
        "MQTT username-password", "E2E-only", "NO", "YES", "NO", "mosquitto pass",
        response_shape="JSON envelope with schema_version, event_id, dedupe_key",
    )


def market_folder(flow: dict[str, Any], classification: str) -> str:
    from enterprise_folder_lib import market_folder as _folder  # noqa: WPS433

    return _folder(flow, classification)


def build_flow_description(
    flow: dict[str, Any],
    classification: str,
    fid: str,
    manifest_dir: Path,
) -> str:
    meta = rest_endpoint_meta(flow, classification)
    method = str(flow.get("method") or "")
    path = str(flow.get("path") or "")
    tmpl = flow.get("request_template") or {}
    body = json.dumps(tmpl, indent=2) if tmpl else "(empty or from template file)"
    if flow.get("request_template_file"):
        body = f"See `{flow['request_template_file']}` under {manifest_dir.name}/"
    capture = flow.get("capture") or {}
    cap_s = ", ".join(f"{k}→{v}" for k, v in capture.items()) if isinstance(capture, dict) else str(capture)
    headers = ["Content-Type: application/json (when body present)", "Authorization: Bearer {{accessToken}} (admin)"]
    if str(flow.get("auth")) == "none":
        headers = ["No Authorization (negative/public)"]
    elif str(flow.get("auth")) == "bearer_machine":
        headers = ["Authorization: Bearer {{machineAccessToken}}"]
    elif "webhook" in str(flow.get("auth", "")):
        headers = ["X-Webhook-Signature (HMAC) — fill locally"]

    return "\n".join(
        [
            f"## [{meta.actor_tag}] {fid}",
            "",
            f"**Used by:** {meta.used_by}",
            f"**Primary actor:** {meta.primary_actor}",
            f"**Production purpose:** {meta.purpose}",
            f"**Market release relevance:** {meta.market_relevance}",
            f"**Auth:** {meta.auth}",
            f"**CRUD:** {meta.crud or _crud_from_method(method, path)}",
            f"**Data mutation:** {meta.data_mutation}",
            f"**Safe to run by default:** {meta.safe_default}",
            f"**Requires write confirmation:** {meta.requires_write_confirm}",
            f"**Requires online payment confirmation:** {meta.requires_online_payment}",
            f"**Test status:** {meta.test_status}",
            "",
            "### Request",
            f"- **Method:** {method}",
            f"- **Path:** {path}",
            f"- **Headers:** " + "; ".join(headers),
            f"- **Body template:**\n```json\n{body}\n```",
            f"- **Path/query vars:** resolved via Postman environment (e2ePrefix, ids)",
            "",
            "### Response",
            f"- **Expected status:** {flow.get('expected_status', 200)}",
            f"- **Shape:** {meta.response_shape or 'JSON API envelope per OpenAPI'}",
            "",
            "### Dependencies",
            f"- {meta.dependencies or 'Prior flows in same folder / manifest order'}",
            f"- **Captured variables:** {cap_s or meta.captured_vars or 'none'}",
            "",
            "### Safety",
            f"- {meta.safety_notes or 'E2E-PROD-{{runId}} prefix on writes; production write gate on mutations'}",
            f"- **Classification:** {classification}",
            f"- **Source:** tests/e2e/production/e2e-manifest.yaml or rest-coverage",
        ]
    )


def request_display_name(flow: dict[str, Any], classification: str, fid: str) -> str:
    meta = rest_endpoint_meta(flow, classification)
    method = str(flow.get("method") or "")
    path = str(flow.get("path") or "").split("?")[0]
    label = str(flow.get("label") or fid)
    short = label if len(label) < 60 else f"{method} {path}"
    return f"[{meta.actor_tag}] {fid} — {short}"

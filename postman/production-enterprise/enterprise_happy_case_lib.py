#!/usr/bin/env python3
"""Happy-case-only filters and naming for AVF Production Enterprise Postman."""
from __future__ import annotations

import re
from typing import Any

# Manifest / coverage flow IDs excluded from the happy-case collection.
UNHAPPY_FLOW_ID_PREFIXES = ("REST-NEG-", "MQTT-NEG-", "MQTT-CONN-002")
UNHAPPY_FLOW_IDS = frozenset(
    {
        "REST-AUTH-003",
        "REST-AUTH-004",
        "REST-AUTH-005",
        "REST-COMMERCE-003-DUP",
    }
)

UNHAPPY_LABEL_KEYWORDS = (
    "invalid",
    "without token",
    "no token",
    "wrong role",
    "wrong token",
    "wrong machine",
    "wrong tenant",
    "malformed",
    "bad password",
    "unauthorized probe",
    "auth_negative",
    "negative",
    "abuse",
)

UNHAPPY_FOLDER_KEYWORDS = (
    "negative",
    "invalid",
    "wrong token",
    "missing token",
    "abuse",
    "security negative",
)

UNHAPPY_AUTH_MODES = frozenset({"webhook_hmac_invalid", "webhook_hmac_stale"})

NUMERIC_PREFIX_RE = re.compile(r"^\s*\d")
REST_ID_PREFIX_RE = re.compile(r"^REST-[A-Z]+-\d", re.I)
FLOW_ID_PREFIX_RE = re.compile(r"^Flow\s+\d", re.I)


def _method(flow: dict[str, Any]) -> str:
    return str(flow.get("method") or "GET").upper()


def _path(flow: dict[str, Any]) -> str:
    return str(flow.get("path") or "").split("?")[0]


def _fid(flow: dict[str, Any]) -> str:
    return str(flow.get("id") or "")


def _phase(flow: dict[str, Any]) -> str:
    return str(flow.get("phase") or "")


def _label(flow: dict[str, Any]) -> str:
    return str(flow.get("label") or "").lower()


def is_unhappy_flow(flow: dict[str, Any]) -> bool:
    """True when flow is a negative/error/abuse case — exclude from happy collection."""
    fid = _fid(flow)
    if any(fid.startswith(p) for p in UNHAPPY_FLOW_ID_PREFIXES):
        return True
    if fid in UNHAPPY_FLOW_IDS:
        return True
    phase = _phase(flow)
    if phase == "negative":
        return True
    label = _label(flow)
    if any(k in label for k in UNHAPPY_LABEL_KEYWORDS):
        return True
    rm = (flow.get("route_matrix") or {}).get("coverage") or ""
    if rm == "auth_negative":
        return True
    auth = str(flow.get("auth") or "none")
    if auth in UNHAPPY_AUTH_MODES:
        return True
    # Login/register happy paths only — skip invalid-credential probes.
    if "login" in label and "invalid" in label:
        return True
    if path := _path(flow):
        if "/auth/login" in path and flow.get("expected_status") == 401:
            return True
    return False


def is_happy_collection_name(name: str) -> bool:
    n = (name or "").strip()
    if not n:
        return True
    if NUMERIC_PREFIX_RE.match(n):
        return False
    if REST_ID_PREFIX_RE.match(n):
        return False
    if FLOW_ID_PREFIX_RE.match(n):
        return False
    low = n.lower()
    if any(k in low for k in UNHAPPY_FOLDER_KEYWORDS):
        return False
    if n.startswith("[") and any(x in n for x in ("ADMIN_WEB]", "PUBLIC]", "MACHINE_APP]", "RELEASE]")):
        return False
    return True


def module_prefix(flow: dict[str, Any]) -> str:
    path = _path(flow)
    phase = _phase(flow)
    if phase == "preflight" or path in ("/health/live", "/health/ready", "/version"):
        return "Health"
    if phase == "auth" or path.startswith("/v1/auth"):
        return "Auth"
    if "/admin/categories" in path:
        return "Category"
    if "/admin/brands" in path:
        return "Brand"
    if "/admin/tags" in path:
        return "Tag"
    if "/admin/products" in path:
        return "Product"
    if "product-images" in path or "/admin/media" in path:
        return "Media"
    if "/admin/sites" in path:
        return "Site"
    if "/admin/machines" in path or path.startswith("/v1/machines/"):
        return "Machine"
    if "activation" in path or "/setup/" in path:
        return "Activation"
    if "topology" in path:
        return "Topology"
    if "planogram" in path:
        return "Planogram"
    if "stock" in path or "/inventory/" in path or "/slots" in path:
        return "Stock Inventory"
    if "operator" in path:
        return "Operator Technician"
    if phase == "commerce" or path.startswith("/v1/commerce"):
        return "Commerce No Online Payment"
    if phase == "report" or path.startswith("/v1/reports"):
        return "Reports"
    if "/audit" in path or "/commands" in path:
        return "Audit Logs Diagnostics"
    if phase == "cleanup":
        return "Cleanup"
    return "Route Coverage Happy Case"


def crud_action_label(flow: dict[str, Any]) -> str:
    method = _method(flow)
    path = _path(flow)
    label = _label(flow)
    has_id = "{" in path
    mod = module_prefix(flow).split()[-1]

    if "login" in path:
        return "Admin Login"
    if "refresh" in path:
        return "Refresh Token"
    if "logout" in path:
        return "Logout"
    if "me" in path:
        return "Current User"
    if "upload" in label or "product-images" in path:
        return "Upload Product Image"
    if "publish" in path:
        return "Publish Planogram"
    if "draft" in path:
        return "Create Planogram Draft"
    if "topology" in path and method in ("PUT", "POST"):
        return "Create Or Update Topology"
    if "activation-codes/claim" in path:
        return "Claim Activation"
    if "activation-codes" in path and method == "POST":
        return "Create Activation Code"
    if "operator-sessions/start" in path:
        return "Start Operator Session"
    if "stock-adjustment" in path:
        return "Restock Slot"
    if "webhook" in path:
        return "Receive Payment Webhook"
    if method == "POST":
        return f"Create {mod}"
    if method == "GET":
        return f"Get {mod}" if has_id else f"List {mod}s"
    if method in ("PUT", "PATCH"):
        if "activ" in label:
            return f"Update {mod} Status"
        return f"Update {mod}"
    if method == "DELETE":
        return f"Delete {mod}"
    return f"{method} {mod}"


def happy_request_name(flow: dict[str, Any], classification: str) -> str:
    mod = module_prefix(flow)
    action = crud_action_label(flow)
    if classification == "ONLINE_PAYMENT_EXCLUDED":
        return f"Payment Guarded - {action}"
    if mod == "gRPC Reference" or mod.startswith("gRPC"):
        svc = str(flow.get("service") or "")
        rpc = str(flow.get("rpc") or "")
        if svc and rpc:
            return f"gRPC - {svc} {rpc}"
    return f"{mod} - {action}"


def happy_grpc_stub_name(service: str, method: str) -> str:
    return f"gRPC - {service} {method}"


def happy_mqtt_stub_name(rel_topic: str, direction: str) -> str:
    verb = "Subscribe" if direction == "subscribe" else "Publish"
    return f"MQTT - Machine {verb} {rel_topic.replace('/', ' ')}"


def happy_business_flow_name(title: str) -> str:
    return f"Business Flow - {title}"

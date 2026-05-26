#!/usr/bin/env python3
"""Resource-oriented Postman folder mapping (no numeric prefixes)."""
from __future__ import annotations

from typing import Any

from enterprise_happy_case_lib import _method, _path, _phase, _label, _fid


def _crud_subfolder(module: str, method: str, *, has_id: bool) -> str:
    last = module.split()[-1]
    if method == "POST":
        return f"Create {last}"
    if method == "GET":
        return f"Get {last}" if has_id else f"List {last}s"
    if method in ("PUT", "PATCH"):
        return f"Update {last}"
    if method == "DELETE":
        return f"Delete {last}"
    return "Other"


def market_folder(flow: dict[str, Any], classification: str) -> str:
    """Map flow to happy-case folder tree (no leading numbers)."""
    fid = _fid(flow)
    phase = _phase(flow)
    path = _path(flow)
    method = _method(flow)
    label = _label(flow)
    has_id = "{" in path

    if classification == "ONLINE_PAYMENT_EXCLUDED":
        if "webhook" in path or "refund" in path:
            return "Online Payment Happy Case Guarded/Receive Successful Payment Webhook"
        if "momo" in path.lower():
            return "Online Payment Happy Case Guarded/Create MoMo Payment"
        if "zalopay" in path.lower():
            return "Online Payment Happy Case Guarded/Create ZaloPay Payment"
        if "vietqr" in path.lower():
            return "Online Payment Happy Case Guarded/Create VietQR Payment"
        return "Online Payment Happy Case Guarded/Payment Reconciliation"

    if classification in ("OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT", "CONFIG_REQUIRED"):
        if "media/uploads" in path:
            return "Optional Contract Disabled/Presigned Upload"
        if path.startswith("/v1/machines/") or path.startswith("/v1/setup/") or path.startswith("/v1/device/"):
            return "Optional Contract Disabled/Legacy Machine HTTP"
        return "Optional Contract Disabled/Config Required Features"

    if phase == "preflight" or path in ("/health/live", "/health/ready", "/version"):
        if "live" in path:
            return "Health Version/Health Live"
        if "ready" in path:
            return "Health Version/Health Ready"
        return "Health Version/Version"

    if phase == "auth" or path.startswith("/v1/auth"):
        if "refresh" in path:
            return "Auth/Refresh Token"
        if "logout" in path or "change-password" in path:
            return "Auth/Logout"
        if method == "POST" and "login" in path:
            return "Auth/Admin Login"
        if "me" in path:
            return "Auth/Current User"
        return "Auth/Session Revoke"

    if "/admin/categories" in path:
        return f"Category/{_crud_subfolder('Category', method, has_id=has_id)}"

    if "/admin/brands" in path:
        return f"Brand/{_crud_subfolder('Brand', method, has_id=has_id)}"

    if "/admin/tags" in path:
        return f"Tag/{_crud_subfolder('Tag', method, has_id=has_id)}"

    if "/admin/products" in path:
        if "/image" in path or "/media" in path:
            return "Product/Attach Product Image"
        return f"Product/{_crud_subfolder('Product', method, has_id=has_id)}"

    if "product-images" in path or "/admin/media" in path:
        if "uploads/init" in path or "uploads/" in path and "complete" in path:
            return "Media/Presigned Upload"
        if method == "POST":
            return "Media/Upload Product Image"
        return "Media/Get Media"

    if "/admin/sites" in path:
        return f"Site/{_crud_subfolder('Site', method, has_id=has_id)}"

    if "/admin/machines" in path:
        if "topology" in path:
            if method in ("PUT", "POST"):
                return "Topology/Create Or Update Topology"
            return "Topology/Get Topology"
        if "operator-sessions" in path:
            if "logout" in path or "end" in path:
                return "Operator Technician/End Operator Session"
            return "Operator Technician/Start Operator Session"
        if "planogram" in path:
            if "draft" in path:
                return "Planogram/Create Planogram Draft"
            if "publish" in path:
                return "Planogram/Publish Planogram"
            return "Planogram/Assign Product To Slot"
        if "stock-adjustment" in path or "/slots" in path:
            if method == "GET":
                return "Stock Inventory/Get Inventory"
            return "Stock Inventory/Restock Slot"
        if "activation-codes" in path:
            return "Activation/Create Activation Code"
        if "commands" in path:
            return "Audit Logs Diagnostics/Command History"
        if "sync" in path:
            return "Machine/Machine Fleet"
        return f"Machine/{_crud_subfolder('Machine', method, has_id=has_id)}"

    if "/setup/activation" in path or "activation-codes/claim" in path:
        return "Activation/Claim Activation"

    if phase == "commerce" or path.startswith("/v1/commerce"):
        if "cancel" in path:
            return "Commerce No Online Payment/Cancel Order"
        if "webhook" in path:
            return "Online Payment Happy Case Guarded/Receive Successful Payment Webhook"
        return "Commerce No Online Payment/Create Cash Order"

    if phase == "report" or path.startswith("/v1/reports") or "/admin/reports" in path:
        if "sales" in path:
            return "Reports/Sales Report"
        if "inventory" in path:
            return "Reports/Inventory Report"
        if "product" in path:
            return "Reports/Product Report"
        if "fleet" in path:
            return "Reports/Fleet Health Report"
        if "fill" in path or "restock" in path:
            return "Reports/Fill Report"
        return "Reports/Machine Activity Report"

    if "/audit" in path or phase == "audit":
        return "Audit Logs Diagnostics/Admin Audit Logs"

    if phase == "cleanup":
        if "product" in label:
            return "Cleanup/Delete E2E Product"
        if "machine" in label:
            return "Cleanup/Delete E2E Machine"
        if "site" in label:
            return "Cleanup/Delete E2E Site"
        return "Cleanup/Verify Cleanup"

    if phase == "rest-coverage":
        rm = (flow.get("route_matrix") or {}).get("coverage") or ""
        if rm == "documented_skip":
            return "Optional Contract Disabled/Config Required Features"
        if path.startswith("/v1/reports"):
            return "Reports/Sales Report"
        if path.startswith("/v1/machines/") or path.startswith("/v1/device/"):
            return "Optional Contract Disabled/Legacy Machine HTTP"
        return "Route Coverage Happy Case/All Readonly Happy Case Routes"

    return "Route Coverage Happy Case/All Readonly Happy Case Routes"

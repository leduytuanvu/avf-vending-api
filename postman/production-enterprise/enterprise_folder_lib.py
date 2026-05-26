#!/usr/bin/env python3
"""Resource-oriented Postman folder mapping for AVF Production Enterprise API."""
from __future__ import annotations

from typing import Any


def _method(flow: dict[str, Any]) -> str:
    return str(flow.get("method") or "GET").upper()


def _path(flow: dict[str, Any]) -> str:
    return str(flow.get("path") or "").split("?")[0]


def _fid(flow: dict[str, Any]) -> str:
    return str(flow.get("id") or "")


def _phase(flow: dict[str, Any]) -> str:
    return str(flow.get("phase") or "")


def _crud_folder(module: str, method: str, *, has_id: bool) -> str:
    if method == "POST":
        return f"{module}/Create {module.split()[-1]}"
    if method == "GET":
        return f"{module}/Get {module.split()[-1]}" if has_id else f"{module}/List {module.split()[-1]}s"
    if method in ("PUT", "PATCH"):
        return f"{module}/Update {module.split()[-1]}"
    if method == "DELETE":
        return f"{module}/Delete Archive {module.split()[-1]}"
    return f"{module}/Other"


def market_folder(flow: dict[str, Any], classification: str) -> str:
    """Map manifest/coverage flow to Part C enterprise folder tree."""
    fid = _fid(flow)
    phase = _phase(flow)
    path = _path(flow)
    method = _method(flow)
    label = str(flow.get("label") or "").lower()
    has_id = "{" in path

    if classification == "ONLINE_PAYMENT_EXCLUDED":
        if "webhook" in path or "refund" in path:
            return "97 - Online Payment Guarded/Webhook Guarded"
        if "momo" in path.lower():
            return "97 - Online Payment Guarded/MoMo Guarded"
        if "zalopay" in path.lower():
            return "97 - Online Payment Guarded/ZaloPay Guarded"
        if "vietqr" in path.lower():
            return "97 - Online Payment Guarded/VietQR Guarded"
        return "97 - Online Payment Guarded/Payment Reconciliation Guarded"

    if classification in ("OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT", "CONFIG_REQUIRED"):
        if "media/uploads" in path:
            return "98 - Optional Contract Disabled/Presigned Upload Optional"
        if path.startswith("/v1/machines/") or path.startswith("/v1/setup/") or path.startswith("/v1/device/"):
            return "98 - Optional Contract Disabled/Legacy Machine HTTP"
        return "98 - Optional Contract Disabled/Config Required Features"

    if phase == "preflight" or path in ("/health/live", "/health/ready", "/version"):
        if "live" in path:
            return "01 - Health Version/Health Live"
        if "ready" in path:
            return "01 - Health Version/Health Ready"
        return "01 - Health Version/Version"

    if phase == "auth" or path.startswith("/v1/auth"):
        if "refresh" in path:
            return "02 - Auth/03 Refresh Token"
        if "logout" in path or "change-password" in path:
            return "02 - Auth/05 Logout Revoke"
        if method == "POST" and "login" in path:
            if fid == "REST-AUTH-003" or "invalid" in label:
                return "02 - Auth/04 Negative Auth"
            return "02 - Auth/01 Admin Login"
        if "me" in path:
            return "02 - Auth/02 Current User"
        return "02 - Auth/04 Negative Auth"

    if "/admin/categories" in path:
        return "03 - Category/" + _crud_folder("Category", method, has_id=has_id).split("/", 1)[1]

    if "/admin/brands" in path:
        return "04 - Brand/" + _crud_folder("Brand", method, has_id=has_id).split("/", 1)[1]

    if "/admin/tags" in path:
        return "05 - Tag/" + _crud_folder("Tag", method, has_id=has_id).split("/", 1)[1]

    if "/admin/products" in path:
        if "/image" in path or "/media" in path:
            return "06 - Product/Product Image Attach"
        if method == "POST":
            return "06 - Product/Create Product"
        if method == "GET" and has_id:
            return "06 - Product/Get Product"
        if method == "GET":
            return "06 - Product/List Products"
        if method in ("PUT", "PATCH"):
            if "activ" in label or "status" in label:
                return "06 - Product/Activate Product"
            return "06 - Product/Update Product"
        if method == "DELETE":
            return "06 - Product/Delete Archive Product"
        return "06 - Product/Product Search Filter"

    if "product-images" in path or "/admin/media" in path:
        if "uploads/init" in path or "uploads/" in path and "complete" in path:
            return "07 - Media/Presigned Upload Optional"
        if method == "POST":
            return "07 - Media/Cloudinary Direct Upload"
        return "07 - Media/Media Detail"

    if "/admin/sites" in path:
        return "08 - Site/" + _crud_folder("Site", method, has_id=has_id).split("/", 1)[1]

    if "/admin/machines" in path:
        if "topology" in path or phase == "topology":
            if method in ("PUT", "POST"):
                return "11 - Topology/Create Update Topology"
            return "11 - Topology/Get Topology"
        if "planogram" in path or phase == "planogram":
            if "draft" in path:
                return "12 - Planogram/Create Planogram Draft"
            if "publish" in path:
                return "12 - Planogram/Publish Planogram"
            if method == "GET":
                return "12 - Planogram/Get Planogram"
            return "12 - Planogram/Assign Product To Slot"
        if "stock-adjustment" in path or "/slots" in path or phase == "stock":
            if "low-stock" in path:
                return "13 - Stock Inventory/Low Stock Out Of Stock"
            if method == "GET":
                return "13 - Stock Inventory/Inventory Readback"
            if "cycle" in label:
                return "13 - Stock Inventory/Cycle Count"
            return "13 - Stock Inventory/Restock"
        if "operator-sessions" in path or phase == "operator":
            if "logout" in path or "end" in path:
                return "14 - Operator Technician/Operator Session End Logout"
            return "14 - Operator Technician/Operator Session Start"
        if "activation-codes" in path:
            return "10 - Activation/Create Activation Code"
        if "commands" in path:
            return "17 - Audit Logs Diagnostics/Command History"
        if "sync" in path:
            return "09 - Machine/Machine Fleet"
        return "09 - Machine/" + _crud_folder("Machine", method, has_id=has_id).split("/", 1)[1]

    if "/setup/activation" in path or "activation-codes/claim" in path:
        return "10 - Activation/Claim Activation"

    if "topology" in path or phase == "topology":
        if method in ("PUT", "POST"):
            return "11 - Topology/Create Update Topology"
        return "11 - Topology/Get Topology"

    if "planogram" in path or phase == "planogram":
        if "draft" in path:
            return "12 - Planogram/Create Planogram Draft"
        if "publish" in path:
            return "12 - Planogram/Publish Planogram"
        if method == "GET":
            return "12 - Planogram/Get Planogram"
        return "12 - Planogram/Assign Product To Slot"

    if "stock-adjustment" in path or "/inventory/" in path or "low-stock" in path:
        if "low-stock" in path:
            return "13 - Stock Inventory/Low Stock Out Of Stock"
        if method == "GET":
            return "13 - Stock Inventory/Inventory Readback"
        if "cycle" in label:
            return "13 - Stock Inventory/Cycle Count"
        return "13 - Stock Inventory/Restock"

    if phase == "commerce" or path.startswith("/v1/commerce"):
        if "webhook" in path:
            return "97 - Online Payment Guarded/Webhook Guarded"
        if "cancel" in path:
            return "15 - Commerce No Online Payment/Cancel Order"
        if "vend" in path or "payment-session" in path:
            return "15 - Commerce No Online Payment/Vend Start"
        return "15 - Commerce No Online Payment/Cash Manual Order"

    if phase == "report" or path.startswith("/v1/reports") or "/admin/reports" in path:
        if "sales" in path:
            return "16 - Reports/Sales Report"
        if "inventory" in path:
            return "16 - Reports/Inventory Report"
        if "product" in path:
            return "16 - Reports/Product Report"
        if "fleet" in path:
            return "16 - Reports/Fleet Health Report"
        if "fill" in path or "restock" in path:
            return "16 - Reports/Fill Restock Report"
        return "16 - Reports/Machine Activity Report"

    if "/audit" in path or "/events" in path or "/diagnostic" in path:
        return "17 - Audit Logs Diagnostics/Admin Audit Logs"

    if phase == "negative" or fid.startswith("REST-NEG") or fid == "REST-AUTH-005":
        if "role" in label or fid == "REST-AUTH-005":
            return "19 - Security Negative/Wrong Role"
        if "machine" in label:
            return "19 - Security Negative/Wrong Machine"
        if "tenant" in label:
            return "19 - Security Negative/Wrong Tenant"
        if "media" in label:
            return "19 - Security Negative/Invalid Media"
        if "date" in label or "report" in path:
            return "19 - Security Negative/Invalid Date Window"
        if "idempotency" in label or "duplicate" in label:
            return "19 - Security Negative/Duplicate Idempotency"
        if "token" in label or fid.startswith("REST-AUTH-00"):
            return "19 - Security Negative/Missing Token"
        return "19 - Security Negative/Invalid Body"

    if phase == "cleanup":
        if "media" in label:
            return "99 - Cleanup/Delete E2E Media"
        if "machine" in label:
            return "99 - Cleanup/Delete E2E Machine"
        if "site" in label:
            return "99 - Cleanup/Delete E2E Site"
        if "product" in label:
            return "99 - Cleanup/Delete E2E Product"
        return "99 - Cleanup/Verify Cleanup"

    if phase == "rest-coverage":
        rm = (flow.get("route_matrix") or {}).get("coverage") or ""
        if rm == "documented_skip":
            return "18 - Route Coverage Smoke/Documented Skips"
        if path.startswith("/v1/reports"):
            return "18 - Route Coverage Smoke/Reports Smoke"
        if path.startswith("/v1/machines/") or path.startswith("/v1/device/"):
            return "18 - Route Coverage Smoke/Machine Smoke"
        if rm == "auth_negative" or path.startswith("/v1/auth"):
            return "18 - Route Coverage Smoke/Public Smoke"
        return "18 - Route Coverage Smoke/Admin Readonly Smoke"

    if fid.startswith("REST-FLOW-"):
        return "90 - Full Business Flows/Flow 90.01 Admin creates sellable product with image"

    return "18 - Route Coverage Smoke/Admin Readonly Smoke"

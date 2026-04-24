#!/usr/bin/env python3
"""Emit the markdown route matrix for docs/api/api-surface-audit.md (OpenAPI-derived paths).

Run from repo root: python scripts/gen_api_surface_audit_table.py
"""
from __future__ import annotations

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SPEC = ROOT / "docs" / "swagger" / "swagger.json"


def main() -> None:
    data = json.loads(SPEC.read_text(encoding="utf-8"))
    paths: dict = data["paths"]
    lines: list[str] = []
    lines.append(
        "| endpoint | method | intended client | auth type | role/scope | "
        "idempotency required | offline retry safe | status | production risk |"
    )
    lines.append(
        "| --- | --- | --- | --- | --- | --- | --- | --- | --- |"
    )

    for path in sorted(paths):
        for method in sorted(paths[path]):
            if str(method).startswith("parameters") or str(method).startswith("x-"):
                continue
            m = str(method).upper()
            lines.append("| " + " | ".join(classify(path, m)) + " |")

    text = "\n".join(lines) + "\n"
    print(text, end="")


def classify(path: str, method: str) -> list[str]:
    """Return one table row (cells without outer pipes)."""
    idem = "no"
    offline = "yes" if method == "GET" else "caution"
    status = "keep"
    risk = "low"
    client = "admin portal"
    auth = "Bearer JWT"
    scope = "org, platform, or machine (see OpenAPI)"

    if path.startswith("/health/") or path == "/version":
        client = "DevOps/monitoring"
        auth = "none"
        scope = "(n/a)"
        idem = "n/a"
        offline = "yes"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path == "/metrics":
        client = "DevOps/monitoring"
        auth = "none (private bind) or Bearer scrape token when exposed"
        scope = "ops; metrics reader"
        idem = "n/a"
        offline = "yes"
        status = "internal"
        risk = "high if exposed on public listener without ACL"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path.startswith("/swagger/"):
        client = "DevOps/monitoring; integrators (when enabled)"
        auth = "none (public only when intentionally enabled)"
        scope = "(n/a)"
        idem = "n/a"
        offline = "yes"
        risk = "med: disable on edge or protect if secrets could leak via misconfig"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path == "/v1/auth/login" or path == "/v1/auth/refresh":
        client = "technician setup app; admin portal"
        auth = "none (body credentials / refresh token)"
        scope = "(n/a)"
        idem = "n/a"
        offline = "caution"
        risk = "med: credential handling"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path in ("/v1/auth/logout", "/v1/auth/me"):
        client = "technician setup app; admin portal"
        auth = "Bearer JWT"
        scope = "authenticated principal"
        idem = "n/a"
        offline = "yes" if method == "GET" else "caution"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path == "/v1/setup/activation-codes/claim":
        client = "kiosk runtime app (first install)"
        auth = "none (public claim)"
        scope = "activation code + device fingerprint"
        idem = "n/a"
        offline = "no"
        risk = "med: provisioning abuse; rate limits"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path == "/v1/setup/machines/{machineId}/bootstrap":
        client = "kiosk runtime app; technician setup app"
        auth = "Bearer JWT"
        scope = "machine tenant"
        idem = "n/a"
        offline = "yes"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path == "/v1/machines/{machineId}/sale-catalog":
        client = "kiosk runtime app"
        auth = "Bearer JWT"
        scope = "machine tenant"
        idem = "n/a"
        offline = "yes"
        risk = "low; not for admin bulk export"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path.startswith("/v1/reports/"):
        client = "admin portal (not kiosk runtime)"
        auth = "Bearer JWT"
        scope = "org_admin or platform_admin"
        idem = "n/a"
        offline = "yes"
        risk = "low"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path.startswith("/v1/commerce/orders/") and path.endswith("/webhooks"):
        client = "payment provider"
        auth = "HMAC (no Bearer)"
        scope = "PSP callback"
        idem = "n/a"
        offline = "no"
        risk = "high: financial integrity; replay protection"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path.startswith("/v1/commerce/"):
        client = "kiosk runtime app; admin portal (reads/cancel/refund)"
        auth = "Bearer JWT"
        scope = "org and order access"
        if method == "GET":
            idem = "n/a"
            offline = "yes"
        elif method in ("POST", "PUT", "PATCH", "DELETE"):
            idem = "yes"
            offline = "yes w/ key"
        if "vend/" in path or path.endswith("/cancel") or path.endswith("/refunds"):
            risk = "high"
        elif path.endswith("/cash-checkout") or path.endswith("/orders"):
            risk = "high"
        else:
            risk = "med"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path == "/v1/device/machines/{machineId}/commands/poll":
        client = "device HTTP fallback; kiosk integration"
        auth = "Bearer JWT"
        scope = "machine + dispatch roles (see server)"
        idem = "no"
        offline = "caution"
        status = "fallback"
        risk = "med: not primary command path; prefer MQTT"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path == "/v1/device/machines/{machineId}/vend-results":
        client = "device HTTP fallback"
        auth = "Bearer JWT"
        scope = "machine / bridge roles"
        idem = "yes"
        offline = "yes w/ key"
        status = "fallback"
        risk = "med"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if "/events/reconcile" in path:
        client = "kiosk runtime app; device HTTP fallback"
        auth = "Bearer JWT"
        scope = "machine, org, or platform"
        idem = "no (batch keys inside body; follow contract)"
        offline = "caution"
        status = "keep"
        risk = (
            "MQTT-first for volume; HTTP batch for critical reconcile only per contract"
        )
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if "/events/" in path and path.endswith("/status"):
        client = "kiosk runtime app"
        auth = "Bearer JWT"
        scope = "machine, org, or platform"
        idem = "n/a"
        offline = "yes"
        risk = "low"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path.startswith("/v1/machines/") and "/telemetry/" in path:
        client = "kiosk runtime app; admin portal"
        auth = "Bearer JWT"
        scope = "machine tenant"
        idem = "n/a"
        offline = "yes"
        risk = (
            "med: snapshot/rollups are low-rate HTTP; flood telemetry is MQTT-first, not these GETs"
        )
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path.startswith("/v1/machines/") and "/operator-sessions/" in path:
        client = "technician setup app"
        auth = "Bearer JWT"
        scope = "machine URL access + operator session"
        if method == "GET":
            idem = "n/a"
            offline = "yes"
        else:
            idem = "no"
            offline = "caution"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path.startswith("/v1/machines/") and (
        path.endswith("/check-ins") or path.endswith("/config-applies")
    ):
        client = "kiosk runtime app"
        auth = "Bearer JWT"
        scope = "machine tenant"
        idem = "no"
        offline = "caution"
        risk = "med"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path.startswith("/v1/machines/") and "/commands/" in path:
        client = "admin portal; kiosk (dispatch receipts)"
        auth = "Bearer JWT"
        scope = "admin roles / machine tenant (see route)"
        if method == "POST" and path.endswith("/dispatch"):
            idem = "yes"
            offline = "yes w/ key"
            risk = "med"
        else:
            idem = "n/a"
            offline = "yes"
            risk = "low"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path.endswith("/shadow"):
        client = "kiosk runtime app; admin portal"
        auth = "Bearer JWT"
        scope = "machine tenant"
        idem = "n/a"
        offline = "yes"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path.startswith("/v1/admin/machines/") and (
        "/topology" in path or "/planograms/draft" in path
    ):
        client = "technician setup app; admin portal"
        auth = "Bearer JWT"
        scope = "org_admin or platform_admin"
        idem = "no"
        offline = "caution"
        risk = "med"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path.startswith("/v1/admin/machines/") and "/activation-codes" in path:
        client = "admin portal (provisioning; not kiosk runtime)"
        auth = "Bearer JWT"
        scope = "org_admin or platform_admin"
        if method == "GET":
            idem = "n/a"
            offline = "yes"
        else:
            idem = "no"
            offline = "caution"
        risk = "med: code issuance abuse"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path.startswith("/v1/admin/machines/") and "/stock-adjustments" in path:
        client = "technician setup app; admin portal"
        auth = "Bearer JWT"
        scope = "org_admin or platform_admin plus operator session"
        idem = "yes"
        offline = "yes w/ key"
        risk = "med: inventory truth"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path.startswith("/v1/admin/machines/") and "/planograms/publish" in path:
        client = "technician setup app"
        auth = "Bearer JWT"
        scope = "org_admin or platform_admin"
        idem = "yes"
        offline = "yes w/ key"
        risk = "med"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path.startswith("/v1/admin/machines/") and "/sync" in path:
        client = "technician setup app"
        auth = "Bearer JWT"
        scope = "org_admin or platform_admin"
        idem = "yes"
        offline = "yes w/ key"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path.startswith("/v1/admin/machines/") and "/cash-collections" in path:
        client = "technician setup app; admin portal"
        auth = "Bearer JWT"
        scope = "org scope + operator session (writes)"
        if method == "POST":
            idem = "yes"
            offline = "yes w/ key"
        elif "close" in path:
            idem = "yes"
            offline = "yes w/ key"
        else:
            idem = "n/a"
            offline = "yes"
        risk = "high: cash settlement"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path.startswith("/v1/admin/organizations/") and "/artifacts" in path:
        client = "admin portal"
        auth = "Bearer JWT"
        scope = "org or platform (artifact storage route)"
        if method in ("POST", "PUT", "PATCH", "DELETE"):
            idem = "yes"
            offline = "yes w/ key"
            risk = "med"
        else:
            idem = "n/a"
            offline = "yes"
            risk = "low"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path.startswith("/v1/admin/"):
        client = "admin portal; technician setup app (where machine-scoped)"
        auth = "Bearer JWT"
        scope = "org_admin or platform_admin"
        if method in ("POST", "PUT", "PATCH", "DELETE"):
            idem = "yes"
            offline = "yes w/ key"
            risk = "med"
        else:
            idem = "n/a"
            offline = "yes"
            risk = "low"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path.startswith("/v1/operator-insights/"):
        client = "admin portal"
        auth = "Bearer JWT"
        scope = "platform, org_admin, or org_member"
        idem = "n/a"
        offline = "yes"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    if path in ("/v1/orders", "/v1/payments"):
        client = "admin portal (not kiosk runtime)"
        auth = "Bearer JWT"
        scope = "org-scoped lists"
        idem = "n/a"
        offline = "yes"
        return [path, method, client, auth, scope, idem, offline, status, risk]

    # default /v1
    if path.startswith("/v1/"):
        client = "admin portal"
        auth = "Bearer JWT"
        scope = "see OpenAPI"
        if method in ("POST", "PUT", "PATCH", "DELETE"):
            idem = "yes"
            offline = "yes w/ key"
        else:
            idem = "n/a"
            offline = "yes"

    return [path, method, client, auth, scope, idem, offline, status, risk]


if __name__ == "__main__":
    main()

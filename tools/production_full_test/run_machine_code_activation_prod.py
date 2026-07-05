#!/usr/bin/env python3
"""Production smoke tests for machine-code activation admin workflow."""

from __future__ import annotations

import json
import os
import re
import sys
import uuid
from datetime import datetime, timezone
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from _common import http_request, new_request_id, redact, report_dir, write_json

REPO = Path(__file__).resolve().parents[2]
ACTIVATION_CODE_RE = re.compile(r"^AVF[0-9]{6}$")


def machine_code_from_registry(reg: dict, machine_id: str) -> str:
    for row in reversed(reg.get("writes") or []):
        if row.get("action") != "POST /v1/admin/machines":
            continue
        if row.get("entity_id") != machine_id:
            continue
        try:
            req = json.loads(str(row.get("request_body") or "{}"))
        except json.JSONDecodeError:
            continue
        code = str(req.get("code") or "").strip().upper()
        if ACTIVATION_CODE_RE.match(code):
            return code
    return ""


def ensure_activation_test_machine(base_url: str, token: str, run_prefix: str) -> tuple[str, str]:
    """Return machine_id + 6-digit AVF machine code, creating an isolated machine if needed."""
    site_payload = {
        "name": f"{run_prefix} Activation Site",
        "code": f"ACT-{uuid.uuid4().hex[:8]}"[:20],
        "timezone": "UTC",
        "address": {"line1": f"{run_prefix} activation test"},
    }
    st, site = admin_json(base_url, token, "POST", "/v1/admin/sites", site_payload)
    if st not in (200, 201) or not site.get("id"):
        raise RuntimeError(f"activation site create failed HTTP {st}")
    machine_code = f"AVF{uuid.uuid4().int % 1_000_000:06d}"
    machine_payload = {
        "name": f"{run_prefix} Activation Machine",
        "code": machine_code,
        "siteId": site["id"],
        "serialNumber": f"{run_prefix}-ACT-{uuid.uuid4().hex[:6]}",
        "model": "AVF-PROD-TEST",
        "status": "draft",
        "timezone": "UTC",
        "cabinetType": "ambient",
    }
    st, machine = admin_json(base_url, token, "POST", "/v1/admin/machines", machine_payload)
    if st not in (200, 201) or not machine.get("id"):
        raise RuntimeError(f"activation machine create failed HTTP {st}")
    admin_json(base_url, token, "PATCH", f"/v1/admin/machines/{machine['id']}", {"status": "active"})
    return str(machine["id"]), machine_code


def login(base_url: str, email: str, password: str) -> str:
    body = json.dumps({"email": email, "password": password}).encode()
    status, raw, _ = http_request(
        "POST",
        f"{base_url.rstrip('/')}/v1/auth/login",
        headers={"Content-Type": "application/json", "X-Correlation-ID": new_request_id()},
        body=body,
    )
    if status not in (200, 201):
        raise RuntimeError(f"login failed HTTP {status}: {redact(raw[:500])}")
    data = json.loads(raw)
    tokens = data.get("tokens") or {}
    token = (
        data.get("accessToken")
        or data.get("access_token")
        or data.get("token")
        or tokens.get("accessToken")
        or tokens.get("access_token")
    )
    if not token:
        raise RuntimeError("login missing access token")
    return str(token)


def admin_json(base_url: str, token: str, method: str, path: str, payload: dict | None = None) -> tuple[int, dict]:
    headers = {
        "Authorization": f"Bearer {token}",
        "X-Request-ID": new_request_id(),
        "X-Correlation-ID": new_request_id(),
    }
    body = None
    if payload is not None:
        headers["Content-Type"] = "application/json"
        body = json.dumps(payload).encode()
    status, raw, _ = http_request("POST" if method == "POST" else method, f"{base_url.rstrip('/')}{path}", headers=headers, body=body)
    try:
        data = json.loads(raw) if raw.strip() else {}
    except json.JSONDecodeError:
        data = {"raw": raw[:2000]}
    return status, data


def main() -> int:
    base_url = os.environ.get("BASE_URL", "https://api.ldtv.dev").rstrip("/")
    email = os.environ.get("ADMIN_EMAIL", "")
    password = os.environ.get("ADMIN_PASSWORD", "")
    run_prefix = os.environ.get("RUN_PREFIX", datetime.now(timezone.utc).strftime("MCODE-ACT-PROD-%Y%m%d-%H%M%S"))
    if not email or not password:
        print("ADMIN_EMAIL and ADMIN_PASSWORD required", file=sys.stderr)
        return 2

    results: list[dict] = []
    fail = 0

    def record(name: str, ok: bool, **extra: object) -> None:
        nonlocal fail
        row = {"name": name, "status": "pass" if ok else "fail", **extra}
        results.append(row)
        if not ok:
            fail += 1
        print(f"{'PASS' if ok else 'FAIL'} {name}")

    # Preflight
    for path in ("/health/live", "/health/ready", "/version"):
        st, raw, _ = http_request("GET", f"{base_url}{path}")
        record(f"GET {path}", st == 200, http_status=st)

    token = login(base_url, email, password)
    st, me, _ = http_request("GET", f"{base_url}/v1/auth/me", headers={"Authorization": f"Bearer {token}"})
    record("GET /v1/auth/me", st == 200, http_status=st)

    # Resolve test machine from bootstrap entity registry if present
    reg_path = report_dir() / "PRODUCTION_TEST_ENTITY_REGISTRY.json"
    machine_id = os.environ.get("TEST_MACHINE_ID", "")
    machine_code = os.environ.get("TEST_MACHINE_CODE", "")
    if reg_path.is_file():
        reg = json.loads(reg_path.read_text(encoding="utf-8"))
        machine_id = machine_id or str((reg.get("entities") or {}).get("machineId", {}).get("id", ""))
        if machine_id and not machine_code:
            machine_code = machine_code_from_registry(reg, machine_id)

    if not machine_code or not ACTIVATION_CODE_RE.match(machine_code.upper()):
        try:
            machine_id, machine_code = ensure_activation_test_machine(base_url, token, run_prefix)
            record("ensure_activation_test_machine", True, machine_id=machine_id, machine_code=machine_code)
        except RuntimeError as exc:
            record("ensure_activation_test_machine", False, error=str(exc))
            evidence = REPO / "docs" / "reports" / "machine-code-activation-production" / "evidence"
            evidence.mkdir(parents=True, exist_ok=True)
            write_json(evidence / "activation_smoke_results.json", {"results": results, "blocked": "machine_setup_failed"})
            return 2

    if not machine_code:
        print("No TEST_MACHINE_CODE or bootstrap registry — run bootstrap first", file=sys.stderr)
        out = Path("docs/reports/machine-code-activation-production/evidence")
        out.mkdir(parents=True, exist_ok=True)
        write_json(out / "activation_smoke_results.json", {"results": results, "blocked": "missing_test_machine"})
        return 2

    # Create by machineCode path
    st, created = admin_json(
        base_url,
        token,
        "POST",
        f"/v1/admin/machine-codes/{machine_code}/activation-codes",
        {"expiresInMinutes": 60, "maxUses": 1, "notes": run_prefix},
    )
    act_id = str(created.get("activationCodeId", ""))
    act_plain = str(created.get("activationCode", ""))
    record(
        "POST /machine-codes/{code}/activation-codes",
        st in (200, 201) and created.get("machineCode") == machine_code and bool(act_id),
        http_status=st,
        machine_id=created.get("machineId"),
        machine_code=created.get("machineCode"),
    )
    if machine_id and created.get("machineId"):
        machine_id = str(created.get("machineId"))

    # UUID backward compat
    if machine_id:
        st2, created2 = admin_json(
            base_url,
            token,
            "POST",
            f"/v1/admin/machines/{machine_id}/activation-codes",
            {"expiresInMinutes": 60, "maxUses": 1},
        )
        record(
            "POST /machines/{uuid}/activation-codes",
            st2 in (200, 201) and created2.get("machineCode") == machine_code,
            http_status=st2,
        )

        st3, created3 = admin_json(
            base_url,
            token,
            "POST",
            f"/v1/admin/machines/{machine_code}/activation-codes",
            {"expiresInMinutes": 60, "maxUses": 1},
        )
        record(
            "POST /machines/{code}/activation-codes",
            st3 in (200, 201),
            http_status=st3,
        )

    # Collection body variants
    for field, val in (("machineCode", machine_code), ("machine_code", machine_code)):
        st4, c4 = admin_json(base_url, token, "POST", "/v1/admin/activation-codes", {field: val, "expiresInMinutes": 60, "maxUses": 1})
        record(f"POST /activation-codes body {field}", st4 in (200, 201), http_status=st4)

    # List
    st5, list_raw, _ = http_request(
        "GET",
        f"{base_url}/v1/admin/machine-codes/{machine_code}/activation-codes",
        headers={"Authorization": f"Bearer {token}"},
    )
    list_body = list_raw.lower()
    record(
        "GET /machine-codes/{code}/activation-codes",
        st5 == 200 and "codehash" not in list_body and act_plain not in list_raw,
        http_status=st5,
    )

    # Revoke first created code
    if act_id:
        st6, _, _ = http_request(
            "DELETE",
            f"{base_url}/v1/admin/machine-codes/{machine_code}/activation-codes/{act_id}",
            headers={"Authorization": f"Bearer {token}"},
        )
        record("DELETE /machine-codes/{code}/activation-codes/{id}", st6 in (200, 204), http_status=st6)

    # Claim smoke with isolated fingerprint (use fresh code if revoked — create another)
    st7, claim_code_resp = admin_json(
        base_url,
        token,
        "POST",
        f"/v1/admin/machine-codes/{machine_code}/activation-codes",
        {"expiresInMinutes": 30, "maxUses": 1, "notes": f"{run_prefix}-claim"},
    )
    claim_code = str(claim_code_resp.get("activationCode", ""))
    if st7 in (200, 201) and claim_code:
        fp = {
            "androidId": f"{run_prefix}-aid",
            "boardSerial": f"{run_prefix}-board",
            "androidSerial": f"{run_prefix}-aserial",
            "deviceSerial": f"{run_prefix}-dserial",
            "manufacturer": "PROD_TEST",
            "model": "PROD_TEST_MODEL",
        }
        body = json.dumps({"activationCode": claim_code, "deviceFingerprint": fp}).encode()
        st8, claim_raw, _ = http_request(
            "POST",
            f"{base_url}/v1/setup/activation-codes/claim",
            headers={"Content-Type": "application/json", "X-Request-ID": new_request_id()},
            body=body,
        )
        claim_data = json.loads(claim_raw) if claim_raw.strip() else {}
        record(
            "POST /setup/activation-codes/claim",
            st8 in (200, 201) and str(claim_data.get("machineId", "")) == machine_id,
            http_status=st8,
            has_device_attachment=bool(claim_data.get("deviceAttachmentId")),
        )

    evidence = REPO / "docs" / "reports" / "machine-code-activation-production" / "evidence"
    evidence.mkdir(parents=True, exist_ok=True)
    safe_results = []
    for r in results:
        safe = dict(r)
        safe_results.append(safe)
    write_json(
        evidence / "activation_smoke_results.json",
        {"run_prefix": run_prefix, "machine_code": machine_code, "machine_id": machine_id, "results": safe_results, "fail_count": fail},
    )
    return 1 if fail else 0


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""Production smoke: gRPC machine_code on ClaimActivation, RefreshMachineToken, GetBootstrap."""

from __future__ import annotations

import json
import os
import re
import sys
import uuid
from datetime import datetime, timezone
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from _common import http_request, new_request_id, report_dir, write_json
from run_grpc_full_production import grpc_call
from run_machine_code_activation_prod import (
    ACTIVATION_CODE_RE,
    admin_json,
    ensure_activation_test_machine,
    login,
)

REPO = Path(__file__).resolve().parents[2]
UUID_RE = re.compile(
    r"^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$",
    re.I,
)


def parse_grpc_json(raw: str) -> dict:
    text = (raw or "").strip()
    if not text:
        return {}
    start = text.find("{")
    if start < 0:
        return {}
    try:
        return json.loads(text[start:])
    except json.JSONDecodeError:
        return {}


def device_fingerprint(serial: str) -> dict:
    return {
        "serialNumber": serial,
        "androidId": f"{serial}-aid",
        "packageName": "dev.avf.vending.prodtest",
        "versionName": "1.0.0",
        "versionCode": 1,
    }


def main() -> int:
    base_url = os.environ.get("BASE_URL", "https://api.ldtv.dev").rstrip("/")
    grpc_host = os.environ.get("GRPC_HOST", "machine-api.ldtv.dev:443")
    email = os.environ.get("ADMIN_EMAIL", "")
    password = os.environ.get("ADMIN_PASSWORD", "")
    run_prefix = os.environ.get("RUN_PREFIX", datetime.now(timezone.utc).strftime("GRPC-MCODE-PROD-%Y%m%d-%H%M%S"))
    if not email or not password:
        print("ADMIN_EMAIL and ADMIN_PASSWORD required", file=sys.stderr)
        return 2

    evidence = REPO / "docs" / "reports" / "grpc-machine-code-response" / "evidence"
    evidence.mkdir(parents=True, exist_ok=True)
    proto_root = REPO / "proto"

    results: list[dict] = []
    fail = 0

    def record(name: str, ok: bool, **extra: object) -> None:
        nonlocal fail
        row = {"name": name, "status": "pass" if ok else "fail", **extra}
        results.append(row)
        if not ok:
            fail += 1
        print(f"{'PASS' if ok else 'FAIL'} {name}")

    token = login(base_url, email, password)
    machine_id, machine_code = ensure_activation_test_machine(base_url, token, run_prefix)
    record("ensure_activation_test_machine", True, machine_id=machine_id, machine_code=machine_code)

    st, created = admin_json(
        base_url,
        token,
        "POST",
        f"/v1/admin/machine-codes/{machine_code}/activation-codes",
        {"expiresInMinutes": 60, "maxUses": 1, "notes": run_prefix},
    )
    act_plain = str(created.get("activationCode", ""))
    record(
        "POST activation code",
        st in (200, 201) and bool(act_plain),
        http_status=st,
    )
    if not act_plain:
        write_json(evidence / "grpc_machine_code_smoke.json", {"results": results, "fail_count": fail + 1})
        return 2

    claim_body = json.dumps(
        {
            "activationCode": act_plain,
            "deviceFingerprint": device_fingerprint(f"{run_prefix}-SN"),
        }
    )
    ok, claim_raw = grpc_call(
        grpc_host,
        "avf.machine.v1.MachineActivationService/ClaimActivation",
        "",
        claim_body,
        proto_root,
    )
    claim = parse_grpc_json(claim_raw)
    resp_machine_id = str(claim.get("machineId") or claim.get("machine_id") or "")
    resp_machine_code = str(claim.get("machineCode") or claim.get("machine_code") or "").upper()
    access_token = str(claim.get("accessToken") or claim.get("access_token") or "")
    refresh_token = str(claim.get("refreshToken") or claim.get("refresh_token") or "")
    mqtt_username = str(claim.get("mqttUsername") or claim.get("mqtt_username") or "")
    claim_ok = (
        ok
        and UUID_RE.match(resp_machine_id)
        and resp_machine_id == machine_id
        and resp_machine_code == machine_code.upper()
        and mqtt_username == machine_id
        and bool(access_token)
        and bool(refresh_token)
    )
    record(
        "gRPC ClaimActivation machine_code",
        claim_ok,
        machine_id=resp_machine_id,
        machine_code=resp_machine_code,
        mqtt_username=mqtt_username,
    )

    refresh_body = json.dumps({"refreshToken": refresh_token})
    ok2, refresh_raw = grpc_call(
        grpc_host,
        "avf.machine.v1.MachineTokenService/RefreshMachineToken",
        "",
        refresh_body,
        proto_root,
    )
    refresh = parse_grpc_json(refresh_raw)
    refresh_id = str(refresh.get("machineId") or refresh.get("machine_id") or "")
    refresh_code = str(refresh.get("machineCode") or refresh.get("machine_code") or "").upper()
    refresh_ok = (
        ok2
        and UUID_RE.match(refresh_id)
        and refresh_id == machine_id
        and refresh_code == machine_code.upper()
    )
    record(
        "gRPC RefreshMachineToken machine_code",
        refresh_ok,
        machine_id=refresh_id,
        machine_code=refresh_code,
    )

    bootstrap_body = json.dumps({})
    ok3, bootstrap_raw = grpc_call(
        grpc_host,
        "avf.machine.v1.MachineBootstrapService/GetBootstrap",
        access_token,
        bootstrap_body,
        proto_root,
    )
    bootstrap = parse_grpc_json(bootstrap_raw)
    machine_obj = bootstrap.get("machine") or {}
    boot_id = str(machine_obj.get("machineId") or machine_obj.get("machine_id") or "")
    boot_code = str(machine_obj.get("machineCode") or machine_obj.get("machine_code") or "").upper()
    mqtt_meta = bootstrap.get("mqtt") or bootstrap.get("mqttConfig") or {}
    boot_mqtt_user = str(mqtt_meta.get("username") or mqtt_meta.get("mqttUsername") or "")
    bootstrap_ok = (
        ok3
        and UUID_RE.match(boot_id)
        and boot_id == machine_id
        and boot_code == machine_code.upper()
        and (not boot_mqtt_user or boot_mqtt_user == machine_id)
    )
    record(
        "gRPC GetBootstrap machine.machine_code",
        bootstrap_ok,
        machine_id=boot_id,
        machine_code=boot_code,
        mqtt_username=boot_mqtt_user or mqtt_username,
    )

    summary = {
        "run_prefix": run_prefix,
        "base_url": base_url,
        "grpc_host": grpc_host,
        "machine_id": machine_id,
        "machine_code": machine_code,
        "results": results,
        "pass_count": sum(1 for r in results if r["status"] == "pass"),
        "fail_count": fail,
        "timestamp": datetime.now(timezone.utc).isoformat(),
    }
    write_json(evidence / "grpc_machine_code_smoke.json", summary)
    write_json(report_dir() / "GRPC_MACHINE_CODE_SMOKE.json", summary)
    return 1 if fail else 0


if __name__ == "__main__":
    raise SystemExit(main())

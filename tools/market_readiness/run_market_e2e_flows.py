#!/usr/bin/env python3
"""Market readiness E2E flows 1-9 (superset of runtime-fleet A-I)."""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import uuid
from datetime import datetime, timezone
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "production_full_test"))
sys.path.insert(0, str(Path(__file__).resolve().parent))

from _common import REPO, admin_headers, build_full_fingerprint, bundle_dir, http_request, new_request_id, setup_market_env, write_json  # noqa: E402
from bootstrap_market_rbac import bootstrap_market_rbac  # noqa: E402
from bootstrap_test_data import claim_activation  # noqa: E402
from run_grpc_full_production import grpc_call  # noqa: E402


def flow(flow_id: str, name: str, ok: bool, detail: str) -> dict:
    return {
        "flow": flow_id,
        "name": name,
        "ok": ok,
        "detail": detail,
        "at_utc": datetime.now(timezone.utc).isoformat(),
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default=os.environ.get("BASE_URL", "https://api.ldtv.dev"))
    parser.add_argument("--pass", dest="pass_num", type=int, default=1)
    args = parser.parse_args()

    os.environ["PROD_TEST_SUFFIX"] = f"e2e-p{args.pass_num}-{uuid.uuid4().hex[:6]}"
    setup_market_env()
    out = bundle_dir()
    flows: list[dict] = []
    base = args.base_url

    try:
        reg = bootstrap_market_rbac(base)
        subst = reg.as_substitution_map()
        flows.append(flow("1", "Admin + technician + multi-machine bootstrap", True, reg.data.get("prefix", "")))
    except Exception as exc:
        flows.append(flow("1", "Admin + technician + multi-machine bootstrap", False, str(exc)))
        write_json(out / f"E2E_FLOW_PASS_{args.pass_num}.json", {"flows": flows})
        return 1

    machine_id = subst["machineId"]
    admin_token = subst["adminAccessToken"]
    machine_token = subst.get("machineToken") or ""
    mqtt_pass = subst.get("mqttPassword") or ""

    flows.append(flow("2", "Claim + MQTT credentials", bool(mqtt_pass), "mqttPassword present" if mqtt_pass else "missing"))

    grpc_host = os.environ.get("GRPC_HOST", "machine-api.ldtv.dev:443")
    ok_grpc, grpc_out = grpc_call(
        grpc_host,
        "avf.machine.v1.MachineBootstrapService/GetBootstrap",
        machine_token,
        json.dumps({"meta": {"machineId": machine_id, "requestId": new_request_id()}}),
        REPO / "proto",
    )
    leaked = "mqttpassword" in grpc_out.lower()
    flows.append(flow("3", "gRPC GetBootstrap metadata", ok_grpc and not leaked, grpc_out[:200]))

    mqtt_rc = subprocess.call(
        [sys.executable, str(REPO / "tools" / "production_full_test" / "run_mqtt_full_production.py")],
        cwd=REPO,
    )
    flows.append(flow("4", "MQTT matrix", mqtt_rc == 0, f"rc={mqtt_rc}"))

    st, raw, _ = http_request("GET", f"{base.rstrip('/')}/health/live")
    flows.append(flow("5", "Health live", st == 200, f"HTTP {st}"))

    st, _, _ = http_request(
        "PATCH",
        f"{base.rstrip('/')}/v1/admin/machines/{machine_id}",
        headers=admin_headers(admin_token),
        body=json.dumps({"status": "suspended"}).encode(),
    )
    st2, _, _ = http_request(
        "PATCH",
        f"{base.rstrip('/')}/v1/admin/machines/{machine_id}",
        headers=admin_headers(admin_token),
        body=json.dumps({"status": "active"}).encode(),
    )
    flows.append(flow("6", "Lifecycle suspend/resume", st in (200, 204) and st2 in (200, 204), f"{st}/{st2}"))

    fp = build_full_fingerprint(reg.data.get("prefix", "e2e"), "flow7")
    st, raw, _ = http_request(
        "POST",
        f"{base.rstrip('/')}/v1/admin/machines/{machine_id}/reattach-device",
        headers=admin_headers(admin_token),
        body=json.dumps({"deviceFingerprint": fp, "reason": "market_e2e_flow7"}).encode(),
    )
    flows.append(flow("7", "Reattach full fingerprint", st in (200, 201), f"HTTP {st}"))

    st, _, _ = http_request(
        "POST",
        f"{base.rstrip('/')}/v1/admin/machines/{machine_id}/compromised",
        headers=admin_headers(admin_token),
        body=json.dumps({"reason": "market_e2e_flow8"}).encode(),
    )
    flows.append(flow("8", "Compromised lifecycle", st in (200, 204, 404), f"HTTP {st}"))

    activation_code = subst.get("activationCode") or ""
    serial = subst.get("machineSerialNumber") or ""
    i_ok = False
    if activation_code and serial:
        try:
            claim2 = claim_activation(base, activation_code, serial)
            i_ok = bool(claim2.get("machineToken") or claim2.get("accessToken"))
        except Exception as exc:
            i_ok = False
            detail = str(exc)
        else:
            detail = "idempotent replay"
    else:
        detail = "no activation code"
    flows.append(flow("9", "Offline replay idempotency", i_ok, detail))

    all_ok = all(f["ok"] for f in flows)
    write_json(out / f"E2E_FLOW_PASS_{args.pass_num}.json", {"pass": args.pass_num, "flows": flows, "all_ok": all_ok})
    return 0 if all_ok else 1


if __name__ == "__main__":
    raise SystemExit(main())

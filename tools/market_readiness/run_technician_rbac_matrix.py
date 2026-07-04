#!/usr/bin/env python3
"""Technician multi-machine RBAC negative matrix (3 passes)."""

from __future__ import annotations

import argparse
import json
import os
import sys
import uuid
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "production_full_test"))
sys.path.insert(0, str(Path(__file__).resolve().parent))

from _common import admin_headers, build_full_fingerprint, bundle_dir, http_request, setup_market_env, write_matrix_result  # noqa: E402
from bootstrap_market_rbac import bootstrap_market_rbac  # noqa: E402


NEGATIVE_CASES = [
    ("unassigned_reattach_machine_c", "POST", "reattach_c"),
    ("machine_jwt_on_admin_list", "GET", "admin_machines_machine_token"),
    ("technician_activate_denied", "PATCH", "activate_c"),
    ("missing_operator_session_reattach", "POST", "reattach_no_session"),
]


def run_pass(base: str, pass_num: int) -> tuple[list[dict], int, int]:
    os.environ["PROD_TEST_SUFFIX"] = f"rbac-p{pass_num}-{uuid.uuid4().hex[:6]}"
    setup_market_env()
    reg = bootstrap_market_rbac(base)
    subst = reg.as_substitution_map()
    admin_token = subst["adminAccessToken"]
    tech_token = subst.get("token_technician") or ""
    machine_a = subst["machineId"]
    machine_b = subst.get("machineIdB") or ""
    machine_c = subst.get("machineIdC") or ""
    machine_token = subst.get("machineToken") or ""

    rows: list[dict] = []

    # Positive: technician can read fleet
    st, _, _ = http_request(
        "GET",
        f"{base.rstrip('/')}/v1/admin/machines",
        headers=admin_headers(tech_token),
    )
    rows.append({"case": "technician_read_fleet", "pass": st == 200, "status": st})

    # Negative: reattach on unassigned machine C with technician token should fail
    fp = build_full_fingerprint(reg.data.get("prefix", "m"), "rbac")
    st, raw, _ = http_request(
        "POST",
        f"{base.rstrip('/')}/v1/admin/machines/{machine_c}/reattach-device",
        headers=admin_headers(tech_token),
        body=json.dumps({"deviceFingerprint": fp, "reason": "rbac_negative"}).encode(),
    )
    rows.append({"case": "unassigned_reattach_denied", "pass": st in (401, 403, 404), "status": st})

    # Negative: machine JWT on admin route
    st, _, _ = http_request(
        "GET",
        f"{base.rstrip('/')}/v1/admin/machines",
        headers=admin_headers(machine_token),
    )
    rows.append({"case": "machine_jwt_admin_denied", "pass": st in (401, 403), "status": st})

    # Negative: technician cannot activate/deactivate
    st, _, _ = http_request(
        "PATCH",
        f"{base.rstrip('/')}/v1/admin/machines/{machine_c}",
        headers=admin_headers(tech_token),
        body=json.dumps({"status": "active", "saleEnabled": True}).encode(),
    )
    rows.append({"case": "technician_activate_denied", "pass": st in (401, 403), "status": st})

    # Operator sessions on A and B
    for label, mid in (("A", machine_a), ("B", machine_b)):
        st, raw, _ = http_request(
            "POST",
            f"{base.rstrip('/')}/v1/admin/machines/{mid}/operator-sessions/start",
            headers=admin_headers(admin_token),
            body=json.dumps({"technicianId": subst.get("technicianId")}).encode(),
        )
        rows.append({"case": f"operator_session_{label}", "pass": st in (200, 201, 409), "status": st})

    # Cross-machine: use machine B token against machine A gRPC path via REST proxy check
    st, _, _ = http_request(
        "GET",
        f"{base.rstrip('/')}/v1/admin/machines/{machine_a}/runtime-sessions/current",
        headers=admin_headers(subst.get("machineTokenB") or machine_token),
    )
    rows.append({"case": "cross_machine_token_denied", "pass": st in (401, 403), "status": st})

    fail = sum(1 for r in rows if not r.get("pass"))
    return rows, len(rows) - fail, fail


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default=os.environ.get("BASE_URL", "https://api.ldtv.dev"))
    parser.add_argument("--pass", dest="pass_num", type=int, default=0)
    args = parser.parse_args()

    out = bundle_dir()
    rc = 0
    for p in ([args.pass_num] if args.pass_num else [1, 2, 3]):
        rows, pc, fc = run_pass(args.base_url, p)
        write_matrix_result(out, f"TECHNICIAN_RBAC_PASS_{p}", rows, pass_count=pc, fail_count=fc)
        if fc:
            rc = 1
    return rc


if __name__ == "__main__":
    raise SystemExit(main())

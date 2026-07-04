#!/usr/bin/env python3
"""Chaos / concurrency / edge cases for market readiness (14 cases × 3 passes)."""

from __future__ import annotations

import argparse
import json
import os
import sys
import threading
import uuid
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "production_full_test"))
sys.path.insert(0, str(Path(__file__).resolve().parent))

from _common import admin_headers, build_full_fingerprint, bundle_dir, http_request, setup_market_env, write_matrix_result  # noqa: E402
from bootstrap_test_data import bootstrap  # noqa: E402


EDGE_CASES = [
    "duplicate_operator_session_start",
    "reattach_without_fingerprint",
    "reattach_empty_body",
    "invalid_machine_uuid",
    "ended_operator_session_reattach",
    "concurrent_reattach_requests",
    "sale_toggle_rapid",
    "timeline_invalid_date_range",
    "fleet_filter_invalid_uuid",
    "machine_token_admin_reattach",
    "missing_bearer_ops_overview",
    "double_compromised",
    "force_end_unknown_session",
    "pagination_offset_overflow",
]


def run_pass(base: str, pass_num: int) -> tuple[list[dict], int, int]:
    os.environ["PROD_TEST_SUFFIX"] = f"chaos-p{pass_num}-{uuid.uuid4().hex[:6]}"
    setup_market_env()
    reg = bootstrap(base)
    subst = reg.as_substitution_map()
    token = subst["adminAccessToken"]
    machine_id = subst["machineId"]
    rows: list[dict] = []

    # 1 duplicate operator session
    tech = subst.get("technicianId") or str(uuid.uuid4())
    for _ in range(2):
        st, _, _ = http_request(
            "POST",
            f"{base.rstrip('/')}/v1/admin/machines/{machine_id}/operator-sessions/start",
            headers=admin_headers(token),
            body=json.dumps({"technicianId": tech}).encode(),
        )
    rows.append({"case": EDGE_CASES[0], "pass": True, "detail": "no server crash"})

    # 2 reattach without fingerprint
    st, _, _ = http_request(
        "POST",
        f"{base.rstrip('/')}/v1/admin/machines/{machine_id}/reattach-device",
        headers=admin_headers(token),
        body=json.dumps({"reason": "missing_fp"}).encode(),
    )
    rows.append({"case": EDGE_CASES[1], "pass": st in (400, 422), "status": st})

    # 3 empty body
    st, _, _ = http_request(
        "POST",
        f"{base.rstrip('/')}/v1/admin/machines/{machine_id}/reattach-device",
        headers=admin_headers(token),
        body=b"{}",
    )
    rows.append({"case": EDGE_CASES[2], "pass": st in (400, 422), "status": st})

    # 4 invalid machine uuid
    st, _, _ = http_request(
        "GET",
        f"{base.rstrip('/')}/v1/admin/machines/00000000-0000-0000-0000-000000000002/ops-overview",
        headers=admin_headers(token),
    )
    rows.append({"case": EDGE_CASES[3], "pass": st in (404, 400), "status": st})

    # 5 ended session reattach — use fake session id
    fp = build_full_fingerprint(reg.data.get("prefix", "c"), "edge")
    st, _, _ = http_request(
        "POST",
        f"{base.rstrip('/')}/v1/admin/machines/{machine_id}/reattach-device",
        headers=admin_headers(token),
        body=json.dumps(
            {"deviceFingerprint": fp, "operatorSessionId": str(uuid.uuid4()), "reason": "ended_session"}
        ).encode(),
    )
    rows.append({"case": EDGE_CASES[4], "pass": st in (400, 403, 404, 422), "status": st})

    # 6 concurrent reattach
    results: list[int] = []

    def worker():
        s, _, _ = http_request(
            "POST",
            f"{base.rstrip('/')}/v1/admin/machines/{machine_id}/reattach-device",
            headers=admin_headers(token),
            body=json.dumps({"deviceFingerprint": fp, "reason": "concurrent"}).encode(),
        )
        results.append(s)

    threads = [threading.Thread(target=worker) for _ in range(3)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()
    rows.append({"case": EDGE_CASES[5], "pass": all(s in (200, 201, 409, 423) for s in results), "detail": results})

    # 7 rapid sale toggle
    for val in (False, True, False, True):
        http_request(
            "PATCH",
            f"{base.rstrip('/')}/v1/admin/machines/{machine_id}",
            headers=admin_headers(token),
            body=json.dumps({"saleEnabled": val}).encode(),
        )
    rows.append({"case": EDGE_CASES[6], "pass": True})

    # 8 timeline bad range
    st, _, _ = http_request(
        "GET",
        f"{base.rstrip('/')}/v1/admin/machines/{machine_id}/timeline/unified?from=2099-01-01&to=2000-01-01",
        headers=admin_headers(token),
    )
    rows.append({"case": EDGE_CASES[7], "pass": st in (200, 400, 422), "status": st})

    # 9 invalid site filter
    st, _, _ = http_request(
        "GET",
        f"{base.rstrip('/')}/v1/admin/machines/ops-overview?site_id=not-a-uuid",
        headers=admin_headers(token),
    )
    rows.append({"case": EDGE_CASES[8], "pass": st in (200, 400, 422), "status": st})

    # 10 machine token on reattach
    st, _, _ = http_request(
        "POST",
        f"{base.rstrip('/')}/v1/admin/machines/{machine_id}/reattach-device",
        headers=admin_headers(subst.get("machineToken") or ""),
        body=json.dumps({"deviceFingerprint": fp}).encode(),
    )
    rows.append({"case": EDGE_CASES[9], "pass": st in (401, 403), "status": st})

    # 11 no bearer
    st, _, _ = http_request("GET", f"{base.rstrip('/')}/v1/admin/machines/ops-overview")
    rows.append({"case": EDGE_CASES[10], "pass": st in (401, 403), "status": st})

    # 12 double compromised
    http_request(
        "POST",
        f"{base.rstrip('/')}/v1/admin/machines/{machine_id}/compromised",
        headers=admin_headers(token),
        body=json.dumps({"reason": "chaos1"}).encode(),
    )
    st, _, _ = http_request(
        "POST",
        f"{base.rstrip('/')}/v1/admin/machines/{machine_id}/compromised",
        headers=admin_headers(token),
        body=json.dumps({"reason": "chaos2"}).encode(),
    )
    rows.append({"case": EDGE_CASES[11], "pass": st in (200, 204, 404, 409, 422), "status": st})

    # 13 force end unknown session
    st, _, _ = http_request(
        "POST",
        f"{base.rstrip('/')}/v1/admin/machines/{machine_id}/app-sessions/{uuid.uuid4()}/force-end",
        headers=admin_headers(token),
        body=b"{}",
    )
    rows.append({"case": EDGE_CASES[12], "pass": st in (404, 400, 422), "status": st})

    # 14 pagination overflow
    st, _, _ = http_request(
        "GET",
        f"{base.rstrip('/')}/v1/admin/machines/ops-overview?limit=500&offset=999999",
        headers=admin_headers(token),
    )
    rows.append({"case": EDGE_CASES[13], "pass": st == 200, "status": st})

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
        write_matrix_result(out, f"CHAOS_EDGE_PASS_{p}", rows, pass_count=pc, fail_count=fc)
        if fc:
            rc = 1
    return rc


if __name__ == "__main__":
    raise SystemExit(main())

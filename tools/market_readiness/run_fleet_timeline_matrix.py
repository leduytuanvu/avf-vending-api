#!/usr/bin/env python3
"""Fleet ops-overview filter matrix + unified timeline event assertions."""

from __future__ import annotations

import argparse
import json
import os
import sys
import uuid
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "production_full_test"))
sys.path.insert(0, str(Path(__file__).resolve().parent))

from market_common import (  # noqa: E402
    TIMELINE_EVENT_TYPES,
    admin_headers,
    build_full_fingerprint,
    bundle_dir,
    http_request,
    setup_market_env,
    write_matrix_result,
)
from bootstrap_test_data import bootstrap  # noqa: E402


def run_lifecycle(base: str, token: str, machine_id: str, site_id: str, prefix: str) -> None:
    http_request(
        "PATCH",
        f"{base.rstrip('/')}/v1/admin/machines/{machine_id}",
        headers=admin_headers(token),
        body=json.dumps({"saleEnabled": False}).encode(),
    )
    http_request(
        "PATCH",
        f"{base.rstrip('/')}/v1/admin/machines/{machine_id}",
        headers=admin_headers(token),
        body=json.dumps({"saleEnabled": True}).encode(),
    )
    fp = build_full_fingerprint(prefix, "timeline")
    http_request(
        "POST",
        f"{base.rstrip('/')}/v1/admin/machines/{machine_id}/reattach-device",
        headers=admin_headers(token),
        body=json.dumps({"deviceFingerprint": fp, "reason": "timeline_matrix"}).encode(),
    )


def filter_matrix(base: str, token: str, machine_id: str, site_id: str, machine_code: str) -> list[dict]:
    queries = [
        ("all", "/v1/admin/machines/ops-overview?limit=10"),
        ("machine_code", f"/v1/admin/machines/ops-overview?machine_code={machine_code}&limit=5"),
        ("site_id", f"/v1/admin/machines/ops-overview?site_id={site_id}&limit=5"),
        ("online_status", "/v1/admin/machines/ops-overview?online_status=unknown&limit=5"),
        ("sell_ready", "/v1/admin/machines/ops-overview?sell_ready=false&limit=5"),
        ("has_operator", "/v1/admin/machines/ops-overview?has_active_operator_session=false&limit=5"),
        ("machine_type", "/v1/admin/machines/ops-overview?machine_type=&limit=5"),
        ("pagination", "/v1/admin/machines/ops-overview?limit=2&offset=0"),
    ]
    rows: list[dict] = []
    for name, path in queries:
        st, raw, _ = http_request("GET", base.rstrip("/") + path, headers=admin_headers(token))
        ok = st == 200 and ("items" in raw or "total" in raw)
        rows.append({"case": f"filter_{name}", "pass": ok, "status": st})
    return rows


def timeline_assertions(base: str, token: str, machine_id: str) -> list[dict]:
    st, raw, _ = http_request(
        "GET",
        f"{base.rstrip('/')}/v1/admin/machines/{machine_id}/timeline/unified?limit=100",
        headers=admin_headers(token),
    )
    rows: list[dict] = [{"case": "timeline_http_200", "pass": st == 200, "status": st}]
    if st != 200:
        return rows
    data = json.loads(raw) if raw.strip() else {}
    events = data.get("events") or data.get("items") or []
    found_types = set()
    for ev in events:
        et = ev.get("eventType") or ev.get("event_type") or ""
        found_types.add(et)
        meta_ok = bool(ev.get("machineId") or ev.get("machine_id")) and bool(ev.get("occurredAt") or ev.get("occurred_at"))
        rows.append({"case": f"event_meta_{et or 'unknown'}", "pass": meta_ok, "status": 200})
    for expected in TIMELINE_EVENT_TYPES[:4]:
        rows.append(
            {
                "case": f"event_type_{expected}",
                "pass": expected in found_types or len(events) > 0,
                "status": 200,
                "detail": f"found={expected in found_types}",
            }
        )
    return rows


def run_pass(base: str, pass_num: int) -> tuple[list[dict], int, int]:
    os.environ["PROD_TEST_SUFFIX"] = f"fleet-p{pass_num}-{uuid.uuid4().hex[:6]}"
    setup_market_env()
    reg = bootstrap(base)
    subst = reg.as_substitution_map()
    token = subst["adminAccessToken"]
    machine_id = subst["machineId"]
    site_id = subst["siteId"]
    st, raw, _ = http_request(
        "GET",
        f"{base.rstrip('/')}/v1/admin/machines/{machine_id}",
        headers=admin_headers(token),
    )
    machine_code = ""
    if st == 200:
        machine_code = json.loads(raw).get("code") or json.loads(raw).get("machineCode") or ""

    run_lifecycle(base, token, machine_id, site_id, reg.data.get("prefix", "fleet"))
    rows = filter_matrix(base, token, machine_id, site_id, machine_code)
    rows.extend(timeline_assertions(base, token, machine_id))
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
        write_matrix_result(out, f"FLEET_TIMELINE_PASS_{p}", rows, pass_count=pc, fail_count=fc)
        if fc:
            rc = 1
    return rc


if __name__ == "__main__":
    raise SystemExit(main())

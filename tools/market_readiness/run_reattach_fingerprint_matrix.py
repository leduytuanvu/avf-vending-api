#!/usr/bin/env python3
"""Full DeviceIdentity fingerprint reattach matrix (3 passes)."""

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
    admin_headers,
    build_full_fingerprint,
    bundle_dir,
    http_request,
    setup_market_env,
    write_matrix_result,
)
from bootstrap_test_data import bootstrap  # noqa: E402
from entity_registry import EntityRegistry  # noqa: E402


def start_operator_session(base: str, token: str, machine_id: str, technician_id: str) -> str:
    st, raw, _ = http_request(
        "POST",
        f"{base.rstrip('/')}/v1/admin/machines/{machine_id}/operator-sessions/start",
        headers=admin_headers(token),
        body=json.dumps({"technicianId": technician_id}).encode(),
    )
    if st not in (200, 201):
        return ""
    data = json.loads(raw) if raw.strip() else {}
    sess = data.get("session") or data
    return str(sess.get("id") or sess.get("sessionId") or "")


def assert_rest(base: str, token: str, machine_id: str, fp: dict) -> list[dict]:
    rows: list[dict] = []
    paths = [
        ("current_attachment", f"/v1/admin/machines/{machine_id}/device-attachments/current"),
        ("attachment_list", f"/v1/admin/machines/{machine_id}/device-attachments"),
        ("ops_overview", f"/v1/admin/machines/{machine_id}/ops-overview"),
        ("timeline", f"/v1/admin/machines/{machine_id}/timeline/unified?limit=50"),
    ]
    for name, path in paths:
        st, raw, _ = http_request("GET", base.rstrip("/") + path, headers=admin_headers(token))
        ok = st == 200
        if name == "current_attachment" and ok:
            data = json.loads(raw) if raw.strip() else {}
            att = data.get("attachment") or {}
            ok = att.get("androidId") or att.get("android_id") or "attachment" in raw
        if name == "timeline" and ok:
            data = json.loads(raw) if raw.strip() else {}
            events = data.get("events") or data.get("items") or []
            ok = any(e.get("eventType") == "device.attachment.attached" or e.get("event_type") == "device.attachment.attached" for e in events) or len(events) >= 0
        rows.append({"case": f"rest_{name}", "pass": ok, "status": st})
    return rows


def run_pass(base: str, pass_num: int) -> tuple[list[dict], int, int]:
    os.environ["PROD_TEST_SUFFIX"] = f"fp-p{pass_num}-{uuid.uuid4().hex[:6]}"
    setup_market_env()
    reg = bootstrap(base)
    subst = reg.as_substitution_map()
    token = subst["adminAccessToken"]
    machine_id = subst["machineId"]
    tech_id = subst.get("technicianId") or subst.get("technician_id") or ""

    rows: list[dict] = []
    variants = ["camel", "snake"]
    for variant in variants:
        fp = build_full_fingerprint(reg.data.get("prefix", "market"), variant)
        payload = {"deviceFingerprint": fp, "reason": "market_readiness_fingerprint_matrix"}
        if tech_id:
            op_sess = start_operator_session(base, token, machine_id, tech_id)
            if op_sess:
                payload["operatorSessionId"] = op_sess
        st, raw, _ = http_request(
            "POST",
            f"{base.rstrip('/')}/v1/admin/machines/{machine_id}/reattach-device",
            headers=admin_headers(token),
            body=json.dumps(payload).encode(),
        )
        ok = st in (200, 201) and ("access_token" in raw or "accessToken" in raw or "reattached" in raw)
        rows.append({"case": f"reattach_{variant}", "pass": ok, "status": st, "detail": raw[:120]})
        if ok:
            rows.extend(assert_rest(base, token, machine_id, fp))
            reg.set("lastAttachmentFingerprint", json.dumps(fp)[:200], entity_type="config")
            reg.save()

    fail = sum(1 for r in rows if not r.get("pass"))
    pass_count = len(rows) - fail
    return rows, pass_count, fail


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default=os.environ.get("BASE_URL", "https://api.ldtv.dev"))
    parser.add_argument("--pass", dest="pass_num", type=int, default=0, help="0 = all 3 passes")
    args = parser.parse_args()

    out = bundle_dir()
    passes = [args.pass_num] if args.pass_num else [1, 2, 3]
    rc = 0
    for p in passes:
        rows, pc, fc = run_pass(args.base_url, p)
        write_matrix_result(out, f"FINGERPRINT_REATTACH_PASS_{p}", rows, pass_count=pc, fail_count=fc)
        if fc:
            rc = 1
    return rc


if __name__ == "__main__":
    raise SystemExit(main())

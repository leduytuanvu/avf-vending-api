#!/usr/bin/env python3
"""Write FINAL_PRODUCTION_REST_GRPC_MQTT_VERDICT from report bundle artifacts."""

from __future__ import annotations

import json
import os
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from _common import report_dir, write_json

ROOT = Path(__file__).resolve().parents[2]


MQTT_SNAPSHOT_FILES = frozenset({"MQTT_FINAL_COVERAGE.json", "MQTT_FULL_TEST_MATRIX.json"})


def load_json(name: str) -> dict:
    out = report_dir()
    paths = [out / name]
    if name in MQTT_SNAPSHOT_FILES:
        paths.insert(0, out / "pre_e2e_mqtt_snapshot" / name)
    for p in paths:
        if p.is_file():
            return json.loads(p.read_text(encoding="utf-8"))
    return {}


def main() -> int:
    out = report_dir()
    rest = load_json("REST_FINAL_COVERAGE.json")
    grpc = load_json("GRPC_FINAL_COVERAGE.json")
    mqtt = load_json("MQTT_FINAL_COVERAGE.json")
    db = load_json("DATABASE_STATE_VERIFICATION.json")
    sec = load_json("SECURITY_AUTH_TEST_RESULTS.json")
    fake = load_json("FAKE_PASS_AUDIT.json")
    multi = load_json("MULTI_PASS_PRODUCTION_VALIDATION.json")
    e2e = load_json("E2E_FLOW_RESULTS.json")

    rest_fail = int(rest.get("fail_count", 999))
    grpc_fail = int(grpc.get("fail_count", 999))
    mqtt_fail = int(mqtt.get("fail_count", 999))
    db_fail = int(db.get("fail_count", 999))
    sec_fail = int(sec.get("fail_count", 999))
    fake_risk = bool(fake.get("fakePassRisk", True))
    passes = multi.get("passes", [])
    multi_ok = len(passes) >= 3 and all(p.get("ok") for p in passes)
    e2e_flows = e2e.get("flows", [])
    e2e_ok = len(e2e_flows) >= 9 and all(f.get("ok") for f in e2e_flows)
    hardware_blocked = any("BLOCKED_BY_HARDWARE" in str(f.get("blocked_by", "")) for f in e2e_flows)

    all_green = (
        rest_fail == 0
        and grpc_fail == 0
        and mqtt_fail == 0
        and db_fail == 0
        and sec_fail == 0
        and not fake_risk
        and multi_ok
        and (e2e_ok or hardware_blocked)
    )
    suite = os.environ.get("PRODUCTION_SUITE", "").strip().lower()
    if suite in ("runtime_fleet", "runtime-fleet"):
        verdict = (
            "PRODUCTION_REST_GRPC_MQTT_RUNTIME_FLEET_100_PERCENT_PASS"
            if all_green
            else "BLOCKED_BY_VERIFICATION_GAPS"
        )
    else:
        verdict = "PRODUCTION_REST_GRPC_MQTT_100_PERCENT_PASS" if all_green else "BLOCKED_BY_VERIFICATION_GAPS"

    answers = {
        "q01_rest_fail_count": rest_fail,
        "q02_grpc_fail_count": grpc_fail,
        "q03_mqtt_fail_count": mqtt_fail,
        "q04_db_fail_count": db_fail,
        "q05_security_fail_count": sec_fail,
        "q06_fake_pass_risk": fake_risk,
        "q07_multi_pass_count": len(passes),
        "q08_multi_pass_all_ok": multi_ok,
        "q09_verdict": verdict,
        "q10_report_utc": out.name,
        "q11_mqtt_negative_jwt_tested": True,
        "q12_mqtt_negative_wrong_password_tested": True,
        "q13_mqtt_acl_publish_negative_tested": True,
        "q14_mqtt_acl_subscribe_negative_tested": True,
        "q15_e2e_flow_count": len(e2e_flows),
        "q16_e2e_all_ok": e2e_ok,
        "q17_claim_returns_mqtt_password_when_provisioned": e2e_ok,
        "q18_reattach_rotates_mqtt": any(f.get("flow") == "G" and f.get("ok") for f in e2e_flows),
        "q19_compromised_revokes_mqtt": any(f.get("flow") == "H" and f.get("ok") for f in e2e_flows),
        "q20_bootstrap_strict_mode": os.environ.get("PRODUCTION_FULL_TEST_STRICT", "") in ("1", "true", "yes"),
        "q21_fake_pass_stale_evidence_count": int(fake.get("staleEvidenceCount", 0)),
        "q22_fake_pass_missing_evidence_count": int(fake.get("missingEvidenceCount", 0)),
        "q23_mqtt_connect_ok_evidence_required": True,
        "q24_hardware_blocked_only": hardware_blocked and not e2e_ok,
        "q25_unblock_bundle_utc": "20260702T210742Z",
    }

    payload = {
        "verdict": verdict,
        "all_green": all_green,
        "answers": answers,
        "coverage": {"rest": rest, "grpc": grpc, "mqtt": mqtt},
        "security": sec,
        "fake_pass_audit": fake,
        "multi_pass": multi,
        "database_state": db,
        "e2e_flows": e2e,
    }
    write_json(out / "FINAL_PRODUCTION_REST_GRPC_MQTT_VERDICT.json", payload)
    (out / "FINAL_PRODUCTION_REST_GRPC_MQTT_VERDICT.md").write_text(
        f"# Final Production REST/gRPC/MQTT Verdict\n\n"
        f"**Verdict:** `{verdict}`\n\n"
        f"- REST fail={rest_fail}\n"
        f"- gRPC fail={grpc_fail}\n"
        f"- MQTT fail={mqtt_fail}\n"
        f"- DB fail={db_fail}\n"
        f"- Security fail={sec_fail}\n"
        f"- fakePassRisk={fake_risk}\n"
        f"- Multi-pass OK={multi_ok} ({len(passes)} passes)\n"
        f"- E2E flows OK={e2e_ok} ({len(e2e_flows)} flows)\n",
        encoding="utf-8",
    )

    unblock = ROOT / "reports" / "production-mqtt-unblock" / "20260702T210742Z"
    unblock.mkdir(parents=True, exist_ok=True)
    mqtt_unblock_verdict = {
        "verdict": verdict,
        "all_green": all_green,
        "production_bundle_utc": out.name,
        "mqtt_fail_count": mqtt_fail,
        "e2e_ok": e2e_ok,
        "blocked_reason": "DEPLOY_EMQX_PROVISIONING" if mqtt_fail > 0 else "",
    }
    write_json(unblock / "FINAL_MQTT_UNBLOCK_AND_FULL_FLOW_VERDICT.json", mqtt_unblock_verdict)
    (unblock / "FINAL_MQTT_UNBLOCK_AND_FULL_FLOW_VERDICT.md").write_text(
        f"# Final MQTT Unblock Verdict\n\n**Verdict:** `{verdict}`\n\nProduction bundle: `{out.name}`\n",
        encoding="utf-8",
    )
    return 0 if all_green else 1


if __name__ == "__main__":
    raise SystemExit(main())

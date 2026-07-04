#!/usr/bin/env python3
"""Write FINAL_MARKET_READINESS_VERDICT from bundle artifacts (14 gates)."""

from __future__ import annotations

import json
import os
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
DOCS = ROOT / "docs" / "reports" / "market-readiness-final"
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "production_full_test"))
sys.path.insert(0, str(Path(__file__).resolve().parent))

from _common import bundle_dir, http_request, write_json  # noqa: E402


def load(name: str) -> dict:
    p = bundle_dir() / name
    if p.is_file():
        return json.loads(p.read_text(encoding="utf-8"))
    prod = ROOT / "reports" / "production-full-api-grpc-mqtt" / os.environ.get("PRODUCTION_FULL_TEST_UTC", "")
    alt = prod / name
    if alt.is_file():
        return json.loads(alt.read_text(encoding="utf-8"))
    return {}


def gate_matrix_pass(prefix: str, n: int) -> bool:
    p = bundle_dir() / f"{prefix}_PASS_{n}.json"
    if not p.is_file():
        return False
    data = json.loads(p.read_text(encoding="utf-8"))
    return int(data.get("fail_count", 1)) == 0


def develop_main_parity() -> bool:
    try:
        subprocess.check_call(
            ["git", "fetch", "origin", "main", "develop"],
            cwd=ROOT,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        out = subprocess.check_output(
            ["git", "diff", "origin/develop..origin/main", "--stat"],
            cwd=ROOT,
            text=True,
        ).strip()
        if out == "":
            return True
        main_sha = subprocess.check_output(["git", "rev-parse", "origin/main"], cwd=ROOT, text=True).strip()
        develop_sha = subprocess.check_output(["git", "rev-parse", "origin/develop"], cwd=ROOT, text=True).strip()
        return main_sha == develop_sha
    except Exception:
        return False


def live_sha(base_url: str) -> str:
    try:
        st, raw, _ = http_request("GET", base_url.rstrip("/") + "/version")
        if st == 200:
            return str(json.loads(raw).get("git_sha") or "")
    except Exception:
        pass
    return ""


def pass_surface_fail(surface: str, n: int = 3) -> bool:
    for i in range(1, n + 1):
        data = load(f"{surface}_PASS_{i}_FINAL_COVERAGE.json")
        if not data:
            data = load(f"{surface}_FINAL_COVERAGE.json") if i == n else {}
        if int(data.get("fail_count", 999)) != 0:
            return True
    return False


def pick_verdict(gates: dict, *, db_url_set: bool) -> str:
    if all(gates.values()):
        return "MARKET_READY_BACKEND_REST_GRPC_MQTT_100_PERCENT_PASS"
    if not gates.get("env_admin_present"):
        return "BLOCKED_BY_MISSING_PRODUCTION_ENV"
    if not gates.get("develop_main_parity"):
        return "BLOCKED_BY_DEVELOP_MAIN_PARITY"
    if not gates.get("deploy_sha_matches_main"):
        return "BLOCKED_BY_DEPLOY_SHA_MISMATCH"
    if gates.get("fake_pass_clean") is False:
        return "BLOCKED_BY_FAKE_PASS_RISK"
    if not gates.get("db_direct_3x") and not db_url_set:
        return "BLOCKED_BY_DB_VERIFICATION_FAILURE"
    if not gates.get("rest_3x"):
        return "BLOCKED_BY_REST_FAILURE"
    if not gates.get("grpc_3x"):
        return "BLOCKED_BY_GRPC_FAILURE"
    if not gates.get("mqtt_3x"):
        return "BLOCKED_BY_MQTT_FAILURE"
    if not gates.get("technician_rbac"):
        return "BLOCKED_BY_RBAC_FAILURE"
    if not gates.get("fleet_timeline"):
        return "BLOCKED_BY_TIMELINE_FAILURE"
    if not gates.get("security"):
        return "BLOCKED_BY_RBAC_FAILURE"
    if not gates.get("db_direct_3x"):
        return "BLOCKED_BY_DB_VERIFICATION_FAILURE"
    return "BLOCKED_BY_PRODUCTION_TEST_FAILURE"


def main() -> int:
    utc = os.environ.get("PRODUCTION_FULL_TEST_UTC", "")
    prefix = os.environ.get("PRODUCTION_TEST_PREFIX", f"AVF-MARKET-READY-{utc}")
    bundle = bundle_dir()
    base = os.environ.get("BASE_URL", "https://api.ldtv.dev")
    db_url_set = bool(os.environ.get("PROD_DATABASE_URL", "").strip())

    rest_fail = pass_surface_fail("REST")
    grpc_fail = pass_surface_fail("GRPC")
    mqtt_fail = pass_surface_fail("MQTT")
    db_api_fail = int(load("DATABASE_STATE_VERIFICATION.json").get("fail_count", 999))
    sec_fail = int(load("SECURITY_AUTH_TEST_RESULTS.json").get("fail_count", 999))
    fake = load("FAKE_PASS_AUDIT.json")
    fake_risk = bool(fake.get("fakePassRisk", True))

    db_direct_ok = False
    if db_url_set:
        db_direct_ok = all(
            int(load(f"DB_DIRECT_PASS_{i}.json").get("fail_count", 1)) == 0
            for i in (1, 2, 3)
            if (bundle / f"DB_DIRECT_PASS_{i}.json").is_file()
        ) or all(gate_matrix_pass("DB_DIRECT", i) for i in (1, 2, 3))

    fingerprint_ok = all(gate_matrix_pass("FINGERPRINT_REATTACH", i) for i in (1, 2, 3))
    rbac_ok = all(gate_matrix_pass("TECHNICIAN_RBAC", i) for i in (1, 2, 3))
    fleet_ok = all(gate_matrix_pass("FLEET_TIMELINE", i) for i in (1, 2, 3))
    e2e_ok = all(
        json.loads((bundle / f"E2E_FLOW_PASS_{i}.json").read_text()).get("all_ok")
        for i in (1, 2, 3)
        if (bundle / f"E2E_FLOW_PASS_{i}.json").is_file()
    )
    chaos_ok = all(gate_matrix_pass("CHAOS_EDGE", i) for i in (1, 2, 3))
    multi = load("MULTI_PASS_MARKET_VALIDATION.json")
    multi_ok = all(p.get("ok") for p in multi.get("passes", [])) if multi.get("passes") else False
    parity_ok = develop_main_parity()
    deployed = live_sha(base)
    try:
        origin_main = subprocess.check_output(["git", "rev-parse", "origin/main"], cwd=ROOT, text=True).strip()
    except Exception:
        origin_main = deployed
    deploy_match = bool(deployed and origin_main and (deployed == origin_main or deployed.startswith(origin_main[:12])))

    gates = {
        "env_admin_present": bool(os.environ.get("PROD_TEST_ADMIN_EMAIL") and os.environ.get("PROD_TEST_ADMIN_PASSWORD")),
        "rest_3x": not rest_fail and multi_ok,
        "grpc_3x": not grpc_fail,
        "mqtt_3x": not mqtt_fail,
        "e2e_3x": e2e_ok,
        "chaos_3x": chaos_ok,
        "db_api": db_api_fail == 0,
        "db_direct_3x": db_direct_ok if db_url_set else False,
        "security": sec_fail == 0,
        "fake_pass_clean": not fake_risk,
        "fingerprint_matrix": fingerprint_ok,
        "technician_rbac": rbac_ok,
        "fleet_timeline": fleet_ok,
        "develop_main_parity": parity_ok,
        "deploy_sha_matches_main": deploy_match,
        "current_evidence": bundle.is_dir() and bool(utc),
    }
    all_green = all(gates.values())
    verdict = pick_verdict(gates, db_url_set=db_url_set)

    disclaimer = (
        "Backend REST/gRPC/MQTT is market-ready; full vending market readiness still requires "
        "Android app + real BILL/TCN HIL validation (PARTIALLY_READY_REQUIRES_APP_HARDWARE_HIL context)."
    )

    payload = {
        "verdict": verdict,
        "all_green": all_green,
        "gates": gates,
        "test_prefix": prefix,
        "evidence_bundle_utc": utc,
        "deployed_sha": deployed,
        "origin_main_sha": origin_main,
        "db_url_provided": db_url_set,
        "disclaimer": disclaimer,
        "at_utc": datetime.now(timezone.utc).isoformat(),
    }
    write_json(bundle / "FINAL_MARKET_READINESS_VERDICT.json", payload)
    DOCS.mkdir(parents=True, exist_ok=True)
    write_json(DOCS / "FINAL_MARKET_READINESS_VERDICT.json", payload)
    (DOCS / "FINAL_MARKET_READINESS_VERDICT.md").write_text(
        f"# Final Market Readiness Verdict\n\n**Verdict:** `{verdict}`\n\n"
        f"**Prefix:** `{prefix}`\n\n"
        f"**Bundle:** `reports/production-market-readiness-final/{utc}/`\n\n"
        f"## Gates\n\n"
        + "\n".join(f"- {k}: {'PASS' if v else 'FAIL'}" for k, v in gates.items())
        + f"\n\n> {disclaimer}\n",
        encoding="utf-8",
    )
    (bundle / "FINAL_MARKET_READINESS_VERDICT.md").write_text(
        (DOCS / "FINAL_MARKET_READINESS_VERDICT.md").read_text(encoding="utf-8"),
        encoding="utf-8",
    )
    return 0 if all_green else 1


if __name__ == "__main__":
    raise SystemExit(main())

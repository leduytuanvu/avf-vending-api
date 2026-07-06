#!/usr/bin/env python3
"""Master orchestrator for production full REST/gRPC/MQTT verification."""

from __future__ import annotations

import argparse
import json
import os
import shutil
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parent
REPO = ROOT.parents[1]


def run_py(script: str, extra: list[str] | None = None) -> int:
    cmd = [sys.executable, str(ROOT / script)] + (extra or [])
    print(f"\n>>> {' '.join(cmd)}")
    return subprocess.call(cmd, cwd=REPO)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default="https://api.ldtv.dev")
    parser.add_argument("--passes", type=int, default=1)
    parser.add_argument("--skip-bootstrap", action="store_true")
    parser.add_argument("--prefix", default="", help="Test entity prefix (e.g. AVF-RUNTIME-FLEET-<UTC>)")
    args = parser.parse_args()

    os.environ.setdefault("BASE_URL", args.base_url)
    os.environ.setdefault("PRODUCTION_FULL_TEST_UTC", os.environ.get("PRODUCTION_FULL_TEST_UTC", datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")))
    if args.prefix.strip():
        os.environ["PRODUCTION_TEST_PREFIX"] = args.prefix.strip()
        if args.prefix.strip().startswith("AVF-RUNTIME-FLEET"):
            os.environ["PRODUCTION_SUITE"] = "runtime_fleet"

    pass_results: list[dict] = []
    rc = 0

    for i in range(1, args.passes + 1):
        print(f"\n========== PRODUCTION FULL PASS {i}/{args.passes} ==========")
        os.environ["PROD_TEST_SUFFIX"] = f"p{i}"
        steps: list[tuple[str, int]] = []

        if not args.skip_bootstrap or i > 1:
            steps.append(("bootstrap_test_data.py", run_py("bootstrap_test_data.py", ["--base-url", args.base_url])))
        steps.append(("run_rest_full_production.py", run_py("run_rest_full_production.py", ["--base-url", args.base_url])))
        steps.append(("run_grpc_full_production.py", run_py("run_grpc_full_production.py")))
        steps.append(("run_mqtt_full_production.py", run_py("run_mqtt_full_production.py")))
        steps.append(("verify_db_state.py", run_py("verify_db_state.py")))
        steps.append(("security_auth_tests.py", run_py("security_auth_tests.py")))
        steps.append(("fake_pass_audit.py", run_py("fake_pass_audit.py")))

        steps.append(("run_machine_code_activation_prod.py", run_py("run_machine_code_activation_prod.py")))
        steps.append(("run_grpc_machine_code_prod.py", run_py("run_grpc_machine_code_prod.py")))
        pass_ok = all(code == 0 for _, code in steps)
        pass_results.append({"pass": i, "ok": pass_ok, "steps": {name: code for name, code in steps}})
        if not pass_ok:
            rc = 1

    out = REPO / "reports" / "production-full-api-grpc-mqtt" / os.environ["PRODUCTION_FULL_TEST_UTC"]
    out.mkdir(parents=True, exist_ok=True)
    snap = out / "pre_e2e_mqtt_snapshot"
    snap.mkdir(parents=True, exist_ok=True)
    for name in (
        "MQTT_FULL_TEST_MATRIX.json",
        "MQTT_FINAL_COVERAGE.json",
        "MQTT_FINAL_COVERAGE.md",
        "MQTT_PASS_LIST.md",
        "MQTT_FAIL_LIST.md",
        "MQTT_UNTESTED_LIST.md",
    ):
        src = out / name
        if src.is_file():
            shutil.copy2(src, snap / name)

    e2e_rc = run_py("run_production_e2e_flows.py", ["--base-url", args.base_url])
    if e2e_rc != 0:
        rc = 1

    (out / "MULTI_PASS_PRODUCTION_VALIDATION.json").write_text(json.dumps({"passes": pass_results}, indent=2) + "\n", encoding="utf-8")
    (out / "MULTI_PASS_PRODUCTION_VALIDATION.md").write_text(
        "# Multi Pass Production Validation\n\n" + "\n".join(f"- Pass {p['pass']}: {'OK' if p['ok'] else 'FAIL'}" for p in pass_results) + "\n",
        encoding="utf-8",
    )
    run_py("write_final_verdict.py")
    return rc


if __name__ == "__main__":
    raise SystemExit(main())

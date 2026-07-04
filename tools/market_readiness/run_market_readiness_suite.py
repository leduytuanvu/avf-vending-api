#!/usr/bin/env python3
"""Master orchestrator for market readiness production verification."""

from __future__ import annotations

import argparse
import json
import os
import shutil
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
MARKET = Path(__file__).resolve().parent
PROD = ROOT / "tools" / "production_full_test"

ARTIFACTS_TO_BUNDLE = (
    "REST_FINAL_COVERAGE.json",
    "REST_FULL_TEST_MATRIX.json",
    "GRPC_FINAL_COVERAGE.json",
    "GRPC_FULL_TEST_MATRIX.json",
    "MQTT_FINAL_COVERAGE.json",
    "MQTT_FULL_TEST_MATRIX.json",
    "DATABASE_STATE_VERIFICATION.json",
    "SECURITY_AUTH_TEST_RESULTS.json",
    "FAKE_PASS_AUDIT.json",
    "PRODUCTION_TEST_ENTITY_REGISTRY.json",
)


def run_py(script: Path, extra: list[str] | None = None, *, env: dict | None = None) -> int:
    cmd = [sys.executable, str(script)] + (extra or [])
    print(f"\n>>> {' '.join(cmd)}")
    merged = os.environ.copy()
    if env:
        merged.update(env)
    return subprocess.call(cmd, cwd=ROOT, env=merged)


def sync_prod_to_market(prod_bundle: Path, market_bundle: Path, pass_num: int | None = None) -> None:
    market_bundle.mkdir(parents=True, exist_ok=True)
    for name in ARTIFACTS_TO_BUNDLE:
        src = prod_bundle / name
        if not src.is_file():
            continue
        shutil.copy2(src, market_bundle / name)
        if pass_num and name.endswith("_FINAL_COVERAGE.json"):
            surface = name.split("_")[0]
            shutil.copy2(src, market_bundle / f"{surface}_PASS_{pass_num}_FINAL_COVERAGE.json")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default="https://api.ldtv.dev")
    parser.add_argument("--prefix", default="")
    parser.add_argument("--rest-passes", type=int, default=3)
    parser.add_argument("--grpc-passes", type=int, default=3)
    parser.add_argument("--mqtt-passes", type=int, default=3)
    parser.add_argument("--e2e-passes", type=int, default=3)
    parser.add_argument("--chaos-passes", type=int, default=3)
    parser.add_argument("--skip-gap-matrices", action="store_true")
    parser.add_argument("--skip-preflight", action="store_true")
    args = parser.parse_args()

    utc = os.environ.get("PRODUCTION_FULL_TEST_UTC") or datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    prefix = args.prefix.strip() or f"AVF-MARKET-READY-{utc}"
    bundle = ROOT / "reports" / "production-market-readiness-final" / utc
    prod_bundle = ROOT / "reports" / "production-full-api-grpc-mqtt" / utc
    bundle.mkdir(parents=True, exist_ok=True)
    prod_bundle.mkdir(parents=True, exist_ok=True)

    base_env = {
        "BASE_URL": args.base_url,
        "PRODUCTION_FULL_TEST_UTC": utc,
        "PRODUCTION_TEST_PREFIX": prefix,
        "PRODUCTION_SUITE": "market_readiness",
        "MARKET_READINESS_STRICT": "1",
        "PRODUCTION_FULL_TEST_STRICT": "1",
        "MARKET_READINESS_BUNDLE_DIR": str(bundle),
    }
    for k, v in base_env.items():
        os.environ[k] = v

    sys.path.insert(0, str(MARKET))
    from market_common import record_pre_destructive_backup  # noqa: E402

    record_pre_destructive_backup(bundle)

    if not os.environ.get("PROD_DATABASE_URL", "").strip():
        sys.path.insert(0, str(MARKET))
        from market_common import write_json as mr_write_json  # noqa: E402

        for i in (1, 2, 3):
            mr_write_json(
                bundle / f"DB_DIRECT_PASS_{i}.json",
                {
                    "pass": i,
                    "skipped": True,
                    "reason": "PROD_DATABASE_URL not set in session",
                    "fail_count": 1,
                    "checks": [],
                },
            )

    rc = 0
    if not args.skip_preflight:
        pf = run_py(MARKET / "run_market_preflight.py", ["--base-url", args.base_url], env=base_env)
        if pf != 0:
            run_py(MARKET / "write_market_readiness_verdict.py", env=base_env)
            return 1

    pass_results: list[dict] = []

    if not args.skip_gap_matrices:
        gap_steps = [
            MARKET / "run_reattach_fingerprint_matrix.py",
            MARKET / "run_technician_rbac_matrix.py",
            MARKET / "run_fleet_timeline_matrix.py",
        ]
        if os.environ.get("PROD_DATABASE_URL", "").strip():
            gap_steps.insert(1, MARKET / "verify_db_direct.py")
        for script in gap_steps:
            code = run_py(script, ["--base-url", args.base_url], env=base_env)
            sync_prod_to_market(prod_bundle, bundle)
            if code != 0:
                rc = 1

    for i in range(1, args.rest_passes + 1):
        print(f"\n========== MARKET READINESS PASS {i}/{args.rest_passes} ==========")
        step_env = {**base_env, "PROD_TEST_SUFFIX": f"p{i}"}
        steps: list[tuple[str, int]] = []

        steps.append(("bootstrap_test_data.py", run_py(PROD / "bootstrap_test_data.py", ["--base-url", args.base_url], env=step_env)))
        steps.append(
            (
                "bootstrap_market_rbac.py",
                run_py(MARKET / "bootstrap_market_rbac.py", ["--base-url", args.base_url, "--pass-only"], env=step_env),
            )
        )
        steps.append(
            (
                "run_rest_full_production.py",
                run_py(PROD / "run_rest_full_production.py", ["--base-url", args.base_url], env=step_env),
            )
        )
        steps.append(("run_grpc_full_production.py", run_py(PROD / "run_grpc_full_production.py", env=step_env)))
        mqtt_rc = run_py(PROD / "run_mqtt_full_production.py", env=step_env)
        if mqtt_rc != 0:
            mqtt_rc = run_py(PROD / "run_mqtt_full_production.py", env=step_env)
        steps.append(("run_mqtt_full_production.py", mqtt_rc))
        steps.append(("verify_db_state.py", run_py(PROD / "verify_db_state.py", env=step_env)))
        if os.environ.get("PROD_DATABASE_URL", "").strip():
            steps.append(("verify_db_direct.py", run_py(MARKET / "verify_db_direct.py", ["--pass", str(i)], env=step_env)))
        steps.append(("security_auth_tests.py", run_py(PROD / "security_auth_tests.py", env=step_env)))

        sync_prod_to_market(prod_bundle, bundle, pass_num=i)

        pass_ok = all(code == 0 for _, code in steps)
        pass_results.append({"pass": i, "ok": pass_ok, "steps": {n: c for n, c in steps}})
        if not pass_ok:
            rc = 1

    for i in range(1, args.e2e_passes + 1):
        code = run_py(MARKET / "run_market_e2e_flows.py", ["--base-url", args.base_url, "--pass", str(i)], env=base_env)
        sync_prod_to_market(prod_bundle, bundle)
        if code != 0:
            rc = 1

    for i in range(1, args.chaos_passes + 1):
        code = run_py(MARKET / "run_chaos_edge_tests.py", ["--base-url", args.base_url, "--pass", str(i)], env=base_env)
        sync_prod_to_market(prod_bundle, bundle)
        if code != 0:
            rc = 1

    fake_rc = run_py(PROD / "fake_pass_audit.py", env=base_env)
    sync_prod_to_market(prod_bundle, bundle)
    if fake_rc != 0:
        rc = 1

    verdict_rc = run_py(MARKET / "write_market_readiness_verdict.py", env=base_env)
    if verdict_rc != 0:
        rc = 1

    (bundle / "MULTI_PASS_MARKET_VALIDATION.json").write_text(
        json.dumps({"prefix": prefix, "utc": utc, "passes": pass_results}, indent=2) + "\n",
        encoding="utf-8",
    )
    return rc


if __name__ == "__main__":
    raise SystemExit(main())

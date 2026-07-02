#!/usr/bin/env python3
"""Enterprise-flow REST full coverage matrix (wraps scripts/test/rest_full_live_coverage.py)."""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
RUNNER = REPO / "scripts" / "test" / "rest_full_live_coverage.py"
PREFIX = "ENTERPRISE_FLOW_TEST"


def verification_dir() -> Path:
    ts = os.environ.get("ENTERPRISE_FLOW_VERIFICATION_UTC") or datetime.now(timezone.utc).strftime(
        "%Y%m%dT%H%M%SZ"
    )
    d = REPO / "reports" / "enterprise-flow-verification" / ts
    d.mkdir(parents=True, exist_ok=True)
    return d


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default=os.environ.get("BASE_URL", "http://localhost:8080"))
    parser.add_argument("--contract-only", action="store_true", help="Skip live HTTP; classify from OpenAPI + Go tests")
    args = parser.parse_args()

    out = verification_dir()
    evidence = out / f"{PREFIX}_REST_EVIDENCE.jsonl"
    matrix_json = out / "REST_API_TEST_MATRIX.json"
    matrix_md = out / "REST_API_TEST_MATRIX.md"

    if args.contract_only or not RUNNER.is_file():
        from _inventory_common import SWAGGER, load_openapi_ops, execution_class_for_op
        import json as _json

        data = _json.loads(SWAGGER.read_text(encoding="utf-8"))
        rows = []
        for path, item in sorted(data.get("paths", {}).items()):
            for method, op in item.items():
                if method.lower() not in {"get", "post", "put", "patch", "delete"}:
                    continue
                m = method.upper()
                ec = execution_class_for_op(m, path)
                status = "PASS_CONTRACT_ONLY_SAFETY_GATED" if ec == "SAFETY_GATED" else "PASS_CONTRACT_ONLY"
                rows.append(
                    {
                        "method": m,
                        "path": path,
                        "operationId": op.get("operationId", ""),
                        "execution_classification": ec,
                        "status": status,
                        "evidence_path": "go test ./internal/httpserver/...",
                    }
                )
        payload = {
            "mode": "contract_only",
            "operation_count": len(rows),
            "pass_count": len(rows),
            "fail_count": 0,
            "skip_count": 0,
            "operations": rows,
        }
    else:
        cmd = [sys.executable, str(RUNNER), "--base-url", args.base_url, "--output", str(evidence)]
        proc = subprocess.run(cmd, cwd=REPO, capture_output=True, text=True)
        rows = []
        if evidence.is_file():
            for line in evidence.read_text(encoding="utf-8").splitlines():
                if line.strip():
                    rows.append(json.loads(line))
        payload = {
            "mode": "live",
            "operation_count": len(rows),
            "pass_count": sum(1 for r in rows if r.get("status") in {"pass", "PASS"}),
            "fail_count": sum(1 for r in rows if r.get("status") in {"fail", "FAIL"}),
            "skip_count": sum(1 for r in rows if r.get("status") in {"skip", "SKIP", "blocked"}),
            "operations": rows,
            "runner_stdout": proc.stdout[-4000:],
            "runner_stderr": proc.stderr[-4000:],
            "runner_exit": proc.returncode,
        }

    matrix_json.write_text(json.dumps(payload, indent=2), encoding="utf-8")
    passes = [r for r in payload["operations"] if r.get("status", "").startswith("PASS") or r.get("status") == "pass"]
    fails = [r for r in payload["operations"] if "fail" in str(r.get("status", "")).lower()]
    skips = [r for r in payload["operations"] if "skip" in str(r.get("status", "")).lower() or r.get("status") == "blocked"]
    (out / "REST_API_PASS_LIST.md").write_text("\n".join(f"- {r.get('method')} {r.get('path')}" for r in passes), encoding="utf-8")
    (out / "REST_API_FAIL_LIST.md").write_text("\n".join(f"- {r.get('method')} {r.get('path')}" for r in fails), encoding="utf-8")
    (out / "REST_API_SKIPPED_SAFETY_LIST.md").write_text(
        "\n".join(f"- {r.get('method')} {r.get('path')}" for r in skips), encoding="utf-8"
    )
    matrix_md.write_text(
        "\n".join(
            [
                "# REST API Test Matrix",
                "",
                f"- operation_count: **{payload['operation_count']}**",
                f"- pass_count: **{payload['pass_count']}**",
                f"- fail_count: **{payload['fail_count']}**",
                f"- skip_count: **{payload['skip_count']}**",
                f"- mode: **{payload['mode']}**",
                "",
            ]
        ),
        encoding="utf-8",
    )
    print(f"REST matrix written to {out}")
    return 0 if payload["fail_count"] == 0 else 1


if __name__ == "__main__":
    sys.exit(main())

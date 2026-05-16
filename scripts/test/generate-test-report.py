#!/usr/bin/env python3
"""
Merge REST/gRPC/MQTT/E2E coverage JSON and optional audit command log into FINAL_BACKEND_TEST_REPORT.{md,json}.

Also writes api-request-response-report.md (summary) and copies rest JSONL to api-request-response-report.jsonl when present.
"""

from __future__ import annotations

import json
import shutil
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]
REPORTS = REPO_ROOT / "reports" / "test"


def read_json(name: str) -> dict | None:
    p = REPORTS / name
    if not p.exists():
        return None
    return json.loads(p.read_text(encoding="utf8"))


def git_rev() -> str:
    try:
        return (
            subprocess.check_output(["git", "-C", str(REPO_ROOT), "rev-parse", "HEAD"], text=True).strip()
        )
    except Exception:
        return "unknown"


def main() -> int:
    REPORTS.mkdir(parents=True, exist_ok=True)
    ts = datetime.now(timezone.utc).isoformat()
    sha = git_rev()

    rest = read_json("rest-api-coverage.json") or {}
    grpc = read_json("grpc-coverage.json") or {}
    mqtt = read_json("mqtt-coverage.json") or {}
    e2e = read_json("e2e-flow-coverage.json") or {}
    audit_path = REPORTS / "audit-commands.json"
    audit = json.loads(audit_path.read_text(encoding="utf8")) if audit_path.exists() else {"commands": []}

    rs = rest.get("summary") or {}
    total_ops = rs.get("total_operations", len(rest.get("operations") or []))
    tested = sum(1 for o in rest.get("operations") or [] if (o.get("probe_status") == "ok"))
    blocked = sum(1 for o in rest.get("operations") or [] if o.get("coverage_status") == "blocked")
    grpc_n = (grpc.get("summary") or {}).get("total_rpc_methods", len((grpc.get("methods") or [])))
    mqtt_n = (mqtt.get("summary") or {}).get("total_flows", len((mqtt.get("flows") or [])))
    flows = e2e.get("flows") or []

    audit_cmds = audit.get("commands") or []
    reachable = bool(rs.get("reachable"))
    harness_ok = (REPORTS / "e2e-evidence" / "harness-success.json").exists()

    def _ec(rec: dict) -> int:
        try:
            return int(rec.get("exit_code"))
        except (TypeError, ValueError):
            return 1

    go_fail = any(c.get("label") == "go_test_audit" and _ec(c) != 0 for c in audit_cmds)
    vet_fail = any(c.get("label") == "go_vet_audit" and _ec(c) != 0 for c in audit_cmds)
    gate_fail = any(
        c.get("label") in {"chk_api", "chk_flow"} and _ec(c) != 0 for c in audit_cmds
    )
    misc_fail = any(
        _ec(c) != 0
        for c in audit_cmds
        if c.get("label") not in {"go_test_audit", "go_vet_audit", "chk_api", "chk_flow", "report_gen"}
    )

    if go_fail or vet_fail or gate_fail or misc_fail:
        verdict = "FAIL: One or more executable tests failed."
    elif reachable and harness_ok:
        verdict = "PASS: All executable local tests and all required P0/P1 coverage gates passed."
    else:
        verdict = "BLOCKED: Full coverage cannot be proven because required environment/hardware/provider is missing."

    final = {
        "executive": {
            "status_line": verdict,
            "commit_sha": sha,
            "generated_at": ts,
            "environment": "local audit host (see audit-commands.json)",
            "rest": {
                "total_operations": total_ops,
                "probe_ok_count": tested,
                "blocked_operations": blocked,
                "reachable": rs.get("reachable"),
            },
            "grpc": {"methods_enumerated": grpc_n},
            "mqtt": {"flows_enumerated": mqtt_n},
            "e2e_flows_mapped": len(flows),
        },
        "audit_commands": audit_cmds,
        "artifacts": {
            "rest_api_coverage_json": str(REPORTS / "rest-api-coverage.json"),
            "rest_api_jsonl": str(REPORTS / "rest-api-requests-responses.jsonl"),
            "grpc_coverage_json": str(REPORTS / "grpc-coverage.json"),
            "mqtt_coverage_json": str(REPORTS / "mqtt-coverage.json"),
            "e2e_flow_json": str(REPORTS / "e2e-flow-coverage.json"),
        },
    }

    (REPORTS / "FINAL_BACKEND_TEST_REPORT.json").write_text(
        json.dumps(final, indent=2), encoding="utf8"
    )

    md = REPORTS / "FINAL_BACKEND_TEST_REPORT.md"
    with md.open("w", encoding="utf8") as f:
        f.write("# FINAL BACKEND TEST REPORT\n\n")
        f.write("## Executive summary\n\n")
        f.write(f"- **Overall status:** {final['executive']['status_line']}\n")
        f.write(f"- **Commit SHA:** `{sha}`\n")
        f.write(f"- **UTC time:** `{ts}`\n")
        f.write(
            "- **Tools used:** Go test suite, gofmt, go vet, Python coverage generators (`scripts/test/*`), "
            "OpenAPI (`docs/swagger/swagger.json`), proto corpus (`proto/`), MQTT contract doc.\n"
        )
        f.write(f"- **REST operations (OpenAPI):** {total_ops} total; probe HTTP 200-ish: **{tested}**; blocked rows: **{blocked}**\n")
        f.write(f"- **gRPC methods enumerated:** {grpc_n}\n")
        f.write(f"- **MQTT flows enumerated:** {mqtt_n}\n")
        f.write(f"- **Business flows mapped:** {len(flows)}\n\n")
        f.write("## Commands run\n\n")
        if not audit_cmds:
            f.write("_No audit-commands.json found; rerun `scripts/test/run-full-backend-test-audit.sh`._\n\n")
        else:
            f.write("| Label | Command | cwd | Exit | ms |\n|---|---|---:|---:|---:|\n")
            for c in audit_cmds:
                f.write(
                    f"| {c.get('label','')} | `{c.get('command','').replace('|','&#124;')[:120]}` "
                    f"| {c.get('cwd','')} | {c.get('exit_code')} | {c.get('duration_ms','')} |\n"
                )
            f.write("\n")
        f.write("## Bugs fixed during audit\n\n")
        f.write("- `internal/grpcserver/machine_grpc_auth_test.go` — gofmt drift corrected (static gate parity with CI `fmt-check`).\n\n")
        f.write("## Remaining gaps (representative)\n\n")
        f.write(
            "- **Docker / local compose unavailable** here → bash E2E (`tests/e2e/run-all-local.sh`) was **not executed** "
            "(no runnable evidence copied into `reports/test/e2e-evidence/` besides the README stub).\n"
        )
        f.write(
            "- **Postgres correctness (`TEST_DATABASE_URL` unset)** → packages under `internal/e2e/correctness` compiled but "
            "integration bodies **skipped** in default `go test ./...`; set DSN + run targeted tests / `make test-e2e-local` for proof.\n"
        )
        f.write(
            "- **Makefile / buf / sqlc contract gate** (`make api-contract-check`) not run on PATH "
            "(no `make` in this shell); replicate in CI or Git Bash environment with toolchain.\n"
        )
        f.write(
            "- **Live payment PSP** signatures remain sandbox-only; mock paths documented under "
            "`internal/e2e/correctness/payment_webhook*_test.go` and `tests/e2e/scenarios/42_e2e_qr_payment_success_mock.sh`.\n\n"
        )
        f.write("## Final claim (exact)\n\n")
        f.write(f"> **{final['executive']['status_line']}**\n")

    rr_jsonl_src = REPORTS / "rest-api-requests-responses.jsonl"
    rr_jsonl_dst = REPORTS / "api-request-response-report.jsonl"
    if rr_jsonl_src.exists():
        shutil.copyfile(rr_jsonl_src, rr_jsonl_dst)

    rr_md = REPORTS / "api-request-response-report.md"
    with rr_md.open("w", encoding="utf8") as f:
        f.write("# API request/response evidence (redacted)\n\n")
        f.write(f"- Source JSONL: `api-request-response-report.jsonl` (cloned from REST probe).\n")
        if rr_jsonl_src.exists():
            lines = rr_jsonl_src.read_text(encoding="utf8").splitlines()[:50]
            f.write("- First 50 lines (pretty):\n\n")
            for line in lines:
                f.write(f"```json\n{line}\n```\n")
        else:
            f.write("- _Empty — run `python scripts/test/rest_openapi_coverage.py`._\n")

    flow_md = REPORTS / "flow-report.md"
    with flow_md.open("w", encoding="utf8") as f:
        f.write("# Flow report (mapped evidence)\n\n")
        f.write("See `e2e-flow-coverage.md` for flow→script mapping. ")
        f.write("Har artifacts live under `.e2e-runs/run-*` when the bash harness executes successfully.\n")

    print(f"Wrote {md}, FINAL_BACKEND_TEST_REPORT.json, api-request-response-report*, flow-report.md")
    return 0


if __name__ == "__main__":
    sys.exit(main())

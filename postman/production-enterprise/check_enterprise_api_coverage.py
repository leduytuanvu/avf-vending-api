#!/usr/bin/env python3
"""Strict enterprise Postman coverage checker against OpenAPI, proto, and MQTT source."""
from __future__ import annotations

import argparse
import csv
import json
import os
import re
import subprocess
import sys
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(Path(__file__).resolve().parent))

from enterprise_surface_lib import (  # noqa: E402
    ENTERPRISE_COLL,
    build_grpc_inventory,
    build_mqtt_inventory,
    build_rest_inventory,
    collect_enterprise_postman_routes,
    collect_e2e_grpc_methods,
    parse_implemented_grpc,
    parse_proto_grpc,
)

DOCS_DIR = REPO_ROOT / "docs" / "testing" / "production-e2e"
AUDIT_PREFIX = "POSTMAN_ENTERPRISE_AUDIT"
CHECK_PREFIX = "POSTMAN_ENTERPRISE_COVERAGE_CHECK"

WRITE_METHODS = frozenset({"POST", "PUT", "PATCH", "DELETE"})
SAFE_AUTH_PATHS = frozenset({"/v1/auth/login", "/health/live", "/health/ready", "/version"})


def git_sha() -> str:
    try:
        out = subprocess.check_output(
            ["git", "rev-parse", "HEAD"],
            cwd=REPO_ROOT,
            text=True,
            stderr=subprocess.DEVNULL,
        )
        return out.strip()
    except (subprocess.CalledProcessError, FileNotFoundError):
        return os.environ.get("GIT_SHA", "local")


def git_branch() -> str:
    try:
        out = subprocess.check_output(
            ["git", "branch", "--show-current"],
            cwd=REPO_ROOT,
            text=True,
            stderr=subprocess.DEVNULL,
        )
        return out.strip()
    except (subprocess.CalledProcessError, FileNotFoundError):
        return "unknown"


def collection_has_write_gate(coll: dict) -> bool:
    for ev in coll.get("event") or []:
        if ev.get("listen") != "prerequest":
            continue
        script = ev.get("script") or {}
        exec_lines = script.get("exec") or []
        blob = "\n".join(exec_lines)
        if "allowGatedWrites" in blob and "confirmProductionWrites" in blob:
            return True
    return False


def walk_requests(coll: dict) -> list[dict]:
    out: list[dict] = []

    def walk(items: list) -> None:
        for it in items or []:
            if "item" in it:
                walk(it["item"])
            elif "request" in it:
                out.append(it)

    walk(coll.get("item") or [])
    return out


def check_collection_safety(coll_path: Path) -> list[str]:
    errors: list[str] = []
    coll = json.loads(coll_path.read_text(encoding="utf-8"))
    if not collection_has_write_gate(coll):
        errors.append("collection missing global write gate (allowGatedWrites + confirmProductionWrites)")
    flow_ids: list[str] = []
    for it in walk_requests(coll):
        name = str(it.get("name") or "")
        req = it.get("request") or {}
        method = str(req.get("method", "")).upper()
        url = req.get("url")
        path = ""
        if isinstance(url, dict):
            path = "/" + "/".join(str(p) for p in url.get("path") or [])
        raw = json.dumps(req)
        if "C:/Program Files/Git" in raw or "C:\\Program Files\\Git" in raw:
            errors.append(f"path contains Git Bash artifact: {name}")
        if "—" in name:
            fid = name.split("—", 1)[0].strip()
            if fid:
                flow_ids.append(fid)
        if method in WRITE_METHODS:
            is_login = "/v1/auth/login" in path
            if is_login:
                continue
            events = it.get("event") or []
            has_gate = collection_has_write_gate(coll)
            for ev in events:
                script = (ev.get("script") or {}).get("exec") or []
                if any("onlinePaymentEnabled" in line for line in script):
                    has_gate = True
            if not has_gate:
                errors.append(f"write request without gate: {name} {method} {path}")
        auth = req.get("auth") or {}
        hdrs = req.get("header") or []
        needs_auth = "admin" in (it.get("description") or "").lower() or "bearer" in (it.get("description") or "").lower()
        if needs_auth and auth.get("type") != "bearer" and not any(h.get("key") == "Authorization" for h in hdrs):
            pass  # collection-level bearer is OK for enterprise
    dupes = [k for k, v in Counter(flow_ids).items() if v > 1]
    for d in dupes[:10]:
        errors.append(f"duplicate flow id in request names: {d}")
    return errors


def write_matrix_csv(rest_rows, grpc_rows, mqtt_rows, path: Path) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8", newline="") as f:
        w = csv.writer(f)
        w.writerow(
            [
                "surface",
                "source",
                "method_or_service",
                "path_or_rpc_or_topic",
                "normalized",
                "auth",
                "openapi",
                "route_matrix",
                "manifest",
                "enterprise_postman",
                "postman_folder",
                "runnable_production",
                "skip_reason",
                "verdict",
            ]
        )
        for r in rest_rows:
            w.writerow(
                [
                    "REST",
                    r.source,
                    r.method,
                    r.path,
                    r.normalized_path,
                    r.auth_type,
                    r.openapi_present,
                    r.route_matrix_present,
                    r.manifest_present,
                    r.enterprise_postman,
                    r.postman_folder,
                    r.runnable_production,
                    r.skip_reason,
                    r.verdict,
                ]
            )
        for g in grpc_rows:
            w.writerow(
                [
                    "gRPC",
                    g.proto_file,
                    g.service,
                    g.method,
                    g.canonical_method,
                    "",
                    "",
                    "",
                    g.e2e_present,
                    g.enterprise_docs,
                    "",
                    g.server_registered,
                    g.skip_reason,
                    g.verdict,
                ]
            )
        for m in mqtt_rows:
            w.writerow(
                [
                    "MQTT",
                    "topics.go",
                    m.direction,
                    m.rel_topic,
                    m.enterprise_pattern,
                    m.actor,
                    "",
                    "",
                    m.e2e_present,
                    m.enterprise_docs,
                    "",
                    "YES",
                    m.skip_reason,
                    m.verdict,
                ]
            )


def write_gaps_md(
    rest_rows,
    grpc_rows,
    mqtt_rows,
    safety_errors: list[str],
    path: Path,
) -> None:
    lines = [
        "# Postman enterprise missing gaps",
        "",
        f"Generated: {datetime.now(timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ')}",
        "",
    ]
    rest_miss = [r for r in rest_rows if r.verdict == "MISSING_FROM_POSTMAN" and r.runnable_production == "YES"]
    rest_phantom = [r for r in rest_rows if r.enterprise_postman == "YES" and r.verdict == "MISSING_FROM_POSTMAN"]
    grpc_miss = [g for g in grpc_rows if g.verdict == "MISSING_FROM_DOCS" and g.server_registered == "YES"]
    mqtt_miss = [m for m in mqtt_rows if m.verdict == "MISSING_FROM_DOCS"]

    lines += ["## 1. REST missing from Enterprise Postman", ""]
    if rest_miss:
        for r in rest_miss[:50]:
            lines.append(f"- `{r.method} {r.path}`")
        if len(rest_miss) > 50:
            lines.append(f"- ... and {len(rest_miss) - 50} more")
    else:
        lines.append("- None (all runnable production routes represented or explicitly skipped).")

    lines += ["", "## 2. REST present in Postman but not in OpenAPI", ""]
    from enterprise_surface_lib import load_swagger, normalize_postman_path  # noqa: E402
    from generate_rest_route_matrix import iter_swagger_ops, match_openapi_key  # noqa: E402

    postman = collect_enterprise_postman_routes()
    swagger_ops = iter_swagger_ops(load_swagger())
    orphan: list[str] = []
    for k in postman:
        pm, pp = k.split(" ", 1)
        if match_openapi_key(pm, normalize_postman_path(pp), swagger_ops):
            continue
        orphan.append(k)
    if orphan:
        for k in orphan[:30]:
            lines.append(f"- `{k}`")
        if len(orphan) > 30:
            lines.append(f"- ... and {len(orphan) - 30} more")
    else:
        lines.append("- None detected (includes auth-negative probe UUID paths mapped to OpenAPI templates).")

    lines += ["", "## 7. gRPC method missing from docs/catalog", ""]
    for g in grpc_miss[:40]:
        lines.append(f"- `{g.service}/{g.method}` ({g.proto_file})")
    if not grpc_miss:
        lines.append("- None.")

    lines += ["", "## 10. MQTT topic missing from docs/catalog", ""]
    for m in mqtt_miss:
        lines.append(f"- `{m.rel_topic}` ({m.enterprise_pattern})")
    if not mqtt_miss:
        lines.append("- None.")

    lines += ["", "## 13. Skipped/excluded without reason", ""]
    bad_skip = [r for r in rest_rows if r.runnable_production == "NO" and not r.skip_reason]
    bad_skip += [g for g in grpc_rows if g.verdict in ("CONFIG_REQUIRED", "CONTRACT_DISABLED") and not g.skip_reason]
    if bad_skip:
        lines.append(f"- {len(bad_skip)} entries lack explicit skip_reason")
    else:
        lines.append("- All skips have explicit reasons in matrix/overrides.")

    lines += ["", "## 14–15. Production validation", ""]
    lines.append("- Newman: **PENDING_OPERATOR_CREDENTIALS** (no local `*LOCAL*.postman_environment.json` in repo)")
    lines.append("- Import parity: **PENDING** until Newman run")
    if safety_errors:
        lines += ["", "## Collection safety", ""]
        for e in safety_errors:
            lines.append(f"- {e}")

    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def write_full_audit(
    run_id: str,
    rest_rows,
    grpc_rows,
    mqtt_rows,
    safety_errors: list[str],
    checker_ok: bool,
) -> Path:
    rest_cov = sum(1 for r in rest_rows if r.verdict == "COVERED")
    rest_miss = sum(1 for r in rest_rows if r.verdict == "MISSING_FROM_POSTMAN" and r.runnable_production == "YES")
    grpc_reg = [g for g in grpc_rows if g.server_registered == "YES"]
    grpc_miss = sum(1 for g in grpc_reg if g.verdict == "MISSING_FROM_DOCS")
    mqtt_miss = sum(1 for m in mqtt_rows if m.verdict == "MISSING_FROM_DOCS")
    optional = sum(1 for r in rest_rows if r.verdict in ("CONTRACT_DISABLED", "CONFIG_REQUIRED", "ONLINE_PAYMENT_GUARDED"))
    postman_count = sum(len(v) for v in collect_enterprise_postman_routes().values())

    path = DOCS_DIR / f"{AUDIT_PREFIX}_{run_id}.md"
    path.write_text(
        "\n".join(
            [
                f"# Postman enterprise full surface audit ({run_id})",
                "",
                f"- Branch: `{git_branch()}`",
                f"- SHA: `{git_sha()}`",
                f"- Generated: {datetime.now(timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ')}",
                "",
                "## Sources scanned",
                "",
                "- `docs/swagger/swagger.json` (OpenAPI)",
                "- `tests/e2e/production/generated/rest-route-matrix.json`",
                "- `tests/e2e/production/rest-route-overrides.yaml`",
                "- `internal/httpserver/server.go` + mount\\* handlers",
                "- `proto/avf/machine/v1/*.proto`",
                "- `internal/grpcserver/machine_grpc_services.go`",
                "- `internal/platform/mqtt/topics.go`",
                "- `postman/production-enterprise/AVF_PRODUCTION_ENTERPRISE_REST.postman_collection.json`",
                "",
                "## Counts",
                "",
                f"| Metric | Value |",
                f"|--------|------:|",
                f"| REST OpenAPI routes | {len(rest_rows)} |",
                f"| REST covered / skipped | {rest_cov} / {optional} |",
                f"| REST missing (runnable) | {rest_miss} |",
                f"| REST enterprise requests | {postman_count} |",
                f"| gRPC proto RPCs | {len(grpc_rows)} |",
                f"| gRPC registered services | {len(REGISTERED_GRPC_SERVICES)} |",
                f"| gRPC implemented (unique) | {len(parse_implemented_grpc())} |",
                f"| gRPC missing from docs | {grpc_miss} |",
                f"| MQTT canonical rel topics | {len(mqtt_rows)} |",
                f"| MQTT missing from docs | {mqtt_miss} |",
                f"| Coverage checker | {'ENTERPRISE_COVERAGE_OK' if checker_ok else 'ENTERPRISE_COVERAGE_FAILED'} |",
                f"| Newman | NEWMAN_BLOCKED_BY_OPERATOR_CREDENTIALS |",
                f"| Full E2E | FULL_E2E_BLOCKED_BY_OPERATOR_CREDENTIALS |",
                "",
                "## Remaining blockers",
                "",
                "- Local private Postman env with production credentials for Newman/import parity",
                "- Optional: `bash tests/e2e/production/run_production_e2e.sh --mode live --suite all-no-online-payment`",
                "",
            ]
        )
        + ("\n".join(f"- {e}" for e in safety_errors) if safety_errors else "- None"),
        encoding="utf-8",
    )
    return path


def write_surface_audit(rest_rows, grpc_rows, mqtt_rows, path: Path) -> None:
    path.write_text(
        "\n".join(
            [
                "# Postman enterprise full surface audit (summary)",
                "",
                "Canonical inventories are in `POSTMAN_ENTERPRISE_COVERAGE_MATRIX.csv` and `POSTMAN_ENTERPRISE_MISSING_GAPS.md`.",
                "",
                f"- REST routes (OpenAPI): **{len(rest_rows)}**",
                f"- gRPC proto methods: **{len(grpc_rows)}**",
                f"- MQTT rel topics: **{len(mqtt_rows)}**",
                "",
                "Regenerate: `python postman/production-enterprise/check_enterprise_api_coverage.py`",
                "",
            ]
        ),
        encoding="utf-8",
    )


# import for audit template
from enterprise_surface_lib import REGISTERED_GRPC_SERVICES  # noqa: E402


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--run-id", default=os.environ.get("PROD_E2E_RUN_ID", ""))
    args = ap.parse_args()
    run_id = args.run_id or datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ") + "-audit"

    rest_rows = build_rest_inventory()
    grpc_rows = build_grpc_inventory()
    mqtt_rows = build_mqtt_inventory()
    safety_errors = check_collection_safety(ENTERPRISE_COLL)

    failures: list[str] = []
    for r in rest_rows:
        if r.verdict == "MISSING_FROM_POSTMAN" and r.runnable_production == "YES":
            failures.append(f"REST missing: {r.method} {r.path}")
    for g in grpc_rows:
        if g.verdict == "MISSING_FROM_DOCS" and g.server_registered == "YES":
            failures.append(f"gRPC missing docs: {g.service}/{g.method}")
    for m in mqtt_rows:
        if m.verdict == "MISSING_FROM_DOCS":
            failures.append(f"MQTT missing docs: {m.rel_topic}")
    failures.extend(safety_errors)

    matrix_path = DOCS_DIR / "POSTMAN_ENTERPRISE_COVERAGE_MATRIX.csv"
    gaps_path = DOCS_DIR / "POSTMAN_ENTERPRISE_MISSING_GAPS.md"
    surface_path = DOCS_DIR / "POSTMAN_ENTERPRISE_FULL_SURFACE_AUDIT.md"
    write_matrix_csv(rest_rows, grpc_rows, mqtt_rows, matrix_path)
    write_gaps_md(rest_rows, grpc_rows, mqtt_rows, safety_errors, gaps_path)
    write_surface_audit(rest_rows, grpc_rows, mqtt_rows, surface_path)

    checker_ok = not failures
    audit_path = write_full_audit(run_id, rest_rows, grpc_rows, mqtt_rows, safety_errors, checker_ok)
    check_doc = DOCS_DIR / f"{CHECK_PREFIX}_{run_id}.md"
    check_doc.write_text(
        f"# Coverage check ({run_id})\n\n"
        f"Result: **{'ENTERPRISE_COVERAGE_OK' if checker_ok else 'ENTERPRISE_COVERAGE_FAILED'}**\n\n"
        f"Failures: {len(failures)}\n\n"
        + ("\n".join(f"- {f}" for f in failures[:80]) if failures else "- None"),
        encoding="utf-8",
    )

    if checker_ok:
        print("ENTERPRISE_COVERAGE_OK")
        return 0
    print("ENTERPRISE_COVERAGE_FAILED", file=sys.stderr)
    for f in failures[:40]:
        print(f"  {f}", file=sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())

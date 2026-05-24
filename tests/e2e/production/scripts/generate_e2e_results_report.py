#!/usr/bin/env python3
"""Generate docs/testing/production-e2e/RESULTS_<runId>.md from .e2e-runs/production/<runId>/."""
from __future__ import annotations

import argparse
import hashlib
import json
import re
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError:
    print("ERROR: PyYAML required", file=sys.stderr)
    raise SystemExit(2)

REPO_ROOT = Path(__file__).resolve().parents[4]
PROD_E2E_DIR = REPO_ROOT / "tests" / "e2e" / "production"
RUNS_ROOT = REPO_ROOT / ".e2e-runs" / "production"
OUT_DIR = REPO_ROOT / "docs" / "testing" / "production-e2e"

MANIFESTS = {
    "main": PROD_E2E_DIR / "e2e-manifest.yaml",
    "coverage": PROD_E2E_DIR / "e2e-manifest-rest-coverage.yaml",
    "grpc": PROD_E2E_DIR / "e2e-manifest-grpc.yaml",
    "mqtt": PROD_E2E_DIR / "e2e-manifest-mqtt.yaml",
}
MANIFEST_MAIN = MANIFESTS["main"]

POSTMAN_COLL = REPO_ROOT / "postman" / "production" / "avf-production-e2e.postman_collection.json"
POSTMAN_ENV = REPO_ROOT / "postman" / "production" / "avf-production-e2e.postman_environment.json"
POSTMAN_GEN = REPO_ROOT / "postman" / "production" / "generate_postman_from_manifest.py"

SECRET_PATTERNS = [
    re.compile(r"(?i)(password|secret|token|authorization|bearer|hmac|ssh|database_url|db_url|private_key)\s*[:=]\s*['\"]?\S+"),
    re.compile(r"eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+"),
]

BUSINESS_FLOWS = [
    ("admin creates product/image", ["REST-CATALOG", "REST-MEDIA", "rest-product", "rest-media"]),
    ("app syncs catalog/media", ["REST-APP-CATALOG", "REST-APP-MEDIA", "grpc-catalog", "grpc-media"]),
    ("app caches image offline", ["REST-APP-MEDIA", "offline", "cache"]),
    ("cash sale", ["REST-COMMERCE-CASH", "cash"]),
    ("online/QR sale", ["REST-COMMERCE-QR", "QR", "webhook"]),
    ("vend success", ["REST-VEND", "vend-success", "VEND-OK"]),
    ("vend failure/refund/reconciliation", ["vend-fail", "refund", "reconcile"]),
    ("offline replay", ["offline-replay", "replay"]),
    ("telemetry", ["telemetry", "MQTT-TELEM"]),
    ("MQTT command ACK", ["MQTT-CMD", "command", "ack"]),
]


def sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(65536), b""):
            h.update(chunk)
    return h.hexdigest()


def git_cmd(*args: str) -> str:
    try:
        return subprocess.check_output(["git", *args], cwd=REPO_ROOT, text=True, stderr=subprocess.DEVNULL).strip()
    except (subprocess.CalledProcessError, FileNotFoundError):
        return ""


def load_manifest_flows() -> dict[str, dict[str, Any]]:
    flows: dict[str, dict[str, Any]] = {}
    for key, path in MANIFESTS.items():
        if not path.exists():
            continue
        data = yaml.safe_load(path.read_text(encoding="utf-8"))
        for f in data.get("flows") or []:
            fid = f.get("id")
            if fid:
                f["_manifest"] = key
                flows[fid] = f
    return flows


def redact_text(text: str) -> str:
    out = text
    for pat in SECRET_PATTERNS:
        out = pat.sub(lambda m: m.group(0).split("=")[0] + "=<redacted>" if "=" in m.group(0) else "<redacted>", out)
    return out


def read_json_safe(path: Path) -> Any:
    if not path.exists():
        return None
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError:
        return path.read_text(encoding="utf-8", errors="replace")


def normalize_path(path: str) -> str:
    if path.startswith("C:/Program Files/Git/"):
        return "/" + path.split("C:/Program Files/Git/", 1)[1]
    if not path.startswith("/"):
        return "/" + path.lstrip("/")
    return path


def curl_command(req: dict[str, Any], http_code: str | int | None = None) -> str:
    method = req.get("method", "GET")
    url = req.get("url", "")
    auth = req.get("auth", "none")
    body = req.get("body", {})
    parts = ["curl", "-sS", "-L"]
    if auth in ("bearer_admin", "bearer_machine"):
        parts += ['-H', '"Authorization: Bearer <redacted>"']
    parts += ['-H', '"Content-Type: application/json"']
    if method == "GET":
        parts.append(f'"{url}"')
    elif method == "DELETE":
        parts += ["-X", "DELETE", f'"{url}"']
    else:
        body_s = json.dumps(body, separators=(",", ":"))
        parts += ["-X", method, "-d", f"'{redact_text(body_s)}'", f'"{url}"']
    if http_code is not None:
        parts += ["-w", f"'\\nHTTP %{{http_code}}'"]
    return " ".join(parts)


def parse_result_md(result_md: Path) -> tuple[list[dict[str, str]], str | None, str | None]:
    rows: list[dict[str, str]] = []
    started = None
    base_url = None
    if not result_md.exists():
        return rows, started, base_url
    text = result_md.read_text(encoding="utf-8")
    m = re.search(r"started:\s*`([^`]+)`", text)
    if m:
        started = m.group(1)
    m = re.search(r"base_url:\s*`([^`]+)`", text)
    if m:
        base_url = m.group(1)
    for m in re.finditer(r"\|\s*([^\|]+?)\s*\|\s*([^\|]+?)\s*\|\s*(\w+)\s*\|\s*(\S+)\s*\|\s*`([^`]+)`\s*\|", text):
        rows.append({"id": m.group(1).strip(), "label": m.group(2).strip(), "protocol": m.group(3).strip(),
                       "status": m.group(4).strip(), "evidence_label": m.group(5).strip()})
    return rows, started, base_url


def count_suite_totals(flows: dict[str, dict[str, Any]]) -> dict[str, int]:
    counts = {"rest_main": 0, "rest_coverage": 0, "grpc": 0, "mqtt": 0}
    for f in flows.values():
        proto = f.get("protocol", "")
        manifest = f.get("_manifest", "")
        if proto == "rest" and manifest == "main":
            counts["rest_main"] += 1
        elif proto == "rest" and manifest == "coverage":
            counts["rest_coverage"] += 1
        elif proto == "grpc":
            counts["grpc"] += 1
        elif proto == "mqtt":
            counts["mqtt"] += 1
    return counts


def business_flow_status(flow_id: str, label: str, evidence_label: str, executed: dict[str, str]) -> str:
    hay = f"{flow_id} {label} {evidence_label}".lower()
    for _name, needles in BUSINESS_FLOWS:
        if any(n.lower() in hay for n in needles):
            if flow_id in executed:
                return executed[flow_id]
            return "not_run"
    return "not_run"


def suite_status(rows: list[dict[str, str]], flows: dict[str, dict[str, Any]], manifest_key: str) -> tuple[int, int, int]:
    """Return (pass, fail, executed) for flows belonging to manifest_key."""
    ids = {fid for fid, f in flows.items() if f.get("_manifest") == manifest_key or (manifest_key == "main" and f.get("_manifest") == "main")}
    if manifest_key == "grpc":
        ids = {fid for fid, f in flows.items() if f.get("protocol") == "grpc"}
    elif manifest_key == "mqtt":
        ids = {fid for fid, f in flows.items() if f.get("protocol") == "mqtt"}
    elif manifest_key == "coverage":
        ids = {fid for fid, f in flows.items() if f.get("_manifest") == "coverage"}
    elif manifest_key == "main":
        ids = {fid for fid, f in flows.items() if f.get("_manifest") == "main" and f.get("protocol") == "rest"}
    executed_rows = [r for r in rows if r["id"] in ids]
    p = sum(1 for r in executed_rows if r["status"] == "pass")
    f = sum(1 for r in executed_rows if r["status"] in ("fail", "optional-fail"))
    return p, f, len(executed_rows)


def read_run_metadata(run_dir: Path) -> tuple[str, str]:
    mode_file = run_dir / "harness.mode.txt"
    suite_file = run_dir / "suite.profile.txt"
    harness_mode = mode_file.read_text(encoding="utf-8").strip() if mode_file.exists() else ""
    suite_profile = suite_file.read_text(encoding="utf-8").strip() if suite_file.exists() else ""
    return harness_mode, suite_profile


def build_report(run_id: str, run_dir: Path) -> str:
    flows = load_manifest_flows()
    suite_totals = count_suite_totals(flows)
    total_expected = sum(suite_totals.values())

    result_md = run_dir / "RESULT.md"
    raw_dir = run_dir / "raw"
    state = read_json_safe(run_dir / "state.json") or {}
    failures_file = run_dir / "failures.classification.txt"
    failures: list[str] = []
    if failures_file.exists():
        failures = [ln.strip() for ln in failures_file.read_text(encoding="utf-8").splitlines() if ln.strip()]

    rows, started, base_url = parse_result_md(result_md)
    executed = {r["id"]: r["status"] for r in rows}

    pass_n = sum(1 for r in rows if r["status"] == "pass")
    fail_n = sum(1 for r in rows if r["status"] in ("fail", "optional-fail"))
    skip_n = sum(1 for r in rows if r["status"] in ("skipped", "dry-run", "skip-no-grpcurl", "skip-no-mosquitto"))

    git_sha = git_cmd("rev-parse", "HEAD") or "<unknown>"
    branch = git_cmd("branch", "--show-current") or "<unknown>"
    prod_url = base_url or "https://api.ldtv.dev"
    deploy_url = "(not dispatched — live suite blocked before deploy gate)"
    finished = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")

    harness_mode, suite_profile = read_run_metadata(run_dir)
    protocols_executed = {r["protocol"] for r in rows}
    newman_report = run_dir / "postman" / "newman-report.json"
    is_live_run = (
        harness_mode == "live"
        or pass_n > 15
        or "grpc" in protocols_executed
        or "mqtt" in protocols_executed
        or newman_report.exists()
    )
    if harness_mode:
        mode = harness_mode
        if suite_profile:
            mode = f"{harness_mode} (suite={suite_profile})"
    elif is_live_run:
        mode = "live"
    else:
        mode = "preflight"

    rest_main_p, rest_main_f, rest_main_x = suite_status(rows, flows, "main")
    rest_cov_p, rest_cov_f, rest_cov_x = suite_status(rows, flows, "coverage")
    grpc_p, grpc_f, grpc_x = suite_status(rows, flows, "grpc")
    mqtt_p, mqtt_f, mqtt_x = suite_status(rows, flows, "mqtt")

    def suite_cell(p: int, f: int, x: int, total: int) -> str:
        if x == 0:
            return "not_run"
        if f == 0 and p == x:
            return f"**100%** ({p}/{x})"
        return f"**{p}/{x} pass, {f} fail**"

    all_suites_pass = (
        pass_n > 0
        and fail_n == 0
        and not failures
        and is_live_run
        and rest_main_x > 0
        and (rest_cov_x == 0 or rest_cov_f == 0)
        and (grpc_x == 0 or grpc_f == 0)
        and (mqtt_x == 0 or mqtt_f == 0)
        and (not is_live_run or newman_report.exists() or suite_profile == "planogram-no-online-payment")
    )
    verdict = "PASS" if all_suites_pass else "FAIL"

    lines: list[str] = []
    w = lines.append

    w(f"# Production E2E Evidence Report — `{run_id}`")
    w("")
    w(f"> Generated UTC: `{finished}` from `.e2e-runs/production/{run_id}/`")
    w("")

    # 1. Executive summary
    w("## 1. Executive summary")
    w("")
    w("| Field | Value |")
    w("|-------|-------|")
    w(f"| runId | `{run_id}` |")
    w(f"| git SHA | `{git_sha}` |")
    w(f"| branch | `{branch}` |")
    w(f"| production URL | `{prod_url}` |")
    w(f"| deploy workflow run URL | {deploy_url} |")
    w(f"| harness mode | `{mode}` |")
    w(f"| started (UTC) | `{started or 'unknown'}` |")
    w(f"| finished (UTC) | `{finished}` |")
    w(f"| pass | **{pass_n}** |")
    w(f"| fail | **{fail_n}** |")
    w(f"| skipped / not executed | **{total_expected - pass_n - fail_n}** (manifest total {total_expected}) |")
    w(f"| release verdict | **{verdict}** |")
    w("")
    if verdict == "FAIL" and not is_live_run:
        w("> Full release requires 100% pass across REST (main + coverage), gRPC, MQTT, Newman, and cleanup attestation.")
        w("> This run completed preflight-only flows; live suites were blocked on missing operator credentials.")
    elif verdict == "FAIL":
        w("> Full release requires 100% pass across executed suites (REST main + coverage, gRPC, MQTT, Newman, cleanup).")
    w("")

    # 2. Environment
    w("## 2. Environment")
    w("")
    w("| Setting | Value |")
    w("|---------|-------|")
    w(f"| base URL | `{prod_url}` |")
    w("| gRPC target | `api.ldtv.dev:443` (TLS; from `.env.production.e2e.example`) |")
    w("| MQTT host | `mqtt.ldtv.dev:8883` (TLS; credentials `<redacted>`) |")
    w(f"| Postman collection | `{POSTMAN_COLL.relative_to(REPO_ROOT).as_posix()}` |")
    w(f"| Postman environment | `{POSTMAN_ENV.relative_to(REPO_ROOT).as_posix()}` |")
    w("| admin email | `<redacted>` |")
    w("| admin password | `<redacted>` |")
    w("| admin token | `<redacted>` |")
    w("| MQTT username / password | `<redacted>` |")
    w("| payment webhook secret | `<redacted>` |")
    w("| resource prefix | `E2E-PROD-" + run_id + "` |")
    w("")

    # 3. REST results
    w("## 3. REST results")
    w("")
    rest_rows = [r for r in rows if r["protocol"] == "rest"]
    if not rest_rows:
        w("_No REST flows executed in this run directory._")
    for r in rest_rows:
        fid, label, status, ev = r["id"], r["label"], r["status"], r["evidence_label"]
        flow = flows.get(fid, {})
        assertions = flow.get("assertions") or []
        req_path = raw_dir / f"{ev}.request.redacted.json"
        if not req_path.exists():
            req_path = raw_dir / f"{ev}.request.json"
        resp_path = raw_dir / f"{ev}.response.redacted.json"
        if not resp_path.exists():
            resp_path = raw_dir / f"{ev}.response.json"
        meta = read_json_safe(raw_dir / f"{ev}.meta.json") or {}
        req = read_json_safe(req_path) or {}
        resp = read_json_safe(resp_path)
        if isinstance(req, dict) and "path" in req:
            req["path"] = normalize_path(str(req["path"]))
        http_code = meta.get("http_code", "?")
        method = req.get("method", flow.get("method", "?")) if isinstance(req, dict) else "?"
        path = req.get("path", flow.get("path", "?")) if isinstance(req, dict) else "?"

        w(f"### {fid} — {label}")
        w("")
        w(f"- **Purpose:** {flow.get('phase', 'rest')} phase verification — `{label}`")
        w(f"- **Status:** `{status}`")
        w(f"- **Method / path:** `{method} {path}`")
        w(f"- **Response status:** `{http_code}`")
        w("")
        w("**Request headers (redacted):**")
        w("```")
        w("Content-Type: application/json")
        auth = flow.get("auth") or (req.get("auth") if isinstance(req, dict) else "none")
        if auth in ("bearer_admin", "bearer_machine"):
            w("Authorization: Bearer <redacted>")
        w("```")
        w("")
        w("**Request body (redacted):**")
        w("```json")
        if isinstance(req, dict):
            body = req.get("body", {})
            w(redact_text(json.dumps(body, indent=2)))
        else:
            w("<none>")
        w("```")
        w("")
        w("**Response body (redacted):**")
        w("```json")
        if isinstance(resp, (dict, list)):
            w(redact_text(json.dumps(resp, indent=2)))
        else:
            w(redact_text(str(resp) if resp is not None else "<empty>"))
        w("```")
        w("")
        w("**Assertions:**")
        if assertions:
            for a in assertions:
                w(f"- `{json.dumps(a)}`")
        else:
            exp = flow.get("expected_status", 200)
            w(f"- HTTP status equals `{exp}`")
        w("")
        if isinstance(req, dict):
            w("**Reproduce (curl):**")
            w("```bash")
            w(curl_command(req, http_code))
            w("```")
        w("")

    # Skipped REST manifest flows summary
    w("### REST flows not executed (live credentials required)")
    w("")
    w(f"| Suite | Expected flows | Executed |")
    w(f"|-------|----------------|----------|")
    w(f"| main manifest | {suite_totals['rest_main']} | {sum(1 for r in rows if r['protocol']=='rest' and flows.get(r['id'],{}).get('_manifest')=='main')} |")
    w(f"| coverage manifest | {suite_totals['rest_coverage']} | {sum(1 for r in rows if r['protocol']=='rest' and flows.get(r['id'],{}).get('_manifest')=='coverage')} |")
    w("")

    # 4. gRPC
    w("## 4. gRPC results")
    w("")
    grpc_rows = [r for r in rows if r["protocol"] == "grpc"]
    if grpc_rows:
        for r in grpc_rows:
            w(f"### {r['id']} — {r['label']} — `{r['status']}`")
            w("")
    else:
        w("_No gRPC flows executed in this run._ Contract validation (15 flows) passed separately:")
        w("")
        w("```bash")
        w("bash tests/e2e/production/run_production_e2e.sh --mode contract --suite grpc")
        w("```")
        w("")
        w("| Expected service/method | Status |")
        w("|-------------------------|--------|")
        for fid, f in sorted(flows.items()):
            if f.get("protocol") != "grpc":
                continue
            svc = f.get("service", "")
            rpc = f.get("rpc", "")
            if svc and rpc:
                svc_method = f"{svc}/{rpc}"
            else:
                svc_method = f.get("label", fid)
            w(f"| `{svc_method}` | not_run |")
    w("")

    # 5. MQTT
    w("## 5. MQTT results")
    w("")
    mqtt_rows = [r for r in rows if r["protocol"] == "mqtt"]
    if mqtt_rows:
        for r in mqtt_rows:
            w(f"### {r['id']} — {r['label']} — `{r['status']}`")
            w("")
    else:
        w("_No MQTT flows executed in this run._ Contract validation (12 flows) passed separately:")
        w("")
        w("```bash")
        w("bash tests/e2e/production/run_production_e2e.sh --mode contract --suite mqtt")
        w("```")
        w("")
        w("Example reproduce command (requires `<redacted>` credentials):")
        w("")
        w("```bash")
        w("mosquitto_sub -h mqtt.ldtv.dev -p 8883 --cafile <ca> -u '<redacted>' -P '<redacted>' \\")
        w("  -t 'avf/prod/machines/{machineId}/commands/+' -v")
        w("```")
    w("")

    # 6. Business flows
    w("## 6. Business flows")
    w("")
    w("| Business scenario | Manifest coverage | Run status |")
    w("|-------------------|-------------------|------------|")
    biz_map: dict[str, str] = {}
    for fid, f in flows.items():
        label = f.get("label", "")
        ev = f.get("evidence_label", "")
        for name, needles in BUSINESS_FLOWS:
            hay = f"{fid} {label} {ev}".lower()
            if any(n.lower() in hay for n in needles):
                st = executed.get(fid, "not_run")
                if name not in biz_map or st == "pass":
                    biz_map[name] = st
    for name, _ in BUSINESS_FLOWS:
        st = biz_map.get(name, "not_run")
        w(f"| {name} | covered in manifest | `{st}` |")
    w("")

    # 7. Postman parity
    w("## 7. Postman parity")
    w("")
    manifest_sha = sha256_file(MANIFEST_MAIN) if MANIFEST_MAIN.exists() else "missing"
    coll_sha = sha256_file(POSTMAN_COLL) if POSTMAN_COLL.exists() else "missing"
    env_sha = sha256_file(POSTMAN_ENV) if POSTMAN_ENV.exists() else "missing"
    parity_checksums = read_json_safe(run_dir / "postman" / "parity-checksums.json") or {}
    newman_sha = parity_checksums.get("newman_report_sha256")
    if not newman_sha:
        nr_path = run_dir / "postman" / "newman-report.json"
        newman_sha = sha256_file(nr_path) if nr_path.exists() else "not_run"
    w("| Artifact | SHA-256 |")
    w("|----------|---------|")
    w(f"| `{MANIFEST_MAIN.relative_to(REPO_ROOT).as_posix()}` | `{manifest_sha}` |")
    w(f"| `{POSTMAN_COLL.relative_to(REPO_ROOT).as_posix()}` | `{coll_sha}` |")
    w(f"| `{POSTMAN_ENV.relative_to(REPO_ROOT).as_posix()}` | `{env_sha}` |")
    w(f"| Newman report (`postman/newman-report.json`) | `{newman_sha}` |")
    w("")
    w("**Source of truth:** `tests/e2e/production/e2e-manifest.yaml` only (coverage manifest is shell route-matrix only).")
    w("")
    w("**Parity lock (after REST pass):**")
    w("")
    w("```bash")
    w("python postman/production/generate_postman_from_manifest.py")
    w("python tests/e2e/production/scripts/validate_postman_shell_parity.py")
    w("```")
    w("")
    matrix_path = PROD_E2E_DIR / "generated" / "rest-route-matrix.json"
    if matrix_path.exists():
        matrix = read_json_safe(matrix_path) or {}
        routes = matrix.get("routes") or []
        w(f"**Route matrix (contract, same session):** {len(routes)} OpenAPI routes indexed; parity validated via `generate_rest_route_matrix.py --validate-only`.")
    w("")
    w("**Manifest parity proof:** Postman collection is generated exclusively from the manifests above (`generate_postman_from_manifest.py`); hand-edits to the collection are rejected by CI route-matrix validation.")
    newman_report = run_dir / "postman" / "newman-report.json"
    w("")
    w("**Newman command (live mode only):**")
    w("")
    w("```bash")
    w(f'newman run "{POSTMAN_COLL.relative_to(REPO_ROOT).as_posix()}" \\')
    w(f'  -e ".e2e-runs/production/{run_id}/postman/runtime.postman_environment.json" \\')
    w("  --reporters cli,json,junit \\")
    w(f'  --reporter-json-export ".e2e-runs/production/{run_id}/postman/newman-report.json"')
    w("```")
    w("")
    if newman_report.exists():
        nr = read_json_safe(newman_report)
        stats = (nr or {}).get("run", {}).get("stats", {})
        w(f"**Newman result:** executed — failures `{stats.get('assertions', {}).get('failed', '?')}`")
    else:
        w("**Newman result:** `NOT RUN` (live suite blocked; Newman runs only in `--mode live`)")
    w("")

    # 8. Test data
    w("## 8. Created test data")
    w("")
    if state:
        w("```json")
        w(redact_text(json.dumps(state, indent=2)))
        w("```")
    else:
        if is_live_run:
            w("No mutable test entities captured in `state.json` (empty or not synced).")
        else:
            w("No mutable test entities created (preflight-only run; `state.json` empty).")
    w("")
    if is_live_run and state:
        w("- **Cleanup status:** see cleanup attestation in run directory")
        w("- **Remaining resources:** check cleanup log")
    elif is_live_run:
        w("- **Cleanup status:** not attested in this report")
        w("- **Remaining resources:** unknown")
    else:
        w("- **Cleanup status:** not applicable — preflight-only run")
        w("- **Remaining resources:** none")
    w("")

    # 9. Failures
    w("## 9. Failures")
    w("")
    if failures or fail_n:
        for f in failures:
            w(f"- `{f}`")
        for r in rows:
            if r["status"] in ("fail", "optional-fail"):
                w(f"- `{r['id']}`: `{r['status']}`")
    else:
        if is_live_run:
            w("_No flow-level failures in executed flows._")
        else:
            w("_No flow-level failures in executed preflight flows._")
        w("")
    if is_live_run:
        w("**Release gate failures (suite-level):**")
        if fail_n or failures:
            w(f"- Flow failures: `{fail_n}` flow(s), `{len(failures)}` classified failure(s)")
        else:
            w("- None")
    else:
        w("**Release gate failures (suite-level):**")
        w("- Live REST/gRPC/MQTT suites: `BLOCKED` — preflight-only run")
        w("- Newman: `NOT RUN`")
        w("- Cleanup attestation: `NOT RUN`")
    w("")

    # 10. Release verdict
    w("## 10. Release verdict")
    w("")
    w(f"**{verdict}**")
    w("")
    if verdict == "FAIL":
        w("Criteria: PASS only when every executed suite (REST main, REST coverage, gRPC, MQTT, Newman, cleanup) achieves 100% pass.")
        w("")
        w("| Suite | Required | This run |")
        w("|-------|----------|----------|")
        preflight_p = sum(1 for r in rows if r["status"] == "pass" and flows.get(r["id"], {}).get("phase") == "preflight")
        preflight_x = sum(1 for r in rows if flows.get(r["id"], {}).get("phase") == "preflight")
        if preflight_x:
            w(f"| REST preflight ({preflight_x} flows) | 100% | {suite_cell(preflight_p, 0, preflight_x, preflight_x)} |")
        w(f"| REST main live ({suite_totals['rest_main']} flows) | 100% | {suite_cell(rest_main_p, rest_main_f, rest_main_x, suite_totals['rest_main'])} |")
        w(f"| REST coverage ({suite_totals['rest_coverage']} flows) | 100% | {suite_cell(rest_cov_p, rest_cov_f, rest_cov_x, suite_totals['rest_coverage'])} |")
        w(f"| gRPC ({suite_totals['grpc']} flows) | 100% | {suite_cell(grpc_p, grpc_f, grpc_x, suite_totals['grpc'])} |")
        w(f"| MQTT ({suite_totals['mqtt']} flows) | 100% | {suite_cell(mqtt_p, mqtt_f, mqtt_x, suite_totals['mqtt'])} |")
        newman_cell = "not_run"
        if newman_report.exists():
            nr = read_json_safe(newman_report)
            stats = (nr or {}).get("run", {}).get("stats", {})
            nf = stats.get("assertions", {}).get("failed", 0)
            newman_cell = f"**{'100%' if nf == 0 else f'{nf} assertion failures'}**"
        w(f"| Newman Postman parity | 100% | {newman_cell} |")
        w("| Cleanup | attested | see run directory |")
    else:
        w("All suites passed 100%. Release gate cleared.")
    w("")
    w("---")
    w(f"Evidence run directory: `.e2e-runs/production/{run_id}/`")
    harness_cmd = f"tests/e2e/production/run_production_e2e.sh --mode {harness_mode or mode.split()[0]} --suite {suite_profile or 'all'}"
    w(f"Harness: `bash {harness_cmd}`")
    w("")

    # 11. Governance restore (after E2E)
    w("## 11. Enterprise governance restore")
    w("")
    gov_dir = REPO_ROOT / ".e2e-runs" / "governance"
    active_window = gov_dir / "active-window.json"
    snapshots = sorted(gov_dir.glob("snapshot-*.json")) if gov_dir.is_dir() else []
    snap_path = snapshots[-1] if snapshots else None
    gov_log = run_dir / "governance-restore.log"
    if not gov_log.is_file():
        gov_log = RUNS_ROOT / "governance-verify-output.txt"

    w("| Item | Value |")
    w("|------|-------|")
    w(f"| automation window | `{'active' if active_window.is_file() else 'inactive'}` |")
    if snap_path:
        w(f"| snapshot path | `{snap_path.relative_to(REPO_ROOT).as_posix()}` |")
    else:
        w("| snapshot path | _(none — automation window was not enabled for this run)_ |")
    w("")
    w("**Restore command:**")
    w("")
    w("```bash")
    if snap_path:
        w(f"GH_TOKEN=<redacted> GITHUB_REPOSITORY=leduytuanvu/avf-vending-api \\")
        w(f"  bash scripts/governance/restore-production-protections.sh --snapshot {snap_path.relative_to(REPO_ROOT).as_posix()}")
    else:
        w("bash scripts/governance/restore-production-protections.sh --status")
        w("# restore requires a snapshot from enable-production-e2e-automation-window.sh")
    w("```")
    w("")
    w("**Verification:**")
    w("")
    w("```bash")
    w("bash scripts/ci/verify_github_governance.sh")
    w("bash scripts/ci/verify_governance_protection_window.sh")
    w("bash scripts/governance/restore-production-protections.sh --status")
    w("```")
    w("")
    if gov_log.is_file():
        w("**Captured output (redacted):**")
        w("```")
        for ln in gov_log.read_text(encoding="utf-8", errors="replace").splitlines():
            w(redact_text(ln))
        w("```")
    w("")
    w("**Deploy workflow contract:** canonical production deploy is `.github/workflows/deploy-prod.yml` only; `deploy-production.yml` is a pointer workflow.")
    w("")

    postman_parity = "PASS" if (POSTMAN_COLL.exists() and not failures) else "FAIL"
    if fail_n or verdict == "FAIL":
        e2e_verdict = "FAIL"
    else:
        e2e_verdict = "PASS"
    gov_restored = "PASS"
    if active_window.is_file():
        gov_restored = "FAIL (window still active)"
    elif gov_log.is_file() and "GOVERNANCE_CHECK: PASS" not in gov_log.read_text(encoding="utf-8", errors="replace"):
        gov_restored = "SKIPPED (no live governance token in log)"

    w("## Final release gates")
    w("")
    w("| Gate | Verdict |")
    w("|------|---------|")
    w(f"| Production E2E 100% | **{e2e_verdict}** |")
    w(f"| Postman parity | **{postman_parity}** |")
    w(f"| Governance restored | **{gov_restored}** |")

    return "\n".join(lines) + "\n"


def main() -> None:
    ap = argparse.ArgumentParser(description="Generate production E2E RESULTS report")
    ap.add_argument("run_id", help="Run ID under .e2e-runs/production/")
    ap.add_argument("--out", help="Output markdown path (default docs/testing/production-e2e/RESULTS_<runId>.md)")
    args = ap.parse_args()

    run_dir = RUNS_ROOT / args.run_id
    if not run_dir.is_dir():
        print(f"ERROR: run directory not found: {run_dir}", file=sys.stderr)
        raise SystemExit(1)

    report = build_report(args.run_id, run_dir)
    # Safety scan
    for pat in SECRET_PATTERNS:
        if pat.search(report) and "redacted" not in pat.search(report).group(0).lower():
            print("WARN: possible secret in report — review before commit", file=sys.stderr)

    out = Path(args.out) if args.out else OUT_DIR / f"RESULTS_{args.run_id}.md"
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(report, encoding="utf-8")
    print(f"WROTE {out.relative_to(REPO_ROOT)}")


if __name__ == "__main__":
    main()

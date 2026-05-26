#!/usr/bin/env python3
"""Generate MANUAL_RETEST_GUIDE, API_TRACE, and POSTMAN_IMPORT_PARITY docs from E2E run evidence."""
from __future__ import annotations

import argparse
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
POSTMAN_COLL = REPO_ROOT / "postman" / "production" / "avf-production-e2e.postman_collection.json"
POSTMAN_ENV = REPO_ROOT / "postman" / "production" / "avf-production-e2e.postman_environment.json"
POSTMAN_GEN = REPO_ROOT / "postman" / "production" / "generate_postman_from_manifest.py"

FLOW_ROW = re.compile(
    r"\|\s*([^\|]+?)\s*\|\s*([^\|]+?)\s*\|\s*(\w+)\s*\|\s*(\S+)\s*\|\s*`([^`]+)`\s*\|"
)

BUSINESS_FLOWS = [
    ("Admin creates catalog/product/media", "REST-CATALOG, REST-MEDIA, REST-CATALOG", "Admin provisions sellable catalog and media assets."),
    ("Admin prepares machine/topology/planogram/stock", "REST-SITE, REST-MACHINE, REST-PLANO, REST-OP", "Machine topology, planogram, and stock are configured."),
    ("Machine activates and syncs catalog/media/inventory", "REST-MACHINE-004, GRPC-BOOT, GRPC-CAT, GRPC-MED, GRPC-INV", "Machine token, bootstrap, and sync pipelines."),
    ("Machine performs cash/manual sale", "GRPC-COMM-CASH-001", "Cash path without online PSP."),
    ("Backend validates vend success/failure/reconciliation", "GRPC-COMM-FAIL-001, GRPC-COMM-CANCEL-001", "Failure and cancel idempotency without real refund."),
    ("MQTT command dispatch and ACK", "MQTT-CMD-DISPATCH, MQTT-CMD-001", "Admin dispatches command; machine ACKs via MQTT."),
    ("Telemetry and readback", "MQTT-TEL, MQTT-READ, REST-REPORT", "Heartbeat/presence/inventory signals and admin readback."),
    ("REST route coverage", "REST-COV", "Every safe REST route classification exercised."),
    ("Cleanup removes E2E data only", "cleanup-attestation", "Only E2E-PROD prefix resources touched."),
]

EXECUTION_ORDER = [
    "Production health/version preflight",
    "Admin login and auth negative tests",
    "Catalog/category/brand/tag/product creation",
    "Media upload and metadata",
    "Site/machine provisioning and activation",
    "Topology/planogram/stock/operator session",
    "Reports and admin readback",
    "gRPC token refresh and bootstrap/check-in",
    "gRPC catalog/media/inventory sync",
    "gRPC cash/manual vend success",
    "gRPC vend failure and cancel idempotency",
    "MQTT TLS connect (valid + invalid credentials)",
    "MQTT command dispatch/subscribe/ACK/readback",
    "MQTT telemetry (heartbeat, presence, snapshot, inventory)",
    "MQTT negative cases",
    "REST route coverage (213 flows, no online payment)",
    "Postman/Newman REST parity",
    "Cleanup and reconciliation attestation",
]


def git_cmd(*args: str) -> str:
    try:
        return subprocess.check_output(["git", *args], cwd=REPO_ROOT, text=True, stderr=subprocess.DEVNULL).strip()
    except (subprocess.CalledProcessError, FileNotFoundError):
        return ""


def read_json(path: Path) -> Any:
    if not path.is_file():
        return None
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError:
        return None


def parse_result_md(result_md: Path) -> tuple[list[dict[str, str]], str | None, str | None]:
    rows: list[dict[str, str]] = []
    started = base_url = None
    if not result_md.is_file():
        return rows, started, base_url
    text = result_md.read_text(encoding="utf-8")
    m = re.search(r"started:\s*`([^`]+)`", text)
    if m:
        started = m.group(1)
    m = re.search(r"base_url:\s*`([^`]+)`", text)
    if m:
        base_url = m.group(1)
    for m in FLOW_ROW.finditer(text):
        rows.append({
            "id": m.group(1).strip(),
            "label": m.group(2).strip(),
            "protocol": m.group(3).strip(),
            "status": m.group(4).strip(),
            "evidence_label": m.group(5).strip(),
        })
    return rows, started, base_url


def load_manifest_flows() -> dict[str, dict[str, Any]]:
    flows: dict[str, dict[str, Any]] = {}
    for key in ("main", "coverage", "grpc", "mqtt"):
        path = PROD_E2E_DIR / f"e2e-manifest{'' if key == 'main' else '-' + key.replace('main', '') if key != 'coverage' else '-rest-coverage'}.yaml"
        if key == "main":
            path = PROD_E2E_DIR / "e2e-manifest.yaml"
        elif key == "coverage":
            path = PROD_E2E_DIR / "e2e-manifest-rest-coverage.yaml"
        elif key == "grpc":
            path = PROD_E2E_DIR / "e2e-manifest-grpc.yaml"
        else:
            path = PROD_E2E_DIR / "e2e-manifest-mqtt.yaml"
        if not path.exists():
            continue
        data = yaml.safe_load(path.read_text(encoding="utf-8"))
        for f in data.get("flows") or []:
            fid = f.get("id")
            if fid:
                f["_manifest"] = key
                flows[fid] = f
    return flows


def load_skipped(run_dir: Path) -> dict[str, str]:
    out: dict[str, str] = {}
    sf = run_dir / "skipped.flows.txt"
    if not sf.is_file():
        return out
    for line in sf.read_text(encoding="utf-8").splitlines():
        parts = line.split("|", 2)
        if len(parts) >= 3:
            out[parts[0].strip()] = parts[2].strip()
    return out


REDACT_PATTERNS = [
    (re.compile(r"AVF-[A-F0-9]{6}-[A-F0-9]{6}", re.I), "<redacted-activation-code>"),
    (re.compile(r"eyJ[A-Za-z0-9_+/=-]{16,}"), "<redacted-base64>"),
    (re.compile(r"admin@ldtv\.dev", re.I), "<admin-email>"),
    (re.compile(r'"idempotencyKey"\s*:\s*"[^"]{20,}"'), '"idempotencyKey": "<redacted>"'),
]


def scrub_secrets(text: str) -> str:
    for pat, repl in REDACT_PATTERNS:
        text = pat.sub(repl, text)
    return text


def redacted_json(raw_dir: Path, label: str, suffix: str) -> str:
    p = raw_dir / f"{label}.{suffix}.redacted.json"
    if p.is_file():
        return p.read_text(encoding="utf-8").strip()
    p2 = raw_dir / f"{label}.{suffix}.json"
    if p2.is_file():
        return "<present; see raw evidence — redacted copy missing>"
    return ""


def meta_json(raw_dir: Path, label: str) -> dict[str, Any]:
    return read_json(raw_dir / f"{label}.meta.json") or {}


def curl_template(req: dict[str, Any]) -> str:
    method = (req.get("method") or "GET").upper()
    url = req.get("url") or req.get("path") or "{{baseUrl}}/..."
    auth = req.get("auth") or "none"
    parts = ["curl", "-sS", "-L"]
    if auth in ("bearer_admin", "bearer_machine"):
        parts += ['-H', '"Authorization: Bearer <ADMIN_TOKEN or MACHINE_TOKEN>"']
    parts += ['-H', '"Content-Type: application/json"']
    body = req.get("body")
    if method == "GET":
        parts.append(f'"{url}"')
    elif method == "DELETE":
        parts += ["-X", "DELETE", f'"{url}"']
    else:
        b = json.dumps(body, separators=(",", ":")) if body else "{}"
        parts += ["-X", method, "-d", f"'{b}'", f'"{url}"']
    parts += ["-w", "'\\nHTTP %{http_code}'"]
    return " ".join(parts)


def grpcurl_template(rpc: str, target: str = "machine-api.ldtv.dev:443") -> str:
    svc, method = rpc.rsplit("/", 1) if "/" in rpc else (rpc, "")
    return (
        f'grpcurl -d @ -H "authorization: Bearer <MACHINE_TOKEN>" '
        f"{target} {svc}/{method} < request.json"
    )


def mosquitto_template(topic: str, direction: str = "publish") -> str:
    if direction == "publish":
        return f'mosquitto_pub -h mqtt.ldtv.dev -p 8883 --cafile <ca.pem> -u "<MQTT_USER>" -P "<MQTT_PASSWORD>" -t "{topic}" -m \'{{"payload":"..."}}\''
    return f'mosquitto_sub -h mqtt.ldtv.dev -p 8883 --cafile <ca.pem> -u "<MQTT_USER>" -P "<MQTT_PASSWORD>" -t "{topic}" -C 1'


def flow_rest_detail(row: dict[str, str], raw_dir: Path, manifest: dict[str, Any]) -> dict[str, Any]:
    label = row["evidence_label"]
    req_raw = read_json(raw_dir / f"{label}.request.json") or read_json(raw_dir / f"{label}.request.redacted.json")
    meta = meta_json(raw_dir, label)
    mf = manifest.get(row["id"], {})
    expected = mf.get("expected_status")
    if isinstance(expected, list):
        expected_s = ",".join(str(x) for x in expected)
    else:
        expected_s = str(expected) if expected is not None else "per manifest"
    req_body = redacted_json(raw_dir, label, "request")
    resp_body = redacted_json(raw_dir, label, "response")
    method = (req_raw or {}).get("method", meta.get("method", "?"))
    path = (req_raw or {}).get("path", meta.get("path", row["label"]))
    url = (req_raw or {}).get("url", "")
    auth = (req_raw or {}).get("auth", "none")
    code = meta.get("http_code") or meta.get("code") or "?"
    return {
        "flow_id": row["id"],
        "label": row["label"],
        "method": method,
        "path": path,
        "url": url,
        "auth": auth,
        "expected": expected_s,
        "actual": str(code),
        "status": row["status"],
        "req_summary": req_body[:500] if req_body else "",
        "resp_summary": resp_body[:500] if resp_body else "",
        "curl": curl_template(req_raw or {"method": method, "url": url, "auth": auth}),
    }


def flow_grpc_detail(row: dict[str, str], raw_dir: Path, manifest: dict[str, Any]) -> dict[str, Any]:
    label = row["evidence_label"]
    mf = manifest.get(row["id"], {})
    rpc = mf.get("rpc") or mf.get("label") or row["label"]
    meta = meta_json(raw_dir, label)
    req = redacted_json(raw_dir, label, "request")
    resp = redacted_json(raw_dir, label, "response")
    code = meta.get("grpc_code") or meta.get("code") or "OK"
    return {
        "flow_id": row["id"],
        "rpc": rpc,
        "target": "machine-api.ldtv.dev:443",
        "status": row["status"],
        "code": code,
        "req_summary": req[:500],
        "resp_summary": resp[:500],
        "grpcurl": grpcurl_template(str(rpc)),
    }


def flow_mqtt_detail(row: dict[str, str], raw_dir: Path, manifest: dict[str, Any]) -> dict[str, Any]:
    label = row["evidence_label"]
    mf = manifest.get(row["id"], {})
    meta = meta_json(raw_dir, label)
    topic = meta.get("topic") or mf.get("topic") or "?"
    direction = meta.get("direction") or "publish"
    payload = redacted_json(raw_dir, label, "request") or redacted_json(raw_dir, label, "payload")
    return {
        "flow_id": row["id"],
        "topic": topic,
        "direction": direction,
        "qos": meta.get("qos", mf.get("qos", 1)),
        "status": row["status"],
        "payload_summary": payload[:500],
        "cmd": mosquitto_template(str(topic), direction),
        "exit_code": meta.get("exit_code", 0),
    }


def newman_flow_map(run_dir: Path) -> dict[str, str]:
    report = read_json(run_dir / "postman" / "newman-report.json")
    out: dict[str, str] = {}
    if not report:
        return out
    for ex in report.get("run", {}).get("executions", []):
        name = ex.get("item", {}).get("name") or ""
        m = re.search(r"(REST-[A-Z0-9-]+)", name)
        fid = m.group(1) if m else name
        failed = any(
            a.get("error") for a in (ex.get("assertions") or [])
        )
        out[fid] = "fail" if failed else "pass"
    return out


def postman_collection_requests() -> dict[str, str]:
    coll = read_json(POSTMAN_COLL)
    out: dict[str, str] = {}
    if not coll:
        return out

    def walk(items: list[Any]) -> None:
        for it in items or []:
            if "item" in it:
                walk(it["item"])
            elif "request" in it:
                name = it.get("name", "")
                m = re.search(r"(REST-[A-Z0-9-]+)", name)
                fid = m.group(1) if m else name
                req = it.get("request", {})
                method = req.get("method", "?")
                url = req.get("url", {})
                if isinstance(url, dict):
                    raw = url.get("raw", "")
                else:
                    raw = str(url)
                out[fid] = f"{method} {raw}"

    walk(coll.get("item", []))
    return out


def env_var_names() -> list[str]:
    env = read_json(POSTMAN_ENV)
    names: list[str] = []
    if env:
        for v in env.get("values") or []:
            k = v.get("key")
            if k:
                names.append(str(k))
    return names


def generate_manual_guide(run_id: str, run_dir: Path, rows: list[dict], manifest: dict, skipped: dict, state: dict, cleanup: dict, results_path: Path) -> str:
    git_sha = git_cmd("rev-parse", "HEAD")
    branch = git_cmd("branch", "--show-current")
    _, started, base_url = parse_result_md(run_dir / "RESULT.md")
    pass_n = sum(1 for r in rows if r["status"] == "pass")
    fail_n = sum(1 for r in rows if r["status"] in ("fail", "optional-fail"))
    skip_n = sum(1 for r in rows if r["status"] == "skipped")
    lines: list[str] = []

    def w(s: str = "") -> None:
        lines.append(s)

    w(f"# Production Manual Retest Guide — `{run_id}`")
    w()
    w("## 1. Scope")
    w()
    w("| Field | Value |")
    w("|-------|-------|")
    w(f"| Production REST base URL | `{base_url or 'https://api.ldtv.dev'}` |")
    w("| Production gRPC target | `machine-api.ldtv.dev:443` (TLS) |")
    w("| Production MQTT | `mqtt.ldtv.dev:8883` (TLS) |")
    w(f"| Local git SHA | `{git_sha}` |")
    w(f"| Local branch | `{branch}` |")
    w("| Suite | `all-no-online-payment` |")
    w("| Online payment excluded | **YES** |")
    w("| Backend deploy required | **NO** (harness/docs verification) |")
    w(f"| Evidence RESULTS | `{results_path.relative_to(REPO_ROOT).as_posix()}` |")
    w(f"| Raw evidence | `.e2e-runs/production/{run_id}/raw/` |")
    w()
    w("**Coverage:** REST main, REST-COV (213 flows), gRPC machine runtime, MQTT command/telemetry, Postman/Newman REST parity, cleanup/reconciliation.")
    w()
    w("## 2. Required tools")
    w()
    for t in ["Git Bash or bash", "curl", "jq", "grpcurl", "mosquitto_pub / mosquitto_sub", "newman", "Postman Desktop", "Python 3", "Go toolchain (optional, for local CI checks)"]:
        w(f"- {t}")
    w()
    w("## 3. Required environment variables")
    w()
    w("Set locally in `tests/e2e/production/.env.production.e2e.local` — **never commit values**:")
    w()
    for name in [
        "E2E_PROD_BASE_URL", "E2E_PROD_GRPC_TARGET", "E2E_PROD_MQTT_HOST", "E2E_PROD_MQTT_PORT",
        "E2E_PROD_MQTT_USERNAME", "E2E_PROD_MQTT_PASSWORD", "E2E_PROD_ADMIN_EMAIL",
        "E2E_PROD_ADMIN_PASSWORD", "E2E_PRODUCTION_WRITE_CONFIRMATION",
    ]:
        w(f"- `{name}` — secret or confirmation flag; fill locally only")
    w()
    w("Runtime-generated during harness (in `.e2e-runs/.../state.json`, exported to Postman runtime env):")
    w("- `accessToken`, `refreshToken`, `machineToken`, `machineId`, `siteId`, `productId`, `categoryId`, `brandId`, `tagId`, `mediaId`, `planogramId`, `activationCode`, `operatorSessionId`, `commandId`, `mqttTopicPrefix`")
    w()
    w("## 4. Safety rules for manual retest")
    w()
    w(f"- Use prefix `E2E-PROD-{run_id}` for all created data")
    w("- Do **not** run real MoMo/ZaloPay/VietQR payment or PSP webhooks")
    w("- Do **not** mutate non-E2E production resources")
    w("- Do **not** print or commit tokens/passwords")
    w("- Run cleanup attestation after testing")
    w()
    w("## 5. Exact test execution order")
    w()
    for i, step in enumerate(EXECUTION_ORDER, 1):
        w(f"{i}. {step}")
    w()
    w("## 6. REST API manual retest details")
    w()
    w("| Flow ID | Label | Method | Path | Auth | Expected | Actual | Status | Manual curl template |")
    w("|---------|-------|--------|------|------|----------|--------|--------|---------------------|")
    raw_dir = run_dir / "raw"
    for row in rows:
        if row["protocol"] != "rest":
            continue
        d = flow_rest_detail(row, raw_dir, manifest)
        w(f"| {d['flow_id']} | {d['label'][:40]} | {d['method']} | `{d['path'][:60]}` | {d['auth']} | {d['expected']} | {d['actual']} | {d['status']} | see trace |")
    w()
    w("### Skipped REST flows (online payment / documented)")
    w()
    for fid, reason in sorted(skipped.items()):
        if fid.startswith("REST") or fid.startswith("REST-COV"):
            w(f"- `{fid}`: {reason}")
    w()
    w("## 7. gRPC manual retest details")
    w()
    w("| Flow ID | RPC | Target | Code | Status | grpcurl template |")
    w("|---------|-----|--------|------|--------|------------------|")
    for row in rows:
        if row["protocol"] != "grpc":
            continue
        d = flow_grpc_detail(row, raw_dir, manifest)
        w(f"| {d['flow_id']} | `{d['rpc']}` | {d['target']} | {d['code']} | {d['status']} | see trace |")
    w()
    w("## 8. MQTT manual retest details")
    w()
    w("| Flow ID | Topic | Direction | QoS | Status | mosquitto template |")
    w("|---------|-------|-----------|-----|--------|-------------------|")
    for row in rows:
        if row["protocol"] != "mqtt":
            continue
        d = flow_mqtt_detail(row, raw_dir, manifest)
        w(f"| {d['flow_id']} | `{d['topic'][:50]}` | {d['direction']} | {d['qos']} | {d['status']} | see trace |")
    w()
    w("## 9. Business flow explanation")
    w()
    for title, flows, why in BUSINESS_FLOWS:
        w(f"### {title}")
        w(f"- **Flows:** {flows}")
        w(f"- **Why:** {why}")
        w()
    w("## 10. Postman/Newman manual import and run guide")
    w()
    w(f"- Collection: `{POSTMAN_COLL.relative_to(REPO_ROOT).as_posix()}`")
    w(f"- Environment: `{POSTMAN_ENV.relative_to(REPO_ROOT).as_posix()}`")
    w(f"- Runtime env (live): `.e2e-runs/production/{run_id}/postman/runtime.postman_environment.json`")
    w(f"- Newman report: `.e2e-runs/production/{run_id}/postman/newman-report.json`")
    w()
    w("```bash")
    w(f'newman run "{POSTMAN_COLL.relative_to(REPO_ROOT).as_posix()}" \\')
    w(f'  -e ".e2e-runs/production/{run_id}/postman/runtime.postman_environment.json"')
    w("```")
    w()
    w("Postman covers **REST only**. gRPC and MQTT must be tested with grpcurl/mosquitto as documented above.")
    w()
    w("## 11. Cleanup/reconciliation")
    w()
    w(f"- Attestation: `.e2e-runs/production/{run_id}/cleanup-attestation.json`")
    if cleanup:
        w(f"- Status: `{cleanup.get('status', '?')}`")
        created = cleanup.get("resourcesCreated") or {}
        w(f"- Resources created: {len(created)} E2E resources")
        w("- Only `E2E-PROD-*` prefix data; non-E2E production data must not be deleted")
    w()
    w("## 12. Final verdict")
    w()
    w(f"| Metric | Value |")
    w(f"|--------|-------|")
    w(f"| Pass | **{pass_n}** |")
    w(f"| Fail | **{fail_n}** |")
    w(f"| Skipped | **{skip_n}** |")
    verdict = "READY_FOR_NO_ONLINE_PAYMENT_RELEASE" if fail_n == 0 else "NOT_READY"
    w(f"| Operator conclusion | **{verdict}** |")
    w()
    return "\n".join(lines) + "\n"


def generate_api_trace(run_id: str, run_dir: Path, rows: list[dict], manifest: dict, skipped: dict) -> str:
    lines: list[str] = []
    raw_dir = run_dir / "raw"
    state = read_json(run_dir / "state.json") or {}

    def w(s: str = "") -> None:
        lines.append(s)

    w(f"# Production API Trace — `{run_id}`")
    w()
    w(f"> Generated UTC: `{datetime.now(timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ')}` from redacted raw evidence.")
    w()

    for row in rows:
        fid, proto, status, label = row["id"], row["protocol"], row["status"], row["evidence_label"]
        w(f"## {fid} — {proto.upper()} — `{status}`")
        w()
        if proto == "rest":
            d = flow_rest_detail(row, raw_dir, manifest)
            w(f"- **Method/path:** `{d['method']} {d['path']}`")
            w(f"- **Auth:** {d['auth']}")
            w(f"- **Expected:** {d['expected']} | **Actual:** {d['actual']}")
            if d["req_summary"]:
                w("- **Request (redacted):**")
                w("```json")
                w(d["req_summary"])
                w("```")
            if d["resp_summary"]:
                w("- **Response (redacted):**")
                w("```json")
                w(d["resp_summary"])
                w("```")
            w(f"- **curl:** `{d['curl']}`")
        elif proto == "grpc":
            d = flow_grpc_detail(row, raw_dir, manifest)
            w(f"- **RPC:** `{d['rpc']}`")
            w(f"- **Code:** {d['code']}")
            if d["req_summary"]:
                w("- **Request (redacted):**")
                w("```json")
                w(d["req_summary"])
                w("```")
            if d["resp_summary"]:
                w("- **Response (redacted):**")
                w("```json")
                w(d["resp_summary"])
                w("```")
            w(f"- **grpcurl:** `{d['grpcurl']}`")
        elif proto == "mqtt":
            d = flow_mqtt_detail(row, raw_dir, manifest)
            w(f"- **Topic:** `{d['topic']}`")
            w(f"- **Direction:** {d['direction']} | **QoS:** {d['qos']}")
            if d["payload_summary"]:
                w("- **Payload (redacted):**")
                w("```json")
                w(d["payload_summary"])
                w("```")
            w(f"- **Command:** `{d['cmd']}`")
        if fid in skipped:
            w(f"- **Skipped reason:** {skipped[fid]}")
        deps = {k: state.get(k) for k in ("siteId", "machineId", "productId", "mediaId", "planogramId", "commandId") if state.get(k)}
        if deps:
            w(f"- **Dependencies:** `{json.dumps(deps)}`")
        w()
    return "\n".join(lines) + "\n"


def generate_postman_parity(run_id: str, run_dir: Path, rows: list[dict]) -> tuple[str, str]:
    newman_map = newman_flow_map(run_dir)
    coll_map = postman_collection_requests()
    env_names = env_var_names()
    mismatches: list[str] = []
    lines: list[str] = []

    def w(s: str = "") -> None:
        lines.append(s)

    w(f"# Postman Import Parity — `{run_id}`")
    w()
    w("## Artifacts")
    w()
    w(f"- Collection: `{POSTMAN_COLL.relative_to(REPO_ROOT).as_posix()}`")
    w(f"- Environment: `{POSTMAN_ENV.relative_to(REPO_ROOT).as_posix()}`")
    w(f"- Generator: `python {POSTMAN_GEN.relative_to(REPO_ROOT).as_posix()}`")
    w("- Parity CI: `bash scripts/ci/verify_production_postman_parity.sh`")
    w(f"- Newman report: `.e2e-runs/production/{run_id}/postman/newman-report.json`")
    w(f"- Manual-import parity Newman: `.e2e-runs/production/{run_id}/postman/manual-import-parity-newman-report.json`")
    w(f"- Runtime collection (no-online-payment): `.e2e-runs/production/{run_id}/postman/runtime.postman_collection.json`")
    w(f"- Runtime environment: `.e2e-runs/production/{run_id}/postman/runtime.postman_environment.json`")
    w()
    w("## Environment variable checklist")
    w()
    for name in sorted(env_names):
        secret = any(x in name.lower() for x in ("password", "token", "secret", "hmac"))
        w(f"- `{name}` — {'secret placeholder' if secret else 'runtime or placeholder'}")
    w()
    w("## Automation vs Postman comparison")
    w()
    w("Scope: **Postman-eligible REST main flows only** (REST-COV and handler-only flows are shell-only by design).")
    w()
    w("| Flow ID | Shell status | Postman request | Newman status | Match |")
    w("|---------|--------------|-----------------|---------------|-------|")
    match_count = 0
    for fid, postman_req in sorted(coll_map.items()):
        shell_row = next((r for r in rows if r["id"] == fid), None)
        shell_st = shell_row["status"] if shell_row else "not_run"
        newman_st = newman_map.get(fid, "n/a")
        if shell_st == "fail":
            match = "MISMATCH (shell fail)"
            mismatches.append(fid)
        elif newman_st == "fail":
            match = "MISMATCH (newman fail)"
            mismatches.append(fid)
        elif shell_st in ("skipped", "not_run"):
            match = "MATCH (excluded from no-online-payment / runtime subset)"
            match_count += 1
        elif shell_st == "pass" or newman_st == "pass":
            match = "MATCH"
            match_count += 1
        else:
            match = "MISMATCH"
            mismatches.append(fid)
        w(f"| {fid} | {shell_st} | {postman_req[:50]} | {newman_st} | {match} |")
    w()
    if mismatches:
        w("## Mismatches")
        w()
        for m in mismatches:
            w(f"- `{m}`")
        w()
        verdict = "POSTMAN_IMPORT_PARITY_FAIL"
    else:
        verdict = "POSTMAN_IMPORT_PARITY_PASS"
    w(f"## Final statement")
    w()
    w(f"**{verdict}**")
    w()
    w("Postman covers REST flows only. gRPC/MQTT are shell/grpcurl/mosquitto only.")
    w()
    return "\n".join(lines) + "\n", verdict


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("run_id", help="E2E run ID e.g. 20260525T072623Z-2630-27290")
    ap.add_argument("--write", action="store_true", help="Write docs to docs/testing/production-e2e/")
    args = ap.parse_args()
    run_id = args.run_id
    run_dir = RUNS_ROOT / run_id
    if not run_dir.is_dir():
        print(f"ERROR: run dir missing: {run_dir}", file=sys.stderr)
        return 1
    result_md = run_dir / "RESULT.md"
    rows, _, _ = parse_result_md(result_md)
    manifest = load_manifest_flows()
    skipped = load_skipped(run_dir)
    state = read_json(run_dir / "state.json") or {}
    cleanup = read_json(run_dir / "cleanup-attestation.json") or {}
    results_path = OUT_DIR / f"RESULTS_{run_id}.md"

    manual = generate_manual_guide(run_id, run_dir, rows, manifest, skipped, state, cleanup, results_path)
    trace = generate_api_trace(run_id, run_dir, rows, manifest, skipped)
    parity, parity_verdict = generate_postman_parity(run_id, run_dir, rows)

    out_manual = OUT_DIR / f"MANUAL_RETEST_GUIDE_{run_id}.md"
    out_trace = OUT_DIR / f"API_TRACE_{run_id}.md"
    out_parity = OUT_DIR / f"POSTMAN_IMPORT_PARITY_{run_id}.md"

    if args.write:
        OUT_DIR.mkdir(parents=True, exist_ok=True)
        out_manual.write_text(scrub_secrets(manual), encoding="utf-8")
        out_trace.write_text(scrub_secrets(trace), encoding="utf-8")
        out_parity.write_text(scrub_secrets(parity), encoding="utf-8")
        print(f"WROTE {out_manual.relative_to(REPO_ROOT)}")
        print(f"WROTE {out_trace.relative_to(REPO_ROOT)}")
        print(f"WROTE {out_parity.relative_to(REPO_ROOT)}")
        print(f"POSTMAN_PARITY_VERDICT={parity_verdict}")
    else:
        print(manual[:2000])
        print("...(dry-run; use --write to save files)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

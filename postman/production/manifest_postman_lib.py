"""Shared production E2E manifest → Postman model (generator + shell parity validator)."""
from __future__ import annotations

import hashlib
import json
import re
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError:
    yaml = None  # type: ignore[assignment]

POSTMAN_VAR_RE = re.compile(r"\{\{([a-zA-Z0-9_]+)\}\}")

# Manifest template vars → Postman environment keys (exported placeholders only).
MANIFEST_TO_POSTMAN_VAR: dict[str, str] = {
    "run_prefix": "runPrefix",
    "run_id": "runId",
    "admin_email": "adminEmail",
    "admin_password": "adminPassword",
    "admin_email_invalid_test": "adminEmailInvalidTest",
    "categoryId": "categoryId",
    "brandId": "brandId",
    "tagId": "tagId",
    "mediaId": "mediaId",
    "media_sha256": "mediaSha256",
    "productId": "productId",
    "siteId": "siteId",
    "machineId": "machineId",
    "activationCode": "activationCode",
    "machineToken": "machineAccessToken",
    "orderId": "orderId",
    "paymentId": "paymentId",
    "commandId": "commandId",
    "webhook_event_id": "webhookEventId",
    "operatorSessionId": "operatorSessionId",
    "planogramId": "planogramId",
    "planogramRevision": "planogramRevision",
}

POSTMAN_ENV_KEYS: list[tuple[str, str]] = [
    ("baseUrl", ""),
    ("adminEmail", ""),
    ("adminPassword", ""),
    ("adminEmailInvalidTest", "e2e-invalid@invalid.local"),
    ("accessToken", ""),
    ("machineAccessToken", ""),
    ("machineId", ""),
    ("activationCode", ""),
    ("runId", ""),
    ("runPrefix", ""),
    ("categoryId", ""),
    ("brandId", ""),
    ("tagId", ""),
    ("mediaId", ""),
    ("mediaSha256", ""),
    ("productId", ""),
    ("siteId", ""),
    ("orderId", ""),
    ("paymentId", ""),
    ("commandId", ""),
    ("webhookSecret", ""),
    ("webhookEventId", ""),
    ("operatorSessionId", ""),
    ("planogramId", ""),
    ("planogramRevision", ""),
    ("allowGatedWrites", "false"),
    ("confirmProductionWrites", ""),
]

GATED_PREREQUEST = [
    "const allow = String(pm.environment.get('allowGatedWrites')||'').toLowerCase()==='true';",
    "const confirm = pm.environment.get('confirmProductionWrites')==='I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION';",
    "if (!allow || !confirm) { throw new Error('Gated write blocked'); }",
]

WEBHOOK_PREREQUEST = {
    "webhook_hmac": [
        "const secret = pm.environment.get('webhookSecret') || '';",
        "const body = pm.request.body ? (pm.request.body.raw || '') : '';",
        "const ts = Math.floor(Date.now() / 1000).toString();",
        "const sig = CryptoJS.HmacSHA256(ts + '.' + body, secret).toString(CryptoJS.enc.Hex);",
        "pm.request.headers.upsert({ key: 'X-AVF-Webhook-Timestamp', value: ts });",
        "pm.request.headers.upsert({ key: 'X-AVF-Webhook-Signature', value: sig });",
    ],
    "webhook_hmac_stale": [
        "const secret = pm.environment.get('webhookSecret') || '';",
        "const body = pm.request.body ? (pm.request.body.raw || '') : '';",
        "const ts = (Math.floor(Date.now() / 1000) - 86400).toString();",
        "const sig = CryptoJS.HmacSHA256(ts + '.' + body, secret).toString(CryptoJS.enc.Hex);",
        "pm.request.headers.upsert({ key: 'X-AVF-Webhook-Timestamp', value: ts });",
        "pm.request.headers.upsert({ key: 'X-AVF-Webhook-Signature', value: sig });",
    ],
    "webhook_hmac_invalid": [
        "pm.request.headers.upsert({ key: 'X-AVF-Webhook-Timestamp', value: '0' });",
        "pm.request.headers.upsert({ key: 'X-AVF-Webhook-Signature', value: 'invalid' });",
    ],
}


def postman_var(name: str) -> str:
    return MANIFEST_TO_POSTMAN_VAR.get(name, name)


def to_postman_template(text: str) -> str:
    return POSTMAN_VAR_RE.sub(lambda m: "{{" + postman_var(m.group(1)) + "}}", text)


def stable_uuid(seed: str) -> str:
    h = hashlib.sha256(seed.encode()).hexdigest()
    return f"{h[:8]}-{h[8:12]}-4{h[13:16]}-{h[16:20]}-{h[20:32]}"


def load_main_manifest(manifest_path: Path) -> dict[str, Any]:
    if yaml is None:
        raise RuntimeError("PyYAML required")
    return yaml.safe_load(manifest_path.read_text(encoding="utf-8")) or {}


def postman_exclude_phases(manifest: dict[str, Any]) -> set[str]:
    return set((manifest.get("postman") or {}).get("exclude_phases") or [])


def collect_manifest_rest_flows(manifest: dict[str, Any], *, postman_only: bool = True) -> list[dict[str, Any]]:
    """REST flows the shell harness executes that belong in Postman parity scope."""
    exclude = postman_exclude_phases(manifest) if postman_only else set()
    out: list[dict[str, Any]] = []

    def add(flow: dict[str, Any], parent_id: str | None = None) -> None:
        if flow.get("protocol") != "rest":
            return
        phase = flow.get("phase") or "misc"
        if postman_only and phase in exclude:
            return
        if not flow.get("method"):
            return
        entry = dict(flow)
        if parent_id:
            entry["_parent_handler"] = parent_id
        out.append(entry)

    for flow in manifest.get("flows") or []:
        handler = flow.get("handler")
        if handler == "media_presigned_upload":
            if postman_only and (flow.get("phase") or "") in exclude:
                continue
            for sub in (flow.get("init_flow"), flow.get("complete_flow")):
                if sub and sub.get("method"):
                    add(sub, parent_id=str(flow.get("id")))
            continue
        if handler and not flow.get("method"):
            continue
        add(flow)

    return out


def manifest_body_raw(flow: dict[str, Any], manifest_dir: Path) -> str | None:
    method = str(flow.get("method", "")).upper()
    if method not in ("POST", "PUT", "PATCH"):
        return None
    tpl_file = flow.get("request_template_file")
    if tpl_file:
        raw = (manifest_dir / tpl_file).read_text(encoding="utf-8")
        return to_postman_template(raw)
    if flow.get("request_template") is not None:
        return to_postman_template(json.dumps(flow["request_template"], indent=2))
    return None


def manifest_path(flow: dict[str, Any]) -> str:
    return to_postman_template(str(flow.get("path", "")))


def manifest_query_pairs(path: str) -> list[tuple[str, str]]:
    if "?" not in path:
        return []
    qs = path.split("?", 1)[1]
    pairs: list[tuple[str, str]] = []
    for part in qs.split("&"):
        if "=" in part:
            k, v = part.split("=", 1)
            pairs.append((k, v))
    return pairs


def manifest_auth_mode(flow: dict[str, Any]) -> str:
    return str(flow.get("auth") or "none")


def manifest_headers(flow: dict[str, Any]) -> dict[str, str]:
    headers = {"Content-Type": "application/json"}
    auth = manifest_auth_mode(flow)
    if auth == "bearer_admin":
        headers["Authorization"] = "Bearer {{accessToken}}"
    elif auth == "bearer_machine":
        headers["Authorization"] = "Bearer {{machineAccessToken}}"
    elif auth.startswith("webhook_hmac"):
        headers["X-AVF-Webhook-Timestamp"] = "<computed>"
        headers["X-AVF-Webhook-Signature"] = "<computed>"
    if flow.get("idempotency"):
        fid = flow.get("id") or flow.get("_parent_handler") or "flow"
        headers["Idempotency-Key"] = f"{{{{runPrefix}}}}-{fid}"
    return headers


def manifest_expected_status(flow: dict[str, Any]) -> int | list[int]:
    exp = flow.get("expected_status", 200)
    if isinstance(exp, list):
        return [int(x) for x in exp]
    return int(exp)


def manifest_assertions(flow: dict[str, Any]) -> list[dict[str, Any]]:
    return list(flow.get("assertions") or [])


def parity_key(flow: dict[str, Any]) -> str:
    path = manifest_path(flow).split("?", 1)[0]
    return f"{str(flow.get('method', '')).upper()} {path}"


def flow_request_spec(flow: dict[str, Any], manifest_dir: Path) -> dict[str, Any]:
    path_full = manifest_path(flow)
    path_only = path_full.split("?", 1)[0]
    return {
        "flow_id": flow.get("id"),
        "method": str(flow.get("method", "")).upper(),
        "path": path_only,
        "path_with_query": path_full,
        "query": manifest_query_pairs(path_full),
        "auth": manifest_auth_mode(flow),
        "headers": manifest_headers(flow),
        "body": manifest_body_raw(flow, manifest_dir),
        "expected_status": manifest_expected_status(flow),
        "assertions": manifest_assertions(flow),
        "idempotency": bool(flow.get("idempotency")),
        "capture": flow.get("capture") or {},
    }


def _url_obj(path_with_query: str) -> dict[str, Any]:
    path = path_with_query.split("?", 1)[0]
    query: list[dict[str, str]] = []
    for k, v in manifest_query_pairs(path_with_query):
        query.append({"key": k, "value": to_postman_template(v)})
    segments = [s for s in path.strip("/").split("/") if s]
    url: dict[str, Any] = {
        "raw": "{{baseUrl}}" + path_with_query if path_with_query.startswith("/") else "{{baseUrl}}/" + path_with_query,
        "host": ["{{baseUrl}}"],
        "path": segments,
    }
    if query:
        url["query"] = query
    return url


def _auth_block(auth: str) -> dict[str, Any] | None:
    if auth == "bearer_admin":
        return {"type": "bearer", "bearer": [{"key": "token", "value": "{{accessToken}}", "type": "string"}]}
    if auth == "bearer_machine":
        return {"type": "bearer", "bearer": [{"key": "token", "value": "{{machineAccessToken}}", "type": "string"}]}
    return None


def _status_test_lines(expected: int | list[int]) -> list[str]:
    if isinstance(expected, list):
        return [
            "pm.test('expected HTTP status (manifest)', function () {",
            f"  pm.expect([{','.join(str(x) for x in expected)}]).to.include(pm.response.code);",
            "});",
        ]
    return [
        "pm.test('expected HTTP status (manifest)', function () {",
        f"  pm.response.to.have.status({expected});",
        "});",
    ]


def _assertion_test_lines(assertions: list[dict[str, Any]]) -> list[str]:
    lines: list[str] = []
    for i, a in enumerate(assertions):
        atype = a.get("type")
        if atype == "body_not_empty":
            lines += [
                f"pm.test('assertion {i}: body_not_empty', function () {{",
                "  pm.expect(pm.response.text()).to.not.be.empty;",
                "});",
            ]
        elif atype == "json_path_exists":
            path = str(a.get("path", ""))
            js = ".".join(path.split("."))
            lines += [
                f"pm.test('assertion {i}: json_path_exists {path}', function () {{",
                "  const j = pm.response.json();",
                f"  pm.expect(j.{js}).to.not.be.undefined;",
                "});",
            ]
        elif atype == "json_path_equals":
            path = str(a.get("path", ""))
            value = json.dumps(a.get("value", ""))
            js = ".".join(path.split("."))
            lines += [
                f"pm.test('assertion {i}: json_path_equals {path}', function () {{",
                "  const j = pm.response.json();",
                f"  pm.expect(String(j.{js})).to.eql(String({value}));",
                "});",
            ]
    return lines


def _capture_test_lines(capture: dict[str, Any] | None) -> list[str]:
    if not capture:
        return []
    lines = ["try {", "  const j = pm.response.json();"]
    for var, jqpath in capture.items():
        pv = postman_var(var)
        js_path = ".".join(str(jqpath).split("."))
        lines.append(f"  if (j.{js_path} !== undefined) pm.environment.set('{pv}', j.{js_path});")
    lines.append("} catch (e) {}")
    return lines


def build_postman_request(flow: dict[str, Any], manifest_dir: Path) -> dict[str, Any]:
    method = str(flow["method"]).upper()
    path_full = manifest_path(flow)
    auth = manifest_auth_mode(flow)
    headers = [{"key": "Content-Type", "value": "application/json"}]
    if flow.get("idempotency"):
        fid = flow.get("id") or "flow"
        headers.append({"key": "Idempotency-Key", "value": f"{{{{runPrefix}}}}-{fid}"})
    body_raw = manifest_body_raw(flow, manifest_dir)
    req: dict[str, Any] = {
        "method": method,
        "header": headers,
        "url": _url_obj(path_full),
    }
    if body_raw and method in ("POST", "PUT", "PATCH"):
        req["body"] = {"mode": "raw", "raw": body_raw}
    ab = _auth_block(auth)
    if ab:
        req["auth"] = ab
    return req


def build_postman_events(flow: dict[str, Any], manifest: dict[str, Any]) -> list[dict[str, Any]]:
    events: list[dict[str, Any]] = []
    auth = manifest_auth_mode(flow)
    phase = flow.get("phase") or "misc"
    exclude = postman_exclude_phases(manifest)
    gate_phases = ("preflight", "negative", "rest-coverage")
    needs_gate = auth not in ("none", "webhook_hmac", "webhook_hmac_invalid", "webhook_hmac_stale")
    if needs_gate and phase not in gate_phases and phase not in exclude:
        events.append({"listen": "prerequest", "script": {"type": "text/javascript", "exec": GATED_PREREQUEST}})
    if auth in WEBHOOK_PREREQUEST:
        events.append(
            {"listen": "prerequest", "script": {"type": "text/javascript", "exec": WEBHOOK_PREREQUEST[auth]}}
        )
    test_lines = _status_test_lines(manifest_expected_status(flow))
    test_lines += _assertion_test_lines(manifest_assertions(flow))
    test_lines += _capture_test_lines(flow.get("capture"))
    if test_lines:
        events.append({"listen": "test", "script": {"type": "text/javascript", "exec": test_lines}})
    return events


def build_postman_collection(manifest_path: Path) -> tuple[dict[str, Any], dict[str, Any], list[dict[str, Any]]]:
    manifest = load_main_manifest(manifest_path)
    manifest_dir = manifest_path.parent
    postman_cfg = manifest.get("postman") or {}
    flows = collect_manifest_rest_flows(manifest, postman_only=True)

    folders: dict[str, list[dict[str, Any]]] = {}
    for flow in flows:
        phase = flow.get("phase") or "misc"
        folders.setdefault(phase, []).append(flow)

    items: list[dict[str, Any]] = []
    for phase in sorted(folders.keys()):
        children: list[dict[str, Any]] = []
        for flow in folders[phase]:
            fid = flow.get("id") or parity_key(flow)
            children.append(
                {
                    "name": f"{fid} — {flow.get('label', fid)}",
                    "request": build_postman_request(flow, manifest_dir),
                    "event": build_postman_events(flow, manifest) or None,
                    "_manifest_flow_id": fid,
                }
            )
        items.append({"name": phase, "item": children})

    seed = hashlib.sha256(json.dumps(flows, sort_keys=True, default=str).encode()).hexdigest()
    collection = {
        "info": {
            "_postman_id": stable_uuid("collection:" + seed),
            "name": postman_cfg.get("collection_name", "AVF Production E2E (manifest)"),
            "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
            "description": (
                "Generated from tests/e2e/production/e2e-manifest.yaml only — do not edit by hand. "
                "Regenerate: python postman/production/generate_postman_from_manifest.py"
            ),
        },
        "item": items,
        "variable": [{"key": "baseUrl", "value": "https://api.ldtv.dev"}],
    }

    environment = {
        "id": stable_uuid("environment:" + seed),
        "name": postman_cfg.get("environment_name", "AVF Production E2E"),
        "values": [{"key": k, "value": v, "enabled": True, "type": "default"} for k, v in POSTMAN_ENV_KEYS],
        "_postman_variable_scope": "environment",
    }
    return collection, environment, flows


def _flow_id_from_postman_name(name: str) -> str | None:
    m = re.match(r"^([A-Z0-9-]+)\s+—", name or "")
    return m.group(1) if m else None


def collect_postman_rest_requests(collection: dict[str, Any]) -> list[dict[str, Any]]:
    out: list[dict[str, Any]] = []

    def walk(items: list[Any]) -> None:
        for it in items or []:
            if "item" in it:
                walk(it["item"])
            elif "request" in it:
                req = it["request"]
                method = str(req.get("method", "")).upper()
                url = req.get("url")
                path_parts: list[str] = []
                query_pairs: list[tuple[str, str]] = []
                if isinstance(url, dict):
                    path_parts = [str(p) for p in url.get("path") or []]
                    for q in url.get("query") or []:
                        query_pairs.append((str(q.get("key", "")), str(q.get("value", ""))))
                path = "/" + "/".join(path_parts)
                auth = "none"
                auth_obj = req.get("auth") or {}
                if auth_obj.get("type") == "bearer":
                    token = ""
                    for b in auth_obj.get("bearer") or []:
                        if b.get("key") == "token":
                            token = str(b.get("value", ""))
                    if token == "{{accessToken}}":
                        auth = "bearer_admin"
                    elif token == "{{machineAccessToken}}":
                        auth = "bearer_machine"
                headers: dict[str, str] = {}
                for h in req.get("header") or []:
                    headers[str(h.get("key"))] = str(h.get("value"))
                if "Authorization" in headers:
                    if "{{accessToken}}" in headers["Authorization"]:
                        auth = "bearer_admin"
                    elif "{{machineAccessToken}}" in headers["Authorization"]:
                        auth = "bearer_machine"
                if "X-AVF-Webhook-Signature" in headers:
                    if headers.get("X-AVF-Webhook-Signature") == "invalid":
                        auth = "webhook_hmac_invalid"
                    elif headers.get("X-AVF-Webhook-Timestamp") == "0":
                        auth = "webhook_hmac_invalid"
                    else:
                        auth = "webhook_hmac"
                body = None
                body_obj = req.get("body") or {}
                if body_obj.get("mode") == "raw":
                    body = str(body_obj.get("raw") or "")
                events = it.get("event") or []
                expected_status: int | list[int] | None = None
                assertions: list[dict[str, Any]] = []
                for ev in events:
                    if ev.get("listen") == "prerequest":
                        script = "\n".join(ev.get("script", {}).get("exec") or [])
                        if "webhook_hmac_invalid" in script or "value: 'invalid'" in script:
                            auth = "webhook_hmac_invalid"
                        elif "86400" in script and "HmacSHA256" in script:
                            auth = "webhook_hmac_stale"
                        elif "HmacSHA256" in script:
                            auth = "webhook_hmac"
                    if ev.get("listen") != "test":
                        continue
                    for line in ev.get("script", {}).get("exec") or []:
                        m = re.search(r"pm\.response\.to\.have\.status\((\d+)\)", line)
                        if m:
                            expected_status = int(m.group(1))
                        m2 = re.search(r"pm\.expect\(\[([0-9,\s]+)\]\)", line)
                        if m2:
                            expected_status = [int(x.strip()) for x in m2.group(1).split(",") if x.strip()]
                        if "body_not_empty" in line:
                            assertions.append({"type": "body_not_empty"})
                        m3 = re.search(r"json_path_exists (\S+)", line)
                        if m3:
                            assertions.append({"type": "json_path_exists", "path": m3.group(1)})
                        m4 = re.search(r"json_path_equals (\S+)", line)
                        if m4:
                            assertions.append({"type": "json_path_equals", "path": m4.group(1)})
                flow_id = it.get("_manifest_flow_id") or _flow_id_from_postman_name(str(it.get("name", "")))
                out.append(
                    {
                        "name": it.get("name", ""),
                        "flow_id": flow_id,
                        "method": method,
                        "path": path,
                        "query": query_pairs,
                        "auth": auth,
                        "headers": headers,
                        "body": body,
                        "expected_status": expected_status,
                        "assertions": assertions,
                        "events": events,
                    }
                )

    walk(collection.get("item") or [])
    return out


def _norm_body(body: str | None) -> str:
    if body is None:
        return ""
    try:
        return json.dumps(json.loads(body), sort_keys=True, separators=(",", ":"))
    except json.JSONDecodeError:
        return re.sub(r"\s+", "", body.strip())


def _norm_headers(headers: dict[str, str]) -> dict[str, str]:
    out: dict[str, str] = {}
    for k, v in sorted(headers.items()):
        if k.lower() == "authorization":
            out[k] = v
        elif k.lower().startswith("x-avf-webhook"):
            out[k] = "<computed>"
        else:
            out[k] = v
    return out


def validate_shell_postman_parity(manifest_path: Path, collection_path: Path) -> list[str]:
    manifest = load_main_manifest(manifest_path)
    manifest_dir = manifest_path.parent
    coll = json.loads(collection_path.read_text(encoding="utf-8"))
    shell_flows = collect_manifest_rest_flows(manifest, postman_only=True)
    shell_by_id = {str(f.get("id")): flow_request_spec(f, manifest_dir) for f in shell_flows if f.get("id")}
    postman_reqs = collect_postman_rest_requests(coll)
    postman_by_id = {str(r["flow_id"]): r for r in postman_reqs if r.get("flow_id")}

    errors: list[str] = []

    if len(shell_by_id) != len(postman_by_id):
        errors.append(
            f"request count mismatch: shell postman-eligible={len(shell_by_id)} postman={len(postman_by_id)}"
        )

    for fid, spec in shell_by_id.items():
        pm = postman_by_id.get(fid)
        if not pm:
            errors.append(f"shell flow {fid} missing in Postman")
            continue
        if pm.get("method") != spec["method"] or pm.get("path") != spec["path"]:
            errors.append(
                f"{fid}: route mismatch shell={spec['method']} {spec['path']} "
                f"postman={pm.get('method')} {pm.get('path')}"
            )
        if pm.get("auth") != spec["auth"]:
            errors.append(f"{fid}: auth mismatch shell={spec['auth']} postman={pm.get('auth')}")
        sh = _norm_headers(spec["headers"])
        ph = _norm_headers(pm.get("headers") or {})
        for hk, hv in sh.items():
            lk = hk.lower()
            if lk in ("authorization", "x-avf-webhook-timestamp", "x-avf-webhook-signature"):
                continue
            if ph.get(hk) != hv:
                errors.append(f"{fid}: header {hk} shell={hv!r} postman={ph.get(hk)!r}")
        if spec["query"] != pm.get("query"):
            errors.append(f"{fid}: query mismatch shell={spec['query']} postman={pm.get('query')}")
        sb = _norm_body(spec.get("body"))
        pb = _norm_body(pm.get("body"))
        if sb != pb:
            errors.append(f"{fid}: body mismatch")
        if pm.get("expected_status") != spec["expected_status"]:
            errors.append(
                f"{fid}: expected_status shell={spec['expected_status']} postman={pm.get('expected_status')}"
            )
        shell_assert_n = len(spec["assertions"]) + 1  # +1 for HTTP status test
        pm_assert_n = len(pm.get("assertions") or []) + (1 if pm.get("expected_status") is not None else 0)
        if pm_assert_n < shell_assert_n:
            errors.append(
                f"{fid}: assertion/test count shell>={shell_assert_n} postman={pm_assert_n}"
            )

    for fid, pm in postman_by_id.items():
        if fid not in shell_by_id:
            errors.append(f"Postman request not in shell manifest scope: {fid} ({pm.get('name')})")

    env_keys = {k for k, _ in POSTMAN_ENV_KEYS}
    coll_text = collection_path.read_text(encoding="utf-8")
    for m in POSTMAN_VAR_RE.finditer(coll_text):
        var = postman_var(m.group(1))
        if var not in env_keys and var != "baseUrl":
            errors.append(f"Postman uses unknown env var {{{{{var}}}}} (from manifest {m.group(1)})")

    return errors

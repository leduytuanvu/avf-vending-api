#!/usr/bin/env python3
"""Generate Postman collection + environment from tests/e2e/production/e2e-manifest.yaml."""
from __future__ import annotations

import json
import re
import sys
import uuid
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError:
    print("ERROR: PyYAML required (pip install pyyaml or apt install python3-yaml)", file=sys.stderr)
    raise SystemExit(2)

REPO_ROOT = Path(__file__).resolve().parents[2]
MANIFEST = REPO_ROOT / "tests" / "e2e" / "production" / "e2e-manifest.yaml"
OUT_COLL = REPO_ROOT / "postman" / "production" / "avf-production-e2e.postman_collection.json"
OUT_ENV = REPO_ROOT / "postman" / "production" / "avf-production-e2e.postman_environment.json"

GATED_PREREQUEST = [
    "const allow = String(pm.environment.get('allowGatedWrites')||'').toLowerCase()==='true';",
    "const confirm = pm.environment.get('confirmProductionWrites')==='I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION';",
    "if (!allow || !confirm) { throw new Error('Gated write blocked'); }",
]

POSTMAN_VAR_RE = re.compile(r"\{\{([a-zA-Z0-9_]+)\}\}")


def _postman_var(name: str) -> str:
    mapping = {
        "run_prefix": "runPrefix",
        "run_id": "runId",
        "admin_email": "adminEmail",
        "admin_password": "adminPassword",
        "categoryId": "categoryId",
        "brandId": "brandId",
        "tagId": "tagId",
        "mediaId": "mediaId",
        "productId": "productId",
        "siteId": "siteId",
        "machineId": "machineId",
        "activationCode": "activationCode",
        "machineToken": "machineToken",
        "orderId": "orderId",
        "paymentId": "paymentId",
        "commandId": "commandId",
    }
    return mapping.get(name, name)


def _to_postman_template(text: str) -> str:
    def repl(m: re.Match[str]) -> str:
        return "{{" + _postman_var(m.group(1)) + "}}"

    return POSTMAN_VAR_RE.sub(repl, text)


def _auth_block(auth: str) -> dict[str, Any] | None:
    if auth in ("bearer_admin", "none"):
        if auth == "bearer_admin":
            return {"type": "bearer", "bearer": [{"key": "token", "value": "{{accessToken}}", "type": "string"}]}
        return None
    if auth == "bearer_machine":
        return {"type": "bearer", "bearer": [{"key": "token", "value": "{{machineToken}}", "type": "string"}]}
    return None


def _url_obj(base_path: str) -> dict[str, Any]:
    path = base_path.split("?", 1)[0]
    query: list[dict[str, str]] = []
    if "?" in base_path:
        qs = base_path.split("?", 1)[1]
        for part in qs.split("&"):
            if "=" in part:
                k, v = part.split("=", 1)
                query.append({"key": k, "value": _to_postman_template(v)})
    segments = [s for s in path.strip("/").split("/") if s]
    return {
        "raw": "{{baseUrl}}" + path if path.startswith("/") else "{{baseUrl}}/" + path,
        "host": ["{{baseUrl}}"],
        "path": segments,
        "query": query or None,
    }


def _build_request(flow: dict[str, Any], manifest_dir: Path) -> dict[str, Any]:
    method = flow["method"]
    path = _to_postman_template(flow["path"])
    auth = flow.get("auth", "none")
    headers = [{"key": "Content-Type", "value": "application/json"}]
    if flow.get("idempotency"):
        headers.append({"key": "Idempotency-Key", "value": "{{runPrefix}}-{{$guid}}"})
    body: dict[str, Any] | None = None
    tpl_file = flow.get("request_template_file")
    if tpl_file:
        raw = (manifest_dir / tpl_file).read_text(encoding="utf-8")
        raw = _to_postman_template(raw)
        body = {"mode": "raw", "raw": raw}
    elif flow.get("request_template"):
        body = {
            "mode": "raw",
            "raw": json.dumps(flow["request_template"], indent=2),
        }
        body["raw"] = _to_postman_template(body["raw"])
    req: dict[str, Any] = {
        "method": method,
        "header": headers,
        "url": _url_obj(path),
    }
    if body and method in ("POST", "PUT", "PATCH"):
        req["body"] = body
    ab = _auth_block(auth)
    if ab:
        req["auth"] = ab
    return req


def _capture_test_script(capture: dict[str, Any] | None) -> list[str]:
    if not capture:
        return []
    lines = ["try {", "  const j = pm.response.json();"]
    for var, jqpath in capture.items():
        postman_var = _postman_var(var)
        # simple path: tokens.accessToken -> j.tokens.accessToken
        js_path = ".".join(jqpath.replace(".", ".").split("."))
        lines.append(f"  if (j.{js_path} !== undefined) pm.environment.set('{postman_var}', j.{js_path});")
    lines.append("} catch (e) {}")
    return lines


def main() -> int:
    manifest = yaml.safe_load(MANIFEST.read_text(encoding="utf-8"))
    postman_cfg = manifest.get("postman") or {}
    exclude = set(postman_cfg.get("exclude_phases") or [])
    manifest_dir = MANIFEST.parent

    folders: dict[str, list[dict[str, Any]]] = {}
    for flow in manifest.get("flows") or []:
        if flow.get("protocol") != "rest":
            continue
        if flow.get("phase") in exclude:
            continue
        phase = flow.get("phase") or "misc"
        folders.setdefault(phase, []).append(flow)

    items: list[dict[str, Any]] = []
    for phase, flows in folders.items():
        children: list[dict[str, Any]] = []
        for flow in flows:
            capture = flow.get("capture")
            test_lines = _capture_test_script(capture)
            events: list[dict[str, Any]] = []
            if flow.get("auth") != "none" and flow.get("phase") not in ("preflight", "negative"):
                events.append({"listen": "prerequest", "script": {"type": "text/javascript", "exec": GATED_PREREQUEST}})
            if test_lines:
                events.append({"listen": "test", "script": {"type": "text/javascript", "exec": test_lines}})
            children.append(
                {
                    "name": f"{flow['id']} — {flow['label']}",
                    "request": _build_request(flow, manifest_dir),
                    "event": events or None,
                }
            )
        items.append({"name": phase, "item": children})

    collection = {
        "info": {
            "_postman_id": str(uuid.uuid4()),
            "name": postman_cfg.get("collection_name", "AVF Production E2E (manifest)"),
            "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
            "description": "Generated from tests/e2e/production/e2e-manifest.yaml — do not edit by hand.",
        },
        "item": items,
        "variable": [{"key": "baseUrl", "value": "https://api.ldtv.dev"}],
    }

    env_values = [
        ("baseUrl", ""),
        ("adminEmail", ""),
        ("adminPassword", ""),
        ("accessToken", ""),
        ("machineToken", ""),
        ("machineId", ""),
        ("runId", ""),
        ("runPrefix", ""),
        ("categoryId", ""),
        ("brandId", ""),
        ("tagId", ""),
        ("mediaId", ""),
        ("productId", ""),
        ("siteId", ""),
        ("activationCode", ""),
        ("orderId", ""),
        ("paymentId", ""),
        ("commandId", ""),
        ("webhookSecret", ""),
        ("allowGatedWrites", "false"),
        ("confirmProductionWrites", ""),
    ]
    environment = {
        "id": str(uuid.uuid4()),
        "name": postman_cfg.get("environment_name", "AVF Production E2E"),
        "values": [{"key": k, "value": v, "enabled": True} for k, v in env_values],
        "_postman_variable_scope": "environment",
    }

    OUT_COLL.parent.mkdir(parents=True, exist_ok=True)
    OUT_COLL.write_text(json.dumps(collection, indent=2) + "\n", encoding="utf-8")
    OUT_ENV.write_text(json.dumps(environment, indent=2) + "\n", encoding="utf-8")
    rest_count = sum(len(v) for v in folders.values())
    print(f"GENERATED {OUT_COLL.name}: {rest_count} REST requests from manifest")
    print(f"GENERATED {OUT_ENV.name}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

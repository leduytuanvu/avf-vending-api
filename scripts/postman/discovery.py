"""Independent API discovery from source-of-truth code artifacts."""
from __future__ import annotations

import json
import re
from pathlib import Path
from typing import Any

from gfs_import import REPO_ROOT, gfs
from folder_business import assign_folder_business, flow_name

SWAGGER = REPO_ROOT / "docs" / "swagger" / "swagger.json"
GENERATED = REPO_ROOT / "postman" / "generated"
INVENTORY_JSON = GENERATED / "API_INVENTORY_CANONICAL.json"


def load_swagger() -> dict:
    return json.loads(SWAGGER.read_text(encoding="utf-8"))


def discover_rest() -> list[dict[str, Any]]:
    spec = load_swagger()
    return gfs.iter_openapi_operations(spec)


def discover_grpc() -> list[dict[str, Any]]:
    rows = gfs.parse_all_protos()
    rows.sort(key=lambda x: (x["package"], x["service"], x["method"]))
    return rows


def discover_mqtt() -> list[dict[str, Any]]:
    return gfs.fix_mqtt_rows()


def rest_operation_id(method: str, path: str, op: dict) -> str:
    opid = op.get("operationId")
    if opid:
        return str(opid)
    return "%s:%s" % (method.upper(), path)


def rest_key(method: str, path: str) -> str:
    return "%s %s" % (method.upper(), path)


def grpc_key(row: dict) -> str:
    return row["fullMethod"]


def mqtt_key(row: dict) -> str:
    return "%s|%s" % (row.get("direction", ""), row.get("topicConcrete") or row.get("topicPattern", ""))


def _param_inventory(spec: dict, op: dict, location: str) -> list[dict]:
    out = []
    for par in gfs.iter_resolved_parameters(spec, op):
        if par.get("in") != location:
            continue
        name = par.get("name", "")
        sch = par.get("schema") or {}
        ptype = sch.get("type") or "string"
        ex = gfs.schema_to_example(spec, sch, prop_name=name) if sch else "{{$guid}}"
        out.append(
            {
                "name": name,
                "required": bool(par.get("required", False)),
                "type": ptype,
                "example": ex,
                "description": (par.get("description") or "")[:2000],
            }
        )
    return out


def _header_inventory(spec: dict, op: dict, auth_bearer: bool) -> list[dict]:
    headers = [
        {
            "key": "X-Request-ID",
            "required": False,
            "example": "{{requestId}}",
            "description": "Correlation request id",
        },
        {
            "key": "X-Correlation-ID",
            "required": False,
            "example": "{{correlationId}}",
            "description": "End-to-end correlation id",
        },
        {"key": "Accept", "required": False, "example": "application/json", "description": "Accept JSON"},
    ]
    if auth_bearer:
        headers.append(
            {
                "key": "Authorization",
                "required": True,
                "example": "Bearer {{accessToken}}",
                "description": "JWT bearer token",
            }
        )
    if gfs.operation_needs_idempotency_key(spec, op):
        headers.append(
            {
                "key": "Idempotency-Key",
                "required": True,
                "example": "{{idempotencyKey}}",
                "description": "Write idempotency key",
            }
        )
    for par in gfs.iter_resolved_parameters(spec, op):
        if par.get("in") != "header":
            continue
        hname = par.get("name", "")
        if hname.lower() in ("authorization", "x-request-id", "x-correlation-id", "idempotency-key", "x-idempotency-key"):
            continue
        headers.append(
            {
                "key": hname,
                "required": bool(par.get("required", False)),
                "example": "{{$guid}}",
                "description": (par.get("description") or "")[:1000],
            }
        )
    return headers


def _request_body_inventory(spec: dict, op: dict) -> dict | None:
    rb = op.get("requestBody")
    if not rb:
        return None
    raw = gfs.build_json_request_body(spec, rb)
    example: Any = {}
    if raw:
        try:
            example = json.loads(raw)
        except json.JSONDecodeError:
            example = {}
    schema_src = "docs/swagger/swagger.json"
    for _mk, mo in gfs._iter_json_media_objects(rb):
        ref = (mo.get("schema") or {}).get("$ref")
        if ref:
            schema_src = "openapi:%s" % ref
        break
    return {
        "required": bool(rb.get("required", True)),
        "contentType": "application/json",
        "schemaSource": schema_src,
        "example": example,
    }


def _responses_inventory(spec: dict, op: dict) -> list[dict]:
    resps = []
    for status, resp in sorted((op.get("responses") or {}).items(), key=lambda x: x[0]):
        if not str(status).isdigit():
            continue
        example: Any = {}
        schema_src = "docs/swagger/swagger.json"
        content = resp.get("content") or {}
        for ct, media in content.items():
            if "json" not in ct.lower():
                continue
            ex = media.get("example")
            sch = media.get("schema")
            if ex is not None:
                example = ex
            elif sch:
                example = gfs.schema_to_example(spec, sch)
            ref = (sch or {}).get("$ref")
            if ref:
                schema_src = "openapi:%s" % ref
            break
        resps.append(
            {
                "status": int(status),
                "description": (resp.get("description") or "")[:2000],
                "schemaSource": schema_src,
                "example": example if example is not None else {},
            }
        )
    if not resps:
        resps.append({"status": 200, "description": "Success", "schemaSource": "inferred", "example": {}})
    return resps


def _auth_rest(op: dict) -> dict:
    bearer = gfs.requires_bearer(op)
    sec = op.get("security") or []
    roles: list[str] = []
    for item in sec:
        if isinstance(item, dict):
            for k in item:
                if k not in ("bearerAuth", "BearerAuth"):
                    roles.append(k)
    auth_type = "bearer" if bearer else "none"
    if any("webhook" in str(x).lower() for x in roles):
        auth_type = "webhook-hmac"
    return {"required": bearer, "type": auth_type, "roles": roles}


def _captures_for_path(path: str) -> list[dict]:
    caps = []
    mapping = {
        "machineId": ("machine_id", "machineId"),
        "siteId": ("site_id", "siteId"),
        "orderId": ("order_id", "orderId"),
        "productId": ("product_id", "productId"),
        "commandId": ("command_id", "commandId"),
        "paymentSessionId": ("payment_session_id", "paymentSessionId"),
    }
    pl = path.lower()
    for var, keys in mapping.items():
        if var.replace("Id", "").lower() in pl or var.lower() in pl:
            caps.append({"from": "response body %s" % keys[0], "toVariable": var})
    if "/v1/auth/login" in pl:
        caps.append({"from": "response body access_token", "toVariable": "accessToken"})
        caps.append({"from": "response body refresh_token", "toVariable": "refreshToken"})
    return caps


def build_rest_inventory_items(spec: dict, ops: list[dict]) -> list[dict]:
    items = []
    for row in ops:
        path, method, op = row["path"], row["method"], row["op"]
        tags = op.get("tags") or []
        folder = assign_folder_business(path, method, tags)
        opid = rest_operation_id(method, path, op)
        items.append(
            {
                "id": opid,
                "method": method,
                "path": path,
                "folder": folder,
                "flow": flow_name(folder),
                "operationId": op.get("operationId", ""),
                "summary": op.get("summary", ""),
                "description": (op.get("description") or op.get("summary") or "")[:4000],
                "auth": _auth_rest(op),
                "headers": _header_inventory(spec, op, gfs.requires_bearer(op)),
                "pathParams": _param_inventory(spec, op, "path"),
                "queryParams": _param_inventory(spec, op, "query"),
                "requestBody": _request_body_inventory(spec, op),
                "responses": _responses_inventory(spec, op),
                "captures": _captures_for_path(path),
                "dependsOn": ["01_Auth login"] if gfs.requires_bearer(op) else [],
                "sourceEvidence": [
                    "docs/swagger/swagger.json",
                    "openapi:%s %s" % (method, path),
                ],
            }
        )
    return items


def _grpc_auth(row: dict) -> dict:
    pkg = row.get("package", "")
    machine = "machine" in pkg
    meta = [
        {"key": "authorization", "required": True, "example": "{{machineToken}}" if machine else "{{accessToken}}"},
        {"key": "x-request-id", "required": False, "example": "{{requestId}}"},
    ]
    return {"required": True, "metadata": meta}


def build_grpc_inventory_items(grpc_rows: list[dict], templates: list[dict]) -> list[dict]:
    tmpl_by_fm = {t["fullMethod"]: t for t in templates}
    items = []
    for row in grpc_rows:
        fm = row["fullMethod"]
        tmpl = tmpl_by_fm.get(fm, {})
        pkg = row["package"]
        svc = row["service"]
        method = row["method"]
        folder = "08_Machines_Runtime_Config" if "machine" in pkg else "02_Admin_Accounts_RBAC"
        if "telemetry" in pkg or "device" in method.lower():
            folder = "09_Machines_Telemetry"
        elif "order" in pkg or "commerce" in pkg or "payment" in pkg:
            folder = "12_Orders"
        elif "inventory" in pkg or "planogram" in pkg:
            folder = "10_Inventory"
        items.append(
            {
                "id": "%s.%s.%s" % (pkg, svc, method),
                "protoFile": row.get("protoFile", ""),
                "package": pkg,
                "service": svc,
                "method": method,
                "fullMethod": fm,
                "folder": folder,
                "flow": flow_name(folder),
                "auth": _grpc_auth(row),
                "requestMessage": row.get("requestType", ""),
                "responseMessage": row.get("responseType", ""),
                "requestExample": tmpl.get("requestJsonTemplate") if "requestJsonTemplate" in tmpl else {},
                "responseExample": tmpl.get("expectedResponseShape") or {},
                "errorExamples": [
                    {"code": "Unauthenticated", "example": {"code": 16, "message": "missing or invalid token"}},
                    {"code": "InvalidArgument", "example": {"code": 3, "message": "validation failed"}},
                ],
                "dependsOn": ["machine activation"] if "machine" in pkg else ["admin login"],
                "sourceEvidence": [
                    row.get("protoFile", ""),
                    "proto/%s" % pkg.replace(".", "/"),
                ],
            }
        )
    return items


def _mqtt_folder(row: dict) -> str:
    t = (row.get("topicConcrete") or row.get("topicPattern") or "").lower()
    d = row.get("direction", "")
    if "heartbeat" in t or "presence" in t or "check" in t:
        return "Machine Heartbeat/Check-in"
    if "telemetry" in t:
        return "Telemetry Publish"
    if "command" in t and "ack" in t:
        return "Command ACK"
    if "command" in t:
        return "Backend Commands" if "backend_publishes" in d else "Command ACK"
    if "shadow" in t:
        return "Config/Shadow"
    if "incident" in t:
        return "Diagnostics"
    if "events" in t:
        return "Telemetry Publish"
    return "Connection"


def build_mqtt_inventory_items(mq_rows: list[dict]) -> list[dict]:
    items = []
    for row in mq_rows:
        direction_raw = row.get("direction", "")
        if "machine_publishes" in direction_raw:
            direction = "publish"
            producer, consumer = "machine", "backend"
        elif "backend_publishes" in direction_raw:
            direction = "publish"
            producer, consumer = "backend", "machine"
        else:
            direction = "subscribe"
            producer, consumer = "broker", "backend"
        folder = _mqtt_folder(row)
        ack = row.get("responseOrAckTopic")
        items.append(
            {
                "id": "mqtt-%s-%s" % (row.get("index", 0), folder.replace("/", "-").replace(" ", "-")),
                "topicPattern": row.get("topicPattern", ""),
                "direction": direction,
                "folder": folder,
                "flow": folder,
                "producer": producer,
                "consumer": consumer,
                "qos": row.get("qos"),
                "retain": row.get("retained") if row.get("retained") not in ("unknown_false_subscribe",) else False,
                "auth": {
                    "required": True,
                    "variables": ["mqttUsername", "mqttPassword", "mqttTopicPrefix"],
                },
                "payloadSchemaSource": "internal/platform/mqtt/topics.go",
                "payloadExample": row.get("payloadJsonTemplate") or {},
                "expectedAckTopic": ack or "",
                "expectedAckPayloadExample": {"status": "ack", "command_id": "{{commandId}}"} if ack else {},
                "dependsOn": ["machine provisioning"] if producer == "machine" else ["command dispatch API"],
                "sourceEvidence": [
                    "postman/suites/full-production-suite/generate_full_postman_suite.py:fix_mqtt_rows",
                    "docs/api/mqtt-contract.md",
                ],
            }
        )
    return items


def build_canonical_inventory() -> dict:
    spec = load_swagger()
    rest_ops = discover_rest()
    grpc_rows = discover_grpc()
    templates = gfs.build_grpc_templates(grpc_rows)
    mq_rows = discover_mqtt()
    return {
        "rest": build_rest_inventory_items(spec, rest_ops),
        "grpc": build_grpc_inventory_items(grpc_rows, templates),
        "mqtt": build_mqtt_inventory_items(mq_rows),
        "_meta": {
            "swagger": str(SWAGGER.relative_to(REPO_ROOT)).replace("\\", "/"),
            "restCount": len(rest_ops),
            "grpcCount": len(grpc_rows),
            "mqttCount": len(mq_rows),
        },
    }


def inventory_md(inv: dict) -> str:
    lines = [
        "# AVF API Inventory (Canonical)",
        "",
        "Generated from current code: OpenAPI swagger, proto files, MQTT topic matrix.",
        "",
        "## Counts",
        "",
        "- REST: %s" % len(inv.get("rest", [])),
        "- gRPC: %s" % len(inv.get("grpc", [])),
        "- MQTT: %s" % len(inv.get("mqtt", [])),
        "",
        "## REST",
        "",
    ]
    for r in inv.get("rest", [])[:5]:
        lines.append("- `%s %s` — %s" % (r["method"], r["path"], r["folder"]))
    lines.append("- … (%s total)" % len(inv.get("rest", [])))
    lines += ["", "## gRPC", ""]
    for g in inv.get("grpc", [])[:5]:
        lines.append("- `%s` — %s" % (g["fullMethod"], g["folder"]))
    lines.append("- … (%s total)" % len(inv.get("grpc", [])))
    lines += ["", "## MQTT", ""]
    for m in inv.get("mqtt", []):
        lines.append("- `%s` (%s) — %s" % (m["topicPattern"], m["direction"], m["folder"]))
    return "\n".join(lines) + "\n"

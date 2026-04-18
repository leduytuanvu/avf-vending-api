#!/usr/bin/env python3
"""
Generate docs/swagger/swagger.json (OpenAPI 2.0) from swag-style comments in:
  - cmd/api/main.go (general API metadata: @title, @version, ...)
  - internal/httpserver/swagger_operations.go (@Router, @Summary, ...)

Run from repository root:  python3 tools/build_openapi.py
Used by: make swagger
"""
from __future__ import annotations

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
MAIN_GO = ROOT / "cmd" / "api" / "main.go"
OPS_GO = ROOT / "internal" / "httpserver" / "swagger_operations.go"
OUT_DIR = ROOT / "docs" / "swagger"
OUT_JSON = OUT_DIR / "swagger.json"
OUT_DOCS_GO = OUT_DIR / "docs.go"


def parse_general_info(src: str) -> dict[str, str]:
    out: dict[str, str] = {}
    for line in src.splitlines():
        s = line.strip()
        if not s.startswith("// @"):
            continue
        s = s[3:].strip()  # drop //
        if not s.startswith("@"):
            continue
        key, _, rest = s[1:].partition(" ")
        out[key] = rest.strip()
    return out


def extract_doc_blocks(ops_src: str) -> list[tuple[str, str]]:
    """Return (func_name, comment_block_text) for each `func DocOp...() {}`."""
    out: list[tuple[str, str]] = []
    for m in re.finditer(r"^func (DocOp\w+)\(\) \{\}\s*$", ops_src, re.MULTILINE):
        name = m.group(1)
        pre = ops_src[: m.start()]
        lines = pre.splitlines()
        block: list[str] = []
        i = len(lines) - 1
        while i >= 0:
            raw = lines[i]
            stripped = raw.lstrip()
            if stripped.startswith("//"):
                block.append(raw)
                i -= 1
                continue
            if stripped == "":
                i -= 1
                continue
            break
        block.reverse()
        out.append((name, "\n".join(block)))
    return out


def parse_op_directives(block: str) -> dict[str, list[str]]:
    d: dict[str, list[str]] = {}
    for line in block.splitlines():
        s = line.strip()
        if not s.startswith("//"):
            continue
        s = s[2:].strip()
        if not s.startswith("@"):
            continue
        rest = s[1:].strip()  # drop leading @
        key, _, val = rest.partition(" ")
        key = key.strip()
        val = val.strip()
        d.setdefault(key, []).append(val)
    return d


def parse_router(line: str) -> tuple[str, str] | None:
    # "/v1/foo/{id} [get]" or full line with @Router
    m = re.search(r"(?:@Router\s+)?(\S+)\s+\[(\w+)\]", line)
    if not m:
        return None
    return m.group(1), m.group(2).lower()


def swagger_schema_from_ref(ref: str) -> dict:
    # ref like "{object} V1StandardError" or "{string} string"
    m = re.match(r"\{(\w+)\}\s+(\S+)", ref.strip())
    if not m:
        return {"type": "object"}
    typ, name = m.group(1), m.group(2)
    if typ == "string":
        return {"type": "string"}
    if typ == "object" and name == "object":
        return {"type": "object"}
    return {"$ref": "#/definitions/" + name}


def build_parameters(param_lines: list[str]) -> list[dict]:
    params: list[dict] = []
    for pl in param_lines:
        # @Param machineId path string true "desc"
        parts = pl.split()
        if len(parts) < 5:
            continue
        name, where, typ = parts[0], parts[1], parts[2]
        required = parts[3] == "true"
        desc = " ".join(parts[4:]).strip("\"")
        p: dict = {"name": name, "in": where, "required": required, "description": desc}
        if typ == "string":
            p["type"] = "string"
        elif typ == "int":
            p["type"] = "integer"
        elif typ == "body":
            p["schema"] = {"type": "object"}
        else:
            p["type"] = "string"
        params.append(p)
    return params


def build_operation(d: dict[str, list[str]]) -> tuple[str, str, dict] | None:
    router_line = None
    for v in d.get("Router", []):
        router_line = v
        break
    if not router_line:
        return None
    pr = parse_router(router_line)
    if not pr:
        return None
    path, method = pr

    op: dict = {}
    if d.get("Summary"):
        op["summary"] = d["Summary"][0]
    if d.get("Description"):
        op["description"] = " ".join(d["Description"])
    if d.get("Tags"):
        op["tags"] = [t.strip() for t in d["Tags"][0].split(",")]
    consumes = d.get("Accept", [])
    produces = d.get("Produce", [])
    if not consumes:
        consumes = ["application/json"]
    if not produces:
        produces = ["application/json"]
    op["consumes"] = consumes
    op["produces"] = produces

    params = build_parameters(d.get("Param", []))
    if params:
        op["parameters"] = params

    if d.get("Security"):
        op["security"] = [{"BearerAuth": []}]

    responses: dict[str, dict] = {}
    for succ in d.get("Success", []):
        # 200 {object} V1StandardError
        # 200 {string} string "ok"
        m = re.match(r"(\d+)\s+(\{[^}]+\}\s+\S+)", succ)
        if not m:
            continue
        code, ref = m.group(1), m.group(2)
        desc = d.get("Summary", [""])[0]
        if code == "200" and "string" in ref:
            responses[code] = {"description": desc, "schema": {"type": "string", "example": "ok"}}
        else:
            responses[code] = {"description": desc, "schema": swagger_schema_from_ref(ref)}
    for fail in d.get("Failure", []):
        m = re.match(r"(\d+)\s+(\{[^}]+\}\s+\S+)", fail)
        if not m:
            continue
        code, ref = m.group(1), m.group(2)
        if ref.strip().startswith("{string}"):
            responses[code] = {"description": "error", "schema": {"type": "string"}}
        else:
            responses[code] = {"description": "error", "schema": swagger_schema_from_ref(ref)}
    if responses:
        op["responses"] = responses

    return path, method, op


def definitions() -> dict:
    return {
        "V1StandardError": {
            "type": "object",
            "properties": {
                "error": {
                    "type": "object",
                    "properties": {
                        "code": {"type": "string", "example": "invalid_json"},
                        "message": {"type": "string"},
                    },
                    "required": ["code", "message"],
                }
            },
            "required": ["error"],
        },
        "V1NotImplementedError": {
            "type": "object",
            "properties": {
                "error": {
                    "type": "object",
                    "properties": {
                        "code": {"type": "string", "example": "not_implemented"},
                        "message": {"type": "string"},
                        "capability": {"type": "string"},
                        "implemented": {"type": "boolean", "example": False},
                    },
                    "required": ["code", "message", "capability", "implemented"],
                }
            },
            "required": ["error"],
        },
        "V1CapabilityNotConfiguredError": {
            "type": "object",
            "properties": {
                "error": {
                    "type": "object",
                    "properties": {
                        "code": {"type": "string", "example": "capability_not_configured"},
                        "message": {"type": "string"},
                        "capability": {"type": "string"},
                        "implemented": {"type": "boolean", "example": False},
                    },
                    "required": ["code", "message", "capability", "implemented"],
                }
            },
            "required": ["error"],
        },
        "V1BearerAuthError": {
            "type": "object",
            "properties": {
                "error": {
                    "type": "object",
                    "properties": {"message": {"type": "string", "example": "unauthenticated"}},
                    "required": ["message"],
                }
            },
            "required": ["error"],
        },
        "V1ListViewEnvelope": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {}},
                "meta": {},
            },
            "required": ["items"],
        },
        "V1OperatorListEnvelope": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {}},
                "meta": {
                    "type": "object",
                    "properties": {
                        "limit": {"type": "integer", "format": "int32"},
                        "returned": {"type": "integer"},
                    },
                    "required": ["limit", "returned"],
                },
            },
            "required": ["items", "meta"],
        },
        "V1OperatorSessionEnvelope": {
            "type": "object",
            "properties": {"session": {"type": "object"}},
            "required": ["session"],
        },
        "V1OperatorCurrentEnvelope": {
            "type": "object",
            "properties": {
                "active_session": {"description": "null when no ACTIVE session"},
                "technician_display_name": {"type": "string"},
            },
        },
        "V1CommerceCreateOrderResponse": {
            "type": "object",
            "properties": {
                "order_id": {"type": "string", "format": "uuid"},
                "vend_session_id": {"type": "string", "format": "uuid"},
                "replay": {"type": "boolean"},
                "order_status": {"$ref": "#/definitions/V1CommerceOrderStatus"},
                "vend_state": {"$ref": "#/definitions/V1VendSessionState"},
            },
            "required": [
                "order_id",
                "vend_session_id",
                "replay",
                "order_status",
                "vend_state",
            ],
        },
        "V1CommerceOrderStatus": {
            "type": "string",
            "description": "Persisted order.status CHECK (see migrations).",
            "enum": [
                "created",
                "quoted",
                "paid",
                "vending",
                "completed",
                "failed",
                "cancelled",
            ],
        },
        "V1VendSessionState": {
            "type": "string",
            "enum": ["pending", "in_progress", "success", "failed"],
        },
        "V1PaymentState": {
            "type": "string",
            "description": "Normalized payment.state for commerce APIs.",
            "enum": ["created", "authorized", "captured", "failed", "refunded"],
        },
        "V1OperatorSessionStatus": {
            "type": "string",
            "enum": ["ACTIVE", "ENDED", "EXPIRED", "REVOKED"],
        },
        "V1OperatorActorType": {
            "type": "string",
            "enum": ["TECHNICIAN", "USER"],
        },
        "V1OperatorAuthMethod": {
            "type": "string",
            "enum": ["pin", "password", "badge", "oidc", "device_cert", "unknown"],
        },
    }


# Every HTTP method/path the Chi router can register for the public API (see internal/httpserver/server.go).
# Fails generation if an operation is missing from swagger_operations.go — keeps docs aligned with wiring.
REQUIRED_OPERATIONS: list[tuple[str, str]] = [
    ("get", "/health/live"),
    ("get", "/health/ready"),
    ("get", "/metrics"),
    ("get", "/swagger/doc.json"),
    ("get", "/swagger/index.html"),
    ("get", "/v1/admin/machines"),
    ("get", "/v1/admin/technicians"),
    ("get", "/v1/admin/assignments"),
    ("get", "/v1/admin/commands"),
    ("get", "/v1/admin/ota"),
    ("post", "/v1/admin/organizations/{orgId}/artifacts"),
    ("get", "/v1/admin/organizations/{orgId}/artifacts"),
    ("get", "/v1/admin/organizations/{orgId}/artifacts/{artifactId}"),
    ("get", "/v1/admin/organizations/{orgId}/artifacts/{artifactId}/download"),
    ("put", "/v1/admin/organizations/{orgId}/artifacts/{artifactId}/content"),
    ("delete", "/v1/admin/organizations/{orgId}/artifacts/{artifactId}"),
    ("get", "/v1/operator-insights/technicians/{technicianId}/action-attributions"),
    ("get", "/v1/operator-insights/users/action-attributions"),
    ("get", "/v1/payments"),
    ("get", "/v1/orders"),
    ("get", "/v1/machines/{machineId}/shadow"),
    ("get", "/v1/machines/{machineId}/telemetry/snapshot"),
    ("get", "/v1/machines/{machineId}/telemetry/incidents"),
    ("get", "/v1/machines/{machineId}/telemetry/rollups"),
    ("get", "/v1/machines/{machineId}/commands/receipts"),
    ("get", "/v1/machines/{machineId}/commands/{sequence}/status"),
    ("post", "/v1/machines/{machineId}/commands/dispatch"),
    ("get", "/v1/machines/{machineId}/operator-sessions/current"),
    ("get", "/v1/machines/{machineId}/operator-sessions/history"),
    ("get", "/v1/machines/{machineId}/operator-sessions/auth-events"),
    ("get", "/v1/machines/{machineId}/operator-sessions/action-attributions"),
    ("get", "/v1/machines/{machineId}/operator-sessions/timeline"),
    ("post", "/v1/machines/{machineId}/operator-sessions/login"),
    ("post", "/v1/machines/{machineId}/operator-sessions/logout"),
    ("post", "/v1/machines/{machineId}/operator-sessions/{sessionId}/heartbeat"),
    ("post", "/v1/commerce/orders"),
    ("post", "/v1/commerce/orders/{orderId}/payment-session"),
    ("get", "/v1/commerce/orders/{orderId}"),
    ("get", "/v1/commerce/orders/{orderId}/reconciliation"),
    ("post", "/v1/commerce/orders/{orderId}/payments/{paymentId}/webhooks"),
    ("post", "/v1/commerce/orders/{orderId}/vend/start"),
    ("post", "/v1/commerce/orders/{orderId}/vend/success"),
    ("post", "/v1/commerce/orders/{orderId}/vend/failure"),
]


def verify_paths(paths: dict[str, dict]) -> list[str]:
    missing: list[str] = []
    for method, path in REQUIRED_OPERATIONS:
        entry = paths.get(path)
        if not entry or method not in entry:
            missing.append(f"{method.upper()} {path}")
    return missing


def main() -> int:
    if not MAIN_GO.is_file() or not OPS_GO.is_file():
        print("expected cmd/api/main.go and internal/httpserver/swagger_operations.go", file=sys.stderr)
        return 1

    gen = parse_general_info(MAIN_GO.read_text(encoding="utf-8"))
    info = {
        "title": gen.get("title", "AVF Vending HTTP API"),
        "version": gen.get("version", "1.0"),
        "description": gen.get("description", ""),
        "termsOfService": gen.get("termsOfService", ""),
        "contact": {"name": gen.get("contact.name", ""), "url": gen.get("contact.url", "")},
        "license": {"name": gen.get("license.name", "")},
    }

    paths: dict[str, dict] = {}
    for _name, block in extract_doc_blocks(OPS_GO.read_text(encoding="utf-8")):
        d = parse_op_directives(block)
        built = build_operation(d)
        if not built:
            continue
        path, method, op = built
        paths.setdefault(path, {})[method] = op

    miss = verify_paths(paths)
    if miss:
        print("swagger route coverage: missing operations:", file=sys.stderr)
        for m in miss:
            print(" ", m, file=sys.stderr)
        return 1

    spec = {
        "swagger": "2.0",
        "info": info,
        "host": gen.get("host", "localhost:8080"),
        "basePath": gen.get("BasePath", "/"),
        "schemes": [s.strip() for s in gen.get("schemes", "http https").split() if s.strip()],
        "paths": paths,
        "definitions": definitions(),
        "securityDefinitions": {
            "BearerAuth": {
                "type": "apiKey",
                "name": "Authorization",
                "in": "header",
                "description": 'Send: Bearer <JWT>. JWT mode is selected by HTTP_AUTH_MODE (hs256 default, rs256_pem, rs256_jwks). 401/403 responses from this layer use JSON {"error":{"message":"..."}} without error.code.',
            }
        },
        "tags": [
            {"name": "Health", "description": "Liveness/readiness without Bearer auth."},
            {"name": "Reliability", "description": "Operational endpoints such as Prometheus metrics when enabled."},
            {"name": "Admin", "description": "Platform/org administrator collection APIs."},
            {"name": "Artifacts", "description": "S3-backed artifact APIs (mounted only when API_ARTIFACTS_ENABLED=true)."},
            {"name": "Operator", "description": "Operator sessions and cross-machine insights."},
            {"name": "Commerce", "description": "Checkout, payments, vend lifecycle (organization-scoped)."},
            {"name": "Fleet", "description": "Machine shadow and fleet reads."},
            {"name": "Device", "description": "Remote command dispatch and receipts (requires MQTT wiring for dispatch)."},
            {
                "name": "Documentation",
                "description": "Embedded Swagger UI and OpenAPI JSON (no Bearer auth; mounted when HTTP_SWAGGER_UI_ENABLED).",
            },
        ],
    }

    OUT_DIR.mkdir(parents=True, exist_ok=True)
    OUT_JSON.write_text(json.dumps(spec, indent=2, sort_keys=True) + "\n", encoding="utf-8")

    data = json.loads(OUT_JSON.read_text(encoding="utf-8"))
    title = data["info"]["title"].replace("\\", "\\\\").replace('"', '\\"')
    ver = data["info"]["version"]
    docs_go = f'''// Package swagger contains the OpenAPI 2.0 document for the HTTP API (generated).
//
// Code generated by tools/build_openapi.py; DO NOT EDIT manually.
//go:generate python3 tools/build_openapi.py
package swagger

import (
	_ "embed"

	"github.com/swaggo/swag"
)

//go:embed swagger.json
var swaggerJSON []byte

func init() {{
	swag.Register("swagger", &swag.Spec{{
		Version:          "{ver}",
		Host:             "",
		BasePath:         "/",
		Schemes:          []string{{"http", "https"}},
		Title:            "{title}",
		Description:      "OpenAPI 2.0 embedded as swagger.json (generated by tools/build_openapi.py).",
		InfoInstanceName: "swagger",
		SwaggerTemplate:  string(swaggerJSON),
	}})
}}
'''
    OUT_DOCS_GO.write_text(docs_go, encoding="utf-8")
    print("wrote", OUT_JSON, "and", OUT_DOCS_GO)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

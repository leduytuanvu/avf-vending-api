#!/usr/bin/env python3
"""
Generate docs/swagger/swagger.json (OpenAPI 3.0) from swag-style comments in:
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
from typing import Any

_TOOLS_DIR = Path(__file__).resolve().parent
if str(_TOOLS_DIR) not in sys.path:
    sys.path.insert(0, str(_TOOLS_DIR))
from openapi_refs import unresolved_local_refs

ROOT = Path(__file__).resolve().parents[1]
MAIN_GO = ROOT / "cmd" / "api" / "main.go"
OPS_GO = ROOT / "internal" / "httpserver" / "swagger_operations.go"
OUT_DIR = ROOT / "docs" / "swagger"
OUT_JSON = OUT_DIR / "swagger.json"
OUT_DOCS_GO = OUT_DIR / "docs.go"

# Example UUIDs (documentation only)
_U = "3fa85f64-5717-4562-b3fc-2c963f66afa6"
_U2 = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
_U3 = "7c9e6679-7425-40de-944b-e07fc1f90ae7"

# Shared OpenAPI schema for API timestamps (handlers emit time.RFC3339Nano in UTC).
_TS_SCHEMA: dict[str, Any] = {
    "type": "string",
    "format": "date-time",
    "description": "RFC3339 with fractional seconds and explicit timezone offset (RFC3339Nano). Responses use UTC (Z).",
}


def parse_general_info(src: str) -> dict[str, str]:
    out: dict[str, str] = {}
    desc_parts: list[str] = []
    for line in src.splitlines():
        s = line.strip()
        if not s.startswith("// @"):
            continue
        s = s[3:].strip()  # drop //
        if not s.startswith("@"):
            continue
        key, _, rest = s[1:].partition(" ")
        key = key.strip()
        rest = rest.strip()
        if key == "description":
            desc_parts.append(rest)
            continue
        out[key] = rest
    if desc_parts:
        out["description"] = "\n\n".join(desc_parts)
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
    m = re.search(r"(?:@Router\s+)?(\S+)\s+\[(\w+)\]", line)
    if not m:
        return None
    return m.group(1), m.group(2).lower()


def oas_schema_from_ref(ref: str) -> dict[str, Any]:
    m = re.match(r"\{(\w+)\}\s+(\S+)", ref.strip())
    if not m:
        return {"type": "object"}
    typ, name = m.group(1), m.group(2)
    if typ == "string":
        return {"type": "string"}
    if typ == "object" and name == "object":
        return {"type": "object"}
    return {"$ref": "#/components/schemas/" + name}


def v1_error_example(
    code: str,
    message: str,
    details: dict[str, Any] | None = None,
    request_id: str = "01ARZ3NDEKTSV4RRFFQ69G5FAV",
) -> dict[str, Any]:
    d: dict[str, Any] = details if details is not None else {}
    return {"error": {"code": code, "message": message, "details": d, "requestId": request_id}}


def json_response(description: str, schema: dict[str, Any], example: Any | None = None) -> dict[str, Any]:
    media: dict[str, Any] = {"schema": schema}
    if example is not None:
        media["example"] = example
    return {"description": description, "content": {"application/json": media}}


def text_plain_response(description: str, example: str = "ok") -> dict[str, Any]:
    return {
        "description": description,
        "content": {"text/plain": {"schema": {"type": "string", "example": example}}},
    }


def split_swagger_params(param_lines: list[str]) -> tuple[list[dict[str, Any]], dict[str, Any] | None]:
    """Split @Param lines into OAS3 query/path/header params and optional requestBody (body param)."""
    params: list[dict[str, Any]] = []
    body: dict[str, Any] | None = None
    for pl in param_lines:
        parts = pl.split()
        if len(parts) < 5:
            continue
        name, where, typ = parts[0], parts[1], parts[2]
        required = parts[3] == "true"
        desc = " ".join(parts[4:]).strip('"')
        if where == "body":
            sch: dict[str, Any] = {"type": "object"}
            if typ != "object":
                sch = {"type": "string"}
            body = {
                "required": required,
                "description": desc,
                "content": {"application/json": {"schema": sch}},
            }
            continue
        schema: dict[str, Any] = {"type": "string"}
        if typ == "int":
            schema = {"type": "integer", "format": "int32"}
        params.append(
            {
                "name": name,
                "in": where,
                "required": required,
                "description": desc,
                "schema": schema,
            }
        )
    return params, body


def build_operation_oas3(d: dict[str, list[str]]) -> tuple[str, str, dict[str, Any]] | None:
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

    op: dict[str, Any] = {}
    if d.get("Summary"):
        op["summary"] = d["Summary"][0]
    if d.get("Description"):
        op["description"] = " ".join(d["Description"])
    if d.get("Tags"):
        op["tags"] = [t.strip() for t in d["Tags"][0].split(",")]

    consumes = d.get("Accept", []) or ["application/json"]
    produces = d.get("Produce", []) or ["application/json"]

    path_params, request_body = split_swagger_params(d.get("Param", []))
    if path_params:
        op["parameters"] = path_params
    if request_body is not None:
        op["requestBody"] = request_body

    if d.get("Security"):
        op["security"] = [{"bearerAuth": []}]

    summary = d.get("Summary", [""])[0]
    responses: dict[str, Any] = {}
    for succ in d.get("Success", []):
        m = re.match(r"(\d+)\s+(\{[^}]+\}\s+\S+)", succ)
        if not m:
            continue
        code, ref = m.group(1), m.group(2)
        if code == "200" and "{string}" in ref:
            responses[code] = text_plain_response(summary or "ok", "ok")
        elif "application/json" in produces or "{object}" in ref or "{string}" not in ref:
            responses[code] = json_response(summary or "success", oas_schema_from_ref(ref))
        else:
            responses[code] = text_plain_response(summary or "ok", "ok")

    for fail in d.get("Failure", []):
        m = re.match(r"(\d+)\s+(\{[^}]+\}\s+\S+)", fail)
        if not m:
            continue
        code, ref = m.group(1), m.group(2)
        if ref.strip().startswith("{string}"):
            responses[code] = text_plain_response("error", "error")
        else:
            responses[code] = json_response("error", oas_schema_from_ref(ref))
    if responses:
        op["responses"] = responses

    return path, method, op


def operational_collection_component_schemas() -> dict[str, Any]:
    """OpenAPI component schemas for operational GET list endpoints (admin fleet + tenant commerce)."""
    meta = {
        "type": "object",
        "properties": {
            "limit": {"type": "integer", "format": "int32"},
            "offset": {"type": "integer", "format": "int32"},
            "returned": {"type": "integer"},
            "total": {"type": "integer", "format": "int64"},
        },
        "required": ["limit", "offset", "returned", "total"],
    }
    uuid_s = {"type": "string", "format": "uuid"}
    ts = dict(_TS_SCHEMA)
    return {
        "V1CollectionListMeta": meta,
        "V1OrderListItem": {
            "type": "object",
            "properties": {
                "orderId": uuid_s,
                "organizationId": uuid_s,
                "machineId": uuid_s,
                "status": {"$ref": "#/components/schemas/V1CommerceOrderStatus"},
                "currency": {"type": "string", "minLength": 3, "maxLength": 3},
                "subtotalMinor": {"type": "integer", "format": "int64"},
                "taxMinor": {"type": "integer", "format": "int64"},
                "totalMinor": {"type": "integer", "format": "int64"},
                "idempotencyKey": {"type": "string"},
                "createdAt": ts,
                "updatedAt": ts,
            },
            "required": [
                "orderId",
                "organizationId",
                "machineId",
                "status",
                "currency",
                "subtotalMinor",
                "taxMinor",
                "totalMinor",
                "createdAt",
                "updatedAt",
            ],
        },
        "V1OrdersListResponse": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {"$ref": "#/components/schemas/V1OrderListItem"}},
                "meta": {"$ref": "#/components/schemas/V1CollectionListMeta"},
            },
            "required": ["items", "meta"],
        },
        "V1PaymentListItem": {
            "type": "object",
            "properties": {
                "paymentId": uuid_s,
                "orderId": uuid_s,
                "organizationId": uuid_s,
                "machineId": uuid_s,
                "provider": {"type": "string"},
                "paymentState": {"$ref": "#/components/schemas/V1PaymentState"},
                "orderStatus": {"$ref": "#/components/schemas/V1CommerceOrderStatus"},
                "amountMinor": {"type": "integer", "format": "int64"},
                "currency": {"type": "string"},
                "reconciliationStatus": {"type": "string"},
                "settlementStatus": {"type": "string"},
                "createdAt": ts,
                "updatedAt": ts,
            },
            "required": [
                "paymentId",
                "orderId",
                "organizationId",
                "machineId",
                "provider",
                "paymentState",
                "orderStatus",
                "amountMinor",
                "currency",
                "reconciliationStatus",
                "settlementStatus",
                "createdAt",
                "updatedAt",
            ],
        },
        "V1PaymentsListResponse": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {"$ref": "#/components/schemas/V1PaymentListItem"}},
                "meta": {"$ref": "#/components/schemas/V1CollectionListMeta"},
            },
            "required": ["items", "meta"],
        },
        "V1CommerceReconciliationCase": {
            "type": "object",
            "properties": {
                "id": uuid_s,
                "organizationId": uuid_s,
                "caseType": {"type": "string"},
                "status": {"type": "string", "enum": ["open", "reviewing", "resolved", "dismissed", "ignored", "escalated"]},
                "severity": {"type": "string", "enum": ["info", "warning", "critical"]},
                "orderId": uuid_s,
                "paymentId": uuid_s,
                "vendSessionId": uuid_s,
                "refundId": uuid_s,
                "provider": {"type": "string"},
                "providerEventId": {"type": "integer", "format": "int64"},
                "reason": {"type": "string"},
                "metadata": {"type": "object", "additionalProperties": True},
                "firstDetectedAt": ts,
                "lastDetectedAt": ts,
                "resolvedAt": ts,
                "resolvedBy": uuid_s,
                "resolutionNote": {"type": "string"},
            },
            "required": [
                "id",
                "organizationId",
                "caseType",
                "status",
                "severity",
                "reason",
                "metadata",
                "firstDetectedAt",
                "lastDetectedAt",
            ],
        },
        "V1CommerceReconciliationListResponse": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {"$ref": "#/components/schemas/V1CommerceReconciliationCase"}},
                "meta": {"$ref": "#/components/schemas/V1CollectionListMeta"},
            },
            "required": ["items", "meta"],
        },
        "V1CommerceReconciliationResolveRequest": {
            "type": "object",
            "properties": {
                "status": {"type": "string", "enum": ["resolved", "dismissed", "ignored", "escalated"]},
                "note": {"type": "string"},
            },
            "required": ["status"],
        },
        "V1CommerceReconciliationIgnoreRequest": {
            "type": "object",
            "properties": {"note": {"type": "string"}},
        },
        "V1OrderTimelineEvent": {
            "type": "object",
            "properties": {
                "id": uuid_s,
                "eventType": {"type": "string"},
                "actorType": {"type": "string"},
                "actorId": {"type": "string"},
                "payload": {"type": "object", "additionalProperties": True},
                "occurredAt": ts,
                "createdAt": ts,
            },
            "required": ["id", "eventType", "actorType", "payload", "occurredAt", "createdAt"],
        },
        "V1OrderTimelineListResponse": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {"$ref": "#/components/schemas/V1OrderTimelineEvent"}},
                "meta": {"$ref": "#/components/schemas/V1CollectionListMeta"},
            },
            "required": ["items", "meta"],
        },
        "V1RefundRequestRow": {
            "type": "object",
            "properties": {
                "id": uuid_s,
                "organizationId": uuid_s,
                "orderId": uuid_s,
                "paymentId": uuid_s,
                "refundId": uuid_s,
                "amountMinor": {"type": "integer", "format": "int64"},
                "currency": {"type": "string"},
                "status": {"type": "string"},
                "reason": {"type": "string"},
                "providerRefundId": {"type": "string"},
                "requestedBy": uuid_s,
                "approvedBy": uuid_s,
                "idempotencyKey": {"type": "string"},
                "createdAt": ts,
                "updatedAt": ts,
                "completedAt": ts,
            },
            "required": [
                "id",
                "organizationId",
                "orderId",
                "amountMinor",
                "currency",
                "status",
                "createdAt",
                "updatedAt",
            ],
        },
        "V1RefundRequestsListResponse": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {"$ref": "#/components/schemas/V1RefundRequestRow"}},
                "meta": {"$ref": "#/components/schemas/V1CollectionListMeta"},
            },
            "required": ["items", "meta"],
        },
        "V1AdminOrderRefundPostRequest": {
            "type": "object",
            "properties": {
                "amountMinor": {"type": "integer", "format": "int64"},
                "currency": {"type": "string"},
                "reason": {"type": "string"},
            },
        },
        "V1AdminOrderRefundPostResponse": {
            "type": "object",
            "properties": {
                "refundRequest": {"$ref": "#/components/schemas/V1RefundRequestRow"},
                "ledgerRefundId": uuid_s,
                "ledgerState": {"type": "string"},
                "ledgerAmountMinor": {"type": "integer", "format": "int64"},
                "ledgerCurrency": {"type": "string"},
            },
            "required": ["refundRequest", "ledgerRefundId", "ledgerState", "ledgerAmountMinor", "ledgerCurrency"],
        },
        "V1AdminMachineInventorySummary": {
            "type": "object",
            "properties": {
                "totalSlots": {"type": "integer", "format": "int64"},
                "occupiedSlots": {"type": "integer", "format": "int64"},
                "lowStockSlots": {"type": "integer", "format": "int64"},
                "outOfStockSlots": {"type": "integer", "format": "int64"},
            },
            "required": ["totalSlots", "occupiedSlots", "lowStockSlots", "outOfStockSlots"],
        },
        "V1AdminAssignedTechnician": {
            "type": "object",
            "properties": {
                "technicianId": uuid_s,
                "displayName": {"type": "string"},
                "role": {"type": "string"},
                "validFrom": ts,
                "validTo": ts,
            },
            "required": ["technicianId", "displayName", "role", "validFrom"],
        },
        "V1AdminCurrentOperator": {
            "type": "object",
            "properties": {
                "sessionId": uuid_s,
                "actorType": {"type": "string"},
                "technicianId": uuid_s,
                "technicianDisplayName": {"type": "string"},
                "userPrincipal": {"type": "string"},
                "sessionStartedAt": ts,
                "sessionStatus": {"type": "string"},
                "sessionExpiresAt": ts,
            },
            "required": ["sessionId", "actorType", "sessionStartedAt", "sessionStatus"],
        },
        "V1AdminMachineListItem": {
            "type": "object",
            "properties": {
                "machineId": uuid_s,
                "machineName": {"type": "string"},
                "organizationId": uuid_s,
                "siteId": uuid_s,
                "siteName": {"type": "string"},
                "hardwareProfileId": uuid_s,
                "serialNumber": {"type": "string"},
                "name": {"type": "string"},
                "status": {"type": "string"},
                "commandSequence": {"type": "integer", "format": "int64"},
                "createdAt": ts,
                "updatedAt": ts,
                "androidId": {"type": "string"},
                "simSerial": {"type": "string"},
                "simIccid": {"type": "string"},
                "appVersion": {"type": "string"},
                "firmwareVersion": {"type": "string"},
                "lastHeartbeatAt": ts,
                "effectiveTimezone": {"type": "string"},
                "assignedTechnicians": {
                    "type": "array",
                    "items": {"$ref": "#/components/schemas/V1AdminAssignedTechnician"},
                },
                "currentOperator": {
                    "nullable": True,
                    "allOf": [{"$ref": "#/components/schemas/V1AdminCurrentOperator"}],
                },
                "inventorySummary": {"$ref": "#/components/schemas/V1AdminMachineInventorySummary"},
            },
            "required": [
                "machineId",
                "machineName",
                "organizationId",
                "siteId",
                "siteName",
                "serialNumber",
                "name",
                "status",
                "commandSequence",
                "createdAt",
                "updatedAt",
                "effectiveTimezone",
                "assignedTechnicians",
                "inventorySummary",
            ],
        },
        "V1MachineTelemetrySnapshotResponse": {
            "type": "object",
            "properties": {
                "machineId": uuid_s,
                "organizationId": uuid_s,
                "siteId": uuid_s,
                "reportedState": {"type": "object", "additionalProperties": True},
                "metricsState": {"type": "object", "additionalProperties": True},
                "lastHeartbeatAt": ts,
                "appVersion": {"type": "string"},
                "firmwareVersion": {"type": "string"},
                "updatedAt": ts,
                "androidId": {"type": "string"},
                "simSerial": {"type": "string"},
                "simIccid": {"type": "string"},
                "deviceModel": {"type": "string"},
                "osVersion": {"type": "string"},
                "lastIdentityAt": ts,
                "effectiveTimezone": {
                    "type": "string",
                    "description": "IANA zone name for business-local interpretation alongside UTC timestamps.",
                },
            },
            "required": [
                "machineId",
                "organizationId",
                "siteId",
                "reportedState",
                "metricsState",
                "updatedAt",
                "effectiveTimezone",
            ],
        },
        "V1MachineTelemetryIncidentItem": {
            "type": "object",
            "properties": {
                "id": uuid_s,
                "severity": {"type": "string"},
                "code": {"type": "string"},
                "title": {"type": "string"},
                "detail": {"type": "object", "additionalProperties": True},
                "dedupeKey": {"type": "string"},
                "openedAt": ts,
                "updatedAt": ts,
            },
            "required": ["id", "severity", "code", "detail", "openedAt", "updatedAt"],
        },
        "V1MachineTelemetryIncidentsMeta": {
            "type": "object",
            "properties": {
                "limit": {"type": "integer", "format": "int32"},
                "returned": {"type": "integer"},
            },
            "required": ["limit", "returned"],
        },
        "V1MachineTelemetryIncidentsResponse": {
            "type": "object",
            "properties": {
                "items": {
                    "type": "array",
                    "items": {"$ref": "#/components/schemas/V1MachineTelemetryIncidentItem"},
                },
                "meta": {"$ref": "#/components/schemas/V1MachineTelemetryIncidentsMeta"},
            },
            "required": ["items", "meta"],
        },
        "V1MachineTelemetryRollupItem": {
            "type": "object",
            "properties": {
                "bucketStart": ts,
                "granularity": {"type": "string"},
                "metricKey": {"type": "string"},
                "sampleCount": {"type": "integer", "format": "int64"},
                "sum": {"type": "number", "format": "double", "nullable": True},
                "min": {"type": "number", "format": "double", "nullable": True},
                "max": {"type": "number", "format": "double", "nullable": True},
                "last": {"type": "number", "format": "double", "nullable": True},
                "extra": {"type": "object", "additionalProperties": True},
            },
            "required": ["bucketStart", "granularity", "metricKey", "sampleCount", "extra"],
        },
        "V1MachineTelemetryRollupsMeta": {
            "type": "object",
            "properties": {
                "granularity": {"type": "string"},
                "from": ts,
                "to": ts,
                "returned": {"type": "integer"},
                "note": {"type": "string"},
            },
            "required": ["granularity", "from", "to", "returned", "note"],
        },
        "V1MachineTelemetryRollupsResponse": {
            "type": "object",
            "properties": {
                "items": {
                    "type": "array",
                    "items": {"$ref": "#/components/schemas/V1MachineTelemetryRollupItem"},
                },
                "meta": {"$ref": "#/components/schemas/V1MachineTelemetryRollupsMeta"},
            },
            "required": ["items", "meta"],
        },
        "V1AdminMachinesListResponse": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminMachineListItem"}},
                "meta": {"$ref": "#/components/schemas/V1CollectionListMeta"},
            },
            "required": ["items", "meta"],
        },
        "V1AdminTechnicianListItem": {
            "type": "object",
            "properties": {
                "technicianId": uuid_s,
                "organizationId": uuid_s,
                "displayName": {"type": "string"},
                "email": {"type": "string"},
                "phone": {"type": "string"},
                "externalSubject": {"type": "string"},
                "createdAt": ts,
            },
            "required": ["technicianId", "organizationId", "displayName", "createdAt"],
        },
        "V1AdminTechniciansListResponse": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminTechnicianListItem"}},
                "meta": {"$ref": "#/components/schemas/V1CollectionListMeta"},
            },
            "required": ["items", "meta"],
        },
        "V1AdminAssignmentListItem": {
            "type": "object",
            "properties": {
                "assignmentId": uuid_s,
                "technicianId": uuid_s,
                "technicianDisplayName": {"type": "string"},
                "machineId": uuid_s,
                "machineName": {"type": "string"},
                "machineSerialNumber": {"type": "string"},
                "role": {"type": "string"},
                "validFrom": ts,
                "validTo": ts,
                "createdAt": ts,
            },
            "required": [
                "assignmentId",
                "technicianId",
                "technicianDisplayName",
                "machineId",
                "machineName",
                "machineSerialNumber",
                "role",
                "validFrom",
                "createdAt",
            ],
        },
        "V1AdminAssignmentsListResponse": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminAssignmentListItem"}},
                "meta": {"$ref": "#/components/schemas/V1CollectionListMeta"},
            },
            "required": ["items", "meta"],
        },
        "V1AdminCommandListItem": {
            "type": "object",
            "properties": {
                "commandId": uuid_s,
                "machineId": uuid_s,
                "organizationId": uuid_s,
                "machineName": {"type": "string"},
                "machineSerialNumber": {"type": "string"},
                "sequence": {"type": "integer", "format": "int64"},
                "commandType": {"type": "string"},
                "createdAt": ts,
                "attemptCount": {"type": "integer", "format": "int32"},
                "latestAttemptStatus": {"type": "string"},
                "correlationId": uuid_s,
            },
            "required": [
                "commandId",
                "machineId",
                "organizationId",
                "machineName",
                "machineSerialNumber",
                "sequence",
                "commandType",
                "createdAt",
                "attemptCount",
                "latestAttemptStatus",
            ],
        },
        "V1AdminCommandsListResponse": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminCommandListItem"}},
                "meta": {"$ref": "#/components/schemas/V1CollectionListMeta"},
            },
            "required": ["items", "meta"],
        },
        "V1AdminOTAListItem": {
            "type": "object",
            "properties": {
                "campaignId": uuid_s,
                "organizationId": uuid_s,
                "campaignName": {"type": "string"},
                "strategy": {"type": "string"},
                "campaignStatus": {"type": "string", "enum": ["draft", "active", "paused", "completed"]},
                "createdAt": ts,
                "artifactId": uuid_s,
                "artifactSemver": {"type": "string"},
                "artifactStorageKey": {"type": "string"},
            },
            "required": [
                "campaignId",
                "organizationId",
                "campaignName",
                "strategy",
                "campaignStatus",
                "createdAt",
                "artifactId",
                "artifactStorageKey",
            ],
        },
        "V1AdminOTAListResponse": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminOTAListItem"}},
                "meta": {"$ref": "#/components/schemas/V1CollectionListMeta"},
            },
            "required": ["items", "meta"],
        },
        "V1AdminOTACampaignListItem": {
            "type": "object",
            "properties": {
                "campaignId": uuid_s,
                "organizationId": uuid_s,
                "name": {"type": "string"},
                "rolloutStrategy": {"type": "string", "enum": ["immediate", "canary"]},
                "status": {
                    "type": "string",
                    "enum": ["draft", "approved", "running", "paused", "completed", "failed", "cancelled", "rolled_back"],
                },
                "campaignType": {"type": "string", "enum": ["app", "firmware", "config"]},
                "canaryPercent": {"type": "integer", "format": "int32"},
                "rolloutNextOffset": {"type": "integer", "format": "int32"},
                "artifactId": uuid_s,
                "artifactSemver": {"type": "string"},
                "artifactStorageKey": {"type": "string"},
                "artifactVersion": {"type": "string"},
                "rollbackArtifactId": uuid_s,
                "createdAt": ts,
                "updatedAt": ts,
                "approvedAt": ts,
            },
            "required": [
                "campaignId",
                "organizationId",
                "name",
                "rolloutStrategy",
                "status",
                "campaignType",
                "canaryPercent",
                "rolloutNextOffset",
                "artifactId",
                "artifactStorageKey",
                "createdAt",
                "updatedAt",
            ],
        },
        "V1AdminOTACampaignListResponse": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminOTACampaignListItem"}},
                "meta": {"$ref": "#/components/schemas/V1CollectionListMeta"},
            },
            "required": ["items", "meta"],
        },
        "V1AdminOTACampaignDetail": {
            "allOf": [
                {"$ref": "#/components/schemas/V1AdminOTACampaignListItem"},
                {
                    "type": "object",
                    "properties": {
                        "createdBy": uuid_s,
                        "approvedBy": uuid_s,
                        "pausedAt": ts,
                    },
                },
            ],
        },
        "V1AdminOTACampaignTargetItem": {
            "type": "object",
            "properties": {
                "machineId": uuid_s,
                "state": {"type": "string"},
                "lastError": {"type": "string"},
                "updatedAt": ts,
            },
            "required": ["machineId", "state", "updatedAt"],
        },
        "V1AdminOTACampaignTargetsResponse": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminOTACampaignTargetItem"}},
            },
            "required": ["items"],
        },
        "V1AdminOTACampaignMachineResultItem": {
            "type": "object",
            "properties": {
                "machineId": uuid_s,
                "wave": {"type": "string"},
                "commandId": uuid_s,
                "status": {"type": "string"},
                "lastError": {"type": "string"},
                "updatedAt": ts,
                "createdAt": ts,
            },
            "required": ["machineId", "wave", "status", "updatedAt", "createdAt"],
        },
        "V1AdminOTACampaignResultsResponse": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminOTACampaignMachineResultItem"}},
            },
            "required": ["items"],
        },
    }


def admin_operations_component_schemas() -> dict[str, Any]:
    """OpenAPI schemas for tenant admin operations (machine health, commands, inventory anomalies)."""
    uuid_s = {"type": "string", "format": "uuid"}
    ts = dict(_TS_SCHEMA)
    i32 = {"type": "integer", "format": "int32"}
    i64 = {"type": "integer", "format": "int64"}
    attempt = {
        "type": "object",
        "properties": {
            "id": uuid_s,
            "attemptNo": i32,
            "status": {"type": "string"},
            "sentAt": ts,
            "dispatchState": {"type": "string"},
            "ackDeadlineAt": ts,
            "resultReceivedAt": ts,
            "timeoutReason": {"type": "string"},
        },
        "required": ["id", "attemptNo", "status", "sentAt", "dispatchState"],
    }
    health_common = {
        "machineId": uuid_s,
        "status": {"type": "string"},
        "pendingCommandCount": i32,
        "failedCommandCount": i32,
        "inventoryAnomalyCount": i32,
        "lastSeenAt": ts,
        "lastCheckInAt": ts,
        "appVersion": {"type": "string"},
        "configVersion": {"type": "string"},
        "catalogVersion": {"type": "string"},
        "mediaVersion": {"type": "string"},
        "mqttConnected": {"type": "boolean"},
        "lastErrorCode": {"type": "string"},
        "telemetryFreshnessSeconds": {"type": "integer", "format": "int64", "description": "Seconds since telemetry snapshot update; -1 when unknown."},
    }
    health_required = [
        "machineId",
        "status",
        "pendingCommandCount",
        "failedCommandCount",
        "inventoryAnomalyCount",
    ]
    return {
        "V1AdminOperationsMachineHealthItem": {
            "type": "object",
            "properties": health_common,
            "required": health_required,
        },
        "V1AdminOperationsMachineHealthListResponse": {
            "type": "object",
            "properties": {
                "items": {
                    "type": "array",
                    "items": {"$ref": "#/components/schemas/V1AdminOperationsMachineHealthItem"},
                },
            },
            "required": ["items"],
        },
        "V1AdminOperationsTimelineEvent": {
            "type": "object",
            "properties": {
                "occurredAt": ts,
                "eventKind": {"type": "string"},
                "title": {"type": "string"},
                "payload": {"type": "object", "additionalProperties": True},
                "refId": uuid_s,
            },
            "required": ["occurredAt", "eventKind", "title", "payload"],
        },
        "V1AdminOperationsTimelineListResponse": {
            "type": "object",
            "properties": {
                "items": {
                    "type": "array",
                    "items": {"$ref": "#/components/schemas/V1AdminOperationsTimelineEvent"},
                },
            },
            "required": ["items"],
        },
        "V1AdminOperationsCommandAttemptItem": attempt,
        "V1AdminOperationsCommandDetailResponse": {
            "type": "object",
            "properties": {
                "commandId": uuid_s,
                "machineId": uuid_s,
                "organizationId": uuid_s,
                "sequence": i64,
                "commandType": {"type": "string"},
                "payload": {"type": "object", "additionalProperties": True},
                "createdAt": ts,
                "correlationId": uuid_s,
                "idempotencyKey": {"type": "string"},
                "attempts": {
                    "type": "array",
                    "items": {"$ref": "#/components/schemas/V1AdminOperationsCommandAttemptItem"},
                },
            },
            "required": ["commandId", "machineId", "organizationId", "sequence", "commandType", "payload", "createdAt", "attempts"],
        },
        "V1AdminOperationsCommandRetryResponse": {
            "type": "object",
            "properties": {
                "commandId": uuid_s,
                "sequence": i64,
                "attemptId": uuid_s,
                "dispatchState": {"type": "string"},
                "replay": {"type": "boolean"},
                "skippedRepublish": {"type": "boolean"},
            },
            "required": ["commandId", "sequence", "attemptId", "dispatchState", "replay", "skippedRepublish"],
        },
        "V1AdminOperationsCommandCancelResponse": {
            "type": "object",
            "properties": {"attemptsCancelled": i32},
            "required": ["attemptsCancelled"],
        },
        "V1AdminOperationsMachineCommandDispatchRequest": {
            "type": "object",
            "properties": {
                "commandType": {"type": "string"},
                "payload": {"type": "object", "additionalProperties": True},
            },
            "required": ["commandType"],
        },
        "V1AdminOperationsMachineCommandDispatchResponse": {
            "type": "object",
            "properties": {
                "commandId": uuid_s,
                "sequence": i64,
                "attemptId": uuid_s,
                "dispatchState": {"type": "string"},
                "replay": {"type": "boolean"},
            },
            "required": ["commandId", "sequence", "attemptId", "dispatchState", "replay"],
        },
        "V1AdminOperationsInventoryAnomalyItem": {
            "type": "object",
            "properties": {
                "id": uuid_s,
                "organizationId": uuid_s,
                "machineId": uuid_s,
                "machineName": {"type": "string"},
                "machineSerialNumber": {"type": "string"},
                "anomalyType": {"type": "string"},
                "status": {"type": "string"},
                "fingerprint": {"type": "string"},
                "detectedAt": ts,
                "createdAt": ts,
                "updatedAt": ts,
                "slotCode": {"type": "string"},
                "productId": uuid_s,
                "payload": {"type": "object", "additionalProperties": True},
                "resolvedAt": ts,
                "resolvedBy": uuid_s,
                "resolutionNote": {"type": "string"},
            },
            "required": [
                "id",
                "organizationId",
                "machineId",
                "machineName",
                "machineSerialNumber",
                "anomalyType",
                "status",
                "fingerprint",
                "detectedAt",
                "createdAt",
                "updatedAt",
            ],
        },
        "V1AdminOperationsInventoryAnomalyListResponse": {
            "type": "object",
            "properties": {
                "items": {
                    "type": "array",
                    "items": {"$ref": "#/components/schemas/V1AdminOperationsInventoryAnomalyItem"},
                },
            },
            "required": ["items"],
        },
        "V1AdminOperationsInventoryAnomalyResolveRequest": {
            "type": "object",
            "properties": {"note": {"type": "string"}},
        },
        "V1AdminOperationsInventoryAnomalyResolveResponse": {
            "type": "object",
            "properties": {"anomalyId": uuid_s, "status": {"type": "string"}},
            "required": ["anomalyId", "status"],
        },
        "V1AdminOperationsInventoryReconcileRequest": {
            "type": "object",
            "properties": {"reason": {"type": "string"}},
        },
        "V1AdminOperationsInventoryReconcileResponse": {
            "type": "object",
            "properties": {"inventoryEventId": i64},
            "required": ["inventoryEventId"],
        },
        "V1AdminProvisioningBulkMachineRow": {
            "type": "object",
            "properties": {
                "serialNumber": {"type": "string"},
                "name": {"type": "string"},
                "model": {"type": "string"},
            },
            "required": ["serialNumber"],
        },
        "V1AdminProvisioningBulkCreateRequest": {
            "type": "object",
            "properties": {
                "siteId": uuid_s,
                "hardwareProfileId": uuid_s,
                "cabinetType": {"type": "string"},
                "machines": {
                    "type": "array",
                    "items": {"$ref": "#/components/schemas/V1AdminProvisioningBulkMachineRow"},
                },
                "generateActivationCodes": {"type": "boolean"},
                "expiresInMinutes": i32,
                "maxUses": i32,
            },
            "required": ["siteId", "cabinetType", "machines", "generateActivationCodes"],
        },
        "V1AdminProvisioningBulkMachineOut": {
            "type": "object",
            "properties": {
                "machineId": uuid_s,
                "serialNumber": {"type": "string"},
                "activationCode": {"type": "string"},
                "activationCodeId": uuid_s,
            },
            "required": ["machineId", "serialNumber"],
        },
        "V1AdminProvisioningBulkCreateResponse": {
            "type": "object",
            "properties": {
                "batchId": uuid_s,
                "status": {"type": "string"},
                "machines": {
                    "type": "array",
                    "items": {"$ref": "#/components/schemas/V1AdminProvisioningBulkMachineOut"},
                },
                "machineCount": {"type": "integer"},
            },
            "required": ["batchId", "status", "machines", "machineCount"],
        },
        "V1AdminProvisioningBatchDetailResponse": {
            "type": "object",
            "properties": {
                "batch": {"type": "object", "additionalProperties": True},
                "machines": {"type": "array", "items": {"type": "object", "additionalProperties": True}},
            },
            "required": ["batch", "machines"],
        },
        "V1AdminRolloutCreateRequest": {
            "type": "object",
            "properties": {
                "rolloutType": {"type": "string"},
                "targetVersion": {"type": "string"},
                "strategy": {"type": "object", "additionalProperties": True},
            },
            "required": ["rolloutType", "targetVersion"],
        },
        "V1AdminRolloutCampaign": {
            "type": "object",
            "properties": {
                "id": uuid_s,
                "organizationId": uuid_s,
                "rolloutType": {"type": "string"},
                "targetVersion": {"type": "string"},
                "status": {"type": "string"},
                "strategy": {"type": "object", "additionalProperties": True},
                "createdBy": uuid_s,
                "createdAt": ts,
                "updatedAt": ts,
                "startedAt": ts,
                "completedAt": ts,
                "cancelledAt": ts,
            },
            "required": ["id", "organizationId", "rolloutType", "targetVersion", "status", "createdAt", "updatedAt"],
        },
        "V1AdminRolloutTarget": {
            "type": "object",
            "properties": {
                "id": uuid_s,
                "organizationId": uuid_s,
                "campaignId": uuid_s,
                "machineId": uuid_s,
                "status": {"type": "string"},
                "error": {"type": "string"},
                "commandId": uuid_s,
                "createdAt": ts,
                "updatedAt": ts,
            },
            "required": ["id", "organizationId", "campaignId", "machineId", "status", "createdAt", "updatedAt"],
        },
        "V1AdminRolloutDetailResponse": {
            "type": "object",
            "properties": {
                "campaign": {"$ref": "#/components/schemas/V1AdminRolloutCampaign"},
                "targets": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminRolloutTarget"}},
            },
            "required": ["campaign", "targets"],
        },
        "V1AdminRolloutListMeta": {
            "type": "object",
            "properties": {
                "limit": i32,
                "offset": i32,
                "returned": {"type": "integer"},
            },
            "required": ["limit", "offset", "returned"],
        },
        "V1AdminRolloutListResponse": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminRolloutCampaign"}},
                "meta": {"$ref": "#/components/schemas/V1AdminRolloutListMeta"},
            },
            "required": ["items", "meta"],
        },
    }


def machine_setup_component_schemas() -> dict[str, Any]:
    """OpenAPI schemas for machine technician setup (bootstrap + admin topology/planogram)."""
    uuid_s = {"type": "string", "format": "uuid"}
    ts = dict(_TS_SCHEMA)
    i32 = {"type": "integer", "format": "int32"}
    i64 = {"type": "integer", "format": "int64"}
    meta_obj = {"type": "object", "additionalProperties": True}
    topo_slot = {
        "type": "object",
        "properties": {
            "configId": uuid_s,
            "slotCode": {"type": "string"},
            "slotIndex": i32,
            "productId": uuid_s,
            "productSku": {"type": "string"},
            "productName": {"type": "string"},
            "maxQuantity": i32,
            "priceMinor": i64,
            "effectiveFrom": ts,
            "isCurrent": {"type": "boolean"},
            "machineSlotLayoutId": uuid_s,
            "metadata": meta_obj,
        },
        "required": [
            "configId",
            "slotCode",
            "productSku",
            "productName",
            "maxQuantity",
            "priceMinor",
            "effectiveFrom",
            "isCurrent",
            "machineSlotLayoutId",
            "metadata",
        ],
    }
    topo_cab = {
        "type": "object",
        "properties": {
            "id": uuid_s,
            "code": {"type": "string"},
            "title": {"type": "string"},
            "sortOrder": i32,
            "metadata": meta_obj,
            "slots": {"type": "array", "items": topo_slot},
        },
        "required": ["id", "code", "title", "sortOrder", "metadata", "slots"],
    }
    cat_prod = {
        "type": "object",
        "properties": {
            "productId": uuid_s,
            "sku": {"type": "string"},
            "name": {"type": "string"},
            "sortOrder": i32,
            "assortmentId": uuid_s,
            "assortmentName": {"type": "string"},
        },
        "required": ["productId", "sku", "name", "sortOrder", "assortmentId", "assortmentName"],
    }
    mach = {
        "type": "object",
        "properties": {
            "machineId": uuid_s,
            "organizationId": uuid_s,
            "siteId": uuid_s,
            "hardwareProfileId": uuid_s,
            "serialNumber": {"type": "string"},
            "name": {"type": "string"},
            "status": {"type": "string"},
            "commandSequence": i64,
            "createdAt": ts,
            "updatedAt": ts,
        },
        "required": [
            "machineId",
            "organizationId",
            "siteId",
            "serialNumber",
            "name",
            "status",
            "commandSequence",
            "createdAt",
            "updatedAt",
        ],
    }
    cmd_info = {
        "type": "object",
        "properties": {
            "commandId": uuid_s,
            "sequence": i64,
            "dispatchState": {"type": "string"},
            "replay": {"type": "boolean"},
        },
        "required": ["commandId", "sequence", "dispatchState", "replay"],
    }
    slot_item = {
        "type": "object",
        "properties": {
            "machineId": uuid_s,
            "machineName": {"type": "string"},
            "machineStatus": {"type": "string"},
            "planogramId": uuid_s,
            "planogramName": {"type": "string"},
            "slotIndex": i32,
            "cabinetCode": {"type": "string"},
            "cabinetIndex": i32,
            "slotCode": {"type": "string"},
            "currentQuantity": i32,
            "currentStock": i32,
            "maxQuantity": i32,
            "capacity": i32,
            "parLevel": i32,
            "lowStockThreshold": i32,
            "priceMinor": i64,
            "currency": {"type": "string"},
            "status": {"type": "string", "enum": ["ok", "low_stock", "out_of_stock"]},
            "planogramRevisionApplied": i32,
            "updatedAt": ts,
            "productId": uuid_s,
            "productSku": {"type": "string"},
            "productName": {"type": "string"},
            "isEmpty": {"type": "boolean"},
            "lowStock": {"type": "boolean"},
        },
        "required": [
            "machineId",
            "machineName",
            "machineStatus",
            "planogramId",
            "planogramName",
            "slotIndex",
            "cabinetCode",
            "cabinetIndex",
            "slotCode",
            "currentQuantity",
            "currentStock",
            "maxQuantity",
            "capacity",
            "parLevel",
            "lowStockThreshold",
            "priceMinor",
            "currency",
            "status",
            "planogramRevisionApplied",
            "updatedAt",
            "isEmpty",
            "lowStock",
        ],
    }
    inv_line = {
        "type": "object",
        "properties": {
            "machineId": uuid_s,
            "machineName": {"type": "string"},
            "machineStatus": {"type": "string"},
            "productId": uuid_s,
            "productName": {"type": "string"},
            "productSku": {"type": "string"},
            "totalQuantity": i64,
            "slotCount": i64,
            "maxCapacityAnySlot": i32,
            "lowStock": {"type": "boolean"},
            "cabinetCode": {
                "type": "string",
                "description": "When all slots for this product map to one cabinet; omitted when stock spans multiple cabinets.",
            },
            "cabinetIndex": {
                "format": "int32",
                "type": "integer",
                "description": "Parallel to cabinetCode when present.",
            },
        },
        "required": [
            "machineId",
            "machineName",
            "machineStatus",
            "productId",
            "productName",
            "productSku",
            "totalQuantity",
            "slotCount",
            "maxCapacityAnySlot",
            "lowStock",
        ],
    }
    adj_item = {
        "type": "object",
        "properties": {
            "planogramId": uuid_s,
            "slotIndex": i32,
            "quantityBefore": i32,
            "quantityAfter": i32,
            "cabinetCode": {"type": "string"},
            "slotCode": {"type": "string"},
            "productId": uuid_s,
        },
        "required": ["planogramId", "slotIndex", "quantityBefore", "quantityAfter"],
    }
    return {
        "V1SetupMachineBootstrapResponse": {
            "type": "object",
            "properties": {
                "machine": mach,
                "topology": {
                    "type": "object",
                    "properties": {"cabinets": {"type": "array", "items": topo_cab}},
                    "required": ["cabinets"],
                },
                "catalog": {
                    "type": "object",
                    "properties": {"products": {"type": "array", "items": cat_prod}},
                    "required": ["products"],
                },
            },
            "required": ["machine", "topology", "catalog"],
        },
        "V1AdminPlanogramCommandInfo": cmd_info,
        "V1AdminPlanogramPublishResponse": {
            "type": "object",
            "properties": {
                "desiredConfigVersion": i32,
                "planogramId": uuid_s,
                "planogramRevision": i32,
                "command": {"$ref": "#/components/schemas/V1AdminPlanogramCommandInfo"},
            },
            "required": ["desiredConfigVersion", "planogramId", "planogramRevision", "command"],
        },
        "V1AdminMachineSyncResponse": {
            "type": "object",
            "properties": {"command": {"$ref": "#/components/schemas/V1AdminPlanogramCommandInfo"}},
            "required": ["command"],
        },
        "V1AdminMachineSlot": slot_item,
        "V1AdminMachineSlotListEnvelope": {
            "type": "object",
            "properties": {"items": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminMachineSlot"}}},
            "required": ["items"],
        },
        "V1AdminMachineInventoryLine": inv_line,
        "V1AdminMachineInventoryEnvelope": {
            "type": "object",
            "properties": {"items": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminMachineInventoryLine"}}},
            "required": ["items"],
        },
        "V1AdminInventoryRefillForecastMeta": {
            "type": "object",
            "properties": {
                "limit": i32,
                "offset": i32,
                "returned": {"type": "integer"},
                "total": {"type": "integer", "format": "int64"},
            },
            "required": ["limit", "offset", "returned", "total"],
        },
        "V1AdminInventoryRefillForecastItem": {
            "type": "object",
            "properties": {
                "machineId": uuid_s,
                "machineName": {"type": "string"},
                "siteId": uuid_s,
                "siteName": {"type": "string"},
                "planogramId": uuid_s,
                "planogramName": {"type": "string"},
                "slotIndex": i32,
                "productId": uuid_s,
                "productSku": {"type": "string"},
                "productName": {"type": "string"},
                "currentQuantity": i32,
                "maxQuantity": i32,
                "unitsSoldInWindow": i64,
                "dailyVelocity": {"type": "number"},
                "daysToEmpty": {"type": "number"},
                "fillRatio": {"type": "number"},
                "suggestedRefillQuantity": i32,
                "urgency": {"type": "string"},
            },
            "required": [
                "machineId",
                "machineName",
                "siteId",
                "siteName",
                "planogramId",
                "planogramName",
                "slotIndex",
                "productId",
                "currentQuantity",
                "maxQuantity",
                "unitsSoldInWindow",
                "dailyVelocity",
                "fillRatio",
                "suggestedRefillQuantity",
                "urgency",
            ],
        },
        "V1AdminInventoryRefillForecastResponse": {
            "type": "object",
            "properties": {
                "organizationId": uuid_s,
                "velocityWindowDays": {"type": "integer", "format": "int32"},
                "windowStart": ts,
                "windowEnd": ts,
                "items": {
                    "type": "array",
                    "items": {"$ref": "#/components/schemas/V1AdminInventoryRefillForecastItem"},
                },
                "meta": {"$ref": "#/components/schemas/V1AdminInventoryRefillForecastMeta"},
            },
            "required": ["organizationId", "velocityWindowDays", "windowStart", "windowEnd", "items", "meta"],
        },
        "V1AdminInventoryEvent": {
            "type": "object",
            "properties": {
                "id": {"type": "integer", "format": "int64"},
                "organizationId": uuid_s,
                "machineId": uuid_s,
                "cabinetCode": {"type": "string"},
                "slotCode": {"type": "string"},
                "productId": uuid_s,
                "eventType": {"type": "string"},
                "reasonCode": {"type": "string"},
                "quantityBefore": i32,
                "quantityDelta": i32,
                "quantityAfter": i32,
                "unitPriceMinor": i64,
                "currency": {"type": "string"},
                "correlationId": uuid_s,
                "operatorSessionId": uuid_s,
                "technicianId": uuid_s,
                "technicianDisplayName": {"type": "string"},
                "refillSessionId": uuid_s,
                "inventoryCountSessionId": uuid_s,
                "occurredAt": ts,
                "recordedAt": ts,
            },
            "required": [
                "id",
                "organizationId",
                "machineId",
                "eventType",
                "quantityDelta",
                "unitPriceMinor",
                "currency",
                "occurredAt",
                "recordedAt",
            ],
        },
        "V1AdminInventoryEventListEnvelope": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminInventoryEvent"}},
            },
            "required": ["items"],
        },
        "V1AdminStockAdjustmentsRequest": {
            "type": "object",
            "properties": {
                "operator_session_id": uuid_s,
                "reason": {
                    "type": "string",
                    "enum": ["restock", "cycle_count", "manual_adjustment", "machine_reconcile"],
                },
                "occurredAt": ts,
                "items": {"type": "array", "items": adj_item, "minItems": 1},
            },
            "required": ["operator_session_id", "reason", "items"],
        },
        "V1AdminStockAdjustmentsResponse": {
            "type": "object",
            "properties": {
                "replay": {"type": "boolean"},
                "eventIds": {"type": "array", "items": {"type": "integer", "format": "int64"}},
            },
            "required": ["replay"],
        },
    }


def reporting_component_schemas() -> dict[str, Any]:
    """OpenAPI schemas for GET /v1/reports/* (read-only analytics)."""
    uuid_s = {"type": "string", "format": "uuid"}
    ts = dict(_TS_SCHEMA)
    int64 = {"type": "integer", "format": "int64"}
    i32 = {"type": "integer", "format": "int32"}
    sales_rollup = {
        "type": "object",
        "properties": {
            "grossTotalMinor": int64,
            "subtotalMinor": int64,
            "taxMinor": int64,
            "orderCount": int64,
            "avgOrderValueMinor": int64,
        },
        "required": ["grossTotalMinor", "subtotalMinor", "taxMinor", "orderCount", "avgOrderValueMinor"],
    }
    sales_break = {
        "type": "object",
        "properties": {
            "bucketStart": ts,
            "siteId": uuid_s,
            "machineId": uuid_s,
            "paymentProvider": {"type": "string"},
            "orderCount": int64,
            "totalMinor": int64,
            "subtotalMinor": int64,
            "taxMinor": int64,
        },
        "required": ["orderCount", "totalMinor", "subtotalMinor", "taxMinor"],
    }
    pay_rollup = {
        "type": "object",
        "properties": {
            "authorizedCount": int64,
            "capturedCount": int64,
            "failedCount": int64,
            "refundedCount": int64,
            "capturedAmountMinor": int64,
            "authorizedAmountMinor": int64,
            "failedAmountMinor": int64,
            "refundedAmountMinor": int64,
        },
        "required": [
            "authorizedCount",
            "capturedCount",
            "failedCount",
            "refundedCount",
            "capturedAmountMinor",
            "authorizedAmountMinor",
            "failedAmountMinor",
            "refundedAmountMinor",
        ],
    }
    pay_break = {
        "type": "object",
        "properties": {
            "bucketStart": ts,
            "provider": {"type": "string"},
            "state": {"type": "string"},
            "paymentCount": int64,
            "amountMinor": int64,
        },
        "required": ["paymentCount", "amountMinor"],
    }
    fleet_status_row = {
        "type": "object",
        "properties": {"status": {"type": "string"}, "count": int64},
        "required": ["status", "count"],
    }
    fleet_sev_row = {
        "type": "object",
        "properties": {"severity": {"type": "string"}, "count": int64},
        "required": ["severity", "count"],
    }
    inv_meta = {
        "type": "object",
        "properties": {
            "limit": i32,
            "offset": i32,
            "returned": {"type": "integer"},
            "total": int64,
        },
        "required": ["limit", "offset", "returned", "total"],
    }
    inv_item = {
        "type": "object",
        "properties": {
            "machineId": uuid_s,
            "machineName": {"type": "string"},
            "machineSerialNumber": {"type": "string"},
            "machineStatus": {"type": "string"},
            "planogramId": uuid_s,
            "planogramName": {"type": "string"},
            "slotIndex": i32,
            "currentQuantity": i32,
            "maxQuantity": i32,
            "productId": uuid_s,
            "productSku": {"type": "string"},
            "productName": {"type": "string"},
            "outOfStock": {"type": "boolean"},
            "lowStock": {"type": "boolean"},
            "attentionNeeded": {"type": "boolean"},
        },
        "required": [
            "machineId",
            "machineName",
            "machineSerialNumber",
            "machineStatus",
            "planogramId",
            "planogramName",
            "slotIndex",
            "currentQuantity",
            "maxQuantity",
            "outOfStock",
            "lowStock",
            "attentionNeeded",
        ],
    }
    return {
        "V1ReportingSalesSummaryResponse": {
            "type": "object",
            "properties": {
                "organizationId": uuid_s,
                "from": ts,
                "to": ts,
                "groupBy": {"type": "string", "description": "day | site | machine | payment_method | none"},
                "summary": sales_rollup,
                "breakdown": {"type": "array", "items": sales_break},
            },
            "required": ["organizationId", "from", "to", "groupBy", "summary", "breakdown"],
        },
        "V1ReportingPaymentsSummaryResponse": {
            "type": "object",
            "properties": {
                "organizationId": uuid_s,
                "from": ts,
                "to": ts,
                "groupBy": {"type": "string", "description": "day | payment_method | status | none"},
                "summary": pay_rollup,
                "breakdown": {"type": "array", "items": pay_break},
            },
            "required": ["organizationId", "from", "to", "groupBy", "summary", "breakdown"],
        },
        "V1ReportingFleetHealthResponse": {
            "type": "object",
            "properties": {
                "organizationId": uuid_s,
                "from": ts,
                "to": ts,
                "machineSummary": {
                    "type": "object",
                    "properties": {
                        "total": int64,
                        "online": int64,
                        "offline": int64,
                        "fault": int64,
                        "warn": int64,
                        "retired": int64,
                    },
                    "required": ["total", "online", "offline", "fault", "warn", "retired"],
                },
                "machinesByStatus": {"type": "array", "items": fleet_status_row},
                "incidentsByStatus": {"type": "array", "items": fleet_status_row},
                "machineIncidentsBySeverity": {"type": "array", "items": fleet_sev_row},
            },
            "required": [
                "organizationId",
                "from",
                "to",
                "machineSummary",
                "machinesByStatus",
                "incidentsByStatus",
                "machineIncidentsBySeverity",
            ],
        },
        "V1ReportingInventoryExceptionsResponse": {
            "type": "object",
            "properties": {
                "organizationId": uuid_s,
                "from": ts,
                "to": ts,
                "exceptionKind": {"type": "string", "enum": ["all", "low_stock", "out_of_stock"]},
                "meta": inv_meta,
                "items": {"type": "array", "items": inv_item},
            },
            "required": ["organizationId", "from", "to", "exceptionKind", "meta", "items"],
        },
        "V1AdminReportSalesResponse": {
            "allOf": [{"$ref": "#/components/schemas/V1ReportingSalesSummaryResponse"}],
        },
        "V1AdminReportPaymentsResponse": {
            "type": "object",
            "properties": {
                "organizationId": uuid_s,
                "from": ts,
                "to": ts,
                "timezone": {"type": "string"},
                "items": {
                    "type": "array",
                    "items": {
                        "type": "object",
                        "properties": {
                            "bucketStart": ts,
                            "provider": {"type": "string"},
                            "state": {"type": "string"},
                            "settlementStatus": {"type": "string"},
                            "reconciliationStatus": {"type": "string"},
                            "paymentCount": int64,
                            "amountMinor": int64,
                        },
                        "required": ["bucketStart", "provider", "state", "settlementStatus", "reconciliationStatus", "paymentCount", "amountMinor"],
                    },
                },
                "meta": inv_meta,
            },
            "required": ["organizationId", "from", "to", "timezone", "items", "meta"],
        },
        "V1AdminReportListResponse": {
            "type": "object",
            "properties": {
                "organizationId": uuid_s,
                "from": ts,
                "to": ts,
                "meta": inv_meta,
                "items": {"type": "array", "items": {"type": "object", "additionalProperties": True}},
            },
            "required": ["organizationId", "from", "to", "meta", "items"],
        },
        "V1FinanceDailyClose": {
            "type": "object",
            "properties": {
                "id": uuid_s,
                "organizationId": uuid_s,
                "closeDate": {"type": "string", "format": "date"},
                "timezone": {"type": "string"},
                "siteId": uuid_s,
                "machineId": uuid_s,
                "idempotencyKey": {"type": "string"},
                "grossSalesMinor": int64,
                "discountMinor": int64,
                "refundMinor": int64,
                "netMinor": int64,
                "cashMinor": int64,
                "qrWalletMinor": int64,
                "failedMinor": int64,
                "pendingMinor": int64,
                "createdAt": ts,
            },
            "required": [
                "id",
                "organizationId",
                "closeDate",
                "timezone",
                "idempotencyKey",
                "grossSalesMinor",
                "discountMinor",
                "refundMinor",
                "netMinor",
                "cashMinor",
                "qrWalletMinor",
                "failedMinor",
                "pendingMinor",
                "createdAt",
            ],
        },
        "V1FinanceDailyCloseCreateRequest": {
            "type": "object",
            "properties": {
                "closeDate": {"type": "string", "format": "date"},
                "timezone": {"type": "string"},
                "siteId": uuid_s,
                "machineId": uuid_s,
            },
            "required": ["closeDate", "timezone"],
        },
        "V1FinanceDailyCloseListResponse": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {"$ref": "#/components/schemas/V1FinanceDailyClose"}},
                "meta": {
                    "type": "object",
                    "properties": {
                        "limit": i32,
                        "offset": i32,
                        "returned": {"type": "integer"},
                        "total": int64,
                    },
                    "required": ["limit", "offset", "returned", "total"],
                },
            },
            "required": ["items", "meta"],
        },
    }


def cash_settlement_component_schemas() -> dict[str, Any]:
    """OpenAPI schemas for admin cashbox + cash collection settlement."""
    cash_coll = {
        "type": "object",
        "properties": {
            "id": {"type": "string", "format": "uuid"},
            "machine_id": {"type": "string", "format": "uuid"},
            "organization_id": {"type": "string", "format": "uuid"},
            "collected_at": {"type": "string", "format": "date-time"},
            "opened_at": {"type": "string", "format": "date-time"},
            "closed_at": {"type": "string", "format": "date-time"},
            "lifecycle_status": {"type": "string", "enum": ["open", "closed", "cancelled"]},
            "counted_amount_minor": {"type": "integer", "format": "int64"},
            "expected_amount_minor": {"type": "integer", "format": "int64"},
            "variance_amount_minor": {"type": "integer", "format": "int64"},
            "countedPhysicalCashMinor": {"type": "integer", "format": "int64"},
            "expectedCloudCashMinor": {"type": "integer", "format": "int64"},
            "varianceMinor": {"type": "integer", "format": "int64"},
            "reviewState": {"type": "string"},
            "requires_review": {"type": "boolean"},
            "close_request_hash_hex": {"type": "string"},
            "currency": {"type": "string"},
            "reconciliation_status": {"type": "string"},
            "disclosure": {"type": "string"},
        },
        "required": [
            "id",
            "machine_id",
            "organization_id",
            "collected_at",
            "opened_at",
            "lifecycle_status",
            "counted_amount_minor",
            "expected_amount_minor",
            "variance_amount_minor",
            "countedPhysicalCashMinor",
            "expectedCloudCashMinor",
            "varianceMinor",
            "reviewState",
            "requires_review",
            "currency",
            "reconciliation_status",
            "disclosure",
        ],
    }
    return {
        "V1CashDenominationExpectation": {
            "type": "object",
            "properties": {
                "denominationMinor": {"type": "integer", "format": "int64"},
                "expectedCount": {"type": "integer", "format": "int64"},
                "source": {"type": "string"},
            },
            "required": ["denominationMinor", "expectedCount", "source"],
        },
        "V1AdminMachineCashboxResponse": {
            "type": "object",
            "properties": {
                "machineId": {"type": "string", "format": "uuid"},
                "currency": {"type": "string"},
                "expectedCashboxMinor": {"type": "integer", "format": "int64"},
                "expectedCloudCashMinor": {"type": "integer", "format": "int64"},
                "expectedRecyclerMinor": {"type": "integer", "format": "int64"},
                "lastCollectionAt": {"type": "string", "format": "date-time"},
                "denominations": {
                    "type": "array",
                    "items": {"$ref": "#/components/schemas/V1CashDenominationExpectation"},
                },
                "openCollectionId": {"type": "string", "format": "uuid"},
                "varianceReviewThresholdMinor": {"type": "integer", "format": "int64"},
                "disclosure": {"type": "string"},
            },
            "required": [
                "machineId",
                "currency",
                "expectedCashboxMinor",
                "expectedCloudCashMinor",
                "expectedRecyclerMinor",
                "denominations",
                "varianceReviewThresholdMinor",
                "disclosure",
            ],
        },
        "V1AdminCashCollection": cash_coll,
        "V1AdminCashCollectionListResponse": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminCashCollection"}},
                "meta": {"$ref": "#/components/schemas/V1CollectionListMeta"},
            },
            "required": ["items", "meta"],
        },
    }


def v1_api_error_schema() -> dict[str, Any]:
    return {
        "type": "object",
        "properties": {
            "error": {
                "type": "object",
                "properties": {
                    "code": {"type": "string"},
                    "message": {"type": "string"},
                    "details": {"type": "object", "additionalProperties": True},
                    "requestId": {"type": "string"},
                },
                "required": ["code", "message", "details", "requestId"],
            }
        },
        "required": ["error"],
    }


def enterprise_error_named_schemas() -> dict[str, Any]:
    """Documented variants of the single JSON error envelope (apierr.V1 / writeAPIError)."""
    rid = "01ARZ3NDEKTSV4RRFFQ69G5AV"

    def wrap(desc: str, code: str, message: str, details: dict[str, Any] | None = None) -> dict[str, Any]:
        return {
            "allOf": [{"$ref": "#/components/schemas/V1APIErrorEnvelope"}],
            "description": desc,
            "example": v1_error_example(code, message, details, request_id=rid),
        }

    return {
        "ErrorResponse": wrap("Generic handler error using the standard envelope.", "invalid_request", "request could not be processed", {}),
        "ValidationErrorResponse": wrap(
            "Validation or malformed input (HTTP 400 family).",
            "invalid_query",
            "organization_id is required for platform_admin",
            {"param": "organization_id"},
        ),
        "ConflictErrorResponse": wrap(
            "State conflict (HTTP 409) — illegal transitions, idempotency mismatch, quantity_before_mismatch, etc.",
            "illegal_transition",
            "cannot start vend from current order status",
            {"from": "created", "to": "vending"},
        ),
        "UnauthorizedErrorResponse": wrap("Missing or invalid Bearer token (HTTP 401).", "unauthenticated", "missing bearer token", {}),
        "ForbiddenErrorResponse": wrap("Authenticated but not permitted (HTTP 403).", "forbidden", "insufficient role for this route", {}),
        "NotFoundErrorResponse": wrap("Unknown resource (HTTP 404).", "not_found", "order not found", {"resource": "order"}),
        "RateLimitErrorResponse": wrap("HTTP 429 when rate limits apply.", "rate_limited", "too many requests", {"retry_after_seconds": 2}),
        "InternalErrorResponse": wrap("Unexpected server failure (HTTP 500).", "internal", "unexpected error", {}),
    }



def missing_reference_component_schemas() -> dict[str, Any]:
    """Component schemas referenced by swag comments but not generated from Go structs."""
    uuid_s = {"type": "string", "format": "uuid"}
    ts = dict(_TS_SCHEMA)
    page_meta = {
        "type": "object",
        "properties": {
            "limit": {"type": "integer", "format": "int32"},
            "offset": {"type": "integer", "format": "int32"},
            "returned": {"type": "integer", "format": "int32"},
            "totalCount": {"type": "integer", "format": "int64"},
        },
        "required": ["limit", "offset", "returned", "totalCount"],
    }
    auth_account_props = {
        "accountId": dict(uuid_s),
        "organizationId": dict(uuid_s),
        "email": {"type": "string", "format": "email"},
        "roles": {"type": "array", "items": {"type": "string"}},
    }
    product_list_item = {
        "type": "object",
        "properties": {
            "id": dict(uuid_s),
            "organizationId": dict(uuid_s),
            "sku": {"type": "string"},
            "barcode": {"type": "string", "nullable": True},
            "name": {"type": "string"},
            "description": {"type": "string"},
            "active": {"type": "boolean"},
            "categoryId": {"type": "string", "format": "uuid", "nullable": True},
            "brandId": {"type": "string", "format": "uuid", "nullable": True},
            "createdAt": ts,
            "updatedAt": ts,
        },
        "required": ["id", "organizationId", "sku", "name", "description", "active", "createdAt", "updatedAt"],
    }
    product_detail = {
        "type": "object",
        "properties": {
            **product_list_item["properties"],
            "attrs": {"type": "object", "nullable": True, "additionalProperties": True},
            "primaryImageId": {"type": "string", "format": "uuid", "nullable": True},
            "countryOfOrigin": {"type": "string", "nullable": True},
            "ageRestricted": {"type": "boolean"},
            "allergenCodes": {"type": "array", "items": {"type": "string"}},
            "nutritionalNote": {"type": "string", "nullable": True},
        },
        "required": [
            "id",
            "organizationId",
            "sku",
            "name",
            "description",
            "active",
            "ageRestricted",
            "allergenCodes",
            "createdAt",
            "updatedAt",
        ],
    }
    price_book = {
        "type": "object",
        "properties": {
            "id": dict(uuid_s),
            "organizationId": dict(uuid_s),
            "name": {"type": "string"},
            "currency": {"type": "string", "example": "USD"},
            "effectiveFrom": ts,
            "effectiveTo": {**ts, "nullable": True},
            "isDefault": {"type": "boolean"},
            "active": {"type": "boolean"},
            "scopeType": {"type": "string"},
            "siteId": {"type": "string", "format": "uuid", "nullable": True},
            "machineId": {"type": "string", "format": "uuid", "nullable": True},
            "priority": {"type": "integer", "format": "int32"},
            "createdAt": ts,
            "updatedAt": ts,
        },
        "required": [
            "id",
            "organizationId",
            "name",
            "currency",
            "effectiveFrom",
            "isDefault",
            "active",
            "scopeType",
            "priority",
            "createdAt",
            "updatedAt",
        ],
    }
    preview_line = {
        "type": "object",
        "properties": {
            "productId": dict(uuid_s),
            "basePrice": {"type": "integer", "format": "int64", "description": "Minor units (same currency as book)"},
            "effectivePrice": {"type": "integer", "format": "int64", "description": "Minor units after scope/target resolution"},
            "currency": {"type": "string"},
            "priceBookId": dict(uuid_s),
            "appliedRuleIds": {"type": "array", "items": {"type": "string"}},
            "reasons": {"type": "array", "items": {"type": "string"}},
        },
        "required": ["productId", "basePrice", "effectivePrice", "currency", "priceBookId", "appliedRuleIds", "reasons"],
    }
    preview_resp = {
        "type": "object",
        "properties": {
            "at": ts,
            "currency": {"type": "string"},
            "lines": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminPricingPreviewLine"}},
        },
        "required": ["at", "currency", "lines"],
    }
    planogram = {
        "type": "object",
        "properties": {
            "id": dict(uuid_s),
            "organizationId": dict(uuid_s),
            "name": {"type": "string"},
            "revision": {"type": "integer", "format": "int32"},
            "status": {"type": "string"},
            "meta": {"type": "object", "nullable": True, "additionalProperties": True},
            "createdAt": ts,
        },
        "required": ["id", "organizationId", "name", "revision", "status", "createdAt"],
    }
    planogram_slot = {
        "type": "object",
        "properties": {
            "id": dict(uuid_s),
            "planogramId": dict(uuid_s),
            "slotIndex": {"type": "integer", "format": "int32"},
            "productId": {"type": "string", "format": "uuid", "nullable": True},
            "maxQuantity": {"type": "integer", "format": "int32"},
            "productSku": {"type": "string", "nullable": True},
            "productName": {"type": "string", "nullable": True},
            "createdAt": ts,
        },
        "required": ["id", "planogramId", "slotIndex", "maxQuantity", "createdAt"],
    }
    return {
        "V1AuthTokenPair": {
            "type": "object",
            "properties": {
                "accessToken": {"type": "string"},
                "accessExpiresAt": ts,
                "refreshToken": {"type": "string"},
                "refreshExpiresAt": ts,
                "tokenType": {"type": "string", "example": "Bearer"},
            },
            "required": ["accessToken", "accessExpiresAt", "refreshToken", "refreshExpiresAt", "tokenType"],
        },
        "V1AuthLoginResponse": {
            "type": "object",
            "properties": {
                **auth_account_props,
                "tokens": {"$ref": "#/components/schemas/V1AuthTokenPair"},
                "mfaRequired": {"type": "boolean"},
                "mfaEnrollmentRequired": {"type": "boolean"},
                "mfaChallengeToken": {"type": "string"},
                "mfaExpiresAt": {"type": "string", "format": "date-time", "nullable": True},
            },
            "required": ["accountId", "organizationId", "email", "roles", "tokens"],
        },
        "V1AuthMeResponse": {
            "type": "object",
            "properties": auth_account_props,
            "required": ["accountId", "organizationId", "email", "roles"],
        },
        "V1AuthRefreshResponse": {
            "type": "object",
            "properties": {"tokens": {"$ref": "#/components/schemas/V1AuthTokenPair"}},
            "required": ["tokens"],
        },
        "V1AuthMFAEnrollResponse": {
            "type": "object",
            "properties": {
                "otpauthUri": {"type": "string"},
                "secret": {"type": "string"},
            },
            "required": ["otpauthUri", "secret"],
        },
        "V1AuthMFAVerifyRequest": {
            "type": "object",
            "properties": {"code": {"type": "string"}},
            "required": ["code"],
        },
        "V1AuthMFADisableRequest": {
            "type": "object",
            "properties": {
                "currentPassword": {"type": "string"},
                "totpCode": {"type": "string"},
            },
            "required": ["currentPassword", "totpCode"],
        },
        "V1AuthSessionItem": {
            "type": "object",
            "properties": {
                "sessionId": dict(uuid_s),
                "organizationId": dict(uuid_s),
                "ipAddress": {"type": "string", "nullable": True},
                "userAgent": {"type": "string", "nullable": True},
                "createdAt": ts,
                "lastUsedAt": {"type": "string", "format": "date-time", "nullable": True},
                "expiresAt": ts,
                "status": {"type": "string"},
            },
            "required": ["sessionId", "organizationId", "createdAt", "expiresAt", "status"],
        },
        "V1AuthSessionsEnvelope": {
            "type": "object",
            "properties": {
                "sessions": {"type": "array", "items": {"$ref": "#/components/schemas/V1AuthSessionItem"}},
            },
            "required": ["sessions"],
        },
        "V1AdminAuthSessionsEnvelope": {
            "type": "object",
            "properties": {
                "sessions": {"type": "array", "items": {"$ref": "#/components/schemas/V1AuthSessionItem"}},
            },
            "required": ["sessions"],
        },
        "V1AuthRevokeOtherSessionsRequest": {
            "type": "object",
            "properties": {"exceptRefreshToken": {"type": "string"}},
            "required": ["exceptRefreshToken"],
        },
        "V1CommerceCashCheckoutResponse": {
            "type": "object",
            "properties": {
                "order_id": dict(uuid_s),
                "vend_session_id": dict(uuid_s),
                "payment_id": dict(uuid_s),
                "order_status": {"$ref": "#/components/schemas/V1CommerceOrderStatus"},
                "payment_state": {"$ref": "#/components/schemas/V1PaymentState"},
                "replay": {"type": "boolean"},
            },
            "required": ["order_id", "vend_session_id", "payment_id", "order_status", "payment_state", "replay"],
        },
        "V1AdminPageMeta": page_meta,
        "V1AdminProductListItem": product_list_item,
        "V1AdminProductListEnvelope": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminProductListItem"}},
                "meta": {"$ref": "#/components/schemas/V1AdminPageMeta"},
            },
            "required": ["items", "meta"],
        },
        "V1AdminProduct": product_detail,
        "V1AdminPriceBook": price_book,
        "V1AdminPriceBookListEnvelope": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminPriceBook"}},
                "meta": {"$ref": "#/components/schemas/V1AdminPageMeta"},
            },
            "required": ["items", "meta"],
        },
        "V1AdminPricingPreviewLine": preview_line,
        "V1AdminPricingPreviewResponse": preview_resp,
        "V1AdminPlanogram": planogram,
        "V1AdminPlanogramSlot": planogram_slot,
        "V1AdminPlanogramListEnvelope": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminPlanogram"}},
                "meta": {"$ref": "#/components/schemas/V1AdminPageMeta"},
            },
            "required": ["items", "meta"],
        },
        "V1AdminPlanogramDetail": {
            "type": "object",
            "properties": {
                "planogram": {"$ref": "#/components/schemas/V1AdminPlanogram"},
                "slots": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminPlanogramSlot"}},
            },
            "required": ["planogram", "slots"],
        },
        "V1AdminPromotion": {
            "type": "object",
            "properties": {
                "id": dict(uuid_s),
                "organizationId": dict(uuid_s),
                "name": {"type": "string"},
                "approvalStatus": {"type": "string"},
                "lifecycleStatus": {"type": "string"},
                "priority": {"type": "integer", "format": "int32"},
                "stackable": {"type": "boolean"},
                "startsAt": ts,
                "endsAt": ts,
                "budgetLimitMinor": {"type": "integer", "format": "int64", "nullable": True},
                "redemptionLimit": {"type": "integer", "format": "int32", "nullable": True},
                "channelScope": {"type": "string", "nullable": True},
                "createdAt": ts,
                "updatedAt": ts,
            },
            "required": [
                "id",
                "organizationId",
                "name",
                "approvalStatus",
                "lifecycleStatus",
                "priority",
                "stackable",
                "startsAt",
                "endsAt",
                "createdAt",
                "updatedAt",
            ],
        },
        "V1AdminPromotionRule": {
            "type": "object",
            "properties": {
                "id": dict(uuid_s),
                "promotionId": dict(uuid_s),
                "ruleType": {"type": "string"},
                "priority": {"type": "integer", "format": "int32"},
                "payload": {"type": "object", "additionalProperties": True},
            },
            "required": ["id", "promotionId", "ruleType", "priority", "payload"],
        },
        "V1AdminPromotionTarget": {
            "type": "object",
            "properties": {
                "id": dict(uuid_s),
                "promotionId": dict(uuid_s),
                "organizationId": dict(uuid_s),
                "targetType": {"type": "string"},
                "productId": {**dict(uuid_s), "nullable": True},
                "categoryId": {**dict(uuid_s), "nullable": True},
                "machineId": {**dict(uuid_s), "nullable": True},
                "siteId": {**dict(uuid_s), "nullable": True},
                "organizationTargetId": {**dict(uuid_s), "nullable": True},
                "tagId": {**dict(uuid_s), "nullable": True},
                "createdAt": ts,
            },
            "required": ["id", "promotionId", "organizationId", "targetType", "createdAt"],
        },
        "V1AdminPromotionDetail": {
            "type": "object",
            "properties": {
                "promotion": {"$ref": "#/components/schemas/V1AdminPromotion"},
                "rules": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminPromotionRule"}},
                "targets": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminPromotionTarget"}},
            },
            "required": ["promotion", "rules", "targets"],
        },
        "V1AdminPromotionListEnvelope": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminPromotion"}},
                "meta": {"$ref": "#/components/schemas/V1AdminPageMeta"},
            },
            "required": ["items", "meta"],
        },
        "V1AdminPromotionSkippedRule": {
            "type": "object",
            "properties": {
                "promotionId": dict(uuid_s),
                "ruleId": dict(uuid_s),
                "ruleType": {"type": "string"},
                "reason": {"type": "string"},
            },
            "required": ["promotionId", "ruleType", "reason"],
        },
        "V1AdminPromotionPreviewLine": {
            "type": "object",
            "properties": {
                "productId": dict(uuid_s),
                "basePriceMinor": {"type": "integer", "format": "int64"},
                "discountMinor": {"type": "integer", "format": "int64"},
                "finalPriceMinor": {"type": "integer", "format": "int64"},
                "currency": {"type": "string"},
                "appliedPromotionIds": {"type": "array", "items": dict(uuid_s)},
                "appliedRuleIds": {"type": "array", "items": {"type": "string"}},
                "skippedRules": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminPromotionSkippedRule"}},
            },
            "required": [
                "productId",
                "basePriceMinor",
                "discountMinor",
                "finalPriceMinor",
                "currency",
                "appliedPromotionIds",
                "appliedRuleIds",
                "skippedRules",
            ],
        },
        "V1AdminPromotionPreviewResponse": {
            "type": "object",
            "properties": {
                "at": ts,
                "lines": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminPromotionPreviewLine"}},
            },
            "required": ["at", "lines"],
        },
        "V1AuthChangePasswordRequest": {
            "type": "object",
            "properties": {
                "currentPassword": {"type": "string"},
                "newPassword": {"type": "string", "minLength": 10},
            },
            "required": ["currentPassword", "newPassword"],
        },
        "V1AuthPasswordResetRequest": {
            "type": "object",
            "properties": {
                "organizationId": dict(uuid_s),
                "email": {"type": "string", "format": "email"},
            },
            "required": ["organizationId", "email"],
        },
        "V1AuthPasswordResetAccepted": {
            "type": "object",
            "properties": {"accepted": {"type": "boolean"}},
            "required": ["accepted"],
        },
        "V1AuthPasswordResetConfirmRequest": {
            "type": "object",
            "properties": {
                "token": {"type": "string"},
                "newPassword": {"type": "string", "minLength": 10},
            },
            "required": ["token", "newPassword"],
        },
        "V1AdminAuthAccount": {
            "type": "object",
            "properties": {
                "accountId": dict(uuid_s),
                "organizationId": dict(uuid_s),
                "email": {"type": "string", "format": "email"},
                "roles": {"type": "array", "items": {"type": "string"}},
                "status": {"type": "string", "enum": ["active", "disabled", "locked", "invited"]},
                "createdAt": ts,
                "updatedAt": ts,
            },
            "required": ["accountId", "organizationId", "email", "roles", "status", "createdAt", "updatedAt"],
        },
        "V1AdminAuthUsersListEnvelope": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminAuthAccount"}},
                "meta": {"$ref": "#/components/schemas/V1CollectionListMeta"},
                "rbacReference": {
                    "allOf": [{"$ref": "#/components/schemas/V1RBACPermissionMatrixDoc"}],
                    "nullable": True,
                    "description": "Reserved for documentation; production responses omit this field.",
                },
            },
            "required": ["items", "meta"],
        },
        "V1RBACPermissionMatrixDoc": {
            "type": "object",
            "description": "Documentation-only RBAC reference; JWT roles expand to fine-grained permissions server-side (internal/platform/auth/permissions.go).",
            "properties": {
                "permissionExamples": {
                    "type": "array",
                    "items": {"type": "string"},
                    "example": [
                        "user:read",
                        "user:write",
                        "user:roles",
                        "user:sessions:revoke",
                        "catalog:read",
                        "catalog:write",
                        "media:write",
                        "fleet:read",
                        "fleet:write",
                        "machine:command",
                        "inventory:read",
                        "inventory:write",
                        "payment:read",
                        "payment:refund",
                        "report:read",
                        "audit:read",
                        "technician:operate",
                        "setup:machine",
                        "ota:read",
                    ],
                },
                "roleSummary": {
                    "type": "string",
                    "example": "platform_admin→admin.all (explicit org routes); org_admin→org matrix + JWT org scope only; org_member/viewer→read-only baseline",
                },
                "auditActionsNote": {
                    "type": "string",
                    "example": "auth.user.*, role.changed, auth.login.success/failed, auth.mfa.*, auth.session.*, user.sessions.revoked",
                },
            },
        },
        "V1AdminOutboxPipelineStats": {
            "type": "object",
            "properties": {
                "pendingTotal": {"type": "integer", "format": "int64"},
                "pendingDueNow": {"type": "integer", "format": "int64"},
                "deadLetteredTotal": {"type": "integer", "format": "int64"},
                "publishingLeasedTotal": {"type": "integer", "format": "int64"},
                "maxPendingAttempts": {"type": "integer", "format": "int64"},
                "oldestPendingCreatedAt": ts,
            },
            "required": ["pendingTotal", "pendingDueNow", "deadLetteredTotal", "publishingLeasedTotal", "maxPendingAttempts"],
        },
        "V1AdminOutboxRow": {
            "type": "object",
            "properties": {
                "id": {"type": "integer", "format": "int64"},
                "organizationId": dict(uuid_s),
                "topic": {"type": "string"},
                "eventType": {"type": "string"},
                "payload": {"type": "object", "additionalProperties": True},
                "aggregateType": {"type": "string"},
                "aggregateId": {"type": "string"},
                "idempotencyKey": {"type": "string"},
                "createdAt": ts,
                "publishedAt": ts,
                "publishAttemptCount": {"type": "integer", "format": "int32"},
                "lastPublishError": {"type": "string"},
                "lastPublishAttemptAt": ts,
                "nextPublishAfter": ts,
                "deadLetteredAt": ts,
                "status": {"type": "string"},
                "lockedBy": {"type": "string"},
                "lockedUntil": ts,
            },
            "required": ["id", "topic", "eventType", "payload", "aggregateType", "aggregateId", "createdAt", "publishAttemptCount", "status"],
        },
        "V1AdminOutboxOpsEnvelope": {
            "type": "object",
            "properties": {
                "stats": {"$ref": "#/components/schemas/V1AdminOutboxPipelineStats"},
                "rows": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminOutboxRow"}},
                "meta": {"$ref": "#/components/schemas/V1CollectionListMeta"},
            },
            "required": ["stats", "rows", "meta"],
        },
        "V1AdminRetentionTableStatus": {
            "type": "object",
            "properties": {
                "tableName": {"type": "string"},
                "totalRows": {"type": "integer", "format": "int64"},
                "oldestRecordAt": ts,
                "oldestRecordAgeDays": {"type": "integer", "format": "int64"},
            },
            "required": ["tableName", "totalRows"],
        },
        "V1AdminRetentionOpsEnvelope": {
            "type": "object",
            "properties": {
                "tables": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminRetentionTableStatus"}},
            },
            "required": ["tables"],
        },
        "V1AdminSystemRetentionTableRow": {
            "type": "object",
            "properties": {
                "tableName": {"type": "string"},
                "totalRows": {"type": "integer", "format": "int64"},
                "oldestRecordAt": ts,
            },
            "required": ["tableName", "totalRows"],
        },
        "V1AdminRetentionPolicySnapshot": {
            "type": "object",
            "properties": {
                "telemetryRetentionDays": {"type": "integer", "format": "int32"},
                "telemetryCriticalRetentionDays": {"type": "integer", "format": "int32"},
                "auditRetentionDays": {"type": "integer", "format": "int32"},
                "commandRetentionDays": {"type": "integer", "format": "int32"},
                "commandReceiptRetentionDays": {"type": "integer", "format": "int32"},
                "paymentWebhookEventRetentionDays": {"type": "integer", "format": "int32"},
                "outboxPublishedRetentionDays": {"type": "integer", "format": "int32"},
                "processedMessageRetentionDays": {"type": "integer", "format": "int32"},
                "offlineEventRetentionDays": {"type": "integer", "format": "int32"},
                "inventoryEventRetentionDays": {"type": "integer", "format": "int32"},
            },
            "required": [
                "telemetryRetentionDays",
                "telemetryCriticalRetentionDays",
                "auditRetentionDays",
                "commandRetentionDays",
                "commandReceiptRetentionDays",
                "paymentWebhookEventRetentionDays",
                "outboxPublishedRetentionDays",
                "processedMessageRetentionDays",
                "offlineEventRetentionDays",
                "inventoryEventRetentionDays",
            ],
        },
        "V1AdminRetentionRuntimeFlags": {
            "type": "object",
            "properties": {
                "enableRetentionWorker": {"type": "boolean"},
                "telemetryCleanupEnabled": {"type": "boolean"},
                "enterpriseCleanupEnabled": {"type": "boolean"},
                "globalDryRun": {"type": "boolean"},
                "destructiveRetentionAllowed": {"type": "boolean"},
            },
            "required": [
                "enableRetentionWorker",
                "telemetryCleanupEnabled",
                "enterpriseCleanupEnabled",
                "globalDryRun",
                "destructiveRetentionAllowed",
            ],
        },
        "V1AdminSystemRetentionStatsEnvelope": {
            "type": "object",
            "properties": {
                "tables": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminSystemRetentionTableRow"}},
                "policy": {"$ref": "#/components/schemas/V1AdminRetentionPolicySnapshot"},
                "runtime": {"$ref": "#/components/schemas/V1AdminRetentionRuntimeFlags"},
            },
            "required": ["tables", "policy", "runtime"],
        },
        "V1AdminSystemRetentionTelemetryOutcome": {
            "type": "object",
            "properties": {
                "enabled": {"type": "boolean"},
                "dryRun": {"type": "boolean"},
                "stages": {"type": "object", "additionalProperties": {"type": "integer", "format": "int64"}},
            },
            "required": ["enabled", "dryRun"],
        },
        "V1AdminSystemRetentionEnterpriseOutcome": {
            "type": "object",
            "properties": {
                "enabled": {"type": "boolean"},
                "dryRun": {"type": "boolean"},
                "candidates": {"type": "object", "additionalProperties": {"type": "integer", "format": "int64"}},
                "deleted": {"type": "object", "additionalProperties": {"type": "integer", "format": "int64"}},
            },
            "required": ["enabled", "dryRun"],
        },
        "V1AdminSystemRetentionRunEnvelope": {
            "type": "object",
            "properties": {
                "telemetry": {"$ref": "#/components/schemas/V1AdminSystemRetentionTelemetryOutcome"},
                "enterprise": {"$ref": "#/components/schemas/V1AdminSystemRetentionEnterpriseOutcome"},
                "overallDryRun": {"type": "boolean"},
                "wouldModifyDatabase": {"type": "boolean"},
            },
            "required": ["telemetry", "enterprise", "overallDryRun", "wouldModifyDatabase"],
        },
        "V1AdminOutboxRetryEnvelope": {
            "type": "object",
            "properties": {"retried": {"type": "boolean"}},
            "required": ["retried"],
        },
        "V1AdminOutboxStatsEnvelope": {
            "type": "object",
            "properties": {"stats": {"$ref": "#/components/schemas/V1AdminOutboxPipelineStats"}},
            "required": ["stats"],
        },
        "V1AdminOutboxMarkDLQEnvelope": {
            "type": "object",
            "properties": {"marked": {"type": "boolean"}},
            "required": ["marked"],
        },
        "V1AdminMediaUploadInitResponse": {
            "type": "object",
            "properties": {
                "media_id": dict(uuid_s),
                "upload_url": {"type": "string", "format": "uri"},
                "upload_method": {"type": "string"},
                "upload_headers": {"type": "object", "additionalProperties": {"type": "array", "items": {"type": "string"}}},
                "expires_at": ts,
                "complete_path": {"type": "string"},
            },
            "required": ["media_id", "upload_url", "upload_method", "upload_headers", "expires_at", "complete_path"],
        },
        "V1AdminMediaAsset": {
            "type": "object",
            "properties": {
                "id": dict(uuid_s),
                "organization_id": dict(uuid_s),
                "kind": {"type": "string"},
                "status": {"type": "string"},
                "mime_type": {"type": "string"},
                "size_bytes": {"type": "integer", "format": "int64"},
                "sha256": {"type": "string"},
                "width": {"type": "integer", "format": "int32"},
                "height": {"type": "integer", "format": "int32"},
                "object_version": {"type": "integer", "format": "int32"},
                "etag": {"type": "string"},
                "created_at": ts,
                "updated_at": ts,
            },
            "required": ["id", "organization_id", "kind", "status", "object_version", "created_at", "updated_at"],
        },
        "V1AdminMediaListEnvelope": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {"$ref": "#/components/schemas/V1AdminMediaAsset"}},
                "meta": {"$ref": "#/components/schemas/V1AdminPageMeta"},
            },
            "required": ["items", "meta"],
        },
        "V1AdminAuthUsersCreateRequest": {
            "type": "object",
            "properties": {
                "email": {"type": "string", "format": "email"},
                "password": {"type": "string", "minLength": 10},
                "roles": {"type": "array", "items": {"type": "string"}, "minItems": 1},
                "status": {"type": "string", "enum": ["active", "disabled", "locked", "invited"]},
            },
            "required": ["email", "password", "roles"],
        },
        "V1AdminAuthUsersPatchRequest": {
            "type": "object",
            "properties": {
                "email": {"type": "string", "format": "email"},
                "roles": {"type": "array", "items": {"type": "string"}},
                "status": {"type": "string", "enum": ["active", "disabled", "locked", "invited"]},
            },
        },
        "V1AdminAuthUsersStatusPatchRequest": {
            "type": "object",
            "properties": {
                "status": {"type": "string", "enum": ["active", "disabled", "locked", "invited"]},
            },
            "required": ["status"],
        },
        "V1AdminAuthResetPasswordRequest": {
            "type": "object",
            "properties": {
                "password": {"type": "string", "minLength": 10},
            },
            "required": ["password"],
        },
        "V1EnterpriseAuditEvent": {
            "type": "object",
            "properties": {
                "id": uuid_s,
                "organizationId": uuid_s,
                "actorType": {"type": "string"},
                "actorId": {"type": "string", "format": "uuid", "nullable": True},
                "action": {"type": "string"},
                "resourceType": {"type": "string"},
                "resourceId": {"type": "string", "nullable": True},
                "machineId": {"type": "string", "format": "uuid", "nullable": True},
                "siteId": {"type": "string", "format": "uuid", "nullable": True},
                "requestId": {"type": "string", "nullable": True},
                "traceId": {"type": "string", "nullable": True},
                "ipAddress": {"type": "string", "nullable": True},
                "userAgent": {"type": "string", "nullable": True},
                "beforeJson": {"type": "object", "nullable": True},
                "afterJson": {"type": "object", "nullable": True},
                "metadata": {"type": "object"},
                "outcome": {"type": "string", "enum": ["success", "failure"]},
                "occurredAt": ts,
                "createdAt": ts,
            },
            "required": [
                "id",
                "organizationId",
                "actorType",
                "action",
                "resourceType",
                "metadata",
                "outcome",
                "occurredAt",
                "createdAt",
            ],
        },
        "V1EnterpriseAuditEventsListEnvelope": {
            "type": "object",
            "properties": {
                "items": {"type": "array", "items": {"$ref": "#/components/schemas/V1EnterpriseAuditEvent"}},
                "meta": {"$ref": "#/components/schemas/V1CollectionListMeta"},
            },
            "required": ["items", "meta"],
        },
    }


def components() -> dict[str, Any]:
    err = v1_api_error_schema()
    schemas: dict[str, Any] = {
        "V1APIErrorEnvelope": err,
        "V1StandardError": err,
        "V1NotImplementedError": err,
        "V1CapabilityNotConfiguredError": err,
        "V1BearerAuthError": err,
        "V1VersionPayload": {
            "type": "object",
            "properties": {
                "name": {"type": "string", "description": "Binary / module name"},
                "version": {"type": "string", "description": "Semantic version from build metadata"},
                "git_sha": {"type": "string"},
                "build_time": {"type": "string"},
                "app_env": {"type": "string", "description": "Application environment (e.g. development, staging, production)"},
                "process": {"type": "string", "description": "Process name (api, worker, …)"},
                "runtime_role": {"type": "string"},
                "node_name": {"type": "string"},
                "instance_id": {"type": "string"},
                "public_base_url": {"type": "string", "description": "Configured public HTTP base URL when set"},
                "machine_public_base_url": {"type": "string", "description": "Machine-facing base URL when set"},
            },
            "required": ["name", "version", "app_env"],
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
                "order_status": {"$ref": "#/components/schemas/V1CommerceOrderStatus"},
                "vend_state": {"$ref": "#/components/schemas/V1VendSessionState"},
            },
            "required": ["order_id", "vend_session_id", "replay", "order_status", "vend_state"],
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
        "V1VendSessionState": {"type": "string", "enum": ["pending", "in_progress", "success", "failed"]},
        "V1PaymentState": {
            "type": "string",
            "description": "Normalized payment.state for commerce APIs.",
            "enum": ["created", "authorized", "captured", "failed", "refunded"],
        },
        "V1OperatorSessionStatus": {"type": "string", "enum": ["ACTIVE", "ENDED", "EXPIRED", "REVOKED"]},
        "V1OperatorActorType": {"type": "string", "enum": ["TECHNICIAN", "USER"]},
        "V1OperatorAuthMethod": {
            "type": "string",
            "enum": ["pin", "password", "badge", "oidc", "device_cert", "unknown"],
        },
    }
    schemas.update(enterprise_error_named_schemas())
    schemas.update(operational_collection_component_schemas())
    schemas.update(admin_operations_component_schemas())
    schemas.update(machine_setup_component_schemas())
    schemas.update(cash_settlement_component_schemas())
    schemas.update(reporting_component_schemas())
    schemas.update(missing_reference_component_schemas())
    return {
        "schemas": schemas,
        "securitySchemes": {
            "bearerAuth": {
                "type": "http",
                "scheme": "bearer",
                "bearerFormat": "JWT",
                "description": (
                    "Send `Authorization: Bearer <JWT>`. Mode is selected by `HTTP_AUTH_MODE` "
                    "(hs256 default, rs256_pem, rs256_jwks). Errors use the same JSON envelope as handlers "
                    "with `error.code` (e.g. unauthenticated, forbidden, auth_misconfigured)."
                ),
            }
        },
        "parameters": {
            "AuthorizationHeader": {
                "name": "Authorization",
                "in": "header",
                "required": True,
                "description": "Bearer JWT access token.",
                "schema": {"type": "string"},
            },
            "XRequestID": {
                "name": "X-Request-ID",
                "in": "header",
                "required": False,
                "description": "Optional client-provided request id; echoed on the response when middleware is enabled.",
                "schema": {"type": "string"},
            },
            "XCorrelationID": {
                "name": "X-Correlation-ID",
                "in": "header",
                "required": False,
                "description": "Optional correlation id (`X-Correlation-Id` accepted); echoed as `X-Correlation-ID` on the response.",
                "schema": {"type": "string"},
            },
            "IdempotencyKeyHeader": {
                "name": "Idempotency-Key",
                "in": "header",
                "required": True,
                "description": "Required on mutating commerce/command routes; `X-Idempotency-Key` is accepted as an alias.",
                "schema": {"type": "string"},
            },
        },
    }


def _param_ref(name: str) -> dict[str, Any]:
    return {"$ref": f"#/components/parameters/{name}"}


def merge_global_parameters(path: str, op: dict[str, Any]) -> None:
    """Attach reusable header parameters; avoid duplicates with explicit @Param entries."""
    params = op.setdefault("parameters", [])
    existing_names: set[str] = set()
    for p in params:
        if isinstance(p, dict) and "name" in p:
            existing_names.add(str(p["name"]))

    def append_ref(ref_name: str, header_name: str) -> None:
        if header_name in existing_names:
            return
        params.append(_param_ref(ref_name))
        existing_names.add(header_name)

    if path.startswith("/v1/"):
        if op.get("security"):
            append_ref("AuthorizationHeader", "Authorization")
        append_ref("XRequestID", "X-Request-ID")
        append_ref("XCorrelationID", "X-Correlation-ID")


# Machine-facing REST surfaces gated by transport_legacy_guard (prefer native gRPC; see docs/architecture/transport-boundary.md).
LEGACY_MACHINE_REST_DEPRECATED: frozenset[tuple[str, str]] = frozenset(
    {
        ("post", "/v1/machines/{machineId}/check-ins"),
        ("post", "/v1/machines/{machineId}/config-applies"),
        ("post", "/v1/device/machines/{machineId}/vend-results"),
        ("post", "/v1/device/machines/{machineId}/commands/poll"),
        ("post", "/v1/device/machines/{machineId}/events/reconcile"),
        ("get", "/v1/device/machines/{machineId}/events/{idempotencyKey}/status"),
    }
)


def mark_deprecated_machine_legacy_rest(paths: dict[str, Any]) -> None:
    """Mark legacy machine HTTP bridge routes as deprecated in the public OpenAPI document."""
    for method, legacy_path in LEGACY_MACHINE_REST_DEPRECATED:
        entry = paths.get(legacy_path)
        if not isinstance(entry, dict):
            continue
        op_obj = entry.get(method)
        if isinstance(op_obj, dict):
            op_obj["deprecated"] = True


IDEMPOTENCY_OPS: set[tuple[str, str]] = {
    ("post", "/v1/commerce/orders"),
    ("post", "/v1/commerce/cash-checkout"),
    ("post", "/v1/commerce/orders/{orderId}/payment-session"),
    ("post", "/v1/commerce/orders/{orderId}/vend/start"),
    ("post", "/v1/commerce/orders/{orderId}/vend/success"),
    ("post", "/v1/commerce/orders/{orderId}/vend/failure"),
    ("post", "/v1/commerce/orders/{orderId}/cancel"),
    ("post", "/v1/commerce/orders/{orderId}/refunds"),
    ("post", "/v1/admin/organizations/{organizationId}/orders/{orderId}/refunds"),
    ("post", "/v1/device/machines/{machineId}/vend-results"),
    ("post", "/v1/machines/{machineId}/commands/dispatch"),
    ("post", "/v1/admin/machines/{machineId}/planograms/publish"),
    ("post", "/v1/admin/machines/{machineId}/sync"),
    ("post", "/v1/admin/machines/{machineId}/stock-adjustments"),
    ("post", "/v1/admin/machines/{machineId}/cash-collections"),
    ("post", "/v1/admin/machines/{machineId}/diagnostics/requests"),
    ("post", "/v1/admin/products"),
    ("put", "/v1/admin/products/{productId}"),
    ("patch", "/v1/admin/products/{productId}"),
    ("delete", "/v1/admin/products/{productId}"),
    ("post", "/v1/admin/products/{productId}/image"),
    ("put", "/v1/admin/products/{productId}/image"),
    ("delete", "/v1/admin/products/{productId}/image"),
    ("post", "/v1/admin/media/assets"),
    ("post", "/v1/admin/media/uploads"),
    ("post", "/v1/admin/media/{mediaId}/complete"),
    ("post", "/v1/admin/organizations/{organizationId}/media/uploads/init"),
    ("post", "/v1/admin/organizations/{organizationId}/media/uploads/complete"),
    ("post", "/v1/admin/organizations/{organizationId}/media/product-images"),
    ("get", "/v1/admin/organizations/{organizationId}/media/assets"),
    ("get", "/v1/admin/organizations/{organizationId}/media/assets/{assetId}"),
    ("delete", "/v1/admin/organizations/{organizationId}/media/assets/{assetId}"),
    ("post", "/v1/admin/organizations/{organizationId}/products/{productId}/media"),
    ("delete", "/v1/admin/organizations/{organizationId}/products/{productId}/media/{mediaId}"),
    ("delete", "/v1/admin/media/assets/{mediaId}"),
    ("delete", "/v1/admin/media/{mediaId}"),
    ("post", "/v1/admin/products/{productId}/media"),
    ("put", "/v1/admin/products/{productId}/media"),
    ("delete", "/v1/admin/products/{productId}/media/{mediaId}"),
    ("post", "/v1/admin/organizations/{organizationId}/products/{productId}/images"),
    ("get", "/v1/admin/organizations/{organizationId}/products/{productId}/images"),
    ("patch", "/v1/admin/organizations/{organizationId}/products/{productId}/images/{imageId}"),
    ("delete", "/v1/admin/organizations/{organizationId}/products/{productId}/images/{imageId}"),
    ("post", "/v1/admin/brands"),
    ("put", "/v1/admin/brands/{brandId}"),
    ("patch", "/v1/admin/brands/{brandId}"),
    ("delete", "/v1/admin/brands/{brandId}"),
    ("post", "/v1/admin/categories"),
    ("put", "/v1/admin/categories/{categoryId}"),
    ("patch", "/v1/admin/categories/{categoryId}"),
    ("delete", "/v1/admin/categories/{categoryId}"),
    ("post", "/v1/admin/tags"),
    ("put", "/v1/admin/tags/{tagId}"),
    ("patch", "/v1/admin/tags/{tagId}"),
    ("delete", "/v1/admin/tags/{tagId}"),
    ("post", "/v1/admin/price-books"),
    ("patch", "/v1/admin/price-books/{priceBookId}"),
    ("post", "/v1/admin/price-books/{priceBookId}/deactivate"),
    ("post", "/v1/admin/price-books/{priceBookId}/activate"),
    ("post", "/v1/admin/price-books/{priceBookId}/archive"),
    ("put", "/v1/admin/price-books/{priceBookId}/items"),
    ("patch", "/v1/admin/price-books/{priceBookId}/items/{productId}"),
    ("delete", "/v1/admin/price-books/{priceBookId}/items/{productId}"),
    ("post", "/v1/admin/price-books/{priceBookId}/assign-target"),
    ("delete", "/v1/admin/price-books/{priceBookId}/targets/{targetId}"),
    ("post", "/v1/admin/promotions"),
    ("patch", "/v1/admin/promotions/{promotionId}"),
    ("post", "/v1/admin/promotions/{promotionId}/activate"),
    ("post", "/v1/admin/promotions/{promotionId}/pause"),
    ("post", "/v1/admin/promotions/{promotionId}/deactivate"),
    ("post", "/v1/admin/promotions/{promotionId}/archive"),
    ("post", "/v1/admin/promotions/{promotionId}/assign-target"),
    ("delete", "/v1/admin/promotions/{promotionId}/targets/{targetId}"),
    ("post", "/v1/admin/ota/campaigns"),
    ("patch", "/v1/admin/ota/campaigns/{campaignId}"),
    ("put", "/v1/admin/ota/campaigns/{campaignId}/targets"),
    ("post", "/v1/admin/ota/campaigns/{campaignId}/approve"),
    ("post", "/v1/admin/ota/campaigns/{campaignId}/start"),
    ("post", "/v1/admin/ota/campaigns/{campaignId}/publish"),
    ("post", "/v1/admin/ota/campaigns/{campaignId}/pause"),
    ("post", "/v1/admin/ota/campaigns/{campaignId}/resume"),
    ("post", "/v1/admin/ota/campaigns/{campaignId}/cancel"),
    ("post", "/v1/admin/ota/campaigns/{campaignId}/rollback"),
    ("post", "/v1/admin/organizations/{organizationId}/machines/{machineId}/commands"),
}


def merge_idempotency_parameter(method: str, path: str, op: dict[str, Any]) -> None:
    if (method, path) not in IDEMPOTENCY_OPS:
        return
    params = op.setdefault("parameters", [])
    if any(isinstance(p, dict) and p.get("name") == "Idempotency-Key" for p in params):
        return
    params.append(_param_ref("IdempotencyKeyHeader"))


def operation_examples() -> dict[tuple[str, str], dict[str, Any]]:
    """Per-operation request/response examples keyed by (lowercase method, path)."""
    checkout = {
        "order": {
            "id": _U,
            "organization_id": _U2,
            "machine_id": _U3,
            "status": "paid",
            "currency": "USD",
            "subtotal_minor": 125,
            "tax_minor": 10,
            "total_minor": 135,
            "created_at": "2026-04-19T12:00:00Z",
            "updated_at": "2026-04-19T12:05:00Z",
        },
        "vend": {
            "id": "8d3e2f10-1111-2222-3333-444455556666",
            "order_id": _U,
            "machine_id": _U3,
            "slot_index": 3,
            "product_id": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
            "state": "in_progress",
            "created_at": "2026-04-19T12:00:01Z",
        },
        "payment": {
            "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
            "order_id": _U,
            "provider": "stripe",
            "state": "captured",
            "amount_minor": 135,
            "currency": "USD",
            "created_at": "2026-04-19T12:04:00Z",
        },
    }
    disp = {
        "command_id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
        "sequence": 42,
        "attempt_id": "cccccccc-dddd-eeee-ffff-000000000001",
        "replay": False,
        "dispatch_state": "published",
    }
    st = {
        "machine_id": _U3,
        "command_id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
        "sequence": 42,
        "command_type": "SET_TEMPERATURE",
        "dispatch_state": "published",
        "attempt": {
            "id": "cccccccc-dddd-eeee-ffff-000000000001",
            "attempt_no": 1,
            "status": "sent",
            "sent_at": "2026-04-19T12:00:10Z",
            "ack_deadline_at": "2026-04-19T12:00:40Z",
        },
    }
    shadow = {
        "machine_id": _U3,
        "reported": {"temperature_c": 4.5},
        "desired": {"temperature_c": 4.0},
        "metadata": {"version": 12},
    }
    op_login = {
        "session": {
            "id": "dddddddd-eeee-ffff-0000-111111111111",
            "organization_id": _U2,
            "machine_id": _U3,
            "actor_type": "TECHNICIAN",
            "status": "ACTIVE",
            "started_at": "2026-04-19T12:10:00Z",
            "last_activity_at": "2026-04-19T12:10:05Z",
            "created_at": "2026-04-19T12:10:00Z",
            "updated_at": "2026-04-19T12:10:05Z",
            "technician_id": "eeeeeeee-ffff-0000-1111-222222222222",
            "client_metadata": {},
        }
    }
    art_reserve = {"artifact_id": "ffffffff-0000-1111-2222-333333333333", "upload_path": "org/acme/artifacts/ff/..."}

    cmeta = {"limit": 50, "offset": 0, "returned": 1, "total": 42}
    ord_item = {
        "orderId": _U,
        "organizationId": _U2,
        "machineId": _U3,
        "status": "paid",
        "currency": "USD",
        "subtotalMinor": 100,
        "taxMinor": 0,
        "totalMinor": 100,
        "createdAt": "2026-04-19T12:00:00Z",
        "updatedAt": "2026-04-19T12:05:00Z",
    }
    pay_item = {
        "paymentId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
        "orderId": _U,
        "organizationId": _U2,
        "machineId": _U3,
        "provider": "stripe",
        "paymentState": "captured",
        "orderStatus": "paid",
        "amountMinor": 100,
        "currency": "USD",
        "reconciliationStatus": "pending",
        "settlementStatus": "unsettled",
        "createdAt": "2026-04-19T12:04:00Z",
        "updatedAt": "2026-04-19T12:04:01Z",
    }
    recon_case = {
        "id": "99999999-8888-7777-6666-555555555555",
        "organizationId": _U2,
        "caseType": "payment_paid_vend_failed",
        "status": "open",
        "severity": "critical",
        "orderId": _U,
        "paymentId": pay_item["paymentId"],
        "reason": "captured payment is attached to a failed vend",
        "metadata": {"payment_state": "captured", "vend_state": "failed"},
        "firstDetectedAt": "2026-04-19T12:10:00Z",
        "lastDetectedAt": "2026-04-19T12:10:00Z",
    }
    mach_item = {
        "machineId": _U3,
        "machineName": "Lobby A",
        "organizationId": _U2,
        "siteId": "11111111-2222-3333-4444-555555555555",
        "siteName": "Main Campus",
        "serialNumber": "SN-001",
        "name": "Lobby A",
        "status": "online",
        "commandSequence": 12,
        "createdAt": "2026-01-01T00:00:00.000000000Z",
        "updatedAt": "2026-04-19T10:00:00.000000000Z",
        "effectiveTimezone": "America/Los_Angeles",
        "assignedTechnicians": [],
        "inventorySummary": {
            "totalSlots": 24,
            "occupiedSlots": 18,
            "lowStockSlots": 2,
            "outOfStockSlots": 0,
        },
    }
    telemetry_snapshot_ex = {
        "machineId": _U3,
        "organizationId": _U2,
        "siteId": "11111111-2222-3333-4444-555555555555",
        "reportedState": {"temperature_c": 4.5},
        "metricsState": {"cpu_pct": 12.3},
        "lastHeartbeatAt": "2026-04-19T12:34:56.789012345Z",
        "appVersion": "1.2.3",
        "firmwareVersion": "fw-9",
        "updatedAt": "2026-04-19T12:35:00.000000001Z",
        "androidId": "dev123",
        "simSerial": "89012601234567890123",
        "simIccid": "89012601234567890123",
        "deviceModel": "Pixel",
        "osVersion": "14",
        "lastIdentityAt": "2026-04-19T12:30:00.111111111Z",
        "effectiveTimezone": "America/Los_Angeles",
    }
    telemetry_incidents_ex = {
        "items": [
            {
                "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
                "severity": "warning",
                "code": "TEMP_HIGH",
                "title": "Cabinet warm",
                "detail": {"threshold_c": 8},
                "dedupeKey": "TEMP_HIGH:slot3",
                "openedAt": "2026-04-19T12:00:00.000000000Z",
                "updatedAt": "2026-04-19T12:05:00.000000000Z",
            }
        ],
        "meta": {"limit": 50, "returned": 1},
    }
    telemetry_rollups_ex = {
        "items": [
            {
                "bucketStart": "2026-04-19T12:00:00.000000000Z",
                "granularity": "1m",
                "metricKey": "temperature_c",
                "sampleCount": 60,
                "sum": 420.5,
                "min": 6.5,
                "max": 8.2,
                "last": 7.1,
                "extra": {},
            }
        ],
        "meta": {
            "granularity": "1m",
            "from": "2026-04-18T12:00:00.000000000Z",
            "to": "2026-04-19T12:00:00.000000000Z",
            "returned": 1,
            "note": "Rollup buckets only — not raw MQTT telemetry history.",
        },
    }
    tech_item = {
        "technicianId": "eeeeeeee-ffff-0000-1111-222222222222",
        "organizationId": _U2,
        "displayName": "Alex Tech",
        "createdAt": "2026-03-01T00:00:00Z",
    }
    asg_item = {
        "assignmentId": "dddddddd-eeee-ffff-0000-111111111111",
        "technicianId": "eeeeeeee-ffff-0000-1111-222222222222",
        "technicianDisplayName": "Alex Tech",
        "machineId": _U3,
        "machineName": "Lobby A",
        "machineSerialNumber": "SN-001",
        "role": "maintainer",
        "validFrom": "2026-04-01T00:00:00Z",
        "createdAt": "2026-04-01T00:00:00Z",
    }
    site_row = {
        "id": "aaaaaaaa-bbbb-cccc-dddd-111111111111",
        "organization_id": _U2,
        "name": "HQ Lobby",
        "timezone": "America/New_York",
        "code": "HQ-01",
        "status": "active",
        "address": {},
        "created_at": "2026-04-01T00:00:00.000000000Z",
        "updated_at": "2026-04-01T00:00:00.000000000Z",
    }
    machine_row_fleet = {
        "id": _U3,
        "organization_id": _U2,
        "site_id": site_row["id"],
        "serial_number": "SN-NEW",
        "name": "New unit",
        "status": "provisioning",
        "command_sequence": 0,
        "created_at": "2026-04-01T00:00:00.000000000Z",
        "updated_at": "2026-04-01T00:00:00.000000000Z",
    }
    technician_detail_snake = {
        "id": tech_item["technicianId"],
        "organization_id": _U2,
        "display_name": tech_item["displayName"],
        "status": "active",
        "created_at": "2026-03-01T00:00:00.000000000Z",
        "updated_at": "2026-03-01T00:00:00.000000000Z",
    }
    assignment_detail_snake = {
        "id": asg_item["assignmentId"],
        "organization_id": _U2,
        "technician_id": asg_item["technicianId"],
        "machine_id": asg_item["machineId"],
        "role": asg_item["role"],
        "status": "active",
        "valid_from": "2026-04-01T00:00:00.000000000Z",
        "created_at": "2026-04-01T00:00:00.000000000Z",
        "updated_at": "2026-04-01T00:00:00.000000000Z",
    }
    cmd_item = {
        "commandId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
        "machineId": _U3,
        "organizationId": _U2,
        "machineName": "Lobby A",
        "machineSerialNumber": "SN-001",
        "sequence": 42,
        "commandType": "SET_TEMPERATURE",
        "createdAt": "2026-04-19T12:00:00Z",
        "attemptCount": 1,
        "latestAttemptStatus": "sent",
    }
    ota_item = {
        "campaignId": "cccccccc-dddd-eeee-ffff-000000000002",
        "organizationId": _U2,
        "campaignName": "April bundle",
        "strategy": "rolling",
        "campaignStatus": "active",
        "createdAt": "2026-04-10T00:00:00Z",
        "artifactId": "dddddddd-eeee-ffff-0000-333333333333",
        "artifactStorageKey": "org/acme/ota/fw.bin",
    }
    audit_event_row_ex = {
        "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
        "organizationId": _U2,
        "actorType": "user",
        "actorId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
        "action": "catalog.product.update",
        "resourceType": "product",
        "resourceId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
        "machineId": _U3,
        "siteId": None,
        "metadata": {},
        "outcome": "success",
        "occurredAt": "2026-04-19T12:00:00Z",
        "createdAt": "2026-04-19T12:00:00Z",
    }
    audit_events_list_ex = {
        "items": [audit_event_row_ex],
        "meta": cmeta,
    }
    outbox_ops_list_ex = {
        "stats": {
            "pendingTotal": 3,
            "pendingDueNow": 2,
            "deadLetteredTotal": 1,
            "publishingLeasedTotal": 0,
            "maxPendingAttempts": 5,
            "oldestPendingCreatedAt": "2026-04-19T12:00:00.000000000Z",
        },
        "rows": [
            {
                "id": 101,
                "organizationId": _U2,
                "topic": "commerce.payments",
                "eventType": "payment.session_started",
                "payload": {},
                "aggregateType": "payment",
                "aggregateId": _U3,
                "createdAt": "2026-04-19T12:00:00.000000000Z",
                "publishAttemptCount": 0,
                "status": "pending",
            }
        ],
        "meta": {**cmeta, "returned": 1},
    }
    retention_ops_ex = {
        "tables": [
            {
                "tableName": "outbox_events",
                "totalRows": 120,
                "oldestRecordAt": "2026-04-01T00:00:00.000000000Z",
                "oldestRecordAgeDays": 28,
            },
            {
                "tableName": "payment_provider_events",
                "totalRows": 240,
                "oldestRecordAt": "2026-03-15T00:00:00.000000000Z",
                "oldestRecordAgeDays": 45,
            },
        ],
    }
    system_retention_stats_ex = {
        "tables": [
            {"tableName": "audit_events", "totalRows": 1000, "oldestRecordAt": "2025-01-01T00:00:00.000000000Z"},
            {"tableName": "inventory_events", "totalRows": 50000, "oldestRecordAt": "2025-06-01T00:00:00.000000000Z"},
        ],
        "policy": {
            "telemetryRetentionDays": 30,
            "telemetryCriticalRetentionDays": 365,
            "auditRetentionDays": 2555,
            "commandRetentionDays": 180,
            "commandReceiptRetentionDays": 180,
            "paymentWebhookEventRetentionDays": 365,
            "outboxPublishedRetentionDays": 30,
            "processedMessageRetentionDays": 30,
            "offlineEventRetentionDays": 180,
            "inventoryEventRetentionDays": 730,
        },
        "runtime": {
            "enableRetentionWorker": True,
            "telemetryCleanupEnabled": True,
            "enterpriseCleanupEnabled": True,
            "globalDryRun": False,
            "destructiveRetentionAllowed": True,
        },
    }
    system_retention_run_ex = {
        "telemetry": {
            "enabled": True,
            "dryRun": True,
            "stages": {"device_telemetry_events_non_critical": 1200},
        },
        "enterprise": {
            "enabled": True,
            "dryRun": True,
            "candidates": {"outbox_events_published": 12, "inventory_events": 9000},
        },
        "overallDryRun": True,
        "wouldModifyDatabase": False,
    }
    outbox_retry_ex = {"retried": True}
    outbox_stats_only_ex = {"stats": outbox_ops_list_ex["stats"]}
    outbox_mark_dlq_ex = {"marked": True}
    outbox_single_row_ex = {
        **outbox_ops_list_ex["rows"][0],
        "attempts": 0,
        "maxAttempts": 24,
        "idempotencyKey": "idem-pay-001",
        "lastPublishError": None,
        "lastPublishAttemptAt": None,
        "nextPublishAfter": None,
        "nextAttemptAt": None,
        "deadLetteredAt": None,
        "lockedBy": None,
        "lockedUntil": None,
        "updatedAt": "2026-04-19T12:00:00.000000000Z",
    }
    ff_row = {
        "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
        "organizationId": _U2,
        "flagKey": "kiosk.beta_ui",
        "displayName": "Beta UI",
        "description": "Experimental UI",
        "enabled": False,
        "metadata": {},
        "createdAt": "2026-04-01T00:00:00Z",
        "updatedAt": "2026-04-19T10:00:00Z",
    }
    ff_create_req = {
        "flagKey": "kiosk.beta_ui",
        "displayName": "Beta UI",
        "description": "Experimental UI",
        "enabled": False,
        "metadata": {},
    }
    ff_patch_req = {"displayName": "Beta UI v2", "enabled": True}
    ff_targets_req = {
        "targets": [
            {
                "targetType": "machine",
                "machineId": _U3,
                "priority": 10,
                "enabled": True,
                "metadata": {},
            }
        ]
    }
    ff_detail_ex = {"flag": ff_row, "targets": []}
    mcr_row = {
        "id": "77777777-8888-9999-aaaa-bbbbbbbbbbbb",
        "organizationId": _U2,
        "scopeType": "organization",
        "status": "pending",
        "targetVersionId": "11111111-2222-3333-4444-555555555555",
        "createdAt": "2026-04-19T12:00:00.000000000Z",
    }
    mcr_create_req = {"scopeType": "organization", "targetVersionId": "11111111-2222-3333-4444-555555555555"}
    ota_campaign_detail_ex = {
        "campaignId": "cccccccc-dddd-eeee-ffff-000000000002",
        "organizationId": _U2,
        "name": "April firmware",
        "rolloutStrategy": "canary",
        "status": "draft",
        "campaignType": "firmware",
        "canaryPercent": 10,
        "rolloutNextOffset": 0,
        "artifactId": "dddddddd-eeee-ffff-0000-333333333333",
        "artifactSemver": "1.2.3",
        "artifactStorageKey": "org/acme/ota/fw.bin",
        "artifactVersion": "1.2.3",
        "rollbackArtifactId": "11111111-2222-3333-4444-555555555555",
        "createdAt": "2026-04-10T00:00:00Z",
        "updatedAt": "2026-04-10T00:00:00Z",
        "approvedAt": "2026-04-10T00:00:00Z",
    }
    ota_campaigns_list_ex = {"items": [ota_campaign_detail_ex], "meta": cmeta}
    ota_targets_ex = {
        "items": [
            {"machineId": _U3, "state": "pending", "updatedAt": "2026-04-19T12:00:00.000000000Z"},
        ]
    }
    ota_results_ex = {
        "items": [
            {
                "machineId": _U3,
                "wave": "canary",
                "commandId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
                "status": "dispatched",
                "updatedAt": "2026-04-19T12:00:00.000000000Z",
                "createdAt": "2026-04-19T12:00:00.000000000Z",
            }
        ]
    }
    ota_create_req = {
        "name": "April firmware",
        "artifactId": "dddddddd-eeee-ffff-0000-333333333333",
        "artifactVersion": "1.2.3",
        "campaignType": "firmware",
        "rolloutStrategy": "canary",
        "canaryPercent": 10,
    }
    ota_patch_req = {"name": "April firmware (edited)"}
    ota_targets_put_req = {"machineIds": [_U3]}
    ota_rollback_req = {"rollbackArtifactId": "dddddddd-eeee-ffff-0000-333333333333"}

    bootstrap_resp = {
        "machine": {
            "machineId": _U3,
            "organizationId": _U2,
            "siteId": "11111111-2222-3333-4444-555555555555",
            "serialNumber": "SN-LOBBY-001",
            "name": "Lobby A",
            "status": "online",
            "commandSequence": 42,
            "createdAt": "2026-01-01T00:00:00.000000000Z",
            "updatedAt": "2026-04-19T12:00:00.000000000Z",
        },
        "topology": {
            "cabinets": [
                {
                    "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
                    "code": "A",
                    "title": "Main",
                    "sortOrder": 1,
                    "slots": [
                        {
                            "configId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
                            "slotCode": "A1",
                            "slotIndex": 1,
                            "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
                            "productSku": "COLA-12",
                            "productName": "Cola 12oz",
                            "maxQuantity": 10,
                            "priceMinor": 150,
                            "effectiveFrom": "2026-04-01T00:00:00.000000000Z",
                            "isCurrent": True,
                            "machineSlotLayout": "cccccccc-dddd-eeee-ffff-000000000001",
                            "metadata": {},
                        }
                    ],
                }
            ]
        },
        "catalog": {
            "products": [
                {
                    "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
                    "sku": "COLA-12",
                    "name": "Cola 12oz",
                    "sortOrder": 1,
                    "assortmentId": "dddddddd-eeee-ffff-0000-111111111111",
                    "assortmentName": "Standard",
                }
            ]
        },
    }
    topology_req = {
        "operator_session_id": "dddddddd-eeee-ffff-0000-111111111111",
        "cabinets": [{"code": "A", "title": "Main cabinet", "sortOrder": 1, "metadata": {}}],
        "layouts": [
            {
                "cabinetCode": "A",
                "layoutKey": "grid-4x6",
                "revision": 1,
                "layoutSpec": {"rows": 4, "cols": 6},
                "status": "active",
            }
        ],
    }
    planogram_draft_req = {
        "operator_session_id": "dddddddd-eeee-ffff-0000-111111111111",
        "planogramId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
        "planogramRevision": 3,
        "syncLegacyReadModel": True,
        "items": [
            {
                "cabinetCode": "A",
                "layoutKey": "grid-4x6",
                "layoutRevision": 1,
                "slotCode": "A3",
                "legacySlotIndex": 3,
                "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
                "maxQuantity": 12,
                "priceMinor": 150,
                "metadata": {},
            }
        ],
    }
    planogram_publish_resp = {
        "desiredConfigVersion": 7,
        "planogramId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
        "planogramRevision": 3,
        "command": {
            "commandId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
            "sequence": 43,
            "dispatchState": "published",
            "replay": False,
        },
    }
    sync_resp = {"command": planogram_publish_resp["command"]}
    stock_adj_req = {
        "operator_session_id": "dddddddd-eeee-ffff-0000-111111111111",
        "reason": "restock",
        "occurredAt": "2026-04-19T12:00:00.000000000Z",
        "items": [
            {
                "planogramId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
                "slotIndex": 3,
                "quantityBefore": 2,
                "quantityAfter": 10,
                "cabinetCode": "A",
                "slotCode": "A3",
                "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
            }
        ],
    }
    stock_adj_resp = {"replay": False, "eventIds": [1001, 1002]}

    cash_coll_start_req = {
        "operator_session_id": "dddddddd-eeee-ffff-0000-111111111111",
        "startedAt": "2026-04-24T00:00:00Z",
        "currency": "USD",
        "notes": "Field collection — tray A",
    }
    cash_coll_close_req = {
        "operator_session_id": "dddddddd-eeee-ffff-0000-111111111111",
        "countedCashboxMinor": 995000,
        "countedRecyclerMinor": 200000,
        "currency": "VND",
        "denominations": [{"denominationMinor": 10000, "count": 50}],
        "closedAt": "2026-04-24T00:10:00Z",
        "evidence": {"photoArtifactId": "22222222-3333-4444-5555-666666666666"},
        "notes": "Monthly collection",
    }
    cash_coll_open_ex = {
        "id": _U,
        "machine_id": _U3,
        "organization_id": _U2,
        "collected_at": "2026-04-19T14:00:00.000000000Z",
        "opened_at": "2026-04-19T14:00:00.000000000Z",
        "closed_at": None,
        "lifecycle_status": "open",
        "counted_amount_minor": 0,
        "expected_amount_minor": 0,
        "variance_amount_minor": 0,
        "countedPhysicalCashMinor": 0,
        "expectedCloudCashMinor": 0,
        "varianceMinor": 0,
        "reviewState": "open",
        "requires_review": False,
        "close_request_hash_hex": None,
        "currency": "USD",
        "reconciliation_status": "pending",
        "disclosure": "Accounting-only: cloud ledger vs operator physical count; does not sense or command hardware.",
    }
    cash_coll_closed_ex = {
        **cash_coll_open_ex,
        "closed_at": "2026-04-19T14:30:00.000000000Z",
        "lifecycle_status": "closed",
        "counted_amount_minor": 1250,
        "expected_amount_minor": 1200,
        "variance_amount_minor": 50,
        "countedPhysicalCashMinor": 1250,
        "expectedCloudCashMinor": 1200,
        "varianceMinor": 50,
        "reviewState": "variance_recorded",
        "requires_review": False,
        "close_request_hash_hex": "a" * 64,
        "reconciliation_status": "mismatch",
    }
    cash_coll_closed_exact_ex = {
        **cash_coll_open_ex,
        "closed_at": "2026-04-19T15:00:00.000000000Z",
        "lifecycle_status": "closed",
        "counted_amount_minor": 1200,
        "expected_amount_minor": 1200,
        "variance_amount_minor": 0,
        "countedPhysicalCashMinor": 1200,
        "expectedCloudCashMinor": 1200,
        "varianceMinor": 0,
        "reviewState": "matched",
        "requires_review": False,
        "close_request_hash_hex": "b" * 64,
        "reconciliation_status": "matched",
    }
    cash_coll_review_ex = {
        **cash_coll_open_ex,
        "closed_at": "2026-04-19T15:30:00.000000000Z",
        "lifecycle_status": "closed",
        "counted_amount_minor": 2000,
        "expected_amount_minor": 1200,
        "variance_amount_minor": 800,
        "countedPhysicalCashMinor": 2000,
        "expectedCloudCashMinor": 1200,
        "varianceMinor": 800,
        "reviewState": "pending_review",
        "requires_review": True,
        "close_request_hash_hex": "c" * 64,
        "reconciliation_status": "pending",
    }
    cashbox_ex = {
        "machineId": _U3,
        "currency": "VND",
        "expectedCashboxMinor": 1000000,
        "expectedCloudCashMinor": 1000000,
        "expectedRecyclerMinor": 0,
        "lastCollectionAt": "2026-04-24T00:00:00Z",
        "denominations": [],
        "openCollectionId": None,
        "varianceReviewThresholdMinor": 500,
        "disclosure": "Accounting-only: cloud ledger expectation only; does not sense or command physical cash hardware.",
    }

    tok = {
        "accessToken": "stub-access-token",
        "accessExpiresAt": "2026-04-19T13:00:00Z",
        "refreshToken": "stub-refresh-token",
        "refreshExpiresAt": "2026-05-19T12:00:00Z",
        "tokenType": "Bearer",
    }
    login_ok = {
        "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
        "organizationId": _U2,
        "email": "operator@example.com",
        "roles": ["org_admin"],
        "tokens": tok,
    }
    auth_acct_row = {
        "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
        "organizationId": _U2,
        "email": "operator@example.com",
        "roles": ["org_admin"],
        "status": "active",
        "createdAt": "2026-01-01T00:00:00Z",
        "updatedAt": "2026-04-19T10:00:00Z",
    }
    version_ex = {
        "name": "avf-vending-api",
        "version": "0.0.0-dev",
        "git_sha": "abc123",
        "build_time": "2026-04-19T12:00:00Z",
        "app_env": "development",
        "process": "api",
    }
    admin_page_meta = {"limit": 50, "offset": 0, "returned": 1, "totalCount": 1}
    product_row = {
        "id": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
        "organizationId": _U2,
        "sku": "COLA-12",
        "barcode": "8850123456789",
        "name": "Cola 12oz",
        "description": "Example product",
        "active": True,
        "createdAt": "2026-01-01T00:00:00Z",
        "updatedAt": "2026-04-19T10:00:00Z",
    }
    product_detail = {
        **product_row,
        "attrs": {},
        "ageRestricted": False,
        "allergenCodes": [],
    }
    product_mut_req = {
        "sku": "COLA-12",
        "name": "Cola 12oz",
        "description": "Example product",
        "active": True,
        "barcode": "8850123456789",
        "ageRestricted": False,
        "allergenCodes": [],
    }
    brand_row = {
        "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
        "organizationId": _U2,
        "slug": "coca-cola",
        "name": "Coca-Cola",
        "active": True,
        "createdAt": "2026-01-01T00:00:00Z",
        "updatedAt": "2026-04-19T10:00:00Z",
    }
    brand_mut_req = {"slug": "coca-cola", "name": "Coca-Cola", "active": True}
    cat_row = {
        "id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
        "organizationId": _U2,
        "slug": "beverages",
        "name": "Beverages",
        "active": True,
        "createdAt": "2026-01-01T00:00:00Z",
        "updatedAt": "2026-04-19T10:00:00Z",
    }
    cat_mut_req = {"slug": "beverages", "name": "Beverages", "active": True}
    tag_row = {
        "id": "cccccccc-dddd-eeee-ffff-000000000000",
        "organizationId": _U2,
        "slug": "chilled",
        "name": "Chilled",
        "active": True,
        "createdAt": "2026-01-01T00:00:00Z",
        "updatedAt": "2026-04-19T10:00:00Z",
    }
    tag_mut_req = {"slug": "chilled", "name": "Chilled", "active": True}
    img_bind_req = {
        "artifactId": "11111111-2222-3333-4444-555555555555",
        "thumbUrl": "https://cdn.example.com/products/coca330-thumb.webp",
        "displayUrl": "https://cdn.example.com/products/coca330-display.webp",
        "contentHash": "sha256:"
        + "a" * 64,
        "width": 800,
        "height": 800,
        "mimeType": "image/webp",
    }
    media_asset_row = {
        "id": "11111111-2222-3333-4444-555555555555",
        "organization_id": _U2,
        "kind": "product_image",
        "status": "ready",
        "mime_type": "image/webp",
        "size_bytes": 12000,
        "sha256": "a" * 64,
        "object_version": 1,
        "etag": 'W/"etag1"',
        "created_at": "2026-01-01T00:00:00Z",
        "updated_at": "2026-04-19T10:00:00Z",
    }
    media_init_req = {"content_type": "image/jpeg"}
    media_init_resp = {
        "media_id": "11111111-2222-3333-4444-555555555555",
        "upload_url": "https://s3.example.com/presigned-put",
        "upload_method": "PUT",
        "upload_headers": {"Content-Type": ["image/jpeg"]},
        "expires_at": "2026-04-19T13:00:00Z",
        "complete_path": "/v1/admin/media/11111111-2222-3333-4444-555555555555/complete",
    }
    product_media_bind_req = {"media_id": "11111111-2222-3333-4444-555555555555"}
    product_image_row = {
        "id": "22222222-2222-3333-4444-555555555555",
        "product_id": product_row["id"],
        "display_url": "https://cdn.example.com/products/display.webp",
        "thumb_url": "https://cdn.example.com/products/thumb.webp",
        "content_hash": "sha256:" + "a" * 64,
        "media_version": 2,
        "sort_order": 0,
        "is_primary": True,
        "status": "active",
        "created_at": "2026-01-01T00:00:00Z",
        "updated_at": "2026-04-19T10:00:00Z",
    }
    product_image_patch_req = {"sort_order": 10, "is_primary": True, "alt_text": "Cold drink"}
    art_upload_ok = {"status": "uploaded", "artifact_id": "11111111-2222-3333-4444-555555555555"}
    inv_by_product_ex = {
        "items": [
            {
                "machineId": _U3,
                "machineName": "Lobby-01",
                "machineStatus": "active",
                "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
                "productName": "Cola 12oz",
                "productSku": "COLA-12",
                "totalQuantity": 24,
                "slotCount": 2,
                "maxCapacityAnySlot": 12,
                "lowStock": False,
                "cabinetCode": "CAB-A",
                "cabinetIndex": 0,
            }
        ],
    }
    refill_forecast_ex = {
        "organizationId": _U2,
        "velocityWindowDays": 14,
        "windowStart": "2026-04-14T00:00:00.000000000Z",
        "windowEnd": "2026-04-28T00:00:00.000000000Z",
        "items": [
            {
                "machineId": _U3,
                "machineName": "Lobby-01",
                "siteId": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
                "siteName": "HQ",
                "planogramId": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
                "planogramName": "Lobby default",
                "slotIndex": 0,
                "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
                "productSku": "COLA-12",
                "productName": "Cola 12oz",
                "currentQuantity": 3,
                "maxQuantity": 10,
                "unitsSoldInWindow": 14,
                "dailyVelocity": 1.0,
                "daysToEmpty": 3.0,
                "fillRatio": 0.3,
                "suggestedRefillQuantity": 7,
                "urgency": "medium",
            }
        ],
        "meta": {"limit": 50, "offset": 0, "returned": 1, "total": 1},
    }
    price_book_row = {
        "id": "11111111-2222-3333-4444-555555555555",
        "organizationId": _U2,
        "name": "Default USD",
        "currency": "USD",
        "effectiveFrom": "2026-01-01T00:00:00Z",
        "isDefault": True,
        "active": True,
        "scopeType": "organization",
        "priority": 0,
        "createdAt": "2026-01-01T00:00:00Z",
        "updatedAt": "2026-01-01T00:00:00Z",
    }
    price_book_create_req = {
        "name": "Lobby promo",
        "currency": "USD",
        "effectiveFrom": "2026-04-01T00:00:00Z",
        "isDefault": False,
        "scopeType": "organization",
        "priority": 10,
    }
    price_book_items_env = {"items": [{"productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff", "unitPriceMinor": 150, "priceBookId": price_book_row["id"]}]}
    pricing_preview_req = {"productIds": ["9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff"], "machineId": _U3}
    pricing_preview_resp = {
        "at": "2026-04-24T12:00:00.000000000Z",
        "currency": "USD",
        "lines": [
            {
                "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
                "basePrice": 150,
                "effectivePrice": 175,
                "currency": "USD",
                "priceBookId": price_book_row["id"],
                "appliedRuleIds": ["price_book:" + price_book_row["id"]],
                "reasons": ["tier_3", "priority_10"],
            }
        ],
    }
    promo_id = "10101010-1010-1010-1010-101010101010"
    promo_rule_id = "20202020-2020-2020-2020-202020202020"
    promo_target_id = "30303030-3030-3030-3030-303030303030"
    promotion_row = {
        "id": promo_id,
        "organizationId": _U2,
        "name": "Summer 10%",
        "approvalStatus": "approved",
        "lifecycleStatus": "draft",
        "priority": 10,
        "stackable": False,
        "startsAt": "2026-06-01T00:00:00Z",
        "endsAt": "2026-09-01T00:00:00Z",
        "createdAt": "2026-04-01T00:00:00Z",
        "updatedAt": "2026-04-01T00:00:00Z",
    }
    promotion_create_req = {
        "name": "Summer 10%",
        "startsAt": "2026-06-01T00:00:00Z",
        "endsAt": "2026-09-01T00:00:00Z",
        "priority": 10,
        "stackable": False,
        "rules": [{"ruleType": "percentage_discount", "priority": 0, "payload": {"percent": 10}}],
    }
    promotion_detail = {
        "promotion": promotion_row,
        "rules": [
            {
                "id": promo_rule_id,
                "promotionId": promo_id,
                "ruleType": "percentage_discount",
                "priority": 0,
                "payload": {"percent": 10},
            }
        ],
        "targets": [],
    }
    promotion_assign_req = {"targetType": "product", "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff"}
    promotion_target_row = {
        "id": promo_target_id,
        "promotionId": promo_id,
        "organizationId": _U2,
        "targetType": "product",
        "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
        "createdAt": "2026-04-01T00:00:00Z",
    }
    promotion_preview_req = {"productIds": ["9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff"], "machineId": _U3}
    promotion_preview_resp = {
        "at": "2026-04-24T12:00:00.000000000Z",
        "lines": [
            {
                "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
                "basePriceMinor": 150,
                "discountMinor": 15,
                "finalPriceMinor": 135,
                "currency": "USD",
                "appliedPromotionIds": [promo_id],
                "appliedRuleIds": ["promotion_rule:" + promo_rule_id],
                "skippedRules": [],
            }
        ],
    }
    planogram_row = {
        "id": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
        "organizationId": _U2,
        "name": "Lobby spring",
        "revision": 3,
        "status": "published",
        "createdAt": "2026-04-01T00:00:00Z",
    }
    planogram_detail = {
        "planogram": planogram_row,
        "slots": [
            {
                "id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
                "planogramId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
                "slotIndex": 1,
                "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
                "maxQuantity": 10,
                "productSku": "COLA-12",
                "productName": "Cola 12oz",
                "createdAt": "2026-04-01T00:00:00Z",
            }
        ],
    }
    sales_summary_ex = {
        "organizationId": _U2,
        "from": "2026-04-01T00:00:00Z",
        "to": "2026-04-20T00:00:00Z",
        "groupBy": "day",
        "summary": {
            "grossTotalMinor": 10000,
            "subtotalMinor": 9000,
            "taxMinor": 1000,
            "orderCount": 50,
            "avgOrderValueMinor": 200,
        },
        "breakdown": [],
    }
    pay_summary_ex = {
        "organizationId": _U2,
        "from": "2026-04-01T00:00:00Z",
        "to": "2026-04-20T00:00:00Z",
        "groupBy": "day",
        "summary": {
            "authorizedCount": 10,
            "capturedCount": 48,
            "failedCount": 2,
            "refundedCount": 0,
            "capturedAmountMinor": 10000,
            "authorizedAmountMinor": 10200,
            "failedAmountMinor": 400,
            "refundedAmountMinor": 0,
        },
        "breakdown": [],
    }
    fleet_health_ex = {
        "organizationId": _U2,
        "from": "2026-04-01T00:00:00Z",
        "to": "2026-04-20T00:00:00Z",
        "machineSummary": {
            "total": 25,
            "online": 22,
            "offline": 2,
            "fault": 1,
            "warn": 0,
            "retired": 0,
        },
        "machinesByStatus": [{"status": "online", "count": 22}],
        "incidentsByStatus": [],
        "machineIncidentsBySeverity": [],
    }
    inv_exceptions_ex = {
        "organizationId": _U2,
        "from": "2026-04-01T00:00:00.000000000Z",
        "to": "2026-04-20T00:00:00.000000000Z",
        "exceptionKind": "low_stock",
        "meta": {"limit": 50, "offset": 0, "returned": 0, "total": 0},
        "items": [],
    }
    report_meta = {"limit": 50, "offset": 0, "returned": 1, "total": 1}
    admin_payments_report_ex = {
        "organizationId": _U2,
        "from": "2026-04-01T00:00:00Z",
        "to": "2026-04-20T00:00:00Z",
        "timezone": "UTC",
        "items": [{
            "bucketStart": "2026-04-01T00:00:00Z",
            "provider": "cash",
            "state": "captured",
            "settlementStatus": "settled",
            "reconciliationStatus": "matched",
            "paymentCount": 5,
            "amountMinor": 12500,
        }],
        "meta": report_meta,
    }
    admin_report_list_ex = {
        "organizationId": _U2,
        "from": "2026-04-01T00:00:00Z",
        "to": "2026-04-20T00:00:00Z",
        "meta": report_meta,
        "items": [{"id": _U, "status": "open"}],
    }
    csv_ex = (
        "organization_id,from,to,group_by,row_type,bucket_start,site_id,machine_id,payment_provider,"
        "order_count,total_minor,subtotal_minor,tax_minor,gross_total_minor,summary_order_count,avg_order_value_minor\n"
    )
    daily_close_ex = {
        "id": _U,
        "organizationId": _U2,
        "closeDate": "2026-04-27",
        "timezone": "Asia/Bangkok",
        "idempotencyKey": "REPLACE_ME",
        "grossSalesMinor": 100000,
        "discountMinor": 0,
        "refundMinor": 500,
        "netMinor": 99500,
        "cashMinor": 60000,
        "qrWalletMinor": 40000,
        "failedMinor": 200,
        "pendingMinor": 300,
        "createdAt": "2026-04-27T18:00:00.000000000Z",
    }
    daily_close_create_req = {"closeDate": "2026-04-27", "timezone": "Asia/Bangkok"}
    daily_close_list_ex = {"items": [daily_close_ex], "meta": {"limit": 50, "offset": 0, "returned": 1, "total": 1}}
    recon_ex = {"kind": "commerce.reconciliation_snapshot", "status": checkout}
    op_list_ex = {"items": [], "meta": {"limit": 50, "returned": 0}}
    check_in_resp = {"id": "12001", "machine_id": _U3, "occurred_at": "2026-04-19T12:00:00.000000000Z"}
    config_apply_resp = {
        "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
        "machine_id": _U3,
        "config_revision": 7,
        "applied_at": "2026-04-19T12:05:00.000000000Z",
    }

    def ex(
        req_body: Any | None = None,
        resp: dict[str, tuple[Any | None, Any | None]] | None = None,
    ) -> dict[str, Any]:
        """resp: status -> (response_example_object, None) attaches to first JSON content."""
        out: dict[str, Any] = {}
        if req_body is not None:
            out["requestBodyExample"] = req_body
        if resp:
            out["responseExamples"] = resp
        return out

    sale_item_ex = {
        "slotIndex": 3,
        "slotCode": "A3",
        "cabinetCode": "A",
        "productId": "22222222-3333-4444-5555-666666666666",
        "sku": "COCA330",
        "name": "Coca Cola 330ml",
        "shortName": "Coca 330",
        "priceMinor": 15000,
        "availableQuantity": 8,
        "maxQuantity": 12,
        "isAvailable": True,
        "unavailableReason": None,
        "image": {
            "thumbUrl": "https://cdn.example.com/products/coca330-thumb.webp",
            "displayUrl": "https://cdn.example.com/products/coca330-display.webp",
            "contentHash": "sha256:" + "b" * 64,
            "updatedAt": "2026-04-24T00:00:00Z",
        },
        "sortOrder": 10,
    }
    sale_catalog_ex = {
        "machineId": _U3,
        "organizationId": _U2,
        "siteId": "11111111-2222-3333-4444-555555555555",
        "configVersion": 7,
        "currency": "VND",
        "generatedAt": "2026-04-24T00:00:00Z",
        "items": [sale_item_ex],
    }
    fingerprint_ex = {
        "androidId": "android-123",
        "serialNumber": "SN-001",
        "manufacturer": "SUNMI",
        "model": "K2",
        "packageName": "com.avf.vending",
        "versionName": "1.0.0",
        "versionCode": 100,
    }

    return {
        ("get", "/health/live"): ex(resp={"200": ("ok", None)}),
        ("get", "/health/ready"): ex(resp={"200": ("ok", None), "503": ("not ready", None)}),
        ("get", "/metrics"): ex(
            resp={
                "200": ("# HELP go_goroutines Number of goroutines.\ngo_goroutines 42\n", None),
            },
        ),
        ("get", "/version"): ex(resp={"200": (version_ex, None)}),
        ("get", "/swagger/doc.json"): ex(
            resp={"200": ({"openapi": "3.0.3", "info": {"title": "AVF Vending HTTP API", "version": "1.0"}}, None)},
        ),
        ("post", "/v1/auth/login"): ex(
            req_body={"organizationId": _U2, "email": "operator@example.com", "password": "example-password"},
            resp={
                "200": (login_ok, None),
                "401": (v1_error_example("unauthenticated", "invalid credentials", {}), None),
            },
        ),
        ("post", "/v1/auth/refresh"): ex(
            req_body={"refreshToken": "stub-refresh-token"},
            resp={"200": ({"tokens": tok}, None)},
        ),
        ("get", "/v1/auth/me"): ex(
            resp={
                "200": (
                    {
                        "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
                        "organizationId": _U2,
                        "email": "operator@example.com",
                        "roles": ["org_admin"],
                    },
                    None,
                ),
            },
        ),
        ("post", "/v1/auth/logout"): ex(req_body={"revokeAll": False}),
        ("post", "/v1/auth/change-password"): ex(
            req_body={"currentPassword": "example-password-old", "newPassword": "example-password-new"},
        ),
        ("post", "/v1/auth/password/change"): ex(
            req_body={"currentPassword": "example-password-old", "newPassword": "example-password-new"},
        ),
        ("post", "/v1/auth/password/reset/request"): ex(
            req_body={"organizationId": _U2, "email": "operator@example.com"},
            resp={"202": ({"accepted": True}, None)},
        ),
        ("post", "/v1/auth/password/reset/confirm"): ex(
            req_body={"token": "opaque-reset-token", "newPassword": "example-password-new"},
            resp={"204": ("", None)},
        ),
        ("post", "/v1/auth/mfa/totp/enroll"): ex(
            resp={
                "200": (
                    {
                        "otpauthUri": "otpauth://totp/AVF%20Admin:operator%40example.com?secret=ABCDABCDABCDABCD&issuer=AVF%20Admin",
                        "secret": "ABCDABCDABCDABCDABCDABCDABCDABCD",
                    },
                    None,
                ),
            },
        ),
        ("post", "/v1/auth/mfa/totp/verify"): ex(
            req_body={"code": "123456"},
            resp={"200": (login_ok, None)},
        ),
        ("post", "/v1/auth/mfa/totp/disable"): ex(
            req_body={"currentPassword": "example-password-old", "totpCode": "123456"},
            resp={"204": ("", None)},
        ),
        ("get", "/v1/auth/sessions"): ex(
            resp={
                "200": (
                    {
                        "sessions": [
                            {
                                "sessionId": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
                                "organizationId": _U2,
                                "createdAt": "2026-04-19T10:00:00Z",
                                "expiresAt": "2026-05-19T12:00:00Z",
                                "status": "active",
                            },
                        ],
                    },
                    None,
                ),
            },
        ),
        ("delete", "/v1/auth/sessions"): ex(
            req_body={"exceptRefreshToken": "stub-refresh-token"},
            resp={"204": ("", None)},
        ),
        ("get", "/v1/admin/auth/users"): ex(resp={"200": ({"items": [auth_acct_row], "meta": cmeta}, None)}),
        ("post", "/v1/admin/auth/users"): ex(
            req_body={"email": "new.user@example.com", "password": "longpassword10", "roles": ["viewer"], "status": "active"},
            resp={"201": (auth_acct_row, None)},
        ),
        ("get", "/v1/admin/auth/users/{accountId}"): ex(resp={"200": (auth_acct_row, None)}),
        ("patch", "/v1/admin/auth/users/{accountId}"): ex(
            req_body={"status": "disabled"},
            resp={"200": (auth_acct_row, None)},
        ),
        ("post", "/v1/admin/auth/users/{accountId}/activate"): ex(resp={"200": (auth_acct_row, None)}),
        ("post", "/v1/admin/auth/users/{accountId}/deactivate"): ex(resp={"200": (auth_acct_row, None)}),
        ("post", "/v1/admin/auth/users/{accountId}/reset-password"): ex(
            req_body={"password": "reset-password12"},
            resp={"200": (auth_acct_row, None)},
        ),
        ("post", "/v1/admin/auth/users/{accountId}/revoke-sessions"): ex(resp={"204": ("", None)}),
        ("get", "/v1/admin/auth/users/{accountId}/sessions"): ex(
            resp={
                "200": (
                    {
                        "sessions": [
                            {
                                "sessionId": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
                                "organizationId": _U2,
                                "createdAt": "2026-04-19T10:00:00Z",
                                "expiresAt": "2026-05-19T12:00:00Z",
                                "status": "active",
                            },
                        ],
                    },
                    None,
                ),
            },
        ),
        ("post", "/v1/admin/auth/users/{accountId}/roles"): ex(
            req_body={"roles": ["viewer"]},
            resp={"200": (auth_acct_row, None)},
        ),
        ("put", "/v1/admin/auth/users/{accountId}/roles"): ex(
            req_body={"roles": ["viewer"]},
            resp={"200": (auth_acct_row, None)},
        ),
        ("patch", "/v1/admin/auth/users/{accountId}/roles"): ex(
            req_body={"roles": ["viewer"]},
            resp={"200": (auth_acct_row, None)},
        ),
        ("patch", "/v1/admin/auth/users/{accountId}/status"): ex(
            req_body={"status": "disabled"},
            resp={"200": (auth_acct_row, None)},
        ),
        ("get", "/v1/admin/users"): ex(resp={"200": ({"items": [auth_acct_row], "meta": cmeta}, None)}),
        ("post", "/v1/admin/users"): ex(
            req_body={"email": "new.user@example.com", "password": "longpassword10", "roles": ["viewer"], "status": "active"},
            resp={"201": (auth_acct_row, None)},
        ),
        ("get", "/v1/admin/users/{userId}"): ex(resp={"200": (auth_acct_row, None)}),
        ("patch", "/v1/admin/users/{userId}"): ex(
            req_body={"status": "disabled"},
            resp={"200": (auth_acct_row, None)},
        ),
        ("put", "/v1/admin/users/{userId}/roles"): ex(
            req_body={"roles": ["catalog_manager"]},
            resp={"200": (auth_acct_row, None)},
        ),
        ("post", "/v1/admin/users/{userId}/roles"): ex(
            req_body={"roles": ["catalog_manager"]},
            resp={"200": (auth_acct_row, None)},
        ),
        ("patch", "/v1/admin/users/{userId}/roles"): ex(
            req_body={"roles": ["catalog_manager"]},
            resp={"200": (auth_acct_row, None)},
        ),
        ("patch", "/v1/admin/users/{userId}/status"): ex(
            req_body={"status": "disabled"},
            resp={"200": (auth_acct_row, None)},
        ),
        ("post", "/v1/admin/users/{userId}/enable"): ex(resp={"200": (auth_acct_row, None)}),
        ("post", "/v1/admin/users/{userId}/disable"): ex(resp={"200": (auth_acct_row, None)}),
        ("post", "/v1/admin/users/{userId}/revoke-sessions"): ex(resp={"204": ("", None)}),
        ("get", "/v1/admin/users/{userId}/sessions"): ex(
            resp={
                "200": (
                    {
                        "sessions": [
                            {
                                "sessionId": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
                                "organizationId": _U2,
                                "createdAt": "2026-04-19T10:00:00Z",
                                "expiresAt": "2026-05-19T12:00:00Z",
                                "status": "active",
                            },
                        ],
                    },
                    None,
                ),
            },
        ),
        ("post", "/v1/admin/users/{userId}/reset-password"): ex(
            req_body={"password": "reset-password12"},
            resp={"200": (auth_acct_row, None)},
        ),
        ("get", "/v1/admin/organizations/{organizationId}/users"): ex(resp={"200": ({"items": [auth_acct_row], "meta": cmeta}, None)}),
        ("post", "/v1/admin/organizations/{organizationId}/users"): ex(
            req_body={"email": "new.user@example.com", "password": "longpassword10", "roles": ["support"], "status": "active"},
            resp={"201": (auth_acct_row, None)},
        ),
        ("get", "/v1/admin/organizations/{organizationId}/users/{userId}"): ex(resp={"200": (auth_acct_row, None)}),
        ("patch", "/v1/admin/organizations/{organizationId}/users/{userId}"): ex(
            req_body={"roles": ["support"]},
            resp={"200": (auth_acct_row, None)},
        ),
        ("post", "/v1/admin/organizations/{organizationId}/users/{userId}/enable"): ex(resp={"200": (auth_acct_row, None)}),
        ("post", "/v1/admin/organizations/{organizationId}/users/{userId}/disable"): ex(resp={"200": (auth_acct_row, None)}),
        ("post", "/v1/admin/organizations/{organizationId}/users/{userId}/roles"): ex(
            req_body={"roles": ["support"]},
            resp={"200": (auth_acct_row, None)},
        ),
        ("patch", "/v1/admin/organizations/{organizationId}/users/{userId}/roles"): ex(
            req_body={"roles": ["support"]},
            resp={"200": (auth_acct_row, None)},
        ),
        (
            "delete",
            "/v1/admin/organizations/{organizationId}/users/{userId}/roles/{role}",
        ): ex(resp={"200": ({**auth_acct_row, "roles": ["viewer"]}, None)}),
        ("patch", "/v1/admin/organizations/{organizationId}/users/{userId}/status"): ex(
            req_body={"status": "disabled"},
            resp={"200": (auth_acct_row, None)},
        ),
        ("post", "/v1/admin/organizations/{organizationId}/users/{userId}/revoke-sessions"): ex(resp={"204": ("", None)}),
        ("get", "/v1/admin/organizations/{organizationId}/users/{userId}/sessions"): ex(
            resp={
                "200": (
                    {
                        "sessions": [
                            {
                                "sessionId": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
                                "organizationId": _U2,
                                "createdAt": "2026-04-19T10:00:00Z",
                                "expiresAt": "2026-05-19T12:00:00Z",
                                "status": "active",
                            },
                        ],
                    },
                    None,
                ),
            },
        ),
        ("post", "/v1/admin/organizations/{organizationId}/users/{userId}/reset-password"): ex(
            req_body={"password": "reset-password12"},
            resp={"200": (auth_acct_row, None)},
        ),
        ("get", "/v1/admin/machines/{machineId}/slots"): ex(
            resp={
                "200": (
                    {
                        "machineId": _U3,
                        "organizationId": _U2,
                        "cabinets": [],
                        "slots": [
                            {
                                "cabinetCode": "A",
                                "slotCode": "A3",
                                "slotIndex": 3,
                                "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
                                "currentQuantity": 8,
                                "maxQuantity": 12,
                            }
                        ],
                    },
                    None,
                ),
            },
        ),
        ("get", "/v1/admin/products"): ex(resp={"200": ({"items": [product_row], "meta": admin_page_meta}, None)}),
        ("get", "/v1/admin/products/{productId}"): ex(resp={"200": (product_detail, None)}),
        ("post", "/v1/admin/products"): ex(req_body=product_mut_req, resp={"200": (product_detail, None)}),
        ("put", "/v1/admin/products/{productId}"): ex(req_body=product_mut_req, resp={"200": (product_detail, None)}),
        ("patch", "/v1/admin/products/{productId}"): ex(req_body=product_mut_req, resp={"200": (product_detail, None)}),
        ("delete", "/v1/admin/products/{productId}"): ex(resp={"200": ({**product_detail, "active": False}, None)}),
        ("post", "/v1/admin/products/{productId}/image"): ex(req_body=img_bind_req, resp={"200": (product_detail, None)}),
        ("put", "/v1/admin/products/{productId}/image"): ex(req_body=img_bind_req, resp={"200": (product_detail, None)}),
        ("delete", "/v1/admin/products/{productId}/image"): ex(resp={"200": (product_detail, None)}),
        ("post", "/v1/admin/media/assets"): ex(req_body=media_init_req, resp={"200": (media_init_resp, None)}),
        ("post", "/v1/admin/media/uploads"): ex(req_body=media_init_req, resp={"200": (media_init_resp, None)}),
        ("post", "/v1/admin/media/{mediaId}/complete"): ex(resp={"200": (media_asset_row, None)}),
        ("post", "/v1/admin/organizations/{organizationId}/media/uploads/init"): ex(req_body=media_init_req, resp={"200": (media_init_resp, None)}),
        ("post", "/v1/admin/organizations/{organizationId}/media/uploads/complete"): ex(req_body={"media_id": media_asset_row["id"]}, resp={"200": (media_asset_row, None)}),
        ("post", "/v1/admin/organizations/{organizationId}/media/product-images"): ex(req_body=media_init_req, resp={"200": (media_init_resp, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/media/assets"): ex(resp={"200": ({"items": [media_asset_row], "meta": admin_page_meta}, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/media/assets/{assetId}"): ex(resp={"200": (media_asset_row, None)}),
        ("delete", "/v1/admin/organizations/{organizationId}/media/assets/{assetId}"): ex(resp={"204": ("", None)}),
        ("post", "/v1/admin/organizations/{organizationId}/products/{productId}/media"): ex(req_body=product_media_bind_req, resp={"200": (product_detail, None)}),
        ("delete", "/v1/admin/organizations/{organizationId}/products/{productId}/media/{mediaId}"): ex(resp={"200": (product_detail, None)}),
        ("get", "/v1/admin/media/assets"): ex(resp={"200": ({"items": [media_asset_row], "meta": admin_page_meta}, None)}),
        ("get", "/v1/admin/media/assets/{mediaId}"): ex(resp={"200": (media_asset_row, None)}),
        ("get", "/v1/admin/media"): ex(resp={"200": ({"items": [media_asset_row], "meta": admin_page_meta}, None)}),
        ("get", "/v1/admin/media/{mediaId}"): ex(resp={"200": (media_asset_row, None)}),
        ("delete", "/v1/admin/media/assets/{mediaId}"): ex(resp={"204": ("", None)}),
        ("delete", "/v1/admin/media/{mediaId}"): ex(resp={"204": ("", None)}),
        ("post", "/v1/admin/products/{productId}/media"): ex(req_body=product_media_bind_req, resp={"200": (product_detail, None)}),
        ("put", "/v1/admin/products/{productId}/media"): ex(req_body=product_media_bind_req, resp={"200": (product_detail, None)}),
        ("delete", "/v1/admin/products/{productId}/media/{mediaId}"): ex(resp={"200": (product_detail, None)}),
        ("post", "/v1/admin/organizations/{organizationId}/products/{productId}/images"): ex(req_body=product_media_bind_req, resp={"200": (product_detail, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/products/{productId}/images"): ex(resp={"200": ({"items": [product_image_row]}, None)}),
        ("patch", "/v1/admin/organizations/{organizationId}/products/{productId}/images/{imageId}"): ex(req_body=product_image_patch_req, resp={"200": (product_image_row, None)}),
        ("delete", "/v1/admin/organizations/{organizationId}/products/{productId}/images/{imageId}"): ex(resp={"204": ("", None)}),
        ("get", "/v1/admin/brands"): ex(resp={"200": ({"items": [brand_row], "meta": admin_page_meta}, None)}),
        ("post", "/v1/admin/brands"): ex(req_body=brand_mut_req, resp={"200": (brand_row, None)}),
        ("put", "/v1/admin/brands/{brandId}"): ex(req_body=brand_mut_req, resp={"200": (brand_row, None)}),
        ("patch", "/v1/admin/brands/{brandId}"): ex(req_body=brand_mut_req, resp={"200": (brand_row, None)}),
        ("delete", "/v1/admin/brands/{brandId}"): ex(resp={"200": ({**brand_row, "active": False}, None)}),
        ("get", "/v1/admin/categories"): ex(resp={"200": ({"items": [cat_row], "meta": admin_page_meta}, None)}),
        ("post", "/v1/admin/categories"): ex(req_body=cat_mut_req, resp={"200": (cat_row, None)}),
        ("put", "/v1/admin/categories/{categoryId}"): ex(req_body=cat_mut_req, resp={"200": (cat_row, None)}),
        ("patch", "/v1/admin/categories/{categoryId}"): ex(req_body=cat_mut_req, resp={"200": (cat_row, None)}),
        ("delete", "/v1/admin/categories/{categoryId}"): ex(resp={"200": ({**cat_row, "active": False}, None)}),
        ("get", "/v1/admin/tags"): ex(resp={"200": ({"items": [tag_row], "meta": admin_page_meta}, None)}),
        ("post", "/v1/admin/tags"): ex(req_body=tag_mut_req, resp={"200": (tag_row, None)}),
        ("put", "/v1/admin/tags/{tagId}"): ex(req_body=tag_mut_req, resp={"200": (tag_row, None)}),
        ("patch", "/v1/admin/tags/{tagId}"): ex(req_body=tag_mut_req, resp={"200": (tag_row, None)}),
        ("delete", "/v1/admin/tags/{tagId}"): ex(resp={"200": ({**tag_row, "active": False}, None)}),
        ("get", "/v1/admin/price-books"): ex(resp={"200": ({"items": [price_book_row], "meta": admin_page_meta}, None)}),
        ("get", "/v1/admin/price-books/{priceBookId}"): ex(resp={"200": (price_book_row, None)}),
        ("post", "/v1/admin/price-books"): ex(req_body=price_book_create_req, resp={"200": (price_book_row, None)}),
        ("patch", "/v1/admin/price-books/{priceBookId}"): ex(
            req_body={"priority": 20},
            resp={"200": (price_book_row, None)},
        ),
        ("post", "/v1/admin/price-books/{priceBookId}/deactivate"): ex(resp={"200": ({**price_book_row, "active": False}, None)}),
        ("post", "/v1/admin/price-books/{priceBookId}/activate"): ex(resp={"200": ({**price_book_row, "active": True}, None)}),
        ("post", "/v1/admin/price-books/{priceBookId}/archive"): ex(resp={"200": ({**price_book_row, "active": False}, None)}),
        ("get", "/v1/admin/price-books/{priceBookId}/items"): ex(resp={"200": (price_book_items_env, None)}),
        ("put", "/v1/admin/price-books/{priceBookId}/items"): ex(
            req_body={"items": [{"productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff", "unitPriceMinor": 150}]},
            resp={"204": ("", None)},
        ),
        ("patch", "/v1/admin/price-books/{priceBookId}/items/{productId}"): ex(
            req_body={"unitPriceMinor": 175},
            resp={
                "200": (
                    {
                        "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
                        "priceBookId": price_book_row["id"],
                        "unitPriceMinor": 175,
                    },
                    None,
                )
            },
        ),
        ("delete", "/v1/admin/price-books/{priceBookId}/items/{productId}"): ex(resp={"204": ("", None)}),
        ("post", "/v1/admin/price-books/{priceBookId}/assign-target"): ex(
            req_body={"machineId": _U3},
            resp={
                "200": (
                    {
                        "id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
                        "priceBookId": price_book_row["id"],
                        "siteId": None,
                        "machineId": _U3,
                        "createdAt": "2026-04-24T12:00:00Z",
                    },
                    None,
                )
            },
        ),
        ("delete", "/v1/admin/price-books/{priceBookId}/targets/{targetId}"): ex(resp={"204": ("", None)}),
        ("get", "/v1/admin/promotions"): ex(resp={"200": ({"items": [promotion_row], "meta": admin_page_meta}, None)}),
        ("post", "/v1/admin/promotions/preview"): ex(
            req_body=promotion_preview_req,
            resp={"200": (promotion_preview_resp, None)},
        ),
        ("get", "/v1/admin/promotions/{promotionId}"): ex(resp={"200": (promotion_detail, None)}),
        ("post", "/v1/admin/promotions"): ex(req_body=promotion_create_req, resp={"200": (promotion_row, None)}),
        ("patch", "/v1/admin/promotions/{promotionId}"): ex(
            req_body={"name": "Summer 12%", "priority": 11},
            resp={"200": ({**promotion_row, "name": "Summer 12%", "priority": 11}, None)},
        ),
        ("post", "/v1/admin/promotions/{promotionId}/activate"): ex(
            resp={"200": ({**promotion_row, "lifecycleStatus": "active"}, None)}
        ),
        ("post", "/v1/admin/promotions/{promotionId}/pause"): ex(
            resp={"200": ({**promotion_row, "lifecycleStatus": "paused"}, None)}
        ),
        ("post", "/v1/admin/promotions/{promotionId}/deactivate"): ex(
            resp={"200": ({**promotion_row, "lifecycleStatus": "deactivated"}, None)}
        ),
        ("post", "/v1/admin/promotions/{promotionId}/archive"): ex(
            resp={"200": ({**promotion_row, "lifecycleStatus": "deactivated"}, None)}
        ),
        ("post", "/v1/admin/promotions/{promotionId}/assign-target"): ex(
            req_body=promotion_assign_req,
            resp={"200": (promotion_target_row, None)},
        ),
        ("delete", "/v1/admin/promotions/{promotionId}/targets/{targetId}"): ex(resp={"204": ("", None)}),
        ("post", "/v1/admin/pricing/preview"): ex(
            req_body=pricing_preview_req,
            resp={"200": (pricing_preview_resp, None)},
        ),
        ("get", "/v1/admin/planograms"): ex(resp={"200": ({"items": [planogram_row], "meta": admin_page_meta}, None)}),
        ("get", "/v1/admin/planograms/{planogramId}"): ex(resp={"200": (planogram_detail, None)}),
        ("get", "/v1/reports/sales-summary"): ex(resp={"200": (sales_summary_ex, None)}),
        ("get", "/v1/reports/payments-summary"): ex(resp={"200": (pay_summary_ex, None)}),
        ("get", "/v1/reports/fleet-health"): ex(resp={"200": (fleet_health_ex, None)}),
        ("get", "/v1/reports/inventory-exceptions"): ex(resp={"200": (inv_exceptions_ex, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/reports/sales"): ex(resp={"200": (sales_summary_ex, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/reports/payments"): ex(resp={"200": (admin_payments_report_ex, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/reports/refunds"): ex(resp={"200": (admin_report_list_ex, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/reports/cash"): ex(resp={"200": (admin_report_list_ex, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/reports/inventory-low-stock"): ex(resp={"200": (inv_exceptions_ex, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/reports/machine-health"): ex(resp={"200": (admin_report_list_ex, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/reports/failed-vends"): ex(resp={"200": (admin_report_list_ex, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/reports/reconciliation-queue"): ex(resp={"200": (admin_report_list_ex, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/reports/vends"): ex(resp={"200": (admin_report_list_ex, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/reports/inventory"): ex(resp={"200": (admin_report_list_ex, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/reports/machines"): ex(resp={"200": (admin_report_list_ex, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/reports/products"): ex(resp={"200": (admin_report_list_ex, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/reports/reconciliation"): ex(resp={"200": (admin_report_list_ex, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/reports/commands"): ex(resp={"200": (admin_report_list_ex, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/reports/fills"): ex(resp={"200": (admin_report_list_ex, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/reports/export"): ex(resp={"200": (csv_ex, None)}),
        ("get", "/v1/admin/reports/sales-summary/export.csv"): ex(resp={"200": (csv_ex, None)}),
        ("get", "/v1/admin/reports/payments-summary/export.csv"): ex(resp={"200": (csv_ex, None)}),
        ("get", "/v1/admin/reports/cash-collections/export.csv"): ex(resp={"200": (csv_ex, None)}),
        ("post", "/v1/admin/finance/daily-close"): ex(
            req_body=daily_close_create_req,
            resp={"201": (daily_close_ex, None)},
        ),
        ("get", "/v1/admin/finance/daily-close"): ex(resp={"200": (daily_close_list_ex, None)}),
        ("get", "/v1/admin/finance/daily-close/{closeId}"): ex(resp={"200": (daily_close_ex, None)}),
        ("get", "/v1/admin/machines/{machineId}"): ex(resp={"200": (mach_item, None)}),
        ("get", "/v1/commerce/orders/{orderId}/reconciliation"): ex(resp={"200": (recon_ex, None)}),
        ("post", "/v1/commerce/orders/{orderId}/payments/{paymentId}/webhooks"): ex(
            req_body={
                "provider": "stripe",
                "provider_reference": "pi_example_123",
                "event_type": "payment_intent.succeeded",
                "normalized_payment_state": "captured",
                "payload_json": {"id": "pi_example_123", "status": "succeeded"},
            },
            resp={
                "200": ({"replay": False, "order_id": _U, "payment_state": "captured"}, None),
                "400": (
                    v1_error_example(
                        "webhook_timestamp_skew",
                        "webhook timestamp outside allowed skew",
                        {},
                    ),
                    None,
                ),
                "401": (v1_error_example("webhook_auth_failed", "invalid webhook signature", {}), None),
                "403": (
                    v1_error_example(
                        "webhook_provider_mismatch",
                        "webhook provider does not match payment provider",
                        {},
                    ),
                    None,
                ),
            },
        ),
        ("post", "/v1/admin/machines/{machineId}/sync"): ex(
            req_body={"operator_session_id": "dddddddd-eeee-ffff-0000-111111111111", "reason": "post_restock_verify"},
            resp={"200": (sync_resp, None)},
        ),
        ("post", "/v1/machines/{machineId}/check-ins"): ex(
            req_body={
                "package_name": "com.example.kiosk",
                "version_name": "1.0.0",
                "version_code": 100,
                "android_release": "14",
                "sdk_int": 34,
                "manufacturer": "Example",
                "model": "Kiosk-1",
                "timezone": "America/Los_Angeles",
                "network_state": "wifi",
                "boot_id": "boot-session-1",
                "occurred_at": "2026-04-19T12:00:00Z",
                "metadata": {},
            },
            resp={"201": (check_in_resp, None)},
        ),
        ("post", "/v1/machines/{machineId}/config-applies"): ex(
            req_body={
                "config_version": 7,
                "applied_at": "2026-04-19T12:05:00Z",
                "android_id": "device-android-1",
                "app_version": "1.0.0",
                "config_payload": {"applied_revision": 7},
            },
            resp={"201": (config_apply_resp, None)},
        ),
        ("get", "/v1/machines/{machineId}/operator-sessions/current"): ex(
            resp={"200": ({"active_session": None, "technician_display_name": ""}, None)},
        ),
        ("get", "/v1/machines/{machineId}/operator-sessions/history"): ex(resp={"200": (op_list_ex, None)}),
        ("get", "/v1/machines/{machineId}/operator-sessions/auth-events"): ex(resp={"200": (op_list_ex, None)}),
        ("get", "/v1/machines/{machineId}/operator-sessions/action-attributions"): ex(resp={"200": (op_list_ex, None)}),
        ("get", "/v1/machines/{machineId}/operator-sessions/timeline"): ex(resp={"200": (op_list_ex, None)}),
        ("get", "/v1/machines/{machineId}/commands/receipts"): ex(resp={"200": (op_list_ex, None)}),
        ("get", "/v1/operator-insights/technicians/{technicianId}/action-attributions"): ex(resp={"200": (op_list_ex, None)}),
        ("get", "/v1/operator-insights/users/action-attributions"): ex(resp={"200": (op_list_ex, None)}),
        ("post", "/v1/machines/{machineId}/operator-sessions/logout"): ex(
            req_body={
                "session_id": "dddddddd-eeee-ffff-0000-111111111111",
                "ended_reason": "user_logout",
                "auth_method": "oidc",
            },
            resp={"200": (op_login, None)},
        ),
        ("get", "/v1/admin/organizations/{orgId}/artifacts"): ex(
            resp={"200": ({"items": [], "meta": {"limit": 50, "offset": 0, "returned": 0, "totalCount": 0}}, None)},
        ),
        ("get", "/v1/admin/organizations/{orgId}/artifacts/{artifactId}"): ex(
            resp={"200": ({"artifact_id": "ffffffff-0000-1111-2222-333333333333", "status": "uploaded"}, None)},
        ),
        ("get", "/v1/admin/organizations/{orgId}/artifacts/{artifactId}/download"): ex(
            resp={
                "200": (
                    {
                        "method": "GET",
                        "url": "https://storage.example/presigned-read",
                        "headers": {},
                        "expires_at": "2026-04-19T13:00:00Z",
                    },
                    None,
                ),
            },
        ),
        ("delete", "/v1/admin/organizations/{orgId}/artifacts/{artifactId}"): ex(
            resp={"200": ({"status": "deleted", "artifact_id": "ffffffff-0000-1111-2222-333333333333"}, None)},
        ),
        ("post", "/v1/commerce/cash-checkout"): ex(
            req_body={
                "machine_id": _U3,
                "product_id": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
                "slot_index": 3,
                "currency": "USD",
                "subtotal_minor": 125,
                "tax_minor": 10,
                "total_minor": 135,
            },
            resp={
                "200": (
                    {
                        "order_id": _U,
                        "vend_session_id": "8d3e2f10-1111-2222-3333-444455556666",
                        "payment_id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
                        "order_status": "paid",
                        "payment_state": "captured",
                        "replay": False,
                    },
                    None,
                ),
                "503": (
                    v1_error_example(
                        "capability_not_configured",
                        "commerce outbox defaults are not configured",
                        {"capability": "v1.commerce.payment_session.outbox", "implemented": False},
                    ),
                    None,
                ),
            },
        ),
        ("post", "/v1/commerce/orders"): ex(
            req_body={
                "machine_id": _U3,
                "product_id": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
                "slot_index": 3,
                "currency": "USD",
                "subtotal_minor": 125,
                "tax_minor": 10,
                "total_minor": 135,
            },
            resp={
                "201": (
                    {
                        "order_id": _U,
                        "vend_session_id": "8d3e2f10-1111-2222-3333-444455556666",
                        "replay": False,
                        "order_status": "created",
                        "vend_state": "pending",
                    },
                    None,
                ),
                "400": (v1_error_example("missing_idempotency_key", "missing idempotency key header (Idempotency-Key or X-Idempotency-Key)"), None),
            },
        ),
        ("post", "/v1/commerce/orders/{orderId}/payment-session"): ex(
            req_body={
                "provider": "stripe",
                "payment_state": "created",
                "amount_minor": 135,
                "currency": "USD",
                "outbox_payload_json": {"source": "http_api"},
            },
            resp={
                "200": (
                    {
                        "payment_id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
                        "payment_state": "created",
                        "outbox_event_id": 9001,
                        "replay": False,
                    },
                    None,
                ),
                "503": (
                    v1_error_example(
                        "capability_not_configured",
                        "commerce outbox defaults are not configured",
                        {"capability": "v1.commerce.payment_session.outbox", "implemented": False},
                    ),
                    None,
                ),
            },
        ),
        ("get", "/v1/commerce/orders/{orderId}"): ex(resp={"200": (checkout, None)}),
        ("post", "/v1/commerce/orders/{orderId}/cancel"): ex(
            req_body={"reason": "user_cancelled", "slot_index": 3},
            resp={
                "200": (
                    {
                        "order_id": _U,
                        "order_status": "cancelled",
                        "payment_state": "none",
                        "refund_state": "not_required",
                        "replay": False,
                    },
                    None,
                ),
                "409": (v1_error_example("cancel_not_allowed", "order cannot be cancelled in current state", {}), None),
            },
        ),
        ("post", "/v1/commerce/orders/{orderId}/refunds"): ex(
            req_body={
                "reason": "vend_failed",
                "amount_minor": 15000,
                "currency": "VND",
                "metadata": {"slot_index": 3, "vend_failure_reason": "motor_timeout"},
            },
            resp={
                "200": (
                    {
                        "refund_id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
                        "order_id": _U,
                        "payment_id": "11111111-2222-3333-4444-555555555555",
                        "refund_state": "pending",
                        "amount_minor": 15000,
                        "currency": "VND",
                        "replay": False,
                    },
                    None,
                ),
                "400": (v1_error_example("refund_not_allowed", "refund exceeds captured amount or order unpaid", {}), None),
            },
        ),
        ("get", "/v1/commerce/orders/{orderId}/refunds"): ex(
            resp={
                "200": (
                    {
                        "items": [
                            {
                                "refund_id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
                                "order_id": _U,
                                "payment_id": "11111111-2222-3333-4444-555555555555",
                                "refund_state": "pending",
                                "amount_minor": 15000,
                                "currency": "VND",
                                "created_at": "2026-04-24T00:00:00Z",
                            }
                        ]
                    },
                    None,
                ),
            },
        ),
        ("get", "/v1/commerce/orders/{orderId}/refunds/{refundId}"): ex(
            resp={
                "200": (
                    {
                        "refund_id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
                        "order_id": _U,
                        "payment_id": "11111111-2222-3333-4444-555555555555",
                        "refund_state": "pending",
                        "amount_minor": 15000,
                        "currency": "VND",
                        "created_at": "2026-04-24T00:00:00Z",
                    },
                    None,
                ),
            },
        ),
        ("post", "/v1/commerce/orders/{orderId}/vend/start"): ex(
            req_body={"slot_index": 3},
            resp={"200": ({"vend_state": "in_progress", "slot_index": 3}, None)},
        ),
        ("post", "/v1/commerce/orders/{orderId}/vend/success"): ex(
            req_body={"slot_index": 3},
            resp={"200": ({"order_id": _U, "order_status": "completed", "vend_state": "success"}, None)},
        ),
        ("post", "/v1/commerce/orders/{orderId}/vend/failure"): ex(
            req_body={"slot_index": 3, "failure_reason": "motor_timeout"},
            resp={
                "200": (
                    {
                        "order_id": _U,
                        "order_status": "failed",
                        "vend_state": "failed",
                        "refund_required": True,
                        "local_cash_refund_required": False,
                    },
                    None,
                ),
                "409": (v1_error_example("payment_not_settled", "payment must be captured before vend completion", {}), None),
            },
        ),
        ("post", "/v1/device/machines/{machineId}/vend-results"): ex(
            req_body={
                "order_id": _U,
                "slot_index": 3,
                "outcome": "success",
                "correlation_id": "11111111-2222-3333-4444-555555555555",
            },
            resp={
                "200": (
                    {
                        "order_id": _U,
                        "order_status": "completed",
                        "vend_state": "success",
                        "replay": False,
                    },
                    None,
                ),
            },
        ),
        ("post", "/v1/device/machines/{machineId}/commands/poll"): ex(
            req_body={"limit": 10},
            resp={
                "200": (
                    {
                        "items": [
                            {
                                "sequence": 42,
                                "command_type": "machine_planogram_publish",
                                "payload": {
                                    "planogramId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
                                    "planogramRevision": 3,
                                    "desiredConfigVersion": 7,
                                },
                                "correlation_id": "11111111-2222-3333-4444-555555555555",
                                "idempotency_key": "example",
                            }
                        ],
                        "meta": {"returned": 1},
                    },
                    None,
                ),
            },
        ),
        ("post", "/v1/machines/{machineId}/commands/dispatch"): ex(
            req_body={
                "command_type": "SET_TEMPERATURE",
                "payload": {"celsius": 4},
                "desired_state": {},
            },
            resp={
                "200": (disp, None),
                "503": (
                    v1_error_example(
                        "capability_not_configured",
                        "MQTT broker client is not configured for this API process (set MQTT_BROKER_URL and MQTT_CLIENT_ID)",
                        {"capability": "mqtt_command_dispatch", "implemented": False},
                    ),
                    None,
                ),
            },
        ),
        ("get", "/v1/machines/{machineId}/commands/{sequence}/status"): ex(resp={"200": (st, None)}),
        ("get", "/v1/machines/{machineId}/shadow"): ex(resp={"200": (shadow, None)}),
        ("get", "/v1/machines/{machineId}/telemetry/snapshot"): ex(resp={"200": (telemetry_snapshot_ex, None)}),
        ("get", "/v1/machines/{machineId}/telemetry/incidents"): ex(resp={"200": (telemetry_incidents_ex, None)}),
        ("get", "/v1/machines/{machineId}/telemetry/rollups"): ex(resp={"200": (telemetry_rollups_ex, None)}),
        ("get", "/v1/setup/machines/{machineId}/bootstrap"): ex(resp={"200": (bootstrap_resp, None)}),
        ("post", "/v1/setup/activation-codes/claim"): ex(
            req_body={"activationCode": "AVF-123456-ABCDEF", "deviceFingerprint": fingerprint_ex},
            resp={
                "200": (
                    {
                        "machineId": _U3,
                        "organizationId": _U2,
                        "siteId": "11111111-2222-3333-4444-555555555555",
                        "machineName": "Lobby A",
                        "machineToken": "<jwt>",
                        "tokenExpiresAt": "2026-04-24T00:00:00Z",
                        "mqtt": {"brokerUrl": "ssl://mqtt.example.com:8883", "topicPrefix": "avf/devices"},
                        "bootstrapUrl": "/v1/setup/machines/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee/bootstrap",
                    },
                    None,
                ),
                "400": (v1_error_example("activation_invalid", "activation code is not valid", {}), None),
            },
        ),
        ("post", "/v1/admin/machines/{machineId}/activation-codes"): ex(
            req_body={"expiresInMinutes": 1440, "maxUses": 1, "notes": "Field install at site A"},
            resp={
                "201": (
                    {
                        "activationCode": "AVF-123456-ABCDEF",
                        "activationCodeId": "11111111-2222-3333-4444-555555555555",
                        "machineId": _U3,
                        "expiresAt": "2026-04-24T00:00:00Z",
                        "maxUses": 1,
                        "remainingUses": 1,
                        "status": "active",
                    },
                    None,
                ),
                "403": (v1_error_example("forbidden", "caller lacks permission for this resource", {}), None),
            },
        ),
        ("get", "/v1/admin/machines/{machineId}/activation-codes"): ex(
            resp={
                "200": (
                    {
                        "items": [
                            {
                                "activationCodeId": "11111111-2222-3333-4444-555555555555",
                                "machineId": _U3,
                                "expiresAt": "2026-04-24T00:00:00Z",
                                "maxUses": 1,
                                "uses": 0,
                                "remainingUses": 1,
                                "status": "active",
                                "notes": "Field install",
                                "createdAt": "2026-04-23T00:00:00Z",
                            }
                        ]
                    },
                    None,
                ),
            },
        ),
        ("delete", "/v1/admin/machines/{machineId}/activation-codes/{activationCodeId}"): ex(
            resp={"204": ("", None)},
        ),
        ("get", "/v1/admin/machines/{machineId}/inventory"): ex(resp={"200": (inv_by_product_ex, None)}),
        ("get", "/v1/admin/inventory/low-stock"): ex(resp={"200": (refill_forecast_ex, None)}),
        ("get", "/v1/admin/inventory/refill-suggestions"): ex(resp={"200": (refill_forecast_ex, None)}),
        ("get", "/v1/admin/machines/{machineId}/refill-suggestions"): ex(resp={"200": (refill_forecast_ex, None)}),
        ("put", "/v1/admin/organizations/{orgId}/artifacts/{artifactId}/content"): ex(
            resp={"200": (art_upload_ok, None)},
        ),
        ("get", "/v1/machines/{machineId}/sale-catalog"): ex(resp={"200": (sale_catalog_ex, None)}),
        ("post", "/v1/device/machines/{machineId}/events/reconcile"): ex(
            req_body={"idempotencyKeys": ["machine-001:boot-20260424:seq-100:events.vend"]},
            resp={
                "200": (
                    {
                        "machineId": _U3,
                        "items": [
                            {
                                "idempotencyKey": "machine-001:boot-20260424:seq-100:events.vend",
                                "status": "processed",
                                "eventType": "events.vend",
                                "acceptedAt": "2026-04-24T00:00:00Z",
                                "processedAt": "2026-04-24T00:00:10Z",
                                "retryable": False,
                            }
                        ],
                    },
                    None,
                ),
                "400": (v1_error_example("invalid_batch_size", "idempotencyKeys must contain 1 to 500 entries", {}), None),
            },
        ),
        ("get", "/v1/device/machines/{machineId}/events/{idempotencyKey}/status"): ex(
            resp={
                "200": (
                    {
                        "idempotencyKey": "machine-001:boot-20260424:seq-100:events.vend",
                        "status": "not_found",
                        "eventType": None,
                        "acceptedAt": None,
                        "processedAt": None,
                        "retryable": True,
                    },
                    None,
                ),
            },
        ),
        ("put", "/v1/admin/machines/{machineId}/topology"): ex(req_body=topology_req),
        ("put", "/v1/admin/machines/{machineId}/planograms/draft"): ex(req_body=planogram_draft_req),
        ("post", "/v1/admin/machines/{machineId}/planograms/publish"): ex(
            req_body=planogram_draft_req,
            resp={"200": (planogram_publish_resp, None)},
        ),
        ("get", "/v1/admin/machines/{machineId}/inventory-events"): ex(
            resp={
                "200": (
                    {
                        "items": [
                            {
                                "id": 1001,
                                "organizationId": _U2,
                                "machineId": _U3,
                                "cabinetCode": "CAB-A",
                                "slotCode": "legacy-0",
                                "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
                                "eventType": "adjustment",
                                "reasonCode": "manual_adjustment",
                                "quantityBefore": 5,
                                "quantityDelta": 2,
                                "quantityAfter": 7,
                                "unitPriceMinor": 199,
                                "currency": "USD",
                                "occurredAt": "2026-04-19T12:34:56.123456789Z",
                                "recordedAt": "2026-04-19T12:34:57.000000000Z",
                            }
                        ]
                    },
                    None,
                ),
            },
        ),
        ("post", "/v1/admin/machines/{machineId}/stock-adjustments"): ex(
            req_body=stock_adj_req,
            resp={"200": (stock_adj_resp, None)},
        ),
        ("get", "/v1/admin/machines/{machineId}/cashbox"): ex(resp={"200": (cashbox_ex, None)}),
        ("post", "/v1/admin/machines/{machineId}/cash-collections"): ex(
            req_body=cash_coll_start_req,
            resp={"200": (cash_coll_open_ex, None)},
        ),
        ("get", "/v1/admin/machines/{machineId}/cash-collections"): ex(
            resp={
                "200": (
                    {"items": [cash_coll_closed_exact_ex, cash_coll_review_ex], "meta": cmeta},
                    None,
                )
            },
        ),
        ("get", "/v1/admin/machines/{machineId}/cash-collections/{collectionId}"): ex(
            resp={"200": (cash_coll_closed_ex, None)},
        ),
        ("post", "/v1/admin/machines/{machineId}/cash-collections/{collectionId}/close"): ex(
            req_body=cash_coll_close_req,
            resp={
                "200": (cash_coll_closed_ex, None),
                "409": (
                    v1_error_example(
                        "close_payload_conflict",
                        "close payload does not match stored close",
                        {},
                    ),
                    None,
                ),
            },
        ),
        ("post", "/v1/machines/{machineId}/operator-sessions/login"): ex(
            req_body={"auth_method": "oidc", "client_metadata": {"kiosk": "A12"}},
            resp={"200": (op_login, None)},
        ),
        ("post", "/v1/machines/{machineId}/operator-sessions/{sessionId}/heartbeat"): ex(
            resp={"200": ({"session": op_login}, None)},
        ),
        ("post", "/v1/admin/organizations/{orgId}/artifacts"): ex(
            req_body={"content_type": "application/zip", "original_filename": "bundle.zip"},
            resp={"201": (art_reserve, None)},
        ),
        ("get", "/v1/orders"): ex(resp={"200": ({"items": [ord_item], "meta": cmeta}, None)}),
        ("get", "/v1/payments"): ex(resp={"200": ({"items": [pay_item], "meta": cmeta}, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/commerce/reconciliation"): ex(
            resp={"200": ({"items": [recon_case], "meta": cmeta}, None)}
        ),
        ("get", "/v1/admin/organizations/{organizationId}/commerce/reconciliation/{id}"): ex(resp={"200": (recon_case, None)}),
        ("post", "/v1/admin/organizations/{organizationId}/commerce/reconciliation/{id}/resolve"): ex(
            req_body={"status": "resolved", "note": "Refund requested in PSP dashboard"},
            resp={"200": ({**recon_case, "status": "resolved", "resolutionNote": "Refund requested in PSP dashboard"}, None)},
        ),
        ("post", "/v1/admin/organizations/{organizationId}/commerce/reconciliation/{id}/ignore"): ex(
            req_body={"note": "Known benign duplicate webhook"},
            resp={"200": ({**recon_case, "status": "ignored"}, None)},
        ),
        ("get", "/v1/admin/organizations/{organizationId}/orders/{orderId}/timeline"): ex(
            resp={"200": ({"items": [], "meta": cmeta}, None)},
        ),
        ("get", "/v1/admin/organizations/{organizationId}/refunds"): ex(
            resp={"200": ({"items": [], "meta": cmeta}, None)},
        ),
        ("get", "/v1/admin/organizations/{organizationId}/refunds/{refundId}"): ex(resp={"200": ({}, None)}),
        ("post", "/v1/admin/organizations/{organizationId}/orders/{orderId}/refunds"): ex(
            req_body={"amountMinor": 100, "reason": "customer courtesy"},
            resp={
                "200": (
                    {
                        "refundRequest": {},
                        "ledgerRefundID": _U,
                        "ledgerState": "requested",
                        "ledgerAmountMinor": 100,
                        "ledgerCurrency": "USD",
                    },
                    None,
                )
            },
        ),
        ("get", "/v1/admin/machines"): ex(resp={"200": ({"items": [mach_item], "meta": cmeta}, None)}),
        ("get", "/v1/admin/sites"): ex(resp={"200": ({"items": [site_row], "meta": cmeta}, None)}),
        ("get", "/v1/admin/sites/{siteId}"): ex(resp={"200": (site_row, None)}),
        ("post", "/v1/admin/sites"): ex(
            req_body={"name": "HQ Lobby", "timezone": "America/New_York", "code": "HQ-01", "address": {}},
            resp={"201": (site_row, None)},
        ),
        ("patch", "/v1/admin/sites/{siteId}"): ex(req_body={"name": "HQ Lobby West"}, resp={"200": (site_row, None)}),
        ("post", "/v1/admin/sites/{siteId}/disable"): ex(resp={"200": ({**site_row, "status": "inactive"}, None)}),
        ("delete", "/v1/admin/sites/{siteId}"): ex(resp={"200": ({**site_row, "status": "inactive"}, None)}),
        ("post", "/v1/admin/machines"): ex(
            req_body={"site_id": site_row["id"], "serial_number": "SN-NEW", "name": "New unit"},
            resp={"201": (machine_row_fleet, None)},
        ),
        ("patch", "/v1/admin/machines/{machineId}"): ex(req_body={"name": "Renamed unit"}, resp={"200": (machine_row_fleet, None)}),
        ("post", "/v1/admin/machines/{machineId}/disable"): ex(resp={"200": (machine_row_fleet, None)}),
        ("post", "/v1/admin/machines/{machineId}/enable"): ex(resp={"200": ({**machine_row_fleet, "status": "offline"}, None)}),
        ("post", "/v1/admin/machines/{machineId}/retire"): ex(resp={"200": (machine_row_fleet, None)}),
        ("post", "/v1/admin/machines/{machineId}/rotate-credential"): ex(resp={"200": (machine_row_fleet, None)}),
        ("get", "/v1/admin/technicians"): ex(resp={"200": ({"items": [tech_item], "meta": cmeta}, None)}),
        ("get", "/v1/admin/technicians/{technicianId}"): ex(resp={"200": (technician_detail_snake, None)}),
        ("post", "/v1/admin/technicians"): ex(
            req_body={"display_name": "Alex Tech", "email": "alex@example.com"},
            resp={"201": (technician_detail_snake, None)},
        ),
        ("patch", "/v1/admin/technicians/{technicianId}"): ex(
            req_body={"display_name": "Alex T."},
            resp={"200": (technician_detail_snake, None)},
        ),
        ("post", "/v1/admin/technicians/{technicianId}/disable"): ex(resp={"200": (technician_detail_snake, None)}),
        ("post", "/v1/admin/technicians/{technicianId}/enable"): ex(resp={"200": ({**technician_detail_snake, "status": "active"}, None)}),
        ("get", "/v1/admin/assignments"): ex(resp={"200": ({"items": [asg_item], "meta": cmeta}, None)}),
        ("get", "/v1/admin/technician-assignments"): ex(resp={"200": ({"items": [asg_item], "meta": cmeta}, None)}),
        ("post", "/v1/admin/technician-assignments"): ex(
            req_body={"technician_id": tech_item["technicianId"], "machine_id": _U3, "role": "maintainer"},
            resp={"201": (assignment_detail_snake, None)},
        ),
        ("get", "/v1/admin/technician-assignments/{assignmentId}"): ex(resp={"200": (assignment_detail_snake, None)}),
        ("patch", "/v1/admin/technician-assignments/{assignmentId}"): ex(
            req_body={"role": "lead"},
            resp={"200": (assignment_detail_snake, None)},
        ),
        ("post", "/v1/admin/technician-assignments/{assignmentId}/cancel"): ex(resp={"200": (assignment_detail_snake, None)}),
        ("delete", "/v1/admin/technician-assignments/{assignmentId}"): ex(resp={"200": (assignment_detail_snake, None)}),
        ("get", "/v1/admin/commands"): ex(resp={"200": ({"items": [cmd_item], "meta": cmeta}, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/sites"): ex(resp={"200": ({"items": [{"id": _U, "organization_id": _U2, "name": "Lobby", "timezone": "UTC", "code": "LOBBY", "status": "active", "created_at": "2026-04-29T00:00:00Z", "updated_at": "2026-04-29T00:00:00Z", "address": {}}], "meta": cmeta}, None)}),
        ("post", "/v1/admin/organizations/{organizationId}/sites"): ex(req_body={"name": "Lobby", "timezone": "UTC", "code": "LOBBY", "address": {"line1": "1 Main St"}}, resp={"201": ({"id": _U, "organization_id": _U2, "name": "Lobby", "timezone": "UTC", "code": "LOBBY", "status": "active", "created_at": "2026-04-29T00:00:00Z", "updated_at": "2026-04-29T00:00:00Z", "address": {}}, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/sites/{siteId}"): ex(resp={"200": ({"id": _U, "organization_id": _U2, "name": "Lobby", "timezone": "UTC", "code": "LOBBY", "status": "active", "created_at": "2026-04-29T00:00:00Z", "updated_at": "2026-04-29T00:00:00Z", "address": {}}, None)}),
        ("patch", "/v1/admin/organizations/{organizationId}/sites/{siteId}"): ex(req_body={"name": "Lobby North", "status": "active"}, resp={"200": ({"id": _U, "organization_id": _U2, "name": "Lobby North", "timezone": "UTC", "code": "LOBBY", "status": "active", "created_at": "2026-04-29T00:00:00Z", "updated_at": "2026-04-29T00:05:00Z", "address": {}}, None)}),
        ("post", "/v1/admin/organizations/{organizationId}/sites/{siteId}/archive"): ex(resp={"200": ({"id": _U, "organization_id": _U2, "name": "Lobby", "timezone": "UTC", "code": "LOBBY", "status": "inactive", "created_at": "2026-04-29T00:00:00Z", "updated_at": "2026-04-29T00:05:00Z", "address": {}}, None)}),
        (
            "delete",
            "/v1/admin/organizations/{organizationId}/sites/{siteId}",
        ): ex(resp={"200": ({"id": _U, "organization_id": _U2, "name": "Lobby", "timezone": "UTC", "code": "LOBBY", "status": "archived", "created_at": "2026-04-29T00:00:00Z", "updated_at": "2026-04-29T00:10:00Z", "address": {}}, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/machines"): ex(resp={"200": ({"items": [mach_item], "meta": cmeta}, None)}),
        ("post", "/v1/admin/organizations/{organizationId}/machines"): ex(req_body={"siteId": _U, "serialNumber": "SN-001", "code": "M001", "name": "Lobby A", "model": "AVF-1", "cabinetType": "ambient", "timezone": "UTC", "status": "draft"}, resp={"201": ({"id": _U3, "organization_id": _U2, "site_id": _U, "serial_number": "SN-001", "code": "M001", "name": "Lobby A", "status": "draft", "credential_version": 0, "command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "updated_at": "2026-04-29T00:00:00Z"}, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/machines/{machineId}"): ex(resp={"200": (mach_item, None)}),
        ("patch", "/v1/admin/organizations/{organizationId}/machines/{machineId}"): ex(req_body={"name": "Lobby A1", "status": "active"}, resp={"200": ({"id": _U3, "organization_id": _U2, "site_id": _U, "serial_number": "SN-001", "code": "M001", "name": "Lobby A1", "status": "active", "credential_version": 0, "command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "updated_at": "2026-04-29T00:05:00Z"}, None)}),
        ("post", "/v1/admin/organizations/{organizationId}/machines/{machineId}/archive"): ex(resp={"200": ({"id": _U3, "organization_id": _U2, "site_id": _U, "serial_number": "SN-001", "name": "Lobby A", "status": "decommissioned", "credential_version": 0, "command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "updated_at": "2026-04-29T00:05:00Z"}, None)}),
        ("post", "/v1/admin/organizations/{organizationId}/machines/{machineId}/suspend"): ex(resp={"200": ({"id": _U3, "organization_id": _U2, "site_id": _U, "serial_number": "SN-001", "name": "Lobby A", "status": "suspended", "credential_version": 0, "command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "updated_at": "2026-04-29T00:05:00Z"}, None)}),
        ("post", "/v1/admin/organizations/{organizationId}/machines/{machineId}/resume"): ex(resp={"200": ({"id": _U3, "organization_id": _U2, "site_id": _U, "serial_number": "SN-001", "name": "Lobby A", "status": "active", "credential_version": 0, "command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "updated_at": "2026-04-29T00:05:00Z"}, None)}),
        ("post", "/v1/admin/organizations/{organizationId}/machines/{machineId}/mark-compromised"): ex(resp={"200": ({"id": _U3, "organization_id": _U2, "site_id": _U, "serial_number": "SN-001", "name": "Lobby A", "status": "compromised", "credential_version": 1, "revoked_at": "2026-04-29T00:05:00Z", "command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "updated_at": "2026-04-29T00:05:00Z"}, None)}),
        ("post", "/v1/admin/organizations/{organizationId}/machines/{machineId}/rotate-credentials"): ex(resp={"200": ({"id": _U3, "organization_id": _U2, "site_id": _U, "serial_number": "SN-001", "name": "Lobby A", "status": "active", "credential_version": 2, "rotated_at": "2026-04-29T00:05:00Z", "command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "updated_at": "2026-04-29T00:05:00Z"}, None)}),
        ("post", "/v1/admin/organizations/{organizationId}/machines/{machineId}/revoke-credentials"): ex(resp={"200": ({"id": _U3, "organization_id": _U2, "site_id": _U, "serial_number": "SN-001", "name": "Lobby A", "status": "active", "credential_version": 3, "revoked_at": "2026-04-29T00:05:00Z", "command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "updated_at": "2026-04-29T00:05:00Z"}, None)}),
        (
            "post",
            "/v1/admin/organizations/{organizationId}/machines/{machineId}/rotate-token-version",
        ): ex(resp={"200": ({"id": _U3, "organization_id": _U2, "site_id": _U, "serial_number": "SN-001", "name": "Lobby A", "status": "active", "credential_version": 2, "rotated_at": "2026-04-29T00:05:00Z", "command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "updated_at": "2026-04-29T00:05:00Z"}, None)}),
        (
            "post",
            "/v1/admin/organizations/{organizationId}/machines/{machineId}/revoke-token",
        ): ex(resp={"200": ({"id": _U3, "organization_id": _U2, "site_id": _U, "serial_number": "SN-001", "name": "Lobby A", "status": "active", "credential_version": 3, "revoked_at": "2026-04-29T00:05:00Z", "command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "updated_at": "2026-04-29T00:05:00Z"}, None)}),
        (
            "post",
            "/v1/admin/organizations/{organizationId}/machines/{machineId}/transfer-site",
        ): ex(
            req_body={"site_id": "11111111-2222-3333-4444-555555555555"},
            resp={
                "200": (
                    {
                        "id": _U3,
                        "organization_id": _U2,
                        "site_id": "11111111-2222-3333-4444-555555555555",
                        "serial_number": "SN-001",
                        "name": "Lobby A",
                        "status": "active",
                        "credential_version": 0,
                        "command_sequence": 0,
                        "created_at": "2026-04-29T00:00:00Z",
                        "updated_at": "2026-04-29T00:06:00Z",
                    },
                    None,
                )
            },
        ),
        ("post", "/v1/admin/machines/{machineId}/diagnostics/requests"): ex(req_body={"reason": "field pilot log bundle"}, resp={"202": ({"requestId": _U, "machineId": _U3, "commandId": _U2, "sequence": 42, "dispatchState": "published", "replay": False}, None)}),
        ("get", "/v1/admin/machines/{machineId}/diagnostics/bundles"): ex(resp={"200": ({"items": [{"bundleId": _U, "organizationId": _U2, "machineId": _U3, "requestId": _U, "commandId": _U2, "storageKey": "diagnostics/org/machine/bundle.tgz", "storageProvider": "s3", "contentType": "application/gzip", "sizeBytes": 1024, "sha256Hex": "abc123", "metadata": {"app_version": "1.2.3"}, "status": "available", "createdAt": "2026-04-29T00:00:00Z"}], "meta": {"limit": 50, "offset": 0, "returned": 1}}, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/machines/{machineId}/technicians"): ex(resp={"200": ({"items": [{"assignmentId": _U, "technicianId": _U2, "technicianDisplayName": "Field Tech", "machineId": _U3, "machineName": "Lobby A", "machineSerialNumber": "SN-001", "role": "field_service", "validFrom": "2026-04-29T00:00:00Z", "createdAt": "2026-04-29T00:00:00Z"}], "meta": cmeta}, None)}),
        ("post", "/v1/admin/organizations/{organizationId}/machines/{machineId}/technicians"): ex(req_body={"userId": _U2, "role": "field_service", "scope": "maintenance"}, resp={"201": ({"id": _U, "organization_id": _U2, "technician_id": _U2, "machine_id": _U3, "role": "field_service", "scope": "maintenance", "status": "active", "valid_from": "2026-04-29T00:00:00Z", "created_at": "2026-04-29T00:00:00Z", "updated_at": "2026-04-29T00:00:00Z"}, None)}),
        ("delete", "/v1/admin/organizations/{organizationId}/machines/{machineId}/technicians/{userId}"): ex(resp={"204": ("", None)}),
        ("get", "/v1/admin/organizations/{organizationId}/technicians"): ex(resp={"200": ({"items": [tech_item], "meta": cmeta}, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/technicians/{technicianId}"): ex(resp={"200": (technician_detail_snake, None)}),
        ("post", "/v1/admin/organizations/{organizationId}/technicians"): ex(
            req_body={"display_name": "Alex Tech", "email": "alex@example.com"},
            resp={"201": (technician_detail_snake, None)},
        ),
        ("patch", "/v1/admin/organizations/{organizationId}/technicians/{technicianId}"): ex(
            req_body={"display_name": "Alex Field"},
            resp={"200": ({**technician_detail_snake, "display_name": "Alex Field"}, None)},
        ),
        ("post", "/v1/admin/organizations/{organizationId}/technicians/{technicianId}/disable"): ex(
            resp={"200": ({**technician_detail_snake, "status": "inactive"}, None)},
        ),
        ("post", "/v1/admin/organizations/{organizationId}/technicians/{technicianId}/enable"): ex(
            resp={"200": ({**technician_detail_snake, "status": "active"}, None)},
        ),
        ("get", "/v1/admin/organizations/{organizationId}/assignments"): ex(resp={"200": ({"items": [asg_item], "meta": cmeta}, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/assignments/{assignmentId}"): ex(resp={"200": (assignment_detail_snake, None)}),
        ("post", "/v1/admin/organizations/{organizationId}/assignments"): ex(
            req_body={"technician_id": _U2, "machine_id": _U3, "role": "field_service"},
            resp={"201": (assignment_detail_snake, None)},
        ),
        ("delete", "/v1/admin/organizations/{organizationId}/assignments/{assignmentId}"): ex(resp={"200": (assignment_detail_snake, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/activation-codes"): ex(
            resp={
                "200": (
                    {
                        "items": [
                            {
                                "activationCodeId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
                                "machineId": _U3,
                                "expiresAt": "2026-04-30T00:00:00Z",
                                "maxUses": 1,
                                "uses": 0,
                                "remainingUses": 1,
                                "status": "active",
                                "notes": "",
                                "createdAt": "2026-04-29T00:00:00Z",
                            }
                        ],
                        "meta": admin_page_meta,
                    },
                    None,
                )
            },
        ),
        (
            "post",
            "/v1/admin/organizations/{organizationId}/activation-codes",
        ): ex(
            req_body={"machineId": _U3, "expiresInMinutes": 1440, "maxUses": 1, "notes": "pilot"},
            resp={
                "201": (
                    {
                        "activationCode": "AVF-123456",
                        "activationCodeId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
                        "machineId": _U3,
                        "expiresAt": "2026-04-30T00:00:00Z",
                        "maxUses": 1,
                        "remainingUses": 1,
                        "status": "active",
                    },
                    None,
                )
            },
        ),
        (
            "post",
            "/v1/admin/organizations/{organizationId}/activation-codes/{codeId}/revoke",
        ): ex(resp={"204": ("", None)}),
        ("get", "/v1/admin/organizations/{organizationId}/operations/machines/health"): ex(
            resp={
                "200": (
                    {
                        "items": [
                            {
                                "machineId": _U3,
                                "status": "online",
                                "pendingCommandCount": 1,
                                "failedCommandCount": 0,
                                "inventoryAnomalyCount": 0,
                                "lastSeenAt": "2026-04-29T12:00:00.000000000Z",
                                "lastCheckInAt": "2026-04-29T11:58:00.000000000Z",
                                "appVersion": "1.4.2",
                                "configVersion": "7",
                                "catalogVersion": "2026-04-29T00:00:00Z",
                                "mediaVersion": "sha256:abcd0000",
                                "mqttConnected": True,
                                "lastErrorCode": "TEMP_SENSOR_DEGRADED",
                                "telemetryFreshnessSeconds": 95,
                            }
                        ]
                    },
                    None,
                )
            },
        ),
        ("get", "/v1/admin/organizations/{organizationId}/machines/{machineId}/health"): ex(
            resp={
                "200": (
                    {
                        "machineId": _U3,
                        "status": "online",
                        "pendingCommandCount": 1,
                        "failedCommandCount": 0,
                        "inventoryAnomalyCount": 0,
                        "lastSeenAt": "2026-04-29T12:00:00.000000000Z",
                        "telemetryFreshnessSeconds": 95,
                    },
                    None,
                )
            },
        ),
        ("get", "/v1/admin/organizations/{organizationId}/machines/{machineId}/timeline"): ex(
            resp={
                "200": (
                    {
                        "items": [
                            {
                                "occurredAt": "2026-04-29T12:00:00.000000000Z",
                                "eventKind": "command_attempt",
                                "title": "Attempt sent",
                                "payload": {"status": "sent"},
                                "refId": "cccccccc-dddd-eeee-ffff-000000000001",
                            }
                        ]
                    },
                    None,
                )
            },
        ),
        ("get", "/v1/admin/organizations/{organizationId}/commands"): ex(resp={"200": ({"items": [cmd_item], "meta": cmeta}, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/commands/{commandId}"): ex(
            resp={
                "200": (
                    {
                        "commandId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
                        "machineId": _U3,
                        "organizationId": _U2,
                        "sequence": 42,
                        "commandType": "SET_TEMPERATURE",
                        "payload": {"celsius": 4},
                        "createdAt": "2026-04-29T12:00:00.000000000Z",
                        "idempotencyKey": "idem-retry-safe",
                        "attempts": [
                            {
                                "id": "cccccccc-dddd-eeee-ffff-000000000001",
                                "attemptNo": 1,
                                "status": "failed",
                                "sentAt": "2026-04-29T12:00:10.000000000Z",
                                "dispatchState": "failed",
                                "timeoutReason": "mqtt_timeout",
                            }
                        ],
                    },
                    None,
                )
            },
        ),
        ("post", "/v1/admin/organizations/{organizationId}/commands/{commandId}/retry"): ex(
            resp={
                "200": (
                    {
                        "commandId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
                        "sequence": 42,
                        "attemptId": "dddddddd-eeee-ffff-0000-111111111111",
                        "dispatchState": "published",
                        "replay": False,
                        "skippedRepublish": False,
                    },
                    None,
                )
            },
        ),
        ("post", "/v1/admin/organizations/{organizationId}/commands/{commandId}/cancel"): ex(
            resp={"200": ({"attemptsCancelled": 1}, None)},
        ),
        ("post", "/v1/admin/organizations/{organizationId}/machines/{machineId}/commands"): ex(
            req_body={"commandType": "REQUEST_DIAGNOSTICS", "payload": {"bundle": "logs"}},
            resp={
                "202": (
                    {
                        "commandId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
                        "sequence": 43,
                        "attemptId": "dddddddd-eeee-ffff-0000-111111111111",
                        "dispatchState": "published",
                        "replay": False,
                    },
                    None,
                ),
                "503": (
                    v1_error_example(
                        "capability_not_configured",
                        "MQTT command publisher is not configured",
                        {"capability": "mqtt_dispatch", "implemented": False},
                    ),
                    None,
                ),
            },
        ),
        ("get", "/v1/admin/organizations/{organizationId}/inventory/anomalies"): ex(
            resp={
                "200": (
                    {
                        "items": [
                            {
                                "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
                                "organizationId": _U2,
                                "machineId": _U3,
                                "machineName": "Lobby A",
                                "machineSerialNumber": "SN-001",
                                "anomalyType": "negative_stock",
                                "status": "open",
                                "fingerprint": "negative-stock:A3",
                                "detectedAt": "2026-04-29T12:00:00.000000000Z",
                                "createdAt": "2026-04-29T12:00:00.000000000Z",
                                "updatedAt": "2026-04-29T12:00:00.000000000Z",
                                "slotCode": "A3",
                                "payload": {"quantity": -1},
                            }
                        ]
                    },
                    None,
                )
            },
        ),
        ("get", "/v1/admin/organizations/{organizationId}/machines/{machineId}/inventory/anomalies"): ex(
            resp={
                "200": (
                    {
                        "items": [
                            {
                                "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
                                "organizationId": _U2,
                                "machineId": _U3,
                                "machineName": "Lobby A",
                                "machineSerialNumber": "SN-001",
                                "anomalyType": "stale_inventory_sync",
                                "status": "open",
                                "fingerprint": "stale-sync",
                                "detectedAt": "2026-04-29T12:00:00.000000000Z",
                                "createdAt": "2026-04-29T12:00:00.000000000Z",
                                "updatedAt": "2026-04-29T12:00:00.000000000Z",
                                "payload": {"publishedVersion": 3, "snapshotVersion": 2},
                            }
                        ]
                    },
                    None,
                )
            },
        ),
        ("post", "/v1/admin/organizations/{organizationId}/inventory/anomalies/{anomalyId}/resolve"): ex(
            req_body={"note": "Verified physical count"},
            resp={"200": ({"anomalyId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "status": "resolved"}, None)},
        ),
        ("get", "/v1/admin/organizations/{organizationId}/anomalies"): ex(
            resp={
                "200": (
                    {
                        "items": [
                            {
                                "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
                                "organizationId": _U2,
                                "machineId": _U3,
                                "machineName": "Lobby A",
                                "machineSerialNumber": "SN-001",
                                "anomalyType": "machine_offline_too_long",
                                "status": "open",
                                "fingerprint": "offline_long|" + _U3,
                                "detectedAt": "2026-04-29T12:00:00.000000000Z",
                                "createdAt": "2026-04-29T12:00:00.000000000Z",
                                "updatedAt": "2026-04-29T12:00:00.000000000Z",
                                "payload": {"last_seen_at": "2026-04-29T08:00:00Z", "threshold": "2 hours"},
                            }
                        ]
                    },
                    None,
                )
            },
        ),
        ("get", "/v1/admin/organizations/{organizationId}/anomalies/{anomalyId}"): ex(
            resp={
                "200": (
                    {
                        "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
                        "organizationId": _U2,
                        "machineId": _U3,
                        "machineName": "Lobby A",
                        "machineSerialNumber": "SN-001",
                        "anomalyType": "repeated_vend_failure",
                        "status": "open",
                        "fingerprint": "repeated_vend_failure|" + _U3,
                        "detectedAt": "2026-04-29T12:00:00.000000000Z",
                        "createdAt": "2026-04-29T12:00:00.000000000Z",
                        "updatedAt": "2026-04-29T12:00:00.000000000Z",
                        "payload": {"failed_vend_count_24h": 4},
                    },
                    None,
                )
            },
        ),
        ("post", "/v1/admin/organizations/{organizationId}/anomalies/{anomalyId}/resolve"): ex(
            req_body={"note": "Field verified"},
            resp={"200": ({"anomalyId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "status": "resolved"}, None)},
        ),
        ("post", "/v1/admin/organizations/{organizationId}/anomalies/{anomalyId}/ignore"): ex(
            req_body={"note": "Benign cluster"},
            resp={"200": ({"anomalyId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "status": "ignored"}, None)},
        ),
        ("get", "/v1/admin/organizations/{organizationId}/restock/suggestions"): ex(resp={"200": (refill_forecast_ex, None)}),
        ("post", "/v1/admin/organizations/{organizationId}/machines/{machineId}/inventory/reconcile"): ex(
            req_body={"reason": "Field recount completed"},
            resp={"202": ({"inventoryEventId": 9001}, None)},
        ),
        ("post", "/v1/admin/organizations/{organizationId}/provisioning/machines/bulk"): ex(
            req_body={
                "siteId": _U,
                "cabinetType": "ambient",
                "machines": [{"serialNumber": "SN-BULK-001", "name": "Lobby", "model": "AVF-1"}],
                "generateActivationCodes": False,
            },
            resp={
                "201": (
                    {
                        "batchId": _U3,
                        "status": "completed",
                        "machineCount": 1,
                        "machines": [{"machineId": _U2, "serialNumber": "SN-BULK-001"}],
                    },
                    None,
                )
            },
        ),
        ("get", "/v1/admin/organizations/{organizationId}/provisioning/batches/{batchId}"): ex(
            resp={
                "200": (
                    {
                        "batch": {
                            "id": _U3,
                            "organizationId": _U2,
                            "siteId": _U,
                            "cabinetType": "ambient",
                            "status": "completed",
                            "machineCount": 1,
                            "createdAt": "2026-04-29T12:00:00.000000000Z",
                            "updatedAt": "2026-04-29T12:00:00.000000000Z",
                            "metadata": {},
                        },
                        "machines": [],
                    },
                    None,
                )
            },
        ),
        ("post", "/v1/admin/organizations/{organizationId}/rollouts"): ex(
            req_body={
                "rolloutType": "config_version",
                "targetVersion": "2026-04-29T00:00:00Z",
                "strategy": {"canary_percent": 10, "confirm_full_rollout": False},
            },
            resp={
                "201": (
                    {
                        "id": _U3,
                        "organizationId": _U2,
                        "rolloutType": "config_version",
                        "targetVersion": "2026-04-29T00:00:00Z",
                        "status": "pending",
                        "strategy": {"canary_percent": 10},
                        "createdAt": "2026-04-29T12:00:00.000000000Z",
                        "updatedAt": "2026-04-29T12:00:00.000000000Z",
                    },
                    None,
                )
            },
        ),
        ("get", "/v1/admin/organizations/{organizationId}/rollouts"): ex(
            resp={
                "200": (
                    {
                        "items": [],
                        "meta": {"limit": 50, "offset": 0, "returned": 0},
                    },
                    None,
                )
            },
        ),
        ("get", "/v1/admin/organizations/{organizationId}/rollouts/{rolloutId}"): ex(
            resp={
                "200": (
                    {
                        "campaign": {
                            "id": _U3,
                            "organizationId": _U2,
                            "rolloutType": "config_version",
                            "targetVersion": "2026-04-29T00:00:00Z",
                            "status": "running",
                            "strategy": {},
                            "createdAt": "2026-04-29T12:00:00.000000000Z",
                            "updatedAt": "2026-04-29T12:00:00.000000000Z",
                        },
                        "targets": [],
                    },
                    None,
                )
            },
        ),
        ("post", "/v1/admin/organizations/{organizationId}/rollouts/{rolloutId}/start"): ex(
            resp={
                "200": (
                    {
                        "campaign": {
                            "id": _U3,
                            "organizationId": _U2,
                            "rolloutType": "config_version",
                            "targetVersion": "2026-04-29T00:00:00Z",
                            "status": "completed",
                            "strategy": {},
                            "createdAt": "2026-04-29T12:00:00.000000000Z",
                            "updatedAt": "2026-04-29T12:00:00.000000000Z",
                        },
                        "targets": [],
                    },
                    None,
                )
            },
        ),
        ("post", "/v1/admin/organizations/{organizationId}/rollouts/{rolloutId}/pause"): ex(
            resp={
                "200": (
                    {
                        "campaign": {
                            "id": _U3,
                            "organizationId": _U2,
                            "rolloutType": "config_version",
                            "targetVersion": "2026-04-29T00:00:00Z",
                            "status": "paused",
                            "strategy": {},
                            "createdAt": "2026-04-29T12:00:00.000000000Z",
                            "updatedAt": "2026-04-29T12:00:00.000000000Z",
                        },
                        "targets": [],
                    },
                    None,
                )
            },
        ),
        ("post", "/v1/admin/organizations/{organizationId}/rollouts/{rolloutId}/resume"): ex(
            resp={
                "200": (
                    {
                        "campaign": {
                            "id": _U3,
                            "organizationId": _U2,
                            "rolloutType": "config_version",
                            "targetVersion": "2026-04-29T00:00:00Z",
                            "status": "running",
                            "strategy": {},
                            "createdAt": "2026-04-29T12:00:00.000000000Z",
                            "updatedAt": "2026-04-29T12:00:00.000000000Z",
                        },
                        "targets": [],
                    },
                    None,
                )
            },
        ),
        ("post", "/v1/admin/organizations/{organizationId}/rollouts/{rolloutId}/cancel"): ex(
            resp={
                "200": (
                    {
                        "campaign": {
                            "id": _U3,
                            "organizationId": _U2,
                            "rolloutType": "config_version",
                            "targetVersion": "2026-04-29T00:00:00Z",
                            "status": "cancelled",
                            "strategy": {},
                            "createdAt": "2026-04-29T12:00:00.000000000Z",
                            "updatedAt": "2026-04-29T12:00:00.000000000Z",
                            "cancelledAt": "2026-04-29T12:01:00.000000000Z",
                        },
                        "targets": [],
                    },
                    None,
                )
            },
        ),
        ("post", "/v1/admin/organizations/{organizationId}/rollouts/{rolloutId}/rollback"): ex(
            resp={
                "200": (
                    {
                        "campaign": {
                            "id": _U3,
                            "organizationId": _U2,
                            "rolloutType": "config_version",
                            "targetVersion": "2026-04-29T00:00:00Z",
                            "status": "rolled_back",
                            "strategy": {"rollback_version": "2026-04-28T00:00:00Z"},
                            "createdAt": "2026-04-29T12:00:00.000000000Z",
                            "updatedAt": "2026-04-29T12:00:00.000000000Z",
                        },
                        "targets": [],
                    },
                    None,
                )
            },
        ),
        ("get", "/v1/admin/audit/events"): ex(resp={"200": (audit_events_list_ex, None)}),
        ("get", "/v1/admin/organizations/{organizationId}/audit-events"): ex(resp={"200": (audit_events_list_ex, None)}),
        (
            "get",
            "/v1/admin/organizations/{organizationId}/audit-events/{auditEventId}",
        ): ex(resp={"200": (audit_event_row_ex, None)}),
        ("get", "/v1/admin/ops/outbox"): ex(resp={"200": (outbox_ops_list_ex, None)}),
        ("get", "/v1/admin/ops/retention"): ex(resp={"200": (retention_ops_ex, None)}),
        ("post", "/v1/admin/ops/outbox/{outboxId}/retry"): ex(resp={"200": (outbox_retry_ex, None)}),
        ("get", "/v1/admin/system/outbox/stats"): ex(resp={"200": (outbox_stats_only_ex, None)}),
        ("get", "/v1/admin/system/outbox"): ex(resp={"200": (outbox_ops_list_ex, None)}),
        ("get", "/v1/admin/system/outbox/{eventId}"): ex(resp={"200": (outbox_single_row_ex, None)}),
        ("post", "/v1/admin/system/outbox/{eventId}/replay"): ex(resp={"200": (outbox_retry_ex, None)}),
        (
            "post",
            "/v1/admin/system/outbox/{eventId}/mark-dlq",
        ): ex(
            req_body={"note": "Operator confirmed upstream outage before manual DLQ"},
            resp={"200": (outbox_mark_dlq_ex, None)},
        ),
        ("get", "/v1/admin/system/retention/stats"): ex(resp={"200": (system_retention_stats_ex, None)}),
        ("post", "/v1/admin/system/retention/dry-run"): ex(resp={"200": (system_retention_run_ex, None)}),
        ("post", "/v1/admin/system/retention/run"): ex(resp={"200": (system_retention_run_ex, None)}),
        ("get", "/v1/admin/feature-flags"): ex(resp={"200": ({"items": [ff_row], "meta": cmeta}, None)}),
        ("post", "/v1/admin/feature-flags"): ex(req_body=ff_create_req, resp={"201": (ff_row, None)}),
        ("get", "/v1/admin/feature-flags/{flagId}"): ex(resp={"200": (ff_detail_ex, None)}),
        ("patch", "/v1/admin/feature-flags/{flagId}"): ex(req_body=ff_patch_req, resp={"200": (ff_row, None)}),
        ("post", "/v1/admin/feature-flags/{flagId}/enable"): ex(resp={"200": (ff_row, None)}),
        ("post", "/v1/admin/feature-flags/{flagId}/disable"): ex(resp={"200": ({**ff_row, "enabled": False}, None)}),
        ("put", "/v1/admin/feature-flags/{flagId}/targets"): ex(req_body=ff_targets_req, resp={"200": ({"targets": []}, None)}),
        ("post", "/v1/admin/machine-config/rollouts"): ex(req_body=mcr_create_req, resp={"201": (mcr_row, None)}),
        ("get", "/v1/admin/machine-config/rollouts"): ex(resp={"200": ({"items": [mcr_row], "meta": cmeta}, None)}),
        ("get", "/v1/admin/machine-config/rollouts/{rolloutId}"): ex(resp={"200": (mcr_row, None)}),
        ("get", "/v1/admin/ota/campaigns"): ex(resp={"200": (ota_campaigns_list_ex, None)}),
        ("post", "/v1/admin/ota/campaigns"): ex(req_body=ota_create_req, resp={"201": (ota_campaign_detail_ex, None)}),
        ("get", "/v1/admin/ota/campaigns/{campaignId}"): ex(resp={"200": (ota_campaign_detail_ex, None)}),
        ("patch", "/v1/admin/ota/campaigns/{campaignId}"): ex(req_body=ota_patch_req, resp={"200": (ota_campaign_detail_ex, None)}),
        ("post", "/v1/admin/ota/campaigns/{campaignId}/approve"): ex(resp={"200": (ota_campaign_detail_ex, None)}),
        ("post", "/v1/admin/ota/campaigns/{campaignId}/start"): ex(resp={"200": (ota_campaign_detail_ex, None)}),
        ("post", "/v1/admin/ota/campaigns/{campaignId}/publish"): ex(resp={"200": (ota_campaign_detail_ex, None)}),
        ("post", "/v1/admin/ota/campaigns/{campaignId}/pause"): ex(resp={"200": (ota_campaign_detail_ex, None)}),
        ("post", "/v1/admin/ota/campaigns/{campaignId}/resume"): ex(resp={"200": (ota_campaign_detail_ex, None)}),
        ("post", "/v1/admin/ota/campaigns/{campaignId}/cancel"): ex(resp={"200": (ota_campaign_detail_ex, None)}),
        ("post", "/v1/admin/ota/campaigns/{campaignId}/rollback"): ex(req_body=ota_rollback_req, resp={"200": (ota_campaign_detail_ex, None)}),
        ("get", "/v1/admin/ota/campaigns/{campaignId}/targets"): ex(resp={"200": (ota_targets_ex, None)}),
        ("put", "/v1/admin/ota/campaigns/{campaignId}/targets"): ex(req_body=ota_targets_put_req, resp={"204": ("", None)}),
        ("get", "/v1/admin/ota/campaigns/{campaignId}/results"): ex(resp={"200": (ota_results_ex, None)}),
        ("get", "/v1/admin/ota"): ex(resp={"200": ({"items": [ota_item], "meta": cmeta}, None)}),
    }


def attach_examples(method: str, path: str, op: dict[str, Any]) -> None:
    bag = operation_examples().get((method, path))
    if not bag:
        return
    if bag.get("requestBodyExample") is not None and "requestBody" in op:
        rb = op["requestBody"]
        if "content" in rb and "application/json" in rb["content"]:
            rb["content"]["application/json"]["example"] = bag["requestBodyExample"]
    for code, pair in bag.get("responseExamples", {}).items():
        ex_obj = pair[0]
        if code not in op.get("responses", {}):
            continue
        resp = op["responses"][code]
        if "content" not in resp:
            continue
        for mime, block in resp["content"].items():
            if mime == "application/json" and ex_obj is not None:
                if isinstance(ex_obj, str):
                    block["example"] = ex_obj
                else:
                    block["example"] = ex_obj
            elif mime == "text/plain" and isinstance(ex_obj, str):
                block["example"] = ex_obj


def enrich_error_response_examples(paths: dict[str, dict[str, Any]]) -> None:
    """Attach representative JSON error examples to 4xx/5xx responses when still missing."""
    defaults: dict[str, tuple[str, str, dict[str, Any]]] = {
        "400": ("invalid_request", "request could not be validated", {}),
        "401": ("unauthenticated", "missing or invalid bearer token", {}),
        "403": ("forbidden", "caller lacks permission for this resource", {}),
        "404": ("not_found", "resource was not found", {}),
        "409": ("illegal_transition", "request conflicts with current state", {}),
        "429": ("rate_limited", "too many requests", {"retry_after_seconds": 60}),
        "500": ("internal", "unexpected server error", {}),
        "503": ("capability_not_configured", "optional capability is not configured for this process", {"capability": "example_capability"}),
    }
    for _path, by_method in paths.items():
        for _method, op in by_method.items():
            for code, resp in (op.get("responses") or {}).items():
                cs = str(code)
                if not cs.isdigit() or int(cs) < 400:
                    continue
                content = resp.get("content") or {}
                for mime, block in content.items():
                    if mime != "application/json":
                        continue
                    if block.get("example") is not None:
                        continue
                    d = defaults.get(cs)
                    if d:
                        block["example"] = v1_error_example(d[0], d[1], d[2])


# Every HTTP method/path the Chi router can register for the public API (see internal/httpserver/server.go).
REQUIRED_OPERATIONS: list[tuple[str, str]] = [
    ("get", "/health/live"),
    ("get", "/health/ready"),
    ("get", "/version"),
    ("get", "/metrics"),
    ("get", "/swagger/doc.json"),
    ("get", "/swagger/index.html"),
    ("post", "/v1/auth/login"),
    ("post", "/v1/auth/refresh"),
    ("get", "/v1/auth/me"),
    ("post", "/v1/auth/logout"),
    ("post", "/v1/auth/change-password"),
    ("post", "/v1/auth/password/change"),
    ("post", "/v1/auth/password/reset/request"),
    ("post", "/v1/auth/password/reset/confirm"),
    ("post", "/v1/auth/mfa/totp/enroll"),
    ("post", "/v1/auth/mfa/totp/verify"),
    ("post", "/v1/auth/mfa/totp/disable"),
    ("get", "/v1/auth/sessions"),
    ("delete", "/v1/auth/sessions"),
    ("delete", "/v1/auth/sessions/{sessionId}"),
    ("get", "/v1/admin/auth/users"),
    ("post", "/v1/admin/auth/users"),
    ("get", "/v1/admin/auth/users/{accountId}"),
    ("patch", "/v1/admin/auth/users/{accountId}"),
    ("post", "/v1/admin/auth/users/{accountId}/activate"),
    ("post", "/v1/admin/auth/users/{accountId}/deactivate"),
    ("post", "/v1/admin/auth/users/{accountId}/reset-password"),
    ("post", "/v1/admin/auth/users/{accountId}/revoke-sessions"),
    ("get", "/v1/admin/auth/users/{accountId}/sessions"),
    ("post", "/v1/admin/auth/users/{accountId}/roles"),
    ("put", "/v1/admin/auth/users/{accountId}/roles"),
    ("patch", "/v1/admin/auth/users/{accountId}/roles"),
    ("patch", "/v1/admin/auth/users/{accountId}/status"),
    ("get", "/v1/admin/users"),
    ("post", "/v1/admin/users"),
    ("get", "/v1/admin/users/{userId}"),
    ("patch", "/v1/admin/users/{userId}"),
    ("post", "/v1/admin/users/{userId}/roles"),
    ("put", "/v1/admin/users/{userId}/roles"),
    ("patch", "/v1/admin/users/{userId}/roles"),
    ("patch", "/v1/admin/users/{userId}/status"),
    ("post", "/v1/admin/users/{userId}/enable"),
    ("post", "/v1/admin/users/{userId}/disable"),
    ("post", "/v1/admin/users/{userId}/revoke-sessions"),
    ("get", "/v1/admin/users/{userId}/sessions"),
    ("post", "/v1/admin/users/{userId}/reset-password"),
    ("get", "/v1/admin/organizations/{organizationId}/users"),
    ("post", "/v1/admin/organizations/{organizationId}/users"),
    ("get", "/v1/admin/organizations/{organizationId}/users/{userId}"),
    ("patch", "/v1/admin/organizations/{organizationId}/users/{userId}"),
    ("post", "/v1/admin/organizations/{organizationId}/users/{userId}/enable"),
    ("post", "/v1/admin/organizations/{organizationId}/users/{userId}/disable"),
    ("post", "/v1/admin/organizations/{organizationId}/users/{userId}/roles"),
    ("patch", "/v1/admin/organizations/{organizationId}/users/{userId}/roles"),
    ("patch", "/v1/admin/organizations/{organizationId}/users/{userId}/status"),
    ("post", "/v1/admin/organizations/{organizationId}/users/{userId}/revoke-sessions"),
    ("get", "/v1/admin/organizations/{organizationId}/users/{userId}/sessions"),
    ("post", "/v1/admin/organizations/{organizationId}/users/{userId}/reset-password"),
    ("delete", "/v1/admin/organizations/{organizationId}/users/{userId}/roles/{role}"),
    ("get", "/v1/admin/organizations/{organizationId}/sites"),
    ("post", "/v1/admin/organizations/{organizationId}/sites"),
    ("get", "/v1/admin/organizations/{organizationId}/sites/{siteId}"),
    ("patch", "/v1/admin/organizations/{organizationId}/sites/{siteId}"),
    ("post", "/v1/admin/organizations/{organizationId}/sites/{siteId}/archive"),
    ("delete", "/v1/admin/organizations/{organizationId}/sites/{siteId}"),
    ("get", "/v1/admin/organizations/{organizationId}/machines"),
    ("post", "/v1/admin/organizations/{organizationId}/machines"),
    ("get", "/v1/admin/organizations/{organizationId}/machines/{machineId}"),
    ("patch", "/v1/admin/organizations/{organizationId}/machines/{machineId}"),
    ("post", "/v1/admin/organizations/{organizationId}/machines/{machineId}/archive"),
    ("post", "/v1/admin/organizations/{organizationId}/machines/{machineId}/suspend"),
    ("post", "/v1/admin/organizations/{organizationId}/machines/{machineId}/resume"),
    ("post", "/v1/admin/organizations/{organizationId}/machines/{machineId}/mark-compromised"),
    ("post", "/v1/admin/organizations/{organizationId}/machines/{machineId}/rotate-credentials"),
    ("post", "/v1/admin/organizations/{organizationId}/machines/{machineId}/revoke-credentials"),
    ("post", "/v1/admin/organizations/{organizationId}/machines/{machineId}/rotate-token-version"),
    ("post", "/v1/admin/organizations/{organizationId}/machines/{machineId}/revoke-token"),
    ("post", "/v1/admin/organizations/{organizationId}/machines/{machineId}/transfer-site"),
    ("get", "/v1/admin/organizations/{organizationId}/machines/{machineId}/technicians"),
    ("post", "/v1/admin/organizations/{organizationId}/machines/{machineId}/technicians"),
    ("delete", "/v1/admin/organizations/{organizationId}/machines/{machineId}/technicians/{userId}"),
    ("get", "/v1/admin/organizations/{organizationId}/technicians"),
    ("post", "/v1/admin/organizations/{organizationId}/technicians"),
    ("get", "/v1/admin/organizations/{organizationId}/technicians/{technicianId}"),
    ("patch", "/v1/admin/organizations/{organizationId}/technicians/{technicianId}"),
    ("post", "/v1/admin/organizations/{organizationId}/technicians/{technicianId}/disable"),
    ("post", "/v1/admin/organizations/{organizationId}/technicians/{technicianId}/enable"),
    ("get", "/v1/admin/organizations/{organizationId}/assignments"),
    ("post", "/v1/admin/organizations/{organizationId}/assignments"),
    ("get", "/v1/admin/organizations/{organizationId}/assignments/{assignmentId}"),
    ("delete", "/v1/admin/organizations/{organizationId}/assignments/{assignmentId}"),
    ("get", "/v1/admin/organizations/{organizationId}/activation-codes"),
    ("post", "/v1/admin/organizations/{organizationId}/activation-codes"),
    ("post", "/v1/admin/organizations/{organizationId}/activation-codes/{codeId}/revoke"),
    ("get", "/v1/admin/organizations/{organizationId}/operations/machines/health"),
    ("get", "/v1/admin/organizations/{organizationId}/machines/{machineId}/health"),
    ("get", "/v1/admin/organizations/{organizationId}/machines/{machineId}/timeline"),
    ("get", "/v1/admin/organizations/{organizationId}/commands"),
    ("get", "/v1/admin/organizations/{organizationId}/commands/{commandId}"),
    ("post", "/v1/admin/organizations/{organizationId}/commands/{commandId}/retry"),
    ("post", "/v1/admin/organizations/{organizationId}/commands/{commandId}/cancel"),
    ("post", "/v1/admin/organizations/{organizationId}/machines/{machineId}/commands"),
    ("get", "/v1/admin/organizations/{organizationId}/inventory/anomalies"),
    ("get", "/v1/admin/organizations/{organizationId}/machines/{machineId}/inventory/anomalies"),
    ("post", "/v1/admin/organizations/{organizationId}/inventory/anomalies/{anomalyId}/resolve"),
    ("get", "/v1/admin/organizations/{organizationId}/anomalies"),
    ("get", "/v1/admin/organizations/{organizationId}/anomalies/{anomalyId}"),
    ("post", "/v1/admin/organizations/{organizationId}/anomalies/{anomalyId}/resolve"),
    ("post", "/v1/admin/organizations/{organizationId}/anomalies/{anomalyId}/ignore"),
    ("get", "/v1/admin/organizations/{organizationId}/restock/suggestions"),
    ("post", "/v1/admin/organizations/{organizationId}/machines/{machineId}/inventory/reconcile"),
    ("post", "/v1/admin/organizations/{organizationId}/provisioning/machines/bulk"),
    ("get", "/v1/admin/organizations/{organizationId}/provisioning/batches/{batchId}"),
    ("post", "/v1/admin/organizations/{organizationId}/rollouts"),
    ("get", "/v1/admin/organizations/{organizationId}/rollouts"),
    ("get", "/v1/admin/organizations/{organizationId}/rollouts/{rolloutId}"),
    ("post", "/v1/admin/organizations/{organizationId}/rollouts/{rolloutId}/start"),
    ("post", "/v1/admin/organizations/{organizationId}/rollouts/{rolloutId}/pause"),
    ("post", "/v1/admin/organizations/{organizationId}/rollouts/{rolloutId}/resume"),
    ("post", "/v1/admin/organizations/{organizationId}/rollouts/{rolloutId}/cancel"),
    ("post", "/v1/admin/organizations/{organizationId}/rollouts/{rolloutId}/rollback"),
    ("get", "/v1/admin/audit/events"),
    ("get", "/v1/admin/organizations/{organizationId}/audit-events"),
    ("get", "/v1/admin/organizations/{organizationId}/audit-events/{auditEventId}"),
    ("get", "/v1/admin/ops/outbox"),
    ("get", "/v1/admin/ops/retention"),
    ("post", "/v1/admin/ops/outbox/{outboxId}/retry"),
    ("get", "/v1/admin/system/outbox/stats"),
    ("get", "/v1/admin/system/outbox"),
    ("get", "/v1/admin/system/outbox/{eventId}"),
    ("post", "/v1/admin/system/outbox/{eventId}/replay"),
    ("post", "/v1/admin/system/outbox/{eventId}/mark-dlq"),
    ("get", "/v1/admin/system/retention/stats"),
    ("post", "/v1/admin/system/retention/dry-run"),
    ("post", "/v1/admin/system/retention/run"),
    ("get", "/v1/admin/products"),
    ("get", "/v1/admin/products/{productId}"),
    ("post", "/v1/admin/products"),
    ("put", "/v1/admin/products/{productId}"),
    ("patch", "/v1/admin/products/{productId}"),
    ("delete", "/v1/admin/products/{productId}"),
    ("post", "/v1/admin/products/{productId}/image"),
    ("put", "/v1/admin/products/{productId}/image"),
    ("delete", "/v1/admin/products/{productId}/image"),
    ("post", "/v1/admin/media/assets"),
    ("post", "/v1/admin/media/uploads"),
    ("post", "/v1/admin/media/{mediaId}/complete"),
    ("post", "/v1/admin/organizations/{organizationId}/media/uploads/init"),
    ("post", "/v1/admin/organizations/{organizationId}/media/uploads/complete"),
    ("post", "/v1/admin/organizations/{organizationId}/media/product-images"),
    ("get", "/v1/admin/organizations/{organizationId}/media/assets"),
    ("get", "/v1/admin/organizations/{organizationId}/media/assets/{assetId}"),
    ("delete", "/v1/admin/organizations/{organizationId}/media/assets/{assetId}"),
    ("post", "/v1/admin/organizations/{organizationId}/products/{productId}/media"),
    ("delete", "/v1/admin/organizations/{organizationId}/products/{productId}/media/{mediaId}"),
    ("get", "/v1/admin/media/assets"),
    ("get", "/v1/admin/media/assets/{mediaId}"),
    ("get", "/v1/admin/media"),
    ("get", "/v1/admin/media/{mediaId}"),
    ("delete", "/v1/admin/media/assets/{mediaId}"),
    ("delete", "/v1/admin/media/{mediaId}"),
    ("post", "/v1/admin/products/{productId}/media"),
    ("put", "/v1/admin/products/{productId}/media"),
    ("delete", "/v1/admin/products/{productId}/media/{mediaId}"),
    ("post", "/v1/admin/organizations/{organizationId}/products/{productId}/images"),
    ("get", "/v1/admin/organizations/{organizationId}/products/{productId}/images"),
    ("patch", "/v1/admin/organizations/{organizationId}/products/{productId}/images/{imageId}"),
    ("delete", "/v1/admin/organizations/{organizationId}/products/{productId}/images/{imageId}"),
    ("get", "/v1/admin/brands"),
    ("post", "/v1/admin/brands"),
    ("put", "/v1/admin/brands/{brandId}"),
    ("patch", "/v1/admin/brands/{brandId}"),
    ("delete", "/v1/admin/brands/{brandId}"),
    ("get", "/v1/admin/categories"),
    ("post", "/v1/admin/categories"),
    ("put", "/v1/admin/categories/{categoryId}"),
    ("patch", "/v1/admin/categories/{categoryId}"),
    ("delete", "/v1/admin/categories/{categoryId}"),
    ("get", "/v1/admin/tags"),
    ("post", "/v1/admin/tags"),
    ("put", "/v1/admin/tags/{tagId}"),
    ("patch", "/v1/admin/tags/{tagId}"),
    ("delete", "/v1/admin/tags/{tagId}"),
    ("get", "/v1/admin/price-books"),
    ("get", "/v1/admin/price-books/{priceBookId}"),
    ("post", "/v1/admin/price-books"),
    ("patch", "/v1/admin/price-books/{priceBookId}"),
    ("post", "/v1/admin/price-books/{priceBookId}/deactivate"),
    ("post", "/v1/admin/price-books/{priceBookId}/activate"),
    ("post", "/v1/admin/price-books/{priceBookId}/archive"),
    ("get", "/v1/admin/price-books/{priceBookId}/items"),
    ("put", "/v1/admin/price-books/{priceBookId}/items"),
    ("patch", "/v1/admin/price-books/{priceBookId}/items/{productId}"),
    ("delete", "/v1/admin/price-books/{priceBookId}/items/{productId}"),
    ("post", "/v1/admin/price-books/{priceBookId}/assign-target"),
    ("delete", "/v1/admin/price-books/{priceBookId}/targets/{targetId}"),
    ("get", "/v1/admin/promotions"),
    ("post", "/v1/admin/promotions/preview"),
    ("get", "/v1/admin/promotions/{promotionId}"),
    ("post", "/v1/admin/promotions"),
    ("patch", "/v1/admin/promotions/{promotionId}"),
    ("post", "/v1/admin/promotions/{promotionId}/activate"),
    ("post", "/v1/admin/promotions/{promotionId}/pause"),
    ("post", "/v1/admin/promotions/{promotionId}/deactivate"),
    ("post", "/v1/admin/promotions/{promotionId}/archive"),
    ("post", "/v1/admin/promotions/{promotionId}/assign-target"),
    ("delete", "/v1/admin/promotions/{promotionId}/targets/{targetId}"),
    ("post", "/v1/admin/pricing/preview"),
    ("get", "/v1/admin/planograms"),
    ("get", "/v1/admin/planograms/{planogramId}"),
    ("get", "/v1/admin/machines/{machineId}/slots"),
    ("post", "/v1/admin/machines/{machineId}/stock-adjustments"),
    ("get", "/v1/admin/machines/{machineId}/inventory"),
    ("get", "/v1/admin/machines/{machineId}/inventory-events"),
    ("get", "/v1/admin/inventory/low-stock"),
    ("get", "/v1/admin/inventory/refill-suggestions"),
    ("get", "/v1/admin/machines/{machineId}/refill-suggestions"),
    ("get", "/v1/admin/feature-flags"),
    ("post", "/v1/admin/feature-flags"),
    ("get", "/v1/admin/feature-flags/{flagId}"),
    ("patch", "/v1/admin/feature-flags/{flagId}"),
    ("post", "/v1/admin/feature-flags/{flagId}/enable"),
    ("post", "/v1/admin/feature-flags/{flagId}/disable"),
    ("put", "/v1/admin/feature-flags/{flagId}/targets"),
    ("post", "/v1/admin/machine-config/rollouts"),
    ("get", "/v1/admin/machine-config/rollouts"),
    ("get", "/v1/admin/machine-config/rollouts/{rolloutId}"),
    ("get", "/v1/admin/machines/{machineId}/cashbox"),
    ("post", "/v1/admin/machines/{machineId}/cash-collections"),
    ("get", "/v1/admin/machines/{machineId}/cash-collections"),
    ("get", "/v1/admin/machines/{machineId}/cash-collections/{collectionId}"),
    ("post", "/v1/admin/machines/{machineId}/cash-collections/{collectionId}/close"),
    ("get", "/v1/setup/machines/{machineId}/bootstrap"),
    ("put", "/v1/admin/machines/{machineId}/topology"),
    ("put", "/v1/admin/machines/{machineId}/planograms/draft"),
    ("post", "/v1/admin/machines/{machineId}/planograms/publish"),
    ("post", "/v1/admin/machines/{machineId}/sync"),
    ("get", "/v1/reports/sales-summary"),
    ("get", "/v1/reports/payments-summary"),
    ("get", "/v1/reports/fleet-health"),
    ("get", "/v1/reports/inventory-exceptions"),
    ("get", "/v1/admin/organizations/{organizationId}/reports/sales"),
    ("get", "/v1/admin/organizations/{organizationId}/reports/payments"),
    ("get", "/v1/admin/organizations/{organizationId}/reports/refunds"),
    ("get", "/v1/admin/organizations/{organizationId}/reports/cash"),
    ("get", "/v1/admin/organizations/{organizationId}/reports/inventory-low-stock"),
    ("get", "/v1/admin/organizations/{organizationId}/reports/machine-health"),
    ("get", "/v1/admin/organizations/{organizationId}/reports/failed-vends"),
    ("get", "/v1/admin/organizations/{organizationId}/reports/reconciliation-queue"),
    ("get", "/v1/admin/organizations/{organizationId}/reports/vends"),
    ("get", "/v1/admin/organizations/{organizationId}/reports/inventory"),
    ("get", "/v1/admin/organizations/{organizationId}/reports/machines"),
    ("get", "/v1/admin/organizations/{organizationId}/reports/products"),
    ("get", "/v1/admin/organizations/{organizationId}/reports/reconciliation"),
    ("get", "/v1/admin/organizations/{organizationId}/reports/commands"),
    ("get", "/v1/admin/organizations/{organizationId}/reports/fills"),
    ("get", "/v1/admin/organizations/{organizationId}/reports/export"),
    ("get", "/v1/admin/reports/sales-summary/export.csv"),
    ("get", "/v1/admin/reports/payments-summary/export.csv"),
    ("get", "/v1/admin/reports/cash-collections/export.csv"),
    ("post", "/v1/admin/finance/daily-close"),
    ("get", "/v1/admin/finance/daily-close"),
    ("get", "/v1/admin/finance/daily-close/{closeId}"),
    ("get", "/v1/admin/machines"),
    ("get", "/v1/admin/machines/{machineId}"),
    ("post", "/v1/admin/machines"),
    ("patch", "/v1/admin/machines/{machineId}"),
    ("post", "/v1/admin/machines/{machineId}/disable"),
    ("post", "/v1/admin/machines/{machineId}/enable"),
    ("post", "/v1/admin/machines/{machineId}/retire"),
    ("post", "/v1/admin/machines/{machineId}/rotate-credential"),
    ("get", "/v1/admin/sites"),
    ("get", "/v1/admin/sites/{siteId}"),
    ("post", "/v1/admin/sites"),
    ("patch", "/v1/admin/sites/{siteId}"),
    ("post", "/v1/admin/sites/{siteId}/disable"),
    ("delete", "/v1/admin/sites/{siteId}"),
    ("get", "/v1/admin/technicians"),
    ("get", "/v1/admin/technicians/{technicianId}"),
    ("post", "/v1/admin/technicians"),
    ("patch", "/v1/admin/technicians/{technicianId}"),
    ("post", "/v1/admin/technicians/{technicianId}/disable"),
    ("post", "/v1/admin/technicians/{technicianId}/enable"),
    ("get", "/v1/admin/assignments"),
    ("get", "/v1/admin/technician-assignments"),
    ("post", "/v1/admin/technician-assignments"),
    ("get", "/v1/admin/technician-assignments/{assignmentId}"),
    ("patch", "/v1/admin/technician-assignments/{assignmentId}"),
    ("post", "/v1/admin/technician-assignments/{assignmentId}/cancel"),
    ("delete", "/v1/admin/technician-assignments/{assignmentId}"),
    ("get", "/v1/admin/commands"),
    ("get", "/v1/admin/machines/{machineId}/diagnostics/bundles"),
    ("post", "/v1/admin/machines/{machineId}/diagnostics/requests"),
    ("get", "/v1/admin/ota"),
    ("get", "/v1/admin/ota/campaigns"),
    ("post", "/v1/admin/ota/campaigns"),
    ("get", "/v1/admin/ota/campaigns/{campaignId}"),
    ("patch", "/v1/admin/ota/campaigns/{campaignId}"),
    ("post", "/v1/admin/ota/campaigns/{campaignId}/approve"),
    ("post", "/v1/admin/ota/campaigns/{campaignId}/start"),
    ("post", "/v1/admin/ota/campaigns/{campaignId}/publish"),
    ("post", "/v1/admin/ota/campaigns/{campaignId}/pause"),
    ("post", "/v1/admin/ota/campaigns/{campaignId}/resume"),
    ("post", "/v1/admin/ota/campaigns/{campaignId}/cancel"),
    ("post", "/v1/admin/ota/campaigns/{campaignId}/rollback"),
    ("get", "/v1/admin/ota/campaigns/{campaignId}/targets"),
    ("put", "/v1/admin/ota/campaigns/{campaignId}/targets"),
    ("get", "/v1/admin/ota/campaigns/{campaignId}/results"),
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
    ("get", "/v1/admin/organizations/{organizationId}/commerce/reconciliation"),
    ("get", "/v1/admin/organizations/{organizationId}/commerce/reconciliation/{id}"),
    ("post", "/v1/admin/organizations/{organizationId}/commerce/reconciliation/{id}/resolve"),
    ("post", "/v1/admin/organizations/{organizationId}/commerce/reconciliation/{id}/ignore"),
    ("get", "/v1/admin/organizations/{organizationId}/orders/{orderId}/timeline"),
    ("get", "/v1/admin/organizations/{organizationId}/refunds"),
    ("get", "/v1/admin/organizations/{organizationId}/refunds/{refundId}"),
    ("post", "/v1/admin/organizations/{organizationId}/orders/{orderId}/refunds"),
    ("get", "/v1/machines/{machineId}/shadow"),
    ("get", "/v1/machines/{machineId}/telemetry/snapshot"),
    ("get", "/v1/machines/{machineId}/telemetry/incidents"),
    ("get", "/v1/machines/{machineId}/telemetry/rollups"),
    ("get", "/v1/machines/{machineId}/commands/receipts"),
    ("get", "/v1/machines/{machineId}/commands/{sequence}/status"),
    ("post", "/v1/machines/{machineId}/commands/dispatch"),
    ("post", "/v1/machines/{machineId}/check-ins"),
    ("post", "/v1/machines/{machineId}/config-applies"),
    ("get", "/v1/machines/{machineId}/operator-sessions/current"),
    ("get", "/v1/machines/{machineId}/operator-sessions/history"),
    ("get", "/v1/machines/{machineId}/operator-sessions/auth-events"),
    ("get", "/v1/machines/{machineId}/operator-sessions/action-attributions"),
    ("get", "/v1/machines/{machineId}/operator-sessions/timeline"),
    ("post", "/v1/machines/{machineId}/operator-sessions/login"),
    ("post", "/v1/machines/{machineId}/operator-sessions/logout"),
    ("post", "/v1/machines/{machineId}/operator-sessions/{sessionId}/heartbeat"),
    ("post", "/v1/commerce/cash-checkout"),
    ("post", "/v1/commerce/orders"),
    ("post", "/v1/commerce/orders/{orderId}/payment-session"),
    ("get", "/v1/commerce/orders/{orderId}"),
    ("get", "/v1/commerce/orders/{orderId}/reconciliation"),
    ("post", "/v1/commerce/orders/{orderId}/payments/{paymentId}/webhooks"),
    ("post", "/v1/commerce/orders/{orderId}/cancel"),
    ("post", "/v1/commerce/orders/{orderId}/refunds"),
    ("get", "/v1/commerce/orders/{orderId}/refunds"),
    ("get", "/v1/commerce/orders/{orderId}/refunds/{refundId}"),
    ("post", "/v1/commerce/orders/{orderId}/vend/start"),
    ("post", "/v1/commerce/orders/{orderId}/vend/success"),
    ("post", "/v1/commerce/orders/{orderId}/vend/failure"),
    ("post", "/v1/setup/activation-codes/claim"),
    ("post", "/v1/admin/machines/{machineId}/activation-codes"),
    ("get", "/v1/admin/machines/{machineId}/activation-codes"),
    ("delete", "/v1/admin/machines/{machineId}/activation-codes/{activationCodeId}"),
    ("get", "/v1/machines/{machineId}/sale-catalog"),
    ("post", "/v1/device/machines/{machineId}/events/reconcile"),
    ("get", "/v1/device/machines/{machineId}/events/{idempotencyKey}/status"),
    ("post", "/v1/device/machines/{machineId}/vend-results"),
    ("post", "/v1/device/machines/{machineId}/commands/poll"),
]


def verify_paths(paths: dict[str, dict[str, Any]]) -> list[str]:
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

    paths: dict[str, dict[str, Any]] = {}
    for name, block in extract_doc_blocks(OPS_GO.read_text(encoding="utf-8")):
        d = parse_op_directives(block)
        built = build_operation_oas3(d)
        if not built:
            continue
        path, method, op = built
        op["operationId"] = name
        merge_global_parameters(path, op)
        merge_idempotency_parameter(method, path, op)
        attach_examples(method, path, op)
        paths.setdefault(path, {})[method] = op

    enrich_error_response_examples(paths)
    mark_deprecated_machine_legacy_rest(paths)

    miss = verify_paths(paths)
    if miss:
        print("swagger route coverage: missing operations:", file=sys.stderr)
        for m in miss:
            print(" ", m, file=sys.stderr)
        return 1

    comp = components()
    spec: dict[str, Any] = {
        "openapi": "3.0.3",
        "info": info,
        "servers": [
            {
                "url": "https://api.ldtv.dev",
                "description": "Production (default — Swagger UI Try it out uses this server first)",
            },
            {"url": "http://localhost:8080", "description": "Local development"},
        ],
        "paths": paths,
        "components": comp,
        "tags": [
            {"name": "Health", "description": "Process liveness/readiness; no JWT. Readiness may return plain text 503 when dependencies fail."},
            {
                "name": "System",
                "description": "Build/version JSON, Prometheus metrics when `METRICS_ENABLED=true`, OpenAPI at `/swagger/doc.json` when `HTTP_OPENAPI_JSON_ENABLED=true`, and Swagger UI when `HTTP_SWAGGER_UI_ENABLED=true` (independent; production often enables JSON only).",
            },
            {
                "name": "Auth",
                "description": "Session-based API authentication (login/refresh without Bearer; me/logout on the Bearer-protected `/v1/auth` group).",
            },
            {
                "name": "Auth Admin",
                "description": "Tenant-scoped API account lifecycle (`platform_auth_accounts`) for **platform_admin** and **org_admin** (`/v1/admin/auth/users`).",
            },
            {
                "name": "Audit Admin",
                "description": "Enterprise append-only audit trail (`audit_events`) for interactive principals with **audit.read** (`GET /v1/admin/audit/events`, `GET /v1/admin/organizations/{organizationId}/audit-events`, `GET .../audit-events/{auditEventId}`).",
            },
            {
                "name": "Activation",
                "description": "Public activation-code claim (`POST /v1/setup/activation-codes/claim`) and org-admin provisioning under `/v1/admin/machines/.../activation-codes`.",
            },
            {
                "name": "Catalog Admin",
                "description": "Read-only org catalog: products, price books, planograms (`platform_admin` or `org_admin`).",
            },
            {
                "name": "Inventory",
                "description": "Machine slot state, aggregate inventory, ledger events, and idempotent stock adjustments (operator session + Idempotency-Key).",
            },
            {
                "name": "Cash settlement",
                "description": "Field cash collection sessions: expected vault from commerce cash payments, open/close with variance and audit (accounting-only; no bill recycler hardware control).",
            },
            {
                "name": "Machine Setup",
                "description": "Technician bootstrap payload, cabinet topology, planogram draft/publish, and setup sync commands.",
            },
            {
                "name": "Runtime Catalog",
                "description": "Kiosk `GET /v1/machines/{machineId}/sale-catalog` — published planogram, price, stock, optional product images.",
            },
            {
                "name": "Machine Admin",
                "description": "Fleet directory: machines, technicians, assignments, command ledger, OTA campaigns.",
            },
            {"name": "Artifacts", "description": "Presigned S3 artifact lifecycle when `API_ARTIFACTS_ENABLED=true` and object storage is configured."},
            {
                "name": "Operator Sessions",
                "description": "Machine-scoped operator login/logout/heartbeat/history and cross-machine insight reads.",
            },
            {
                "name": "Commerce",
                "description": "Checkout (cash + online), payment-session outbox, provider webhooks (HMAC, no Bearer), vend lifecycle, and tenant orders/payments lists.",
            },
            {
                "name": "Reporting",
                "description": "Read-only analytics (`platform_admin` or `org_admin`). Routes require RFC3339 **from**/**to** (max 366 days where applicable).",
            },
            {"name": "Telemetry", "description": "Projected machine telemetry snapshot, incidents, and rollups (not raw MQTT)."},
            {
                "name": "Telemetry Reconcile",
                "description": "Device-facing critical telemetry idempotency batch/status (`/v1/device/machines/{machineId}/events/...`).",
            },
            {
                "name": "Device Runtime",
                "description": "Shadow document, remote commands (dispatch, poll, receipts), Android check-ins, config acknowledgements, and HTTP vend-result bridge.",
            },
        ],
    }

    unresolved = unresolved_local_refs(spec)
    if unresolved:
        print("openapi refs: unresolved local $ref values:", file=sys.stderr)
        for ref in unresolved:
            print(" ", ref, file=sys.stderr)
        return 1

    OUT_DIR.mkdir(parents=True, exist_ok=True)
    OUT_JSON.write_text(json.dumps(spec, indent=2, sort_keys=True) + "\n", encoding="utf-8", newline="\n")

    data = json.loads(OUT_JSON.read_text(encoding="utf-8"))
    servers = data.get("servers") or []
    if (
        len(servers) < 2
        or servers[0].get("url") != "https://api.ldtv.dev"
        or servers[1].get("url") != "http://localhost:8080"
    ):
        print(
            "openapi servers: expected servers[0].url=https://api.ldtv.dev and "
            "servers[1].url=http://localhost:8080 (Production first, Development second)",
            file=sys.stderr,
        )
        return 1
    title = data["info"]["title"].replace("\\", "\\\\").replace('"', '\\"')
    ver = data["info"]["version"]
    docs_go = f'''// Package swagger contains the OpenAPI 3.0 document for the HTTP API (generated).
//
// Code generated by tools/build_openapi.py; DO NOT EDIT manually.
//
//go:generate python3 tools/build_openapi.py
package swagger

import (
	_ "embed"

	"github.com/swaggo/swag"
)

//go:embed swagger.json
var swaggerJSON []byte

// OpenAPIJSON returns the embedded OpenAPI 3.0 document (for tests and offline tooling).
func OpenAPIJSON() []byte {{
	return swaggerJSON
}}

func init() {{
	swag.Register("swagger", &swag.Spec{{
		Version:          "{ver}",
		Host:             "",
		BasePath:         "/",
		Schemes:          []string{{"http", "https"}},
		Title:            "{title}",
		Description:      "OpenAPI 3.0 embedded as swagger.json (generated by tools/build_openapi.py).",
		InfoInstanceName: "swagger",
		SwaggerTemplate:  string(swaggerJSON),
	}})
}}
'''
    OUT_DOCS_GO.write_text(docs_go, encoding="utf-8", newline="\n")
    print("wrote", OUT_JSON, "and", OUT_DOCS_GO)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

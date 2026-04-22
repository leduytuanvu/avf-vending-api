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


def components() -> dict[str, Any]:
    err = v1_api_error_schema()
    schemas: dict[str, Any] = {
        "V1APIErrorEnvelope": err,
        "V1StandardError": err,
        "V1NotImplementedError": err,
        "V1CapabilityNotConfiguredError": err,
        "V1BearerAuthError": err,
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
    schemas.update(operational_collection_component_schemas())
    schemas.update(machine_setup_component_schemas())
    schemas.update(reporting_component_schemas())
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


IDEMPOTENCY_OPS: set[tuple[str, str]] = {
    ("post", "/v1/commerce/orders"),
    ("post", "/v1/commerce/cash-checkout"),
    ("post", "/v1/commerce/orders/{orderId}/payment-session"),
    ("post", "/v1/commerce/orders/{orderId}/payments/{paymentId}/webhooks"),
    ("post", "/v1/commerce/orders/{orderId}/vend/start"),
    ("post", "/v1/commerce/orders/{orderId}/vend/success"),
    ("post", "/v1/commerce/orders/{orderId}/vend/failure"),
    ("post", "/v1/device/machines/{machineId}/vend-results"),
    ("post", "/v1/machines/{machineId}/commands/dispatch"),
    ("post", "/v1/admin/machines/{machineId}/planograms/publish"),
    ("post", "/v1/admin/machines/{machineId}/sync"),
    ("post", "/v1/admin/machines/{machineId}/stock-adjustments"),
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

    return {
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
            resp={"200": ({"order_id": _U, "order_status": "failed", "vend_state": "failed"}, None)},
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
                                "idempotency_key": "planogram-publish-001",
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
        ("post", "/v1/machines/{machineId}/operator-sessions/login"): ex(
            req_body={"auth_method": "oidc", "client_metadata": {"kiosk": "A12"}},
            resp={"200": (op_login, None)},
        ),
        ("post", "/v1/admin/organizations/{orgId}/artifacts"): ex(
            req_body={"content_type": "application/zip", "original_filename": "bundle.zip"},
            resp={"201": (art_reserve, None)},
        ),
        ("get", "/v1/orders"): ex(resp={"200": ({"items": [ord_item], "meta": cmeta}, None)}),
        ("get", "/v1/payments"): ex(resp={"200": ({"items": [pay_item], "meta": cmeta}, None)}),
        ("get", "/v1/admin/machines"): ex(resp={"200": ({"items": [mach_item], "meta": cmeta}, None)}),
        ("get", "/v1/admin/technicians"): ex(resp={"200": ({"items": [tech_item], "meta": cmeta}, None)}),
        ("get", "/v1/admin/assignments"): ex(resp={"200": ({"items": [asg_item], "meta": cmeta}, None)}),
        ("get", "/v1/admin/commands"): ex(resp={"200": ({"items": [cmd_item], "meta": cmeta}, None)}),
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
                block["example"] = ex_obj
                break


# Every HTTP method/path the Chi router can register for the public API (see internal/httpserver/server.go).
REQUIRED_OPERATIONS: list[tuple[str, str]] = [
    ("get", "/health/live"),
    ("get", "/health/ready"),
    ("get", "/metrics"),
    ("get", "/swagger/doc.json"),
    ("get", "/swagger/index.html"),
    ("post", "/v1/auth/login"),
    ("post", "/v1/auth/refresh"),
    ("get", "/v1/auth/me"),
    ("post", "/v1/auth/logout"),
    ("get", "/v1/admin/products"),
    ("get", "/v1/admin/products/{productId}"),
    ("get", "/v1/admin/price-books"),
    ("get", "/v1/admin/planograms"),
    ("get", "/v1/admin/planograms/{planogramId}"),
    ("get", "/v1/admin/machines/{machineId}/slots"),
    ("post", "/v1/admin/machines/{machineId}/stock-adjustments"),
    ("get", "/v1/admin/machines/{machineId}/inventory"),
    ("get", "/v1/admin/machines/{machineId}/inventory-events"),
    ("get", "/v1/setup/machines/{machineId}/bootstrap"),
    ("put", "/v1/admin/machines/{machineId}/topology"),
    ("put", "/v1/admin/machines/{machineId}/planograms/draft"),
    ("post", "/v1/admin/machines/{machineId}/planograms/publish"),
    ("post", "/v1/admin/machines/{machineId}/sync"),
    ("get", "/v1/reports/sales-summary"),
    ("get", "/v1/reports/payments-summary"),
    ("get", "/v1/reports/fleet-health"),
    ("get", "/v1/reports/inventory-exceptions"),
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
    ("post", "/v1/commerce/orders/{orderId}/vend/start"),
    ("post", "/v1/commerce/orders/{orderId}/vend/success"),
    ("post", "/v1/commerce/orders/{orderId}/vend/failure"),
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
    for _name, block in extract_doc_blocks(OPS_GO.read_text(encoding="utf-8")):
        d = parse_op_directives(block)
        built = build_operation_oas3(d)
        if not built:
            continue
        path, method, op = built
        merge_global_parameters(path, op)
        merge_idempotency_parameter(method, path, op)
        attach_examples(method, path, op)
        paths.setdefault(path, {})[method] = op

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
            {"url": "http://localhost:8080", "description": "Development"},
            {"url": "https://api.ldtv.dev", "description": "Production"},
        ],
        "paths": paths,
        "components": comp,
        "tags": [
            {"name": "Health", "description": "Process liveness/readiness; no JWT. Readiness may return plain text 503 when dependencies fail."},
            {"name": "Reliability", "description": "Prometheus metrics when `METRICS_ENABLED=true`."},
            {
                "name": "Auth",
                "description": "Session-based API authentication (login/refresh without Bearer; me/logout on the Bearer-protected `/v1/auth` group).",
            },
            {"name": "Admin", "description": "Fleet and org administration (`platform_admin` or `org_admin`). Operational list routes are Postgres-backed with pagination and typed `items` + `meta` envelopes."},
            {"name": "Artifacts", "description": "Presigned S3 artifact lifecycle when `API_ARTIFACTS_ENABLED=true` and object storage is configured."},
            {"name": "Operator", "description": "Technician/user operator sessions, attribution, and cross-machine insights."},
            {"name": "Commerce", "description": "Tenant-scoped checkout (order, payment session + outbox, provider webhooks, vend state machine) plus read-only operational lists for orders and payments."},
            {
                "name": "Reporting",
                "description": "Read-only analytics for finance and operations (`platform_admin` or `org_admin`). All routes require explicit RFC3339 **from**/**to** bounds (max 366 days) and organization scoping consistent with admin lists.",
            },
            {"name": "Fleet", "description": "Machine shadow projection and telemetry rollups (read models, not raw MQTT firehose)."},
            {"name": "Device", "description": "Remote command ledger + MQTT dispatch; **503** when MQTT publisher is not configured."},
            {"name": "Documentation", "description": "Embedded Swagger UI + OpenAPI JSON when `HTTP_SWAGGER_UI_ENABLED=true`."},
        ],
    }

    OUT_DIR.mkdir(parents=True, exist_ok=True)
    OUT_JSON.write_text(json.dumps(spec, indent=2, sort_keys=True) + "\n", encoding="utf-8", newline="\n")

    data = json.loads(OUT_JSON.read_text(encoding="utf-8"))
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

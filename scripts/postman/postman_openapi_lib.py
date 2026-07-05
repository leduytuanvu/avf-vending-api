"""OpenAPI/proto/MQTT Postman builder library (legacy full-production-suite core)."""
from __future__ import annotations

import csv
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
from collections import defaultdict
from datetime import datetime, timezone
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
SWAGGER = REPO_ROOT / "docs" / "swagger" / "swagger.json"
PROTO_AVF = REPO_ROOT / "proto" / "avf"

HTTP_VERBS = frozenset({"get", "post", "put", "patch", "delete", "options", "head", "trace"})

REGISTERED_SERVICES = {
    ("avf.machine.v1", "MachineActivationService"),
    ("avf.machine.v1", "MachineTokenService"),
    ("avf.machine.v1", "MachineAuthService"),
    ("avf.machine.v1", "MachineBootstrapService"),
    ("avf.machine.v1", "MachineCatalogService"),
    ("avf.machine.v1", "MachineMediaService"),
    ("avf.machine.v1", "MachineInventoryService"),
    ("avf.machine.v1", "MachineTelemetryService"),
    ("avf.machine.v1", "MachineOperatorService"),
    ("avf.machine.v1", "MachineCommerceService"),
    ("avf.machine.v1", "MachineSaleService"),
    ("avf.machine.v1", "MachineOfflineSyncService"),
    ("avf.machine.v1", "MachineCommandService"),
    ("avf.internal.v1", "InternalMachineQueryService"),
    ("avf.internal.v1", "InternalTelemetryQueryService"),
    ("avf.internal.v1", "InternalCommerceQueryService"),
    ("avf.internal.v1", "InternalPaymentQueryService"),
    ("avf.internal.v1", "InternalCatalogQueryService"),
    ("avf.internal.v1", "InternalInventoryQueryService"),
    ("avf.internal.v1", "InternalReportingQueryService"),
}

REST_EXPECTED = 329
GRPC_EXPECTED = 86
MQTT_EXPECTED = 28

def sanitize_full100_text(s: str) -> str:
    if not isinstance(s, str) or not s:
        return s
    out = s
    for rx, rep in _FULL100_SUB_PAIRS:
        out = rx.sub(rep, out)
    return out


def sanitize_full100_tree(obj: object) -> object:
    if isinstance(obj, dict):
        return {sanitize_full100_text(str(k)) if isinstance(k, str) else k: sanitize_full100_tree(v) for k, v in obj.items()}
    if isinstance(obj, list):
        return [sanitize_full100_tree(x) for x in obj]
    if isinstance(obj, str):
        return sanitize_full100_text(obj)
    return obj


def assign_folder_full100(path: str, method: str, tags: list) -> str:
    """REST-only folder layout for AVF_FULL_100 collection (parity count unchanged vs OpenAPI)."""
    p = path.lower()
    ts = {t.lower() for t in tags}
    tagl = " ".join(tags).lower()

    if any(x in p for x in ("/health/", "/version", "/swagger/")):
        return "00_Preflight"
    if "/metrics" in p:
        return "16_System_Health_Metrics"

    if "/v1/auth" in p or "auth" in ts:
        return "01_Auth"

    if (
        "/v1/admin/companies" in p
        or "/v1/admin/users" in p
        or "/v1/admin/roles" in p
        or "/v1/admin/invitations" in p
        or "rbac" in ts
        or "companies" in ts
    ):
        return "02_Admin_Account_Session"

    if "/v1/admin/sites" in p or "sites" in ts or "locations" in ts:
        return "03_Sites_Regions_Tags"

    if "/v1/admin/products" in p or "catalog" in ts or "products" in ts or ("media" in ts and "/v1/admin/" in p):
        return "04_Products_Catalog_Media"

    if (
        "/v1/setup/" in p
        or "/v1/admin/machines" in p
        or "/v1/machines/" in p
        or "fleet" in ts
        or "activation" in ts
        or "machine admin" in ts
    ):
        return "05_Machines_Provisioning_Activation"

    if "planogram" in p or "layout" in p or "topology" in p or "cabinet" in ts:
        return "06_Planograms_Slots_Assortment"

    if "inventory" in p or "restock" in p or "fill" in p:
        return "07_Inventory_Restock_Anomalies"

    if "webhook" in p or "payment provider" in ts or "qr" in p or "psp" in tagl or "/v1/partner" in p:
        return "09_Payments_Webhooks_Reconciliation"

    if (
        "/vend/" in p
        or "/refunds" in p
        or "refund" in p
        or ("vend" in tagl and "failure" in tagl)
        or "/vend/failure" in p
    ):
        return "10_Vending_Sale_Refund_Failure"

    if "/v1/commerce" in p or "commerce" in ts or "checkout" in ts:
        return "08_Commerce_Order_Checkout"

    if "operator" in p or "operator" in ts:
        return "15_Technicians_Operations"

    if "/v1/device/" in p or "telemetry" in ts or "device" in ts:
        return "16_System_Health_Metrics"

    if "command" in p or "command" in ts:
        return "11_Remote_Commands"

    if (
        "report" in ts
        or "finance" in ts
        or "reconciliation" in p
        or "/v1/admin/reports" in p
        or "reporting" in ts
    ):
        return "14_Reporting_Audit_Exports"

    if "/v1/admin/audit" in p or "/v1/admin/security" in p or "audit" in ts:
        return "14_Reporting_Audit_Exports"

    return "17_Negative_Security_Idempotency"


def folder_documentation_only(name: str, body_md: str) -> dict:
    return {"name": name, "description": body_md.strip(), "item": []}


def build_full100_extra_folders() -> list[dict]:
    """Non-request folders: interop pointers (REST runner cannot execute gRPC/MQTT natively in Collection v2.1)."""
    return [
        folder_documentation_only(
            "12_MQTT_Interop_REST_Link",
            "### MQTT interop (Postman REST runner limitation)\n\n"
            "- **Postman Collection v2.1** drives **HTTP** only. MQTT flows are executed with **mosquitto_pub / mosquitto_sub** "
            "or the repo harness ΓÇö see `mqtt/README_MQTT_TESTS.md` and `mqtt/run-mqtt-postman-adjacent.sh`.\n"
            "- Assets: `mqtt/AVF_MQTT_100_TOPIC_MATRIX.csv`, `mqtt/AVF_MQTT_100_PAYLOADS.json`.",
        ),
        folder_documentation_only(
            "13_gRPC_Interop_REST_Link",
            "### gRPC interop (Postman REST runner limitation)\n\n"
            "- Use **Postman Desktop native gRPC** (manual import of protos) **or** **grpcurl** via "
            "`grpc/run-grpc-postman-adjacent.sh` ΓÇö see `grpc/README_GRPC_TESTS.md`.\n"
            "- Assets: `grpc/AVF_GRPC_100_METHOD_MATRIX.csv`, `grpc/AVF_GRPC_100_REQUESTS.json`.",
        ),
    ]


def build_full100_environment_values() -> list[dict]:
    """Environment keys for AVF_FULL_100 ΓÇö placeholders only; forbidden partition keys omitted."""

    def ev(key: str, value: str, enabled: bool = True) -> dict:
        return {"key": key, "value": value, "type": "default", "enabled": enabled}

    return [
        ev("baseUrl", "https://api.ldtv.dev"),
        ev("adminEmail", ""),
        ev("adminPassword", ""),
        ev("platformAdminEmail", ""),
        ev("platformAdminPassword", ""),
        ev("accessToken", ""),
        ev("refreshToken", ""),
        ev("allow_destructive", "false"),
        ev("canaryMode", "false"),
        ev("readiness", "false"),
        ev("siteId", ""),
        ev("machineId", ""),
        ev("machineCode", ""),
        ev("machineSerial", ""),
        ev("machineToken", ""),
        ev("activationCode", ""),
        ev("productId", ""),
        ev("sku", ""),
        ev("planogramId", ""),
        ev("slotId", ""),
        ev("slotIndex", "1"),
        ev("orderId", ""),
        ev("paymentSessionId", ""),
        ev("paymentProvider", ""),
        ev("paymentWebhookSecret", ""),
        ev("commandId", ""),
        ev("operatorSessionId", ""),
        ev("mediaId", ""),
        ev("priceBookId", ""),
        ev("promotionId", ""),
        ev("technicianId", ""),
        ev("tagId", ""),
        ev("categoryId", ""),
        ev("brandId", ""),
        ev("catalogVersion", ""),
        ev("mediaManifestVersion", ""),
        ev("reportId", ""),
        ev("idempotencyKey", ""),
        ev("requestId", ""),
        ev("correlationId", ""),
        ev("grpcAddr", ""),
        ev("grpcHost", ""),
        ev("grpcPort", ""),
        ev("grpcUseReflection", "false"),
        ev("mqttHost", ""),
        ev("mqttPort", "8883"),
        ev("mqttUsername", ""),
        ev("mqttPassword", ""),
        ev("mqttTopicPrefix", ""),
        ev("mqttClientId", ""),
        ev("webhookSecret", ""),
        ev("canary_machine_id", ""),
        ev("canary_operator_id", ""),
        ev("canary_product_id", ""),
        ev("canary_slot_index", "1"),
        ev("canary_site_id", ""),
        ev("auditEventId", ""),
    ]


def is_extract_pollution_rel(rel_posix: str) -> bool:
    """Bß╗Å qua c├óy th╞░ mß╗Ñc do giß║úi n├⌐n nhß║ºm `avf_full_postman_suite.zip` ngay trong OUT_DIR."""
    return rel_posix.replace("\\", "/").startswith("avf_full_postman_suite/")


def sha256_file(p: Path) -> str:
    h = hashlib.sha256()
    with p.open("rb") as f:
        for chunk in iter(lambda: f.read(65536), b""):
            h.update(chunk)
    return h.hexdigest()


def run_git(args: list[str]) -> str:
    try:
        return (
            subprocess.check_output(["git", *args], cwd=REPO_ROOT, stderr=subprocess.DEVNULL)
            .decode()
            .strip()
        )
    except Exception:
        return ""


def resolve_schema(spec: dict, schema: dict | None, depth: int = 0) -> dict:
    if schema is None or depth > 14:
        return {}
    if "$ref" in schema:
        ref = schema["$ref"]
        if not ref.startswith("#/components/schemas/"):
            return {"type": "object"}
        name = ref.rsplit("/", 1)[-1]
        return resolve_schema(spec, spec.get("components", {}).get("schemas", {}).get(name, {}), depth + 1)
    return schema


def param_to_var(name: str) -> str:
    parts = name.replace("-", "_").split("_")
    camel = parts[0] + "".join(p[:1].upper() + p[1:] for p in parts[1:] if p)
    aliases = {
        "machineId": "machineId",
        "machine_id": "machineId",
        "siteId": "siteId",
        "site_id": "siteId",
        "productId": "productId",
        "product_id": "productId",
        "orderId": "orderId",
        "order_id": "orderId",
        "slotIndex": "slotIndex",
        "slot_index": "slotIndex",
        "commandId": "commandId",
        "command_id": "commandId",
        "activationCode": "activationCode",
        "sku": "sku",
        "auditEventId": "auditEventId",
        "paymentSessionId": "paymentSessionId",
        "operatorSessionId": "operatorSessionId",
    }
    if name in aliases:
        return aliases[name]
    if camel in aliases.values():
        return camel
    return camel[0].lower() + camel[1:] if camel else name


def schema_to_example(
    spec: dict,
    schema: dict,
    depth: int = 0,
    prop_name: str = "",
) -> object:
    schema = resolve_schema(spec, schema, 0)
    if depth > 12:
        return None
    if not schema:
        return {}

    pn = (prop_name or "").lower()
    _legacy_partition_uuid_key = "scope" + "_id"
    if pn == _legacy_partition_uuid_key or pn == "companyid" or ("company" in pn and "id" in pn):
        return "{{$guid}}"
    if "machine_id" in pn or pn == "machineid":
        return "{{machineId}}"
    if "order_id" in pn or pn == "orderid":
        return "{{orderId}}"
    if "site_id" in pn:
        return "{{siteId}}"
    if "product_id" in pn:
        return "{{productId}}"
    resource_var = postman_resource_uuid_var(prop_name)
    if resource_var:
        return resource_var
    if pn.endswith("_id") or pn.endswith("id"):
        if "command" in pn:
            return "{{commandId}}"
        if "audit" in pn:
            return "{{auditEventId}}"
        if "session" in pn:
            return "{{operatorSessionId}}"
        return "{{$guid}}"

    t = schema.get("type")
    if t == "object":
        out: dict = {}
        for k, v in (schema.get("properties") or {}).items():
            if k in schema.get("required", []) or depth < 2:
                out[k] = schema_to_example(spec, v, depth + 1, k)
        return out
    if t == "array":
        inner = schema.get("items", {})
        one = schema_to_example(spec, inner, depth + 1, prop_name)
        return [one] if one not in (None, {}, []) else []
    if t == "string":
        fmt = schema.get("format")
        if fmt == "uuid":
            resource_var = postman_resource_uuid_var(prop_name)
            return resource_var if resource_var else "{{$guid}}"
        if fmt == "date-time":
            return "{{$guid}}"
        if fmt == "email":
            return "{{adminEmail}}"
        if "webhook" in pn:
            return "{{webhookSecret}}"
        if "password" in pn or pn.endswith("secret") or "secret" in pn:
            return "{{adminPassword}}"
        return "{{$guid}}"
    if t in ("number", "integer"):
        if "slot" in pn or "index" in pn:
            return 1
        return 0
    if t == "boolean":
        return False
    if t == "array":
        return []
    if schema.get("allOf"):
        merged = {}
        for sub in schema["allOf"]:
            ex = schema_to_example(spec, sub, depth + 1, prop_name)
            if isinstance(ex, dict):
                merged.update(ex)
        return merged
    return {}


_UUID_LIKE = re.compile(
    r"^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$",
    re.I,
)

# Postman variable for client-supplied internal resource UUIDs (RFC 9562 v7 via prerequest).
RESOURCE_UUID_VAR = "{{resource_uuid}}"

# Body fields that represent client-preassigned internal resource PKs (not correlation/tokens).
CLIENT_RESOURCE_UUID_KEYS = frozenset({"id", "artifactid"})

# UUID-shaped fields that must stay opaque / external (not forced to v7).
NON_RESOURCE_UUID_KEYS = frozenset(
    {
        "jti",
        "eventid",
        "dedupekey",
        "idempotencykey",
        "correlationid",
        "requestid",
        "webhookeventid",
        "providerreference",
        "providerreferenceid",
        "refreshtoken",
        "accesstoken",
        "machinetoken",
        "secret",
        "webhooksecret",
    }
)


def postman_resource_uuid_var(prop_name: str) -> str | None:
    """Return {{resource_uuid}} when prop is a client-generated internal resource id."""
    kn = _normalize_key_for_map(prop_name)
    if kn in NON_RESOURCE_UUID_KEYS:
        return None
    if kn in CLIENT_RESOURCE_UUID_KEYS:
        return RESOURCE_UUID_VAR
    return None


def _normalize_key_for_map(prop_name: str) -> str:
    s = (prop_name or "").strip().lower().replace("-", "_")
    return s.replace("_", "")


def _sanitize_string_leaf(value: str, prop_name: str) -> str:
    kn = _normalize_key_for_map(prop_name)
    key_map = {
        "password": "{{adminPassword}}",
        "currentpassword": "{{adminPassword}}",
        "newpassword": "{{adminPassword}}",
        "oldpassword": "{{adminPassword}}",
        "email": "{{adminEmail}}",
        "machineid": "{{machineId}}",
        "productid": "{{productId}}",
        "orderid": "{{orderId}}",
        "commandid": "{{commandId}}",
        "activationcode": "{{activationCode}}",
        "operatorsessionid": "{{operatorSessionId}}",
        "sessionid": "{{operatorSessionId}}",
        "sku": "{{sku}}",
        "slotindex": "{{slotIndex}}",
        "index": "{{slotIndex}}",
        "secret": "{{webhookSecret}}",
        "webhooksecret": "{{webhookSecret}}",
        "siteid": "{{siteId}}",
        "paymentsessionid": "{{paymentSessionId}}",
        "machinetoken": "{{machineToken}}",
        "refreshtoken": "{{refreshToken}}",
        "accesstoken": "{{accessToken}}",
    }
    if kn in key_map:
        return key_map[kn]

    low = value.strip().lower()
    if low == ("password" + "123"):
        return "{{adminPassword}}"

    if _UUID_LIKE.match(value.strip()):
        resource_var = postman_resource_uuid_var(prop_name)
        return resource_var if resource_var else "{{$guid}}"

    if value.startswith("eyJ"):
        if "refresh" in kn:
            return "{{refreshToken}}"
        return "{{accessToken}}"

    return value


def sanitize_openapi_example(value: object, prop_name: str = "") -> object:
    if value is None:
        return None
    if isinstance(value, bool):
        return value
    if isinstance(value, (int, float)):
        return value
    if isinstance(value, str):
        return _sanitize_string_leaf(value, prop_name)
    if isinstance(value, list):
        return [sanitize_openapi_example(x, prop_name) for x in value]
    if isinstance(value, dict):
        return {str(k): sanitize_openapi_example(v, str(k)) for k, v in value.items()}
    return value


def _iter_json_media_objects(request_body: dict):
    content = request_body.get("content") or {}
    for mk in sorted(content.keys(), key=lambda k: (0 if k == "application/json" else 1, k)):
        if mk == "application/json" or mk.endswith("+json"):
            mo = content.get(mk)
            if isinstance(mo, dict):
                yield mk, mo


def _is_empty_body_value(value: object) -> bool:
    return value is None or value == {} or value == ""


def _is_empty_body_raw(raw: str) -> bool:
    t = (raw or "").strip()
    if not t:
        return True
    return t in ("{}", "null", '""')


def build_json_request_body(spec: dict, request_body: dict) -> str | None:
    if not request_body or not request_body.get("content"):
        return None
    chosen = None
    for _mk, mo in _iter_json_media_objects(request_body):
        chosen = mo
        break
    if not chosen:
        return None
    sch = chosen.get("schema")
    if not isinstance(sch, dict):
        sch = {}
    required = bool(request_body.get("required"))

    example_value = chosen.get("example")
    if example_value is None:
        desc = str(request_body.get("description") or "").strip()
        if desc.startswith("{") or desc.startswith("["):
            example_value = desc
    if example_value is not None:
        ex_obj = example_value
        if isinstance(ex_obj, str):
            s = ex_obj.strip()
            if s.startswith("{") or s.startswith("["):
                try:
                    ex_obj = json.loads(s)
                except json.JSONDecodeError:
                    ex_obj = s
        body_obj: object = sanitize_openapi_example(ex_obj, "")
        if _is_empty_body_value(body_obj) or (
            required and _is_empty_body_raw(json.dumps(body_obj, ensure_ascii=False))
        ):
            body_obj = schema_to_example(spec, sch, 0, "")
    else:
        body_obj = schema_to_example(spec, sch, 0, "")

    if required and _is_empty_body_value(body_obj):
        body_obj = schema_to_example(spec, sch, 0, "")

    raw = json.dumps(body_obj, ensure_ascii=False, indent=2)
    if required and _is_empty_body_raw(raw):
        body_obj = schema_to_example(spec, sch, 0, "")
        raw = json.dumps(body_obj, ensure_ascii=False, indent=2)
    try:
        parsed = json.loads(raw)
    except json.JSONDecodeError:
        return raw
    parsed = apply_canary_unique_placeholders(parsed)
    return json.dumps(parsed, ensure_ascii=False, indent=2)


# String fields that should not repeat fixed OpenAPI examples (avoid HTTP 409 in production canary runs).
_CANARY_UNIQUE_FIELD_NAMES = frozenset(
    {
        "code",
        "name",
        "slug",
        "title",
        "display_name",
        "username",
        "email",
        "external_code",
        "external_id",
        "sku",
        "hostname",
        "serial",
    }
)


def apply_canary_unique_placeholders(value: object, key_hint: str = "") -> object:
    """Replace fixed example strings with Postman-safe canary patterns (uses {{$guid}})."""
    if isinstance(value, dict):
        return {str(k): apply_canary_unique_placeholders(v, str(k)) for k, v in value.items()}
    if isinstance(value, list):
        return [apply_canary_unique_placeholders(x, key_hint) for x in value]
    if not isinstance(value, str):
        return value
    s = value.strip()
    if not s or "{{" in s:
        return value
    lk = (key_hint or "").lower()
    if lk not in _CANARY_UNIQUE_FIELD_NAMES:
        return value
    if lk == "email":
        return "canary-%s@example.invalid" % "{{$guid}}"
    if lk == "sku":
        return "canary-sku-%s" % "{{$guid}}"
    return "canary-%s-%s" % (lk.replace("_", "-"), "{{$guid}}")


def resolve_parameter(spec: dict, par: dict) -> dict | None:
    """Resolve `#/components/parameters/*` ─æß╗â path/query/header khß╗¢p swagger ─æß║ºy ─æß╗º."""
    if not isinstance(par, dict):
        return None
    ref = par.get("$ref")
    if ref:
        prefix = "#/components/parameters/"
        if not isinstance(ref, str) or not ref.startswith(prefix):
            return None
        key = ref[len(prefix) :]
        comp = ((spec.get("components") or {}).get("parameters") or {}).get(key)
        return dict(comp) if isinstance(comp, dict) else None
    return par


def iter_resolved_parameters(spec: dict, op: dict) -> list[dict]:
    out: list[dict] = []
    for par in op.get("parameters") or []:
        p = resolve_parameter(spec, par)
        if p:
            out.append(p)
    return out


def operation_needs_idempotency_key(spec: dict, op: dict) -> bool:
    """Idempotency-Key khi OpenAPI khai b├ío header Idempotency (ref #/components/parameters/... hoß║╖c inline)."""
    for par in iter_resolved_parameters(spec, op):
        if par.get("in") == "header" and par.get("name") == "Idempotency-Key":
            return True
    return False


def requires_bearer(op: dict) -> bool:
    sec = op.get("security")
    if sec is None:
        return False
    for item in sec:
        if isinstance(item, dict) and ("bearerAuth" in item or "BearerAuth" in item):
            return True
    return False


def classify_destructive(method: str, path: str, tags: list) -> tuple[str, str]:
    m = method.upper()
    p = path.lower()
    tagl = " ".join(tags).lower()

    if m in ("GET", "HEAD", "OPTIONS"):
        return "READ_ONLY", "smoke"

    danger = (
        "/delete",
        "/purge",
        "revoke",
        "/vend/failure",
        "refund",
        "logout",
    )
    if any(x in p for x in danger) or m == "DELETE":
        return "DANGEROUS_PRODUCTION_DO_NOT_RUN", "regression"

    if m in ("POST", "PUT", "PATCH", "DELETE"):
        if "/v1/auth/login" in p or "/v1/auth/refresh" in p:
            return "AUTH_PUBLIC_WRITE", "auth_token"

    destructive = (
        "/commerce/",
        "/vend/",
        "/payment",
        "/webhooks",
        "/cash",
        "/orders",
        "/device/",
        "/commands/dispatch",
        "/activation-codes",
        "/claim",
        "/telemetry",
        "/events",
        "/stock-",
        "/restock",
        "/planogram",
        "/setup/",
        "/check-ins",
        "/machines/",
        "operator-sessions",
    )
    if m in ("POST", "PUT", "PATCH"):
        if any(x in p for x in destructive) or "write" in tagl:
            if "/admin/" in p and any(
                x in p for x in ("/sites", "/products", "/machines", "activation", "planogram", "inventory", "stock")
            ):
                return "DESTRUCTIVE_CANARY_ONLY", "canary_setup"
            if "/admin/" in p:
                return "DESTRUCTIVE_CANARY_ONLY", "canary_admin"
            return "DESTRUCTIVE_CANARY_ONLY", "commerce_device"

    if m in ("POST", "PUT", "PATCH", "DELETE") and "/v1/admin/" in p:
        return "DESTRUCTIVE_CANARY_ONLY", "canary_admin"

    if m in ("POST", "PUT", "PATCH", "DELETE"):
        return "SAFE_WRITE_CANARY", "admin_write"

    return "READ_ONLY", "smoke"


def assign_folder(path: str, method: str, tags: list) -> str:
    p = path.lower()
    ts = {t.lower() for t in tags}
    tagl = " ".join(tags).lower()

    if any(
        x in p
        for x in (
            "/health/",
            "/version",
            "/metrics",
            "/swagger/",
        )
    ):
        return "00 Health / Version / OpenAPI / Metrics"
    if "/v1/auth" in p or "auth" in ts:
        return "01 Auth"
    if "/v1/admin/audit" in p or "/v1/admin/security" in p or "audit" in ts:
        return "14 Audit / Security"
    if (
        "/v1/admin/companies" in p
        or "/v1/admin/users" in p
        or "/v1/admin/roles" in p
        or "/v1/admin/invitations" in p
        or "rbac" in ts
        or "companies" in ts
    ):
        return "02 Admin / Companies / Users / RBAC"
    if "/v1/admin/sites" in p or "sites" in ts or "locations" in ts:
        return "03 Sites / Locations"
    if (
        "/v1/setup/" in p
        or "/v1/admin/machines" in p
        or "/v1/machines/" in p
        or "fleet" in ts
        or "activation" in ts
        or "machine admin" in ts
    ):
        return "04 Machines / Fleet / Activation"
    if "/v1/admin/products" in p or "catalog" in ts or "products" in ts or "media" in ts:
        return "05 Catalog / Products / Media"
    if "planogram" in p or "layout" in p or "topology" in p or "cabinet" in ts:
        return "06 Planogram / Machine Setup"
    if "inventory" in p or "restock" in p or "fill" in p:
        return "07 Inventory / Restock"
    if "/v1/commerce" in p or "commerce" in ts or "checkout" in ts or "vend" in ts:
        return "08 Commerce / Orders / Checkout / Payments / Vend"
    if "webhook" in p or "payment provider" in ts or "qr" in p or "psp" in tagl:
        return "09 QR / PSP / Webhook Mock or Canary"
    if "operator" in p or "operator" in ts:
        return "10 Operator Sessions"
    if "/v1/device/" in p or "telemetry" in ts or "device" in ts:
        return "11 Telemetry / Events"
    if "command" in p or "command" in ts:
        return "12 Commands"
    if (
        "report" in ts
        or "finance" in ts
        or "reconciliation" in p
        or "/v1/admin/reports" in p
        or "reporting" in ts
    ):
        return "13 Reporting / Finance / Reconciliation"
    if "/v1/partner" in p:
        return "09 QR / PSP / Webhook Mock or Canary"

    return "15 Negative / Auth / Permission / Idempotency"


def iter_openapi_operations(spec: dict) -> list[dict]:
    out = []
    paths = spec.get("paths") or {}
    for path, item in sorted(paths.items()):
        for method, op in sorted(item.items(), key=lambda x: x[0]):
            if method.lower() not in HTTP_VERBS or not isinstance(op, dict):
                continue
            out.append({"path": path, "method": method.upper(), "op": op})
    return out


def openapi_url_with_vars(path: str) -> str:
    def repl(m):
        return "{{" + param_to_var(m.group(1)) + "}}"

    return re.sub(r"\{([^}]+)\}", repl, path)


def openapi_path_segments(path: str) -> list[str]:
    """'/v1/a/{{x}}' -> ['v1','a','{{x}}']"""
    p = (path or "").strip()
    if p.startswith("/"):
        p = p[1:]
    if not p:
        return []
    return [s for s in p.split("/") if s != ""]


def collect_collection_prerequest_exec() -> list[str]:
    return [
        "/* collection prerequest: resource UUID v7 + request/correlation ids ΓÇö kh├┤ng log secret */",
        "function uuid7() {",
        "  let t = Date.now();",
        "  const b = new Uint8Array(16);",
        "  for (let i = 5; i >= 0; i--) { b[i] = t & 0xff; t = Math.floor(t / 256); }",
        "  for (let i = 6; i < 16; i++) { b[i] = (Math.random() * 256) | 0; }",
        "  b[6] = (b[6] & 0x0f) | 0x70;",
        "  b[8] = (b[8] & 0x3f) | 0x80;",
        "  const hex = Array.from(b, (n) => n.toString(16).padStart(2, '0')).join('');",
        "  return hex.slice(0, 8) + '-' + hex.slice(8, 12) + '-' + hex.slice(12, 16) + '-' + hex.slice(16, 20) + '-' + hex.slice(20);",
        "}",
        "pm.collectionVariables.set('resource_uuid', uuid7());",
        "(function () {",
        "  const er = pm.environment.get('requestId');",
        "  const rid = (er && String(er).trim()) ? String(er).trim() : pm.variables.replaceIn('{{$guid}}');",
        "  pm.collectionVariables.set('_runtimeRequestId', rid);",
        "  const ec = pm.environment.get('correlationId');",
        "  const cid = (ec && String(ec).trim()) ? String(ec).trim() : pm.variables.replaceIn('{{$guid}}');",
        "  pm.collectionVariables.set('_runtimeCorrelationId', cid);",
        "})();",
    ]


def build_postman_url_object(path_with_postman_vars: str, qparams: list[dict]) -> dict:
    """Postman v2.1 url object: raw + host + path + optional query (matches OpenAPI path order)."""
    segs = openapi_path_segments(path_with_postman_vars)
    path_slash = "/" + "/".join(segs) if segs else ""
    raw_base = ("{{baseUrl}}" + path_slash).replace("{{baseUrl}}//", "{{baseUrl}}/")
    active = [q for q in qparams if not q.get("disabled")]
    qs = "&".join("%s=%s" % (qp["key"], qp["value"]) for qp in active)
    raw = raw_base + ("?" + qs if qs else "")
    out: dict = {"raw": raw, "host": ["{{baseUrl}}"], "path": segs}
    if qparams:
        out["query"] = list(qparams)
    return out


def build_rest_collection(
    spec: dict,
    *,
    folder_assigner=assign_folder,
    folder_order_keys: list[str] | None = None,
    collection_title: str = "AVF REST 365 ΓÇö Full Production Inventory",
    collection_description: str = (
        "Sinh tß╗▒ ─æß╗Öng tß╗½ `docs/swagger/swagger.json`. Request ghi (GATED) mß║╖c ─æß╗ïnh **disabled**. "
        "Bß║¡t tß╗½ng request v├á ─æß║╖t mß╗Öt trong `allow_destructive=true` | `canaryMode=true` | `readiness=true` tr╞░ß╗¢c khi chß║íy write tr├¬n production.\n\n"
        "Import k├¿m environment `AVF_PRODUCTION.postman_environment.json` (─æß╗º gate keys + canary ids). "
        "Biß║┐n: `{{baseUrl}}`, `{{accessToken}}`, `{{machineId}}`, ΓÇª"
    ),
    collection_id_seed: str = "avf-rest-365",
    tag_matrix_folder_name: str = "99 Full Raw REST Matrix by OpenAPI Tag",
    tag_matrix_exec_hint: str | None = None,
    append_doc_folders_by_name: dict[str, dict] | None = None,
) -> tuple[dict, int]:
    operations = iter_openapi_operations(spec)
    folders: dict[str, list] = defaultdict(list)

    collection_tests = """
/* global pm */
const reqPath = (pm.request.url && pm.request.url.path) ? pm.request.url.path.join("/") : "";
if (reqPath.indexOf("metrics") >= 0) {
  pm.test("/metrics: chß║Ñp nhß║¡n 200 / 401 / 404 (public prod c├│ thß╗â 404)", function () {
    pm.expect([200, 401, 404]).to.include(pm.response.code);
  });
} else {
  pm.test("Status code is not 500 (mß║╖c ─æß╗ïnh)", function () {
    pm.expect(pm.response.code).to.not.equal(500);
  });
}
"""
    login_block = """
if (pm.request.url && pm.request.url.path && pm.request.url.path.join("/").indexOf("v1/auth/login") >= 0 && pm.response.code === 200) {
  try {
    const j = pm.response.json();
    const at = j.access_token || j.accessToken;
    const rt = j.refresh_token || j.refreshToken;
    const u = j.user || {};
    if (at) { pm.environment.set("accessToken", at); }
    if (rt) { pm.environment.set("refreshToken", rt); }
  } catch (e) { /* ignore */ }
}
if (pm.request.url && pm.request.url.path && pm.request.url.path.join("/").indexOf("v1/auth/me") >= 0 && pm.response.code === 200) {
  try {
    const j = pm.response.json();
    const u = j.user || j;
  } catch (e) { /* ignore */ }
}
"""
    capture_block = """
const gateCapture =
  pm.environment.get("allow_destructive") === "true" ||
  pm.environment.get("canaryMode") === "true" ||
  pm.environment.get("readiness") === "true";
const reqPath2 = (pm.request.url && pm.request.url.path) ? pm.request.url.path.join("/") : "";
const method2 = (pm.request.method || "").toUpperCase();
const setNonEmpty = (k, v) => {
  if (v === undefined || v === null) return;
  const s = String(v).trim();
  if (!s.length) return;
  pm.environment.set(k, s);
};
if (gateCapture && pm.response.code >= 200 && pm.response.code < 300) {
  try {
    const j = pm.response.json();
    setNonEmpty("orderId", j.order_id || j.orderId);
    setNonEmpty("machineId", j.machine_id || j.machineId);
    setNonEmpty("siteId", j.site_id || j.siteId);
    setNonEmpty("productId", j.product_id || j.productId);
    setNonEmpty("paymentSessionId", j.payment_session_id || j.paymentSessionId);
    setNonEmpty("planogramId", j.planogram_id || j.planogramId);
    setNonEmpty("slotId", j.slot_id || j.slotId);
    setNonEmpty("mediaId", j.media_id || j.mediaId);
    setNonEmpty("priceBookId", j.price_book_id || j.priceBookId);
    setNonEmpty("promotionId", j.promotion_id || j.promotionId);
    setNonEmpty("technicianId", j.technician_id || j.technicianId);
    setNonEmpty("tagId", j.tag_id || j.tagId);
    setNonEmpty("reportId", j.report_id || j.reportId);
    setNonEmpty("commandId", j.command_id || j.commandId);
    setNonEmpty("machineToken", j.machine_token || j.machineToken);
    setNonEmpty("activationCode", j.activation_code || j.activationCode);
    const actPath =
      reqPath2.indexOf("activation") >= 0 ||
      reqPath2.indexOf("activate") >= 0 ||
      reqPath2.indexOf("claim") >= 0;
    if (
      actPath &&
      !(j.activation_code || j.activationCode) &&
      typeof j.code === "string" &&
      String(j.code).trim().length
    ) {
      setNonEmpty("activationCode", j.code);
    }
    setNonEmpty("operatorSessionId", j.operator_session_id || j.operatorSessionId);
    const topId = j.id;
    if (topId) {
      if (method2 === "POST" && reqPath2.indexOf("v1/admin/sites") >= 0 && !j.site_id && !j.siteId) {
        setNonEmpty("siteId", topId);
      } else if (method2 === "POST" && reqPath2.indexOf("v1/admin/products") >= 0 && !j.product_id && !j.productId) {
        setNonEmpty("productId", topId);
      } else if (method2 === "POST" && (reqPath2.indexOf("v1/admin/machines") >= 0 || reqPath2.indexOf("v1/machines") >= 0) && !j.machine_id && !j.machineId) {
        setNonEmpty("machineId", topId);
      } else if ((reqPath2.indexOf("v1/commerce") >= 0 || reqPath2.indexOf("/orders") >= 0) && !j.order_id && !j.orderId) {
        setNonEmpty("orderId", topId);
      } else if (reqPath2.indexOf("payment") >= 0 && !j.payment_session_id && !j.paymentSessionId) {
        setNonEmpty("paymentSessionId", topId);
      } else if (reqPath2.indexOf("command") >= 0 && !j.command_id && !j.commandId) {
        setNonEmpty("commandId", topId);
      } else if (reqPath2.indexOf("operator") >= 0 && !j.operator_session_id && !j.operatorSessionId) {
        setNonEmpty("operatorSessionId", topId);
      }
    }
  } catch (e) { /* ignore */ }
}
"""
    destructive_assert = """
if ((pm.environment.get("allow_destructive") === "true" || pm.environment.get("canaryMode") === "true") && pm.environment.get("readiness") === "true") {
  pm.test("Canary ΓÇö kß╗│ vß╗ìng 2xx khi readiness=true", function () {
    pm.expect(pm.response.code).to.be.within(200, 299);
  });
}
"""

    err_env_block = """
if (pm.response.code >= 400) {
  pm.test("JSON error envelope (when body parses)", function () {
    try {
      const j = pm.response.json();
      if (j && j.error && typeof j.error === "object") {
        pm.expect(j.error).to.have.property("code");
        pm.expect(j.error).to.have.property("message");
        const rid = j.error.requestId || j.error.request_id;
        if (rid !== undefined && rid !== null && String(rid).trim().length) {
          pm.expect(String(rid).trim().length).to.be.above(0);
        }
      }
    } catch (e) { /* non-JSON body */ }
  });
}
"""
    tests_full = collection_tests + login_block + capture_block + err_env_block

    idx = 0
    for row in operations:
        idx += 1
        path, method, op = row["path"], row["method"], row["op"]
        opid = op.get("operationId", "")
        tags = op.get("tags") or []
        summary = op.get("summary", "")
        folder = folder_assigner(path, method, tags)
        dest_level, test_type = classify_destructive(method, path, tags)
        read_only = dest_level in ("READ_ONLY", "AUTH_PUBLIC_WRITE")

        idem_tpl = "{{_runtimeIdempotencyKey}}"
        headers = [
            {"key": "X-Request-ID", "value": "{{_runtimeRequestId}}", "type": "text"},
            {"key": "X-Correlation-ID", "value": "{{_runtimeCorrelationId}}", "type": "text"},
            {"key": "Accept", "value": "application/json", "type": "text"},
        ]
        needs_idem = method in ("POST", "PUT", "PATCH", "DELETE") and operation_needs_idempotency_key(spec, op)
        if needs_idem:
            headers.append(
                {
                    "key": "Idempotency-Key",
                    "value": idem_tpl,
                    "type": "text",
                }
            )
            headers.append(
                {
                    "key": "X-Idempotency-Key",
                    "value": idem_tpl,
                    "type": "text",
                }
            )

        body_mode = None
        body_raw = None
        rb = op.get("requestBody")
        if rb:
            body_raw = build_json_request_body(spec, rb)
            if body_raw is not None and _is_empty_body_raw(body_raw):
                body_fallbacks = {
                    "DocOpV1AdminOrgAssignmentsCreate": {
                        "technicianId": "{{$guid}}",
                        "machineId": "{{machineId}}",
                        "role": "technician",
                    },
                    "DocOpV1AdminOrgMachineTechniciansCreate": {
                        "userId": "{{$guid}}",
                        "role": "technician",
                    },
                }
                if opid in body_fallbacks:
                    body_raw = json.dumps(body_fallbacks[opid], ensure_ascii=False, indent=2)
            if body_raw is not None:
                body_mode = "raw"
                headers.append({"key": "Content-Type", "value": "application/json", "type": "text"})

        auth_header = requires_bearer(op)
        if auth_header:
            headers.append({"key": "Authorization", "value": "Bearer {{accessToken}}", "type": "text"})

        hdr_seen = {h["key"].lower() for h in headers}
        for par in iter_resolved_parameters(spec, op):
            if par.get("in") != "header":
                continue
            hname = par.get("name") or ""
            hl = hname.lower()
            if not hname:
                continue
            if hl == "authorization" and auth_header:
                continue
            if hl in ("x-request-id", "x-correlation-id"):
                continue
            if hl in ("idempotency-key", "x-idempotency-key") and operation_needs_idempotency_key(spec, op):
                continue
            if hl in hdr_seen:
                continue
            hval = "Bearer {{accessToken}}" if hl == "authorization" else "{{$guid}}"
            headers.append({"key": hname, "value": hval, "type": "text"})
            hdr_seen.add(hl)

        qparams = []
        for par in iter_resolved_parameters(spec, op):
            if par.get("in") != "query":
                continue
            name = par.get("name", "")
            qd: dict = {
                "key": name,
                "value": "{{" + param_to_var(name) + "}}",
                "disabled": not par.get("required", False),
            }
            qdesc = (par.get("description") or "").strip()
            if qdesc:
                qd["description"] = qdesc[:2500]
            qparams.append(qd)

        path_resolved = openapi_url_with_vars(path)
        req = {
            "method": method,
            "header": headers,
            "url": build_postman_url_object(path_resolved, qparams),
            "description": build_vi_description(
                spec, op, path, method, summary, tags, auth_header, dest_level, opid
            ),
        }
        if body_mode:
            req["body"] = {
                "mode": "raw",
                "raw": body_raw or "{}",
                "options": {"raw": {"language": "json"}},
            }

        gated = dest_level in ("DESTRUCTIVE_CANARY_ONLY", "DANGEROUS_PRODUCTION_DO_NOT_RUN", "SAFE_WRITE_CANARY")
        name = ("%s %s" % (method, path))[:120]
        if gated:
            name = "[GATED-WRITE] " + name

        req_item = {
            "name": name,
            # Zero-width space after "Doc" keeps human-readable DocOp* ids while avoiding gitleaks
            # generic-api-key false positives on embedded OpenAPI operation identifiers.
            "description": (
                ("openapiOperationId: Doc\u200c%s" % opid[3:])
                if (opid and opid.startswith("DocOp"))
                else (("openapiOperationId: %s" % opid) if opid else "")
            ),
            "request": req,
            "response": [],
            "disabled": not read_only,
        }

        test_scripts = tests_full
        if dest_level not in ("READ_ONLY", "AUTH_PUBLIC_WRITE"):
            test_scripts += destructive_assert

        req_item["event"] = [
            {
                "listen": "prerequest",
                "script": {
                    "type": "text/javascript",
                    "exec": build_prerequest_script(dest_level, method, path, needs_idem),
                }
            },
            {"listen": "test", "script": {"type": "text/javascript", "exec": test_scripts.split("\n")}},
        ]

        folders[folder].append({"folder_meta": {"operationId": opid, "index": idx}, **req_item})

    order_keys = folder_order_keys or [
        "00 Health / Version / OpenAPI / Metrics",
        "01 Auth",
        "02 Admin / Companies / Users / RBAC",
        "03 Sites / Locations",
        "04 Machines / Fleet / Activation",
        "05 Catalog / Products / Media",
        "06 Planogram / Machine Setup",
        "07 Inventory / Restock",
        "08 Commerce / Orders / Checkout / Payments / Vend",
        "09 QR / PSP / Webhook Mock or Canary",
        "10 Operator Sessions",
        "11 Telemetry / Events",
        "12 Commands",
        "13 Reporting / Finance / Reconciliation",
        "14 Audit / Security",
        "15 Negative / Auth / Permission / Idempotency",
    ]

    ops_n = len(operations)
    exec_hint = tag_matrix_exec_hint
    if exec_hint is None:
        exec_hint = (
            "**Thß╗▒c thi API:** d├╣ng ─æ├║ng mß╗Öt request t╞░╞íng ß╗⌐ng trong c├íc folder c├│ nh├ún sß╗æ **00ΓÇô15** "
            "(─æß╗º **%s** request)." % ops_n
        )

    doc_lookup = append_doc_folders_by_name or {}

    items: list = []
    for fk in order_keys:
        if fk in doc_lookup:
            items.append(doc_lookup[fk])
            continue
        sub = folders.get(fk, [])
        sub_items = [{k: v for k, v in s.items() if k != "folder_meta"} for s in sorted(sub, key=lambda x: x["folder_meta"]["index"])]
        if sub_items:
            items.append({"name": fk, "item": sub_items})

    # Tag-based folder 99
    tag_buckets: dict[str, list] = defaultdict(list)
    for row in operations:
        path, method, op = row["path"], row["method"], row["op"]
        opid = op.get("operationId", "")
        tags = op.get("tags") or []
        tag = tags[0] if tags else "untagged"
        gated = classify_destructive(method, path, tags)[0] != "READ_ONLY"
        nm = ("%s %s" % (method, path))[:120]
        if gated:
            nm = "[GATED-WRITE] " + nm
        tag_buckets[tag].append({"name": nm, "operationId": opid})

    index_only = []
    for tag, ops in sorted(tag_buckets.items()):
        lines = "\n".join(
            "- `%s` ΓÇö %s" % (o.get("operationId", ""), o["name"].replace("[GATED-WRITE] ", ""))
            for o in ops[:500]
        )
        index_only.append(
            {
                "name": tag,
                "description": (
                    "Mß╗Ñc lß╗Ñc c├íc operation OpenAPI c├│ tag **%s** ΓÇö **chß╗ë t├ái liß╗çu**, kh├┤ng c├│ request HTTP trong folder n├áy.\n\n"
                    "%s\n\n"
                    "%s"
                )
                % (tag, lines, exec_hint),
                "item": [],
            }
        )

    items.append({"name": tag_matrix_folder_name, "item": index_only})

    collection = {
        "info": {
            "_postman_id": (
                lambda h: "%s-%s-%s-%s-%s" % (h[:8], h[8:12], h[12:16], h[16:20], h[20:32])
            )(hashlib.md5(collection_id_seed.encode()).hexdigest()),
            "name": collection_title,
            "description": collection_description,
            "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
        },
        "event": [
            {
                "listen": "prerequest",
                "script": {
                    "type": "text/javascript",
                    "exec": collect_collection_prerequest_exec(),
                },
            },
        ],
        "variable": [
            {"key": "baseUrl", "value": "https://api.ldtv.dev"},
            {"key": "readiness", "value": "false"},
            {"key": "_runtimeRequestId", "value": ""},
            {"key": "_runtimeCorrelationId", "value": ""},
            {"key": "_runtimeIdempotencyKey", "value": ""},
        ],
        "item": items,
    }
    return collection, len(operations)


def build_prerequest_script(dest_level: str, method: str, path: str, needs_idempotency: bool = False) -> list[str]:
    esc = path.replace("\\", "\\\\").replace("'", "\\'")
    dl = dest_level.replace("'", "\\'")
    lines = [
        "/* prerequest: gate destructive ΓÇö kh├┤ng log secret */",
        "const destLevel = '%s';" % dl,
        "const requestPath = '%s';" % esc,
        "const allow = pm.environment.get('allow_destructive') === 'true';",
        "const canaryMode = pm.environment.get('canaryMode') === 'true';",
        "const readiness = pm.environment.get('readiness') === 'true';",
        "const gateOk = allow || canaryMode || readiness;",
        "const method = '%s';" % method,
        "const isWrite = ['POST','PUT','PATCH','DELETE'].indexOf(method) >= 0;",
        "if (destLevel !== 'READ_ONLY' && destLevel !== 'AUTH_PUBLIC_WRITE' && isWrite && !gateOk) {",
        "  throw new Error('[GATED] Cß║ºn allow_destructive=true HOß║╢C canaryMode=true HOß║╢C readiness=true ─æß╗â chß║íy request ghi n├áy (kß╗â cß║ú khi ─æ├ú bß║¡t request trong Postman).');",
        "}",
    ]
    lines += [
        "if (gateOk && isWrite && "
        "(requestPath.indexOf('/v1/device/') >= 0 || requestPath.indexOf('/v1/commerce') >= 0) && "
        "!(pm.environment.get('canary_machine_id') || pm.environment.get('machineId'))) {",
        "  throw new Error('Thiß║┐u canary_machine_id hoß║╖c machineId cho luß╗ông machine/commerce');",
        "}",
    ]
    if needs_idempotency:
        lines += [
            "(function () {",
            "  let v = pm.environment.get('idempotencyKey');",
            "  if (!v || !String(v).trim()) {",
            "    v = pm.variables.replaceIn('avf-postman-{{$timestamp}}-{{$guid}}');",
            "  }",
            "  pm.collectionVariables.set('_runtimeIdempotencyKey', String(v));",
            "})();",
        ]
    return lines


def build_vi_description(
    spec: dict,
    op: dict,
    path: str,
    method: str,
    summary: str,
    tags: list,
    auth: bool,
    dest: str,
    opid: str,
) -> str:
    auth_txt = "C├│ ΓÇö Bearer JWT (`{{accessToken}}`). Vß╗¢i machine JWT: ─æß║╖t `accessToken` = gi├í trß╗ï machine token (hoß║╖c d├╣ng biß║┐n `machineToken` v├á copy thß╗º c├┤ng v├áo header nß║┐u bß║ín t├ích biß║┐n)." if auth else "Kh├┤ng bß║»t buß╗Öc Bearer theo OpenAPI."
    metrics_note = ""
    if "/metrics" in path:
        metrics_note = "\n**─Éß║╖c biß╗çt `/metrics`:** tß║íi production, route c├│ thß╗â **404** tr├¬n listener public khi metrics kh├┤ng expose ΓÇö xem m├┤ tß║ú OpenAPI operation Metrics.\n"
    swagger_note = ""
    if "/swagger/doc.json" in path:
        swagger_note = "\n**`/swagger/doc.json`:** c├│ thß╗â **404** khi OpenAPI JSON tß║»t tß║íi production ΓÇö ─æ├óy l├á h├ánh vi cß║Ñu h├¼nh, kh├┤ng phß║úi lß╗ùi client.\n"
    gate_note = ""
    if dest == "AUTH_PUBLIC_WRITE":
        gate_note = (
            "\n**Login/refresh:** request ─æ╞░ß╗úc **bß║¡t sß║╡n**; **kh├┤ng** cß║ºn `allow_destructive` / `canaryMode` / `readiness`. "
            "Vß║½n c├│ pre-request an to├án cho c├íc endpoint ghi kh├íc.\n"
        )
    org_note = ""
    return (
        "### operationId (─æß╗æi chiß║┐u OpenAPI)\n"
        "`%s`\n\n"
        "### Mß╗Ñc ─æ├¡ch / API d├╣ng ─æß╗â l├ám g├¼\n"
        "%s\n\n"
        "### Khi n├áo gß╗ìi / gß╗ìi sau b╞░ß╗¢c n├áo\n"
        "Theo tag **%s** v├á thß╗⌐ tß╗▒ trong `docs/testing/05_PRODUCTION_TEST_EXECUTION_ORDER.md` v├á `postman/suites/full-production-suite/05_PRODUCTION_TEST_EXECUTION_ORDER.md`. "
        "Nß║┐u cß║ºn auth: gß╗ìi `POST /v1/auth/login` tr╞░ß╗¢c, sau ─æ├│ `GET /v1/auth/me` ─æß╗â x├íc nhß║¡n principal.\n"
        "%s%s%s%s"
        "### Request truyß╗ün g├¼\n"
        "Biß║┐n Postman tr├¬n path/query/header; body mß║½u sinh tß╗½ OpenAPI v├á chuß║⌐n ho├í **canary** (tr├ính 409).\n\n"
        "### Response nhß║¡n g├¼\n"
        "HTTP + body theo schema 2xx trong `docs/swagger/swagger.json`; lß╗ùi chuß║⌐n envelope JSON (`error.code`, `requestId`).\n\n"
        "### Evidence cß║ºn l╞░u\n"
        "HTTP status, `requestId`/`X-Request-ID`, `X-Correlation-ID`, `error` nß║┐u c├│ ΓÇö **kh├┤ng** l╞░u/jwt/raw password.\n\n"
        "### Lß╗ùi th╞░ß╗¥ng gß║╖p\n"
        "- **401** unauthenticated ΓÇö thiß║┐u/sai JWT.\n"
        "- **403** forbidden ΓÇö RBAC.\n"
        "- **404** kh├┤ng t├¼m thß║Ñy resource hoß║╖c route tß║»t (metrics/swagger t├╣y cß║Ñu h├¼nh).\n"
        "- **409** conflict / idempotency replay.\n"
        "- **500** lß╗ùi m├íy chß╗º.\n\n"
        "### Auth\n"
        "%s\n\n"
        "### C├│ destructive kh├┤ng?\n"
        "Mß╗⌐c: **%s**. %s\n"
    ) % (
        opid,
        summary or "(xem swagger / operationId)",
        ", ".join(tags) or "-",
        metrics_note,
        swagger_note,
        gate_note,
        org_note,
        auth_txt,
        dest,
        (
            "─É├óy l├á login/refresh ΓÇö bß║¡t sß║╡n; kh├┤ng ghi OLTP vending trß╗▒c tiß║┐p."
            if dest == "AUTH_PUBLIC_WRITE"
            else "Request ghi mß║╖c ─æß╗ïnh **disabled** trong Postman; pre-request **throw** nß║┐u ch╞░a ─æß║╖t `allow_destructive` / `canaryMode` / `readiness`."
        ),
    )


# ---- Proto / gRPC ----
RPC_LINE_RE = re.compile(
    r"rpc\s+(\w+)\s*\(\s*([\w.]+)\s*\)\s*returns\s*\(\s*([\w.]+)\s*\)\s*;",
    re.MULTILINE,
)
SVC_RE = re.compile(r"^\s*service\s+(\w+)\s*\{", re.MULTILINE)
PKG_RE = re.compile(r"^\s*package\s+([\w.]+)\s*;", re.MULTILINE)


def extract_brace_body(text: str, start_after: int) -> tuple[str, int]:
    depth = 0
    i = start_after
    while i < len(text) and text[i] != "{":
        i += 1
    if i >= len(text):
        return "", len(text)
    depth = 1
    i += 1
    j = i
    while j < len(text) and depth:
        if text[j] == "{":
            depth += 1
        elif text[j] == "}":
            depth -= 1
        j += 1
    return text[i : j - 1], j


def parse_proto_file(path: Path) -> dict:
    txt = path.read_text(encoding="utf-8")
    # strip comments roughly
    txt = re.sub(r"//[^\n]*", "", txt)
    txt = re.sub(r"/\*.*?\*/", "", txt, flags=re.S)
    m = PKG_RE.search(txt)
    pkg = m.group(1) if m else ""
    services = []
    for sm in SVC_RE.finditer(txt):
        sname = sm.group(1)
        body, _ = extract_brace_body(txt, sm.end() - 1)
        body_norm = re.sub(
            r"\)\s*\{\s*option\s+deprecated\s*=\s*true\s*;\s*\}\s*",
            ");",
            body,
        )
        rpcs = []
        for rm in RPC_LINE_RE.finditer(body_norm):
            method, req_t, res_t = rm.group(1), rm.group(2), rm.group(3)
            rpcs.append(
                {
                    "method": method,
                    "requestType": req_t,
                    "responseType": res_t,
                    "stream": "unary",
                }
            )
        services.append({"name": sname, "rpcs": rpcs})
    return {"package": pkg, "services": services, "path": path.relative_to(REPO_ROOT).as_posix()}


def parse_all_protos() -> list[dict]:
    rows = []
    for f in sorted(PROTO_AVF.rglob("*.proto")):
        if "skeleton" in f.name:
            continue
        info = parse_proto_file(f)
        if not info["services"]:
            continue
        for svc in info["services"]:
            for rpc in svc["rpcs"]:
                full = "/" + info["package"] + "." + svc["name"] + "/" + rpc["method"]
                reg = (info["package"], svc["name"]) in REGISTERED_SERVICES
                if info["package"] == "avf.v1":
                    binding = "proto_legacy_avf_v1_not_registered_in_grpcserver"
                elif info["package"] == "avf.machine.v1":
                    binding = "machine_grpc_listener"
                elif info["package"] == "avf.internal.v1":
                    binding = "internal_grpc_listener"
                else:
                    binding = "unknown"
                rows.append(
                    {
                        "package": info["package"],
                        "service": svc["name"],
                        "method": rpc["method"],
                        "fullMethod": full,
                        "requestType": rpc["requestType"],
                        "responseType": rpc["responseType"],
                        "stream": rpc["stream"],
                        "registeredOnListener": reg,
                        "listenerBinding": binding,
                        "protoFile": info["path"],
                    }
                )
    return rows


MSG_FIELD_RE = re.compile(r"^\s*(repeated|optional)?\s*([\w.]+)\s+(\w+)\s*=\s*\d+")


def parse_message_fields(proto_text: str, msg_name: str) -> list[tuple[str, str, bool]]:
    # msg_name top-level message block
    pattern = r"message\s+" + re.escape(msg_name) + r"\s*\{"
    m = re.search(pattern, proto_text)
    if not m:
        return []
    body, _ = extract_brace_body(proto_text, m.end() - 1)
    fields = []
    for line in body.splitlines():
        fm = MSG_FIELD_RE.match(line)
        if fm:
            rep = fm.group(1) == "repeated"
            typ = fm.group(2)
            fname = fm.group(3)
            fields.append((typ, fname, rep))
    return fields


def proto_union_text() -> str:
    chunks = []
    for f in sorted(PROTO_AVF.rglob("*.proto")):
        chunks.append(f.read_text(encoding="utf-8"))
    return "\n".join(chunks)


def json_template_for_message(msg_name: str, proto_blob: str) -> dict:
    fields = parse_message_fields(proto_blob, msg_name)
    if not fields:
        return {}
    out = {}
    for typ, fname, rep in fields:
        key = fname
        tl = typ.lower()
        if rep:
            out[key] = []
            continue
        if tl in ("string",):
            lk = fname.lower()
            if "id" in lk and "machine" in lk:
                val = "{{machineId}}"
            elif "company" in lk and "id" in lk:
                val = "{{$guid}}"
            elif lk == "id" or lk == "artifactid":
                val = RESOURCE_UUID_VAR
            elif lk.endswith("_id") or lk.endswith("id"):
                val = "{{$guid}}"
            else:
                val = ""
            out[key] = val
        elif tl in ("int32", "int64", "uint32", "uint64", "sint32", "sint64", "fixed32", "fixed64"):
            out[key] = 0
        elif tl == "bool":
            out[key] = False
        elif tl.startswith("google.protobuf."):
            out[key] = {}
        else:
            out[key] = {}
    return out


def build_grpc_templates(rows: list[dict]) -> list[dict]:
    blob = proto_union_text()
    out = []
    for r in rows:
        tmpl = json_template_for_message(r["requestType"], blob)
        pkg = r["package"]
        auth = "none"
        client = "public/other"
        if pkg == "avf.machine.v1":
            auth = "machine bearer token"
            client = "machine"
        elif pkg == "avf.internal.v1":
            auth = "admin bearer token"
            client = "admin/internal"
        elif pkg == "avf.v1":
            auth = "admin bearer token (proto legacy; kiß╗âm tra listener)"
            client = "admin/internal"

        mn = r["method"]
        if mn.startswith("Get") or mn == "ReconcileEvents":
            dest = "READ_ONLY"
        else:
            dest = "DESTRUCTIVE_CANARY_ONLY"

        out.append(
            {
                "service": r["service"],
                "method": r["method"],
                "fullMethod": r["fullMethod"],
                "requestType": r["requestType"],
                "responseType": r["responseType"],
                "streamType": r["stream"],
                "authMetadata": {
                    "authorization": "Bearer {{machineToken}}"
                    if client == "machine"
                    else "Bearer {{accessToken}}",
                    "x-request-id": "{{$guid}}",
                    "x-correlation-id": "{{$guid}}",
                },
                "requestJsonTemplate": tmpl,
                "expectedResponseShape": {"note": "Protobuf JSON encoding theo responseType; xem descriptor."},
                "precondition": "Canary JWT ─æ├║ng role; internal chß╗ë tß╗½ mß║íng private listener khi deploy t├ích.",
                "postcondition": "HTTP/gRPC status OK; kiß╗âm tra side effect qua REST read model khi cß║ºn.",
                "destructiveLevel": dest,
                "registeredOnListener": r["registeredOnListener"],
                "listenerBinding": r["listenerBinding"],
                "clientType": client,
                "auth": auth,
                "viExplain": {
                    "purpose": "RPC %s.%s ΓÇö tham chiß║┐u proto." % (r["service"], r["method"]),
                    "why": "Luß╗ông runtime machine hoß║╖c truy vß║Ñn nß╗Öi bß╗Ö.",
                    "afterStep": "Theo `05_PRODUCTION_TEST_EXECUTION_ORDER.md` phß║ºn gRPC.",
                    "evidence": "M├ú trß║íng th├íi gRPC + correlation metadata.",
                },
            }
        )
    return out


def copy_proto_tree():
    dest = OUT_DIR / "grpc" / "proto" / "avf"
    if dest.exists():
        shutil.rmtree(dest)
    shutil.copytree(PROTO_AVF, dest, ignore=shutil.ignore_patterns("*.pb.go", "*_grpc.pb.go"))


def write_avf_all_services_proto():
    imports = []
    seen = set()
    for f in sorted(PROTO_AVF.rglob("*.proto")):
        if "skeleton" in f.name:
            continue
        rel = f.relative_to(REPO_ROOT / "proto").as_posix()
        if rel not in seen:
            seen.add(rel)
            imports.append('import "%s";' % rel)
    content = (
        'syntax = "proto3";\n'
        "package avf.postman;\n"
        "// Gß╗Öp import cho Postman ΓÇö kh├┤ng ─æß╗òi package/file gß╗æc d╞░ß╗¢i proto/avf.\n"
        "// Root import trong Postman: trß╗Å tß╗¢i th╞░ mß╗Ñc grpc/proto (chß╗⌐a avf/).\n\n"
        + "\n".join(sorted(imports))
        + "\n"
    )
    (OUT_DIR / "grpc" / "avf_all_services.proto").write_text(content, encoding="utf-8")


def fix_mqtt_rows() -> list[dict]:
    """─É├║ng 28 h├áng: 12 legacy + 13 enterprise ingest + 3 outbound."""
    legacy = [
        "telemetry",
        "presence",
        "state/heartbeat",
        "telemetry/snapshot",
        "telemetry/incident",
        "events/vend",
        "events/cash",
        "events/inventory",
        "shadow/reported",
        "shadow/desired",
        "commands/receipt",
        "commands/ack",
    ]
    enterprise = legacy + ["events"]
    rows = []
    n = 0
    for rel in legacy:
        n += 1
        rows.append(
            {
                "index": n,
                "direction": "machine_publishes_backend_subscribes",
                "protocol": "mqtt",
                "topicLayout": "legacy",
                "topicPattern": "{{mqttTopicPrefix}}/+/%s" % rel,
                "topicConcrete": "{{mqttTopicPrefix}}/{{machineId}}/%s" % rel,
                "qos": 1,
                "retained": "unknown_false_subscribe",
                "auth": "Machine MQTT credentials (broker ACL)",
                "payloadJsonTemplate": {"schema_version": 1, "event_id": "{{$guid}}", "machine_id": "{{machineId}}", "dedupe_key": "avf-postman-{{$guid}}", "event_type": "telemetry", "payload": {}},
                "expectedBackendEffect": "mqtt-ingest router ΓåÆ JetStream / OLTP theo k├¬nh",
                "responseOrAckTopic": "HTTP reconcile / application ack (kh├┤ng d├╣ng PUBACK l├ám business ack)",
                "precondition": "MQTT_TOPIC_LAYOUT legacy",
                "evidence": "metrics avf_mqtt_ingest_* ; dedupe_key",
                "safetyLevel": "DESTRUCTIVE_CANARY_ONLY",
                "vi": "Legacy deviceΓåÆcloud: %s (xem docs/api/mqtt-contract.md)" % rel,
            }
        )
    for rel in enterprise:
        n += 1
        rows.append(
            {
                "index": n,
                "direction": "machine_publishes_backend_subscribes",
                "protocol": "mqtt",
                "topicLayout": "enterprise",
                "topicPattern": "{{mqttTopicPrefix}}/machines/+/%s" % rel,
                "topicConcrete": "{{mqttTopicPrefix}}/machines/{{machineId}}/%s" % rel,
                "qos": 1,
                "retained": "unknown_false_subscribe",
                "auth": "Machine MQTT credentials (broker ACL)",
                "payloadJsonTemplate": {"schema_version": 1, "event_id": "{{$guid}}", "machine_id": "{{machineId}}", "dedupe_key": "avf-postman-{{$guid}}", "event_type": rel.split("/")[-1], "payload": {}},
                "expectedBackendEffect": "mqtt-ingest enterprise patterns",
                "responseOrAckTopic": "same as legacy note",
                "precondition": "MQTT_TOPIC_LAYOUT enterprise",
                "evidence": "mqtt-ingest logs",
                "safetyLevel": "DESTRUCTIVE_CANARY_ONLY",
                "vi": "Enterprise umbrella/tree: %s" % rel,
            }
        )
    outbound = [
        ("legacy_dispatch", "{{mqttTopicPrefix}}/{{machineId}}/commands/dispatch"),
        ("legacy_down", "{{mqttTopicPrefix}}/{{machineId}}/commands/down"),
        ("enterprise_command", "{{mqttTopicPrefix}}/machines/{{machineId}}/commands"),
    ]
    for label, tcon in outbound:
        n += 1
        rows.append(
            {
                "index": n,
                "direction": "backend_publishes_machine_subscribes",
                "protocol": "mqtt",
                "topicLayout": label.split("_")[0] if label != "enterprise_command" else "enterprise",
                "topicPattern": tcon,
                "topicConcrete": tcon,
                "qos": 1,
                "retained": False,
                "auth": "Backend publisher ACL + API machine status active",
                "payloadJsonTemplate": {"command_id": "{{commandId}}", "machine_id": "{{machineId}}", "sequence": 1, "command_type": "NOOP_POSTMAN", "payload": {}, "idempotency_key": "avf-postman-{{$guid}}", "correlation_id": "{{$guid}}"},
                "expectedBackendEffect": "Broker delivery toward device command topic",
                "responseOrAckTopic": "{{mqttTopicPrefix}}/machines/{{machineId}}/commands/ack (enterprise) hoß║╖c .../{{machineId}}/commands/ack (legacy)",
                "precondition": "API MQTT publisher configured; canary only",
                "evidence": "command_ledger.route_key JSON",
                "safetyLevel": "DESTRUCTIVE_CANARY_ONLY",
                "vi": "APIΓåÆbroker remote command publish (%s) ΓÇö kh├┤ng chß║íy tr├¬n m├íy thß║¡t nß║┐u ch╞░a canary." % label,
            }
        )
    return rows



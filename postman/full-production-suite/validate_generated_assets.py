#!/usr/bin/env python3
"""Validate generated Postman full-production-suite assets."""
from __future__ import annotations

import csv
import hashlib
import json
import re
import subprocess
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
OUT = Path(__file__).resolve().parent
SWAGGER = REPO_ROOT / "docs" / "swagger" / "swagger.json"
HTTP_VERBS = frozenset({"get", "post", "put", "patch", "delete", "options", "head", "trace"})
WRITE_METHODS = frozenset({"POST", "PUT", "PATCH", "DELETE"})
REST_EXPECTED = 327
GRPC_EXPECTED = 86
MQTT_EXPECTED = 28

# File định nghĩa mẫu regex — không quét nội dung (tránh false positive tự khớp).
SKIP_SECRET_SCAN_NAMES = frozenset({"validate_generated_assets.py"})

SECRET_REGEX = [
    (re.compile(r"Bearer\s+eyJ"), "Bearer JWT-like"),
    (re.compile(r"\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b"), "jwt_compact"),
    (re.compile(r"sk_live_"), "stripe_live"),
    (re.compile(r"\bghp_[A-Za-z0-9]{20,}\b"), "github_pat"),
    (re.compile(r"github_pat_[A-Za-z0-9_]{20,}", re.I), "github_pat_prefix"),
    (re.compile(r"whsec_[A-Za-z0-9]{20,}"), "webhook_secret_like"),
    (re.compile(r"password123", re.I), "weak_password_literal"),
    (re.compile(r"-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY-----"), "private_key_pem"),
    (re.compile(r"\bpostgresql://[^:]+:[^@\s]+@"), "database_url_with_embedded_password"),
    (re.compile(r"METRICS_SCRAPE_TOKEN\s*=\s*[^\s\"']{8,}", re.I), "metrics_scrape_token_assignment"),
]

REQUIRED_ENV_KEYS = frozenset(
    {
        "baseUrl",
        "grpcHost",
        "grpcPort",
        "mqttHost",
        "mqttPort",
        "mqttUsername",
        "mqttPassword",
        "mqttTopicPrefix",
        "adminEmail",
        "adminPassword",
        "accessToken",
        "refreshToken",
        "siteId",
        "machineId",
        "machineToken",
        "activationCode",
        "productId",
        "sku",
        "slotIndex",
        "orderId",
        "paymentSessionId",
        "operatorSessionId",
        "commandId",
        "webhookSecret",
        "allow_destructive",
        "readiness",
        "canaryMode",
        "canary_machine_id",
        "canary_operator_id",
        "canary_product_id",
        "canary_slot_index",
        "canary_site_id",
        "idempotencyKey",
        "requestId",
        "correlationId",
    }
)

try:
    import generate_full_postman_suite as _gfs

    FULL100_ENV_KEYS = frozenset(x["key"] for x in _gfs.build_full100_environment_values())
except Exception:
    FULL100_ENV_KEYS = frozenset()


def count_idempotency_headers(collection: dict) -> int:
    n = 0

    def walk(entry_list):
        nonlocal n
        for it in entry_list or []:
            if not isinstance(it, dict):
                continue
            if "item" in it:
                walk(it["item"])
            elif "request" in it:
                req = it.get("request") or {}
                for h in req.get("header") or []:
                    if isinstance(h, dict) and h.get("key") == "Idempotency-Key":
                        n += 1
                        break

    walk(collection.get("item", []))
    return n


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


def swagger_path_to_postman_path(swagger_path: str) -> str:
    def repl(m):
        return "{{" + param_to_var(m.group(1)) + "}}"

    return re.sub(r"\{([^}]+)\}", repl, swagger_path)


def request_url_raw(req: dict) -> str:
    u = req.get("url")
    if isinstance(u, str):
        return u.strip()
    if isinstance(u, dict):
        return (u.get("raw") or "").strip()
    return ""


def collect_all_request_items(collection: dict) -> list[dict]:
    out: list[dict] = []

    def walk(entry_list):
        for it in entry_list or []:
            if not isinstance(it, dict):
                continue
            if "item" in it:
                walk(it["item"])
            elif "request" in it:
                out.append(it)

    walk(collection.get("item", []))
    return out


def openapi_operations_index(spec: dict) -> dict[str, tuple[str, str]]:
    idx: dict[str, tuple[str, str]] = {}
    for path, item in (spec.get("paths") or {}).items():
        for method, op in item.items():
            if method.lower() not in HTTP_VERBS or not isinstance(op, dict):
                continue
            oid = op.get("operationId")
            if isinstance(oid, str) and oid:
                idx[oid] = (method.upper(), path)
    return idx


def count_openapi_ops(spec: dict) -> int:
    n = 0
    for path, item in (spec.get("paths") or {}).items():
        for method, op in item.items():
            if method.lower() in HTTP_VERBS and isinstance(op, dict):
                n += 1
    return n


def openapi_operation_ids(spec: dict) -> set[str]:
    ids: set[str] = set()
    for path, item in (spec.get("paths") or {}).items():
        for method, op in item.items():
            if method.lower() in HTTP_VERBS and isinstance(op, dict):
                oid = op.get("operationId")
                if isinstance(oid, str) and oid:
                    ids.add(oid)
    return ids


def openapi_operation_metadata(spec: dict) -> dict[str, tuple[str, str, str, str]]:
    """operationId -> (METHOD, path, tags_sorted_join, summary)."""
    meta: dict[str, tuple[str, str, str, str]] = {}
    for path, item in (spec.get("paths") or {}).items():
        for method, op in item.items():
            if method.lower() not in HTTP_VERBS or not isinstance(op, dict):
                continue
            oid = op.get("operationId")
            if not isinstance(oid, str) or not oid:
                continue
            tags = op.get("tags") or []
            tag_s = ";".join(tags)
            summ = (op.get("summary") or "").strip()
            meta[oid] = (method.upper(), path, tag_s, summ)
    return meta


def validate_rest_matrix_csv_swagger(spec: dict, csv_path: Path) -> list[str]:
    """CSV matrix method/path/tag/summary must match swagger per operationId."""
    bad: list[str] = []
    meta = openapi_operation_metadata(spec)
    with csv_path.open(encoding="utf-8", newline="") as f:
        r = csv.DictReader(f)
        rows = list(r)
    exp = count_openapi_ops(spec)
    if len(rows) != exp:
        bad.append("rest matrix CSV row count %s != openapi operations %s" % (len(rows), exp))
    seen_ids: set[str] = set()
    for row in rows:
        oid = (row.get("operationId") or "").strip()
        if not oid:
            bad.append("csv row missing operationId")
            continue
        if oid in seen_ids:
            bad.append("csv duplicate operationId %s" % oid)
        seen_ids.add(oid)
        if oid not in meta:
            bad.append("csv operationId not in swagger: %s" % oid)
            continue
        sm, sp, tg, su = meta[oid]
        if (row.get("method") or "").strip() != sm:
            bad.append("rest_matrix method mismatch for %s" % oid)
        if (row.get("path") or "").strip() != sp:
            bad.append("rest_matrix path mismatch for %s" % oid)
        if (row.get("tag") or "").strip() != tg:
            bad.append("rest_matrix tag mismatch for %s" % oid)
        if (row.get("summary") or "").strip() != su:
            bad.append("rest_matrix summary mismatch for %s" % oid)
    if len(seen_ids) != exp:
        bad.append("csv unique operationId count %s != openapi %s" % (len(seen_ids), exp))
    return bad


def iter_collection_leaf_requests(collection: dict):
    """Yield (folder_name, item) for each Postman request item."""

    def walk(entry_list, folder: str):
        for it in entry_list or []:
            if not isinstance(it, dict):
                continue
            name = it.get("name") or ""
            if "item" in it:
                yield from walk(it["item"], name if name else folder)
            elif "request" in it:
                yield folder, it

    yield from walk(collection.get("item", []), "")


def openapi_operation_id_from_item(item: dict) -> str | None:
    desc = (item.get("description") or "").strip()
    m = re.match(r"^openapiOperationId:\s*(\S+)", desc)
    return m.group(1) if m else None


def validate_collection_urls(spec: dict, collection: dict) -> list[str]:
    bad: list[str] = []
    all_items = collect_all_request_items(collection)
    if len(all_items) != REST_EXPECTED:
        bad.append("postman items with request object %s != %s" % (len(all_items), REST_EXPECTED))

    op_ix = openapi_operations_index(spec)
    seen: set[str] = set()
    single_brace = re.compile(r"(?<!\{)\{(?!\{)")

    for it in all_items:
        nm = it.get("name") or "?"
        req = it.get("request") or {}
        method = (req.get("method") or "").strip().upper()
        if not method:
            bad.append("url_audit: %s missing HTTP method" % nm)

        raw = request_url_raw(req)
        if not raw:
            bad.append("url_audit: %s empty url" % nm)
            continue
        if raw == "Enter URL or paste text":
            bad.append("url_audit: %s Postman placeholder url" % nm)
        if not raw.startswith("{{baseUrl}}"):
            bad.append("url_audit: %s url must start with {{baseUrl}}" % nm)
        if method == "GET" and raw.replace("{{baseUrl}}", "").strip("/") == "":
            bad.append("url_audit: %s GET url must not be only {{baseUrl}}" % nm)
        if single_brace.search(raw):
            bad.append("url_audit: %s url contains single-brace {param}" % nm)

        url_obj = req.get("url")
        if not isinstance(url_obj, dict):
            bad.append("url_audit: %s url must be object (raw/host/path)" % nm)
        else:
            if url_obj.get("host") != ["{{baseUrl}}"]:
                bad.append("url_audit: %s url.host must be [\"{{baseUrl}}\"]" % nm)
            path_arr = url_obj.get("path")
            if not isinstance(path_arr, list) or not path_arr:
                bad.append("url_audit: %s url.path must be non-empty list" % nm)

        oid = openapi_operation_id_from_item(it)
        if not oid:
            bad.append("url_audit: %s missing openapiOperationId on item" % nm)
            continue
        if oid not in op_ix:
            bad.append("url_audit: unknown operationId %s" % oid)
            continue
        exp_m, exp_p = op_ix[oid]
        if method != exp_m:
            bad.append("url_audit: %s method %s != swagger %s" % (oid, method, exp_m))
        path_only = raw[len("{{baseUrl}}") :] if raw.startswith("{{baseUrl}}") else ""
        path_only = path_only.split("?", 1)[0]
        exp_path = swagger_path_to_postman_path(exp_p)
        if path_only != exp_path:
            bad.append("url_audit: %s path %r != expected %r" % (oid, path_only, exp_path))
        seen.add(oid)

    missing = set(op_ix.keys()) - seen
    if missing:
        bad.append("url_audit: swagger ops missing in collection (first 15): %s" % ", ".join(sorted(missing)[:15]))
    if seen - set(op_ix.keys()):
        bad.append("url_audit: extra operationIds in collection vs swagger")

    return bad


def request_item_by_operation_id(collection: dict, op_id: str) -> dict | None:
    for _folder, it in iter_collection_leaf_requests(collection):
        if openapi_operation_id_from_item(it) == op_id:
            return it
    return None


def test_script_exec(item: dict) -> str:
    parts: list[str] = []
    for ev in item.get("event") or []:
        if ev.get("listen") != "test":
            continue
        scr = ev.get("script") or {}
        lines = scr.get("exec") or []
        if isinstance(lines, list):
            parts.append("\n".join(lines))
    return "\n".join(parts)


def validate_auth_login_and_me(collection: dict) -> list[str]:
    bad: list[str] = []
    login = request_item_by_operation_id(collection, "DocOpV1AuthLogin")
    if not login:
        bad.append("collection missing DocOpV1AuthLogin")
    else:
        ts = test_script_exec(login)
        if "accessToken" not in ts or "refreshToken" not in ts:
            bad.append("login test script must capture accessToken and refreshToken")
        if 'pm.environment.set("accessToken"' not in ts and "pm.environment.set('accessToken'" not in ts:
            bad.append("login test script must pm.environment.set accessToken")
        if 'pm.environment.set("refreshToken"' not in ts and "pm.environment.set('refreshToken'" not in ts:
            bad.append("login test script must pm.environment.set refreshToken")
        req_lo = login.get("request") or {}
        body_lo = req_lo.get("body") or {}
        raw_lo = (body_lo.get("raw") or "").strip()
        for sub in ("{{adminEmail}}", "{{adminPassword}}"):
            if sub not in raw_lo:
                bad.append("DocOpV1AuthLogin body must include %s" % sub)
    me = request_item_by_operation_id(collection, "DocOpV1AuthMe")
    if not me:
        bad.append("collection missing DocOpV1AuthMe")
    else:
        req = me.get("request") or {}
        if (req.get("method") or "").upper() != "GET":
            bad.append("DocOpV1AuthMe must be GET")
        body = req.get("body")
        if body:
            if isinstance(body, dict) and body.get("mode") == "raw":
                raw = (body.get("raw") or "").strip()
                if raw and raw not in ("{}", '""', "null"):
                    bad.append("DocOpV1AuthMe must not have non-empty body")
            elif isinstance(body, dict):
                bad.append("DocOpV1AuthMe must not define a non-raw body")
        authz = None
        for h in req.get("header") or []:
            if isinstance(h, dict) and (h.get("key") or "") == "Authorization":
                authz = h.get("value") or ""
                break
        if authz != "Bearer {{accessToken}}":
            bad.append("DocOpV1AuthMe must have Authorization: Bearer {{accessToken}}")
    return bad


def openapi_json_body_operation_ids(spec: dict) -> set[str]:
    ids: set[str] = set()
    for path, item in (spec.get("paths") or {}).items():
        for method, op in item.items():
            if method.lower() not in HTTP_VERBS or not isinstance(op, dict):
                continue
            oid = op.get("operationId")
            if not isinstance(oid, str) or not oid:
                continue
            rb = op.get("requestBody")
            if not rb:
                continue
            content = rb.get("content") or {}
            for mk in sorted(content.keys(), key=lambda k: (0 if k == "application/json" else 1, k)):
                if mk == "application/json" or mk.endswith("+json"):
                    mo = content.get(mk)
                    if isinstance(mo, dict):
                        ids.add(oid)
                    break
    return ids


def validate_json_request_bodies(spec: dict, collection: dict) -> list[str]:
    bad: list[str] = []
    for oid in sorted(openapi_json_body_operation_ids(spec)):
        it = request_item_by_operation_id(collection, oid)
        if not it:
            bad.append("json_request_body: missing collection item %s" % oid)
            continue
        req = it.get("request") or {}
        body = req.get("body")
        if not isinstance(body, dict):
            bad.append("json_request_body: %s missing body object" % oid)
            continue
        if body.get("mode") != "raw":
            bad.append("json_request_body: %s body.mode must be raw" % oid)
            continue
        opts = body.get("options") or {}
        if (opts.get("raw") or {}).get("language") != "json":
            bad.append("json_request_body: %s body.options.raw.language must be json" % oid)
        raw = (body.get("raw") or "").strip()
        if not raw or raw in ("{}", "null", '""'):
            bad.append("json_request_body: %s empty raw JSON" % oid)
            continue
        try:
            json.loads(raw)
        except json.JSONDecodeError:
            bad.append("json_request_body: %s raw is not valid JSON" % oid)
        ct_ok = False
        for h in req.get("header") or []:
            if not isinstance(h, dict):
                continue
            if (h.get("key") or "").strip().lower() == "content-type":
                v = (h.get("value") or "").strip().lower().split(";")[0].strip()
                if v == "application/json":
                    ct_ok = True
                break
        if not ct_ok:
            bad.append("json_request_body: %s missing Content-Type application/json" % oid)
    return bad


def resolve_parameter(spec: dict, par: dict) -> dict | None:
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
    for par in iter_resolved_parameters(spec, op):
        if par.get("in") == "header" and par.get("name") == "Idempotency-Key":
            return True
    return False


def openapi_idempotency_op_count(spec: dict) -> int:
    """Đếm operation có khai báo Idempotency-Key **và** method là ghi (GET có ref vẫn bỏ qua)."""
    n = 0
    for path, item in (spec.get("paths") or {}).items():
        for method, op in item.items():
            mu = method.upper()
            if method.lower() not in HTTP_VERBS or not isinstance(op, dict):
                continue
            if mu not in WRITE_METHODS:
                continue
            if operation_needs_idempotency_key(spec, op):
                n += 1
    return n


def count_postman_requests(collection: dict) -> int:
    return len(collect_all_request_items(collection))


def collection_operation_ids(collection: dict) -> set[str]:
    found: set[str] = set()

    def walk(entry_list):
        for it in entry_list or []:
            if not isinstance(it, dict):
                continue
            if "item" in it:
                walk(it["item"])
            elif "request" in it:
                desc = it.get("description") or ""
                m = re.match(r"^openapiOperationId:\s*(\S+)", desc.strip())
                if m:
                    found.add(m.group(1))

    walk(collection.get("item", []))
    return found


def prerequest_exec(coll_item: dict) -> str:
    for ev in coll_item.get("event") or []:
        if ev.get("listen") != "prerequest":
            continue
        scr = ev.get("script") or {}
        lines = scr.get("exec") or []
        if isinstance(lines, list):
            return "\n".join(lines)
    return ""


def validate_write_gates(collection: dict) -> list[str]:
    bad: list[str] = []

    def walk(entry_list, folder: str):
        for it in entry_list or []:
            if not isinstance(it, dict):
                continue
            name = it.get("name") or ""
            if "item" in it:
                walk(it["item"], name if name else folder)
            elif "request" in it:
                req = it["request"]
                if isinstance(req, dict):
                    m = (req.get("method") or "").upper()
                    if m in WRITE_METHODS:
                        pre = prerequest_exec(it)
                        if "AUTH_PUBLIC_WRITE" not in pre and "[GATED]" not in pre:
                            bad.append(
                                "write_gate: missing GATED throw or AUTH_PUBLIC_WRITE in folder %s / %s"
                                % (folder, it.get("name") or "?")
                            )

    walk(collection.get("item", []), "")
    return bad


def duplicate_request_names_same_folder(collection: dict) -> list[str]:
    dups: list[str] = []

    def walk_folder(entry_list):
        for it in entry_list or []:
            if not isinstance(it, dict):
                continue
            name = it.get("name") or ""
            if "item" in it:
                children = it["item"]
                names = []
                for ch in children or []:
                    if isinstance(ch, dict) and "request" in ch:
                        names.append(ch.get("name") or "")
                seen: set[str] = set()
                for nm in names:
                    if nm in seen:
                        dups.append("duplicate request name in folder %r: %r" % (name, nm))
                    seen.add(nm)
                walk_folder(children)

    walk_folder(collection.get("item", []))
    return dups


def csv_operation_ids(csv_path: Path) -> set[str]:
    with csv_path.open(encoding="utf-8", newline="") as f:
        r = csv.DictReader(f)
        if "operationId" not in (r.fieldnames or []):
            return set()
        return {row["operationId"] for row in r if row.get("operationId")}


def scan_secrets() -> list[str]:
    hits = []
    exts = {".md", ".json", ".csv", ".proto", ".txt", ".py", ".sh"}
    for p in OUT.rglob("*"):
        if not p.is_file():
            continue
        if p.suffix.lower() not in exts:
            continue
        if p.name in SKIP_SECRET_SCAN_NAMES:
            continue
        if p.name in ("avf_full_postman_suite.zip", "avf_full_100_postman_suite.zip"):
            continue
        try:
            rel = p.relative_to(OUT)
        except ValueError:
            continue
        if rel.parts and rel.parts[0] == "avf_full_postman_suite":
            continue
        try:
            text = p.read_text(encoding="utf-8", errors="replace")
        except OSError:
            continue
        lines = text.splitlines()
        for i, line in enumerate(lines):
            if "secret-scan-allowline" in line:
                continue
            for rx, label in SECRET_REGEX:
                if rx.search(line):
                    hits.append("%s:%s pattern %s" % (p.relative_to(OUT), i + 1, label))
    return hits


def git_head() -> str:
    try:
        p = subprocess.run(
            ["git", "rev-parse", "HEAD"],
            cwd=str(REPO_ROOT),
            capture_output=True,
            text=True,
            check=False,
        )
        return (p.stdout or "").strip()
    except OSError:
        return ""


def main() -> int:
    fails: list[str] = []

    if (OUT / "avf_full_postman_suite").is_dir():
        fails.append(
            "suite_pollution: remove directory postman/full-production-suite/avf_full_postman_suite/ "
            "(do not unzip avf_full_postman_suite.zip inside the generated suite folder — duplicates artifacts)"
        )

    spec = json.loads(SWAGGER.read_text(encoding="utf-8"))
    ops = count_openapi_ops(spec)
    op_ids_sw = openapi_operation_ids(spec)

    coll = json.loads((OUT / "AVF_REST_365_FULL.postman_collection.json").read_text(encoding="utf-8"))
    req_n = count_postman_requests(coll)
    op_ids_coll = collection_operation_ids(coll)
    op_ids_csv = csv_operation_ids(OUT / "AVF_REST_365_OPERATION_MATRIX.csv")

    if ops != REST_EXPECTED:
        fails.append("openapi operations %s != expected %s" % (ops, REST_EXPECTED))
    if req_n != REST_EXPECTED:
        fails.append("postman requests %s != expected %s" % (req_n, REST_EXPECTED))

    if coll.get("info", {}).get("schema") != "https://schema.getpostman.com/json/collection/v2.1.0/collection.json":
        fails.append("collection schema is not Postman v2.1")

    env = json.loads((OUT / "AVF_PRODUCTION.postman_environment.json").read_text(encoding="utf-8"))
    env_keys = {v.get("key") for v in (env.get("values") or []) if isinstance(v, dict)}
    missing_env = sorted(REQUIRED_ENV_KEYS - env_keys)
    if missing_env:
        fails.append("environment missing keys: %s" % ", ".join(missing_env))

    for v in env.get("values") or []:
        if not isinstance(v, dict):
            continue
        if v.get("key") == "allow_destructive" and v.get("value") != "false":
            fails.append("allow_destructive must default to false in generated env")
            break

    for v in env.get("values") or []:
        if not isinstance(v, dict):
            continue
        if v.get("key") == "readiness" and v.get("value") != "false":
            fails.append("readiness must default to false in generated env")
            break

    for v in env.get("values") or []:
        if not isinstance(v, dict):
            continue
        if v.get("key") == "canaryMode" and v.get("value") != "false":
            fails.append("canaryMode must default to false in generated env")
            break

    grpc = json.loads((OUT / "grpc" / "grpc_request_templates.json").read_text(encoding="utf-8"))
    mqtt = json.loads((OUT / "mqtt" / "mqtt_request_templates.json").read_text(encoding="utf-8"))

    if len(grpc) != GRPC_EXPECTED:
        fails.append("grpc templates %s != expected %s" % (len(grpc), GRPC_EXPECTED))
    if len(mqtt) != MQTT_EXPECTED:
        fails.append("mqtt templates %s != expected %s" % (len(mqtt), MQTT_EXPECTED))

    man = json.loads((OUT / "manifest.json").read_text(encoding="utf-8"))

    head = git_head()
    if head and man.get("gitCommit") != head:
        fails.append("manifest gitCommit %s != current HEAD %s" % (man.get("gitCommit"), head))

    if ops != man.get("restActualCount"):
        fails.append("openapi count %s != manifest restActualCount %s" % (ops, man.get("restActualCount")))
    if req_n != man.get("restCollectionRequestCount"):
        fails.append("collection requests %s != manifest restCollectionRequestCount %s" % (req_n, man.get("restCollectionRequestCount")))
    if len(grpc) != man.get("grpcActualCount"):
        fails.append("grpc templates %s != manifest grpcActualCount %s" % (len(grpc), man.get("grpcActualCount")))
    if len(mqtt) != man.get("mqttActualCount"):
        fails.append("mqtt templates %s != manifest mqttActualCount %s" % (len(mqtt), man.get("mqttActualCount")))

    if ops != req_n:
        fails.append("openapi ops %s != postman requests %s" % (ops, req_n))

    if op_ids_sw != op_ids_coll:
        fails.append(
            "operationId set mismatch openapi vs collection (symmetric diff size %s)"
            % len(op_ids_sw ^ op_ids_coll)
        )
    if op_ids_sw != op_ids_csv:
        fails.append(
            "operationId set mismatch openapi vs CSV (symmetric diff size %s)" % len(op_ids_sw ^ op_ids_csv)
        )

    fails.extend(validate_rest_matrix_csv_swagger(spec, OUT / "AVF_REST_365_OPERATION_MATRIX.csv"))
    fails.extend(validate_auth_login_and_me(coll))
    fails.extend(validate_json_request_bodies(spec, coll))
    fails.extend(validate_collection_urls(spec, coll))

    idem_sw = openapi_idempotency_op_count(spec)
    idem_coll = count_idempotency_headers(coll)
    if idem_sw != idem_coll:
        fails.append("Idempotency-Key header count openapi %s != collection %s" % (idem_sw, idem_coll))

    coll100_path = OUT / "AVF_FULL_100.postman_collection.json"
    env100_path = OUT / "AVF_FULL_100.postman_environment.json"
    mat100_path = OUT / "AVF_FULL_100_OPERATION_MATRIX.csv"
    if coll100_path.is_file():
        coll100 = json.loads(coll100_path.read_text(encoding="utf-8"))
        req100 = count_postman_requests(coll100)
        if req100 != REST_EXPECTED:
            fails.append("AVF_FULL_100 postman requests %s != expected %s" % (req100, REST_EXPECTED))
        if coll100.get("info", {}).get("schema") != "https://schema.getpostman.com/json/collection/v2.1.0/collection.json":
            fails.append("AVF_FULL_100 collection schema is not Postman v2.1")
        if openapi_operation_ids(spec) != collection_operation_ids(coll100):
            fails.append("AVF_FULL_100 operationId set mismatch vs openapi")
        fails.extend(validate_collection_urls(spec, coll100))
        fails.extend(validate_json_request_bodies(spec, coll100))
        fails.extend(validate_auth_login_and_me(coll100))
        fails.extend(validate_write_gates(coll100))
        fails.extend(duplicate_request_names_same_folder(coll100))
        id100 = count_idempotency_headers(coll100)
        if idem_sw != id100:
            fails.append("AVF_FULL_100 Idempotency-Key header openapi %s != collection %s" % (idem_sw, id100))
    else:
        fails.append("AVF_FULL_100.postman_collection.json missing")

    if mat100_path.is_file():
        fails.extend(validate_rest_matrix_csv_swagger(spec, mat100_path))
    else:
        fails.append("AVF_FULL_100_OPERATION_MATRIX.csv missing")

    if env100_path.is_file():
        env100 = json.loads(env100_path.read_text(encoding="utf-8"))
        env_keys100 = {v.get("key") for v in (env100.get("values") or []) if isinstance(v, dict)}
        if FULL100_ENV_KEYS:
            missing_full = sorted(FULL100_ENV_KEYS - env_keys100)
            if missing_full:
                fails.append("AVF_FULL_100 environment missing keys: %s" % ", ".join(missing_full))
        for flag_k in ("allow_destructive", "readiness", "canaryMode"):
            found_bad = False
            for v in env100.get("values") or []:
                if not isinstance(v, dict):
                    continue
                if v.get("key") == flag_k and str(v.get("value")).lower() != "false":
                    found_bad = True
                    break
            if found_bad:
                fails.append("AVF_FULL_100 env `%s` must default false in repo template" % flag_k)
    else:
        fails.append("AVF_FULL_100.postman_environment.json missing")

    fm = [t.get("fullMethod") for t in grpc if isinstance(t, dict)]
    if len(fm) != len(set(fm)):
        fails.append("grpc_request_templates.json duplicate fullMethod")

    fails.extend(validate_write_gates(coll))
    fails.extend(duplicate_request_names_same_folder(coll))

    if "values" not in env:
        fails.append("environment missing values")

    for h in scan_secrets():
        fails.append("secret_scan: %s" % h)

    manifests_paths = {x.get("path") for x in (man.get("filesGenerated") or []) if isinstance(x, dict)}
    for rel in [
        "AVF_REST_365_FULL.postman_collection.json",
        "AVF_PRODUCTION.postman_environment.json",
        "AVF_REST_365_OPERATION_MATRIX.csv",
        "AVF_REST_365_OPERATION_MATRIX.md",
        "grpc/AVF_GRPC_86_METHOD_MATRIX.csv",
        "grpc/AVF_GRPC_86_METHOD_MATRIX.md",
        "grpc/grpc_request_templates.json",
        "grpc/avf_all_services.proto",
        "mqtt/AVF_MQTT_28_TOPIC_FLOW_MATRIX.csv",
        "mqtt/AVF_MQTT_28_TOPIC_FLOW_MATRIX.md",
        "mqtt/mqtt_request_templates.json",
        "AVF_FULL_100.postman_collection.json",
        "AVF_FULL_100.postman_environment.json",
        "AVF_FULL_100_OPERATION_MATRIX.csv",
        "AVF_FULL_100_OPERATION_MATRIX.md",
        "README_IMPORT_AND_RUN_VI.md",
        "05_PRODUCTION_TEST_EXECUTION_ORDER.md",
        "REST_COVERAGE_AUDIT.md",
        "POSTMAN_IMPORT_VALIDATION_REPORT.md",
        "grpc/AVF_GRPC_100_REQUESTS.json",
        "grpc/AVF_GRPC_100_METHOD_MATRIX.csv",
        "grpc/README_GRPC_TESTS.md",
        "grpc/run-grpc-postman-adjacent.sh",
        "mqtt/AVF_MQTT_100_PAYLOADS.json",
        "mqtt/AVF_MQTT_100_TOPIC_MATRIX.csv",
        "mqtt/README_MQTT_TESTS.md",
        "mqtt/run-mqtt-postman-adjacent.sh",
    ]:
        if rel not in manifests_paths:
            fails.append("manifest filesGenerated missing %s" % rel)

    for entry in man.get("filesGenerated") or []:
        if not isinstance(entry, dict):
            continue
        rel = entry.get("path")
        hx = entry.get("sha256")
        if not rel or not hx:
            fails.append("manifest hash entry incomplete for %s" % rel)
            continue
        fp = OUT.joinpath(*rel.split("/"))
        if not fp.is_file():
            fails.append("hashed file missing on disk %s" % rel)
            continue
        digest = hashlib.sha256(fp.read_bytes()).hexdigest()
        if digest != hx:
            fails.append("sha256 mismatch for %s (manifest stale — regenerate)" % rel)

    if fails:
        print("VALIDATION_FAIL")
        for f in fails:
            print(" ", f)
        return 1

    print("VALIDATION_PASS")
    print("openapi_operations:", ops)
    print("postman_requests:", req_n)
    print("grpc_templates:", len(grpc))
    print("mqtt_templates:", len(mqtt))
    print("manifest_finalStatus:", man.get("finalStatus"))
    print("openapi_idempotency_ops:", idem_sw)
    return 0


if __name__ == "__main__":
    sys.exit(main())

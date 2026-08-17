#!/usr/bin/env python3
"""Gate 0: OpenAPI ⊆ Postman REST, registered gRPC (incl. RuntimeSession), MQTT=28. FAIL CLOSED."""
from __future__ import annotations

import json
import re
import sys
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

from gfs_import import REPO_ROOT, gfs  # noqa: E402

SWAGGER = REPO_ROOT / "docs" / "swagger" / "swagger.json"
COLLECTION = (
    REPO_ROOT
    / "postman"
    / "suites"
    / "production-full"
    / "avf-vending-production.full.postman_collection.json"
)
EVIDENCE_DIR = REPO_ROOT / "evidence"
HTTP_VERBS = frozenset({"get", "post", "put", "patch", "delete", "options", "head", "trace"})
GRPC_MQTT_DOC_PREFIXES = ("30 ", "31 ", "32 ", "33 ", "34 ", "35 ")


def skeleton_path(path: str) -> str:
    p = path.split("?", 1)[0]
    p = re.sub(r"\{\{baseUrl\}\}", "", p, flags=re.I)
    p = re.sub(r"https?://[^/]+", "", p)
    if not p.startswith("/"):
        p = "/" + p.lstrip("/")
    p = re.sub(r"\{\{[^}]+\}\}", "{}", p)
    p = re.sub(r"\{[^}]+\}", "{}", p)
    p = re.sub(r":[A-Za-z_][\w]*", "{}", p)
    p = re.sub(r"/+", "/", p)
    if p != "/" and p.endswith("/"):
        p = p[:-1]
    return p


def openapi_ops() -> set[tuple[str, str]]:
    spec = json.loads(SWAGGER.read_text(encoding="utf-8"))
    ops: set[tuple[str, str]] = set()
    for path, item in (spec.get("paths") or {}).items():
        if not isinstance(item, dict):
            continue
        for method, op in item.items():
            if method.lower() not in HTTP_VERBS or not isinstance(op, dict):
                continue
            ops.add((method.upper(), skeleton_path(path)))
    return ops


def walk_items(items, folder_prefix: str = "") -> list[tuple[str, dict, str]]:
    out: list[tuple[str, dict, str]] = []
    for it in items or []:
        name = str(it.get("name") or "")
        prefix = folder_prefix or name
        if "item" in it:
            out.extend(walk_items(it["item"], prefix if folder_prefix else name))
        elif "request" in it:
            out.append((prefix, it, name))
    return out


def postman_rest_ops() -> tuple[set[tuple[str, str]], int]:
    coll = json.loads(COLLECTION.read_text(encoding="utf-8"))
    keys: set[tuple[str, str]] = set()
    leaf = 0
    for folder, it, _name in walk_items(coll.get("item") or []):
        leaf += 1
        top = folder.split(" - ")[0] if folder else ""
        if top.startswith(GRPC_MQTT_DOC_PREFIXES) or top.startswith("30") or "gRPC" in folder or "MQTT" in folder:
            if re.match(r"^3[0-5]\b", top) or folder.startswith(GRPC_MQTT_DOC_PREFIXES):
                continue
        req = it.get("request") or {}
        method = (req.get("method") or "").upper()
        if method not in {v.upper() for v in HTTP_VERBS}:
            continue
        url = req.get("url")
        raw = ""
        path_segs: list[str] = []
        if isinstance(url, dict):
            raw = str(url.get("raw") or "")
            segs = url.get("path") or []
            if isinstance(segs, list):
                path_segs = [str(s) for s in segs]
        else:
            raw = str(url or "")
        if path_segs:
            path = "/" + "/".join(path_segs)
        else:
            raw = raw.replace("{{baseUrl}}", "").replace("{{base_url}}", "")
            path = raw.split("?", 1)[0]
            if "://" in path:
                path = "/" + path.split("/", 3)[-1] if path.count("/") >= 3 else path
        keys.add((method, skeleton_path(path)))
    return keys, leaf


def main() -> int:
    errors: list[str] = []
    swagger_ops = openapi_ops()
    postman_keys, postman_leaf = postman_rest_ops()
    missing = sorted(swagger_ops - postman_keys)
    extra_note = len(postman_keys)

    grpc_rows = gfs.parse_all_protos()
    mqtt_rows = gfs.fix_mqtt_rows()
    registered = [r for r in grpc_rows if r.get("registeredOnListener")]
    runtime = [
        r
        for r in grpc_rows
        if r.get("service") == "MachineRuntimeSessionService"
    ]
    runtime_unreg = [r for r in runtime if not r.get("registeredOnListener")]
    avf_v1_reg = [
        r
        for r in grpc_rows
        if r.get("package") == "avf.v1" and r.get("registeredOnListener")
    ]

    rest_n = len(swagger_ops)
    grpc_n = len(grpc_rows)
    mqtt_n = len(mqtt_rows)

    if rest_n != gfs.REST_EXPECTED:
        errors.append("REST_EXPECTED=%s but swagger ops=%s — update constant" % (gfs.REST_EXPECTED, rest_n))
    if grpc_n != gfs.GRPC_EXPECTED:
        errors.append("GRPC_EXPECTED=%s but proto rows=%s — update constant" % (gfs.GRPC_EXPECTED, grpc_n))
    if mqtt_n != gfs.MQTT_EXPECTED:
        errors.append("MQTT_EXPECTED=%s but mqtt rows=%s" % (gfs.MQTT_EXPECTED, mqtt_n))
    if missing:
        errors.append("OpenAPI ops missing from Postman REST: %s" % len(missing))
        for m, p in missing[:40]:
            errors.append("  missing %s %s" % (m, p))
    if runtime_unreg:
        errors.append(
            "MachineRuntimeSessionService registeredOnListener=false for %s RPC(s)" % len(runtime_unreg)
        )
    if not runtime:
        errors.append("MachineRuntimeSessionService missing from proto inventory")
    if avf_v1_reg:
        errors.append("avf.v1 RPCs marked registeredOnListener=true (must be not_registered): %s" % len(avf_v1_reg))
    if mqtt_n != 28:
        errors.append("fix_mqtt_rows()=%s expected 28" % mqtt_n)

    EVIDENCE_DIR.mkdir(parents=True, exist_ok=True)
    counts = (
        "swagger_ops=%s\n"
        "postman_rest_skeletons=%s\n"
        "postman_leaf_requests=%s\n"
        "openapi_missing_in_postman_rest=%s\n"
        "grpc_rows=%s\n"
        "grpc_registered=%s\n"
        "runtime_session_rpcs=%s\n"
        "runtime_session_unregistered=%s\n"
        "mqtt_rows=%s\n"
        "REST_EXPECTED=%s\n"
        "GRPC_EXPECTED=%s\n"
        "MQTT_EXPECTED=%s\n"
        % (
            rest_n,
            extra_note,
            postman_leaf,
            len(missing),
            grpc_n,
            len(registered),
            len(runtime),
            len(runtime_unreg),
            mqtt_n,
            gfs.REST_EXPECTED,
            gfs.GRPC_EXPECTED,
            gfs.MQTT_EXPECTED,
        )
    )
    (EVIDENCE_DIR / "inventory-counts.txt").write_text(counts, encoding="utf-8")
    diff = {
        "missing_openapi_in_postman_rest": [{"method": m, "path": p} for m, p in missing],
        "runtime_session_unregistered": [r.get("fullMethod") for r in runtime_unreg],
        "avf_v1_registered": [r.get("fullMethod") for r in avf_v1_reg],
    }
    (EVIDENCE_DIR / "openapi-postman-rest-diff.json").write_text(
        json.dumps(diff, indent=2) + "\n", encoding="utf-8"
    )
    print(counts)
    if errors:
        print("GATE0 FAIL")
        for e in errors:
            print(" -", e)
        return 1
    print("GATE0 PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

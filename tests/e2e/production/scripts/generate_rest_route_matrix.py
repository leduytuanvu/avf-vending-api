#!/usr/bin/env python3
"""Generate production REST route matrix from live swagger + repo OpenAPI + E2E manifest.

Outputs:
  tests/e2e/production/generated/rest-route-matrix.json
  docs/testing/production-e2e/rest-route-coverage.md
  tests/e2e/production/e2e-manifest-rest-coverage.yaml (auto flows for uncovered routes)

Usage:
  python tests/e2e/production/scripts/generate_rest_route_matrix.py
  python tests/e2e/production/scripts/generate_rest_route_matrix.py --fetch-swagger
  python tests/e2e/production/scripts/generate_rest_route_matrix.py --validate-only
"""
from __future__ import annotations

import argparse
import json
import os
import re
import sys
import urllib.error
import urllib.request
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError:
    print("ERROR: PyYAML required", file=sys.stderr)
    raise SystemExit(2)

REPO_ROOT = Path(__file__).resolve().parents[4]
PROD_E2E = REPO_ROOT / "tests" / "e2e" / "production"
MANIFEST_MAIN = PROD_E2E / "e2e-manifest.yaml"
MANIFEST_COV = PROD_E2E / "e2e-manifest-rest-coverage.yaml"
OVERRIDES = PROD_E2E / "rest-route-overrides.yaml"
OUT_JSON = PROD_E2E / "generated" / "rest-route-matrix.json"
OUT_MD = REPO_ROOT / "docs" / "testing" / "production-e2e" / "rest-route-coverage.md"
REPO_SWAGGER = REPO_ROOT / "docs" / "swagger" / "swagger.json"
CACHE_SWAGGER = REPO_ROOT / ".tmp-swagger-prod.json"
POSTMAN_COLL = REPO_ROOT / "postman" / "production" / "avf-production-e2e.postman_collection.json"

METHODS = frozenset({"get", "post", "put", "patch", "delete"})
COVERAGE_KINDS = frozenset(
    {
        "success",
        "readonly_smoke",
        "auth_negative",
        "permission_negative",
        "documented_skip",
    }
)

PARAM_RE = re.compile(r"\{([^}]+)\}")
TEMPLATE_RE = re.compile(r"\{\{([^}]+)\}\}")


@dataclass
class RouteKey:
    method: str
    path: str  # OpenAPI template e.g. /v1/admin/machines/{machineId}

    def key(self) -> str:
        return f"{self.method.upper()} {self.path}"


@dataclass
class CoverageEntry:
    method: str
    path: str
    coverage: str
    flow_ids: list[str] = field(default_factory=list)
    skip_reason: str | None = None
    postman: bool = True
    non_postman: bool = False
    non_postman_reason: str | None = None
    openapi_summary: str = ""
    openapi_tags: list[str] = field(default_factory=list)
    source: str = "auto"
    expected_status: int | list[int] | None = None
    auth: str | None = None


def fetch_swagger(url: str, dest: Path) -> dict[str, Any]:
    req = urllib.request.Request(url, headers={"Accept": "application/json"})
    with urllib.request.urlopen(req, timeout=30) as resp:
        raw = resp.read()
    dest.write_bytes(raw)
    return json.loads(raw.decode("utf-8"))


def load_yaml(path: Path) -> dict[str, Any]:
    if not path.exists():
        return {}
    return yaml.safe_load(path.read_text(encoding="utf-8")) or {}


def normalize_manifest_path(path: str) -> str:
    p = path.split("?", 1)[0]
    p = TEMPLATE_RE.sub(lambda m: "{" + _camel_to_openapi_param(m.group(1)) + "}", p)
    return p


def _camel_to_openapi_param(name: str) -> str:
    mapping = {
        "machineId": "machineId",
        "productId": "productId",
        "orderId": "orderId",
        "paymentId": "paymentId",
        "categoryId": "categoryId",
        "brandId": "brandId",
        "siteId": "siteId",
        "mediaId": "mediaId",
        "tagId": "tagId",
        "commandId": "commandId",
        "run_prefix": "runPrefix",
        "run_id": "runId",
    }
    return mapping.get(name, name)


def openapi_path_to_manifest(path: str, param_map: dict[str, str]) -> str:
    def repl(m: re.Match[str]) -> str:
        p = m.group(1)
        state = param_map.get(p)
        if state:
            return "{{" + state + "}}"
        return "{{" + p + "}}"

    return PARAM_RE.sub(repl, path)


def walk_manifest_flows(obj: Any, out: list[dict[str, Any]]) -> None:
    if isinstance(obj, dict):
        if obj.get("method") and obj.get("path"):
            out.append(obj)
        for v in obj.values():
            walk_manifest_flows(v, out)
    elif isinstance(obj, list):
        for item in obj:
            walk_manifest_flows(item, out)


def collect_manifest_routes(manifest_path: Path) -> dict[str, list[dict[str, Any]]]:
    manifest = load_yaml(manifest_path)
    flows: list[dict[str, Any]] = []
    walk_manifest_flows(manifest.get("flows") or [], flows)
    by_key: dict[str, list[dict[str, Any]]] = {}
    for f in flows:
        if f.get("protocol") not in (None, "rest"):
            continue
        method = str(f.get("method", "")).upper()
        raw_path = str(f.get("path", ""))
        path = normalize_manifest_path(raw_path)
        if not method or not path:
            continue
        by_key.setdefault(f"{method} {path}", []).append(f)
    return by_key


def infer_manifest_coverage(flow: dict[str, Any]) -> str:
    phase = str(flow.get("phase") or "")
    fid = str(flow.get("id") or "")
    auth = str(flow.get("auth") or "none")
    if phase == "preflight":
        return "readonly_smoke"
    if phase == "negative" or fid in ("REST-AUTH-003", "REST-AUTH-004"):
        if auth in ("webhook_hmac_invalid", "webhook_hmac_stale"):
            return "auth_negative"
        return "auth_negative"
    if auth == "bearer_machine" and phase not in ("provisioning", "commerce"):
        return "permission_negative"
    return "success"


def iter_swagger_ops(doc: dict[str, Any]) -> list[tuple[str, str, dict[str, Any]]]:
    repo_doc = {}
    if REPO_SWAGGER.exists():
        repo_doc = json.loads(REPO_SWAGGER.read_text(encoding="utf-8"))
    repo_paths = repo_doc.get("paths") or {}
    out: list[tuple[str, str, dict[str, Any]]] = []
    for path, item in sorted((doc.get("paths") or {}).items()):
        if not isinstance(item, dict):
            continue
        repo_item = repo_paths.get(path) or {}
        for method, op in item.items():
            m = method.lower()
            if m not in METHODS or not isinstance(op, dict):
                continue
            repo_op = repo_item.get(method) or {}
            merged = dict(op)
            if not merged.get("summary") and repo_op.get("summary"):
                merged["summary"] = repo_op["summary"]
            if not merged.get("description") and repo_op.get("description"):
                merged["description"] = repo_op["description"]
            if not merged.get("tags") and repo_op.get("tags"):
                merged["tags"] = repo_op["tags"]
            out.append((m.upper(), path, merged))
    return out


def load_postman_routes(coll_path: Path) -> set[str]:
    if not coll_path.exists():
        return set()
    coll = json.loads(coll_path.read_text(encoding="utf-8"))
    routes: set[str] = set()

    def walk(items: list[Any]) -> None:
        for it in items or []:
            if "item" in it:
                walk(it["item"])
            elif "request" in it:
                req = it["request"]
                method = str(req.get("method", "")).upper()
                url = req.get("url")
                if isinstance(url, dict):
                    parts = url.get("path") or []
                    path = "/" + "/".join(str(p) for p in parts)
                    path = TEMPLATE_RE.sub(lambda m: "{" + m.group(1) + "}", path)
                    routes.add(f"{method} {path}")

    walk(coll.get("item") or [])
    return routes


def match_pattern(rule: dict[str, Any], method: str, path: str) -> bool:
    methods = rule.get("methods")
    if methods and method.upper() not in [str(x).upper() for x in methods]:
        return False
    if rule.get("path") and rule["path"] != path:
        return False
    prefix = rule.get("path_prefix")
    if prefix and not path.startswith(prefix):
        return False
    regex = rule.get("path_regex")
    if regex and not re.match(regex, path):
        return False
    return bool(rule.get("path") or prefix or regex)


def find_override(
    overrides: dict[str, Any], method: str, path: str
) -> dict[str, Any] | None:
    for rule in overrides.get("routes") or []:
        if str(rule.get("method", "")).upper() == method.upper() and rule.get("path") == path:
            return rule
    for rule in overrides.get("skip_patterns") or []:
        if match_pattern(rule, method, path):
            return rule
    return None


def path_params(path: str) -> list[str]:
    return PARAM_RE.findall(path)


def all_params_mappable(path: str, param_map: dict[str, str]) -> bool:
    params = path_params(path)
    return bool(params) and all(p in param_map for p in params)


def auto_coverage(
    method: str,
    path: str,
    overrides: dict[str, Any],
) -> CoverageEntry:
    param_map = overrides.get("param_state_map") or {}
    ov = find_override(overrides, method, path)
    if ov:
        cov = str(ov.get("coverage", "documented_skip"))
        return CoverageEntry(
            method=method,
            path=path,
            coverage=cov,
            skip_reason=ov.get("skip_reason"),
            postman=bool(ov.get("postman", cov != "documented_skip")),
            non_postman=bool(ov.get("non_postman", cov == "documented_skip")),
            non_postman_reason=ov.get("non_postman_reason") or ov.get("skip_reason"),
            source="override",
            expected_status=ov.get("expected_status"),
            auth=ov.get("auth"),
        )

    params = path_params(path)
    unmapped = [p for p in params if p not in param_map]

    if method == "GET":
        if not params:
            return CoverageEntry(
                method=method,
                path=path,
                coverage="readonly_smoke",
                auth="bearer_admin",
                expected_status=[200, 404],
                source="auto",
            )
        if all_params_mappable(path, param_map):
            return CoverageEntry(
                method=method,
                path=path,
                coverage="readonly_smoke",
                auth="bearer_admin",
                expected_status=[200, 404],
                source="auto",
            )
        if path.startswith("/v1/machines/") or path.startswith("/v1/device/"):
            if unmapped:
                return CoverageEntry(
                    method=method,
                    path=path,
                    coverage="auth_negative",
                    auth="none",
                    expected_status=401,
                    source="auto",
                )
            return CoverageEntry(
                method=method,
                path=path,
                coverage="readonly_smoke",
                auth="bearer_machine",
                expected_status=[200, 403, 404],
                source="auto",
            )

    if method in ("POST", "PUT", "PATCH", "DELETE"):
        if unmapped:
            policy = overrides.get("unmapped_param_policy", "auth_negative")
            if policy == "auth_negative":
                return CoverageEntry(
                    method=method,
                    path=path,
                    coverage="auth_negative",
                    auth="none",
                    expected_status=401,
                    source="auto",
                )
            return CoverageEntry(
                method=method,
                path=path,
                coverage="documented_skip",
                skip_reason=f"Unmapped path params {unmapped}; no E2E state ID — destructive if guessed",
                postman=False,
                non_postman=True,
                source="auto",
            )
        if method == "DELETE" or method in ("PUT", "PATCH"):
            return CoverageEntry(
                method=method,
                path=path,
                coverage="auth_negative",
                auth="none",
                expected_status=401,
                source="auto",
            )
        return CoverageEntry(
            method=method,
            path=path,
            coverage="auth_negative",
            auth="none",
            expected_status=401,
            source="auto",
        )

    return CoverageEntry(
        method=method,
        path=path,
        coverage="documented_skip",
        skip_reason="No auto rule matched",
        postman=False,
        non_postman=True,
        source="auto",
    )


def build_flow_from_entry(entry: CoverageEntry, overrides: dict[str, Any], idx: int) -> dict[str, Any] | None:
    if entry.coverage == "documented_skip":
        return None
    if entry.flow_ids:
        return None
    param_map = overrides.get("param_state_map") or {}
    manifest_path = openapi_path_to_manifest(entry.path, param_map)
    if entry.coverage == "auth_negative":
        manifest_path = PARAM_RE.sub(
            lambda m: "00000000-0000-4000-8000-000000000099"
            if m.group(1) not in param_map
            else "{{" + param_map[m.group(1)] + "}}",
            entry.path,
        )
    if "?" not in manifest_path and entry.coverage == "readonly_smoke" and entry.method == "GET":
        if not path_params(entry.path):
            manifest_path += "?limit=1"

    fid = f"REST-COV-{entry.method[:3]}-{idx:04d}"
    auth = entry.auth or ("none" if entry.coverage == "auth_negative" else "bearer_admin")
    if entry.path.startswith("/v1/machines/") and entry.coverage == "readonly_smoke":
        auth = entry.auth or "bearer_machine"

    exp = entry.expected_status
    if exp is None:
        exp = 401 if entry.coverage == "auth_negative" else 200
    optional = entry.coverage in ("readonly_smoke", "auth_negative", "permission_negative")

    flow: dict[str, Any] = {
        "id": fid,
        "label": f"{entry.method} {entry.path} ({entry.coverage})",
        "phase": "rest-coverage",
        "protocol": "rest",
        "evidence_label": f"rest-cov-{idx:04d}",
        "auth": auth,
        "method": entry.method,
        "path": manifest_path,
        "request_template": {},
        "expected_status": exp,
        "optional": optional,
        "cleanup": "none",
        "route_matrix": {
            "coverage": entry.coverage,
            "openapi_path": entry.path,
        },
    }
    if entry.method in ("POST", "PUT", "PATCH") and entry.coverage == "auth_negative":
        flow["request_template"] = {"probe": "auth-negative"}
    return flow


def generate_coverage_manifest(
    entries: list[CoverageEntry], overrides: dict[str, Any]
) -> dict[str, Any]:
    flows: list[dict[str, Any]] = []
    idx = 0
    for entry in sorted(entries, key=lambda e: e.key() if hasattr(e, "key") else f"{e.method} {e.path}"):
        if entry.flow_ids or entry.coverage == "documented_skip":
            continue
        idx += 1
        f = build_flow_from_entry(entry, overrides, idx)
        if f:
            flows.append(f)
            entry.flow_ids.append(f["id"])
    return {
        "version": 1,
        "name": "avf-production-rest-coverage",
        "description": "AUTO-GENERATED readonly/auth-negative REST coverage — do not edit by hand; regenerate via generate_rest_route_matrix.py",
        "flows": flows,
    }


def entry_key(method: str, path: str) -> str:
    return f"{method.upper()} {path}"


def match_openapi_key(
    method: str, concrete_path: str, swagger_ops: list[tuple[str, str, dict[str, Any]]]
) -> str | None:
    """Map a concrete request path (probe UUIDs, {{vars}} resolved) to OpenAPI template key."""
    concrete_path = concrete_path.split("?", 1)[0]
    if not concrete_path.startswith("/"):
        concrete_path = "/" + concrete_path
    cparts = [p for p in concrete_path.split("/") if p]
    for m, p, _ in swagger_ops:
        if m.upper() != method.upper():
            continue
        sparts = [x for x in p.split("/") if x]
        if len(sparts) != len(cparts):
            continue
        matched = True
        for a, b in zip(sparts, cparts):
            if a.startswith("{") and a.endswith("}"):
                continue
            if a != b:
                matched = False
                break
        if matched:
            return entry_key(m, p)
    return None


def postman_route_to_openapi_key(
    route: str,
    param_map: dict[str, str],
    swagger_ops: list[tuple[str, str, dict[str, Any]]] | None = None,
) -> str:
    """Convert Postman METHOD /path to OpenAPI METHOD /path/{template}."""
    parts = route.split(" ", 1)
    if len(parts) != 2:
        return route
    method, path = parts[0].upper(), parts[1]
    path = path.split("?", 1)[0]
    inv = {v: k for k, v in param_map.items()}
    inv.setdefault("activationCode", "activationCodeId")

    def repl(m: re.Match[str]) -> str:
        var = m.group(1)
        return "{" + inv.get(var, var) + "}"

    templated = TEMPLATE_RE.sub(repl, path)
    if swagger_ops:
        hit = match_openapi_key(method, templated, swagger_ops)
        if hit:
            return hit
        hit = match_openapi_key(method, path, swagger_ops)
        if hit:
            return hit
    return entry_key(method, templated)


def load_online_payment_exclude_profile() -> dict[str, Any]:
    if os.environ.get("PROD_E2E_EXCLUDE_ONLINE_PAYMENT") != "1":
        return {}
    profile_path = PROD_E2E / "suite-profiles.yaml"
    if not profile_path.is_file():
        return {}
    data = load_yaml(profile_path)
    return (data.get("profiles") or {}).get("all-no-online-payment") or {}


def apply_online_payment_exclusions(entries: dict[str, CoverageEntry]) -> None:
    cfg = load_online_payment_exclude_profile()
    if not cfg:
        return
    skip_reason = str(
        cfg.get("skip_reason") or "excluded by operator request: no online payment test"
    )
    exclude_ids = {str(x) for x in (cfg.get("exclude_flow_ids") or [])}
    subs = [str(x) for x in (cfg.get("exclude_coverage_path_substrings") or []) if x]
    for ent in entries.values():
        if exclude_ids.intersection(ent.flow_ids):
            ent.coverage = "documented_skip"
            ent.skip_reason = skip_reason
            ent.postman = False
            ent.non_postman = True
            ent.non_postman_reason = skip_reason
            continue
        hay = f"{ent.path} {ent.method}"
        if any(sub in hay for sub in subs):
            ent.coverage = "documented_skip"
            ent.skip_reason = skip_reason
            ent.postman = False
            ent.non_postman = True
            ent.non_postman_reason = skip_reason


def validate_matrix(
    entries: dict[str, CoverageEntry],
    swagger_ops: list[tuple[str, str, dict[str, Any]]],
    postman_routes: set[str],
    param_map: dict[str, str],
    *,
    check_postman: bool = True,
) -> list[str]:
    errors: list[str] = []
    swagger_keys = {entry_key(m, p) for m, p, _ in swagger_ops}

    missing = swagger_keys - set(entries.keys())
    manifest_only = {k for k, e in entries.items() if e.source == "manifest_only"}
    extra = set(entries.keys()) - swagger_keys - manifest_only
    if missing:
        errors.append(
            f"uncovered swagger routes ({len(missing)}): " + ", ".join(sorted(missing)[:20])
        )
    if extra:
        errors.append(
            f"matrix routes not in swagger ({len(extra)}): " + ", ".join(sorted(extra)[:10])
        )

    for key, ent in entries.items():
        if ent.coverage not in COVERAGE_KINDS:
            errors.append(f"{key}: invalid coverage kind {ent.coverage}")
        if ent.coverage == "documented_skip" and not ent.skip_reason:
            errors.append(f"{key}: documented_skip requires skip_reason")
        if ent.coverage != "documented_skip" and not ent.flow_ids:
            errors.append(f"{key}: non-skip route has no manifest flow_ids")

    if not check_postman:
        return errors

    postman_openapi_keys: set[str] = set()
    for pr in postman_routes:
        postman_openapi_keys.add(postman_route_to_openapi_key(pr, param_map, swagger_ops))

    for pr in postman_routes:
        ok = postman_route_to_openapi_key(pr, param_map, swagger_ops)
        ent = entries.get(ok)
        if not ent:
            errors.append(f"Postman route not in matrix/swagger: {pr} (-> {ok})")
            continue
        if ent.coverage == "documented_skip":
            errors.append(f"Postman route {pr} is documented_skip in matrix")
        if ent.non_postman:
            errors.append(f"Postman route {pr} marked non_postman but present in Postman")

    for key, ent in entries.items():
        if ent.coverage == "documented_skip" or ent.non_postman or not ent.postman:
            continue
        if key not in postman_openapi_keys:
            errors.append(f"Tested route missing from Postman: {key}")

    return errors


def write_markdown(
    entries: dict[str, CoverageEntry],
    swagger_ops: list[tuple[str, str, dict[str, Any]]],
    errors: list[str],
    generated_at: str,
    swagger_source: str,
) -> None:
    by_cov: dict[str, int] = {}
    for e in entries.values():
        by_cov[e.coverage] = by_cov.get(e.coverage, 0) + 1

    lines = [
        "# Production REST route coverage",
        "",
        f"- **Generated:** `{generated_at}`",
        f"- **Swagger source:** `{swagger_source}`",
        f"- **Total routes:** {len(entries)}",
        f"- **Coverage buckets:** `{by_cov}`",
        f"- **Matrix JSON:** [`tests/e2e/production/generated/rest-route-matrix.json`](../../../tests/e2e/production/generated/rest-route-matrix.json)",
        "",
    ]
    if errors:
        lines.extend(["## Validation errors", ""])
        for err in errors:
            lines.append(f"- {err}")
        lines.append("")

    lines.extend(
        [
            "## Coverage rules",
            "",
            "Every production `method+path` from `/swagger/doc.json` must map to exactly one of:",
            "",
            "| Kind | Meaning |",
            "|------|---------|",
            "| `success` | Happy path with E2E-PROD resources (main manifest) |",
            "| `readonly_smoke` | GET/list probe with admin or machine token |",
            "| `auth_negative` | Unauthenticated or invalid auth → 401/403/422 |",
            "| `permission_negative` | Wrong principal → 403 |",
            "| `documented_skip` | Not live-tested; reason required |",
            "",
            "Regenerate: `python tests/e2e/production/scripts/generate_rest_route_matrix.py --fetch-swagger`",
            "",
            "## Route index (sample — full list in JSON)",
            "",
            "| Method | Path | Coverage | Flows | Postman | Skip reason |",
            "|--------|------|----------|-------|---------|-------------|",
        ]
    )
    for m, p, op in swagger_ops[:60]:
        ent = entries.get(entry_key(m, p))
        if not ent:
            continue
        flows = ", ".join(ent.flow_ids[:2]) or "—"
        skip = (ent.skip_reason or "")[:60]
        lines.append(
            f"| {m} | `{p}` | {ent.coverage} | {flows} | {ent.postman} | {skip} |"
        )
    if len(swagger_ops) > 60:
        lines.append(f"| … | *{len(swagger_ops) - 60} more* | | | | |")

    OUT_MD.parent.mkdir(parents=True, exist_ok=True)
    OUT_MD.write_text("\n".join(lines) + "\n", encoding="utf-8")


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--fetch-swagger", action="store_true")
    ap.add_argument("--swagger-url", default="https://api.ldtv.dev/swagger/doc.json")
    ap.add_argument("--swagger-file", type=Path, default=CACHE_SWAGGER)
    ap.add_argument("--validate-only", action="store_true")
    ap.add_argument("--no-write-manifest", action="store_true")
    ap.add_argument("--skip-postman-check", action="store_true")
    args = ap.parse_args()

    if args.fetch_swagger:
        print(f"Fetching {args.swagger_url} …")
        doc = fetch_swagger(args.swagger_url, args.swagger_file)
        swagger_source = args.swagger_url
    elif args.swagger_file.exists():
        doc = json.loads(args.swagger_file.read_text(encoding="utf-8"))
        swagger_source = str(args.swagger_file.relative_to(REPO_ROOT))
    elif REPO_SWAGGER.exists():
        doc = json.loads(REPO_SWAGGER.read_text(encoding="utf-8"))
        swagger_source = str(REPO_SWAGGER.relative_to(REPO_ROOT))
    else:
        print("No swagger source; use --fetch-swagger", file=sys.stderr)
        return 2

    overrides = load_yaml(OVERRIDES)
    manifest_main = collect_manifest_routes(MANIFEST_MAIN)
    swagger_ops = iter_swagger_ops(doc)
    entries: dict[str, CoverageEntry] = {}

    for method, path, op in swagger_ops:
        key = entry_key(method, path)
        summ = (op.get("summary") or op.get("description") or "").strip()
        tags = list(op.get("tags") or [])

        if key in manifest_main:
            flows = manifest_main[key]
            cov = infer_manifest_coverage(flows[0])
            has_handler = bool(flows[0].get("handler"))
            ent = CoverageEntry(
                method=method,
                path=path,
                coverage=cov,
                flow_ids=[str(f.get("id")) for f in flows if f.get("id")],
                postman=flows[0].get("phase") not in ("negative",),
                non_postman=flows[0].get("phase") == "negative"
                or (has_handler and flows[0].get("handler") == "media_presigned_upload"),
                non_postman_reason="Negative phase excluded from Postman per manifest postman.exclude_phases"
                if flows[0].get("phase") == "negative"
                else (
                    "Media handler parent — init/complete sub-flows are in Postman"
                    if has_handler and flows[0].get("handler") == "media_presigned_upload"
                    else None
                ),
                openapi_summary=summ[:300],
                openapi_tags=tags,
                source="manifest",
            )
            entries[key] = ent
            continue

        ent = auto_coverage(method, path, overrides)
        ent.postman = False
        ent.non_postman = True
        ent.non_postman_reason = (
            "Auto coverage manifest — exercised by shell route-matrix only; "
            "Postman collection is generated from e2e-manifest.yaml main flows only"
        )
        ent.openapi_summary = summ[:300]
        ent.openapi_tags = tags
        entries[key] = ent

    for key, flows in manifest_main.items():
        if key in entries:
            continue
        parts = key.split(" ", 1)
        if len(parts) != 2:
            continue
        method, path = parts[0], parts[1]
        cov = infer_manifest_coverage(flows[0])
        has_handler = bool(flows[0].get("handler"))
        entries[key] = CoverageEntry(
            method=method,
            path=path,
            coverage=cov,
            flow_ids=[str(f.get("id")) for f in flows if f.get("id")],
            postman=flows[0].get("phase") not in ("negative",),
            non_postman=flows[0].get("phase") == "negative"
            or (has_handler and flows[0].get("handler") == "media_presigned_upload"),
            non_postman_reason="Negative phase excluded from Postman per manifest postman.exclude_phases"
            if flows[0].get("phase") == "negative"
            else (
                "Media handler parent — init/complete sub-flows are in Postman"
                if has_handler and flows[0].get("handler") == "media_presigned_upload"
                else None
            ),
            openapi_summary="Main E2E manifest route (pending OpenAPI sync)",
            source="manifest_only",
        )

    apply_online_payment_exclusions(entries)

    # Attach auto flows
    auto_entries = [e for e in entries.values() if e.source != "manifest" and e.coverage != "documented_skip"]
    cov_manifest = generate_coverage_manifest(auto_entries, overrides)
    if not args.no_write_manifest and not args.validate_only:
        MANIFEST_COV.write_text(
            yaml.safe_dump(cov_manifest, sort_keys=False, allow_unicode=True),
            encoding="utf-8",
        )
        print(f"WROTE {MANIFEST_COV.relative_to(REPO_ROOT)} ({len(cov_manifest['flows'])} flows)")

    generated_at = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    matrix = {
        "version": 1,
        "generated_at": generated_at,
        "swagger_source": swagger_source,
        "total_routes": len(entries),
        "routes": {
            k: {
                "method": v.method,
                "path": v.path,
                "coverage": v.coverage,
                "flow_ids": v.flow_ids,
                "skip_reason": v.skip_reason,
                "postman": v.postman,
                "non_postman": v.non_postman,
                "non_postman_reason": v.non_postman_reason,
                "openapi_summary": v.openapi_summary,
                "openapi_tags": v.openapi_tags,
                "source": v.source,
            }
            for k, v in sorted(entries.items())
        },
    }
    OUT_JSON.parent.mkdir(parents=True, exist_ok=True)
    OUT_JSON.write_text(json.dumps(matrix, indent=2) + "\n", encoding="utf-8")
    print(f"WROTE {OUT_JSON.relative_to(REPO_ROOT)}")

    postman_routes = load_postman_routes(POSTMAN_COLL)
    param_map = overrides.get("param_state_map") or {}
    errors = validate_matrix(
        entries,
        swagger_ops,
        postman_routes,
        param_map,
        check_postman=not args.skip_postman_check,
    )
    write_markdown(entries, swagger_ops, errors, generated_at, swagger_source)

    if errors:
        print("VALIDATION_FAILED:")
        for e in errors:
            print(f"  - {e}")
        return 1

    print(f"ROUTE_MATRIX_OK routes={len(entries)} coverage={ {k: sum(1 for e in entries.values() if e.coverage==k) for k in COVERAGE_KINDS} }")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

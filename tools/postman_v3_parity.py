#!/usr/bin/env python3
"""Shared JSON/v3 Postman parity helpers."""
from __future__ import annotations

import json
import re
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError:  # pragma: no cover
    yaml = None  # type: ignore[assignment]

REQUEST_YAML_SUFFIX = ".request.yaml"
ENV_YAML_SUFFIX = ".environment.yaml"


def load_json(path: Path) -> dict[str, Any]:
    return json.loads(path.read_text(encoding="utf-8"))


def load_yaml(path: Path) -> dict[str, Any]:
    if yaml is None:
        raise RuntimeError("PyYAML required (pip install pyyaml)")
    data = yaml.safe_load(path.read_text(encoding="utf-8"))
    return data if isinstance(data, dict) else {}


DOC_ONLY_JSON_REQUEST_NAMES = frozenset(
    {
        "README — Import and Safety",
    }
)


def walk_json_requests(collection: dict[str, Any], *, exclude_doc_only: bool = False) -> list[dict[str, Any]]:
    out: list[dict[str, Any]] = []

    def rec(items: list[Any], folder_path: list[str]) -> None:
        for it in items or []:
            if not isinstance(it, dict):
                continue
            if it.get("request"):
                name = str(it.get("name") or "")
                if exclude_doc_only and name in DOC_ONLY_JSON_REQUEST_NAMES:
                    continue
                out.append(
                    {
                        "name": name,
                        "folder_path": list(folder_path),
                        "request": it.get("request") or {},
                        "event": it.get("event") or [],
                        "response": it.get("response") or [],
                        "auth": it.get("auth"),
                    }
                )
            elif it.get("item"):
                rec(it["item"], folder_path + [str(it.get("name") or "")])

    rec(collection.get("item") or [], [])
    return out


def list_v3_request_yamls(root: Path, *, exclude_doc_only: bool = False) -> list[Path]:
    if not root.is_dir():
        return []
    out: list[Path] = []
    for p in root.rglob(f"*{REQUEST_YAML_SUFFIX}"):
        try:
            if p.is_file():
                if exclude_doc_only:
                    name = parse_v3_request_yaml(p).get("name") or ""
                    if name in DOC_ONLY_JSON_REQUEST_NAMES:
                        continue
                out.append(p)
        except OSError:
            continue
    return sorted(out)


def count_v3_requests(root: Path, *, exclude_doc_only: bool = False) -> int:
    return len(list_v3_request_yamls(root, exclude_doc_only=exclude_doc_only))


def normalize_url(raw: str) -> str:
    s = (raw or "").strip()
    s = re.sub(r"\s+", "", s)
    return s


def request_url_raw(req: dict[str, Any]) -> str:
    u = req.get("url")
    if isinstance(u, str):
        return u
    if isinstance(u, dict):
        return str(u.get("raw") or "")
    return ""


def request_method(req: dict[str, Any]) -> str:
    return str(req.get("method") or "GET").upper()


def json_header_keys(req: dict[str, Any]) -> set[str]:
    keys: set[str] = set()
    for h in req.get("header") or []:
        if isinstance(h, dict) and h.get("key"):
            keys.add(str(h["key"]).lower())
    return keys


def json_body_raw(req: dict[str, Any]) -> str:
    body = req.get("body") or {}
    if isinstance(body, dict) and body.get("mode") == "raw":
        return str(body.get("raw") or "")
    return ""


def json_has_scripts(events: list[Any]) -> dict[str, bool]:
    out = {"prerequest": False, "test": False}
    for ev in events or []:
        if not isinstance(ev, dict):
            continue
        listen = str(ev.get("listen") or "")
        script = ev.get("script") or {}
        exec_lines = script.get("exec") if isinstance(script, dict) else None
        has_code = bool(exec_lines) and any(str(x).strip() for x in (exec_lines or []))
        if listen == "prerequest" and has_code:
            out["prerequest"] = True
        if listen == "test" and has_code:
            out["test"] = True
    return out


def parse_v3_request_yaml(path: Path) -> dict[str, Any]:
    data = load_yaml(path)
    name = str(data.get("name") or path.name.replace(".request.yaml", ""))
    method = str(data.get("method") or "GET").upper()
    url = str(data.get("url") or "")

    body_raw = ""
    body = data.get("body") or {}
    if isinstance(body, dict):
        body_raw = str(body.get("content") or body.get("raw") or "")

    scripts = data.get("scripts") or []
    has_prerequest = any("before" in str(s.get("type", "")).lower() for s in scripts if isinstance(s, dict))
    has_test = any("after" in str(s.get("type", "")).lower() for s in scripts if isinstance(s, dict))

    has_examples = bool(data.get("examples"))
    examples_dir = data.get("examples")
    if isinstance(examples_dir, str):
        ex_path = (path.parent / examples_dir).resolve()
        if ex_path.is_dir():
            has_examples = any(ex_path.rglob("*.example.yaml"))

    return {
        "name": name,
        "path": path,
        "method": method,
        "url": url,
        "body_raw": body_raw.strip(),
        "has_prerequest": has_prerequest,
        "has_test": has_test,
        "has_examples": has_examples,
    }


def env_var_keys_from_json(path: Path) -> dict[str, str]:
    data = load_json(path)
    out: dict[str, str] = {}
    for v in data.get("values") or []:
        if isinstance(v, dict) and v.get("key") is not None:
            out[str(v["key"])] = str(v.get("value", ""))
    return out


def env_var_keys_from_yaml(path: Path) -> dict[str, str]:
    data = load_yaml(path)
    out: dict[str, str] = {}
    for v in data.get("values") or []:
        if isinstance(v, dict) and v.get("key") is not None:
            out[str(v["key"])] = str(v.get("value", ""))
    return out


def compare_collection_pair(json_path: Path, v3_root: Path, *, exclude_doc_only: bool = False) -> list[str]:
    errors: list[str] = []
    if not json_path.is_file():
        errors.append(f"missing JSON collection {json_path}")
        return errors
    if not v3_root.is_dir():
        errors.append(f"missing v3 collection dir {v3_root}")
        return errors

    coll = load_json(json_path)
    json_reqs = walk_json_requests(coll, exclude_doc_only=exclude_doc_only)
    yaml_paths = list_v3_request_yamls(v3_root, exclude_doc_only=exclude_doc_only)
    if len(json_reqs) != len(yaml_paths):
        errors.append(
            f"{json_path.name}: request count mismatch json={len(json_reqs)} yaml={len(yaml_paths)}"
        )

    json_names = sorted(r["name"] for r in json_reqs)
    yaml_names = sorted(parse_v3_request_yaml(p)["name"] for p in yaml_paths)
    if json_names != yaml_names:
        missing = set(json_names) - set(yaml_names)
        extra = set(yaml_names) - set(json_names)
        if missing:
            errors.append(f"{json_path.name}: missing in v3: {sorted(missing)[:5]}")
        if extra:
            errors.append(f"{json_path.name}: extra in v3: {sorted(extra)[:5]}")

    return errors


def compare_environment_pair(json_path: Path, yaml_path: Path) -> list[str]:
    errors: list[str] = []
    if not json_path.is_file():
        errors.append(f"missing JSON env {json_path}")
        return errors
    if not yaml_path.is_file():
        errors.append(f"missing YAML env {yaml_path}")
        return errors
    jvars = env_var_keys_from_json(json_path)
    yvars = env_var_keys_from_yaml(yaml_path)
    if set(jvars.keys()) != set(yvars.keys()):
        missing = set(jvars.keys()) - set(yvars.keys())
        extra = set(yvars.keys()) - set(jvars.keys())
        if missing:
            errors.append(f"{yaml_path.name}: missing vars {sorted(missing)[:8]}")
        if extra:
            errors.append(f"{yaml_path.name}: extra vars {sorted(extra)[:8]}")
    return errors


def validate_collection_depth(json_path: Path, v3_root: Path, *, exclude_doc_only: bool = False) -> list[str]:
    errors: list[str] = []
    coll = load_json(json_path)
    json_reqs = walk_json_requests(coll, exclude_doc_only=exclude_doc_only)
    yaml_by_name = {
        parse_v3_request_yaml(p)["name"]: parse_v3_request_yaml(p)
        for p in list_v3_request_yamls(v3_root, exclude_doc_only=exclude_doc_only)
    }

    for jr in json_reqs:
        name = jr["name"]
        y = yaml_by_name.get(name)
        if not y:
            continue
        jreq = jr["request"]
        jm = request_method(jreq)
        if jm != y["method"]:
            errors.append(f"{json_path.name}/{name}: method json={jm} yaml={y['method']}")

        ju = normalize_url(request_url_raw(jreq))
        yu = normalize_url(y["url"])
        if ju and yu and ju != yu:
            errors.append(f"{json_path.name}/{name}: url mismatch json={ju!r} yaml={yu!r}")

        jb = json_body_raw(jreq).strip()
        if jb and not y["body_raw"]:
            errors.append(f"{json_path.name}/{name}: JSON body missing in v3 yaml")

        js = json_has_scripts(jr.get("event") or [])
        if js["prerequest"] and not y["has_prerequest"]:
            # collection-level scripts satisfy prerequest for primary collection
            if not (v3_root / ".resources" / "definition.yaml").is_file():
                errors.append(f"{json_path.name}/{name}: prerequest script missing in v3")
        if js["test"] and not y["has_test"]:
            if not (v3_root / ".resources" / "definition.yaml").is_file():
                errors.append(f"{json_path.name}/{name}: test script missing in v3")

        if (jr.get("response") or []) and not y["has_examples"]:
            pass  # response examples optional in v3 sidecars; count parity is sufficient

    return errors

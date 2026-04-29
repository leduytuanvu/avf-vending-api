#!/usr/bin/env python3
"""Shared OpenAPI 3.x local JSON Reference ($ref) validation.

Used by tools/build_openapi.py (generation) and tools/openapi_verify_release.py (committed spec).

Rules:
  - Every ``$ref`` must either resolve as a local JSON Pointer (``#/...``) in the same document,
    or be reported as unresolved / forbidden external refs.
"""
from __future__ import annotations

from typing import Any


def resolve_local_json_pointer(doc: dict[str, Any], pointer: str) -> bool:
    """Return True if ``pointer`` exists in ``doc`` (RFC6901, including ~0/~1 escapes)."""
    if not pointer.startswith("#/"):
        return False
    cur: Any = doc
    for raw in pointer[2:].split("/"):
        part = raw.replace("~1", "/").replace("~0", "~")
        if not isinstance(cur, dict) or part not in cur:
            return False
        cur = cur[part]
    return True


def unresolved_local_refs(doc: dict[str, Any]) -> list[str]:
    """Collect ``#/`` fragment refs that do not resolve to an existing location in ``doc``."""
    missing: list[str] = []

    def walk(obj: Any, path: str) -> None:
        if isinstance(obj, dict):
            ref = obj.get("$ref")
            if isinstance(ref, str) and ref.startswith("#/") and not resolve_local_json_pointer(doc, ref):
                missing.append(f"{path} -> {ref}")
            for k, v in obj.items():
                walk(v, f"{path}/{k}")
        elif isinstance(obj, list):
            for i, v in enumerate(obj):
                walk(v, f"{path}/{i}")

    walk(doc, "#")
    return missing


def non_local_refs(doc: dict[str, Any]) -> list[tuple[str, str]]:
    """Return ``(json-path, ref-value)`` for every ``$ref`` that is not a same-document fragment."""
    found: list[tuple[str, str]] = []

    def walk(obj: Any, path: str) -> None:
        if isinstance(obj, dict):
            ref = obj.get("$ref")
            if isinstance(ref, str) and not ref.startswith("#/"):
                found.append((path, ref))
            for k, v in obj.items():
                walk(v, f"{path}/{k}")
        elif isinstance(obj, list):
            for i, v in enumerate(obj):
                walk(v, f"{path}/{i}")

    walk(doc, "#")
    return found


def duplicate_operation_ids(paths: dict[str, Any]) -> list[str]:
    """Return human-readable collision lines when two operations share the same ``operationId``."""
    seen: dict[str, str] = {}
    dups: list[str] = []
    for path, methods in paths.items():
        if not isinstance(methods, dict):
            continue
        for method, op_any in methods.items():
            m = str(method).lower()
            if m.startswith("x-"):
                continue
            if not isinstance(op_any, dict):
                continue
            oid_raw = op_any.get("operationId")
            if not isinstance(oid_raw, str):
                continue
            oid = oid_raw.strip()
            if not oid:
                continue
            label = f"{m.upper()} {path}"
            prev = seen.get(oid)
            if prev:
                dups.append(f"duplicate operationId {oid!r}: {prev} vs {label}")
            else:
                seen[oid] = label
    return dups

#!/usr/bin/env python3
"""Map Postman collection requests to docs/testing/e2e-flow-coverage.md; write coverage.json and gate critical gaps."""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path
from typing import Any

CRITICAL_METHODS = frozenset({"POST", "PUT", "PATCH", "DELETE"})


def iter_requests(
    items: list[dict[str, Any]] | None, prefix: str = ""
) -> list[dict[str, str]]:
    out: list[dict[str, str]] = []
    for it in items or []:
        name = (it.get("name") or "").strip()
        full_name = f"{prefix} / {name}" if prefix else name
        if "item" in it:
            out.extend(iter_requests(it["item"], full_name))
            continue
        req = it.get("request")
        if not req:
            continue
        method = (req.get("method") or "GET").upper()
        url = req.get("url")
        raw = ""
        if isinstance(url, str):
            raw = url
        elif isinstance(url, dict):
            raw = url.get("raw") or ""
            if not raw and url.get("path"):
                parts = url["path"]
                if isinstance(parts, list):
                    raw = "/" + "/".join(str(p) for p in parts)
        out.append({"name": full_name, "method": method, "url_raw": raw})
    return out


def normalize_path(url_raw: str) -> str:
    s = url_raw.strip()
    s = re.sub(r"\{\{[^}]+\}\}", "", s)
    s = re.sub(r"^https?://[^/]+", "", s, flags=re.I)
    if not s.startswith("/"):
        s = "/" + s.lstrip("/")
    s = s.split("?", 1)[0].rstrip("/")
    return s if s else "/"


def extract_matrix_paths(matrix_text: str) -> set[str]:
    paths: set[str] = set()
    # Backtick-enclosed fragments in the matrix
    for m in re.finditer(r"`([^`]+)`", matrix_text):
        frag = m.group(1).strip()
        for chunk in re.split(r"[\n;,]+", frag):
            chunk = chunk.strip()
            if not chunk:
                continue
            chunk = re.sub(
                r"^(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\s+",
                "",
                chunk,
                flags=re.I,
            ).strip()
            for cand in re.findall(r"(/[a-zA-Z0-9_\-./{}]*[a-zA-Z0-9_\-{}])(?:\s|$|,)", chunk + " "):
                c = cand.split("?")[0].rstrip("/") or "/"
                if "/v1/" in c or c.startswith("/health") or c.startswith("/version") or c.startswith("/swagger") or c.startswith("/metrics"):
                    paths.add(c)
            # Also whole chunk if it is a path
            if chunk.startswith("/"):
                c = chunk.split("?")[0].rstrip("/") or "/"
                paths.add(c)
    return paths


def parse_exclusions(matrix_text: str) -> list[dict[str, str]]:
    """Parse table under '## Postman / Newman coverage exclusions'."""
    if "## Postman / Newman coverage exclusions" not in matrix_text:
        return []
    tail = matrix_text.split("## Postman / Newman coverage exclusions", 1)[1]
    if "## " in tail[1:]:
        next_h = tail.find("\n## ", 1)
        if next_h != -1:
            tail = tail[:next_h]
    rows: list[dict[str, str]] = []
    for line in tail.splitlines():
        line = line.strip()
        if not line.startswith("|") or line.startswith("|----"):
            continue
        cells = [c.strip() for c in line.split("|")[1:-1]]
        if len(cells) < 3:
            continue
        match, kind, reason = cells[0], cells[1].lower(), cells[2]
        if match.lower() == "match":
            continue
        rows.append({"match": match, "kind": kind, "reason": reason})
    return rows


def path_matches_matrix(norm_path: str, matrix_paths: set[str]) -> bool:
    if norm_path in matrix_paths:
        return True
    for p in matrix_paths:
        if not p:
            continue
        if norm_path.startswith(p) or p.startswith(norm_path):
            return True
        if p in norm_path or norm_path in p:
            return True
    return False


def exclusion_applies(
    exc: dict[str, str], norm_path: str, request_name: str
) -> bool:
    kind = exc["kind"]
    m = exc["match"].strip()
    if kind == "path":
        em = normalize_path(m)
        return norm_path == em or norm_path.startswith(em.rstrip("/"))
    if kind == "prefix":
        return norm_path.startswith(m.rstrip("/"))
    if kind == "name":
        return m.lower() in request_name.lower()
    if kind == "regex":
        try:
            return re.search(m, norm_path) is not None or re.search(
                m, request_name
            ) is not None
        except re.error:
            return False
    return False


def is_critical_method_path(method: str, norm_path: str) -> bool:
    if method not in CRITICAL_METHODS:
        return False
    if norm_path.startswith("/health") or norm_path.startswith("/version"):
        return False
    for prefix in (
        "/v1/admin",
        "/v1/commerce",
        "/v1/setup",
        "/v1/device",
        "/v1/machines",
    ):
        if norm_path.startswith(prefix):
            return True
    return False


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--collection", required=True, type=Path)
    ap.add_argument("--matrix", required=True, type=Path)
    ap.add_argument("--out", required=True, type=Path)
    ap.add_argument(
        "--fail-on-uncovered-critical",
        action="store_true",
        default=True,
        help="Exit 1 if a mutating /v1/admin|commerce|... request is neither covered nor excluded (default: on).",
    )
    ap.add_argument(
        "--no-fail-on-uncovered-critical",
        action="store_false",
        dest="fail_on_uncovered_critical",
    )
    args = ap.parse_args()

    coll = json.loads(args.collection.read_text(encoding="utf-8"))
    matrix_text = args.matrix.read_text(encoding="utf-8")
    matrix_paths = extract_matrix_paths(matrix_text)
    exclusions = parse_exclusions(matrix_text)

    requests = iter_requests(coll.get("item"))
    covered: list[dict[str, Any]] = []
    uncovered: list[dict[str, Any]] = []
    excluded: list[dict[str, Any]] = []
    uncovered_critical: list[dict[str, Any]] = []

    for r in requests:
        norm = normalize_path(r["url_raw"])
        name = r["name"]
        method = r["method"]
        exc_hit: dict[str, str] | None = None
        for exc in exclusions:
            if exclusion_applies(exc, norm, name):
                exc_hit = exc
                break

        cov = path_matches_matrix(norm, matrix_paths)

        entry = {
            "name": name,
            "method": method,
            "path": norm,
            "matched_matrix": cov,
            "exclusion": exc_hit,
        }

        if exc_hit:
            excluded.append(entry)
        elif cov:
            covered.append(entry)
        else:
            uncovered.append(entry)
            if is_critical_method_path(method, norm):
                uncovered_critical.append(entry)

    report = {
        "total_requests": len(requests),
        "covered_requests": len(covered),
        "uncovered_requests": len(uncovered),
        "excluded_requests": len(excluded),
        "covered": covered,
        "uncovered": uncovered,
        "excluded": excluded,
        "uncovered_critical": uncovered_critical,
    }
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")

    if uncovered_critical and args.fail_on_uncovered_critical:
        print(
            "ERROR: uncovered critical Postman requests (mutating /v1/*) — "
            "map them in e2e-flow-coverage.md or add a Postman exclusions row:",
            file=sys.stderr,
        )
        for u in uncovered_critical:
            print(f"  {u['method']} {u['path']}  ({u['name']})", file=sys.stderr)
        return 1

    if uncovered:
        print(
            f"WARN: {len(uncovered)} Postman request(s) not matched to matrix paths "
            f"(non-critical or add exclusions). See {args.out}",
            file=sys.stderr,
        )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

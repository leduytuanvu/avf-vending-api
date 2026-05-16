#!/usr/bin/env python3
"""Normalize legacy Postman sidecar YAML under postman/collections/.

Flatten artifact URLs that still nest artifacts under a legacy ``/v1/admin/companies/{{…}}/artifacts…``
segment into ``/v1/admin/artifacts…``.

Run from repo root (often chained after ``generate_full_postman_suite.py``)::

    python tools/sanitize_postman_sidecar_yamls.py
"""

from __future__ import annotations

import re
import sys
from pathlib import Path


def _repo_root() -> Path:
    p = Path(__file__).resolve()
    for d in [p, *p.parents]:
        if (d / "go.mod").is_file():
            return d
    raise RuntimeError("go.mod not found above %s" % p)


ROOT = _repo_root()
COLLECTIONS = ROOT / "postman" / "collections"

# Matches `/v1/admin/companies/{{any}}/artifacts` plus optional `/{{artifactId}}[/…]`.
_COMPANY_ARTIFACT_POSTMAN = re.compile(
    r"/v1/admin/companies/\{\{[^}]+\}\}/artifacts(?:/\{\{[^}]+\}\}(?:/(?:content|download))?)?"
)


def _replace_company_artifacts(match: re.Match[str]) -> str:
    tail = match.group(0)
    marker = "/artifacts"
    i = tail.find(marker)
    suffix = tail[i + len(marker) :] if i >= 0 else ""
    return "/v1/admin/artifacts" + suffix


# Matches quoted OpenAPI-style `/v1/admin/companies/{param}/artifacts…`.
_COMPANY_ARTIFACT_BRACE = re.compile(
    r"/v1/admin/companies/\{[^}]+\}/artifacts(?:/\{[^}]+\}(?:/(?:content|download))?)?"
)


def _flatten_company_nested_artifacts(text: str) -> str:
    text = _COMPANY_ARTIFACT_POSTMAN.sub(_replace_company_artifacts, text)
    text = _COMPANY_ARTIFACT_BRACE.sub(_replace_company_artifacts, text)
    return text


def sanitize_file(path: Path) -> bool:
    raw = path.read_text(encoding="utf-8")
    text = _flatten_company_nested_artifacts(raw)
    if text != raw:
        path.write_text(text, encoding="utf-8", newline="\n")
        return True
    return False


def main() -> int:
    if not COLLECTIONS.is_dir():
        print("no postman/collections tree", file=sys.stderr)
        return 0
    n = 0
    for p in COLLECTIONS.rglob("*.yaml"):
        if sanitize_file(p):
            n += 1
    for p in COLLECTIONS.rglob("*.yml"):
        if sanitize_file(p):
            n += 1
    print("sanitized", n, "YAML files under postman/collections")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

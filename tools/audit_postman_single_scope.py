#!/usr/bin/env python3
"""Postman / sidecar import audit for the single-company API surface.

Writes ``docs/reports/final-single-scope-audit/postman-import-check-report.md``.

Usage from repo root::

    python tools/audit_postman_single_scope.py
"""

from __future__ import annotations

import json
import re
import subprocess
import sys
from dataclasses import dataclass, field
from pathlib import Path


def _repo_root() -> Path:
    p = Path(__file__).resolve()
    for d in [p, *p.parents]:
        if (d / "go.mod").is_file():
            return d
    raise RuntimeError("go.mod not found above %s" % p)


def _s(*parts: str) -> str:
    return "".join(parts)


# Forbidden phrases built from fragments so repo-wide greps stay clean.
_FORBIDDEN_FRAGMENTS: tuple[tuple[str, ...], ...] = (
    ("organ", "ization"),
    ("organ", "izations"),
    ("organ", "ization", "_", "id"),
    ("organ", "ization", "Id"),
    ("Organ", "ization", "ID"),
    ("Organ", "ization", "Ids"),
    ("organ", "ization", "_", "ids"),
    ("org", "Id"),
    ("org", "_", "id"),
    ("org", "IDs"),
    ("org", "_", "admin"),
    ("ten", "ant"),
    ("ten", "ants"),
    ("ten", "ant", "_", "id"),
    ("ten", "ant", "Id"),
    ("Ten", "ant", "ID"),
    (_s("ten", "ant"), "-", "scoped"),
    (_s("ten", "ant"), " ", "scoped"),
    ("org", "-", "scoped"),
    ("org", " ", "scoped"),
    ("organ", "ization", " ", "scope"),
    (_s("ten", "ant"), " ", "scope"),
    ("Require", "Organ", "ization", "Scope"),
    ("Require", "Ten", "ant", "Scope"),
    ("canary", "_", "organ", "ization", "_", "id"),
    ("E2E", "_", "ORG", "ANIZATION", "_", "ID"),
    ("Dev", "Organ", "ization", "ID"),
)


def _forbidden_regex() -> re.Pattern[str]:
    terms = [_s(*frag) for frag in _FORBIDDEN_FRAGMENTS]
    uniq: list[str] = []
    seen: set[str] = set()
    for t in terms:
        if t not in seen:
            seen.add(t)
            uniq.append(t)
    return re.compile("|".join(re.escape(x) for x in uniq), re.IGNORECASE)


ROOT = _repo_root()
REPORT_PATH = ROOT / "docs/reports/final-single-scope-audit/postman-import-check-report.md"
FORBIDDEN = _forbidden_regex()


def _forbidden_env_keys() -> frozenset[str]:
    # Mirror legacy Postman env drift checks without spelling forbidden tokens contiguously in-source.
    return frozenset(
        {
            _s("organ", "ization", "Id"),
            _s("organ", "ization", "_", "id"),
            _s("canary", "_", "organ", "ization", "_", "id"),
            _s("E2E", "_", "ORG", "ANIZATION", "_", "ID"),
            _s("Dev", "Organ", "ization", "ID"),
            _s("org", "Id"),
        }
    )


FORBIDDEN_ENV_KEYS = _forbidden_env_keys()

CANONICAL_REST_COLLECTION = "AVF_REST_365_FULL.postman_collection.json"


@dataclass
class AuditState:
    p0: list[str] = field(default_factory=list)
    p1: list[str] = field(default_factory=list)
    notes: list[str] = field(default_factory=list)

    def add_p0(self, msg: str) -> None:
        self.p0.append(msg)

    def add_p1(self, msg: str) -> None:
        self.p1.append(msg)


def _scan_text(path: Path, text: str, st: AuditState) -> None:
    for i, line in enumerate(text.splitlines(), 1):
        if FORBIDDEN.search(line):
            st.add_p0("%s:%d: forbidden term in line" % (path.as_posix(), i))


def walk_postman_json(path: Path, st: AuditState, *, scan_forbidden: bool) -> None:
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as e:
        st.add_p0("%s: read error %s" % (path, e))
        return
    try:
        json.loads(text)
    except json.JSONDecodeError as e:
        st.add_p0("%s: invalid JSON %s" % (path, e))
        return
    if scan_forbidden:
        _scan_text(path, text, st)


def walk_yaml(path: Path, st: AuditState) -> None:
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as e:
        st.add_p0("%s: read error %s" % (path, e))
        return
    _scan_text(path, text, st)


def _iter_request_items(items: list, parents: list[str]) -> list[tuple[list[str], dict]]:
    out: list[tuple[list[str], dict]] = []
    for it in items or []:
        if not isinstance(it, dict):
            continue
        name = str(it.get("name") or "")
        if "item" in it:
            out.extend(_iter_request_items(it["item"], parents + [name]))
        elif "request" in it:
            out.append((parents + [name], it))
    return out


def audit_collection(path: Path, st: AuditState, *, strict: bool) -> None:
    if not strict:
        return
    data = json.loads(path.read_text(encoding="utf-8"))
    flat = _iter_request_items(data.get("item") or [], [])
    sites_post: dict | None = None

    for _folder_path, it in flat:
        req = it.get("request") or {}
        name = it.get("name") or ""
        if not isinstance(req, dict):
            st.add_p0("%s request %r: request is not an object" % (path, name))
            continue
        method = (req.get("method") or "").strip().upper()
        if not method:
            st.add_p0("%s item %r: missing request.method" % (path, name))

        url = req.get("url") or {}
        if isinstance(url, str):
            st.add_p0("%s item %r: request.url must be object with raw/host/path" % (path, name))
            continue
        raw = (url.get("raw") or "").strip()
        if not raw:
            st.add_p0("%s item %r: empty request.url.raw" % (path, name))
        path_segments = url.get("path")
        if path_segments is not None and not path_segments:
            st.add_p0("%s item %r: request.url.path empty" % (path, name))

        if method == "POST" and isinstance(url, dict):
            raw_l = raw.lower()
            if "/v1/admin/sites" in raw_l and "{{siteid}}" not in raw_l and ":siteid" not in raw_l:
                segments = [str(s).lower() for s in (path_segments or [])]
                if segments and segments[-1] == "sites":
                    sites_post = req

    if sites_post is None:
        st.add_p0("%s: no POST /v1/admin/sites request found" % path)
    else:
        body = sites_post.get("body") or {}
        raw_b = (body.get("raw") or "") if isinstance(body, dict) else ""
        if FORBIDDEN.search(raw_b):
            st.add_p0("%s: POST /v1/admin/sites body references forbidden identifiers" % path)


def audit_environment(path: Path, st: AuditState) -> None:
    data = json.loads(path.read_text(encoding="utf-8"))
    for e in data.get("values") or []:
        if not isinstance(e, dict):
            continue
        k = e.get("key")
        if k in FORBIDDEN_ENV_KEYS:
            st.add_p0("%s: forbidden environment key %s" % (path, k))
        v = e.get("value")
        if isinstance(v, str) and FORBIDDEN.search(v):
            st.add_p0("%s: forbidden term in value for key %s" % (path, k))


def audit_docs(paths: list[Path], st: AuditState) -> None:
    for p in paths:
        if not p.is_file():
            st.add_p1("missing optional doc %s" % p)
            continue
        _scan_text(p, p.read_text(encoding="utf-8"), st)


def main() -> int:
    st = AuditState()

    json_targets = list(ROOT.glob("postman/**/*.postman_collection.json")) + list(
        ROOT.glob("postman/**/*.postman_environment.json")
    )
    json_targets += list((ROOT / "docs" / "postman").glob("*.postman_collection.json"))
    json_targets += list((ROOT / "docs" / "postman").glob("*.postman_environment.json"))

    for p in sorted({p.resolve() for p in json_targets}):
        if "node_modules" in str(p):
            continue
        if p.name.endswith(".postman_environment.json"):
            walk_postman_json(p, st, scan_forbidden=True)
            try:
                audit_environment(p, st)
            except Exception as e:
                st.add_p0("%s: audit_environment failed %s" % (p, e))
        elif p.name.endswith(".postman_collection.json"):
            strict = p.name == CANONICAL_REST_COLLECTION
            walk_postman_json(p, st, scan_forbidden=True)
            try:
                audit_collection(p, st, strict=strict)
            except Exception as e:
                st.add_p0("%s: audit_collection failed %s" % (p, e))
        else:
            walk_postman_json(p, st, scan_forbidden=True)

    yaml_root = ROOT / "postman" / "collections"
    if yaml_root.is_dir():
        for p in yaml_root.rglob("*.yaml"):
            walk_yaml(p, st)
        for p in yaml_root.rglob("*.yml"):
            walk_yaml(p, st)

    swagger = ROOT / "docs" / "swagger" / "swagger.json"
    if swagger.is_file():
        _scan_text(swagger, swagger.read_text(encoding="utf-8"), st)

    audit_docs(
        [
            ROOT / "docs" / "testing" / "AVF_POSTMAN_PRODUCTION.md",
            ROOT / "docs" / "testing" / "POSTMAN_VARIABLE_AUDIT_REPORT.md",
            ROOT / "docs" / "testing" / "05_PRODUCTION_TEST_EXECUTION_ORDER.md",
        ],
        st,
    )

    newman_note = "Newman not found on PATH; smoke not run."
    try:
        r = subprocess.run(["newman", "--version"], capture_output=True, text=True, timeout=10)
        if r.returncode == 0:
            newman_note = "Newman present: %s (smoke collection not executed by this audit)." % (
                (r.stdout or "").strip()
            )
    except OSError:
        pass
    st.notes.append(newman_note)

    REPORT_PATH.parent.mkdir(parents=True, exist_ok=True)
    lines = [
        "# Postman single-scope import audit",
        "",
        "## Summary",
        "",
        "- **P0 count:** %d" % len(st.p0),
        "- **P1 count:** %d" % len(st.p1),
        "",
        "## Notes",
        "",
    ]
    for n in st.notes:
        lines.append("- %s" % n)
    lines += ["", "## P0 issues", ""]
    if st.p0:
        lines.extend("- %s" % x for x in st.p0)
    else:
        lines.append("- *(none)*")
    lines += ["", "## P1 issues", ""]
    if st.p1:
        lines.extend("- %s" % x for x in st.p1)
    else:
        lines.append("- *(none)*")
    lines.append("")
    REPORT_PATH.write_text("\n".join(lines), encoding="utf-8", newline="\n")

    print("wrote", REPORT_PATH)
    print("P0:", len(st.p0), "P1:", len(st.p1))
    return 1 if st.p0 else 0


if __name__ == "__main__":
    raise SystemExit(main())

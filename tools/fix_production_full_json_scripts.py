#!/usr/bin/env python3
"""Fix production-full Postman JSON scripts for classic Postman Import.

- Collection-level test: replace illegal top-level `return` with `if (j) { ... }`.
- Request-level tests: remove duplicated capture scripts (collection already runs them).
  Login / auth/me keep dedicated small test scripts.
"""
from __future__ import annotations

import json
import re
import subprocess
import sys
import tempfile
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[1]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from scripts.postman.generate_production_full_suite import (  # noqa: E402
    AUTH_ME_TEST_SCRIPT,
    LOGIN_TEST_SCRIPT,
)

EARLY_RETURN_RE = re.compile(
    r"(const j = tryJson\(\);\s*)if \(!j\) \{\s*return;\s*\}",
    re.MULTILINE,
)
NEGATIVE_TEST_MARKER = 'pm.test("expected HTTP success unless negative test"'

MINIMAL_REQUEST_TEST = [
    'pm.test("expected HTTP success unless negative test", function () {',
    "  if (pm.collectionVariables.get('negative_test') === 'true') { return; }",
    "  pm.expect(pm.response.code).to.be.within(200, 299);",
    "});",
]


def request_path_key(req_item: dict) -> str:
    req = req_item.get("request") or {}
    method = (req.get("method") or "").upper()
    url = req.get("url")
    if isinstance(url, dict):
        raw = url.get("raw") or ""
        path = raw.split("?", 1)[0].replace("{{baseUrl}}", "")
    else:
        path = str(url or "")
    return f"{method} {path.strip()}".lower()


def fix_collection_capture_script(code: str) -> str:
    if "tryJson()" not in code or "if (!j)" not in code:
        return code
    if not EARLY_RETURN_RE.search(code):
        return code
    code = EARLY_RETURN_RE.sub(r"\1if (j) {", code, count=1)
    if NEGATIVE_TEST_MARKER in code:
        idx = code.index(NEGATIVE_TEST_MARKER)
        code = code[:idx] + "}\n" + code[idx:]
    else:
        code = code.rstrip() + "\n}\n"
    return code


def request_test_exec(item: dict) -> list[str]:
    path_key = request_path_key(item)
    if "/v1/auth/login" in path_key:
        return list(LOGIN_TEST_SCRIPT)
    if "/v1/auth/me" in path_key:
        return list(AUTH_ME_TEST_SCRIPT)
    return list(MINIMAL_REQUEST_TEST)


def set_test_script(item: dict, exec_lines: list[str]) -> None:
    events = list(item.get("event") or [])
    replaced = False
    for ev in events:
        if ev.get("listen") == "test":
            ev["script"] = {"type": "text/javascript", "exec": exec_lines}
            replaced = True
    if not replaced:
        events.append({"listen": "test", "script": {"type": "text/javascript", "exec": exec_lines}})
    item["event"] = events


def walk_items(items: list[Any]) -> int:
    changed = 0
    for it in items or []:
        if it.get("request"):
            new_exec = request_test_exec(it)
            current = []
            for ev in it.get("event") or []:
                if ev.get("listen") == "test":
                    current = (ev.get("script") or {}).get("exec") or []
            if current != new_exec:
                set_test_script(it, new_exec)
                changed += 1
        elif it.get("item"):
            changed += walk_items(it["item"])
    return changed


def validate_js(code: str) -> str | None:
    with tempfile.NamedTemporaryFile("w", suffix=".js", delete=False, encoding="utf-8") as f:
        f.write(code)
        path = f.name
    try:
        proc = subprocess.run(["node", "--check", path], capture_output=True, text=True)
        if proc.returncode != 0:
            return proc.stderr.strip() or proc.stdout.strip()
        return None
    finally:
        Path(path).unlink(missing_ok=True)


def fix_production_full_collection(data: dict) -> tuple[int, list[str]]:
    errors: list[str] = []
    changed = 0

    for ev in data.get("event") or []:
        if ev.get("listen") != "test":
            continue
        script = ev.get("script") or {}
        exec_lines = script.get("exec") or []
        if not exec_lines:
            continue
        code = "\n".join(exec_lines)
        fixed = fix_collection_capture_script(code)
        if fixed != code:
            ev["script"]["exec"] = fixed.splitlines()
            changed += 1
            code = fixed
        err = validate_js(code)
        if err:
            errors.append(f"collection test script: {err}")

    changed += walk_items(data.get("item") or [])

    return changed, errors


def fix_json_file(path: Path) -> int:
    data = json.loads(path.read_text(encoding="utf-8"))
    changed, errors = fix_production_full_collection(data)
    if errors:
        for err in errors:
            print(f"ERROR: {path.name}: {err}", file=sys.stderr)
        raise RuntimeError(f"script validation failed for {path}")
    if changed:
        path.write_text(json.dumps(data, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    return changed


def main() -> int:
    import argparse

    parser = argparse.ArgumentParser(description="Fix production-full Postman JSON test scripts")
    parser.add_argument(
        "paths",
        nargs="*",
        type=Path,
        help="Collection JSON paths (default: suite source + published copy)",
    )
    args = parser.parse_args()

    paths = args.paths or [
        ROOT / "postman/suites/production-full/avf-vending-production.full.postman_collection.json",
        ROOT / "postman/collections/avf-vending-production-full.postman_collection.json",
    ]

    total = 0
    for path in paths:
        if not path.is_file():
            print(f"WARN: skip missing {path}", file=sys.stderr)
            continue
        n = fix_json_file(path)
        print(f"OK: {path.relative_to(ROOT)} ({n} change(s))")
        total += n
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

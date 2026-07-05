#!/usr/bin/env python3
"""Fix Postman test scripts that use illegal top-level return (breaks Postman v3 Local Mode)."""
from __future__ import annotations

import json
import re
import sys
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError:  # pragma: no cover
    yaml = None  # type: ignore[assignment]

ROOT = Path(__file__).resolve().parents[1]

EARLY_RETURN_RE = re.compile(
    r"(const j = tryJson\(\);\s*)if \(!j\) \{\s*return;\s*\}",
    re.MULTILINE,
)

NEGATIVE_TEST_MARKER = 'pm.test("expected HTTP success unless negative test"'

JSON_COLLECTIONS = [
    ROOT / "postman/collections/avf-vending-api.postman_collection.json",
    ROOT / "postman/collections/avf-vending-api-function-path.postman_collection.json",
    ROOT / "postman/production/avf-production-e2e.postman_collection.json",
    ROOT / "postman/suites/production-full/avf-vending-production.full.postman_collection.json",
]


def fix_early_return_in_script(code: str) -> str:
    """Replace top-level early return after tryJson() with an if (j) { ... } block."""
    if "tryJson()" not in code or "if (!j)" not in code:
        return code
    if not EARLY_RETURN_RE.search(code):
        return code

    fixed = EARLY_RETURN_RE.sub(r"\1if (j) {", code, count=1)
    if NEGATIVE_TEST_MARKER in fixed:
        idx = fixed.index(NEGATIVE_TEST_MARKER)
        fixed = fixed[:idx] + "}\n" + fixed[idx:]
    else:
        fixed = fixed.rstrip() + "\n}\n"
    return fixed


def _walk_json_events(node: Any, *, changed: list[int]) -> None:
    if isinstance(node, dict):
        for ev in node.get("event") or []:
            if ev.get("listen") != "test":
                continue
            script = ev.get("script") or {}
            exec_lines = script.get("exec")
            if not exec_lines:
                continue
            code = "\n".join(exec_lines) if isinstance(exec_lines, list) else str(exec_lines)
            fixed = fix_early_return_in_script(code)
            if fixed != code:
                ev["script"]["exec"] = fixed.splitlines()
                changed[0] += 1
        for value in node.values():
            _walk_json_events(value, changed=changed)
    elif isinstance(node, list):
        for item in node:
            _walk_json_events(item, changed=changed)


def fix_json_collection(path: Path) -> int:
    if not path.is_file():
        return 0
    data = json.loads(path.read_text(encoding="utf-8"))
    changed = [0]
    _walk_json_events(data, changed=changed)
    if changed[0]:
        path.write_text(json.dumps(data, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    return changed[0]


def fix_v3_yaml_file(path: Path) -> int:
    if yaml is None or not path.is_file():
        return 0
    text = path.read_text(encoding="utf-8")
    data = yaml.safe_load(text)
    if not isinstance(data, dict):
        return 0

    new_text = text
    changed = 0
    scripts = data.get("scripts")
    if isinstance(scripts, list):
        for script in scripts:
            if not isinstance(script, dict):
                continue
            code = script.get("code")
            if not isinstance(code, str) or "tryJson()" not in code:
                continue
            fixed = fix_early_return_in_script(code)
            if fixed != code:
                new_text = new_text.replace(code, fixed, 1)
                changed += 1

    if changed:
        path.write_text(new_text, encoding="utf-8", newline="\n")
    return changed


def fix_all_json_collections(paths: list[Path] | None = None) -> int:
    total = 0
    for path in paths or JSON_COLLECTIONS:
        n = fix_json_collection(path)
        if n:
            print(f"fixed {n} test script(s) in {path.relative_to(ROOT)}")
        total += n
    return total


def fix_all_v3_yaml(v3_root: Path | None = None) -> int:
    root = v3_root or (ROOT / "postman" / "v3")
    if not root.is_dir():
        return 0
    total = 0
    for ypath in root.rglob("*.yaml"):
        if ".resources" in ypath.parts and ypath.name == "definition.yaml":
            n = fix_v3_yaml_file(ypath)
            if n:
                print(f"fixed collection script in {ypath.relative_to(ROOT)}")
            total += n
        elif ypath.name.endswith(".request.yaml"):
            n = fix_v3_yaml_file(ypath)
            if n:
                total += n
    if total:
        print(f"fixed {total} v3 YAML script block(s) under {root.relative_to(ROOT)}")
    return total


def main() -> int:
    import argparse

    parser = argparse.ArgumentParser(description="Fix illegal top-level return in Postman scripts")
    parser.add_argument("--json-only", action="store_true")
    parser.add_argument("--v3-only", action="store_true")
    args = parser.parse_args()

    total = 0
    if not args.v3_only:
        total += fix_all_json_collections()
    if not args.json_only:
        total += fix_all_v3_yaml()
    print(f"OK: {total} script block(s) updated")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

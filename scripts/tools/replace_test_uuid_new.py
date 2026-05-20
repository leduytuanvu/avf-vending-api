#!/usr/bin/env python3
"""One-off: replace uuid.New() with id.NewUUIDV7() in Go test files."""
from __future__ import annotations

import os
import re
import sys

ID_IMPORT = "github.com/avf/avf-vending-api/internal/platform/id"
SKIP_DIRS = {"tools", "vendor", ".git", "node_modules"}


def should_process(path: str) -> bool:
    if not path.endswith(".go"):
        return False
    if "_test.go" not in path and "integration_test.go" not in path:
        return False
    parts = path.replace("\\", "/").split("/")
    return not any(p in SKIP_DIRS for p in parts)


def add_import(src: str) -> str:
    if ID_IMPORT in src:
        return src
    m = re.search(r"import \(\n", src)
    if m:
        return src[: m.end()] + f'\t"{ID_IMPORT}"\n' + src[m.end() :]
    m = re.search(r'import "([^"]+)"\n', src)
    if m:
        old = m.group(0)
        block = f'import (\n\t"{ID_IMPORT}"\n\t"{m.group(1)}"\n)\n'
        return src.replace(old, block, 1)
    raise RuntimeError("could not find import block")


def main() -> int:
    root = sys.argv[1] if len(sys.argv) > 1 else "."
    changed: list[str] = []
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS and not d.startswith(".")]
        for fn in filenames:
            path = os.path.join(dirpath, fn)
            if not should_process(path):
                continue
            with open(path, encoding="utf-8") as f:
                src = f.read()
            if "uuid.New()" not in src:
                continue
            new = src.replace("uuid.New()", "id.NewUUIDV7()")
            if ID_IMPORT not in new:
                new = add_import(new)
            if new != src:
                with open(path, "w", encoding="utf-8", newline="\n") as f:
                    f.write(new)
                changed.append(path.replace("\\", "/"))
    print(f"changed {len(changed)} files")
    for p in sorted(changed):
        print(p)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

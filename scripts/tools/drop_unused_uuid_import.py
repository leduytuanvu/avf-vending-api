#!/usr/bin/env python3
"""Remove unused github.com/google/uuid import from Go files."""
from __future__ import annotations

import os
import re
import sys

UUID_IMPORT = "github.com/google/uuid"


def clean_file(path: str) -> bool:
    with open(path, encoding="utf-8") as f:
        src = f.read()
    if UUID_IMPORT not in src:
        return False
    if re.search(r"\buuid\.", src):
        return False
    new = src
    new = re.sub(r'\t"' + re.escape(UUID_IMPORT) + r'"\n', "", new)
    new = re.sub(r'\t?' + re.escape(UUID_IMPORT) + r'\n', "", new)
    new = re.sub(
        r'import \(\n\t"' + re.escape(UUID_IMPORT) + r'"\n\)\n',
        "",
        new,
    )
    if new == src:
        return False
    with open(path, "w", encoding="utf-8", newline="\n") as f:
        f.write(new)
    return True


def main() -> int:
    root = sys.argv[1] if len(sys.argv) > 1 else "internal"
    n = 0
    for dirpath, _, filenames in os.walk(root):
        for fn in filenames:
            if not fn.endswith(".go"):
                continue
            path = os.path.join(dirpath, fn)
            if clean_file(path):
                n += 1
                print(path.replace("\\", "/"))
    print(f"cleaned {n}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

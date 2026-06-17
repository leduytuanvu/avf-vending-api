#!/usr/bin/env python3
"""Scan active markdown docs for broken relative links."""

from __future__ import annotations

import re
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]

# Glob patterns relative to repo root (active docs only).
# docs/archive/** is excluded via should_scan() — historical audits/reports are not link-gated.
SCAN_PATTERNS = [
    "README.md",
    "docs/**/*.md",
    "deployments/**/README.md",
    "tests/e2e/README.md",
    "tools/**/README.md",
]

# Skip archived / generated paths even if matched by globs.
SKIP_PATH_PARTS = {
    "archive",
    ".pytest_cache",
}

LINK_RE = re.compile(r"\[([^\]]*)\]\(([^)]+)\)")


def should_scan(path: Path) -> bool:
    rel = path.relative_to(REPO_ROOT)
    parts = rel.parts
    if any(part in SKIP_PATH_PARTS for part in parts):
        return False
    if "docs" in parts and "archive" in parts:
        return False
    return True


def collect_markdown_files() -> list[Path]:
    seen: set[Path] = set()
    files: list[Path] = []
    for pattern in SCAN_PATTERNS:
        for path in REPO_ROOT.glob(pattern):
            if not path.is_file() or path.suffix.lower() != ".md":
                continue
            resolved = path.resolve()
            if resolved in seen:
                continue
            if not should_scan(path):
                continue
            seen.add(resolved)
            files.append(path)
    return sorted(files, key=lambda p: str(p.relative_to(REPO_ROOT)))


def is_external(link_target: str) -> bool:
    lowered = link_target.strip().lower()
    return lowered.startswith(("http://", "https://", "mailto:"))


def strip_link_suffix(target: str) -> str:
    """Remove #anchor and ?query from link target."""
    target = target.strip()
    if not target:
        return target
    hash_idx = target.find("#")
    query_idx = target.find("?")
    cut = len(target)
    for idx in (hash_idx, query_idx):
        if idx != -1:
            cut = min(cut, idx)
    return target[:cut]


def resolve_link(source_file: Path, link_target: str) -> Path | None:
    target = strip_link_suffix(link_target)
    if not target or is_external(target):
        return None
    if target.startswith("/"):
        return REPO_ROOT / target.lstrip("/")
    return (source_file.parent / target).resolve()


def find_broken_links() -> list[tuple[Path, int, str, str, str]]:
    broken: list[tuple[Path, int, str, str, str]] = []
    for md_file in collect_markdown_files():
        try:
            text = md_file.read_text(encoding="utf-8")
        except UnicodeDecodeError:
            text = md_file.read_text(encoding="latin-1")
        for line_no, line in enumerate(text.splitlines(), start=1):
            for match in LINK_RE.finditer(line):
                label, target = match.group(1), match.group(2)
                resolved = resolve_link(md_file, target)
                if resolved is None:
                    continue
                if not resolved.exists():
                    rel_source = md_file.relative_to(REPO_ROOT)
                    broken.append((rel_source, line_no, label, target, str(resolved)))
    return broken


def main() -> int:
    broken = find_broken_links()
    if not broken:
        print("OK: no broken relative markdown links found.")
        return 0

    print(f"BROKEN: {len(broken)} relative link(s) in active docs\n")
    current_file: Path | None = None
    for rel_source, line_no, _label, target, resolved in broken:
        if rel_source != current_file:
            current_file = rel_source
            print(f"{rel_source}:")
        print(f"  L{line_no}: [{target}] -> missing ({resolved})")
    return 1


if __name__ == "__main__":
    sys.exit(main())

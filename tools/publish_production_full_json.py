#!/usr/bin/env python3
"""Publish full production Postman JSON for classic Import (461-request inventory).

Uses the committed inventory under postman/suites/production-full/ — does NOT regen
from OpenAPI (that path produces ~344 REST ops only).
"""
from __future__ import annotations

import argparse
import json
import shutil
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from tools.fix_production_full_json_scripts import fix_json_file  # noqa: E402

SOURCE_DIR = ROOT / "postman" / "suites" / "production-full"
SOURCE_COLLECTION = SOURCE_DIR / "avf-vending-production.full.postman_collection.json"
SOURCE_ENV = SOURCE_DIR / "avf-vending-production.full.postman_environment.json"

PUBLISH_COLLECTION = ROOT / "postman" / "collections" / "avf-vending-production-full.postman_collection.json"
PUBLISH_ENV = ROOT / "postman" / "environments" / "avf-production-full.postman_environment.json"

MIN_FULL_REQUESTS = 450


def count_requests(collection: dict) -> int:
    total = 0

    def walk(items: list) -> None:
        nonlocal total
        for it in items or []:
            if it.get("request"):
                total += 1
            else:
                walk(it.get("item") or [])

    walk(collection.get("item") or [])
    return total


def load_json(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def publish(*, fix_scripts: bool = True) -> int:
    if not SOURCE_COLLECTION.is_file():
        print(f"ERROR: missing source collection {SOURCE_COLLECTION.relative_to(ROOT)}", file=sys.stderr)
        return 1
    if not SOURCE_ENV.is_file():
        print(f"ERROR: missing source environment {SOURCE_ENV.relative_to(ROOT)}", file=sys.stderr)
        return 1

    coll = load_json(SOURCE_COLLECTION)
    req_count = count_requests(coll)
    if req_count < MIN_FULL_REQUESTS:
        print(
            f"ERROR: source collection has only {req_count} requests "
            f"(expected >={MIN_FULL_REQUESTS}). "
            "Restore postman/suites/production-full/*.json from git; "
            "do not use generate_production_full_suite.py alone.",
            file=sys.stderr,
        )
        return 1

    PUBLISH_COLLECTION.parent.mkdir(parents=True, exist_ok=True)
    PUBLISH_ENV.parent.mkdir(parents=True, exist_ok=True)

    shutil.copy2(SOURCE_COLLECTION, PUBLISH_COLLECTION)
    if fix_scripts:
        n = fix_json_file(SOURCE_COLLECTION)
        if n:
            print(f"fixed {n} script block(s) in source collection")
        shutil.copy2(SOURCE_COLLECTION, PUBLISH_COLLECTION)

    shutil.copy2(SOURCE_ENV, PUBLISH_ENV)

    published_count = count_requests(load_json(PUBLISH_COLLECTION))
    env_vars = len(load_json(PUBLISH_ENV).get("values") or [])

    print(f"OK: published {published_count} requests -> {PUBLISH_COLLECTION.relative_to(ROOT)}")
    print(f"OK: published {env_vars} env vars -> {PUBLISH_ENV.relative_to(ROOT)}")
    print(f"    (source inventory: {SOURCE_COLLECTION.relative_to(ROOT)})")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description="Publish full production Postman JSON for Import")
    parser.add_argument("--no-fix-scripts", action="store_true")
    args = parser.parse_args()
    return publish(fix_scripts=not args.no_fix_scripts)


if __name__ == "__main__":
    raise SystemExit(main())

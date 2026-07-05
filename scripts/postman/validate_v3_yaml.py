#!/usr/bin/env python3
"""Deep JSON↔v3 YAML parity validation for Postman assets."""
from __future__ import annotations

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from tools.postman_v3_parity import (  # noqa: E402
    compare_collection_pair,
    compare_environment_pair,
    validate_collection_depth,
)

V3_ROOT = ROOT / "postman" / "v3"

PRODUCTION_FULL_JSON = ROOT / "postman/suites/production-full/avf-vending-production.full.postman_collection.json"


def collection_compare_kwargs(json_path: Path) -> dict[str, bool]:
    if json_path.resolve() == PRODUCTION_FULL_JSON.resolve():
        return {"exclude_doc_only": True}
    return {"exclude_doc_only": False}


COLLECTIONS = [
    (ROOT / "postman/collections/avf-vending-api.postman_collection.json", V3_ROOT / "collections/avf-vending-api"),
    (
        ROOT / "postman/collections/avf-vending-api-function-path.postman_collection.json",
        V3_ROOT / "collections/avf-vending-api-function-path",
    ),
    (ROOT / "postman/production/avf-production-e2e.postman_collection.json", V3_ROOT / "collections/avf-production-e2e"),
    (
        ROOT / "postman/suites/production-full/avf-vending-production.full.postman_collection.json",
        V3_ROOT / "suites/production-full/avf-vending-production-full",
    ),
]

ENVIRONMENTS = [
    (ROOT / "postman/environments/avf-local.postman_environment.json", V3_ROOT / "environments/avf-local.environment.yaml"),
    (ROOT / "postman/environments/avf-staging.postman_environment.json", V3_ROOT / "environments/avf-staging.environment.yaml"),
    (ROOT / "postman/environments/avf-production.postman_environment.json", V3_ROOT / "environments/avf-production.environment.yaml"),
    (
        ROOT / "postman/suites/production-full/avf-vending-production.full.postman_environment.json",
        V3_ROOT / "environments/avf-production-full.environment.yaml",
    ),
    (ROOT / "postman/production/avf-production-e2e.postman_environment.json", V3_ROOT / "environments/avf-production-e2e.environment.yaml"),
]


def main() -> int:
    errors: list[str] = []
    for jpath, vpath in COLLECTIONS:
        kw = collection_compare_kwargs(jpath)
        errors.extend(compare_collection_pair(jpath, vpath, **kw))
        errors.extend(validate_collection_depth(jpath, vpath, **kw))
    for jpath, ypath in ENVIRONMENTS:
        errors.extend(compare_environment_pair(jpath, ypath))

    if errors:
        for e in errors:
            print(f"ERROR: {e}", file=sys.stderr)
        print(f"VALIDATION_FAIL: {len(errors)} issue(s)", file=sys.stderr)
        return 1

    print("VALIDATION_PASS: v3 YAML parity OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

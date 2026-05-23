#!/usr/bin/env python3
"""Generate Postman collection + environment from tests/e2e/production/e2e-manifest.yaml only."""
from __future__ import annotations

import json
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(REPO_ROOT / "postman" / "production"))

from manifest_postman_lib import (  # noqa: E402
    build_postman_collection,
)

MANIFEST_MAIN = REPO_ROOT / "tests" / "e2e" / "production" / "e2e-manifest.yaml"
OUT_COLL = REPO_ROOT / "postman" / "production" / "avf-production-e2e.postman_collection.json"
OUT_ENV = REPO_ROOT / "postman" / "production" / "avf-production-e2e.postman_environment.json"


def main() -> int:
    if not MANIFEST_MAIN.is_file():
        print(f"ERROR: missing manifest {MANIFEST_MAIN}", file=sys.stderr)
        return 1
    collection, environment, flows = build_postman_collection(MANIFEST_MAIN)
    OUT_COLL.parent.mkdir(parents=True, exist_ok=True)
    OUT_COLL.write_text(json.dumps(collection, indent=2) + "\n", encoding="utf-8")
    OUT_ENV.write_text(json.dumps(environment, indent=2) + "\n", encoding="utf-8")
    print(f"GENERATED {OUT_COLL.name}: {len(flows)} REST requests from e2e-manifest.yaml")
    print(f"GENERATED {OUT_ENV.name}: placeholders only (no secrets)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

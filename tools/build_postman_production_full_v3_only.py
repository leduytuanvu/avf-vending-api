#!/usr/bin/env python3
"""Generate Postman v3 YAML for production-full only (from committed JSON; no OpenAPI regen)."""
from __future__ import annotations

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from tools.build_postman_v3_yaml import (  # noqa: E402
    MIGRATE_TMP,
    _rm_tree,
    find_postman_cli,
    migrate_collection,
    sanitize_stale_paths_in_v3,
)
from tools.postman_v3_environment import export_environment_yaml  # noqa: E402
from tools.postman_v3_script_fixup import fix_all_json_collections, fix_all_v3_yaml  # noqa: E402

JSON_COLLECTION = ROOT / "postman/suites/production-full/avf-vending-production.full.postman_collection.json"
JSON_ENV = ROOT / "postman/suites/production-full/avf-vending-production.full.postman_environment.json"
V3_COLLECTION = ROOT / "postman/v3/suites/production-full/avf-vending-production-full"
V3_ENV = ROOT / "postman/v3/environments/avf-production-full.environment.yaml"


def main() -> int:
    if not JSON_COLLECTION.is_file():
        print(f"ERROR: missing {JSON_COLLECTION.relative_to(ROOT)}", file=sys.stderr)
        return 1

    cli = find_postman_cli()
    if not cli:
        print(
            "ERROR: Postman CLI not found. Install: npm install -g postman-cli",
            file=sys.stderr,
        )
        return 1

    MIGRATE_TMP.mkdir(parents=True, exist_ok=True)
    fix_all_json_collections([JSON_COLLECTION])
    _rm_tree(V3_COLLECTION)
    print(f"migrate {JSON_COLLECTION.name} -> {V3_COLLECTION.relative_to(ROOT)}")
    migrate_collection(cli, JSON_COLLECTION, V3_COLLECTION)
    fix_all_v3_yaml(V3_COLLECTION)
    sanitize_stale_paths_in_v3()

    if JSON_ENV.is_file():
        export_environment_yaml(JSON_ENV, V3_ENV)
        print(f"export env -> {V3_ENV.relative_to(ROOT)}")

    print(f"OK: production-full v3 at {V3_COLLECTION.relative_to(ROOT)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

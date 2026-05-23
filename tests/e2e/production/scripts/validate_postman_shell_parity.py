#!/usr/bin/env python3
"""Fail if Postman collection diverges from production E2E shell REST manifest."""
from __future__ import annotations

import argparse
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[4]
sys.path.insert(0, str(REPO_ROOT / "postman" / "production"))

from manifest_postman_lib import validate_shell_postman_parity  # noqa: E402

MANIFEST = REPO_ROOT / "tests" / "e2e" / "production" / "e2e-manifest.yaml"
COLLECTION = REPO_ROOT / "postman" / "production" / "avf-production-e2e.postman_collection.json"


def main() -> int:
    ap = argparse.ArgumentParser(description="Validate shell REST ↔ Postman parity")
    ap.add_argument("--manifest", type=Path, default=MANIFEST)
    ap.add_argument("--collection", type=Path, default=COLLECTION)
    args = ap.parse_args()
    if not args.manifest.is_file():
        print(f"ERROR: missing manifest {args.manifest}", file=sys.stderr)
        return 1
    if not args.collection.is_file():
        print(f"ERROR: missing collection {args.collection}", file=sys.stderr)
        return 1
    errors = validate_shell_postman_parity(args.manifest, args.collection)
    if errors:
        print("POSTMAN_PARITY_FAIL", file=sys.stderr)
        for e in errors:
            print(f"  - {e}", file=sys.stderr)
        return 1
    print("POSTMAN_PARITY_OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

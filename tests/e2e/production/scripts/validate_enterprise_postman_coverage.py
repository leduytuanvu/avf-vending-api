#!/usr/bin/env python3
"""Validate enterprise Postman collection is a superset of manifest + route-coverage REST flows."""
from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[4]
sys.path.insert(0, str(REPO_ROOT / "postman" / "production"))

from manifest_postman_lib import (  # noqa: E402
    collect_manifest_rest_flows,
    collect_postman_rest_requests,
    flow_request_spec,
    load_main_manifest,
    parity_key,
)

MANIFEST_MAIN = REPO_ROOT / "tests" / "e2e" / "production" / "e2e-manifest.yaml"
MANIFEST_COV = REPO_ROOT / "tests" / "e2e" / "production" / "e2e-manifest-rest-coverage.yaml"
DEFAULT_COLL = REPO_ROOT / "postman" / "production-enterprise" / "AVF_PRODUCTION_ENTERPRISE_REST.postman_collection.json"


def load_coverage_flows() -> list[dict]:
    import yaml

    data = yaml.safe_load(MANIFEST_COV.read_text(encoding="utf-8")) or {}
    return [f for f in data.get("flows") or [] if f.get("protocol") == "rest" and f.get("method")]


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--collection", type=Path, default=DEFAULT_COLL)
    args = ap.parse_args()
    manifest = load_main_manifest(MANIFEST_MAIN)
    manifest_dir = MANIFEST_MAIN.parent
    repo_root = REPO_ROOT
    shell = collect_manifest_rest_flows(manifest, postman_only=True, repo_root=repo_root)
    shell += [f for f in collect_manifest_rest_flows(manifest, postman_only=False, repo_root=repo_root) if f.get("phase") == "negative"]
    shell += load_coverage_flows()
    coll = json.loads(args.collection.read_text(encoding="utf-8"))
    postman_reqs = [r for r in collect_postman_rest_requests(coll) if r.get("flow_id")]

    def find_postman(flow: dict) -> dict | None:
        spec = flow_request_spec(flow, manifest_dir)
        for pm in postman_reqs:
            if pm.get("flow_id") != flow.get("id"):
                continue
            if pm.get("method") == spec["method"] and pm.get("path") == spec["path"]:
                return pm
        return None

    errors: list[str] = []
    for flow in shell:
        fid = str(flow.get("id") or "")
        if not fid:
            continue
        spec = flow_request_spec(flow, manifest_dir)
        pm = find_postman(flow)
        if not pm:
            errors.append(f"missing flow {fid} {spec['method']} {spec['path']} in enterprise collection")
            continue

    if errors:
        print("ENTERPRISE_COVERAGE_FAIL", file=sys.stderr)
        for e in errors[:30]:
            print(f"  - {e}", file=sys.stderr)
        if len(errors) > 30:
            print(f"  ... and {len(errors) - 30} more", file=sys.stderr)
        return 1
    print(f"ENTERPRISE_COVERAGE_OK shell_flows={len(shell)} postman_requests={len(postman_reqs)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""Merge cosign verify --output json results into a single signing-evidence document for CI artifacts."""
from __future__ import annotations

import argparse
import json
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


def _load_verify(path: Path) -> Any:
    if not path.is_file():
        return None
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return None


def _simplify_verify(payload: Any) -> dict[str, Any]:
    if not isinstance(payload, list) or not payload:
        return {"verified": False, "detail": "empty or invalid verify payload"}
    first = payload[0]
    if not isinstance(first, dict):
        return {"verified": False, "detail": "unexpected verify entry shape"}
    out: dict[str, Any] = {"verified": bool(first.get("verified"))}
    if "issuer" in first:
        out["issuer"] = first["issuer"]
    # Optional: certificate chain summary (structure varies by cosign version)
    cert = first.get("certificate")
    if isinstance(cert, dict):
        if cert.get("issuer"):
            out["certificate_issuer"] = cert["issuer"]
        if cert.get("subject"):
            out["certificate_subject"] = cert["subject"]
    return out


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--out", required=True, help="Output JSON path")
    ap.add_argument("--app-ref", required=True)
    ap.add_argument("--goose-ref", required=True)
    ap.add_argument("--verify-app-json", default="", help="Path from cosign verify --output json for app")
    ap.add_argument("--verify-goose-json", default="", help="Path from cosign verify --output json for goose")
    ap.add_argument("--workflow-run-id", default="")
    ap.add_argument("--repository", default="")
    ap.add_argument("--workflow-ref", default="")
    ap.add_argument("--identity-regexp", default="")
    ap.add_argument("--issuer-regexp", default="")
    args = ap.parse_args()

    signed_at = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    app_path = Path(args.verify_app_json) if args.verify_app_json else None
    goose_path = Path(args.verify_goose_json) if args.verify_goose_json else None
    app_raw = _load_verify(app_path) if app_path else None
    goose_raw = _load_verify(goose_path) if goose_path else None

    doc = {
        "schema_version": "v1",
        "signed_at_utc": signed_at,
        "workflow_run_id": args.workflow_run_id,
        "repository": args.repository,
        "workflow_ref": args.workflow_ref,
        "policy": {
            "certificate_identity_regexp": args.identity_regexp,
            "certificate_oidc_issuer_regexp": args.issuer_regexp,
        },
        "signed_image_refs": {
            "app": args.app_ref,
            "goose": args.goose_ref,
        },
        "post_sign_verify": {
            "app": _simplify_verify(app_raw),
            "goose": _simplify_verify(goose_raw),
        },
    }
    out_path = Path(args.out)
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(json.dumps(doc, indent=2) + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

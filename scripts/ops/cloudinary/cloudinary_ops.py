#!/usr/bin/env python3
"""Cloudinary Admin API inventory, wipe, and verification (no secrets in output)."""
from __future__ import annotations

import argparse
import json
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from collections import defaultdict
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

RESOURCE_TYPES = ("image", "video", "raw")
DELIVERY_TYPES = ("upload", "private", "authenticated")
# fetch/list are not valid for all resource types; probed separately when needed.
OPTIONAL_DELIVERY_TYPES = ("fetch", "list")
AUDIO_FORMATS = frozenset({"mp3", "wav", "aac", "m4a", "ogg", "flac", "wma"})


def utc_ts() -> str:
    return datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")


def creds() -> tuple[str, str, str]:
    cloud = (os.environ.get("CLOUDINARY_CLOUD_NAME") or "").strip()
    key = (os.environ.get("CLOUDINARY_API_KEY") or "").strip()
    secret = (os.environ.get("CLOUDINARY_API_SECRET") or "").strip()
    if not cloud or not key or not secret:
        print("error: CLOUDINARY_CLOUD_NAME, CLOUDINARY_API_KEY, CLOUDINARY_API_SECRET required", file=sys.stderr)
        sys.exit(2)
    return cloud, key, secret


def api_request(
    method: str,
    path: str,
    params: dict[str, str] | None = None,
    retries: int = 8,
    allow_404: bool = False,
) -> dict[str, Any]:
    cloud, key, secret = creds()
    base = f"https://api.cloudinary.com/v1_1/{cloud}"
    url = base + path
    if params:
        url += "?" + urllib.parse.urlencode(params)
    req = urllib.request.Request(url, method=method)
    password_mgr = urllib.request.HTTPPasswordMgrWithDefaultRealm()
    password_mgr.add_password(None, base, key, secret)
    opener = urllib.request.build_opener(urllib.request.HTTPBasicAuthHandler(password_mgr))
    delay = 1.0
    for attempt in range(retries):
        try:
            with opener.open(req, timeout=120) as resp:
                raw = resp.read().decode("utf-8", errors="replace")
                if not raw:
                    return {}
                return json.loads(raw)
        except urllib.error.HTTPError as e:
            body = e.read().decode("utf-8", errors="replace")
            if allow_404 and e.code in (404, 400):
                return {}
            if e.code in (420, 429) and attempt < retries - 1:
                time.sleep(delay)
                delay = min(delay * 2, 60)
                continue
            print(f"error: HTTP {e.code} {method} {path}: {body[:500]}", file=sys.stderr)
            sys.exit(1)
        except urllib.error.URLError as e:
            if attempt < retries - 1:
                time.sleep(delay)
                delay = min(delay * 2, 60)
                continue
            print(f"error: network {method} {path}: {e}", file=sys.stderr)
            sys.exit(1)
    return {}


def list_resources(resource_type: str, delivery_type: str) -> list[dict[str, Any]]:
    assets: list[dict[str, Any]] = []
    cursor: str | None = None
    while True:
        params: dict[str, str] = {"max_results": "500"}
        if cursor:
            params["next_cursor"] = cursor
        data = api_request(
            "GET",
            f"/resources/{resource_type}/{delivery_type}",
            params,
            allow_404=True,
        )
        resources = data.get("resources") or []
        for r in resources:
            assets.append(
                {
                    "asset_id": r.get("asset_id"),
                    "public_id": r.get("public_id"),
                    "resource_type": resource_type,
                    "type": delivery_type,
                    "format": r.get("format"),
                    "bytes": r.get("bytes", 0),
                    "created_at": r.get("created_at"),
                    "folder": r.get("folder") or r.get("asset_folder"),
                    "tags": r.get("tags") or [],
                }
            )
        cursor = data.get("next_cursor")
        if not cursor:
            break
    return assets


def count_cell(resource_type: str, delivery_type: str) -> int:
    params: dict[str, str] = {"max_results": "1"}
    try:
        data = api_request("GET", f"/resources/{resource_type}/{delivery_type}", params)
    except SystemExit:
        return 0
    # total_count when available; else len resources + cursor check
    if "total_count" in data:
        return int(data["total_count"])
    n = len(data.get("resources") or [])
    if data.get("next_cursor"):
        # need full list for accurate count — caller uses inventory for truth
        return -1
    return n


def iter_delivery_types(resource_type: str) -> tuple[str, ...]:
    types = list(DELIVERY_TYPES)
    if resource_type == "image":
        types.extend(OPTIONAL_DELIVERY_TYPES)
    return tuple(types)


def full_inventory() -> dict[str, Any]:
    all_assets: list[dict[str, Any]] = []
    matrix: dict[str, int] = {}
    for rt in RESOURCE_TYPES:
        for dt in iter_delivery_types(rt):
            key = f"{rt}/{dt}"
            batch = list_resources(rt, dt)
            matrix[key] = len(batch)
            all_assets.extend(batch)

    totals = defaultdict(int)
    bytes_total = 0
    audio_count = 0
    for a in all_assets:
        totals[a["resource_type"]] += 1
        bytes_total += int(a.get("bytes") or 0)
        fmt = (a.get("format") or "").lower()
        if fmt in AUDIO_FORMATS:
            audio_count += 1

    by_folder: dict[str, int] = defaultdict(int)
    avf_tagged = 0
    avf_folder = 0
    for a in all_assets:
        folder = a.get("folder") or "(root)"
        by_folder[str(folder)] += 1
        tags = a.get("tags") or []
        if "avf-vending" in tags:
            avf_tagged += 1
        pid = str(a.get("public_id") or "")
        if pid.startswith("avf-vending/") or "/avf-vending/" in pid:
            avf_folder += 1

    return {
        "cloud_name": creds()[0],
        "timestamp": utc_ts(),
        "matrix": matrix,
        "total_original_assets": len(all_assets),
        "total_derived_assets": 0,
        "image_count": totals["image"],
        "video_count": totals["video"],
        "raw_count": totals["raw"],
        "audio_count": audio_count,
        "total_bytes": bytes_total,
        "by_folder": dict(by_folder),
        "avf_tagged_count": avf_tagged,
        "avf_folder_prefix_count": avf_folder,
        "assets": all_assets,
    }


def delete_all_loop(resource_type: str, delivery_type: str, dry_run: bool) -> dict[str, Any]:
    before = list_resources(resource_type, delivery_type)
    if dry_run:
        return {"deleted_batches": 0, "would_delete": len(before), "partial": False}

    if not before:
        return {"deleted_batches": 0, "partial": False}

    batches = 0
    cursor: str | None = None
    while True:
        params: dict[str, str] = {
            "all": "true",
            "keep_original": "false",
            "invalidate": "true",
        }
        if cursor:
            params["next_cursor"] = cursor
        data = api_request("DELETE", f"/resources/{resource_type}/{delivery_type}", params)
        batches += 1
        partial = bool(data.get("partial"))
        cursor = data.get("next_cursor")
        if not partial and not cursor:
            break
        if not cursor:
            break
        time.sleep(0.5)
    return {"deleted_batches": batches, "partial": False}


def delete_all_matrix(dry_run: bool) -> dict[str, Any]:
    results: dict[str, Any] = {}
    for rt in RESOURCE_TYPES:
        for dt in iter_delivery_types(rt):
            key = f"{rt}/{dt}"
            before = len(list_resources(rt, dt))
            if before == 0:
                results[key] = {"before": 0, "after": 0, "skipped": True}
                continue
            out = delete_all_loop(rt, dt, dry_run)
            after = 0 if dry_run else len(list_resources(rt, dt))
            results[key] = {"before": before, "after": after, **out}
    return results


def list_folders() -> list[str]:
    try:
        data = api_request("GET", "/folders", {})
    except SystemExit:
        return []
    folders = data.get("folders") or []
    return [str(f.get("path") or f) for f in folders if f]


def delete_empty_folders(dry_run: bool) -> list[str]:
    deleted: list[str] = []
    folders = sorted(list_folders(), key=lambda p: p.count("/"), reverse=True)
    for folder in folders:
        if dry_run:
            deleted.append(folder)
            continue
        try:
            api_request("DELETE", f"/folders/{urllib.parse.quote(folder, safe='')}")
            deleted.append(folder)
        except SystemExit:
            pass
        time.sleep(0.3)
    return deleted


def verify_empty() -> dict[str, Any]:
    inv = full_inventory()
    inv["assets"] = []  # strip for summary
    inv["verified_empty"] = inv["total_original_assets"] == 0
    folders = list_folders()
    inv["nonempty_folder_count"] = len(folders)
    inv["folders"] = folders
    return inv


def cmd_discover_targets(_: argparse.Namespace) -> None:
    targets = []
    prod_env = os.environ.get("PROD_ENV_FILE", "")
    stg_env = os.environ.get("STAGING_ENV_FILE", "")
    for env, path in (("production", prod_env), ("staging", stg_env)):
        if not path or not Path(path).is_file():
            targets.append({"environment": env, "configured": False, "cloud_name": None})
            continue
        cloud = None
        for line in Path(path).read_text(encoding="utf-8", errors="replace").splitlines():
            line = line.strip()
            if line.startswith("CLOUDINARY_CLOUD_NAME="):
                cloud = line.split("=", 1)[1].strip().strip('"').strip("'")
                break
        targets.append(
            {
                "environment": env,
                "configured": bool(cloud),
                "cloud_name": cloud,
                "env_file": path,
            }
        )
    clouds = {t["cloud_name"] for t in targets if t.get("cloud_name")}
    print(json.dumps({"targets": targets, "unique_cloud_names": sorted(clouds)}, indent=2))


def main() -> None:
    p = argparse.ArgumentParser(description="Cloudinary Admin API ops")
    p.add_argument("--evidence-dir", default=os.environ.get("CLOUDINARY_EVIDENCE_DIR", ".cloudinary-wipe-evidence"))
    sub = p.add_subparsers(dest="cmd", required=True)

    sub.add_parser("inventory", help="Full paginated inventory")
    sub.add_parser("verify", help="Verify empty state")
    wp = sub.add_parser("wipe", help="Delete all assets")
    wp.add_argument("--dry-run", action="store_true")
    wp.add_argument("--delete-folders", action="store_true")
    sub.add_parser("discover-targets", help="Parse env files for cloud names (no API)")

    args = p.parse_args()
    evidence = Path(args.evidence_dir)
    evidence.mkdir(parents=True, exist_ok=True)

    if args.cmd == "discover-targets":
        cmd_discover_targets(args)
        return

    if args.cmd == "inventory":
        inv = full_inventory()
        out_assets = evidence / f"asset-inventory-before-{inv['timestamp']}.json"
        out_counts = evidence / f"asset-counts-before-{inv['timestamp']}.txt"
        out_assets.write_text(json.dumps(inv, indent=2), encoding="utf-8")
        summary = {k: inv[k] for k in inv if k != "assets"}
        out_counts.write_text(json.dumps(summary, indent=2), encoding="utf-8")
        print(json.dumps(summary, indent=2))
        print(f"wrote {out_assets}", file=sys.stderr)
        return

    if args.cmd == "verify":
        v = verify_empty()
        ts = v.get("timestamp") or utc_ts()
        out = evidence / f"asset-counts-after-{ts}.json"
        out.write_text(json.dumps(v, indent=2), encoding="utf-8")
        print(json.dumps(v, indent=2))
        if not v.get("verified_empty"):
            sys.exit(1)
        return

    if args.cmd == "wipe":
        ts = utc_ts()
        results = delete_all_matrix(args.dry_run)
        folder_results: list[str] = []
        if args.delete_folders and not args.dry_run:
            folder_results = delete_empty_folders(False)
        elif args.delete_folders and args.dry_run:
            folder_results = list_folders()
        payload = {
            "timestamp": ts,
            "cloud_name": creds()[0],
            "dry_run": args.dry_run,
            "matrix_results": results,
            "folders_deleted": folder_results,
        }
        out = evidence / f"deletion-results-{ts}.json"
        out.write_text(json.dumps(payload, indent=2), encoding="utf-8")
        print(json.dumps(payload, indent=2))
        if not args.dry_run:
            v = verify_empty()
            if not v.get("verified_empty"):
                sys.exit(1)


if __name__ == "__main__":
    main()

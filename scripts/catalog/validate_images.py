#!/usr/bin/env python3
"""Validate all source product image URLs from enriched manifest."""
from __future__ import annotations

import argparse
import hashlib
import json
import mimetypes
import sys
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime, timezone
from io import BytesIO
from pathlib import Path
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

try:
    from PIL import Image
except ImportError:
    Image = None  # type: ignore


def validate_url(url: str, timeout: float = 30.0) -> dict[str, Any]:
    result: dict[str, Any] = {
        "url": url,
        "status": "SOURCE_IMAGE_UNAVAILABLE",
        "http_status": None,
        "content_type": None,
        "content_length": None,
        "mime_type": None,
        "width": None,
        "height": None,
        "bytes": None,
        "sha256": None,
        "error": None,
    }
    if not url:
        result["error"] = "empty url"
        return result
    try:
        req = Request(url, headers={"User-Agent": "avf-catalog-bootstrap/1.0"})
        with urlopen(req, timeout=timeout) as resp:
            result["http_status"] = getattr(resp, "status", None) or resp.getcode()
            result["content_type"] = resp.headers.get("Content-Type")
            cl = resp.headers.get("Content-Length")
            result["content_length"] = int(cl) if cl and cl.isdigit() else None
            body = resp.read()
    except HTTPError as e:
        result["http_status"] = e.code
        result["error"] = str(e)
        return result
    except URLError as e:
        result["error"] = str(e.reason)
        return result
    except Exception as e:
        result["error"] = str(e)
        return result

    result["bytes"] = len(body)
    result["sha256"] = hashlib.sha256(body).hexdigest()
    mime = (result["content_type"] or "").split(";")[0].strip().lower()
    if not mime:
        mime, _ = mimetypes.guess_type(url)
        mime = (mime or "").lower()
    result["mime_type"] = mime

    if not mime.startswith("image/"):
        result["status"] = "INVALID_NOT_IMAGE"
        result["error"] = f"unexpected mime {mime}"
        return result

    if Image is not None:
        try:
            with Image.open(BytesIO(body)) as img:
                result["width"], result["height"] = img.size
                img.verify()
        except Exception as e:
            result["status"] = "INVALID_NOT_IMAGE"
            result["error"] = f"decode failed: {e}"
            return result
    result["status"] = "OK"
    return result


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--manifest",
        default="../../../docs/avf_products_enriched_cloudinary_import_manifest.json",
    )
    parser.add_argument("--evidence-dir", default="../../.catalog-bootstrap-evidence")
    parser.add_argument("--workers", type=int, default=8)
    args = parser.parse_args()
    manifest_path = Path(__file__).resolve().parent / args.manifest
    evidence_dir = Path(__file__).resolve().parent / args.evidence_dir
    evidence_dir.mkdir(parents=True, exist_ok=True)

    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    products = manifest.get("products", [])
    results: list[dict[str, Any]] = []
    sha_to_skus: dict[str, list[str]] = {}

    with ThreadPoolExecutor(max_workers=args.workers) as pool:
        futures = {
            pool.submit(validate_url, p.get("source_image_url", "")): p for p in products
        }
        for fut in as_completed(futures):
            p = futures[fut]
            row = fut.result()
            row["sku"] = p["sku"]
            row["product_name"] = p["name"]
            results.append(row)
            if row.get("sha256"):
                sha_to_skus.setdefault(row["sha256"], []).append(p["sku"])

    results.sort(key=lambda r: r["sku"])
    ok = sum(1 for r in results if r["status"] == "OK")
    dup_urls = {}
    url_map: dict[str, list[str]] = {}
    for r in results:
        url_map.setdefault(r["url"], []).append(r["sku"])
    dup_urls = {u: s for u, s in url_map.items() if len(s) > 1}
    dup_sha = {h: s for h, s in sha_to_skus.items() if len(s) > 1}

    report = {
        "validated_at": datetime.now(timezone.utc).isoformat(),
        "total": len(results),
        "ok": ok,
        "failed": len(results) - ok,
        "duplicate_url_groups": dup_urls,
        "duplicate_sha256_groups": dup_sha,
        "items": results,
    }
    out = evidence_dir / "07-source-image-validation.json"
    out.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"total": report["total"], "ok": ok, "failed": report["failed"]}, indent=2))
    if ok < len(results):
        sys.exit(1)


if __name__ == "__main__":
    main()

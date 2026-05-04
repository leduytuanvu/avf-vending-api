#!/usr/bin/env python3
"""Validate GET /version blackbox response: HTTP 200, non-empty JSON, optional app_env / public_base_url assertions."""
from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any


class VersionSmokeError(RuntimeError):
    """Blackbox /version response failed validation."""


def normalize_base_url(url: str) -> str:
    return url.rstrip("/")


def body_preview(data: bytes | str, limit: int = 500) -> str:
    if isinstance(data, bytes):
        text = data.decode("utf-8", errors="replace")
    else:
        text = data
    snippet = text[:limit].replace("\r", "\\r").replace("\n", "\\n")
    return snippet


def validate_public_version_response(
    *,
    endpoint: str,
    http_status: str,
    content_type: str,
    body_raw: bytes,
    expect_app_env: str | None,
    expect_public_base_url: str | None,
) -> None:
    ct = (content_type or "").strip() or "unknown"
    if http_status != "200":
        pv = body_preview(body_raw)
        raise VersionSmokeError(
            f"version JSON smoke: endpoint={endpoint} http_status={http_status} "
            f"content_type={ct!r}; expected HTTP 200; first 500 chars of body: {pv!r}"
        )
    if not body_raw.strip():
        raise VersionSmokeError(
            f"version JSON smoke: endpoint={endpoint} http_status={http_status} "
            f"content_type={ct!r}; response body is empty (expected JSON object)"
        )
    text = body_raw.decode("utf-8", errors="replace")
    try:
        parsed: Any = json.loads(text)
    except json.JSONDecodeError as e:
        pv = body_preview(text)
        raise VersionSmokeError(
            f"version JSON smoke: endpoint={endpoint} http_status={http_status} content_type={ct!r}; "
            f"body is not valid JSON ({e.msg} at line {e.lineno} column {e.colno}); "
            f"first 500 chars: {pv!r}"
        ) from None
    if not isinstance(parsed, dict):
        raise VersionSmokeError(
            f"version JSON smoke: endpoint={endpoint}; JSON root must be an object, got {type(parsed).__name__}"
        )

    if expect_app_env is not None and expect_app_env != "" and "app_env" in parsed:
        got = str(parsed.get("app_env", ""))
        if got != expect_app_env:
            raise VersionSmokeError(
                f"version JSON smoke: app_env mismatch for {endpoint}: got {got!r}, expected {expect_app_env!r}"
            )

    if (
        expect_public_base_url is not None
        and expect_public_base_url != ""
        and "public_base_url" in parsed
        and parsed.get("public_base_url") not in (None, "")
    ):
        got_u = normalize_base_url(str(parsed.get("public_base_url")))
        exp_u = normalize_base_url(expect_public_base_url)
        if got_u != exp_u:
            raise VersionSmokeError(
                "version JSON smoke: public_base_url mismatch for "
                f"{endpoint}: got {got_u!r}, expected {exp_u!r}"
            )


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(description="Validate blackbox /version JSON response.")
    ap.add_argument("--endpoint", required=True, help="Requested URL (for logs only).")
    ap.add_argument("--body-file", required=True, type=Path, help="Path to curl response body.")
    ap.add_argument("--http-status", required=True, help="HTTP status code from curl.")
    ap.add_argument("--content-type", default="", help="Content-Type header value if known.")
    ap.add_argument("--expect-app-env", default=None, help="When set and app_env present in JSON, must match.")
    ap.add_argument(
        "--expect-public-base-url",
        default=None,
        help="When set and public_base_url present in JSON, must match (trailing slashes ignored).",
    )
    args = ap.parse_args(argv)
    body_raw = Path(args.body_file).read_bytes()
    try:
        validate_public_version_response(
            endpoint=args.endpoint,
            http_status=str(args.http_status).strip(),
            content_type=args.content_type or "",
            body_raw=body_raw,
            expect_app_env=args.expect_app_env,
            expect_public_base_url=args.expect_public_base_url,
        )
    except VersionSmokeError as ex:
        print(str(ex), file=sys.stderr)
        return 2
    except OSError as ex:
        print(f"version JSON smoke: cannot read body file {args.body_file}: {ex}", file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))

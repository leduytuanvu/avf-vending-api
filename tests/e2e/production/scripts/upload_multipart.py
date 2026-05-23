#!/usr/bin/env python3
"""Multipart POST for production E2E Cloudinary upload (no secrets in stdout)."""
from __future__ import annotations

import json
import mimetypes
import os
import sys
import uuid
from pathlib import Path
from urllib.error import HTTPError
from urllib.request import Request, urlopen


def main() -> int:
    if len(sys.argv) != 7:
        print("usage: upload_multipart.py <url> <token> <idem_key> <file_path> <hdr_out> <resp_out>", file=sys.stderr)
        return 2
    url, token, idem_key, file_path, hdr_out, resp_out = sys.argv[1:7]
    path = Path(file_path)
    if not path.is_file():
        print("0")
        Path(resp_out).write_text(json.dumps({"error": "missing_upload_file"}), encoding="utf-8")
        return 1
    boundary = f"----AVF-E2E-{uuid.uuid4().hex}"
    ctype = mimetypes.guess_type(path.name)[0] or "application/octet-stream"
    body_parts = [
        f"--{boundary}\r\n".encode(),
        b'Content-Disposition: form-data; name="purpose"\r\n\r\n',
        b"product_image\r\n",
        f"--{boundary}\r\n".encode(),
        f'Content-Disposition: form-data; name="file"; filename="{path.name}"\r\n'.encode(),
        f"Content-Type: {ctype}\r\n\r\n".encode(),
        path.read_bytes(),
        b"\r\n",
        f"--{boundary}--\r\n".encode(),
    ]
    body = b"".join(body_parts)
    req = Request(
        url,
        data=body,
        method="POST",
        headers={
            "Authorization": f"Bearer {token}",
            "Idempotency-Key": idem_key,
            "Content-Type": f"multipart/form-data; boundary={boundary}",
        },
    )
    code = 0
    try:
        with urlopen(req, timeout=120) as resp:
            code = resp.getcode() or 0
            raw = resp.read()
            headers = "\r\n".join(f"{k}: {v}" for k, v in resp.headers.items())
    except HTTPError as exc:
        code = exc.code
        raw = exc.read()
        headers = "\r\n".join(f"{k}: {v}" for k, v in exc.headers.items()) if exc.headers else ""
    Path(hdr_out).write_text(headers + "\n", encoding="utf-8")
    try:
        parsed = json.loads(raw.decode("utf-8"))
        Path(resp_out).write_text(json.dumps(parsed), encoding="utf-8")
    except json.JSONDecodeError:
        Path(resp_out).write_text(raw.decode("utf-8", errors="replace"), encoding="utf-8")
    print(code)
    return 0 if code else 1


if __name__ == "__main__":
    raise SystemExit(main())

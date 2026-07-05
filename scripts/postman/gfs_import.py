"""Import OpenAPI Postman builder from tracked scripts/postman/postman_openapi_lib.py."""
from __future__ import annotations

import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
LIB_DIR = REPO_ROOT / "scripts" / "postman"

if str(LIB_DIR) not in sys.path:
    sys.path.insert(0, str(LIB_DIR))

import postman_openapi_lib as gfs  # noqa: E402

GFS_DIR = REPO_ROOT / "postman" / "generated"

__all__ = ["REPO_ROOT", "GFS_DIR", "gfs"]

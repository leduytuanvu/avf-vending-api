"""Import generate_full_postman from postman/generated/."""
from __future__ import annotations

import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
GFS_DIR = REPO_ROOT / "postman" / "generated"

if str(GFS_DIR) not in sys.path:
    sys.path.insert(0, str(GFS_DIR))

import generate_full_postman as gfs  # noqa: E402

__all__ = ["REPO_ROOT", "GFS_DIR", "gfs"]

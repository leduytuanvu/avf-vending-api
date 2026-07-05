#!/usr/bin/env python3
"""Thin wrapper for tools/build_postman_v3_yaml.py."""
from __future__ import annotations

import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
raise SystemExit(
    subprocess.call([sys.executable, str(ROOT / "tools/build_postman_v3_yaml.py"), *sys.argv[1:]], cwd=ROOT)
)

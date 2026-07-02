#!/usr/bin/env python3
"""Orchestrate REST/gRPC/MQTT/Postman surface validators."""

from __future__ import annotations

import json
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
TOOLS = Path(__file__).resolve().parent


def run(script: str) -> int:
    return subprocess.call([sys.executable, str(TOOLS / script)], cwd=REPO)


def main() -> int:
    results = {}
    for name in [
        "validate_rest_surface.py",
        "validate_grpc_surface.py",
        "validate_mqtt_surface.py",
        "validate_postman_surface.py",
    ]:
        rc = run(name)
        results[name] = "pass" if rc == 0 else "fail"

    out_dir = REPO / "reports" / "enterprise-flow"
    ts_dirs = sorted(out_dir.glob("*"), reverse=True)
    target = ts_dirs[0] if ts_dirs else out_dir

    parity = {
        "timestamp": datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ"),
        "validators": results,
        "overall": "pass" if all(v == "pass" for v in results.values()) else "fail",
    }
    for fname in ["REST_SURFACE_COVERAGE.json", "GRPC_SURFACE_COVERAGE.json", "MQTT_SURFACE_COVERAGE.json"]:
        fp = target / fname
        if fp.exists():
            parity[fname.replace(".json", "").lower()] = json.loads(fp.read_text(encoding="utf-8"))

    (target / "API_SURFACE_FINAL_PARITY.json").write_text(json.dumps(parity, indent=2), encoding="utf-8")
    (target / "API_SURFACE_FINAL_PARITY.md").write_text(
        "# API Surface Final Parity\n\n" + "\n".join(f"- {k}: {v}" for k, v in results.items()),
        encoding="utf-8",
    )
    return 0 if parity["overall"] == "pass" else 1


if __name__ == "__main__":
    sys.exit(main())

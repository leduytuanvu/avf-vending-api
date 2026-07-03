#!/usr/bin/env python3
"""Verify API state matches registry after bootstrap (read-back)."""

from __future__ import annotations

import json
import os
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from _common import http_request, report_dir, write_json
from entity_registry import EntityRegistry


def main() -> int:
    base = os.environ.get("BASE_URL", "https://api.ldtv.dev")
    out = report_dir()
    reg = EntityRegistry()
    reg_map = reg.as_substitution_map()
    token = reg_map.get("adminAccessToken", "")
    checks: list[dict] = []

    def check(name: str, path: str, expect_key: str) -> None:
        status, raw, _ = http_request("GET", base.rstrip("/") + path, headers={"Authorization": f"Bearer {token}"})
        ok = status == 200 and expect_key in raw
        checks.append({"name": name, "path": path, "status": status, "pass": ok})

    mid = reg_map.get("machineId")
    sid = reg_map.get("siteId")
    if sid:
        check("site_exists", f"/v1/admin/sites/{sid}", sid)
    if mid:
        check("machine_exists", f"/v1/admin/machines/{mid}", mid)
        check("machine_runtime_sessions", f"/v1/admin/machines/{mid}/runtime-sessions/current", "machine")
        check("machine_ops_overview", f"/v1/admin/machines/{mid}/ops-overview", "machine")
        check("machine_device_attachments", f"/v1/admin/machines/{mid}/device-attachments/current", "machine")
        check("fleet_ops_overview", "/v1/admin/machines/ops-overview?limit=5", "items")

    fail = sum(1 for c in checks if not c.get("pass"))
    write_json(out / "DATABASE_STATE_VERIFICATION.json", {"checks": checks, "fail_count": fail})
    (out / "DATABASE_STATE_VERIFICATION.md").write_text(
        "# Database State Verification\n\n" + "\n".join(f"- {c['name']}: {'PASS' if c['pass'] else 'FAIL'}" for c in checks) + "\n",
        encoding="utf-8",
    )
    return 1 if fail else 0


if __name__ == "__main__":
    raise SystemExit(main())

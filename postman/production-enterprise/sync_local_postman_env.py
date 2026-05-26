#!/usr/bin/env python3
"""Sync LOCAL Postman env from tests/e2e/production/.env.production.e2e.local (no stdout secrets)."""
from __future__ import annotations

import json
import re
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
TEMPLATE = Path(__file__).resolve().parent / "AVF_PRODUCTION_ENTERPRISE_PRIVATE.template.postman_environment.json"
LOCAL = Path(__file__).resolve().parent / "AVF_PRODUCTION_ENTERPRISE_LOCAL.postman_environment.json"
DOTENV = REPO / "tests" / "e2e" / "production" / ".env.production.e2e.local"

MAP = {
    "adminEmail": ("ADMIN_EMAIL", "E2E_PROD_ADMIN_EMAIL"),
    "adminPassword": ("ADMIN_PASSWORD", "E2E_PROD_ADMIN_PASSWORD"),
    "accessToken": ("ADMIN_TOKEN", "E2E_PROD_ADMIN_TOKEN"),
    "webhookSecret": ("COMMERCE_PAYMENT_WEBHOOK_SECRET", "E2E_PROD_PAYMENT_WEBHOOK_SECRET"),
    "mediaSha256": ("PROD_E2E_MEDIA_SHA256",),
    "machineId": ("MACHINE_ID", "E2E_PROD_MACHINE_ID"),
    "machineAccessToken": ("MACHINE_TOKEN", "E2E_PROD_MACHINE_TOKEN"),
    "machineRefreshToken": ("MACHINE_REFRESH_TOKEN", "E2E_PROD_MACHINE_REFRESH_TOKEN"),
    "mqttTopicPrefix": ("MQTT_TOPIC_PREFIX",),
}


def load_dotenv(path: Path) -> dict[str, str]:
    out: dict[str, str] = {}
    if not path.is_file():
        return out
    for line in path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        m = re.match(r"([A-Za-z_][A-Za-z0-9_]*)=(.*)", line)
        if m:
            out[m.group(1)] = m.group(2).strip().strip('"').strip("'")
    return out


def main() -> int:
    if not DOTENV.is_file():
        print("SYNC_LOCAL_ENV_FAIL: missing .env.production.e2e.local", file=sys.stderr)
        return 1
    if not TEMPLATE.is_file():
        print("SYNC_LOCAL_ENV_FAIL: missing private template", file=sys.stderr)
        return 1
    env = load_dotenv(DOTENV)
    coll = json.loads(TEMPLATE.read_text(encoding="utf-8"))
    for item in coll.get("values") or []:
        key = item.get("key")
        if key == "allowGatedWrites":
            item["value"] = "true"
        elif key == "confirmProductionWrites":
            item["value"] = env.get(
                "E2E_PRODUCTION_WRITE_CONFIRMATION",
                "I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION",
            )
        elif key in MAP:
            for src in MAP[key]:
                if env.get(src):
                    item["value"] = env[src]
                    break
    if not any(v.get("key") == "adminEmail" and v.get("value") not in ("", "<fill locally>") for v in coll["values"]):
        print("SYNC_LOCAL_ENV_FAIL: adminEmail not set in .env.production.e2e.local", file=sys.stderr)
        return 1
    LOCAL.write_text(json.dumps(coll, indent=2) + "\n", encoding="utf-8")
    print(f"SYNC_LOCAL_ENV_OK path={LOCAL.relative_to(REPO)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

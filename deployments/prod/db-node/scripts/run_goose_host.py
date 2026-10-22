#!/usr/bin/env python3
from pathlib import Path
import subprocess

APP_ENV = Path("/opt/avf-vending-api/deployments/prod/app-node/.env.app-node")
kv = {}
for raw in APP_ENV.read_text().splitlines():
    if not raw or raw.startswith("#") or "=" not in raw:
        continue
    k, v = raw.split("=", 1)
    kv[k.strip()] = v.strip().strip('"').strip("'")
backup = kv["BACKUP_DATABASE_URL"]
image = kv["APP_IMAGE_REF"]
overlay = Path("/root/.migrate.env")
overlay.write_text(f"DATABASE_URL={backup}\n")
overlay.chmod(0o600)
try:
    subprocess.check_call(
        [
            "docker",
            "run",
            "--rm",
            "--network",
            "host",
            "--env-file",
            str(overlay),
            image,
            "/app/migrate",
            "up",
        ]
    )
finally:
    overlay.unlink(missing_ok=True)
print("goose_done")

#!/usr/bin/env python3
from pathlib import Path
import shutil
import subprocess

APP_DIR = Path("/opt/avf-vending-api/deployments/prod/app-node")
APP_ENV = APP_DIR / ".env.app-node"

text = APP_ENV.read_text()
shutil.copy2(APP_ENV, APP_ENV.with_suffix(APP_ENV.suffix + ".bak-loopback-fix"))
lines = []
changed = False
for line in text.splitlines():
    if line.startswith("DATABASE_URL=") and "127.0.0.1:6432" in line:
        lines.append(line.replace("127.0.0.1:6432", "72.62.244.94:6432"))
        changed = True
        continue
    lines.append(line)
if not changed:
    raise SystemExit("DATABASE_URL loopback not found")
APP_ENV.write_text("\n".join(lines) + "\n")
print("A_RUNTIME_HOST 72.62.244.94:6432")
subprocess.check_call(
    [
        "docker",
        "compose",
        "--env-file",
        str(APP_ENV),
        "-f",
        "docker-compose.app-node.yml",
        "up",
        "-d",
        "api",
        "worker",
        "mqtt-ingest",
        "reconciler",
    ],
    cwd=str(APP_DIR),
)

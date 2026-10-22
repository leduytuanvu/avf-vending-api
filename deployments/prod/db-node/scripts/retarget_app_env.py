#!/usr/bin/env python3
from __future__ import annotations

import shutil
import sys
from pathlib import Path

APP_ENV = Path("/opt/avf-vending-api/deployments/prod/app-node/.env.app-node")
DB_ENV = Path("/opt/avf-vending-api/deployments/prod/db-node/.env.db-node")


def load_kv(path: Path) -> dict[str, str]:
    out: dict[str, str] = {}
    for raw in path.read_text().splitlines():
        if not raw or raw.startswith("#") or "=" not in raw:
            continue
        k, v = raw.split("=", 1)
        out[k.strip()] = v.strip().strip('"').strip("'")
    return out


def main() -> int:
    mode = sys.argv[1] if len(sys.argv) > 1 else ""
    db = load_kv(DB_ENV)
    user = db["POSTGRES_USER"]
    password = db["POSTGRES_PASSWORD"]
    name = db["POSTGRES_DB"]
    if mode == "a":
        # Production config rejects localhost DATABASE_URL. App containers also
        # cannot reach host loopback, so both nodes use VPS A:6432.
        runtime = f"postgres://{user}:{password}@72.62.244.94:6432/{name}?sslmode=disable"
        backup = f"postgres://{user}:{password}@127.0.0.1:5432/{name}?sslmode=disable"
    elif mode == "b":
        runtime = f"postgres://{user}:{password}@72.62.244.94:6432/{name}?sslmode=disable"
        backup = runtime
    else:
        sys.exit("usage: retarget_app_env.py a|b")
    text = APP_ENV.read_text()
    shutil.copy2(APP_ENV, APP_ENV.with_suffix(APP_ENV.suffix + ".bak-before-selfhost"))
    lines = []
    wrote_runtime = False
    wrote_backup = False
    old_runtime = None
    for line in text.splitlines():
        if line.startswith("DATABASE_URL="):
            old_runtime = line
            lines.append(f"# previous {line}")
            lines.append(f"DATABASE_URL={runtime}")
            wrote_runtime = True
            continue
        if line.startswith("BACKUP_DATABASE_URL="):
            lines.append(f"# previous {line}")
            lines.append(f"BACKUP_DATABASE_URL={backup}")
            wrote_backup = True
            continue
        lines.append(line)
    if not wrote_runtime:
        lines.append(f"DATABASE_URL={runtime}")
    if not wrote_backup:
        lines.append(f"BACKUP_DATABASE_URL={backup}")
    APP_ENV.write_text("\n".join(lines) + "\n")
    if mode == "a":
        Path("/root/node-b-db.env").write_text(
            f"DATABASE_URL={runtime.replace('127.0.0.1', '72.62.244.94')}\n"
            f"BACKUP_DATABASE_URL={runtime.replace('127.0.0.1', '72.62.244.94')}\n"
        )
        Path("/root/node-b-db.env").chmod(0o600)
    print("retargeted", mode, "previous_commented", bool(old_runtime))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

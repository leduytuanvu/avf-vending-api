#!/usr/bin/env python3
from __future__ import annotations

import os
import subprocess
import sys
from pathlib import Path

APP_DIR = Path("/opt/avf-vending-api/deployments/prod/app-node")
APP_ENV = APP_DIR / ".env.app-node"
APPLY = Path("/root/apply_env_fragment.py")
if not APPLY.exists():
    APPLY = Path("/opt/avf-vending-api/deployments/prod/db-node/scripts/apply_env_fragment.py")
FRAG = Path("/root/node-b-db.env")


def load_kv(path: Path) -> dict[str, str]:
    out: dict[str, str] = {}
    for raw in path.read_text().splitlines():
        if not raw or raw.startswith("#") or "=" not in raw:
            continue
        k, v = raw.split("=", 1)
        out[k.strip()] = v.strip().strip('"').strip("'")
    return out


def hostport(url: str) -> str:
    rest = url.split("://", 1)[1]
    return rest.split("@", 1)[1].split("/", 1)[0]


def compose(args: list[str]) -> None:
    subprocess.check_call(
        [
            "docker",
            "compose",
            "--env-file",
            str(APP_ENV),
            "-f",
            "docker-compose.app-node.yml",
            *args,
        ],
        cwd=str(APP_DIR),
    )


def main() -> int:
    if not FRAG.exists():
        print("missing fragment", FRAG, file=sys.stderr)
        return 2
    subprocess.check_call(["python3", str(APPLY), str(FRAG)])
    kv = load_kv(APP_ENV)
    runtime = kv.get("DATABASE_URL", "")
    print("B_RUNTIME_HOST", hostport(runtime) if runtime else "missing")
    if "72.62.244.94:6432" not in runtime or "supabase" in runtime.lower():
        print("refusing: DATABASE_URL is not VPS A pgbouncer", file=sys.stderr)
        return 2
    print("starting writers")
    compose(["up", "-d", "api", "worker", "mqtt-ingest", "reconciler"])
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

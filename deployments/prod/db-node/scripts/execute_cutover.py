#!/usr/bin/env python3
"""Dump current DATABASE_URL into local Postgres. Does not rewrite app env."""
from __future__ import annotations

import datetime as dt
import os
import re
import subprocess
import sys
from pathlib import Path
from urllib.parse import urlsplit, urlunsplit, parse_qsl, urlencode

APP_ENV = Path("/opt/avf-vending-api/deployments/prod/app-node/.env.app-node")
DB_ENV = Path("/opt/avf-vending-api/deployments/prod/db-node/.env.db-node")
DUMP_DIR = Path("/opt/avf-backups/selfhost-cutover")


def load_kv(path: Path) -> dict[str, str]:
    out: dict[str, str] = {}
    for raw in path.read_text().splitlines():
        if not raw or raw.startswith("#") or "=" not in raw:
            continue
        k, v = raw.split("=", 1)
        out[k.strip()] = v.strip().strip('"').strip("'")
    return out


def psql_url(raw: str) -> str:
    parts = urlsplit(raw)
    query = [(k, v) for k, v in parse_qsl(parts.query, keep_blank_values=True) if k.lower() in {"sslmode"}]
    if not query:
        query = [("sslmode", "require")]
    return urlunsplit((parts.scheme, parts.netloc, parts.path, urlencode(query), ""))


def redact(url: str) -> str:
    return re.sub(r"://[^@]*@", "://***@", url)


def run(cmd: list[str], env: dict[str, str] | None = None) -> None:
    print("+", " ".join(redact(c) if "://" in c else c for c in cmd), flush=True)
    subprocess.check_call(cmd, env=env)


def main() -> int:
    app = load_kv(APP_ENV)
    db = load_kv(DB_ENV)
    source = psql_url(app["DATABASE_URL"])
    local_user = db["POSTGRES_USER"]
    local_pass = db["POSTGRES_PASSWORD"]
    local_name = db["POSTGRES_DB"]
    target = f"postgres://{local_user}:{local_pass}@127.0.0.1:5432/{local_name}?sslmode=disable"
    DUMP_DIR.mkdir(parents=True, exist_ok=True)
    stamp = dt.datetime.utcnow().strftime("%Y%m%dT%H%M%SZ")
    dump_path = DUMP_DIR / f"supabase-{stamp}.dump"
    print("source", redact(source), flush=True)
    print("target", redact(target), flush=True)
    print("dump", dump_path, flush=True)
    src_file = Path("/root/.cutover-src.url")
    tgt_file = Path("/root/.cutover-tgt.url")
    src_file.write_text(source)
    tgt_file.write_text(target)
    src_file.chmod(0o600)
    tgt_file.chmod(0o600)
    run(
        [
            "docker",
            "run",
            "--rm",
            "--network",
            "host",
            "-v",
            f"{src_file}:/src.url:ro",
            "-v",
            f"{DUMP_DIR}:/dump",
            "postgres:17-alpine",
            "sh",
            "-c",
            f'pg_dump --format=custom --compress=9 --no-owner --no-privileges --file /dump/{dump_path.name} "$(cat /src.url)"',
        ]
    )
    if os.environ.get("CONFIRM_MANAGED_RESTORE") != "managed-production-restore":
        print("dump complete; set CONFIRM_MANAGED_RESTORE=managed-production-restore to restore")
        return 0
    run(
        [
            "docker",
            "run",
            "--rm",
            "--network",
            "host",
            "-v",
            f"{tgt_file}:/tgt.url:ro",
            "-v",
            f"{DUMP_DIR}:/dump",
            "postgres:17-alpine",
            "sh",
            "-c",
            f'pg_restore --clean --if-exists --no-owner --no-privileges --exit-on-error --dbname="$(cat /tgt.url)" /dump/{dump_path.name}',
        ]
    )
    print("restore complete", flush=True)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

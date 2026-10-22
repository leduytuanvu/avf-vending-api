#!/usr/bin/env python3
from __future__ import annotations

import subprocess
from pathlib import Path

DUMP = Path("/opt/avf-backups/selfhost-cutover/supabase-20261021T191101Z.dump")
LIST = Path("/opt/avf-backups/selfhost-cutover/restore.list")
TGT = Path("/root/.cutover-tgt.url")
SKIP = (
    "supabase_vault",
    "pgbouncer",
    "grant_pg_cron_access",
    "grant_pg_graphql_access",
    "grant_pg_net_access",
    "pgrst_ddl_watch",
    "pgrst_drop_watch",
    "set_graphql_placeholder",
    " SCHEMA - vault ",
    " SCHEMA - auth ",
    " SCHEMA - graphql ",
    " SCHEMA - graphql_public ",
    " SCHEMA - realtime ",
    " SCHEMA - storage ",
    " SCHEMA - pgbouncer ",
    " TABLE auth ",
    " TABLE graphql",
    " TABLE realtime ",
    " TABLE storage ",
    " TABLE vault ",
    " FUNCTION auth ",
    " FUNCTION graphql",
    " FUNCTION realtime ",
    " FUNCTION storage ",
    " FUNCTION vault ",
    " FUNCTION pgbouncer",
)


def main() -> int:
    raw = subprocess.check_output(
        [
            "docker",
            "run",
            "--rm",
            "-v",
            "/opt/avf-backups/selfhost-cutover:/dump",
            "postgres:17-alpine",
            "pg_restore",
            "-l",
            "/dump/supabase-20261021T191101Z.dump",
        ],
        text=True,
    )
    kept = []
    for line in raw.splitlines():
        if any(token in line for token in SKIP):
            continue
        kept.append(line)
    LIST.write_text("\n".join(kept) + "\n")
    print("kept_lines", len(kept), "of", len(raw.splitlines()), flush=True)
    cmd = [
        "docker",
        "run",
        "--rm",
        "--network",
        "host",
        "-v",
        f"{TGT}:/tgt.url:ro",
        "-v",
        "/opt/avf-backups/selfhost-cutover:/dump",
        "postgres:17-alpine",
        "sh",
        "-c",
        'pg_restore --clean --if-exists --no-owner --no-privileges --use-list=/dump/restore.list --dbname="$(cat /tgt.url)" /dump/supabase-20261021T191101Z.dump',
    ]
    subprocess.check_call(cmd)
    print("restore_ok", flush=True)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
import subprocess
from pathlib import Path

tgt = Path("/root/.cutover-tgt.url").read_text().strip()
sql = """
select 'machines' as rel, count(*)::text from public.machines
union all select 'orders', count(*)::text from public.orders
union all select 'payments', count(*)::text from public.payments
union all select 'goose', count(*)::text from public.goose_db_version;
"""
print(
    subprocess.check_output(
        [
            "docker",
            "run",
            "--rm",
            "--network",
            "host",
            "-v",
            "/root/.cutover-tgt.url:/tgt.url:ro",
            "postgres:17-alpine",
            "sh",
            "-c",
            'psql "$(cat /tgt.url)" -c "' + sql.replace("\n", " ") + '"',
        ],
        text=True,
    )
)

#!/usr/bin/env python3
from pathlib import Path
import re
import subprocess
import sys

env = Path("/opt/avf-vending-api/deployments/prod/app-node/.env.app-node")
url = None
for line in env.read_text().splitlines():
    if line.startswith("DATABASE_URL="):
        url = line.split("=", 1)[1].strip().strip('"')
        break
if not url:
    sys.exit("DATABASE_URL missing")
redacted = re.sub(r"://[^@]*@", "://***@", url)
print("source", redacted)
cmd = [
    "psql",
    url,
    "-Atqc",
    "select pg_size_pretty(pg_database_size(current_database()));",
]
print(subprocess.check_output(cmd, text=True).strip())

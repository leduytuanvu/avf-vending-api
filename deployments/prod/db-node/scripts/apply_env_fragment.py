#!/usr/bin/env python3
from pathlib import Path
import shutil
import sys

app = Path("/opt/avf-vending-api/deployments/prod/app-node/.env.app-node")
frag = Path(sys.argv[1] if len(sys.argv) > 1 else "/root/node-b-db.env")
kv = {}
for line in frag.read_text().splitlines():
    if "=" in line:
        k, v = line.split("=", 1)
        kv[k.strip()] = v.strip()
shutil.copy2(app, app.with_suffix(app.suffix + ".bak-before-selfhost"))
lines = []
wrote = set()
for line in app.read_text().splitlines():
    hit = False
    for key in kv:
        if line.startswith(key + "="):
            lines.append("# previous " + line)
            lines.append(f"{key}={kv[key]}")
            wrote.add(key)
            hit = True
            break
    if not hit:
        lines.append(line)
for key, val in kv.items():
    if key not in wrote:
        lines.append(f"{key}={val}")
app.write_text("\n".join(lines) + "\n")
print("applied", sorted(kv))

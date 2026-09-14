#!/usr/bin/env python3
"""Run AVF000132 slot repair on production VPS via SSH."""
from __future__ import annotations

import json
import os
import sys
import urllib.parse
from pathlib import Path

import paramiko

HOST = os.environ.get("PROD_SSH_HOST", "72.62.244.94")
USER = os.environ.get("PROD_SSH_USER", "root")
PW = os.environ.get("SSH_PASS", "")
SCRIPT_DIR = Path(__file__).resolve().parent
LAYOUT_PATH = SCRIPT_DIR / "output" / "avf000132-machine-layout.json"
MACHINE_ID = "01a089ec-c7bb-7e0d-83a9-6f599f061f12"
LAYOUT_ID = "01a096ed-f77b-75c3-ba0b-4eda79639029"


def run(client: paramiko.SSHClient, cmd: str, timeout: int = 600) -> tuple[int, str, str]:
    print(f"--- {cmd[:160]}...")
    _, stdout, stderr = client.exec_command(cmd, timeout=timeout)
    out = stdout.read().decode("utf-8", errors="replace")
    err = stderr.read().decode("utf-8", errors="replace")
    code = stdout.channel.recv_exit_status()
    if out:
        print(out)
    if err:
        print(err, file=sys.stderr)
    return code, out, err


def main() -> int:
    if not PW:
        print("SSH_PASS environment variable is required", file=sys.stderr)
        return 2
    apply = "--apply" in sys.argv
    dry = not apply
    layout = json.loads(LAYOUT_PATH.read_text(encoding="utf-8"))

    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(HOST, username=USER, password=PW, timeout=60)

    code, out, _ = run(
        client,
        "docker ps -q --filter name=avf-vending-prod-app-a-api | sed -n 1p",
    )
    api_cid = out.strip().splitlines()[0] if out.strip() else ""
    if not api_cid:
        print("API container not found", file=sys.stderr)
        return 2

    code, out, _ = run(
        client,
        f"docker inspect {api_cid} --format '{{{{range .Config.Env}}}}{{{{println .}}}}{{{{end}}}}' | grep '^DATABASE_URL='",
    )
    db_url = ""
    for line in out.splitlines():
        if line.startswith("DATABASE_URL="):
            db_url = line.split("=", 1)[1]
            break
    if not db_url:
        print("DATABASE_URL missing in api container", file=sys.stderr)
        return 2
    print("DATABASE_URL loaded (redacted)")

    # Upload layout json
    sftp = client.open_sftp()
    remote_layout = "/tmp/avf000132-machine-layout.json"
    sftp.put(str(LAYOUT_PATH), remote_layout)
    sftp.close()

    # Build inline repair via psql + python3 on server (python3 available on ubuntu host)
    remote_py = "/tmp/repair_avf000132_slots_from_db.py"
    sftp = client.open_sftp()
    sftp.put(str(SCRIPT_DIR / "repair_avf000132_slots_from_db.py"), remote_py)
    sftp.close()

    flag = "--dry-run" if dry else "--apply"
    escaped_url = db_url.replace("'", "'\"'\"'")
    cmd = (
        f"export DATABASE_URL='{escaped_url}'; "
        f"python3 {remote_py} --layout {remote_layout} {flag}"
    )
    code, _, _ = run(client, cmd, timeout=900)
    client.close()
    return code


if __name__ == "__main__":
    raise SystemExit(main())

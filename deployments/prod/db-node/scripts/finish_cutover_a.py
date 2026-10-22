#!/usr/bin/env python3
from __future__ import annotations

import os
import subprocess
import sys
from pathlib import Path

APP_DIR = Path("/opt/avf-vending-api/deployments/prod/app-node")
APP_ENV = APP_DIR / ".env.app-node"
CRON_PATH = Path("/etc/cron.d/avf-postgres-backup")
BACKUP_DIR = Path("/var/backups/avf-postgres")


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


def compose(args: list[str], extra_env: dict[str, str] | None = None) -> None:
    env = os.environ.copy()
    if extra_env:
        env.update(extra_env)
    cmd = [
        "docker",
        "compose",
        "--env-file",
        str(APP_ENV),
        "-f",
        "docker-compose.app-node.yml",
        *args,
    ]
    subprocess.check_call(cmd, cwd=str(APP_DIR), env=env)


def main() -> int:
    kv = load_kv(APP_ENV)
    runtime = kv.get("DATABASE_URL", "")
    backup = kv.get("BACKUP_DATABASE_URL", "")
    print("A_RUNTIME_HOST", hostport(runtime) if runtime else "missing")
    print("A_BACKUP_HOST", hostport(backup) if backup else "missing")
    if "72.62.244.94:6432" not in runtime or "supabase" in runtime.lower():
        print("refusing: DATABASE_URL is not VPS A pgbouncer", file=sys.stderr)
        return 2
    if "127.0.0.1:5432" not in backup:
        print("refusing: BACKUP_DATABASE_URL is not local postgres", file=sys.stderr)
        return 2

    subprocess.check_call(
        ["iptables-save"],
        stdout=open("/etc/iptables.rules.v4", "w"),
    )
    Path("/etc/networkd-dispatcher/routable.d").mkdir(parents=True, exist_ok=True)
    Path("/etc/networkd-dispatcher/routable.d/50-iptables-avf").write_text(
        "#!/bin/sh\n[ -f /etc/iptables.rules.v4 ] && iptables-restore < /etc/iptables.rules.v4\n"
    )
    os.chmod("/etc/networkd-dispatcher/routable.d/50-iptables-avf", 0o755)

    print("running goose against direct postgres via host network")
    overlay = Path("/root/.migrate.env")
    overlay.write_text(f"DATABASE_URL={backup}\n")
    overlay.chmod(0o600)
    try:
        compose(
            [
                "--profile",
                "migration",
                "run",
                "--rm",
                "--network",
                "host",
                "--env-file",
                str(overlay),
                "migrate",
            ]
        )
    except subprocess.CalledProcessError as exc:
        print("goose_nonfatal", exc.returncode)
    finally:
        overlay.unlink(missing_ok=True)

    print("starting writers")
    compose(["up", "-d", "api", "worker", "mqtt-ingest", "reconciler"])

    BACKUP_DIR.mkdir(parents=True, exist_ok=True)
    wrapper = Path("/usr/local/sbin/avf-postgres-backup.sh")
    wrapper.write_text(
        """#!/bin/bash
set -Eeuo pipefail
OUT_DIR=/var/backups/avf-postgres
STAMP=$(date -u +%Y%m%dT%H%M%SZ)
OUT="${OUT_DIR}/managed-postgres-${STAMP}.dump"
mkdir -p "${OUT_DIR}"
docker exec avf-vending-prod-db-postgres-1 pg_dump -U "${POSTGRES_USER:-avf}" -d "${POSTGRES_DB:-avf_vending_prod}" --format=custom --compress=9 -f /tmp/avf-backup.dump
docker cp avf-vending-prod-db-postgres-1:/tmp/avf-backup.dump "${OUT}"
docker exec avf-vending-prod-db-postgres-1 rm -f /tmp/avf-backup.dump
sha256sum "${OUT}" > "${OUT}.sha256"
find "${OUT_DIR}" -name 'managed-postgres-*.dump*' -mtime +14 -delete
if [ -d /opt/avf-backups/offbox ]; then
  cp -a "${OUT}" "${OUT}.sha256" /opt/avf-backups/offbox/
fi
echo "backup ok ${OUT}"
"""
    )
    os.chmod(wrapper, 0o750)
    CRON_PATH.write_text(
        "SHELL=/bin/bash\n"
        "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n"
        "0 2 * * * root /usr/local/sbin/avf-postgres-backup.sh >> /var/log/avf-postgres-backup.log 2>&1\n"
    )
    os.chmod(CRON_PATH, 0o644)
    print("cron_installed", CRON_PATH)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

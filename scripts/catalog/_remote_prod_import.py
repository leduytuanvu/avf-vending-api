#!/usr/bin/env python3
"""Run catalog bootstrap on production VPS via SSH. Requires SSH_PASS env var."""
from __future__ import annotations

import os
import sys
import paramiko

HOST = os.environ.get("PROD_SSH_HOST", "72.62.244.94")
USER = os.environ.get("PROD_SSH_USER", "root")
PW = os.environ.get("SSH_PASS", "")
REMOTE_ROOT = os.environ.get("PROD_DEPLOY_ROOT", "/opt/avf-vending-api")

# Load production secrets from the API container env (host .env may have empty DATABASE_URL).
LOAD_PROD_ENV = r"""
API_CID=$(docker ps -aq --filter name=avf-vending-prod-app-a-api | head -1)
if [ -z "$API_CID" ]; then echo "missing api container" >&2; exit 2; fi
while IFS= read -r line; do
  key="${line%%=*}"
  val="${line#*=}"
  case "$key" in
    DATABASE_URL|CLOUDINARY_CLOUD_NAME|CLOUDINARY_API_KEY|CLOUDINARY_API_SECRET|CLOUDINARY_FOLDER|MEDIA_COMPANY_ID|MEDIA_PROVIDER|APP_ENV)
      export "$key=$val"
      ;;
  esac
done < <(docker inspect "$API_CID" --format '{{range .Config.Env}}{{println .}}{{end}}')
"""


def run(cmd: str, timeout: int = 3600) -> int:
    if not PW:
        print("SSH_PASS environment variable is required", file=sys.stderr)
        return 2
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(HOST, username=USER, password=PW, timeout=60)
    print(f"--- remote: {cmd[:120]}...")
    _, stdout, stderr = client.exec_command(cmd, timeout=timeout)
    out = stdout.read().decode("utf-8", errors="replace")
    err = stderr.read().decode("utf-8", errors="replace")
    code = stdout.channel.recv_exit_status()
    if out:
        print(out)
    if err:
        print(err, file=sys.stderr)
    client.close()
    return code


def main() -> None:
    phase = sys.argv[1] if len(sys.argv) > 1 else "import"
    base = f"""
set -Eeuo pipefail
cd {REMOTE_ROOT}
set -a
source deployments/prod/app-node/.env.app-node
set +a
{LOAD_PROD_ENV}
export MEDIA_UPLOAD_ENABLED=true
"""
    if phase == "dbcheck":
        cmd = base + r"""
grep -n 'DATABASE_URL' deployments/prod/app-node/.env.app-node | sed 's/=.*/=***REDACTED***/' | head -5
docker ps -a --format '{{.Names}}' | grep -E 'api|worker' || true
cid=$(docker ps -aq --filter name=avf-vending-prod-app-a-api | head -1)
if [ -n "$cid" ]; then docker inspect "$cid" --format '{{range .Config.Env}}{{println .}}{{end}}' | grep -E '^DATABASE_URL=' | sed 's/=.*/=***REDACTED***/'; else echo NO_API_CONTAINER; fi
cd deployments/prod/app-node && docker compose --env-file .env.app-node -f docker-compose.app-node.yml run --rm --no-deps api printenv DATABASE_URL 2>/dev/null | awk '{print "compose_len=" length($0)}' || echo compose_run_failed
"""
    elif phase == "envcheck":
        cmd = base + r"""
grep -E '^(APP_ENV|DATABASE_URL|CLOUDINARY_CLOUD_NAME|MEDIA_COMPANY_ID|MEDIA_PROVIDER|MEDIA_UPLOAD_ENABLED)=' deployments/prod/app-node/.env.app-node | sed 's/=.*/=***REDACTED***/'
if [ -n "${DATABASE_URL:-}" ]; then echo RUNTIME_DATABASE_URL=set; else echo RUNTIME_DATABASE_URL=empty; fi
env | grep '^DATABASE_' | sed 's/=.*/=***REDACTED***/' || true
"""
    elif phase == "preflight":
        cmd = base + """
./bin/catalog-bootstrap --manifest docs/catalog-import-final.json --evidence-dir .catalog-bootstrap-evidence --environment production --preflight-only
./bin/catalog-bootstrap --manifest docs/catalog-import-final.json --evidence-dir .catalog-bootstrap-evidence --environment production --validate-only
"""
    elif phase == "import":
        cmd = base + """
export CONFIRM_CATALOG_BOOTSTRAP=CONFIRM-CATALOG-BOOTSTRAP-PRODUCTION
LOG=.catalog-bootstrap-evidence/import-run.log
nohup ./bin/catalog-bootstrap --manifest docs/catalog-import-final.json --evidence-dir .catalog-bootstrap-evidence --environment production \
  --upload-images --import-taxonomy --import-products --import-prices \
  --confirm-production=CONFIRM-CATALOG-BOOTSTRAP-PRODUCTION --resume >"$LOG" 2>&1 &
echo $! > .catalog-bootstrap-evidence/import.pid
echo STARTED_PID=$(cat .catalog-bootstrap-evidence/import.pid)
"""
    elif phase == "log":
        cmd = (
            f"cd {REMOTE_ROOT} && tail -n 80 .catalog-bootstrap-evidence/import-run.log 2>/dev/null; "
            "python3 -c \"import json;from pathlib import Path;"
            "p=Path('.catalog-bootstrap-evidence/import-state.json');"
            "d=json.loads(p.read_text()) if p.exists() else {};"
            "items=d.get('items',{});"
            "states={};"
            "[states.update({v.get('state','?'): states.get(v.get('state','?'),0)+1}) for v in items.values()];"
            "print('items',len(items),'states',states)\"; "
            "ls -la .catalog-bootstrap-evidence/*.json 2>/dev/null | tail -12"
        )
    elif phase == "status":
        cmd = base + r"""
if [ -f .catalog-bootstrap-evidence/import.pid ]; then pid=$(cat .catalog-bootstrap-evidence/import.pid); if kill -0 "$pid" 2>/dev/null; then echo RUNNING pid=$pid; else echo FINISHED pid=$pid; fi; else echo NO_PID; fi
tail -n 50 .catalog-bootstrap-evidence/import-run.log 2>/dev/null || true
wc -l .catalog-bootstrap-evidence/import-state.json 2>/dev/null || true
"""
    elif phase == "cloudinary-count":
        cmd = base + r"""
cd scripts/ops/cloudinary
python3 cloudinary_ops.py inventory-readonly --summary 2>/dev/null || bash inventory.sh --summary-only 2>/dev/null || echo cloudinary_inventory_unavailable
"""
    elif phase == "verify":
        cmd = base + """
./bin/catalog-bootstrap --manifest docs/catalog-import-final.json --evidence-dir .catalog-bootstrap-evidence --environment production --verify-only
"""
    else:
        print(f"unknown phase: {phase}", file=sys.stderr)
        sys.exit(2)
    sys.exit(run(cmd))


if __name__ == "__main__":
    main()

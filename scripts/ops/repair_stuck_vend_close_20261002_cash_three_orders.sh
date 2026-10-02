#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SQL="${SCRIPT_DIR}/repair_stuck_vend_close_20261002_cash_three_orders.sql"
DRY_RUN="${1:-1}"
if [[ -z "${DATABASE_URL:-}" ]]; then
  echo "DATABASE_URL is required" >&2
  exit 1
fi
psql "$DATABASE_URL" -v dry_run="$DRY_RUN" -f "$SQL"

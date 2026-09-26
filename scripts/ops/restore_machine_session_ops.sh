#!/usr/bin/env bash
# Ops recovery for machine session / offline cash storefront blocker.
# Requires DATABASE_URL and psql on PATH.
set -euo pipefail

MACHINE_ID="${LOOKUP_MACHINE_ID:-01a089ec-c7bb-7e0d-83a9-6f599f061f12}"
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

echo "=== diagnose machine session: ${MACHINE_ID} ==="
psql "$DATABASE_URL" -v lookup_machine_id="$MACHINE_ID" \
  -f "$ROOT/scripts/ops/diagnose_machine_session.sql"

echo "=== enable offline_cash_sale_allowed ==="
psql "$DATABASE_URL" -v lookup_machine_id="$MACHINE_ID" \
  -f "$ROOT/scripts/ops/enable_offline_cash_sale_machine.sql"

echo "=== done: restart app on device or wait for bootstrap sync ==="

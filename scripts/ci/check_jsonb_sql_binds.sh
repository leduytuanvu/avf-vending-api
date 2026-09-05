#!/usr/bin/env bash
# Reject uncast sqlc jsonb parameter binds in commerce/payment SQL (pgx exec mode / SQLSTATE 22P02).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
FILES=(
  "${ROOT}/db/queries/commerce.sql"
  "${ROOT}/db/queries/checkout_quotes.sql"
  "${ROOT}/db/queries/commerce_timelines_refunds.sql"
  "${ROOT}/db/queries/financial_correctness.sql"
  "${ROOT}/db/queries/financial_ledger.sql"
  "${ROOT}/db/queries/payment_reconciliation.sql"
)

hits=""
for f in "${FILES[@]}"; do
  if [[ ! -f "${f}" ]]; then
    echo "check_jsonb_sql_binds: missing ${f}" >&2
    exit 1
  fi
  file_hits="$(rg -n "sqlc\\.(arg|narg)\\('[^']+'\\)::jsonb" "${f}" || true)"
  if [[ -n "${file_hits}" ]]; then
    hits+="${file_hits}"$'\n'
  fi
done

if [[ -n "${hits}" ]]; then
  echo "check_jsonb_sql_binds: uncast jsonb binds (use ::text cast first):" >&2
  echo "${hits}" >&2
  exit 1
fi

# machine_pricing_snapshot inserts must cast via ::text before ::jsonb
for f in "${ROOT}/db/queries/checkout_quotes.sql" "${ROOT}/db/queries/commerce.sql"; do
  if rg -n "machine_pricing_snapshot" "${f}" | rg -q "INSERT" && ! rg -q "machine_pricing_snapshot.*::text" "${f}"; then
    echo "check_jsonb_sql_binds: ${f} must cast machine_pricing_snapshot via ::text" >&2
    exit 1
  fi
done

echo "check_jsonb_sql_binds: ok"

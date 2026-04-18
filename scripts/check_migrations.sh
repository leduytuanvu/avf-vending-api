#!/usr/bin/env bash
# Deterministic migration layout checks (no database required).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

MIG_DIR="${ROOT}/migrations"
if [[ ! -d "$MIG_DIR" ]]; then
  fail "migrations directory missing: $MIG_DIR"
fi

echo "== migration file checks =="

mapfile -t files < <(find "$MIG_DIR" -maxdepth 1 -type f -name '*.sql' | LC_ALL=C sort)
if [[ ${#files[@]} -eq 0 ]]; then
  fail "no .sql files under migrations/"
fi

prev_num=-1
for f in "${files[@]}"; do
  base="$(basename "$f")"
  if [[ ! "$base" =~ ^[0-9]{5}_.+\.sql$ ]]; then
    fail "migration must be named NNNNN_description.sql (5 digit prefix): $base"
  fi
  ver="${base:0:5}"
  ver_num=$((10#$ver))
  if (( prev_num >= 0 && ver_num <= prev_num )); then
    fail "migration versions must be strictly increasing: $base (previous numeric $prev_num)"
  fi
  prev_num=$ver_num

  if ! grep -q '^-- +goose Up' "$f"; then
    fail "missing '-- +goose Up' in $base"
  fi
  if ! grep -q '^-- +goose Down' "$f"; then
    fail "missing '-- +goose Down' in $base"
  fi
done

echo "OK: goose migration files look well-formed."

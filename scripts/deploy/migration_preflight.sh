#!/usr/bin/env bash
# Run migration safety verifier before remote deploy runs goose Up.
# Usage: migration_preflight.sh staging|production
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

target="${1:-}"
if [[ "${target}" != "staging" && "${target}" != "production" ]]; then
  echo "usage: migration_preflight.sh staging|production" >&2
  exit 2
fi

export DEPLOY_TARGET="${target}"
bash "${ROOT}/scripts/ci/verify_migrations.sh" --report "${ROOT}/migration-evidence/migration-safety-report.json"

#!/usr/bin/env bash
# Non-secret local validation for production release assets.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
TMP_DIR="$(mktemp -d)"
TMP_ENV="${TMP_DIR}/.env.production"

cleanup() {
	rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

cp "${ROOT}/.env.production.example" "${TMP_ENV}"

echo "==> validate_release_assets"
bash "${ROOT}/scripts/validate_release_assets.sh" "${TMP_ENV}"

echo "==> docker compose config"
docker compose --env-file "${TMP_ENV}" -f "${ROOT}/docker-compose.prod.yml" config >/dev/null

echo "test_release_assets: PASS"

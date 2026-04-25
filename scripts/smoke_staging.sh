#!/usr/bin/env bash
# Staging HTTP smoke: health, version, swagger (if present), critical OpenAPI paths.
# Usage:
#   STAGING_BASE_URL=https://staging-api.ldtv.dev bash scripts/smoke_staging.sh
# Optional:
#   STAGING_SMOKE_CHECK_SWAGGER=1   fail if /swagger/doc.json is not HTTP 200
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HTTP_SMOKE="${ROOT}/deployments/prod/shared/scripts/smoke_http.sh"

fail() {
	echo "smoke_staging: $*" >&2
	exit 1
}

[[ -f "${HTTP_SMOKE}" ]] || fail "missing ${HTTP_SMOKE}"

BASE_URL="${STAGING_BASE_URL:-}"
if [[ -z "${BASE_URL}" ]]; then
	if [[ -n "${STAGING_SMOKE_API_DOMAIN:-}" ]]; then
		BASE_URL="https://${STAGING_SMOKE_API_DOMAIN}"
	else
		BASE_URL="https://staging-api.ldtv.dev"
	fi
fi
BASE_URL="${BASE_URL%/}"

echo "smoke_staging: base_url=${BASE_URL}"

bash "${HTTP_SMOKE}" "staging /health/live" "${BASE_URL}/health/live" '^ok$'
bash "${HTTP_SMOKE}" "staging /health/ready" "${BASE_URL}/health/ready" '^ok$'
bash "${HTTP_SMOKE}" "staging /version" "${BASE_URL}/version" '"version"[[:space:]]*:'

SWAGGER_TMP="$(mktemp)"
trap 'rm -f "${SWAGGER_TMP}"' EXIT
code="$(
	curl -sS -o "${SWAGGER_TMP}" -w "%{http_code}" \
		-H "Accept: application/json" \
		"${BASE_URL}/swagger/doc.json" || printf "000"
)"
paths_check=0
if [[ "${code}" == "200" ]]; then
	echo "smoke_staging: /swagger/doc.json -> 200"
	paths_check=1
elif [[ "${code}" == "404" || "${code}" == "401" ]]; then
	echo "smoke_staging: /swagger/doc.json -> ${code} (swagger likely disabled; ok)"
elif [[ "${STAGING_SMOKE_CHECK_SWAGGER:-0}" == "1" ]]; then
	fail "/swagger/doc.json unexpected status ${code}"
else
	echo "smoke_staging: /swagger/doc.json -> ${code} (non-fatal unless STAGING_SMOKE_CHECK_SWAGGER=1)"
fi

if [[ "${paths_check}" == "1" ]]; then
	export SWAGGER_JSON="${SWAGGER_TMP}"
	python3 - <<'PY'
import json, os, pathlib, sys
p = pathlib.Path(os.environ["SWAGGER_JSON"])
doc = json.loads(p.read_text(encoding="utf-8"))
paths = set((doc.get("paths") or {}).keys())
need = [
  "/v1/setup/activation-codes/claim",
  "/v1/machines/{machineId}/sale-catalog",
  "/v1/device/machines/{machineId}/events/reconcile",
  "/v1/commerce/orders/{orderId}/refunds",
]
missing = [n for n in need if n not in paths]
if missing:
    print("smoke_staging: missing OpenAPI paths:", file=sys.stderr)
    for m in missing:
        print(" ", m, file=sys.stderr)
    sys.exit(1)
print("smoke_staging: critical OpenAPI paths present")
PY
fi

echo "smoke_staging: PASS"

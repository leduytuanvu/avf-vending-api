#!/usr/bin/env bash
# Authenticated production smoke for sites/machines/products/assignments filter APIs.
# Usage: ADMIN_EMAIL=... ADMIN_PASSWORD=... bash scripts/e2e/production-feature-smoke.sh
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=lib/common.sh
source "${ROOT}/scripts/e2e/lib/common.sh"

BASE_URL="${BASE_URL:-https://api.ldtv.dev}"
BASE_URL="${BASE_URL%/}"
export BASE_URL

e2e_require_cmd curl jq
e2e_init_run_dir "production-feature-smoke"

FAILURES=0
pass() { echo "PASS  $*"; }
fail() { echo "FAIL  $*"; FAILURES=$((FAILURES + 1)); }

ADMIN_TOK=""
if ! ADMIN_TOK="$(e2e_admin_token 2>/dev/null)"; then
	echo "error: ADMIN_TOKEN or ADMIN_EMAIL+ADMIN_PASSWORD required" >&2
	exit 1
fi
pass "admin.auth"

declare -a PATHS=(
	"/v1/admin/sites?limit=5&search=test&region=Mi%E1%BB%81n%20Nam"
	"/v1/admin/sites?limit=5&city=Th%C3%A0nh%20ph%E1%BB%91%20H%E1%93%90%20Ch%C3%AD%20Minh"
	"/v1/admin/machines?limit=5&q=test"
	"/v1/admin/products?limit=5"
	"/v1/admin/technician-assignments?limit=5"
)

for path in "${PATHS[@]}"; do
	slug="${path//[\/?=&]/-}"
	read -r code lat < <(e2e_curl_get "feature${slug}" "${BASE_URL}${path}" "$ADMIN_TOK")
	if [[ "$code" == "200" ]]; then
		pass "${path} status=${code} (${lat}ms)"
	else
		fail "${path} expected 200 got ${code}"
	fi
done

if [[ "$FAILURES" -gt 0 ]]; then
	echo "production-feature-smoke: FAIL (${FAILURES} checks)"
	exit 1
fi
echo "production-feature-smoke: PASS"

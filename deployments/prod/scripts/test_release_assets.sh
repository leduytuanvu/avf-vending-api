#!/usr/bin/env bash
# Non-secret local validation for production release assets.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
TMP_DIR="$(mktemp -d)"
TMP_ENV="${TMP_DIR}/.env.production"
DEPLOY_WF="${REPO_ROOT}/.github/workflows/deploy-prod.yml"

cleanup() {
	rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

cp "${ROOT}/.env.production.example" "${TMP_ENV}"

bash -n "${ROOT}/scripts/smoke_prod.sh"
bash -n "${SCRIPT_DIR}/test_release_assets.sh"

echo "==> deploy-prod.yml smoke / rollback contracts"
[[ -f "${DEPLOY_WF}" ]] || {
	echo "error: missing ${DEPLOY_WF}" >&2
	exit 1
}
grep -q 'deployment-evidence/smoke-cluster-final\.json' "${DEPLOY_WF}" || {
	echo "error: deploy-prod must redirect final smoke stdout to smoke-cluster-final.json" >&2
	exit 1
}
grep -q 'deployment-evidence/smoke-cluster-final\.log' "${DEPLOY_WF}" || {
	echo "error: deploy-prod must redirect final smoke stderr to smoke-cluster-final.log" >&2
	exit 1
}
grep -q 'final-smoke-meta\.env' "${DEPLOY_WF}" || {
	echo "error: deploy-prod must write final-smoke-meta.env for rollback policy" >&2
	exit 1
}
grep -q 'evidence_format_failure' "${DEPLOY_WF}" || {
	echo "error: deploy-prod must record evidence_format_failure in final-smoke-meta" >&2
	exit 1
}
grep -qF '"phase":"evidence"' "${DEPLOY_WF}" || {
	echo "error: deploy-prod must emit release-events phase evidence on invalid final smoke JSON" >&2
	exit 1
}
grep -q 'automatic rollback skipped because failure was evidence_format_failure' "${DEPLOY_WF}" || {
	echo "error: deploy-prod must skip automatic rollback when failure is evidence-only" >&2
	exit 1
}
grep -q 'python3 -m json\.tool deployment-evidence/smoke-cluster-final\.json' "${DEPLOY_WF}" || {
	echo "error: deploy-prod must validate smoke-cluster-final.json with python3 -m json.tool" >&2
	exit 1
}

echo "==> validate_release_assets"
ALLOW_LEGACY_SINGLE_HOST=1 bash "${ROOT}/scripts/validate_release_assets.sh" "${TMP_ENV}"

echo "==> docker compose config"
docker compose --env-file "${TMP_ENV}" -f "${ROOT}/docker-compose.prod.yml" config >/dev/null

echo "==> smoke_prod.sh --json (local mock HTTP)"
if command -v python3 >/dev/null 2>&1; then
	MOCK_PORT_FILE="${TMP_DIR}/mock-port"
	python3 - "${MOCK_PORT_FILE}" <<'PY' &
import sys
import threading
import time
from http.server import BaseHTTPRequestHandler, HTTPServer

path_port = sys.argv[1]


class H(BaseHTTPRequestHandler):
	def log_message(self, *args):
		return

	def do_GET(self):
		p = self.path.split("?", 1)[0]
		self.send_response(200)
		if p.rstrip("/").endswith("/swagger/doc.json"):
			self.send_header("Content-Type", "application/json")
			self.end_headers()
			self.wfile.write(b'{"openapi":"3.0.0","paths":{}}\n')
		elif p.rstrip("/").endswith("/health/ready") or p.rstrip("/").endswith("/health/live"):
			self.send_header("Content-Type", "text/plain")
			self.end_headers()
			self.wfile.write(b"ok\n")
		elif p.rstrip("/").endswith("/version"):
			self.send_header("Content-Type", "application/json")
			self.end_headers()
			self.wfile.write(b'{"version":"0-local-test"}\n')
		else:
			self.end_headers()
			self.wfile.write(b"ok\n")


srv = HTTPServer(("127.0.0.1", 0), H)
open(path_port, "w", encoding="utf-8").write(str(srv.server_port))
threading.Thread(target=srv.serve_forever, daemon=True).start()
try:
	while True:
		time.sleep(3600)
except KeyboardInterrupt:
	pass
PY
	MOCK_PID=$!
	sleep 0.5
	MOCK_PORT="$(cat "${MOCK_PORT_FILE}")"
	SMOKE_JSON_OUT="${TMP_DIR}/smoke-stdout.json"
	SMOKE_LOG_OUT="${TMP_DIR}/smoke-stderr.log"
	if ! SMOKE_BASE_URL="http://127.0.0.1:${MOCK_PORT}" SMOKE_LEVEL=health bash "${ROOT}/scripts/smoke_prod.sh" --json >"${SMOKE_JSON_OUT}" 2>"${SMOKE_LOG_OUT}"; then
		kill "${MOCK_PID}" 2>/dev/null || true
		echo "error: smoke_prod.sh --json expected exit 0 against mock server (health)" >&2
		cat "${SMOKE_LOG_OUT}" >&2 || true
		exit 1
	fi
	if [[ ! -s "${SMOKE_JSON_OUT}" ]]; then
		kill "${MOCK_PID}" 2>/dev/null || true
		echo "error: smoke stdout JSON must be non-empty (health)" >&2
		exit 1
	fi
	python3 -m json.tool "${SMOKE_JSON_OUT}" >/dev/null
	if ! grep -q '"overall_status"' "${SMOKE_JSON_OUT}"; then
		kill "${MOCK_PID}" 2>/dev/null || true
		echo "error: smoke JSON must contain overall_status (health)" >&2
		exit 1
	fi
	if grep -qE '^(PASS|FAIL|SKIP|NOTE|WARN|==>)' "${SMOKE_JSON_OUT}"; then
		kill "${MOCK_PID}" 2>/dev/null || true
		echo "error: smoke JSON stdout contains human-oriented line(s) (health)" >&2
		exit 1
	fi
	if ! grep -q 'PASS:' "${SMOKE_LOG_OUT}"; then
		kill "${MOCK_PID}" 2>/dev/null || true
		echo "error: expected PASS lines on smoke stderr log (health)" >&2
		exit 1
	fi
	SMOKE_JSON_BR="${TMP_DIR}/smoke-stdout-business-readonly.json"
	SMOKE_LOG_BR="${TMP_DIR}/smoke-stderr-business-readonly.log"
	if ! SMOKE_BASE_URL="http://127.0.0.1:${MOCK_PORT}" SMOKE_LEVEL=business-readonly bash "${ROOT}/scripts/smoke_prod.sh" --json >"${SMOKE_JSON_BR}" 2>"${SMOKE_LOG_BR}"; then
		kill "${MOCK_PID}" 2>/dev/null || true
		echo "error: smoke_prod.sh --json expected exit 0 against mock server (business-readonly)" >&2
		cat "${SMOKE_LOG_BR}" >&2 || true
		exit 1
	fi
	if [[ ! -s "${SMOKE_JSON_BR}" ]]; then
		kill "${MOCK_PID}" 2>/dev/null || true
		echo "error: smoke stdout JSON must be non-empty (business-readonly)" >&2
		exit 1
	fi
	python3 -m json.tool "${SMOKE_JSON_BR}" >/dev/null
	if ! grep -q '"overall_status"' "${SMOKE_JSON_BR}"; then
		kill "${MOCK_PID}" 2>/dev/null || true
		echo "error: smoke JSON must contain overall_status (business-readonly)" >&2
		exit 1
	fi
	if grep -qE '^(PASS|FAIL|SKIP|NOTE|WARN|==>)' "${SMOKE_JSON_BR}"; then
		kill "${MOCK_PID}" 2>/dev/null || true
		echo "error: smoke JSON stdout contains human-oriented line(s) (business-readonly)" >&2
		exit 1
	fi
	kill "${MOCK_PID}" 2>/dev/null || true
	wait "${MOCK_PID}" 2>/dev/null || true
else
	echo "==> smoke_prod.sh --json: skipped (python3 not found)"
fi

echo "test_release_assets: PASS"

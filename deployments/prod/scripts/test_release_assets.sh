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

resolve_smoke_python() {
	local c
	for c in python python3; do
		if command -v "${c}" >/dev/null 2>&1 && "${c}" -c "import sys" >/dev/null 2>&1; then
			printf '%s' "${c}"
			return 0
		fi
	done
	return 1
}

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
grep -q 'smoke_semantics_failure' "${DEPLOY_WF}" || {
	echo "error: deploy-prod must classify semantics-only final smoke as smoke_semantics_failure" >&2
	exit 1
}
grep -q 'automatic rollback skipped because final smoke had no failed checks and only optional skips' "${DEPLOY_WF}" || {
	echo "error: deploy-prod must skip automatic rollback for optional-skip-only final smoke semantics" >&2
	exit 1
}
grep -q 'runtime_smoke_failure' "${DEPLOY_WF}" || {
	echo "error: deploy-prod must still classify real smoke failures as runtime_smoke_failure" >&2
	exit 1
}
grep -q 'failed_n > 0' "${DEPLOY_WF}" || {
	echo "error: deploy-prod final smoke classifier must use failed_checks length (failed_n)" >&2
	exit 1
}
grep -q '90154f32ffe0f0feabe5bd0f2bb81355ddbae7cec53c42169ab1764138171011' "${DEPLOY_WF}" || {
	echo "error: deploy-prod must guard rollback against known-incompatible prior app digest" >&2
	exit 1
}
grep -q 'python3 -m json\.tool deployment-evidence/smoke-cluster-final\.json' "${DEPLOY_WF}" || {
	echo "error: deploy-prod must validate smoke-cluster-final.json with python3 -m json.tool" >&2
	exit 1
}
grep -q 'FINAL_SMOKE_NORMALIZED_RESULT' "${DEPLOY_WF}" || {
	echo "error: deploy-prod final-smoke-meta must record FINAL_SMOKE_NORMALIZED_RESULT" >&2
	exit 1
}
if grep -qF 'exit "${smoke_rc}"' "${DEPLOY_WF}"; then
	echo "error: deploy-prod must not use raw exit \"\${smoke_rc}\" for final smoke (use JSON-normalized step exit)" >&2
	exit 1
fi
grep -qF 'exit "${FINAL_STEP_EXIT}"' "${DEPLOY_WF}" || {
	echo "error: deploy-prod final smoke must exit using validated FINAL_STEP_EXIT" >&2
	exit 1
}
grep -q 'FINAL_SMOKE_NORMALIZED_RESULT=pass' "${DEPLOY_WF}" || {
	echo "error: deploy-prod rollback/summary must recognize FINAL_SMOKE_NORMALIZED_RESULT=pass" >&2
	exit 1
}
grep -qF '"${smoke_rc}" =~ ^[0-9]+$' "${DEPLOY_WF}" || {
	echo "error: deploy-prod must validate smoke_rc is numeric after smoke_prod.sh" >&2
	exit 1
}
grep -qE 'FINAL_STEP_EXIT.*=~.*\[0-9\]' "${DEPLOY_WF}" || grep -qF 'FINAL_STEP_EXIT="${FINAL_STEP_EXIT:-1}"' "${DEPLOY_WF}" || {
	echo "error: deploy-prod must validate FINAL_STEP_EXIT before exit" >&2
	exit 1
}

echo "==> validate_release_assets"
ALLOW_LEGACY_SINGLE_HOST=1 bash "${ROOT}/scripts/validate_release_assets.sh" "${TMP_ENV}"

echo "==> docker compose config"
docker compose --env-file "${TMP_ENV}" -f "${ROOT}/docker-compose.prod.yml" config >/dev/null

	echo "==> smoke_prod.sh --json (local mock HTTP)"
if SMOKE_PY="$(resolve_smoke_python)"; then
	start_mock_http() {
		local port_file="$1"
		local openapi_mode="$2"
		"${SMOKE_PY}" - "${port_file}" "${openapi_mode}" <<'PY' &
import sys
import threading
import time
from http.server import BaseHTTPRequestHandler, HTTPServer

path_port = sys.argv[1]
openapi_mode = sys.argv[2] if len(sys.argv) > 2 else "200"


class H(BaseHTTPRequestHandler):
	def log_message(self, *args):
		return

	def do_GET(self):
		p = self.path.split("?", 1)[0]
		if p.rstrip("/").endswith("/swagger/doc.json"):
			if openapi_mode == "404":
				self.send_response(404)
				self.end_headers()
				return
			self.send_response(200)
			self.send_header("Content-Type", "application/json")
			self.end_headers()
			self.wfile.write(b'{"openapi":"3.0.0","paths":{}}\n')
			return
		self.send_response(200)
		if p.rstrip("/").endswith("/health/ready") or p.rstrip("/").endswith("/health/live"):
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
		echo $!
	}

	MOCK_PORT_FILE="${TMP_DIR}/mock-port"
	MOCK_PID="$(start_mock_http "${MOCK_PORT_FILE}" 200)"
	sleep 0.5
	MOCK_PORT="$(cat "${MOCK_PORT_FILE}")"
	SMOKE_JSON_OUT="${TMP_DIR}/smoke-stdout.json"
	SMOKE_LOG_OUT="${TMP_DIR}/smoke-stderr.log"
	if ! SMOKE_PYTHON="${SMOKE_PY}" SMOKE_BASE_URL="http://127.0.0.1:${MOCK_PORT}" SMOKE_LEVEL=health bash "${ROOT}/scripts/smoke_prod.sh" --json >"${SMOKE_JSON_OUT}" 2>"${SMOKE_LOG_OUT}"; then
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
	"${SMOKE_PY}" -m json.tool "${SMOKE_JSON_OUT}" >/dev/null
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
	if ! SMOKE_PYTHON="${SMOKE_PY}" SMOKE_BASE_URL="http://127.0.0.1:${MOCK_PORT}" SMOKE_LEVEL=business-readonly bash "${ROOT}/scripts/smoke_prod.sh" --json >"${SMOKE_JSON_BR}" 2>"${SMOKE_LOG_BR}"; then
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
	"${SMOKE_PY}" -m json.tool "${SMOKE_JSON_BR}" >/dev/null
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
	MOCK_PORT_404_FILE="${TMP_DIR}/mock-port-openapi404"
	MOCK_PID_404="$(start_mock_http "${MOCK_PORT_404_FILE}" 404)"
	sleep 0.5
	MOCK_PORT_404="$(cat "${MOCK_PORT_404_FILE}")"
	SMOKE_JSON_404="${TMP_DIR}/smoke-stdout-openapi404.json"
	SMOKE_LOG_404="${TMP_DIR}/smoke-stderr-openapi404.log"
	if ! SMOKE_PYTHON="${SMOKE_PY}" SMOKE_BASE_URL="http://127.0.0.1:${MOCK_PORT_404}" SMOKE_LEVEL=business-readonly bash "${ROOT}/scripts/smoke_prod.sh" --json >"${SMOKE_JSON_404}" 2>"${SMOKE_LOG_404}"; then
		kill "${MOCK_PID}" "${MOCK_PID_404}" 2>/dev/null || true
		echo "error: smoke_prod.sh --json expected exit 0 when OpenAPI doc is 404-only skip" >&2
		cat "${SMOKE_LOG_404}" >&2 || true
		exit 1
	fi
	if [[ ! -s "${SMOKE_JSON_404}" ]]; then
		kill "${MOCK_PID}" "${MOCK_PID_404}" 2>/dev/null || true
		echo "error: smoke stdout JSON must be non-empty (openapi 404 optional skip)" >&2
		exit 1
	fi
	"${SMOKE_PY}" -m json.tool "${SMOKE_JSON_404}" >/dev/null
	if ! "${SMOKE_PY}" - "${SMOKE_JSON_404}" <<'PY'
import json, sys

p = json.load(open(sys.argv[1], encoding="utf-8"))
assert p.get("overall_status") == "pass", p
assert p.get("final_result") == "pass", p
assert p.get("failed_checks") == [], p
assert p.get("zero_side_effects_claim") is True, p
assert any(c.get("status") == "skip" for c in p.get("checks", [])), p
PY
	then
		kill "${MOCK_PID}" "${MOCK_PID_404}" 2>/dev/null || true
		echo "error: openapi-404 business-readonly smoke JSON assertions failed" >&2
		exit 1
	fi
	if grep -qE '^(PASS|FAIL|SKIP|NOTE|WARN|==>)' "${SMOKE_JSON_404}"; then
		kill "${MOCK_PID}" "${MOCK_PID_404}" 2>/dev/null || true
		echo "error: smoke JSON stdout contains human-oriented line(s) (openapi 404)" >&2
		exit 1
	fi
	kill "${MOCK_PID}" "${MOCK_PID_404}" 2>/dev/null || true
	wait "${MOCK_PID}" 2>/dev/null || true
	wait "${MOCK_PID_404}" 2>/dev/null || true
else
	echo "==> smoke_prod.sh --json: skipped (no working python3/python in PATH)"
fi

echo "test_release_assets: PASS"

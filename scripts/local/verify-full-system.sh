#!/usr/bin/env bash
# Full-system verification wrapper (Phase 11).
# Runs offline gates by default; optional integration when env flags are set.
# Exit 0 only when all executed steps pass. Skipped steps are listed separately.
#
# Usage:
#   bash scripts/local/verify-full-system.sh
#   VERIFY_WITH_DB=1 TEST_DATABASE_URL=postgres://... bash scripts/local/verify-full-system.sh
#   VERIFY_WITH_BROKER=1 MQTT_HOST=127.0.0.1 MQTT_TOPIC_PREFIX=avf bash scripts/local/verify-full-system.sh
#   VERIFY_WITH_NEWman=1 bash scripts/local/verify-full-system.sh
#   VERIFY_WITH_WORKFLOWS=1 bash scripts/local/verify-full-system.sh
#   VERIFY_DESTRUCTIVE=1  # never set unless you explicitly want production/destructive E2E
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

REPORT_DIR="${ROOT}/docs/testing"
mkdir -p "${REPORT_DIR}"

resolve_python() {
	local c
	for c in "${PY:-}" python python3; do
		[[ -n "${c}" ]] || continue
		if command -v "${c}" >/dev/null 2>&1 && "${c}" -c "import sys" >/dev/null 2>&1; then
			echo "${c}"
			return 0
		fi
	done
	if [[ -x "/c/Python314/python.exe" ]] && /c/Python314/python.exe -c "import sys" >/dev/null 2>&1; then
		echo "/c/Python314/python.exe"
		return 0
	fi
	if [[ -x "/c/Python312/python.exe" ]] && /c/Python312/python.exe -c "import sys" >/dev/null 2>&1; then
		echo "/c/Python312/python.exe"
		return 0
	fi
	echo "verify-full-system: no working python/python3 found" >&2
	return 1
}

PY="$(resolve_python)"
export PY PYTHON="${PY}"
GO="${GO:-go}"

pass=0
fail=0
skip=0
results=()

note() {
	echo "[verify-full-system] $*"
}

run_step() {
	local label="$1"
	shift
	note "RUN: ${label}"
	set +e
	"$@"
	local ec=$?
	set -e
	if [[ "${ec}" -eq 0 ]]; then
		results+=("PASS  ${label}")
		pass=$((pass + 1))
		note "OK: ${label}"
	else
		results+=("FAIL  ${label} (exit ${ec})")
		fail=$((fail + 1))
		note "FAIL: ${label} (exit ${ec})"
	fi
	return 0
}

skip_step() {
	local label="$1"
	local reason="$2"
	results+=("SKIP  ${label} — ${reason}")
	skip=$((skip + 1))
	note "SKIP: ${label} — ${reason}"
}

# --- Offline gates (always) ---

run_step "gofmt-check" bash -c 'test -z "$(gofmt -l .)"'
run_step "go-vet" "${GO}" vet ./...
run_step "go-test-unit" bash -c 'unset TEST_DATABASE_URL; '"${GO}"' test ./...'

if command -v rg >/dev/null 2>&1; then
	run_step "check-production-placeholders" bash scripts/ci/check_production_placeholders.sh
else
	skip_step "check-production-placeholders" "ripgrep (rg) not on PATH — install rg or run in CI"
fi

run_step "check-feature-wiring" bash scripts/ci/check_feature_wiring.sh
run_step "check-migrations-offline" bash scripts/ci/verify_migrations.sh
run_step "check-uuid-v7" bash scripts/audit/verify-uuid-v7.sh

run_step "sqlc-check" bash -c "${GO} run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate && git diff --exit-code -- internal/gen/db/"
run_step "swagger-generate" "${PY}" tools/build_openapi.py
run_step "openapi-verify" "${PY}" tools/openapi_verify_release.py
run_step "swagger-drift" git diff --exit-code -- docs/swagger/
run_step "postman-generate" "${PY}" tools/build_postman_collection.py
run_step "postman-check" "${PY}" tools/check_postman_artifacts.py
run_step "postman-drift" git diff --exit-code -- postman/collections/ postman/environments/
run_step "proto-generate" bash -c "cd proto && ${GO} run github.com/bufbuild/buf/cmd/buf@v1.47.0 generate --exclude-path avf/internal && ${GO} run github.com/bufbuild/buf/cmd/buf@v1.47.0 generate --template buf.gen.avfinternal.yaml --path avf/internal/v1"
run_step "proto-lint" bash -c "cd proto && ${GO} run github.com/bufbuild/buf/cmd/buf@v1.47.0 lint"
run_step "proto-drift" git diff --exit-code -- proto/avf/machine/v1/ proto/avf/v1/ internal/gen/avfinternalv1/
run_step "machine-grpc-docs-check" "${PY}" scripts/ci/check_machine_grpc_docs.py
run_step "docker-compose-config" docker compose -f deployments/docker/docker-compose.yml config --quiet
run_step "migrate-cmd-validate" bash -c "MIGRATIONS_DIR=migrations ${GO} run ./cmd/migrate validate"

# --- Optional: fresh DB migration ---

if [[ "${VERIFY_WITH_DB:-0}" == "1" ]]; then
	if [[ -z "${TEST_DATABASE_URL:-}" ]]; then
		skip_step "fresh-db-migration" "VERIFY_WITH_DB=1 but TEST_DATABASE_URL unset"
	else
		run_step "fresh-db-migration" bash -c '
			set -e
			MIGRATIONS_DIR=migrations DATABASE_URL="'"${TEST_DATABASE_URL}"'" '"${GO}"' run ./cmd/migrate up
			MIGRATIONS_DIR=migrations DATABASE_URL="'"${TEST_DATABASE_URL}"'" '"${GO}"' run ./cmd/migrate version
		'
		run_step "postgres-integration-uuid-v7" bash -c 'TEST_DATABASE_URL="'"${TEST_DATABASE_URL}"'" '"${GO}"' test -count=1 ./internal/modules/postgres/... -run TestUUIDV7DefaultOnInsert'
	fi
else
	skip_step "fresh-db-migration" "set VERIFY_WITH_DB=1 and TEST_DATABASE_URL to run"
	skip_step "postgres-integration-uuid-v7" "set VERIFY_WITH_DB=1 and TEST_DATABASE_URL to run"
fi

# --- Optional: MQTT broker profile ---

if [[ "${VERIFY_WITH_BROKER:-0}" == "1" ]]; then
	run_step "mqtt-integration-tests" bash -c 'MQTT_HOST="${MQTT_HOST:-127.0.0.1}" MQTT_PORT="${MQTT_PORT:-1883}" '"${GO}"' test -count=1 ./internal/platform/mqtt/...'
	run_step "mqtt-full-coverage-script" env PYTHON="${PY}" bash scripts/test/run-mqtt-full-coverage.sh
else
	skip_step "mqtt-integration-tests" "set VERIFY_WITH_BROKER=1 (broker profile: docker compose --profile broker up -d)"
fi

# --- Optional: gRPC live smoke ---

if [[ "${VERIFY_WITH_GRPC:-0}" == "1" ]]; then
	run_step "grpc-full-coverage-script" bash scripts/test/run-grpc-full-coverage.sh
else
	skip_step "grpc-full-coverage-script" "set VERIFY_WITH_GRPC=1 and run API with gRPC on GRPC_ADDR"
fi

# --- Optional: Newman Postman smoke ---

if [[ "${VERIFY_WITH_NEWMAN:-0}" == "1" ]]; then
	if command -v newman >/dev/null 2>&1; then
		run_step "newman-smoke" newman run postman/collections/avf-vending-api.postman_collection.json \
			-e postman/environments/avf-local.postman_environment.json \
			--folder "Health" --bail
	else
		skip_step "newman-smoke" "newman not installed (npm i -g newman)"
	fi
else
	skip_step "newman-smoke" "set VERIFY_WITH_NEWMAN=1 for Postman CLI smoke"
fi

# --- Optional: GitHub workflow contract validation ---

if [[ "${VERIFY_WITH_WORKFLOWS:-0}" == "1" ]]; then
	if command -v actionlint >/dev/null 2>&1; then
		run_step "actionlint" actionlint
		run_step "verify-workflow-contracts" bash scripts/ci/verify_workflow_contracts.sh
	else
		skip_step "verify-workflow-contracts" "actionlint not on PATH"
	fi
else
	skip_step "verify-workflow-contracts" "set VERIFY_WITH_WORKFLOWS=1 (requires actionlint)"
fi

# --- Destructive / production E2E (explicit opt-in only) ---

if [[ "${VERIFY_DESTRUCTIVE:-0}" == "1" ]]; then
	note "WARNING: VERIFY_DESTRUCTIVE=1 — running production/destructive paths"
	run_step "test-e2e-local" bash -c '"'"${GO}"' test -count=1 -timeout=45m ./internal/e2e/correctness/... ./internal/grpcserver ./internal/platform/auth ./internal/app/background -run '"'"'TestP06_|TestMachineReplayLedger_|TestMachineOfflineSync_'"'"
else
	skip_step "test-e2e-local" "requires VERIFY_DESTRUCTIVE=1 + TEST_DATABASE_URL (45m integration suite)"
fi

# --- Summary ---

SUMMARY_FILE="${REPORT_DIR}/_verify_full_system_last_run.txt"
{
	echo "verify-full-system summary — $(date -u +%Y-%m-%dT%H:%M:%SZ)"
	echo "pass=${pass} fail=${fail} skip=${skip}"
	echo ""
	for row in "${results[@]}"; do
		echo "${row}"
	done
} | tee "${SUMMARY_FILE}"

echo ""
echo "=============================================="
if [[ "${fail}" -eq 0 ]]; then
	echo "verify-full-system: PASS (${pass} passed, ${skip} skipped)"
	echo "=============================================="
	exit 0
fi

echo "verify-full-system: FAIL (${fail} failed, ${pass} passed, ${skip} skipped)"
echo "See ${SUMMARY_FILE}"
echo "=============================================="
exit 1

#!/usr/bin/env bash
# CI guard: python3 shim safety (syntax, tests, static patterns).
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

fail() {
	echo "check_python3_shim_safety: $*" >&2
	exit 1
}

note() {
	echo "check_python3_shim_safety: $*"
}

note "bash -n on shim library and incident tooling"
bash -n scripts/ops/lib/python3_shim.sh
bash -n scripts/ops/tests/python3_shim.test.sh
bash -n scripts/ops/terminate-verified-db-audit-orphans.sh

PR1_AUDIT_SCRIPTS=(
	scripts/ops/run-table-data-audit.sh
	scripts/ops/run-clean-slate-audit.sh
	scripts/ops/emqx-audit.sh
	scripts/ops/nats-audit-streams.sh
	scripts/ops/lib/redis_resolve.sh
	scripts/ops/lib/redact.sh
)
for f in "${PR1_AUDIT_SCRIPTS[@]}"; do
	[[ -f "${f}" ]] || fail "missing PR1 audit script: ${f}"
	bash -n "${f}"
done

PR2_WIPE_SCRIPTS=(
	scripts/ops/run-environment-data-wipe.sh
	scripts/ops/emqx-purge.sh
	scripts/ops/nats-purge-streams.sh
	scripts/ops/lib/redis_wipe.sh
	scripts/ops/lib/ops_lock.sh
	scripts/ops/lib/ops_evidence.sh
)
for f in "${PR2_WIPE_SCRIPTS[@]}"; do
	[[ -f "${f}" ]] || fail "missing PR2 wipe script: ${f}"
	bash -n "${f}"
done

note "running python3_shim regression tests"
bash scripts/ops/tests/python3_shim.test.sh

note "static guard: no recursive exec python3 shim generators in tracked scripts/ops"
while IFS= read -r file; do
	case "${file}" in
	scripts/ops/tests/* | scripts/ops/lib/python3_shim.sh | scripts/ops/terminate-verified-db-audit-orphans.sh) continue ;;
	esac
	if grep -Eq 'exec (python3|\$\{?py_cmd\}?)' "${file}"; then
		fail "unsafe shim pattern in ${file}"
	fi
done < <(git ls-files 'scripts/ops/*.sh' 'scripts/ops/**/*.sh' | LC_ALL=C sort)

note "OK: python3 shim safety checks passed"

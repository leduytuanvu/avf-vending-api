#!/usr/bin/env bash
# Offline CI guard: production deploy script permissions, migration gate contracts, workflow ordering.
# No network, no secrets, no docker required.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

fail() {
	echo "validate-production-deploy.sh: error: $*" >&2
	exit 1
}

note() {
	echo "validate-production-deploy: $*"
}

run_python() {
	if [[ "${PY}" == "py -3" ]]; then
		py -3 "$@"
	else
		"${PY}" "$@"
	fi
}

PY=""
for candidate in python3 python; do
	if command -v "${candidate}" >/dev/null 2>&1 && "${candidate}" -c 'pass' >/dev/null 2>&1; then
		PY="${candidate}"
		break
	fi
done
if [[ -z "${PY}" ]] && command -v py >/dev/null 2>&1 && py -3 -c 'pass' >/dev/null 2>&1; then
	PY="py -3"
fi
[[ -n "${PY}" ]] || fail "python3, python, or py -3 required for nested .sh scan"

MIGRATE_SCRIPT="${ROOT}/scripts/deploy/production-migrate.sh"
DEPLOY_PROD_WF="${ROOT}/.github/workflows/deploy-prod.yml"

# --- 1. bash -n on production deploy shell scripts ---
note "bash -n production deploy shell scripts"
PROD_SCRIPT_PATHS=()
while IFS= read -r -d '' f; do
	PROD_SCRIPT_PATHS+=("${f}")
done < <(
	find \
		deployments/prod/shared/scripts \
		deployments/prod/app-node/scripts \
		deployments/prod/data-node/scripts \
		deployments/prod/scripts \
		-type f -name '*.sh' -print0 2>/dev/null | LC_ALL=C sort -z
)
for extra in \
	"${MIGRATE_SCRIPT}" \
	"${ROOT}/scripts/verify_database_environment.sh" \
	"${ROOT}/scripts/db/verify_database_environment.sh" \
	"${ROOT}/scripts/deploy/validate_migration_image.sh"; do
	[[ -f "${extra}" ]] && PROD_SCRIPT_PATHS+=("${extra}")
done
[[ "${#PROD_SCRIPT_PATHS[@]}" -gt 0 ]] || fail "no production deploy shell scripts found"
for f in "${PROD_SCRIPT_PATHS[@]}"; do
	bash -n "${f}" || fail "bash -n failed: ${f#"${ROOT}/"}"
done
note "OK: bash -n ${#PROD_SCRIPT_PATHS[@]} script(s)"

# --- 2. Required canonical paths (repo-root scripts + app-node/shared orchestration) ---
note "required production deploy assets"
REQUIRED_FILES=(
	scripts/deploy/production-migrate.sh
	scripts/verify_database_environment.sh
	scripts/db/verify_database_environment.sh
	deployments/prod/app-node/scripts/release_app_node.sh
	deployments/prod/shared/scripts/release_app_cluster.sh
	deployments/prod/shared/scripts/lib_release.sh
	deployments/prod/shared/scripts/validate_digest_pinned_image_refs.sh
	deployments/prod/app-node/docker-compose.app-node.yml
)
for rel in "${REQUIRED_FILES[@]}"; do
	[[ -f "${ROOT}/${rel}" ]] || fail "missing required file: ${rel}"
done
note "OK: ${#REQUIRED_FILES[@]} required file(s) present"

# Legacy/shared path aliases documented in Phase 0 — must not be the only copy.
LEGACY_ALIASES=(
	deployments/prod/shared/scripts/production-migrate.sh
	deployments/prod/shared/scripts/verify_database_environment.sh
	deployments/prod/shared/scripts/db/verify_database_environment.sh
	deployments/prod/shared/scripts/release_app_node.sh
)
for rel in "${LEGACY_ALIASES[@]}"; do
	if [[ -f "${ROOT}/${rel}" ]]; then
		fail "unexpected duplicate at ${rel}; canonical paths live under scripts/ and deployments/prod/app-node/scripts/"
	fi
done
note "OK: no stale duplicate paths under deployments/prod/shared/scripts/ for migration orchestration"

# --- 3. Git index executable bit on direct-entry scripts ---
note "git index executable bits (100755) for direct-entry deploy scripts"
GIT_EXECUTABLE=(
	scripts/verify_database_environment.sh
	scripts/db/verify_database_environment.sh
	scripts/deploy/production-migrate.sh
	deployments/prod/app-node/scripts/release_app_node.sh
	deployments/prod/app-node/scripts/rollback_app_node.sh
	deployments/prod/shared/scripts/release_app_cluster.sh
	deployments/prod/app-node/scripts/release_app_cluster.sh
	scripts/ci/validate-production-deploy.sh
)
for rel in "${GIT_EXECUTABLE[@]}"; do
	mode="$(git ls-files -s "${rel}" 2>/dev/null | awk '{print $1}' || true)"
	[[ -n "${mode}" ]] || fail "not tracked in git: ${rel}"
	[[ "${mode}" == "100755" ]] || fail "expected git mode 100755 for ${rel} (got ${mode}); run: git update-index --chmod=+x ${rel}"
done
note "OK: git executable modes"

# --- 4. Unsafe nested .sh invocation (must use bash or run_script) ---
note "scan for nested .sh calls that rely on executable bit"
run_python - \
	"${ROOT}/deployments/prod/shared/scripts" \
	"${ROOT}/deployments/prod/app-node/scripts" \
	"${ROOT}/deployments/prod/data-node/scripts" \
	"${ROOT}/scripts/deploy" \
	"${ROOT}/scripts/verify_database_environment.sh" \
	"${ROOT}/scripts/db/verify_database_environment.sh" \
	<<'PY'
import re
import sys
from pathlib import Path

paths = [Path(p) for p in sys.argv[1:]]
root = Path.cwd()

files: list[Path] = []
for p in paths:
    if p.is_dir():
        files.extend(sorted(p.rglob("*.sh")))
    elif p.is_file():
        files.append(p)

safe_line = re.compile(
    r"^\s*(#|$)|"
    r"\[\[ -f |\btest -f |\brequire_file\b|=\$\{|=\$\(|=\$\{ROOT\}|_SCRIPT=|_SH=|"
    r"\b(bash|run_script|source|shellcheck|require_file|require_bash_syntax|test|exec bash|exec python)\b|"
    r"bash\s+-n|<<|^\s*echo\b|^\s*note\b|^\s*fail\b|^\s*grep\b|^\s*cat\b|^\s*chmod\b|"
    r"^\s*# shellcheck",
    re.I,
)
exec_bad = re.compile(r"\bexec\s+(?!bash\s)([^\s|;&]+)")
# Command-position .sh (not file tests or variable assignment).
direct_sh = re.compile(
    r"^\s*(?:export\s+)?[A-Za-z_][A-Za-z0-9_]*=.*\.sh|"
    r"^\s*\[\[ -f .*\.sh"
)
direct_invoke = re.compile(
    r"^\s*(?:if\s+!\s+)?(?:\"?\$\{[^}]+\}|\"?\$\([^)]+\)|\./)[^\"'\s|;&]*\.sh\"?(?:\s|$|\")"
)

violations: list[str] = []
for path in files:
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as e:
        violations.append(f"{path}: read failed: {e}")
        continue
    for i, line in enumerate(text.splitlines(), 1):
        if safe_line.search(line):
            continue
        if "bash " in line and ".sh" in line:
            continue
        if "run_script " in line:
            continue
        if "source " in line and ".sh" in line:
            continue
        m = exec_bad.search(line)
        if m and ".sh" in m.group(1):
            rel = path.relative_to(root) if path.is_relative_to(root) else path
            violations.append(f"{rel}:{i}: exec without bash: {line.strip()}")
            continue
        if direct_sh.search(line):
            continue
        if direct_invoke.search(line) and "bash" not in line and "run_script" not in line:
            rel = path.relative_to(root) if path.is_relative_to(root) else path
            violations.append(f"{rel}:{i}: direct .sh invocation: {line.strip()}")

if violations:
    print("validate-production-deploy.sh: unsafe nested shell invocations:", file=sys.stderr)
    for v in violations:
        print(f"  {v}", file=sys.stderr)
    sys.exit(1)
print(f"OK: nested .sh scan clean ({len(files)} files)")
PY
note "OK: nested .sh scan"

# --- 5. Forbidden migration backup sources ---
note "forbidden canonical migration source paths"
FORBIDDEN_RE='/opt/avf-backups|migrations_old|pending-migrations'
SCAN_PATHS=(
	deployments/prod
	scripts/deploy/production-migrate.sh
	scripts/verify_database_environment.sh
	scripts/db/verify_database_environment.sh
	.github/workflows/deploy-prod.yml
)
for rel in "${SCAN_PATHS[@]}"; do
	target="${ROOT}/${rel}"
	if [[ -f "${target}" ]]; then
		if grep -En "${FORBIDDEN_RE}" "${target}" >/tmp/validate-prod-forbidden.txt 2>/dev/null; then
			fail "forbidden migration/backup path reference in ${rel}:$(head -1 /tmp/validate-prod-forbidden.txt)"
		fi
	elif [[ -d "${target}" ]]; then
		if grep -REn "${FORBIDDEN_RE}" "${target}" >/tmp/validate-prod-forbidden.txt 2>/dev/null; then
			fail "forbidden migration/backup path reference under ${rel}:$(head -1 /tmp/validate-prod-forbidden.txt)"
		fi
	fi
done
note "OK: no /opt/avf-backups or legacy migration folder references"

# --- 6. production-migrate.sh migration gate content ---
note "production-migrate.sh migration gate symbols"
[[ -f "${MIGRATE_SCRIPT}" ]] || fail "missing ${MIGRATE_SCRIPT}"
MIGRATE_CHECKS=(
	'pg_dump'
	'pg_restore -l'
	'run_compose_migrate_cmd status'
	'run_compose_migrate_cmd up'
	'require_digest_image_ref'
	'APP_IMAGE_REF'
	'GOOSE_IMAGE_REF'
	'mask_database_url'
	'postgres:17-alpine'
	'/app/migrations'
)
for pat in "${MIGRATE_CHECKS[@]}"; do
	grep -qF "${pat}" "${MIGRATE_SCRIPT}" || fail "production-migrate.sh missing required fragment: ${pat}"
done
grep -qE '@sha256:' "${MIGRATE_SCRIPT}" || fail "production-migrate.sh must reference digest-pinned @sha256: checks"
note "OK: production-migrate.sh gate symbols"

# --- 7. release_app_node migration ordering ---
RELEASE_NODE="${ROOT}/deployments/prod/app-node/scripts/release_app_node.sh"
grep -qF 'production-migrate.sh' "${RELEASE_NODE}" || fail "release_app_node.sh must invoke production-migrate.sh"
grep -qF 'leaving running containers unchanged' "${RELEASE_NODE}" || fail "release_app_node.sh must fail closed on migration error"
grep -qF 'restore_revision previous' "${RELEASE_NODE}" || fail "release_app_node.sh must restore env snapshot after migration failure"
grep -qF 'exit 41' "${RELEASE_NODE}" || fail "release_app_node.sh must exit 41 on migration failure"
migrate_ln="$(grep -n 'production-migrate.sh' "${RELEASE_NODE}" | head -1 | cut -d: -f1)"
drain_ln="$(grep -n 'PHASE="drain"' "${RELEASE_NODE}" | head -1 | cut -d: -f1)"
[[ -n "${migrate_ln}" && -n "${drain_ln}" && "${migrate_ln}" -lt "${drain_ln}" ]] || \
	fail "release_app_node.sh must run migration before traffic drain phase"
note "OK: release_app_node migration-before-drain ordering"

# --- 8. lib_release.sh run_script helper ---
grep -qF 'run_script()' "${ROOT}/deployments/prod/shared/scripts/lib_release.sh" || \
	fail "lib_release.sh must define run_script()"
grep -qF 'bash "${script}"' "${ROOT}/deployments/prod/shared/scripts/lib_release.sh" || \
	fail "lib_release.sh run_script must invoke bash"
note "OK: run_script helper present"

# --- 9. deploy-prod.yml: smoke before success-oriented summary/evidence ---
note "deploy-prod.yml smoke-before-summary ordering"
[[ -f "${DEPLOY_PROD_WF}" ]] || fail "missing ${DEPLOY_PROD_WF}"
smoke_ln="$(grep -n 'name: Run final public smoke' "${DEPLOY_PROD_WF}" | head -1 | cut -d: -f1 || true)"
summary_ln="$(grep -n 'name: Write production deployment summary' "${DEPLOY_PROD_WF}" | head -1 | cut -d: -f1 || true)"
manifest_ln="$(grep -n 'name: Upload production deployment manifest' "${DEPLOY_PROD_WF}" | head -1 | cut -d: -f1 || true)"
[[ -n "${smoke_ln}" && -n "${summary_ln}" ]] || fail "deploy-prod.yml must define final smoke and deployment summary steps"
[[ "${smoke_ln}" -lt "${summary_ln}" ]] || fail "final smoke step must precede Write production deployment summary"
if [[ -n "${manifest_ln}" ]]; then
	[[ "${smoke_ln}" -lt "${manifest_ln}" ]] || fail "final smoke step must precede Upload production deployment manifest"
fi
grep -qF 'id: smoke_cluster_final' "${DEPLOY_PROD_WF}" || fail "deploy-prod.yml must define smoke_cluster_final step id"
grep -qF 'FINAL_CLUSTER_SMOKE_STEP_OUTCOME' "${DEPLOY_PROD_WF}" || \
	fail "deploy-prod.yml summary must record FINAL_CLUSTER_SMOKE_STEP_OUTCOME"
# Smoke step must fail the job on error (no continue-on-error on the smoke step itself).
if awk '
  /^      - name: Run final public smoke/ { p=1; next }
  p && /^      - name:/ { exit }
  p && /continue-on-error:/ { found=1; exit }
  END { exit(found ? 0 : 1) }
' "${DEPLOY_PROD_WF}"; then
	fail "Run final public smoke step must not use continue-on-error"
fi
grep -qF 'chmod +x' "${DEPLOY_PROD_WF}" || fail "deploy-prod.yml must chmod +x synced migration scripts after tar extract"
grep -qF 'scripts/deploy/production-migrate.sh' "${DEPLOY_PROD_WF}" || \
	fail "deploy-prod.yml must sync scripts/deploy/production-migrate.sh to VPS"
note "OK: deploy-prod.yml smoke gate ordering"

note "PASS: validate-production-deploy.sh"

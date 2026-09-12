#!/usr/bin/env bash
# Regression tests for scripts/ops/lib/python3_shim.sh
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=../lib/python3_shim.sh
source "${ROOT}/scripts/ops/lib/python3_shim.sh"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

failures=0
passes=0

pass() {
	echo "PASS  $1"
	passes=$((passes + 1))
}

fail() {
	echo "FAIL  $1" >&2
	failures=$((failures + 1))
}

assert_exit() {
	local label="$1"
	local want="$2"
	shift 2
	local got=0
	set +e
	"$@" >/dev/null 2>&1
	got=$?
	set -e
	if [[ "${got}" -eq "${want}" ]]; then
		pass "${label}"
	else
		fail "${label}: exit ${got} want ${want}"
	fi
}

assert_contains() {
	local label="$1"
	local haystack="$2"
	local needle="$3"
	if [[ "${haystack}" == *"${needle}"* ]]; then
		pass "${label}"
	else
		fail "${label}: missing '${needle}'"
	fi
}

sandbox_root() {
	local dir="${TMP}/case-$$-$RANDOM"
	mkdir -p "${dir}/.db-destroy-evidence"
	printf '%s' "${dir}"
}

# 1. Normal interpreter
root="$(sandbox_root)"
if ops_prepare_python3_shim "${root}"; then
	if "${root}/.db-destroy-evidence/.bin/python3" -c 'import sys; print(0)' | grep -qx 0; then
		pass "normal interpreter executes"
	else
		fail "normal interpreter executes"
	fi
else
	fail "normal interpreter prepares shim"
fi
ops_cleanup_python3_shim

# 2. Historical py_cmd=python3 path (resolver must not hang)
root="$(sandbox_root)"
PATH="${PATH}"
if ops_prepare_python3_shim "${root}"; then
	if timeout 5s "${root}/.db-destroy-evidence/.bin/python3" -c 'import sys; sys.exit(0)'; then
		pass "historical python3 name does not recurse"
	else
		fail "historical python3 name does not recurse"
	fi
else
	fail "historical python3 shim prepare"
fi
ops_cleanup_python3_shim

# 3. PATH prefers shim after install
root="$(sandbox_root)"
ops_prepare_python3_shim "${root}"
resolved="$(command -v python3)"
if [[ "${resolved}" == "${root}/.db-destroy-evidence/.bin/python3" ]]; then
	if python3 -c 'import sys; print(bool(sys.executable))' | grep -qx True; then
		pass "PATH prefers shim and runs real interpreter"
	else
		fail "PATH prefers shim and runs real interpreter"
	fi
else
	fail "PATH prefers shim after install"
fi
ops_cleanup_python3_shim

# 4. Recursion target inside shim_dir fails fast
root="$(sandbox_root)"
shim_dir="${root}/.db-destroy-evidence/.bin"
mkdir -p "${shim_dir}"
fake="${shim_dir}/fake-python3"
printf '#!/usr/bin/env bash\nexit 0\n' >"${fake}"
chmod +x "${fake}"
OPS_PYTHON3_EXECUTABLE="${fake}"
if ops_validate_python3_shim_target "${shim_dir}" "${fake}"; then
	fail "recursion target inside shim_dir should fail"
else
	pass "recursion target inside shim_dir fails fast"
fi

# 5. Invalid interpreter
root="$(sandbox_root)"
shim_dir="${root}/.db-destroy-evidence/.bin"
invalid="${TMP}/not-executable-python3"
printf 'not a script\n' >"${invalid}"
if ops_validate_python3_shim_target "${shim_dir}" "${invalid}"; then
	fail "invalid interpreter should fail validation"
else
	pass "invalid interpreter fails validation"
fi

# 6. Args preserved
root="$(sandbox_root)"
ops_prepare_python3_shim "${root}"
last_arg="$("${root}/.db-destroy-evidence/.bin/python3" -c 'import sys; print(sys.argv[-1])' arg1)"
if [[ "${last_arg}" == "arg1" ]]; then
	pass "args preserved"
else
	fail "args preserved: got last argv ${last_arg}"
fi
ops_cleanup_python3_shim

# 7. Repeated execution
root="$(sandbox_root)"
ops_prepare_python3_shim "${root}"
ops_cleanup_python3_shim
trap - EXIT INT TERM
if ops_prepare_python3_shim "${root}"; then
	if timeout 5s python3 -c 'import sys; sys.exit(0)'; then
		pass "repeated execution does not recurse"
	else
		fail "repeated execution does not recurse"
	fi
else
	fail "repeated execution prepares shim"
fi
ops_cleanup_python3_shim

# 8. Cleanup on EXIT
root="$(sandbox_root)"
(
	ops_prepare_python3_shim "${root}"
	exit 0
)
if [[ ! -f "${root}/.db-destroy-evidence/.bin/python3" ]]; then
	pass "cleanup on EXIT removes shim"
else
	fail "cleanup on EXIT removes shim"
fi

# 9. Shim content uses absolute exec target
root="$(sandbox_root)"
ops_prepare_python3_shim "${root}"
shim_path="${root}/.db-destroy-evidence/.bin/python3"
if grep -q '^exec ' "${shim_path}"; then
	pass "shim uses exec with resolved target"
else
	fail "shim uses exec with resolved target"
fi
shim_body="$(cat "${shim_path}")"
if [[ "${shim_body}" == *'exec python3'* || "${shim_body}" == *'exec ${'* ]]; then
	fail "shim must not use bare python3 name in exec line"
else
	pass "shim avoids bare python3 exec name"
fi
ops_cleanup_python3_shim

echo ""
echo "Results: ${passes} passed, ${failures} failed"
if [[ "${failures}" -gt 0 ]]; then
	exit 1
fi
echo "All python3_shim tests passed."

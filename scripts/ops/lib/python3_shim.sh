#!/usr/bin/env bash
# Shared python3 shim for database audit/wipe tooling.
# Resolves the real interpreter before prepending shim_dir to PATH so the shim
# never execs a bare "python3" name (infinite recursion when py_cmd=python3).
set -Eeuo pipefail

OPS_PYTHON3_SHIM_DIR=""
OPS_PYTHON3_EXECUTABLE=""
OPS_PYTHON3_SHIM_INSTALLED=0

ops_resolve_python3_executable() {
	local shim_dir="$1"
	local launcher="" resolved=""

	if command -v python3 >/dev/null 2>&1 && python3 -c "import sys" >/dev/null 2>&1; then
		launcher="python3"
	elif command -v python >/dev/null 2>&1 && python -c "import sys" >/dev/null 2>&1; then
		launcher="python"
	elif command -v py >/dev/null 2>&1 && py -3 -c "import sys" >/dev/null 2>&1; then
		launcher="py -3"
	else
		echo "python3_shim: no working python3/python/py -3 interpreter found" >&2
		return 1
	fi

	resolved="$(${launcher} -c 'import sys; print(sys.executable)' 2>/dev/null)" || {
		echo "python3_shim: failed to resolve sys.executable via ${launcher}" >&2
		return 1
	}

	if command -v readlink >/dev/null 2>&1; then
		resolved="$(readlink -f "${resolved}" 2>/dev/null || echo "${resolved}")"
	fi

	if [[ -z "${resolved}" ]]; then
		echo "python3_shim: resolved interpreter is empty" >&2
		return 1
	fi
	if [[ "${resolved}" != /* && ! "${resolved}" =~ ^[A-Za-z]:[/\\] ]]; then
		echo "python3_shim: resolved interpreter is not an absolute path: ${resolved}" >&2
		return 1
	fi
	if [[ ! -x "${resolved}" ]]; then
		echo "python3_shim: resolved interpreter is not executable: ${resolved}" >&2
		return 1
	fi
	case "${resolved}" in
	"${shim_dir}"/*)
		echo "python3_shim: resolved interpreter is inside shim_dir: ${resolved}" >&2
		return 1
		;;
	esac

	OPS_PYTHON3_EXECUTABLE="${resolved}"
	return 0
}

ops_validate_python3_shim_target() {
	local shim_dir="$1"
	local target="$2"
	local shim_path="${shim_dir}/python3"

	if [[ -z "${target}" || ! -x "${target}" ]]; then
		echo "python3_shim: shim target is missing or not executable: ${target:-<empty>}" >&2
		return 1
	fi
	if [[ "${target}" == "${shim_path}" ]]; then
		echo "python3_shim: shim target equals shim path (${shim_path})" >&2
		return 1
	fi
	case "${target}" in
	"${shim_dir}"/*)
		echo "python3_shim: shim target is inside shim_dir: ${target}" >&2
		return 1
		;;
	esac
	return 0
}

ops_install_python3_shim() {
	local shim_dir="$1"
	local target="$2"
	local shim_path="${shim_dir}/python3"
	local tmp_path="${shim_path}.tmp.$$"

	mkdir -p "${shim_dir}"
	{
		printf '#!/usr/bin/env bash\n'
		printf 'exec %q "$@"\n' "${target}"
	} >"${tmp_path}"
	chmod +x "${tmp_path}"
	mv -f "${tmp_path}" "${shim_path}"
}

ops_cleanup_python3_shim() {
	local shim_path=""

	if [[ "${OPS_PYTHON3_SHIM_INSTALLED}" != "1" || -z "${OPS_PYTHON3_SHIM_DIR}" ]]; then
		return 0
	fi

	shim_path="${OPS_PYTHON3_SHIM_DIR}/python3"
	if [[ -f "${shim_path}" ]]; then
		rm -f "${shim_path}"
	fi
	if [[ -d "${OPS_PYTHON3_SHIM_DIR}" ]] && [[ -z "$(ls -A "${OPS_PYTHON3_SHIM_DIR}" 2>/dev/null || true)" ]]; then
		rmdir "${OPS_PYTHON3_SHIM_DIR}" 2>/dev/null || true
	fi
	OPS_PYTHON3_SHIM_INSTALLED=0
}

ops_prepare_python3_shim() {
	local root="$1"
	local shim_dir="${root}/.db-destroy-evidence/.bin"
	local saved_path="${PATH}"

	OPS_PYTHON3_SHIM_DIR="${shim_dir}"
	OPS_PYTHON3_SHIM_INSTALLED=0

	if ! ops_resolve_python3_executable "${shim_dir}"; then
		return 1
	fi
	if ! ops_validate_python3_shim_target "${shim_dir}" "${OPS_PYTHON3_EXECUTABLE}"; then
		return 1
	fi

	ops_install_python3_shim "${shim_dir}" "${OPS_PYTHON3_EXECUTABLE}"
	OPS_PYTHON3_SHIM_INSTALLED=1
	export PATH="${shim_dir}:${saved_path}"
	trap ops_cleanup_python3_shim EXIT INT TERM
	return 0
}

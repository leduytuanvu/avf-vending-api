#!/usr/bin/env bash
# Fail if production code reintroduces UUID v4 generation for internal resource IDs.
# See docs/architecture/UUID_V7_POLICY.md and scripts/checks/uuid-v7-allowlist.txt
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
ALLOWLIST="${SCRIPT_DIR}/uuid-v7-allowlist.txt"

cd "${ROOT}"

failures=0

note() {
	echo "check-uuid-v7: $*"
}

fail_hit() {
	echo "check-uuid-v7: FAIL: $*" >&2
	failures=$((failures + 1))
}

# --- allowlist helpers ---

allowlist_go_paths=()
allowlist_go_prefixes=()
allowlist_sql_paths=()

load_allowlist() {
	local line kind path
	while IFS= read -r line || [[ -n "${line}" ]]; do
		line="${line%%#*}"
		line="$(echo "${line}" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
		[[ -z "${line}" ]] && continue
		kind="${line%%:*}"
		path="${line#*:}"
		case "${kind}" in
		go)
			if [[ "${path}" == */ ]]; then
				allowlist_go_prefixes+=("${path}")
			else
				allowlist_go_paths+=("${path}")
			fi
			;;
		sql)
			allowlist_sql_paths+=("${path}")
			;;
		*)
			fail_hit "invalid allowlist entry (expected go: or sql:): ${line}"
			;;
		esac
	done <"${ALLOWLIST}"
}

go_path_allowed() {
	local rel="$1"
	local entry
	for entry in "${allowlist_go_paths[@]}"; do
		[[ "${rel}" == "${entry}" ]] && return 0
	done
	for entry in "${allowlist_go_prefixes[@]}"; do
		[[ "${rel}" == "${entry}"* ]] && return 0
	done
	return 1
}

sql_path_allowed() {
	local rel="$1"
	local entry
	for entry in "${allowlist_sql_paths[@]}"; do
		[[ "${rel}" == "${entry}" ]] && return 0
	done
	return 1
}

line_has_inline_allow() {
	local file="$1"
	local lineno="$2"
	local cur prev
	cur="$(sed -n "${lineno}p" "${file}")"
	if [[ "${cur}" == *"uuid-v7-allow"* ]]; then
		return 0
	fi
	if [[ "${lineno}" -gt 1 ]]; then
		prev="$(sed -n "$((lineno - 1))p" "${file}")"
		if [[ "${prev}" == *"uuid-v7-allow"* ]]; then
			return 0
		fi
	fi
	return 1
}

# --- Go scan ---

scan_go() {
	note "scanning Go (non-test production paths)"
	local pattern='uuid\.New\(|uuid\.NewString\(|uuid\.NewRandom\(|uuid\.NewV4\('
	local hits file rel lineno line

	if ! command -v git >/dev/null 2>&1; then
		fail_hit "git is required"
		return
	fi

	while IFS= read -r hit; do
		[[ -z "${hit}" ]] && continue
		file="${hit%%:*}"
		lineno="${hit#*:}"
		lineno="${lineno%%:*}"
		rel="${file//\\//}"
		if [[ "${rel}" == *_test.go ]]; then
			continue
		fi
		if go_path_allowed "${rel}"; then
			continue
		fi
		if line_has_inline_allow "${file}" "${lineno}"; then
			continue
		fi
		line="$(sed -n "${lineno}p" "${file}")"
		fail_hit "forbidden UUID v4 generation at ${rel}:${lineno}: ${line}"
	done < <(git grep -nE "${pattern}" -- '*.go' 2>/dev/null | grep -v '_test.go' || true)
}

# --- SQL scan (goose Up sections only) ---

extract_goose_up() {
	local file="$1"
	awk '
		/^-- \+goose Up/ { in_up=1; next }
		/^-- \+goose Down/ { in_up=0 }
		in_up { print }
	' "${file}"
}

scan_sql_file() {
	local rel="$1"
	local file="${ROOT}/${rel}"
	local up_body

	if sql_path_allowed "${rel}"; then
		return
	fi

	up_body="$(extract_goose_up "${file}")"
	if echo "${up_body}" | grep -qE 'DEFAULT[[:space:]]+gen_random_uuid[[:space:]]*\('; then
		fail_hit "forbidden DEFAULT gen_random_uuid() in goose Up: ${rel}"
	fi
	if echo "${up_body}" | grep -qE 'DEFAULT[[:space:]]+uuid_generate_v4[[:space:]]*\('; then
		fail_hit "forbidden DEFAULT uuid_generate_v4() in goose Up: ${rel}"
	fi
}

scan_sql() {
	note "scanning migrations/*.sql goose Up sections"
	local rel
	for file in migrations/*.sql; do
		[[ -f "${file}" ]] || continue
		rel="${file#./}"
		rel="${rel//\\//}"
		scan_sql_file "${rel}"
	done

	note "scanning db/schema/*.sql (sqlc mirror)"
	for file in db/schema/*.sql; do
		[[ -f "${file}" ]] || continue
		rel="${file#./}"
		rel="${rel//\\//}"
		if grep -qE 'DEFAULT[[:space:]]+gen_random_uuid[[:space:]]*\(' "${file}"; then
			fail_hit "forbidden DEFAULT gen_random_uuid() in ${rel} (use public.uuid_generate_v7())"
		fi
		if grep -qE 'DEFAULT[[:space:]]+uuid_generate_v4[[:space:]]*\(' "${file}"; then
			fail_hit "forbidden DEFAULT uuid_generate_v4() in ${rel} (use public.uuid_generate_v7())"
		fi
	done
}

# --- main ---

if [[ ! -f "${ALLOWLIST}" ]]; then
	fail_hit "missing allowlist: ${ALLOWLIST}"
fi

load_allowlist
scan_go
scan_sql

if [[ "${failures}" -gt 0 ]]; then
	echo "check-uuid-v7: ${failures} violation(s). See docs/architecture/UUID_V7_POLICY.md" >&2
	exit 1
fi

echo "check-uuid-v7: OK (no forbidden internal UUID v4 generation detected)"

#!/usr/bin/env bash
# Fail if SQL functions call pgcrypto without extensions. qualification (Supabase installs pgcrypto in extensions).
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

cd "${ROOT}"

failures=0

note() {
	echo "check-pgcrypto-schema-qualification: $*"
}

fail_hit() {
	echo "check-pgcrypto-schema-qualification: FAIL: $*" >&2
	failures=$((failures + 1))
}

scan_file() {
	local rel="$1"
	local file="${ROOT}/${rel}"
	local line lineno=0 content

	if [[ ! -f "${file}" ]]; then
		fail_hit "missing file: ${rel}"
		return
	fi

	while IFS= read -r line || [[ -n "${line}" ]]; do
		lineno=$((lineno + 1))
		content="${line%%--*}"
		if [[ "${content}" =~ gen_random_bytes[[:space:]]*\( ]] && [[ "${content}" != *"extensions.gen_random_bytes"* ]]; then
			fail_hit "unqualified gen_random_bytes() at ${rel}:${lineno}"
		fi
		if [[ "${content}" =~ uuid_generate_v7 ]] && [[ "${content}" =~ search_path[[:space:]]*=[[:space:]]*public,[[:space:]]*pg_temp ]]; then
			fail_hit "uuid_generate_v7 search_path missing extensions at ${rel}:${lineno}"
		fi
	done <"${file}"
}

note "scanning UUID v7 / pgcrypto SQL sources"
for rel in \
	migrations/00005_uuid_v7_defaults.sql \
	migrations/00006_fix_pgcrypto_uuid_v7_schema_qualification.sql \
	db/schema/01_platform.sql
do
	scan_file "${rel}"
done

if [[ "${failures}" -gt 0 ]]; then
	echo "check-pgcrypto-schema-qualification: ${failures} violation(s)" >&2
	exit 1
fi

echo "check-pgcrypto-schema-qualification: OK"

#!/usr/bin/env bash
# Assert production app image embeds /app/migrations and /app/migrate.
set -Eeuo pipefail

IMAGE="${1:-}"
if [[ -z "${IMAGE}" ]]; then
	echo "usage: validate_migration_image.sh IMAGE_REF" >&2
	exit 2
fi

fail() {
	echo "validate_migration_image: error: $*" >&2
	exit 1
}

note() {
	echo "validate_migration_image: $*"
}

note "checking ${IMAGE}"

# Git Bash on Windows converts /app/migrate to a host path unless path conversion is disabled.
docker_run() {
	if [[ -n "${MSYSTEM:-}" || "${OSTYPE:-}" == msys* ]]; then
		MSYS2_ARG_CONV_EXCL='*' MSYS_NO_PATHCONV=1 docker "$@"
	else
		docker "$@"
	fi
}

if ! docker_run run --rm --entrypoint /app/migrate "${IMAGE}" validate; then
	fail "migrate validate failed inside image"
fi

count="$(docker_run run --rm --entrypoint /bin/sh "${IMAGE}" -lc 'find /app/migrations -maxdepth 1 -type f -name "*.sql" | wc -l' | tr -d '[:space:]')"
[[ "${count}" -ge 1 ]] || fail "expected at least one .sql file under /app/migrations (got ${count})"

note "OK: ${count} migration file(s), /app/migrate validate passed"

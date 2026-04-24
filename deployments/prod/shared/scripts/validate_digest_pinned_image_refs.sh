#!/usr/bin/env bash
# Validate that image refs are digest-pinned (contain @sha256: and do not use :latest).
# Usage: validate_digest_pinned_image_refs.sh <ref> [<ref> ...]
set -euo pipefail

fail() {
	echo "error: $*" >&2
	exit 1
}

[[ "${#}" -ge 1 ]] || fail "at least one image ref argument is required"

for ref in "$@"; do
	[[ -n "${ref}" ]] || fail "empty image ref"
	[[ "${ref}" == *"@sha256:"* ]] || fail "image ref must be digest-pinned (...@sha256:...): ${ref}"
	[[ "${ref}" != *":latest"* ]] || fail "image ref must not use the latest tag: ${ref}"
done

echo "validate_digest_pinned_image_refs: ok"

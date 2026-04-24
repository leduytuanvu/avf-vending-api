#!/usr/bin/env bash
# Bootstrap a local app-node env file from the in-repo example (never overwrites).
# Usage: render_rollout_env.sh <destination.env> [path/to/.env.app-node.example]
set -euo pipefail

DST="${1:?usage: render_rollout_env.sh <destination.env> [example.env]}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLE="${2:-${SCRIPT_DIR}/../../app-node/.env.app-node.example}"

if [[ ! -f "${EXAMPLE}" ]]; then
	echo "error: example env not found: ${EXAMPLE}" >&2
	exit 1
fi
if [[ -e "${DST}" ]]; then
	echo "error: ${DST} already exists; remove or pick another path (refuse to overwrite)" >&2
	exit 1
fi

cp "${EXAMPLE}" "${DST}"
echo "wrote ${DST} from ${EXAMPLE}"
echo "Next: edit secrets and placeholders, then: bash ${SCRIPT_DIR}/validate_production_deploy_inputs.sh ${DST}"

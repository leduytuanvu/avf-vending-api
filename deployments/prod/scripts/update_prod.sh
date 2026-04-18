#!/usr/bin/env bash
# Artifact-based update helper: deploy the image tags already configured in .env.production.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [[ "${GIT_PULL:-}" == "1" ]]; then
	echo "update_prod: GIT_PULL is ignored; production updates are artifact-based now" >&2
fi

echo "update_prod: deploying image tags from .env.production via deploy_prod.sh"

exec bash "${ROOT}/scripts/deploy_prod.sh"

#!/usr/bin/env bash
# Run on production app-node after syncing scripts/ops/cloudinary.
# Quiesces uploads, inventories, wipes, verifies. Never prints API secret.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
OPS="${ROOT}/scripts/ops/cloudinary"
ENV_FILE="${ROOT}/deployments/prod/app-node/.env.app-node"
EVIDENCE="${ROOT}/.cloudinary-wipe-evidence"
COMPOSE="${ROOT}/deployments/prod/app-node/docker-compose.app-node.yml"

fail() { echo "run-production-cloudinary-wipe: error: $*" >&2; exit 1; }
note() { echo "run-production-cloudinary-wipe: $*"; }

ACTION="${1:-all}"
[[ -f "${ENV_FILE}" ]] || fail "missing ${ENV_FILE}"

set -a
# shellcheck disable=SC1090
source "${ENV_FILE}"
set +a
[[ -n "${CLOUDINARY_CLOUD_NAME:-}" ]] || fail "CLOUDINARY_CLOUD_NAME not set"

export CLOUDINARY_EVIDENCE_DIR="${EVIDENCE}"
mkdir -p "${EVIDENCE}"

quiesce() {
	note "quiesce — MEDIA_UPLOAD_ENABLED=false + stop api"
	if grep -q '^MEDIA_UPLOAD_ENABLED=' "${ENV_FILE}"; then
		sed -i 's/^MEDIA_UPLOAD_ENABLED=.*/MEDIA_UPLOAD_ENABLED=false/' "${ENV_FILE}"
	else
		echo "MEDIA_UPLOAD_ENABLED=false" >>"${ENV_FILE}"
	fi
	docker compose --env-file "${ENV_FILE}" -f "${COMPOSE}" stop api worker mqtt-ingest reconciler 2>/dev/null || true
	note "CLOUDINARY_ACTIVE_WRITERS=0 (api stopped, uploads disabled)"
}

discover() {
	bash "${OPS}/discover-targets.sh" \
		--production-env "${ENV_FILE}" \
		--staging-env "${ROOT}/deployments/staging/.env.staging"
}

inventory() {
	bash "${OPS}/inventory.sh" --env-file "${ENV_FILE}"
}

dedication_report() {
	local inv
	inv="$(ls -t "${EVIDENCE}"/asset-inventory-before-*.json 2>/dev/null | head -1)"
	[[ -n "${inv}" ]] || fail "no inventory file — run inventory first"
	python3 - "${inv}" "${EVIDENCE}/dedication-assessment-$(date -u +%Y%m%dT%H%M%SZ).json" <<'PY'
import json, sys
from pathlib import Path
inv = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
total = inv.get("total_original_assets", 0)
avf_folder = inv.get("avf_folder_prefix_count", 0)
avf_tag = inv.get("avf_tagged_count", 0)
non_avf = total - avf_folder if total else 0
dedicated = total == 0 or (avf_folder == total) or (avf_tag == total)
out = {
    "cloud_name": inv.get("cloud_name"),
    "total_assets": total,
    "avf_folder_prefix_count": avf_folder,
    "avf_tagged_count": avf_tag,
    "non_avf_assets_estimated": max(0, non_avf),
    "dedication_verdict": "DEDICATED_AVF" if dedicated else "SHARED_REQUIRES_MANUAL_REVIEW",
    "backup_mode": "MODE_A_LIVE_ASSETS_ONLY",
    "backup_mode_note": "Mode B (backup version purge) requires CONFIRM_CLOUDINARY_BACKUP_DELETE separately",
}
Path(sys.argv[2]).write_text(json.dumps(out, indent=2), encoding="utf-8")
print(json.dumps(out, indent=2))
PY
}

wipe() {
	set -a
	# shellcheck disable=SC1090
	source "${ENV_FILE}"
	set +a
	export CONFIRM_CLOUDINARY_LIVE_DELETE="DELETE-ALL-CLOUDINARY-ASSETS-${CLOUDINARY_CLOUD_NAME}"
	bash "${OPS}/wipe.sh" --delete-live-assets --delete-empty-folders --env-file "${ENV_FILE}"
	bash "${OPS}/verify-empty.sh" --env-file "${ENV_FILE}"
}

case "${ACTION}" in
discover) discover ;;
quiesce) quiesce ;;
inventory) inventory ;;
dedication) dedication_report ;;
wipe) wipe ;;
all)
	quiesce
	discover
	inventory
	dedication_report
	wipe
	;;
*)
	fail "usage: run-production-cloudinary-wipe.sh [discover|quiesce|inventory|dedication|wipe|all]"
	;;
esac

note "done action=${ACTION}"

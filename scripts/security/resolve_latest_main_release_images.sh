#!/usr/bin/env bash
# Resolve digest-pinned app/goose image refs from the latest successful "Build and Push Images"
# run on branch main that still has a promotion-manifest artifact.
# Writes nightly-reports/nightly-image-candidate.json (path: first arg or nightly-reports).
# Optional GITHUB_OUTPUT lines: resolution_status, app_image_ref, goose_image_ref, build_run_id.
# Exit: 0 = ok, 2 = no candidate (not an infra error), 1 = unexpected failure.
set -Eeuo pipefail

REPO="${GITHUB_REPOSITORY:?}"
OUT_DIR="${1:-nightly-reports}"
mkdir -p "${OUT_DIR}"
export RESOLVE_JSON_OUT="${OUT_DIR}/nightly-image-candidate.json"

append_github_out() {
  if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    printf '%s\n' "$1" >> "${GITHUB_OUTPUT}"
  fi
}

emit_json() {
  RESOLVE_JSON_OUT="${RESOLVE_JSON_OUT}" python3 - <<'PY'
import json, os
from pathlib import Path

path = os.environ["RESOLVE_JSON_OUT"]
payload = json.loads(os.environ["RESOLVE_PAYLOAD"])
Path(path).parent.mkdir(parents=True, exist_ok=True)
Path(path).write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
PY
}

no_candidate() {
  export RESOLVE_PAYLOAD="$(python3 -c 'import json,sys; print(json.dumps({"status":"no-candidate","reason":sys.argv[1]}))' "$1")"
  emit_json
  append_github_out "resolution_status=no-candidate"
  exit 2
}

if ! command -v gh >/dev/null 2>&1; then
  no_candidate "gh CLI not available"
fi

mapfile -t RUN_IDS < <(
  gh api "repos/${REPO}/actions/workflows/build-push.yml/runs" -f branch=main -f per_page=30 \
    --jq '.workflow_runs[] | select(.conclusion=="success") | .id' 2>/dev/null || true
)

if [[ ${#RUN_IDS[@]} -eq 0 ]]; then
  no_candidate "no successful Build and Push Images runs found for branch main"
fi

for rid in "${RUN_IDS[@]}"; do
  [[ -n "${rid}" ]] || continue
  has_promo="$(gh api "repos/${REPO}/actions/runs/${rid}/artifacts" --jq '[.artifacts[]?.name] | index("promotion-manifest") != null' 2>/dev/null || echo false)"
  if [[ "${has_promo}" != "true" ]]; then
    continue
  fi
  tmp="$(mktemp -d)"
  if ! gh run download "${rid}" -n promotion-manifest -D "${tmp}" 2>/dev/null; then
    rm -rf "${tmp}"
    continue
  fi
  if [[ ! -f "${tmp}/promotion-manifest.json" ]]; then
    rm -rf "${tmp}"
    continue
  fi
  readarray -t _coords < <(python3 -c '
import json, sys
p = json.load(open(sys.argv[1], encoding="utf-8"))
branch = (p.get("source_branch") or "").strip()
app = (p.get("app_ref") or "").strip()
goose = (p.get("goose_ref") or "").strip()
sha = (p.get("source_sha") or p.get("commit_sha") or "").strip()
print(branch)
print(app)
print(goose)
print(sha)
' "${tmp}/promotion-manifest.json")
  rm -rf "${tmp}"
  branch="${_coords[0]:-}"
  app="${_coords[1]:-}"
  goose="${_coords[2]:-}"
  sha="${_coords[3]:-}"

  if [[ "${branch}" != "main" ]]; then
    continue
  fi
  if [[ "${app}" != *@sha256:* || "${goose}" != *@sha256:* ]]; then
    continue
  fi

  export RESOLVE_PAYLOAD="$(
    python3 -c '
import json, sys
print(json.dumps({
  "status": "ok",
  "build_run_id": sys.argv[1],
  "source_branch": "main",
  "source_sha": sys.argv[2],
  "app_image_ref": sys.argv[3],
  "goose_image_ref": sys.argv[4],
}))
' "${rid}" "${sha}" "${app}" "${goose}"
  )"
  emit_json
  append_github_out "resolution_status=ok"
  append_github_out "app_image_ref=${app}"
  append_github_out "goose_image_ref=${goose}"
  append_github_out "build_run_id=${rid}"
  exit 0
done

no_candidate "no successful main build with digest-pinned app_ref/goose_ref in promotion-manifest (artifacts may have expired)"

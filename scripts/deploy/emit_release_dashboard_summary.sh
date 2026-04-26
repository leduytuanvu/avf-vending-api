#!/usr/bin/env bash
# Append a concise "Release dashboard" table to GITHUB_STEP_SUMMARY (CI) or stdout (local).
# Usage:
#   emit_release_dashboard_summary.sh security-release
#   emit_release_dashboard_summary.sh production-deploy
#
# production-deploy reads DASH_* environment variables (set by deploy-prod.yml) and optionally
# deployment-evidence/production-deployment-manifest.json for rollback hints.
set -Eeuo pipefail

MODE="${1:-}"
SUMMARY="${GITHUB_STEP_SUMMARY:-}"

_out() {
  if [[ -n "${SUMMARY}" ]]; then
    cat >>"${SUMMARY}"
  else
    cat
  fi
}

security_release_dashboard() {
  python3 - <<'PY'
from __future__ import annotations

import json
import os
from pathlib import Path

def load_json(p: Path) -> dict | None:
    if not p.is_file():
        return None
    try:
        return json.loads(p.read_text(encoding="utf-8"))
    except Exception:
        return None

verdict_path = Path("security-reports/security-verdict.json")
manifest_path = Path("release-reports/release-manifest.json")
v = load_json(verdict_path)
m = load_json(manifest_path)

sha = (v or {}).get("source_sha") or "—"
branch = (v or {}).get("source_branch") or "—"
sec_verdict = (v or {}).get("verdict") or "—"
pub = (v or {}).get("published_images") or {}
app_ref = (pub.get("app_image_ref") or "").strip() or "—"
goose_ref = (pub.get("goose_image_ref") or "").strip() or "—"

mig = "—"
if m:
    mig = str(m.get("migration_safety_verdict") or "—")

lines = [
    "",
    "## Release dashboard",
    "",
    "| Field | Value |",
    "|---|---|",
    "| **source_sha** | `%s` |" % sha,
    "| **branch** | `%s` |" % branch,
    "| **image refs (app / goose)** | `%s` / `%s` |" % (app_ref, goose_ref),
    "| **security verdict** | `%s` |" % sec_verdict,
    "| **migration verdict (CI snapshot)** | `%s` |" % mig,
    "| **smoke verdict** | — *(deploy stage)* |",
    "| **rollback candidate** | — *(pre-deploy)* |",
    "| **deploy status** | `security release complete` |",
    "",
]
print("\n".join(lines))
PY
}

production_deploy_dashboard() {
  python3 - <<'PY'
from __future__ import annotations

import json
import os
from pathlib import Path

def load_manifest() -> dict | None:
    p = Path(os.environ.get("DASH_MANIFEST_PATH", "deployment-evidence/production-deployment-manifest.json"))
    if not p.is_file():
        return None
    try:
        return json.loads(p.read_text(encoding="utf-8"))
    except Exception:
        return None

def g(name: str, default: str = "—") -> str:
    return (os.environ.get(name) or "").strip() or default

m = load_manifest()
rollback_hint = g("DASH_ROLLBACK_SUMMARY")
if rollback_hint == "—" and m:
    rb_avail = str(m.get("rollback_available_before_deploy", "—"))
    prev_ad = (m.get("previous_app_digest") or "—") or "—"
    prev_gd = (m.get("previous_goose_digest") or "—") or "—"
    rollback_hint = "before_deploy=%s; LKG digests app=%s goose=%s" % (rb_avail, prev_ad, prev_gd)

mig = g("DASH_MIGRATION_VERDICT")
if mig == "—" and m:
    mig = str(m.get("migration_safety_verdict") or "—")

lines = [
    "",
    "## Release dashboard",
    "",
    "| Field | Value |",
    "|---|---|",
    "| **source_sha** | `%s` |" % g("DASH_SOURCE_SHA"),
    "| **branch** | `%s` |" % g("DASH_SOURCE_BRANCH"),
    "| **image digests (app / goose)** | `%s` / `%s` |" % (g("DASH_APP_DIGEST"), g("DASH_GOOSE_DIGEST")),
    "| **image refs (app / goose)** | `%s` / `%s` |" % (g("DASH_APP_REF"), g("DASH_GOOSE_REF")),
    "| **security verdict** | `%s` |" % g("DASH_SECURITY_VERDICT"),
    "| **migration verdict** | `%s` |" % mig,
    "| **smoke verdict** | `%s` |" % g("DASH_SMOKE_VERDICT"),
    "| **rollback candidate** | `%s` |" % rollback_hint,
    "| **deploy status** | `%s` |" % g("DASH_DEPLOY_STATUS"),
    "",
]
print("\n".join(lines))
PY
}

case "${MODE}" in
  security-release)
    security_release_dashboard | _out
    ;;
  production-deploy)
    production_deploy_dashboard | _out
    ;;
  *)
    echo "usage: $0 security-release|production-deploy" >&2
    exit 2
    ;;
esac

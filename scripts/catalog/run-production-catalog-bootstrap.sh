#!/usr/bin/env bash
# Production catalog bootstrap orchestrator. Run on production app-node with env loaded.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MANIFEST="${MANIFEST:-$ROOT/../../docs/catalog-import-final.json}"
EVIDENCE="${EVIDENCE:-$ROOT/.catalog-bootstrap-evidence}"
CLI="${CLI:-$ROOT/bin/catalog-bootstrap}"

if [[ ! -f "$MANIFEST" ]]; then
	echo "run-production-catalog-bootstrap: manifest not found: $MANIFEST" >&2
	exit 2
fi

if [[ "${APP_ENV:-}" != "production" ]]; then
	echo "run-production-catalog-bootstrap: APP_ENV must be production" >&2
	exit 2
fi

mkdir -p "$EVIDENCE"

echo "== record source state =="
git -C "$ROOT" rev-parse HEAD >"$EVIDENCE/01-source-state.json.tmp"
printf '{"api_sha":"%s","manifest":"%s","started_at":"%s"}\n' \
	"$(git -C "$ROOT" rev-parse HEAD)" "$MANIFEST" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
	>"$EVIDENCE/01-source-state.json"

echo "== preflight =="
"$CLI" --manifest "$MANIFEST" --evidence-dir "$EVIDENCE" --environment production --preflight-only

echo "== validate manifest =="
"$CLI" --manifest "$MANIFEST" --evidence-dir "$EVIDENCE" --validate-only

echo "== dry run =="
"$CLI" --manifest "$MANIFEST" --evidence-dir "$EVIDENCE" --environment production --dry-run

if [[ "${CONFIRM_CATALOG_BOOTSTRAP:-}" != "CONFIRM-CATALOG-BOOTSTRAP-PRODUCTION" ]]; then
	echo "Set CONFIRM_CATALOG_BOOTSTRAP=CONFIRM-CATALOG-BOOTSTRAP-PRODUCTION to execute live import" >&2
	exit 0
fi

echo "== live import =="
"$CLI" --manifest "$MANIFEST" --evidence-dir "$EVIDENCE" --environment production \
	--upload-images --import-taxonomy --import-products --import-prices \
	--confirm-production=CONFIRM-CATALOG-BOOTSTRAP-PRODUCTION --resume

echo "== verify =="
"$CLI" --manifest "$MANIFEST" --evidence-dir "$EVIDENCE" --environment production --verify-only

echo "catalog bootstrap complete; evidence in $EVIDENCE"

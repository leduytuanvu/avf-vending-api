#!/usr/bin/env bash
# Optional outbound webhook for deployment / release outcomes. Safe by default: no URL → no-op.
# Never logs NOTIFY_WEBHOOK_URL or any secret. Hook failures are non-fatal (exit 0).
#
# Usage (preferred):
#   notify_deployment_status.sh --status <success|failure|cancelled|...> \
#     --environment <name> --source-sha <40hex|''> --source-branch <name> \
#     [--workflow-run-url <url>] [--message <text>] [--event <kind>]
#
# Environment (set by GitHub Actions or local testing):
#   NOTIFY_WEBHOOK_URL  — if unset/empty: prints skip message and exits 0
#   NOTIFY_STATUS, NOTIFY_OUTCOME  — same as --status (legacy; OUTCOME wins if both set and status empty)
#   NOTIFY_ENVIRONMENT
#   NOTIFY_SOURCE_SHA, NOTIFY_SOURCE_BRANCH
#   NOTIFY_WORKFLOW_RUN_URL
#   NOTIFY_MESSAGE
#   NOTIFY_EVENT  — e.g. security-release, production-deploy, staging-deploy, production-rollback
#
# If --workflow-run-url is omitted, uses GITHUB_SERVER_URL/GITHUB_REPOSITORY/GITHUB_RUN_ID when present.
set -Eeuo pipefail

STATUS=""
ENV_NAME=""
SRC_SHA=""
SRC_BR=""
RUN_URL_EXPLICIT=""
MSG=""
EVENT_KIND=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --status)
      STATUS="${2:-}"
      shift 2
      ;;
    --environment)
      ENV_NAME="${2:-}"
      shift 2
      ;;
    --source-sha)
      SRC_SHA="${2:-}"
      shift 2
      ;;
    --source-branch)
      SRC_BR="${2:-}"
      shift 2
      ;;
    --workflow-run-url)
      RUN_URL_EXPLICIT="${2:-}"
      shift 2
      ;;
    --message)
      MSG="${2:-}"
      shift 2
      ;;
    --event)
      EVENT_KIND="${2:-}"
      shift 2
      ;;
    -h|--help)
      sed -n '1,20p' "$0" >&2
      exit 0
      ;;
    *)
      echo "error: unknown argument: $1 (use --help)" >&2
      exit 2
      ;;
  esac
done

# Legacy / env fallbacks
if [[ -z "${STATUS}" ]]; then
  STATUS="${NOTIFY_STATUS:-}"
fi
if [[ -z "${STATUS}" ]]; then
  STATUS="${NOTIFY_OUTCOME:-}"
fi
if [[ -z "${STATUS}" ]]; then
  STATUS="unknown"
fi
if [[ -z "${ENV_NAME}" ]]; then
  ENV_NAME="${NOTIFY_ENVIRONMENT:-}"
fi
if [[ -z "${ENV_NAME}" ]]; then
  ENV_NAME="unspecified"
fi
if [[ -z "${SRC_SHA}" ]]; then
  SRC_SHA="${NOTIFY_SOURCE_SHA:-}"
fi
if [[ -z "${SRC_BR}" ]]; then
  SRC_BR="${NOTIFY_SOURCE_BRANCH:-}"
fi
if [[ -z "${MSG}" ]]; then
  MSG="${NOTIFY_MESSAGE:-}"
fi
if [[ -z "${EVENT_KIND}" ]]; then
  EVENT_KIND="${NOTIFY_EVENT:-}"
fi

RUN_URL="${RUN_URL_EXPLICIT:-}"
if [[ -z "${RUN_URL}" && -n "${NOTIFY_WORKFLOW_RUN_URL:-}" ]]; then
  RUN_URL="${NOTIFY_WORKFLOW_RUN_URL}"
fi
if [[ -z "${RUN_URL}" && -n "${GITHUB_SERVER_URL:-}" && -n "${GITHUB_REPOSITORY:-}" && -n "${GITHUB_RUN_ID:-}" ]]; then
  RUN_URL="${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}"
fi

if [[ -z "${NOTIFY_WEBHOOK_URL:-}" ]]; then
  echo "notification skipped: NOTIFY_WEBHOOK_URL not configured"
  exit 0
fi

PAYLOAD="$(
  NOTIFY_PAYLOAD_STATUS="${STATUS}" \
    NOTIFY_PAYLOAD_ENV="${ENV_NAME}" \
    NOTIFY_PAYLOAD_SHA="${SRC_SHA}" \
    NOTIFY_PAYLOAD_BRANCH="${SRC_BR}" \
    NOTIFY_PAYLOAD_RUN_URL="${RUN_URL}" \
    NOTIFY_PAYLOAD_MESSAGE="${MSG}" \
    NOTIFY_PAYLOAD_EVENT="${EVENT_KIND}" \
    NOTIFY_PAYLOAD_REPO="${GITHUB_REPOSITORY:-local}" \
    NOTIFY_PAYLOAD_WORKFLOW="${GITHUB_WORKFLOW:-}" \
    python3 - <<'PY'
import json, os
print(
    json.dumps(
        {
            "schema": "avf-deployment-notify/v1",
            "status": os.environ.get("NOTIFY_PAYLOAD_STATUS", "unknown"),
            "environment": os.environ.get("NOTIFY_PAYLOAD_ENV", ""),
            "source_sha": os.environ.get("NOTIFY_PAYLOAD_SHA", ""),
            "source_branch": os.environ.get("NOTIFY_PAYLOAD_BRANCH", ""),
            "workflow_run_url": os.environ.get("NOTIFY_PAYLOAD_RUN_URL") or None,
            "message": os.environ.get("NOTIFY_PAYLOAD_MESSAGE", ""),
            "event": (os.environ.get("NOTIFY_PAYLOAD_EVENT") or None),
            "repository": os.environ.get("NOTIFY_PAYLOAD_REPO", ""),
            "workflow": os.environ.get("NOTIFY_PAYLOAD_WORKFLOW") or None,
        },
        separators=(",", ":"),
    )
)
PY
)"

_body="$(mktemp)"
_err="$(mktemp)"
# Non-fatal: do not fail the parent workflow if curl or the endpoint fails
set +e
HTTP_CODE="$(
  curl -sS -o "${_body}" -w "%{http_code}" \
    -X POST \
    -H "Content-Type: application/json" \
    --data-binary "${PAYLOAD}" \
    "${NOTIFY_WEBHOOK_URL}" 2>"${_err}"
)"
CURL_RC=$?
set -e
rm -f "${_body}" "${_err}" 2>/dev/null || true

if [[ "${CURL_RC}" -ne 0 ]]; then
  echo "notify_deployment_status: request failed (curl rc=${CURL_RC}); original workflow outcome is unchanged" >&2
  exit 0
fi
if [[ "${HTTP_CODE}" =~ ^2 ]]; then
  echo "notify_deployment_status: notification sent (HTTP ${HTTP_CODE})"
else
  echo "notify_deployment_status: webhook returned HTTP ${HTTP_CODE} (non-fatal; deploy outcome unchanged)" >&2
fi
exit 0

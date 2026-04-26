#!/usr/bin/env bash
# Resolve repo-level Security (security.yml) push run evidence for Security Release signal.
# Writes a shell fragment to the path in $1 (source with: set -a && source "$file" && set +a).
# Required env: EVENT_NAME, GITHUB_REPOSITORY, release_source_sha, want_branch
# Optional: GH_TOKEN, GITHUB_STEP_SUMMARY
set -Eeuo pipefail

_out="${1:?usage: resolve_repo_security_evidence.sh <output-env-file>}"

repo_release_evidence_source="api-security-workflow"
repo_release_source_event="push"
repo_release_matched_run_id=""
repo_release_matched_run_conclusion=""
repo_release_verdict=""
repo_release_dependency_review="not_applicable"
repo_release_govulncheck_repo="not_applicable"
repo_release_secret_scan="not_applicable"
repo_release_trivy_config="not_applicable"
repo_release_summary="(pending) API lookup for **Security** repo workflow run"
release_resolution_error=""

if [[ "${EVENT_NAME}" == "workflow_run" ]]; then
  if [[ -z "${release_source_sha// }" || -z "${want_branch// }" ]]; then
    release_resolution_error="workflow_run: could not determine canonical source SHA/branch (artifact + fallbacks) for the repo **Security** workflow match. Check promotion-manifest / image-metadata and Build trigger fields."
  fi
  if [[ -n "${release_resolution_error}" ]]; then
    repo_release_evidence_source="workflow-resolution-failed"
    repo_release_verdict="unavailable"
    repo_release_summary="${release_resolution_error}"
  else
    repo_release_verdict="unavailable"
    max_attempts=20
    for ((attempt = 1; attempt <= max_attempts; attempt++)); do
      latest_json=""
      set +e
      latest_json="$(
        gh api "repos/${GITHUB_REPOSITORY}/actions/workflows/security.yml/runs" \
          -f "head_sha=${release_source_sha}" \
          -f "branch=${want_branch}" \
          -f "event=push" \
          -f "per_page=30" 2>/dev/null || true
      )"
      set -e
      most_recent_id=""
      most_recent_concl=""
      security_run_row=""
      if [[ -n "${latest_json// }" ]]; then
        set +e
        security_run_row="$(
          { echo "${latest_json}" | jq -r --arg b "${want_branch}" '
                    [.workflow_runs[]?
                      | select(
                          (.name // "") == "Security"
                          and ((.head_branch // "") == $b)
                        )
                    ]
                    | sort_by(.created_at)
                    | reverse
                    | (if length > 0 then .[0] | [((.id|tostring), (.conclusion // ""))] | @tsv else empty end)
                  ' 2>/dev/null | head -n 1; } || true
        )"
        set -e
      fi
      if [[ -n "${security_run_row// }" ]]; then
        IFS=$'\t' read -r most_recent_id most_recent_concl <<<"${security_run_row}" || true
      fi
      most_recent_id="${most_recent_id//[[:space:]]/}"
      most_recent_concl="${most_recent_concl//[[:space:]]/}"
      if [[ -n "${most_recent_id}" && "${most_recent_id}" =~ ^[0-9]+$ ]]; then
        if [[ "${most_recent_concl}" == "success" ]]; then
          repo_release_matched_run_id="${most_recent_id}"
          repo_release_matched_run_conclusion="success"
          repo_release_verdict="pass"
          repo_release_evidence_source="security-workflow-run-success"
          repo_release_summary="Security (repo) push run ${most_recent_id} completed successfully for head_sha=${release_source_sha} branch=${want_branch} (matches Build candidate)."
          break
        fi
        if [[ -n "${most_recent_concl}" && "${most_recent_concl}" != "success" && "${most_recent_concl}" != "null" ]]; then
          repo_release_evidence_source="security-workflow-run-not-successful"
          repo_release_matched_run_id="${most_recent_id}"
          repo_release_matched_run_conclusion="${most_recent_concl}"
          repo_release_verdict="fail"
          repo_release_summary="Security (repo) push run ${most_recent_id} for head_sha=${release_source_sha} branch=${want_branch} has conclusion=${most_recent_concl} (required: success)."
          break
        fi
      fi
      if [[ "${attempt}" -lt "${max_attempts}" ]]; then
        {
          echo "- **Poll ${attempt}/${max_attempts}**: waiting for a completed **Security** (repo) **push** run for \`head_sha=${release_source_sha}\` on branch \`${want_branch}\` …"
        } >>"${GITHUB_STEP_SUMMARY}" 2>/dev/null || true
        sleep 20 || true
      fi
    done
    if [[ "${repo_release_verdict}" != "pass" && "${repo_release_verdict}" != "fail" ]]; then
      repo_release_evidence_source="security-workflow-not-found-or-not-successful"
      repo_release_verdict="unavailable"
      repo_release_matched_run_id=""
      repo_release_matched_run_conclusion=""
      repo_release_summary="No successful **Security** (repo) **push** run found for head_sha=${release_source_sha} branch=${want_branch} after ${max_attempts} attempts (~$((max_attempts * 20))s). A push to develop/main that starts **Security** must exist for the same commit."
    fi
  fi
fi

{
  printf 'RELEASE_RESOLUTION_ERROR=%q\n' "${release_resolution_error}"
  printf 'REPO_RELEASE_EVIDENCE_SOURCE=%q\n' "${repo_release_evidence_source}"
  printf 'REPO_RELEASE_SOURCE_EVENT=%q\n' "${repo_release_source_event}"
  printf 'REPO_RELEASE_MATCHED_RUN_ID=%q\n' "${repo_release_matched_run_id}"
  printf 'REPO_RELEASE_MATCHED_RUN_CONCLUSION=%q\n' "${repo_release_matched_run_conclusion}"
  printf 'REPO_RELEASE_VERDICT=%q\n' "${repo_release_verdict}"
  printf 'REPO_RELEASE_DEPENDENCY_REVIEW=%q\n' "${repo_release_dependency_review}"
  printf 'REPO_RELEASE_GOVULNCHECK_REPO=%q\n' "${repo_release_govulncheck_repo}"
  printf 'REPO_RELEASE_SECRET_SCAN=%q\n' "${repo_release_secret_scan}"
  printf 'REPO_RELEASE_TRIVY_CONFIG=%q\n' "${repo_release_trivy_config}"
  printf 'REPO_RELEASE_SUMMARY=%q\n' "${repo_release_summary}"
} >"${_out}"

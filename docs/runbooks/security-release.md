# Security Release (`security-release.yml`)

Security Release runs after **Build and Push Images** and is the **only** workflow that publishes the `security-verdict` artifact used by deploy gates.

## Verdict artifact

Path in the artifact: `security-reports/security-verdict.json`.

The JSON always includes (machine-readable contract):

- `verdict`, `release_gate_verdict`, `release_gate_mode`
- `repo_security_verdict`, `repo_release_verdict`, `published_image_verdict`, `provenance_release_verdict`
- `provenance_release_checks` (includes `provenance_enforcement`, `allow_private_repo_provenance_fallback`, `signing_enforcement`, `published_image_provenance_verdict`, `evidence_source`)
- `source_sha`, `source_branch`, `source_build_run_id`, `source_workflow_name`
- `security_workflow_run_id`, `generated_at_utc`
- `failure_reasons`, `job_results`

## Top-level `verdict` values

| Verdict | Workflow result | Deploy |
|--------|-----------------|--------|
| `pass` | success | Allowed (subject to branch, digest, and other deploy checks) |
| `skipped` | success | **Not** allowed |
| `no-candidate` | success | **Not** allowed |
| `fail` | failure | **Not** allowed |
| Emergency / missing JSON | failure | **Not** allowed |

**Skipped** means the run is ineligible (for example wrong trigger shape or branch policy) and must not block ordinary CI; it is still **not** a deploy approval.

**No-candidate** means there was no valid release candidate (for example resolve steps did not succeed or the Build run was not a promotion candidate); the workflow succeeds so automation stays green, but **nothing may deploy** from that verdict.

**Fail** means a real candidate was evaluated and failed policy or required evidence; the workflow fails and the `security-verdict` artifact should still be uploaded when the writer ran.

## GitHub Actions outputs

After each verdict write, `scripts/security/emit_security_verdict_outputs.py` appends to `GITHUB_OUTPUT`, including `security_verdict` and `SECURITY_VERDICT` (same value as JSON `verdict`), plus `release_gate_verdict`, `source_sha`, `source_branch`, and related fields.

## Emergency writer

`scripts/security/write_security_verdict.py emergency` defaults to **not** overwriting an existing file that already has a contract `verdict` in `pass`, `fail`, `skipped`, or `no-candidate`. Use `--emergency-force` only when intentionally replacing a valid verdict (for example operator recovery).

The signal step exit trap follows the same rule: it emits an emergency fail only when the verdict file is missing, empty, or does not carry a valid contract verdict.

## Provenance and signing fields

- **`provenance_release_verdict`:** `pass` when **`published_images.provenance_verdict`** is **`verified`**; **`accepted-private-repo-no-github-attestations`** only when the private-repo fallback is allowed by repo variables; **`fail`** / **`unavailable`** when verification failed or evidence is missing. **`attestation-verify-failed`** is carried on **`published_images.provenance_verdict`** in warn mode when **`gh attestation verify`** did not succeed — the release gate still **fails** until fixed.
- **`provenance_release_checks`** snapshots **`PROVENANCE_ENFORCEMENT`**, **`ALLOW_PRIVATE_REPO_PROVENANCE_FALLBACK`**, and **`SIGNING_ENFORCEMENT`** so summaries and audits show which policy was active.

## Related

- Supply chain variables and manual verification: [supply-chain-security.md](./supply-chain-security.md)
- Release manifest and production chain: [release-process.md](./release-process.md)
- Repo **Security** (`security.yml`) does not publish `security-verdict` and does not authorize deploys. Blocking scans (govulncheck, secret scan, Trivy config), PR-only Dependency Review, CodeQL, and nightly informational artifacts are documented in [github-governance.md](./github-governance.md).
- **Dependency update snapshot** for merge/release gates is **not** required: the `go list -u` snapshot lives in **Nightly Security Rescan** only; Security Release may record `dependency_snapshot` as not applicable when resolving repo Security evidence.

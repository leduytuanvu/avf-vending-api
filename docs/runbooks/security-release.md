# Security Release (`security-release.yml`)

Security Release runs after **Build and Push Images** and is the **only** workflow that publishes the `security-verdict` artifact used by deploy gates.

## Verdict artifact

Path in the artifact: `security-reports/security-verdict.json`.

## Automatic Security Release after CI-chained Build

**Build and Push Images** is often started by **CI** (`workflow_run`). Security Release still runs the **full** gate when that Build completes **successfully** on `develop` or `main`, because releasable identity is taken from **promotion-manifest** / **immutable-image-contract** semantic **`source_event`** (`push` or `workflow_dispatch`), not from rejecting Build's outer GitHub `event` when it is `workflow_run`.

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

## Production deploy candidate artifact (`production-deploy-candidate`)

After **verdict=`pass`** on **`main`** only, Security Release may attach **`production-deploy-candidate`** (retention **30** days). **`develop`** passes never upload this artifact. This is **not** a deploy — it is a prefilled input bundle for manual **Deploy Production** (`deploy-prod.yml`). Skipped/fail verdicts **do not** produce it.

### Download

1. Open the successful **Security Release** run on GitHub Actions.
2. Under **Artifacts**, download **`production-deploy-candidate`** (zip).
3. Unzip and read **`README.md`** before editing inputs or running **`gh`**.

### Files

| File | Purpose |
|---|---|
| `README.md` | Operator checklist (staging evidence TODO, run id semantics, no auto-deploy). |
| `production-deploy-inputs.json` | **`workflow_dispatch`** inputs for **Deploy Production**, keyed like `deploy-prod.yml`. **`staging_evidence_id`** starts empty — set a real Staging Deployment Contract run id before dispatch unless you intentionally use workflow bypass inputs. |
| `production-deploy-inputs.env` | Same values as `KEY=value` for review. |
| `deploy-production-gh-command.sh` | Example wrapper calling `gh workflow run "Deploy Production" --ref main --json < production-deploy-inputs.json` after **`REPO_ROOT`** is set. |

### GitHub CLI (`production-deploy-inputs.json`)

From a checkout of this repo, after exporting **`REPO_ROOT`**:

`gh workflow run "Deploy Production" --ref main --json < production-deploy-inputs.json`

### Mandatory semantics (do not mix run ids)

| Topic | Rule |
|-------|------|
| Skipped verdicts | **`verdict: skipped`** (and similar non-pass outcomes) **must never** authorize production. Only **`verdict: pass`** on **`main`** with the full gate is eligible for the candidate pack; deploy workflows **fail closed** otherwise. |
| **`security_release_run_id`** | Must be the **Security Release** workflow run id carried in **`security-verdict.json`** (`security_workflow_run_id` / `workflow_run_id`). It is **not** the **Build and Push Images** run id. |
| **`build_run_id`** | Must equal **`security-verdict.source_build_run_id`** for that verdict (the **Build and Push Images** run that produced the digest-pinned images). |
| Digest pins | **`app_image_ref`** and **`goose_image_ref`** must stay **`@sha256:…`** — production gates re-verify them against the verdict and build artifacts. |

## Related

- Supply chain variables and manual verification: [supply-chain-security.md](./supply-chain-security.md)
- Release manifest and production chain: [release-process.md](./release-process.md)
- Repo **Security** (`security.yml`) does not publish `security-verdict` and does not authorize deploys. Blocking scans (govulncheck, secret scan, Trivy config), PR-only Dependency Review, CodeQL, and nightly informational artifacts are documented in [github-governance.md](./github-governance.md).
- **Dependency update snapshot** for merge/release gates is **not** required: the `go list -u` snapshot lives in **Nightly Security Rescan** only; Security Release may record `dependency_snapshot` as not applicable when resolving repo Security evidence.

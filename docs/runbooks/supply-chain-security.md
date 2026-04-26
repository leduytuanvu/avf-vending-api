# Supply chain security (images, SBOM, scans, signing)

**Pipeline context:** [cicd-release.md](./cicd-release.md) — §7.

This repository ties **digest-pinned** container images to **build metadata**, **SBOMs**, **Cosign signatures**, and **Security Release** gating. Nothing here replaces human review of dependency risk; it makes what was built **auditable** and **consistent** across CI and deploys.

## Release-candidate build outputs

Each successful **Build and Push Images** run for `develop` / `main` (after the upstream **CI** gate) publishes:

| Artifact | Purpose |
| --- | --- |
| `image-metadata` | Resolved app/goose image names, tags, **digests**, and immutable `ref` strings. |
| `promotion-manifest` | Promotion identity (commit, branch, semantic `source_event`, digests) plus **`sbom`** and **`cosign`** metadata. |
| `immutable-image-contract` | Same digest-pinned refs and build run id for downstream verification. |
| `release-candidate` | Single `release-candidate.json` with **`source_sha`**, **`source_branch`**, **`build_run_id`**, **`workflow_run_id`**, **`repo`**, digest-pinned **`app_image_ref`** / **`goose_image_ref`**, **`generated_at_utc`**, and pointers to the other build artifacts (enterprise audit + Security Release input). |
| `sbom-reports` | CycloneDX JSON: `sbom-app.cdx.json`, `sbom-goose.cdx.json` (generated against **digest-pinned** refs, not mutable tags). |
| `cosign-signing-evidence` | Summary JSON: digest-pinned **signed** refs, GitHub OIDC verify policy regexps, optional verify payload excerpts. |
| `build-metadata-manifest` / `image-build-metadata` | Human and compatibility bundles. |

### Cosign (keyless)

The reusable image build signs **`app_image@sha256:…`** and **`goose_image@sha256:…`** with **Cosign** using **GitHub Actions OIDC** (no long-lived private keys in the repo). Signatures are stored in the registry alongside the image. A post-sign **`cosign verify`** step (issuer `https://token.actions.githubusercontent.com`, identity `https://github.com/<this-repo>/…`) confirms signatures before manifests are published.

Docker BuildKit SBOM (`sbom: true` on `docker/build-push-action`) and GitHub **artifact attestations** (public repos) remain additional signals; **authoritative** CycloneDX files are **`sbom-reports`**.

## Security Release

**Security Release** still **requires Trivy image scans** (HIGH/CRITICAL policy). SBOM and Cosign do **not** replace Trivy.

Preflight checks require **`sbom-reports`**, **`cosign-signing-evidence`**, and **`release-candidate`** on the Build run, alongside `image-metadata`, `promotion-manifest`, and `immutable-image-contract`. **Security Release** downloads **`release-candidate.json`** and **`promotion-manifest.json`** from the Build run and passes them to **`write_security_verdict.py`**: the verdict records **`build_release_evidence`** (and **`sbom`** from the manifest when present) so scans and the gate are tied to the same **digest** evidence—mutable tags are rejected.

### `SBOM_POLICY` (repository variable)

| Value | Behavior |
| --- | --- |
| `warn` (default) | If the **sbom** block is missing from the downloaded **promotion-manifest**, the gate records a **warning** in **`security-verdict.json`** and does not fail the verdict solely for that. |
| `enforce` | Missing **sbom** metadata fails the **Security Release** gate (use when enterprise policy requires a CycloneDX pointer chain from Build). |

A dedicated job **`image-cosign-verify`** runs **`cosign verify`** on the same digest-pinned refs using the same issuer/identity policy as the build.

### `SIGNING_ENFORCEMENT` (repository variable)

Set on the GitHub repo (Actions variables):

| Value | Behavior |
| --- | --- |
| `warn` (default) | Cosign verify failures emit **warnings** and are recorded in **`security-verdict`** (`image_signing`), but do **not** fail the release gate or block deploy unless other gates fail. Use for first rollout while monitoring green Cosign runs. |
| `enforce` | Cosign verify failures **fail** the `image-cosign-verify` job and the **Security Release** gate (with **`SIGNING_ENFORCEMENT=enforce`** included in **`published_image_verdict`**). **Staging and production** deploy workflows refuse deploy when the verdict does not show **`image_signing.overall=pass`**. |

**Rollout:** keep **`warn`** until several successful Build + Security Release cycles show stable Cosign verification, then set **`enforce`** for production-ready policy.

The **security-verdict** summary includes **`image_signing`** (enforcement mode, per-image and overall status, OIDC issuer).

### `PROVENANCE_ENFORCEMENT` and `ALLOW_PRIVATE_REPO_PROVENANCE_FALLBACK`

These repository **Actions variables** control **GitHub Artifact Attestation** verification (`gh attestation verify` in **`_reusable-deploy`**) and how the **Security Release** records **`provenance_release_verdict`** and **`published_images.provenance_verdict`**.

| Variable | Values | Role |
| --- | --- | --- |
| `PROVENANCE_ENFORCEMENT` | `warn` (default), `enforce` | **`warn`:** attestation verify may use `continue-on-error` in reusable deploy so the job can finish, but **`write_security_verdict.py` still fails the release gate** if the outcome is `attestation-verify-failed` or provenance is otherwise unacceptable. **`enforce`:** attestation verify failure **fails** reusable deploy; **`enforce`** also **rejects** the private-repo provenance fallback in the verdict writer. |
| `ALLOW_PRIVATE_REPO_PROVENANCE_FALLBACK` | `true` (default), `false` | When **`true`**, a **private** repository may record **`accepted-private-repo-no-github-attestations`** only if **`PROVENANCE_ENFORCEMENT=warn`** (explicit opt-in). When **`false`**, that fallback path **fails** even in warn mode. |

**Security Release** and deploy summaries call out when **provenance is not fully enforced** (warn + private fallback). Nothing in CI **fabricates** an attestation; eligibility follows GitHub’s attestation model for the repo.

**Rollout (warn → enforce):** while on a private repo, **`PROVENANCE_ENFORCEMENT=enforce`** is intentionally unsatisfiable for automatic attestation verification — keep **`warn`** until the repository can run **`gh attestation verify`** (typically **public**), then set **`enforce`** and **`ALLOW_PRIVATE_REPO_PROVENANCE_FALLBACK=false`**.

## Deploy workflows

**deploy-develop.yml** and **deploy-prod.yml** read **`vars.SIGNING_ENFORCEMENT`**. When it is **`enforce`**, they require **`security-verdict.image_signing.overall == pass`** in addition to existing digest alignment and gate checks. **`warn`** does not add this deploy-time block.

They also read **`vars.PROVENANCE_ENFORCEMENT`**. When it is **`enforce`**, they require **`provenance_release_checks.required_for_release_verdict == pass`** and **`published_images.provenance_verdict == verified`** (so **private-repo fallback is not deployable** under enforce). **`warn`** keeps the previous behavior: pass or accepted private-repo fallback, subject to other gates.

## Consuming artifacts locally

1. From a green Build run, download **`sbom-reports`** and **`cosign-signing-evidence`**.
2. Verify images with the same policy as CI, for example:  
   `cosign verify --certificate-identity-regexp '^https://github.com/ORG/REPO/' --certificate-oidc-issuer-regexp '^https://token.actions.githubusercontent.com$' 'ghcr.io/…@sha256:…'`
3. Cross-check **`promotion-manifest.json`** digests against what you deploy.
4. Where GitHub attestations apply, verify bundle provenance for the **same digest** the workflow used, for example:  
   `gh attestation verify oci://ghcr.io/ORG/IMAGE@sha256:… --repo ORG/REPO`  
   (exact flags depend on GH CLI version; use the digest from **`image-metadata`** / **`immutable-image-contract`**, not a mutable tag.)

## GitHub Actions version pins

**Policy (this repository)**

- **Tag- or version-based `uses: org/action@vMAJOR`** (and similar) is **allowed** for day-to-day work. The repo does **not** require every workflow to be SHA-pinned to merge.
- **Full commit SHA pinning** (`org/action@7b3f…` or 40 hex characters) is **recommended** for **production-critical** and **high-impact** paths: **Build and Push Images** (`build-push.yml`), **Security Release** (`security-release.yml`), and **staging / production deploy** (`deploy-develop.yml`, `deploy-prod.yml`), plus any third-party action outside the official **`actions/`** and **`github/codeql-action/** namespaces.
- A future **stricter** mode can be turned on in CI with **`ENFORCE_ACTION_SHA_PINNING=true`** (repository or workflow `env` only when ready): the offline verifier in `tools/verify_github_workflow_cicd_contract.py` then **fails** if a **third-party** action is not pinned to a **git commit SHA**. Official **`actions/*`** and **`github/codeql-action/*`** stay **exempt** from enforce until you adopt org-wide SHA pins for them too.

**Verifier behavior (always runs with the workflow contract check)**

- Prints an **inventory** (stderr) of action references in the high-impact workflows above (official + third party), with a note whether the ref **looks like a commit SHA** or a **tag/branch**.
- **Warns** (stderr) on any **third-party** `uses` line that is **not** commit-SHA-pinned. This does **not** fail the job by default.
- Set **`ENFORCE_ACTION_SHA_PINNING=true`** to **fail** the contract on unpinned **third-party** actions only. **Do not** enable this in a branch until pins are merged and recorded.
- Optional: **`VERIFY_ACTION_SHA_PINNING=1`** adds **informational** lines for **official** tag-based `uses` (no failure).

**How to pin safely (no “random” SHAs)**

1. **Verify the upstream action release** you intend to use: open the action’s **GitHub Releases** and/or the tag on the default branch; read release notes and security advisories.
2. **Resolve the exact commit** that tag points to: on the action repo, open the tag and copy the **commit** URL, or use `git ls-remote https://github.com/ORG/ACTION-REPO v3.0.0`.
3. **Update the workflow** to `uses: org/action@<full_40_char_sha>` (optionally with a comment `# v3.0.0` for humans).
4. **Open a PR** and let **CI** run; confirm no behavior change beyond the intended version.
5. **Record the change** in **`CI_CD_FINAL_AUDIT.md`** (or your change log) with action name, old ref, new SHA, and date, so the next hardening pass is auditable.

Never paste an untested SHA from a search engine; always take it from the **canonical** `github.com` repository for that action.

## Contract enforcement

CI asserts that **build** uploads **`cosign-signing-evidence`**, that **`_reusable-build.yml`** performs **Cosign sign**, that **Security Release** requires the artifact and runs **`image-cosign-verify`**, and that **`SIGNING_ENFORCEMENT`**, **`PROVENANCE_ENFORCEMENT`**, and **`ALLOW_PRIVATE_REPO_PROVENANCE_FALLBACK`** are wired through **`security-release.yml`**, **`_reusable-deploy.yml`**, and deploy validation. Changes that drop signing or verification should fail **`tools/verify_github_workflow_cicd_contract.py`** and **`scripts/ci/verify_workflow_contracts.sh`**. The same verifier applies **action pin policy** (see [GitHub Actions version pins](#github-actions-version-pins) above; **`ENFORCE_ACTION_SHA_PINNING`** is off by default).

## Private repositories

GitHub **Artifact Attestations** / **`gh attestation verify`** are often **not available** the same way on **private** repositories. This repo therefore supports an **explicit** fallback: with **`ALLOW_PRIVATE_REPO_PROVENANCE_FALLBACK=true`** and **`PROVENANCE_ENFORCEMENT=warn`**, the verdict may record **`accepted-private-repo-no-github-attestations`** while still requiring **digest-pinned** images, **Trivy**, and **Cosign** (per **`SIGNING_ENFORCEMENT`**). **Cosign** remains the registry-local signature story and is independent of GitHub’s attestation product.

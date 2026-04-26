# Supply chain pinning

This repository pins third-party **GitHub Actions**, **production base images** (Dockerfiles and production compose), **Go-based CI tools** (`go install`), and the **Trivy** scanner image. CI runs `scripts/ci/verify_supply_chain_pinning.sh` (implementation: `tools/supply_chain_pinning.py`) from `scripts/ci/verify_workflow_contracts.sh` before the offline workflow graph check.

## Policy in brief

- **Actions**: External `uses:` values must be `owner/repo@<40-hex-commit-sha>` with an optional trailing `# human tag` comment. Reusable workflows under `./.github/...` and `docker://` references are not treated as “external” actions in this check.
- **Production Docker**: `FROM` in `deployments/prod/**/Dockerfile*` and public `image:` lines in `deployments/prod/**/docker-compose*.yml` must include `@sha256:<64-hex>` (GHCR app images referenced via `${...}` are skipped).
- **Go tools in workflows**: `go install` must not use `@latest`. Known tools in CI (actionlint, shfmt, govulncheck, gitleaks) must match the versions encoded in `tools/supply_chain_pinning.py` (kept in sync with the workflow YAML and documentation).
- **Trivy**: The scanner must use `aquasec/trivy:0.57.1` (exact) or a digest-pinned `aquasec/trivy@sha256:...` / `...:ver@sha256:...` form. When upgrading Trivy, bump the constant in `tools/supply_chain_pinning.py` and every workflow that sets `trivy_image=`.
- **Installers**: `curl|sh`, `wget|sh`, and similar one-liner patterns in `.github/workflows`, `scripts/ci`, and `tools/` are rejected unless you add a dated entry to the allowlist with owner and reason.

## Exceptions: `scripts/ci/supply-chain-allowlist.txt`

If you must temporarily allow a non-compliant line (for example a tag-based action during an emergency), add a **single row** with these pipe-separated fields:

`expiry_utc` | `owner` | `path_glob` | `check_kind` | `match_sub` | `reason`

- **expiry_utc**: `YYYY-MM-DD` (UTC calendar day; renew or remove before expiry).
- **owner**: team or person accountable for review.
- **path_glob**: `fnmatch` pattern, e.g. `.github/workflows/foo.yml` or `**/*.yml` (use sparingly).
- **check_kind**: one of `uses_ref`, `dockerfile_from`, `go_install`, `curl_bash`, `trivy_image`, or `generic` (matches any check).
- **match_sub**: a substring of the **reported** violation; use `.*` in `reason` only if the format allows, or a distinctive substring. Empty `match_sub` matches if path and kind match.
- **reason**: short justification (avoid raw `|` inside the reason or escape consistently).

Rows past **expiry** are ignored; remove stale rows in routine maintenance.

## Updating pinned GitHub Actions

1. Find the current release for the action (repository releases or tags).
2. Resolve the **commit SHA** of that tag: open the tag on GitHub, copy the full commit hash.
3. Update the workflow line to `uses: org/action@<40-char-sha> # vX.Y.Z` and run locally:
   - `bash scripts/ci/verify_supply_chain_pinning.sh`
   - `actionlint`
4. In PR description, link the action release notes and the commit you pinned.

**Dependabot** (`.github/dependabot.yml`, `github-actions` ecosystem) may open grouped PRs; treat them as a prompt to re-pin: verify the new tag’s commit SHA, update workflows, and run the same checks. Do not merge a PR that reverts a SHA pin to a tag only.

## Updating Docker image digests

1. For a public image `repo/name:tag`, get the **index** digest:  
   `docker buildx imagetools inspect repo/name:tag`  
   Use the `Digest:` value in the form `@sha256:<64-hex>` (production compose often uses `name:tag@sha256:... # tag` for readability).
2. Update the Dockerfile `FROM` or compose `image:` line.
3. Run `bash scripts/ci/verify_supply_chain_pinning.sh` and a targeted `docker build` or compose config validation if the change is non-trivial.

## Updating Go tools in CI

1. Change the `go install ...@<version>` line in the relevant workflow (for example `ci.yml`, `security.yml`).
2. Update the same version expectations in `tools/supply_chain_pinning.py` (`assert_pinned_go_tool_table`) and any Makefile or docs that document the install command.
3. Re-run the verifier and `actionlint`.

## Reviewing Dependabot PRs safely

- **Scope**: Check whether the PR updates only Go modules, Actions, or Docker; split concerns if a single PR is too large to review.
- **Actions**: Dependabot may propose a new tag; **replace** with a full **commit SHA** in workflow YAML, then run `verify_supply_chain_pinning.sh` and `actionlint`.
- **Docker** (`/deployments/prod`): Prefer digest-pinned images; re-resolve digests when the base tag changes.
- **Go** (`gomod`): Review changelog for security; run the normal test/CI path before merge.
- If a Dependabot change cannot be reconciled with pinning policy, **close** or **rebase** the PR and apply a hand-crafted pin using the steps above; use the allowlist only as a last resort with expiry and owner.

## Local commands (pre-push)

```bash
bash scripts/ci/verify_supply_chain_pinning.sh
actionlint
bash scripts/ci/verify_workflow_contracts.sh
python3 tools/verify_github_workflow_cicd_contract.py
```

See also: `.github/dependabot.yml` for update schedules; enterprise workflow contracts are enforced by `scripts/ci/verify_workflow_contracts.sh` and the Python graph checker in `tools/verify_github_workflow_cicd_contract.py`.

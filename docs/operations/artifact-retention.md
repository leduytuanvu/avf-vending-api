# GitHub Actions artifact retention (release evidence)

This repository stores **build, security release, and deployment evidence** as workflow artifacts. Retention is set per `actions/upload-artifact` step in `.github/workflows/`.

## Policy (summary)

| Tier | `retention-days` | Use |
|------|------------------|-----|
| Release & image chain | 180 | SBOM, promotion / immutable image contracts, cosign evidence, `image-metadata`, `release-candidate`, Security Release Trivy + `security-verdict` |
| **Canonical audit packages** | **90** | `release-audit-package` (build), `security-release-audit-package`, `production-release-audit-package` — see [release-evidence-retention.md](release-evidence-retention.md) |
| Production & scale gate | 90–365 | Full `production-deploy-evidence` tree now **90** in `deploy-prod.yml` (was 14); storm / long windows as documented per workflow |
| Staging & governance scans | 90 | Staging contract / verdict, nightly security, repo **Security** workflow scan outputs |
| CI & ops telemetry (non-release) | 30 | PR CI migration/legacy reports, **Manual Ops Evidence Check** (`nightly-ops.yml`) reports, storm suite log bundles |

## Public repositories

GitHub caps artifact retention for **public** repositories (commonly **90 days** max). If artifact uploads fail with a retention error, lower the relevant `retention-days` in the workflow to **90** (or the maximum your plan allows) while keeping the relative tiers: release evidence longer than short-lived CI-only noise.

## Names unchanged

Artifact **names** (e.g. `promotion-manifest`, `security-verdict`, `production-deploy-evidence`) are stable; downstream jobs resolve artifacts by name and run id. Only retention metadata changes.

For checksums, manifest fields, and export to long-term storage, see [release-evidence-retention.md](release-evidence-retention.md).


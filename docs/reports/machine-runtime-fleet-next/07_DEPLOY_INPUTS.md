# Deploy Inputs — Machine Runtime Fleet

**UTC:** 20260703T225800Z

## Discovery results

### `origin/main` SHA

```
277a3ad4dbe34f204704ed4c3d713ec49bff4ec2
```

### Build and Push Images (main, success)

| Field | Value |
|-------|-------|
| Run ID | `28686709336` |
| URL | https://github.com/leduytuanvu/avf-vending-api/actions/runs/28686709336 |
| headSha | `277a3ad4dbe34f204704ed4c3d713ec49bff4ec2` |
| Conclusion | success |

### Security Release (main, success)

| Field | Value |
|-------|-------|
| Run ID | `28686850966` |
| URL | https://github.com/leduytuanvu/avf-vending-api/actions/runs/28686850966 |
| headSha | `277a3ad4dbe34f204704ed4c3d713ec49bff4ec2` |
| `source_build_run_id` | `28686709336` |
| Verdict | pass |

### Digest-pinned image refs (from `security-verdict.json`)

| Image | Ref |
|-------|-----|
| App | `ghcr.io/leduytuanvu/avf-vending-api@sha256:a29ac35626a45829ee8bc085e8c024a092685723d74cf857f14a1281f446ae4b` |
| Goose | `ghcr.io/leduytuanvu/avf-vending-api-goose@sha256:6d7ea92c549707b607911c23d6295af4226d50dc6c811dd5c0b8757a4ec5375e` |

## Deploy workflow inputs

| Input | Value |
|-------|-------|
| `action_mode` | `deploy` |
| `build_run_id` | `28686709336` |
| `security_release_run_id` | `28686850966` |
| `release_tag` | `runtime-fleet-20260703T225800Z` |
| `source_commit_sha` | `277a3ad4dbe34f204704ed4c3d713ec49bff4ec2` (full SHA required — short prefix rejected by promotion gate) |
| `app_image_ref` | `ghcr.io/leduytuanvu/avf-vending-api@sha256:a29ac35626a45829ee8bc085e8c024a092685723d74cf857f14a1281f446ae4b` |
| `goose_image_ref` | `ghcr.io/leduytuanvu/avf-vending-api-goose@sha256:6d7ea92c549707b607911c23d6295af4226d50dc6c811dd5c0b8757a4ec5375e` |
| `deploy_production_confirmation` | `DEPLOY_PRODUCTION` |
| `run_migration` | `true` |
| `allow_missing_staging_evidence` | `true` |
| `missing_staging_evidence_reason` | `Runtime fleet production deploy; staging evidence bypass per operator approval` |

## Dispatch command (executed)

```bash
gh workflow run deploy-prod.yml --ref main \
  -f action_mode=deploy \
  -f build_run_id=28686709336 \
  -f security_release_run_id=28686850966 \
  -f release_tag=runtime-fleet-20260703T225800Z \
  -f source_commit_sha=277a3ad4 \
  -f app_image_ref=ghcr.io/leduytuanvu/avf-vending-api@sha256:a29ac35626a45829ee8bc085e8c024a092685723d74cf857f14a1281f446ae4b \
  -f goose_image_ref=ghcr.io/leduytuanvu/avf-vending-api-goose@sha256:6d7ea92c549707b607911c23d6295af4226d50dc6c811dd5c0b8757a4ec5375e \
  -f deploy_production_confirmation=DEPLOY_PRODUCTION \
  -f run_migration=true \
  -f allow_missing_staging_evidence=true \
  -f missing_staging_evidence_reason="Runtime fleet production deploy; staging evidence bypass per operator approval"
```

Gate: **PASS** — Build + Security Release green for `277a3ad4`.

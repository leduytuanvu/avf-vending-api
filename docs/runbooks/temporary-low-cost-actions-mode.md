# Temporary Low-Cost Actions Mode

This runbook describes the smallest practical set of temporary workflow changes to bring GitHub Actions cost close to zero while preserving a manual path for production operations.

## Enterprise / P2 evidence when CI workflows are paused

If **`ci.yml`**, **`build-push.yml`**, or **`enterprise-release-verify.yml`** are temporarily manual-only (see sections below), **releases to production still require**:

- **`make verify-enterprise-release`** (or equivalent **`verify_enterprise_result.json`**) executed on a controlled runner **before** **`deploy-prod`** approval.
- Field evidence packs per **[`testing/field-test-cases.md`](../testing/field-test-cases.md)** + **[`operations/field-pilot-checklist.md`](../operations/field-pilot-checklist.md)** — **not replaced** by a green smoke script alone.

---

## Goal

Reduce GitHub-hosted runner usage to near zero by:

- disabling automatic CI/build/security/staging runs
- disabling scheduled **Nightly Security Rescan** (and not running **Manual Ops Evidence Check** ad hoc)
- keeping a manual production workflow available
- making re-enable steps explicit and reversible

This runbook does not change repository code paths or deployment scripts. It only changes when workflows are allowed to run.

## What costs the most in this repo

The highest-cost workflows are the automatic ones:

1. `build-push.yml`
   - builds and pushes two Docker images
   - publishes build artifacts
2. `security.yml`
   - runs on `push`, `pull_request`, `workflow_run`, and `schedule`
   - includes published image scans
3. `nightly-ops.yml` (**Manual Ops Evidence Check**)
   - is already **`workflow_dispatch` only** in this repository (no `schedule`); cost accrues only when someone runs it
4. `deploy-develop.yml`
   - runs automatically from successful `develop` image builds
5. `ci.yml`
   - runs on every push / PR to `develop` and `main`

`deploy-prod.yml` is already `workflow_dispatch` only, so it is not a recurring cost unless an operator runs it.

## Recommended temporary low-cost mode

Use this mode if you want cost close to zero with the least operational impact:

- keep `deploy-prod.yml` as-is
- make `build-push.yml` manual-only
- make `security.yml` manual-only
- disable `deploy-develop.yml` automatic staging
- for **Nightly Security Rescan**, remove the `schedule` block or set `on: workflow_dispatch` only
- disable `ci.yml` automatic push / PR execution

After these changes, Actions usage should only happen when someone explicitly runs a workflow by hand.

## Files to change now

### 1. Make `build-push.yml` manual-only

File: `.github/workflows/build-push.yml`

Replace:

```yaml
on:
  push:
    branches:
      - develop
      - main
    tags:
      - "v*.*.*"
  workflow_dispatch:
```

With:

```yaml
on:
  workflow_dispatch:
```

Impact:

- stops automatic image builds on `develop`, `main`, and release tags
- downstream automatic security/staging runs will also stop unless manually triggered

### 2. Make `security.yml` manual-only

File: `.github/workflows/security.yml`

Replace:

```yaml
on:
  pull_request:
    branches:
      - develop
      - main
  push:
    branches:
      - develop
      - main
  workflow_run:
    workflows:
      - Build and Push Images
    types:
      - completed
  schedule:
    - cron: "17 3 * * *"
  workflow_dispatch:
```

With:

```yaml
on:
  workflow_dispatch:
```

Impact:

- stops automatic repo scans on push / PR
- stops automatic image scan after build
- stops scheduled nightly security scan

### 3. Disable automatic staging deployment

File: `.github/workflows/deploy-develop.yml`

Replace:

```yaml
on:
  workflow_run:
    workflows:
      - Build and Push Images
    types:
      - completed
```

With:

```yaml
on:
  workflow_dispatch:
```

Notes:

- this temporarily disables the automatic staging/pre-prod contract
- if you later want manual staging, you may need to add explicit `workflow_dispatch` inputs; if not, simply restore the original trigger when re-enabling

### 4. `nightly-ops.yml` (Manual Ops Evidence Check)

This workflow is **already** `workflow_dispatch` only (no nightly `schedule` in the default file). You do **not** need to “remove a schedule” here — simply **do not run** it in low-cost mode, or add a `concurrency` cancel policy if you want to prevent parallel heavy runs. **Nightly Security Rescan** (`nightly-security.yml`) is the one with a `schedule` to disable if you need to cut scheduled runner minutes (see that file’s `on:` block).

### 5. Disable automatic CI

File: `.github/workflows/ci.yml`

Replace:

```yaml
on:
  pull_request:
    branches:
      - develop
      - main
  push:
    branches:
      - develop
      - main
```

With:

```yaml
on:
  workflow_dispatch:
```

Impact:

- stops automatic lint/test/compose validation on code changes
- should be considered temporary only

## Workflows to keep enabled

Keep these unchanged:

- `.github/workflows/deploy-prod.yml`
  - already manual-only
  - keep this if you still want on-demand production deploy/rollback

Keep `codeql.yml` unchanged unless you explicitly enable it later:

- `.github/workflows/codeql.yml`
  - already gated by `vars.ENABLE_CODE_SCANNING == 'true'`
  - cost should remain negligible while that variable is not enabled

## Lowest-risk order to disable

Apply in this order:

1. `nightly-security.yml` (remove `schedule` if present)
2. `security.yml`
3. `deploy-develop.yml`
4. `build-push.yml`
5. `ci.yml`

This order removes recurring and downstream cost first, then removes developer-facing automation last.

## How to re-enable later

Restore the original `on:` blocks shown above:

- `ci.yml`: restore `push` + `pull_request`
- `build-push.yml`: restore `push` + tag trigger + `workflow_dispatch`
- `security.yml`: restore `pull_request`, `push`, `workflow_run`, `schedule`, `workflow_dispatch`
- `deploy-develop.yml`: restore `workflow_run`
- `nightly-security.yml`: restore `schedule` + `workflow_dispatch` if you disabled the cron

If you want a safer phased restore:

1. restore `ci.yml`
2. restore `build-push.yml`
3. restore `security.yml`
4. restore `deploy-develop.yml`
5. (optional) re-enable any scheduled security rescan

## Expected cost profile after these temporary changes

After the recommended changes:

- no cost from pushes to `develop` / `main`
- no cost from PR validation
- no daily scheduled cost
- cost only when an operator manually runs a workflow

That is the closest practical path to zero cost without deleting workflows or changing the release/deploy scripts themselves.


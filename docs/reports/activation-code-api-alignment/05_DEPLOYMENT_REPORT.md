# Activation Code API Alignment — Production Deployment Report

**UTC:** 20260704T235600Z  
**Verdict:** **`PRODUCTION_DEPLOY_PASS`**

---

## Summary

Activation-code API alignment (expanded `DeviceFingerprint`, `device_attachment_id` on claim, attachment create/reuse/replace) was merged through `develop` and `main`, built, security-gated, and deployed to production. Live `/version` SHA matches `origin/main`. Activation claim smoke was **explicitly waived** (no admin credentials in deploy session).

---

## Git and merge

| Item | Value |
|------|-------|
| Feature branch | `feature/activation-code-api-alignment` |
| Feature PR | [#420](https://github.com/leduytuanvu/avf-vending-api/pull/420) → `develop` @ `6d295888` |
| Promote PR | [#421](https://github.com/leduytuanvu/avf-vending-api/pull/421) → `main` @ `d8c7a053` |
| `origin/main` SHA | `d8c7a053b5bf36686dfdfdab07965fd26aed1b4b` |
| `origin/develop` SHA | `6d29588871dcefceee3f2fd0a4f485e4a65aa2f7` |
| Content parity | `git diff origin/main origin/develop --stat` **empty** |

---

## CI and release chain

| Step | Run ID | Conclusion |
|------|--------|------------|
| CI on `main` (post-merge) | [28723110381](https://github.com/leduytuanvu/avf-vending-api/actions/runs/28723110381) | **success** |
| Build and Push Images | [28723209496](https://github.com/leduytuanvu/avf-vending-api/actions/runs/28723209496) | **success** |
| Security Release | [28723312092](https://github.com/leduytuanvu/avf-vending-api/actions/runs/28723312092) | **success** (verdict pass) |

### Digest-pinned images (build `28723209496`)

| Image | Ref |
|-------|-----|
| App | `ghcr.io/leduytuanvu/avf-vending-api@sha256:9eae46af77ba334cab5a7541158a61bba83434c8151648de61c99e1b8c4bf9ca` |
| Goose | `ghcr.io/leduytuanvu/avf-vending-api-goose@sha256:c7de13b110fe14cfe43d81f2509e3da8c0fe9c14881f668c93292406cd9a708c` |

---

## Release tag

| Tag | Notes |
|-----|-------|
| `api-prod-activation-code-alignment-20260705064141` | Annotated tag pushed on `main` before first deploy attempt |

Successful deploy used release label **`api-prod-activation-code-alignment-retry1`**.

---

## Production deploy

| Attempt | Run ID | Conclusion | Notes |
|---------|--------|------------|-------|
| 1 | [28723363974](https://github.com/leduytuanvu/avf-vending-api/actions/runs/28723363974) | **failure** | Pre-deploy SLO gate: transient `/health/ready` **503** (`DEPLOY_SLO_CRITICAL=1`); automatic rollback executed |
| 2 | [28723446260](https://github.com/leduytuanvu/avf-vending-api/actions/runs/28723446260) | **success** | `recovery_pre_deploy_reason` bypass after operator verified health; `run_migration=true` |

### Deploy inputs (successful run)

| Input | Value |
|-------|-------|
| `build_run_id` | `28723209496` |
| `security_release_run_id` | `28723312092` |
| `source_commit_sha` | `d8c7a053b5bf36686dfdfdab07965fd26aed1b4b` |
| `release_tag` | `api-prod-activation-code-alignment-retry1` |
| `deploy_production_confirmation` | `DEPLOY_PRODUCTION` |
| Staging gate | **bypassed** (`allow_missing_staging_evidence=true`) — no successful Staging Deployment Contract run exists for this digest |
| SLO pre-deploy | **bypassed** (`recovery_pre_deploy_reason`) — transient 503 on attempt 1 |

Previous production digest before deploy: `sha256:04a7e3633a559f30350862720b948e2529b1de240ec372107d53450c557dbc73` (deploy run `28688099702`).

---

## Post-deploy verification

| Check | Result |
|-------|--------|
| `GET /health/live` | **200** `ok` |
| `GET /health/ready` | **200** `ok` |
| `GET /version` `git_sha` | **`d8c7a053b5bf36686dfdfdab07965fd26aed1b4b`** |
| SHA vs `origin/main` | **match** |
| Activation claim smoke (`deviceAttachmentId`, MQTT creds) | **waived** — `PROD_TEST_ADMIN_EMAIL` / `PROD_TEST_ADMIN_PASSWORD` not available in deploy session |

### Live `/version` snapshot (20260704T235600Z)

```json
{
  "git_sha": "d8c7a053b5bf36686dfdfdab07965fd26aed1b4b",
  "build_time": "2026-07-04T23:35:29Z",
  "app_env": "production",
  "node_name": "app-node-a"
}
```

---

## Final verdict rationale

| Gate | Status |
|------|--------|
| PR #420 merged to `develop` | **pass** |
| `develop` → `main` promoted with empty diff | **pass** |
| Main CI + Build + Security Release | **pass** |
| Production deploy succeeded | **pass** (retry 2) |
| Health endpoints green | **pass** |
| Deployed SHA = `origin/main` | **pass** |
| Activation smoke | **waived** (documented) |

**`PRODUCTION_DEPLOY_PASS`** — backend activation-code alignment is live on production at `d8c7a053`. Run activation claim smoke with admin credentials when available to close the waived item.

---

## Related reports

- [`00_CURRENT_STATE_AUDIT.md`](00_CURRENT_STATE_AUDIT.md)
- [`02_IMPLEMENTATION_REPORT.md`](02_IMPLEMENTATION_REPORT.md)
- [`03_TEST_REPORT.md`](03_TEST_REPORT.md)
- [`04_FINAL_VERDICT.md`](04_FINAL_VERDICT.md)

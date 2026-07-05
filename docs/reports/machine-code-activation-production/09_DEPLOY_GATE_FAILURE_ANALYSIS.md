# Deploy Gate Failure Analysis — Pre-Deploy SLO

**UTC:** 20260705T213550Z  
**Workflow run:** [28755628234](https://github.com/leduytuanvu/avf-vending-api/actions/runs/28755628234)  
**Classification:** Transient public readiness failure — production genuinely returned 503 on first probe; release blocked before rollout.

---

## Summary

Production deploy for machine-code activation follow-up (`e57c9486`) failed at **Collect pre-deploy SLO evidence**. Public `/health/live` was **200** while `/health/ready` was **503**. Automatic rollback did not change app nodes because the release never started. Production remained on **`22e56f0f`**. No user impact.

---

## Failure details

| Field | Value |
|-------|-------|
| Failed job | `Deploy production release` |
| Failed step | `Collect pre-deploy SLO evidence` |
| Command | `bash scripts/deploy/monitoring/collect_deploy_slo_evidence.sh --json --phase pre_deploy --out deployment-evidence/slo-pre-deploy.json` |
| Exit code | **1** (`critical_health_ok=false` when `DEPLOY_SLO_CRITICAL=1`) |
| Target SHA (not deployed) | `e57c9486` |
| Production SHA (unchanged) | `22e56f0f` |

---

## Artifact evidence (`slo-pre-deploy.json`)

Collected at `2026-07-05T21:35:50Z`:

```json
"critical": {
  "mode_enabled": true,
  "assessment": "fail",
  "public_health_ready": { "http_code": "503", "status": "fail" },
  "public_health_live":  { "http_code": "200", "status": "pass" }
}
```

Optional `/version` returned **200** with `git_sha: 22e56f0f972cc94031d95371ce79007f57cf6fb8`.

---

## Root cause

| Layer | Finding |
|-------|---------|
| Gate script | Single-shot probe with no retry when `DEPLOY_SLO_CRITICAL=1` |
| Production | Transient readiness **503** while liveness **200** — same pattern as deploy [28723363974](https://github.com/leduytuanvu/avf-vending-api/actions/runs/28723363974) (documented in [activation-code alignment deploy report](../activation-code-api-alignment/05_DEPLOYMENT_REPORT.md)) |
| Rollout | Blocked correctly; no image rollout occurred |

---

## Rollback and user impact

| Item | Outcome |
|------|---------|
| Automatic rollback | Executed in workflow; **no app-node change** (release never started) |
| Production SHA | Unchanged `22e56f0f` |
| User impact | **None** — deploy gate prevented rollout on unhealthy readiness |

---

## Remediation

1. **Script fix (primary):** Add retry/backoff for `/health/ready` and `/health/live` when `DEPLOY_SLO_CRITICAL=1` (5 attempts, 3s backoff); record `critical.retry_attempts` in JSON.
2. **Redeploy:** After fix merges to `main`, trigger **Deploy Production** with `DEPLOY_SLO_CRITICAL=1` and **no** `recovery_pre_deploy_reason` bypass.
3. **Preflight:** Operator confirms both health endpoints return **200** immediately before dispatch.

---

## References

- Collector: `scripts/deploy/monitoring/collect_deploy_slo_evidence.sh` (critical assessment ~lines 318–324, exit ~401–403)
- Local artifact: `.deploy-tmp/evidence-28755628234/slo-pre-deploy.json`
- Prior transient 503: [05_DEPLOYMENT_REPORT.md](../activation-code-api-alignment/05_DEPLOYMENT_REPORT.md)

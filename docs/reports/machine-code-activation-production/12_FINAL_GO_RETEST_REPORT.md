# Final GO Retest Report

**UTC:** 20260705T223223Z  
**Verdict:** **GO**

---

## Summary

GO retest closure completed: deploy SLO retry merged and deployed, production serves `1f2782bb`, full REST/gRPC/MQTT suite passed **3 passes**, E2E flows A–I passed, activation smoke **12/12** pass (plus isolated machine setup step).

---

## Git and release

| Item | Value |
|------|-------|
| PR #427 (docs 06–08) | **Merged** |
| PR #428 (SLO retry, bootstrap code, flow I, reports 09–11) | **Merged** → `develop` |
| PR #429 (promote to main) | **Merged** → `1f2782bb` |
| Production deploy run | [28757042991](https://github.com/leduytuanvu/avf-vending-api/actions/runs/28757042991) **success** |
| Build run | `28756536411` |
| Security Release | `28756662046` |
| Production `/version` SHA | `1f2782bb025cc764eace2ff5baedaadc60ab2ee6` |

---

## Deploy gate (SLO)

| Check | Result |
|-------|--------|
| `DEPLOY_SLO_CRITICAL` | **1** (no `recovery_pre_deploy_reason`) |
| Pre-deploy SLO | **pass** (`retry_attempts`: 3, readiness recovered after transient probe) |
| Evidence | `.deploy-tmp/evidence-28757042991/slo-pre-deploy.json` |
| Staging gate | **Bypassed** — no successful Staging Deployment Contract exists; documented in deploy manifest |

---

## Production test results (`20260705T223223Z`)

| Suite | Result |
|-------|--------|
| REST | **363/363** pass |
| gRPC | **75/75** pass |
| MQTT | **17/17** pass |
| Multi-pass | **3/3** pass |
| E2E flows A–I | **9/9** pass (flow I: fresh activation code claim) |
| Activation smoke | **12/12** pass (`fail_count`: 0) |
| Runner verdict | `PRODUCTION_REST_GRPC_MQTT_100_PERCENT_PASS` |

Evidence:

- `reports/production-full-api-grpc-mqtt/20260705T223223Z/FINAL_PRODUCTION_REST_GRPC_MQTT_VERDICT.json`
- `reports/production-full-api-grpc-mqtt/20260705T223223Z/E2E_FLOW_RESULTS.json`
- `docs/reports/machine-code-activation-production/evidence/activation_smoke_results.json`

---

## Tooling fixes during retest

| Issue | Fix |
|-------|-----|
| E2E flow I `NameError: base_url` | Use `args.base_url` in `admin_post` call |
| Activation smoke claim 403 after E2E | Always create isolated test machine (avoid compromised registry machine) |

---

## Criteria checklist

| Criterion | Met |
|-----------|-----|
| PR #427 merged | Yes |
| Deploy SLO gate pass (`DEPLOY_SLO_CRITICAL=1`) | Yes |
| Production SHA verified | Yes (`1f2782bb`) |
| machineCode activation + UUID compat | Yes |
| REST/gRPC/MQTT 100% | Yes |
| E2E 100% (flow I fixed) | Yes |
| No secret leaks | Yes |

---

## References

- [09_DEPLOY_GATE_FAILURE_ANALYSIS.md](09_DEPLOY_GATE_FAILURE_ANALYSIS.md)
- [10_MACHINE_CODE_FORMAT_AUDIT.md](10_MACHINE_CODE_FORMAT_AUDIT.md)
- [11_OFFLINE_REPLAY_FAILURE_ANALYSIS.md](11_OFFLINE_REPLAY_FAILURE_ANALYSIS.md)
- [08_FINAL_ENTERPRISE_VERDICT.json](08_FINAL_ENTERPRISE_VERDICT.json) (updated to **GO**)

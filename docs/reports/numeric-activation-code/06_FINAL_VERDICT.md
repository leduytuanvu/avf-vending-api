# Numeric Activation Code — Final Verdict

**Verdict:** **GO**

**UTC:** 20260706T103900Z  
**Production SHA:** `999b9e93e4acc63dacb2c8087bc0a8ea47316a00`

## Criteria

| Requirement | Status |
|-------------|--------|
| Production deploy succeeded | Yes (run 28784499441) |
| `/version` SHA match | Yes |
| REST 100% pass (363/363 × 3 passes) | Yes |
| gRPC 100% pass (75/75 × 3 passes) | Yes |
| MQTT 100% pass (17/17 × 3 passes) | Yes |
| Numeric activation smoke 100% | Yes (16/16, exit 0) |
| Security/fake-pass audit | Pass |
| Required failed/blocked/not_run | 0 |

## Activation code behavior verified in production

- New codes are exactly **6 digits** (e.g. from create response `activation_code_len: 6`)
- Old `AVF-...` format rejected with `activation_invalid`
- `machineCode` unchanged (`AVF406164` etc.)
- `machineId` remains UUID

## Evidence

- `docs/reports/numeric-activation-code/04_PRODUCTION_DEPLOY_REPORT.md`
- `docs/reports/numeric-activation-code/05_PRODUCTION_TEST_REPORT.md`
- `reports/production-full-api-grpc-mqtt/20260706T103058Z/`

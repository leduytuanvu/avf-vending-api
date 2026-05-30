# Production E2E — production-readonly-smoke

- **Verdict:** PASS
- **Exit code:** 0
- **Generated:** 2026-05-29T17:51:48.613204+00:00

| Probe | Outcome | Latency ms | Detail |
|---|---|---:|---|
| http/health/live | PASS | 1023 | status=200 |
| http/health/ready | PASS | 683 | status=200 |
| http/version | PASS | 701 | status=200 |
| version.payment_runtime | SKIP | 0 | field absent on this deployment |
| admin.auth | SKIP | 0 | ADMIN_TOKEN or ADMIN_EMAIL+ADMIN_PASSWORD not set |
| grpc | SKIP | 0 | GRPC_ADDR / GRPC_TARGET unset |

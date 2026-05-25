# Production E2E Result

- run_id: `sample-fail`
- prefix: `E2E-PROD-sample-fail`
- base_url: `https://api.example.test`
- started: `2026-05-25T00:00:00Z`

## Flow results

| id | label | protocol | status | evidence |
|----|-------|----------|--------|----------|
| REST-PREFLIGHT-001 | GET /health/live | rest | pass | `rest-health-live` |
| REST-COV-GET-0063 | GET /v1/admin/reports/export | rest | fail | `rest-cov-0063` |

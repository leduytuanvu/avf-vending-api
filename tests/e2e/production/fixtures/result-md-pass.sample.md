# Production E2E Result

- run_id: `sample-pass`
- prefix: `E2E-PROD-sample-pass`
- base_url: `https://api.example.test`
- started: `2026-05-25T00:00:00Z`

## Flow results

| id | label | protocol | status | evidence |
|----|-------|----------|--------|----------|
| REST-PREFLIGHT-001 | GET /health/live | rest | pass | `rest-health-live` |
| GRPC-COMM-FAIL-001 | Vend failure path | grpc | pass | `grpc-vend-fail` |
| MQTT-CONN-001 | MQTT connect | mqtt | pass | `mqtt-connect` |

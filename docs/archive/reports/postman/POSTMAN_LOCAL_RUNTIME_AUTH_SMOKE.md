> **Relocated from avf-vending-system** — canonical home: `avf-vending-api/docs/reports/postman/`. System copy removed 2026-06-16.

﻿# Postman Local Runtime Auth Smoke

Generated: 2026-06-12T09:00:34.6223288Z

- GET /health/live HTTP 200
- GET /version HTTP 200
- POST /v1/auth/login: PASS (in-memory helper, no persisted secrets)
- GET /v1/auth/me: PASS

Secrets: in-memory only; no adminPassword/tokens in local-postman-runtime.

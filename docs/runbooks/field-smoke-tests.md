# Field smoke tests

Use these checks for local development, staging rehearsal, and field-pilot validation. Production deploy smoke remains read-only; mutating field smoke must use local, staging, or an explicitly approved pilot sandbox.

## Read-only post-deploy smoke

Git Bash:

```bash
export BASE_URL="https://api.example.com"
export ENVIRONMENT_NAME="staging"
bash scripts/deploy/smoke_test.sh
```

PowerShell:

```powershell
$env:BASE_URL = "https://api.example.com"
$env:ENVIRONMENT_NAME = "staging"
python tools/smoke_test.py --report smoke-reports/smoke-test.json
```

Checks: DNS for HTTPS, `GET /health/live`, `GET /health/ready`, `GET /version`, optional MQTT/EMQX HTTP probe, optional authenticated read-only GET, and Swagger/OpenAPI when configured by smoke vars. It does not run writes.

## Local or staging field smoke

Git Bash:

```bash
export BASE_URL="http://localhost:8080"
export ORG_ID="11111111-1111-1111-1111-111111111111"
export ADMIN_EMAIL="admin@local.test"
export ADMIN_PASSWORD="password123"
export MACHINE_ID="55555555-5555-5555-5555-555555555555"
export COMMERCE_PAYMENT_WEBHOOK_SECRET="dev-secret"   # optional; skip if not configured
bash scripts/smoke/local_field_smoke.sh --evidence-json smoke-reports/local-field-smoke.json
```

PowerShell:

```powershell
$env:BASE_URL = "http://localhost:8080"
$env:ORG_ID = "11111111-1111-1111-1111-111111111111"
$env:ADMIN_EMAIL = "admin@local.test"
$env:ADMIN_PASSWORD = "password123"
$env:MACHINE_ID = "55555555-5555-5555-5555-555555555555"
$env:COMMERCE_PAYMENT_WEBHOOK_SECRET = "dev-secret"   # optional; omit with -SkipPaymentWebhook
.\scripts\smoke\local_field_smoke.ps1 -EvidenceJson smoke-reports/local-field-smoke.json
```

This mutating smoke currently exercises:

- health live/ready
- `/swagger/doc.json` fetch and local `$ref` validation
- admin login and `/v1/auth/me`
- catalog brand/category/tag/product create and product list
- machine activation-code create/list and public claim when possible
- setup bootstrap, sale catalog, and telemetry snapshot reads
- cash checkout and vend start/success
- payment webhook HMAC plus idempotent replay when a webhook secret is configured
- refund create/list when supported by the configured provider path
- operator session plus inventory cycle-count no-op when data supports it
- report reads

## Conditional checks

- **Machine gRPC auth smoke:** run only when you have a machine gRPC test client and a valid Machine JWT. The repo implements machine gRPC services and auth, but this smoke wrapper does not shell out to a generic client because local TLS/auth configuration varies.
- **Command dispatch/ACK smoke:** run only with local MQTT/EMQX and a test device or simulator subscribed to the command topic. Do not run command dispatch against production hardware from CI.
- **Reconciler dry-run:** `cmd/reconciler` is safe to start in list-only/default mode; close-the-loop actions require `RECONCILER_ACTIONS_ENABLED=true` and should not be part of default smoke.

## Evidence

Keep JSON evidence and terminal output with the deployment or pilot ticket. Redact Bearer tokens, activation plaintext codes, webhook signatures, MQTT credentials, and payment provider secrets.

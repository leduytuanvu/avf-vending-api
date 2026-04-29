# Redis outage behavior

Redis is used for optional cache, rate limit, session, lock, revocation, and machine gRPC hot-method rate-limit helpers. PostgreSQL remains the source of truth.

## Startup and readiness

- `REDIS_ENABLED=true` requires `REDIS_ADDR` or `REDIS_URL`.
- In deployed environments, Redis TLS settings are validated and insecure TLS skip-verify is rejected.
- `READINESS_STRICT=true` can make `/health/ready` fail when required Redis wiring is unavailable.
- Local development can run without Redis for many paths; the API logs fallback behavior for in-memory rate limiting when allowed.

## Runtime impact by feature

- Sale-catalog/media cache: stale or missed cache hits; PostgreSQL-backed reads remain authoritative.
- HTTP abuse/rate limiting: Redis-backed distributed counters may fall back only where explicitly allowed for local development. Production should treat Redis rate-limit loss as degraded protection.
- Refresh-session cache: Postgres remains authoritative; cache misses increase DB load.
- Login lockout counters: Redis outage can weaken fast lockout counters if enabled without fail-closed policy.
- Access-token/JTI revocation: behavior depends on `AUTH_REVOCATION_REDIS_FAIL_OPEN`; fail-open is local troubleshooting only, not a production default to rely on.
- Owner-token locks: critical multi-worker sections may fail to acquire distributed locks and should be treated as degraded.
- Machine gRPC hot-method rate limiting: distributed limits require Redis; local may fall back to in-memory.

## Incident steps

1. Check `/health/ready`, API logs, worker logs, and Redis provider status.
2. Identify enabled Redis runtime features from environment variables.
3. If production readiness is failing, fix Redis connectivity or intentionally roll back the release/config.
4. If cache only is affected, verify direct DB-backed reads and let caches refill after Redis recovers.
5. If revocation, lock, or rate-limit behavior is affected, treat as security-sensitive and escalate.

## Validation commands

Git Bash:

```bash
go run ./cmd/cli -validate-config
curl -fsS "$BASE_URL/health/ready"
```

PowerShell:

```powershell
go run ./cmd/cli -validate-config
Invoke-WebRequest -UseBasicParsing "$env:BASE_URL/health/ready"
```

Do not disable Redis-backed security controls in production to clear readiness without incident approval.

## Prometheus signals

- Elevated **`http_errors_total`** / **`grpc_errors_total`** (429 / readiness failures) when rate limits or revocation caches degrade.
- **`grpc_auth_failures_total{reason=…}`** should not spike solely because Redis returned errors unless auth plumbing maps failures into credential rejection — correlate with Redis provider dashboards first.

Canonical names: [`docs/observability/production-metrics.md`](../observability/production-metrics.md).

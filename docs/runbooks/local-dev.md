# Local development (Docker)

## Start dependencies

From the repository root:

```bash
make dev-up
# or: docker compose -f deployments/docker/docker-compose.yml up -d
```

Postgres is listening on `localhost:5432` (default user/password in compose: `postgres` / `postgres`, database `avf_vending`). Optional profiles: `broker` (EMQX, MinIO), `observability`, etc. — see `deployments/docker/docker-compose.yml` and `deployments/docker/README.md`.

## Environment

Copy [`.env.local.example`](../../.env.local.example) to `.env` in the repository root (or load manually). A typical `DATABASE_URL`:

`postgres://postgres:postgres@localhost:5432/avf_vending?sslmode=disable`

Set `TEST_DATABASE_URL` the same for integration tests if needed.

## Migrations

```bash
make dev-migrate
```

Runs `scripts/verify_database_environment.sh` with `APP_ENV=development`, then Goose `up` against the local DSN.

## Tests

```bash
make dev-test
# or: go test ./...
```

## Run services

- API: `make run-api` (uses `go run ./cmd/api`; load env first).
- Worker / reconciler / mqtt-ingest: `make run-worker`, and see `cmd/reconciler`, `cmd/mqtt-ingest` in the Makefile `build` list.

**Swagger (local):** With `HTTP_SWAGGER_UI_ENABLED=true`, UI is at `http://localhost:8080/swagger/index.html` (or your `HTTP_ADDR`) and OpenAPI JSON at `/swagger/doc.json`.

## Reset local database only (destructive)

```bash
make dev-reset-db
```

Then re-run `make dev-migrate` when Postgres is ready.

## Validate config

```bash
go run ./cmd/cli -validate-config
```

## Related

- [environment-strategy.md](./environment-strategy.md) — how local fits with staging and production.
- [docs/api/](../../api/) — product/API docs.

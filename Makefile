.PHONY: tidy fmt fmt-check vet test build proto sqlc sqlc-check swagger swagger-check ci ci-gates check-placeholders check-wiring check-migrations run-api run-worker migrate-up migrate-down docker-up docker-down prod-up prod-down prod-restart prod-logs prod-status prod-migrate prod-deploy prod-backup prod-restore prod-smoke prod-compose-config prod-validate-telemetry prod-smoke-full

BIN_DIR := bin
GO ?= go
BUF ?= buf
SQLC ?= sqlc
# Python 3 for OpenAPI/Swagger generation (use `PY=python` on Windows if `python3` is not on PATH).
PY ?= python3

tidy:
	$(GO) mod tidy

fmt:
	$(GO) fmt ./...

# CI-style formatting check (fails if any .go file is not gofmt-clean).
fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed on:" && gofmt -l . && exit 1)

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

test-short:
	$(GO) test ./... -short

# Regenerate sqlc and fail if generated Go drifts from what is committed.
sqlc-check:
	$(SQLC) generate
	git diff --exit-code -- internal/gen/db/

# Regenerate embedded OpenAPI 3.0 + docs/swagger/docs.go from swag-style comments (see tools/build_openapi.py).
swagger:
	@"$(PY)" tools/build_openapi.py

# Fail if generated Swagger artifacts are out of date (run `make swagger` locally, then commit).
swagger-check: swagger
	git diff --exit-code -- docs/swagger/

check-placeholders:
	bash scripts/check_production_placeholders.sh

check-wiring:
	bash scripts/check_feature_wiring.sh

check-migrations:
	bash scripts/check_migrations.sh

# Repo-local gates (no Postgres or unit tests). Use before push; GitHub Actions runs `make ci-gates` and compose validation separately.
ci-gates: fmt-check vet check-placeholders check-wiring check-migrations sqlc-check swagger-check

# Fast local check (skips postgres integration tests via -short).
ci: ci-gates test-short

# Mirrors GitHub Actions: same gates plus full go test (set TEST_DATABASE_URL for postgres tests).
ci-full: ci-gates test

proto:
	cd proto && $(BUF) generate

# Regenerate internal/gen/db after editing db/queries/*.sql or db/schema (requires sqlc on PATH or via Docker).
sqlc:
	$(SQLC) generate

build:
	mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags "-s -w" -o $(BIN_DIR)/api ./cmd/api
	$(GO) build -trimpath -ldflags "-s -w" -o $(BIN_DIR)/worker ./cmd/worker
	$(GO) build -trimpath -ldflags "-s -w" -o $(BIN_DIR)/mqtt-ingest ./cmd/mqtt-ingest
	$(GO) build -trimpath -ldflags "-s -w" -o $(BIN_DIR)/reconciler ./cmd/reconciler
	$(GO) build -trimpath -ldflags "-s -w" -o $(BIN_DIR)/cli ./cmd/cli

run-api:
	$(GO) run ./cmd/api

run-worker:
	$(GO) run ./cmd/worker

migrate-up:
	$(GO) run github.com/pressly/goose/v3/cmd/goose@v3.24.1 -dir migrations postgres "$${DATABASE_URL}" up

migrate-down:
	$(GO) run github.com/pressly/goose/v3/cmd/goose@v3.24.1 -dir migrations postgres "$${DATABASE_URL}" down

docker-up:
	docker compose -f deployments/docker/docker-compose.yml up -d

docker-down:
	docker compose -f deployments/docker/docker-compose.yml down

# --- Lean production profile (Ubuntu VPS): deployments/prod ---
PROD_DIR := deployments/prod
PROD_COMPOSE := docker compose --env-file .env.production -f docker-compose.prod.yml
PROD_SERVICES := postgres nats emqx api worker mqtt-ingest reconciler caddy

prod-up:
	cd $(PROD_DIR) && $(PROD_COMPOSE) up -d --remove-orphans $(PROD_SERVICES)

prod-down:
	cd $(PROD_DIR) && $(PROD_COMPOSE) down --remove-orphans

prod-restart:
	cd $(PROD_DIR) && $(PROD_COMPOSE) restart $(PROD_SERVICES)

prod-deploy:
	bash $(PROD_DIR)/scripts/deploy_prod.sh

prod-logs:
	cd $(PROD_DIR) && $(PROD_COMPOSE) logs -f --tail=200 $(PROD_SERVICES)

prod-status:
	cd $(PROD_DIR) && $(PROD_COMPOSE) ps

prod-migrate:
	cd $(PROD_DIR) && $(PROD_COMPOSE) run --rm migrate

prod-backup:
	bash $(PROD_DIR)/scripts/backup_postgres.sh

prod-restore:
	@test -n "$(FILE)" || (echo "usage: make prod-restore FILE=deployments/prod/backups/avf_vending_....sql.gz CONFIRM=YES" && exit 1)
	@test "$(CONFIRM)" = "YES" || (echo "refusing destructive restore; rerun with CONFIRM=YES" && exit 1)
	bash $(PROD_DIR)/scripts/restore_postgres.sh --yes "$(FILE)"

prod-smoke:
	bash $(PROD_DIR)/scripts/healthcheck_prod.sh

prod-compose-config:
	cd $(PROD_DIR) && $(PROD_COMPOSE) config >/dev/null
	@echo "prod-compose-config: OK"

prod-validate-telemetry:
	bash $(PROD_DIR)/scripts/validate_prod_telemetry.sh

prod-smoke-full: prod-validate-telemetry prod-smoke

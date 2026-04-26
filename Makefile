.PHONY: tidy fmt fmt-apply fmt-check vet test test-short build proto sqlc sqlc-check swagger swagger-check postman-generate postman-check ci ci-gates verify-workflows ci-workflows check-placeholders check-wiring check-migrations verify-governance verify-enterprise-release build-release-evidence-pack run-api run-worker migrate-up migrate-down docker-up docker-down dev-up dev-down dev-reset-db dev-migrate dev-test staging-validate-env staging-migrate staging-smoke production-validate-env production-preflight prod-up prod-down prod-restart prod-logs prod-status prod-migrate prod-deploy prod-backup prod-restore prod-smoke prod-compose-config prod-validate-telemetry prod-smoke-full

BIN_DIR := bin
GO ?= go
BUF ?= buf
# Pin sqlc to match CI (.github/workflows/ci.yml); uses go run so PATH sqlc is not required.
SQLC_VERSION := v1.29.0
SQLC_GEN := $(GO) run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION)
# Python 3 for OpenAPI/Swagger generation (use `PY=python` on Windows if `python3` is not on PATH).
PY ?= python3

tidy:
	$(GO) mod tidy

# CI policy matches .github/workflows/ci.yml: verify gofmt, do not only apply (use fmt-apply to fix).
fmt: fmt-check

# Apply gofmt to all packages (CI uses `fmt` / fmt-check to verify only).
fmt-apply:
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
	$(SQLC_GEN) generate
	git diff --exit-code -- internal/gen/db/

# Regenerate embedded OpenAPI 3.0 + docs/swagger/docs.go from swag-style comments (see tools/build_openapi.py).
swagger:
	@"$(PY)" tools/build_openapi.py

# Fail if generated Swagger artifacts are out of date (run `make swagger` locally, then commit).
swagger-check: swagger
	git diff --exit-code -- docs/swagger/

# Regenerate Postman v2.1 collection + environment files (native artifacts; not a replacement for /swagger/doc.json).
postman-generate: swagger
	"$(PY)" tools/build_postman_collection.py

# Validate committed Postman JSON, production/staging safety flags, and no secret-like content (offline).
postman-check:
	"$(PY)" tools/check_postman_artifacts.py

check-placeholders:
	bash scripts/check_production_placeholders.sh

check-wiring:
	bash scripts/check_feature_wiring.sh

check-migrations:
	bash scripts/ci/verify_migrations.sh

# Repo-local gates (no Postgres or unit tests). Use before push; GitHub Actions runs `make ci-gates` and compose validation separately.
ci-gates: fmt-check vet check-placeholders check-wiring check-migrations sqlc-check swagger-check postman-check

# fmt (check), vet, all package tests, and build. The Go CI job also runs tidy, sqlc, swagger, etc. (see
# ci-gates) and `make test-short` in the workflow. For full static gates: make ci-gates; for short tests: make test-short
ci: fmt vet test build

# Mirrors GitHub Actions: same gates plus full go test (set TEST_DATABASE_URL for postgres tests).
ci-full: ci-gates test

# Local mirror of the workflow quality job in .github/workflows/ci.yml (actionlint + verify_workflow_contracts.sh).
# verify_workflow_contracts.sh also runs tools/verify_github_workflow_cicd_contract.py (Make targets, SHA-pinned
# third-party uses, production Docker digest policy, deploy-prod on/workflow_dispatch, staging gate strings).
# Requires: actionlint on PATH (CI: go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.12).
ci-workflows: verify-workflows
verify-workflows:
	@set -e; \
	if ! command -v actionlint >/dev/null 2>&1; then \
		echo "verify-workflows: actionlint is not on PATH. Install: go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.12" >&2; \
		exit 1; \
	fi; \
	actionlint; \
	chmod +x scripts/ci/verify_workflow_contracts.sh; \
	bash scripts/ci/verify_workflow_contracts.sh

# Live GitHub settings: branch protection (main/develop) and environment `production` (REST API). Requires
# GH_TOKEN or GITHUB_TOKEN and GITHUB_REPOSITORY=owner/repo. See docs/runbooks/github-governance.md.
verify-governance:
	bash scripts/ci/verify_github_governance.sh

# Enterprise release readiness (pilot gate; scale tiers still need storm evidence — see production-release-readiness.md):
#   1) go test ./...
#   2) make swagger + make swagger-check
#   3) bash -n on scripts/**/*.sh and deployments/**/*.sh
#   4) docker compose config against *example* env (offline)
#   5) tools/openapi_verify_release.py (production+local servers, required P0 paths, Bearer on /v1, write examples, 2xx+error examples, no planned paths, no secret-like examples)
#   6) scripts/check_stale_p0_docs.sh (no contradictory “not applied / unmounted P0” wording in docs/)
#   7) deployment example + docs/testdata secret heuristics
#   8) optional YAML parse (deployments/**/*.yml|yaml)
verify-enterprise-release:
	bash scripts/verify_enterprise_release.sh

# Assemble release evidence pack (requires env vars — see docs/runbooks/production-release-readiness.md).
build-release-evidence-pack:
	bash deployments/prod/scripts/build_release_evidence_pack.sh

proto:
	cd proto && $(BUF) generate

# Regenerate internal/gen/db after editing db/queries/*.sql or db/schema (pinned sqlc via SQLC_VERSION).
sqlc:
	$(SQLC_GEN) generate

build:
	mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags "-s -w" -o $(BIN_DIR)/api ./cmd/api
	$(GO) build -trimpath -ldflags "-s -w" -o $(BIN_DIR)/worker ./cmd/worker
	$(GO) build -trimpath -ldflags "-s -w" -o $(BIN_DIR)/mqtt-ingest ./cmd/mqtt-ingest
	$(GO) build -trimpath -ldflags "-s -w" -o $(BIN_DIR)/reconciler ./cmd/reconciler
	$(GO) build -trimpath -ldflags "-s -w" -o $(BIN_DIR)/temporal-worker ./cmd/temporal-worker
	$(GO) build -trimpath -ldflags "-s -w" -o $(BIN_DIR)/cli ./cmd/cli

run-api:
	$(GO) run ./cmd/api

run-worker:
	$(GO) run ./cmd/worker

migrate-up:
	$(GO) run github.com/pressly/goose/v3/cmd/goose@v3.27.0 -dir migrations postgres "$${DATABASE_URL}" up

migrate-down:
	$(GO) run github.com/pressly/goose/v3/cmd/goose@v3.27.0 -dir migrations postgres "$${DATABASE_URL}" down

docker-up:
	docker compose -f deployments/docker/docker-compose.yml up -d

docker-down:
	docker compose -f deployments/docker/docker-compose.yml down

# --- Local dev (Docker compose) ---
DC_LOCAL := docker compose -f deployments/docker/docker-compose.yml

dev-up:
	$(DC_LOCAL) up -d

dev-down:
	$(DC_LOCAL) down

# Destructive: removes the compose Postgres volume (name varies by project; see docs/runbooks/local-dev.md).
dev-reset-db:
	$(DC_LOCAL) down
	-@docker volume rm "avf-vending-local_postgres_data" 2>/dev/null
	-@docker volume rm "docker_postgres_data" 2>/dev/null
	$(DC_LOCAL) up -d postgres
	@echo "dev-reset-db: postgres volume reset; re-run make dev-migrate when psql is ready"

dev-migrate:
	bash -c 'set -euo pipefail; \
	  export APP_ENV=development; \
	  export DATABASE_URL=postgres://postgres:postgres@localhost:5432/avf_vending?sslmode=disable; \
	  bash scripts/verify_database_environment.sh; \
	  exec $(GO) run github.com/pressly/goose/v3/cmd/goose@v3.27.0 -dir migrations postgres "$$DATABASE_URL" up'

dev-test:
	$(GO) test ./... -count=1

# --- Staging / production (guarded; use GitHub or server-side secrets) ---

staging-validate-env:
	@bash -c 'set -euo pipefail; test "$$APP_ENV" = "staging" || { echo "staging-validate-env: set APP_ENV=staging" >&2; exit 1; }; \
	  export DATABASE_URL="$${STAGING_DATABASE_URL:-$$DATABASE_URL}"; \
	  test -n "$$DATABASE_URL" || { echo "staging-validate-env: set STAGING_DATABASE_URL or DATABASE_URL" >&2; exit 1; }; \
	  export PAYMENT_ENV="$${PAYMENT_ENV:-sandbox}"; \
	  exec bash scripts/verify_database_environment.sh'

staging-migrate:
	@bash -c 'set -euo pipefail; test "$$APP_ENV" = "staging" || { echo "staging-migrate: set APP_ENV=staging" >&2; exit 1; }; \
	  export DATABASE_URL="$${STAGING_DATABASE_URL:-$$DATABASE_URL}"; \
	  test -n "$$DATABASE_URL" || { echo "staging-migrate: set STAGING_DATABASE_URL or DATABASE_URL" >&2; exit 1; }; \
	  export PAYMENT_ENV="$${PAYMENT_ENV:-sandbox}"; \
	  bash scripts/verify_database_environment.sh; \
	  exec $(GO) run github.com/pressly/goose/v3/cmd/goose@v3.27.0 -dir migrations postgres "$$DATABASE_URL" up'

staging-smoke:
	@bash scripts/smoke_staging.sh

production-validate-env:
	@bash -c 'set -euo pipefail; test "$$APP_ENV" = "production" || { echo "production-validate-env: set APP_ENV=production" >&2; exit 1; }; \
	  export DATABASE_URL="$${PRODUCTION_DATABASE_URL:-$$DATABASE_URL}"; \
	  test -n "$$DATABASE_URL" || { echo "production-validate-env: set PRODUCTION_DATABASE_URL or DATABASE_URL" >&2; exit 1; }; \
	  exec bash scripts/verify_database_environment.sh'

production-preflight: production-validate-env
	$(MAKE) verify-enterprise-release

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

# Runs goose in the legacy prod compose profile. For full rollout + backup + guard ordering, use deployments/prod/scripts/release.sh.
prod-migrate:
	cd $(PROD_DIR) && bash -c 'set -euo pipefail; \
	  REPO_ROOT="$$(cd ../.. && pwd)"; \
	  [ -f .env.production ] || { echo "prod-missing: $(PROD_DIR)/.env.production" >&2; exit 1; }; \
	  set -a; . ./.env.production; set +a; \
	  export APP_ENV="$${APP_ENV:-production}"; \
	  if [ "$${GITHUB_ACTIONS:-}" != "true" ] && [ "$${CONFIRM_PRODUCTION_MIGRATION:-}" != "true" ]; then \
	    echo "prod-migrate: set CONFIRM_PRODUCTION_MIGRATION=true, or use bash deployments/prod/scripts/release.sh" >&2; \
	    exit 1; \
	  fi; \
	  export PAYMENT_ENV="$${PAYMENT_ENV:-live}"; \
	  export PUBLIC_BASE_URL="$${PUBLIC_BASE_URL:-https://api.ldtv.dev}"; \
	  export READINESS_STRICT="$${READINESS_STRICT:-true}"; \
	  export MQTT_TOPIC_PREFIX="$${MQTT_TOPIC_PREFIX:-avf/devices}"; \
	  bash "$$REPO_ROOT/scripts/verify_database_environment.sh" && \
	  docker compose --env-file .env.production -f docker-compose.prod.yml run --rm migrate'

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

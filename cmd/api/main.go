// @title AVF Vending HTTP API
// @version 1.0
// @description HTTP API for the AVF vending platform (`cmd/api`). All `/v1/*` routes require `Authorization: Bearer <JWT>` unless explicitly noted (liveness/readiness, optional metrics scrape, embedded Swagger when enabled).
// @description **OpenAPI:** served at `/swagger/doc.json` as **OpenAPI 3.0.3** when `HTTP_SWAGGER_UI_ENABLED=true`. Swagger UI exposes a **Servers** dropdown (Development: `http://localhost:8080`, Production: `https://api.ldtv.dev`); regenerate via `tools/build_openapi.py` when hosts change.
// @description **Errors:** JSON responses share one envelope: `{"error":{"code","message","details","requestId"}}`. `details` is always an object (empty `{}` when there is nothing extra). Middleware and handlers use the same shape. Typical codes: `unauthenticated`, `forbidden`, `organization_scope_required`, `invalid_machine_id`, `auth_misconfigured` (JWT validation misconfiguration, HTTP 503), `rate_limited` (429).
// @description **501 vs 503:** **501** `not_implemented` means the route exists but the capability is not built for this revision—`details.capability` names the feature flag. **503** `capability_not_configured` means optional infrastructure for this process is missing (e.g. MQTT publisher for remote dispatch, commerce persistence, commerce payment-session outbox env)—operators must wire configuration; clients may retry after deployment changes.
// @description **Tracing headers:** Clients may send `X-Request-ID` and `X-Correlation-ID` (alias `X-Correlation-Id`); both are echoed on responses when tracing middleware is enabled. `error.requestId` mirrors the resolved request id when available.
// @description **Idempotency:** Mutating commerce routes and remote command dispatch require `Idempotency-Key` or `X-Idempotency-Key` (see route docs). Replayed logical writes return stable ids and `replay` flags where the domain supports it.
// @description **Commerce flow (happy path):** `POST /v1/commerce/orders` → `POST .../payment-session` (outbox row) → provider capture → `POST .../webhooks` → `POST .../vend/start` → device vend → `POST .../vend/success` or `.../vend/failure`. States and conflicts are expressed as HTTP 409 with codes such as `illegal_transition` or `payment_not_settled`.
// @description **Operator sessions:** `POST .../operator-sessions/login` establishes an ACTIVE session from JWT-derived actor identity (never trust actor type from the JSON body). Logout/heartbeat/end flows are under the same `/operator-sessions` prefix; machine-scoped reads use `limit` + `meta.returned` pagination.
// @description **Remote commands:** `POST .../commands/dispatch` appends the shadow/command ledger and publishes MQTT; `GET .../commands/{sequence}/status` reads transport state mapped to `dispatch_state`. **503** when MQTT publisher env is unset. `GET .../commands/receipts` lists recent device receipts (separate read model).
// @description **Artifacts:** When `API_ARTIFACTS_ENABLED=true`, `POST /v1/admin/organizations/{orgId}/artifacts` reserves an id + upload path, then clients `PUT` bytes to object storage using returned instructions—see Artifacts tag routes.
// @description **Versioning:** Public HTTP revision is the `/v1` prefix. Fields may be added without bumping minor versions; treat unknown JSON fields as forward-compatible. Breaking changes ship under a new `/v2` prefix (not mounted here yet).
// @termsOfService https://github.com/avf/avf-vending-system/tree/main/avf-vending-api
//
// @contact.name AVF Engineering
// @contact.url https://github.com/avf/avf-vending-system
//
// @license.name License terms apply per your deployment agreement; see repository NOTICE if present.
//
// @BasePath /
package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/avf/avf-vending-api/internal/bootstrap"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/observability"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {
	// Best-effort local developer ergonomics; production should inject env via orchestrator.
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	cfg.ProcessName = "api"

	log, err := observability.NewLogger(cfg)
	if err != nil {
		panic(err)
	}
	defer func() { _ = log.Sync() }()
	if role := strings.TrimSpace(cfg.Runtime.RuntimeRole); role != "" && role != cfg.ProcessName {
		log.Fatal("runtime role mismatch", zap.String("configured_role", role), zap.String("process", cfg.ProcessName))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := bootstrap.RunAPI(ctx, cfg, log); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal("api stopped with error", zap.Error(err))
	}

	log.Info("api stopped cleanly")
}

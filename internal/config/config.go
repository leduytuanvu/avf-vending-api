package config

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/version"
	"github.com/google/uuid"
)

// AppEnvironment controls logging defaults and whether .env loading is expected in dev.
type AppEnvironment string

const (
	AppEnvDevelopment AppEnvironment = "development"
	AppEnvTest        AppEnvironment = "test"
	AppEnvStaging     AppEnvironment = "staging"
	AppEnvProduction  AppEnvironment = "production"
)

// CommerceHTTPConfig configures durable outbox defaults for payment-session HTTP (no PSP I/O).
type CommerceHTTPConfig struct {
	PaymentOutboxTopic         string
	PaymentOutboxEventType     string
	PaymentOutboxAggregateType string
	// PaymentWebhookHMACSecret is loaded from COMMERCE_PAYMENT_WEBHOOK_SECRET (preferred),
	// PAYMENT_WEBHOOK_SECRET, or COMMERCE_PAYMENT_WEBHOOK_HMAC_SECRET (legacy alias).
	// When non-empty, POST .../payments/{id}/webhooks requires X-AVF-Webhook-Timestamp + X-AVF-Webhook-Signature
	// (HMAC-SHA256 over "{timestamp}.{rawBody}").
	PaymentWebhookHMACSecret string
	// PaymentWebhookVerification selects webhook signature verification. Only "avf_hmac" is implemented.
	PaymentWebhookVerification string
	// PaymentWebhookTimestampSkew bounds replay/stale detection for X-AVF-Webhook-Timestamp (COMMERCE_PAYMENT_WEBHOOK_REPLAY_WINDOW
	// seconds preferred; COMMERCE_PAYMENT_WEBHOOK_TIMESTAMP_SKEW_SECONDS is a legacy alias).
	PaymentWebhookTimestampSkew time.Duration
	// PaymentWebhookAllowUnsigned, when true with an empty secret, skips HMAC verification only when APP_ENV is development or test.
	PaymentWebhookAllowUnsigned bool
	// PaymentWebhookUnsafeAllowUnsignedProduction allows empty secret / no HMAC in staging or production (documented unsafe).
	PaymentWebhookUnsafeAllowUnsignedProduction bool
	// PaymentWebhookProviderSecrets maps lowercased JSON "provider" keys to webhook HMAC secrets (COMMERCE_PAYMENT_WEBHOOK_SECRETS_JSON).
	// When set for a provider, it overrides PaymentWebhookHMACSecret for signature verification of that provider's callbacks.
	PaymentWebhookProviderSecrets map[string]string
	// DefaultPaymentProvider is COMMERCE_PAYMENT_PROVIDER (lowercased): preferred registry key for outbound payment sessions when implemented.
	DefaultPaymentProvider string
	// MachineOrderCheckoutMaxAge is the maximum age of an order (since created_at) for machine gRPC checkout
	// mutations (payment session, cash confirm, vend start/outcome). Loaded from COMMERCE_MACHINE_ORDER_CHECKOUT_MAX_AGE (default 30m).
	MachineOrderCheckoutMaxAge time.Duration
}

// RestrictsUnsignedCommercePaymentWebhooks reports whether APP_ENV rejects unsigned callbacks unless the unsafe escape hatch is enabled.
func (e AppEnvironment) RestrictsUnsignedCommercePaymentWebhooks() bool {
	switch e {
	case AppEnvProduction, AppEnvStaging:
		return true
	default:
		return false
	}
}

// AllowsUnsignedCommerceWebhooksInDevelopment reports whether COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED may apply (development or test only).
func (e AppEnvironment) AllowsUnsignedCommerceWebhooksInDevelopment() bool {
	switch e {
	case AppEnvDevelopment, AppEnvTest:
		return true
	default:
		return false
	}
}

// CashSettlementConfig thresholds for field cash collection close review (minor units of collection currency).
type CashSettlementConfig struct {
	VarianceReviewThresholdMinor int64
}

// APIWiringRequirements fail-fast checks for optional subsystem wiring.
// When a flag is true, API startup requires the matching RuntimeDeps field to be non-nil.
type APIWiringRequirements struct {
	RequireAuthAdapter             bool
	RequireOutboxPublisher         bool
	RequireMQTTPublisher           bool
	RequireNATSRuntime             bool
	RequirePaymentProviderRegistry bool
}

// OutboxConfig controls durable outbox publish retry and DLQ behavior.
type OutboxConfig struct {
	PublisherRequired bool
	MaxAttempts       int
	BackoffMin        time.Duration
	BackoffMax        time.Duration
	DLQEnabled        bool
}

// Config is the complete process configuration loaded from the environment.
type Config struct {
	AppEnv AppEnvironment
	// PaymentEnv is "sandbox" or "live" from PAYMENT_ENV; empty means unset (rules depend on APP_ENV).
	PaymentEnv  string
	ProcessName string
	Runtime     RuntimeConfig
	Build       BuildConfig

	LogLevel  string
	LogFormat string

	HTTP              HTTPConfig
	GRPC              GRPCConfig
	TransportBoundary TransportBoundaryConfig
	InternalGRPC      InternalGRPCConfig
	Ops               OperationsConfig

	Postgres PostgresConfig
	Redis    RedisConfig
	NATS     NATSConfig
	MQTT     MQTTConfig
	Outbox   OutboxConfig
	// Capacity limits OLTP ingress, reporting scans, and worker recovery batches (see capacity.go).
	Capacity CapacityLimitsConfig

	ReadinessStrict bool
	MetricsEnabled  bool

	// RedisRuntime enables optional Redis-backed cache/revocation/gRPC rate limits (see redis_runtime.go).
	RedisRuntime RedisRuntimeFeatures
	// MetricsExposeOnPublicHTTP registers GET /metrics on the main HTTP listener (HTTP_ADDR, e.g. :8080).
	// When false in production (the default when METRICS_EXPOSE_ON_PUBLIC_HTTP is unset), Prometheus must
	// scrape HTTP_OPS_ADDR/metrics on the private ops listener. When true in production, METRICS_SCRAPE_TOKEN
	// (min 16 chars) and PRODUCTION_PUBLIC_METRICS_ENDPOINT_ALLOWED=true are required in addition to protecting the route.
	MetricsExposeOnPublicHTTP bool
	// MetricsScrapeToken protects GET /metrics on the public listener when set (Authorization: Bearer <token>).
	// Required (min 16 chars) when APP_ENV=production and METRICS_EXPOSE_ON_PUBLIC_HTTP=true.
	MetricsScrapeToken string
	// SwaggerUIEnabled mounts Swagger UI (HTML) under /swagger/ when true. If HTTP_SWAGGER_UI_ENABLED is set,
	// only true/1 enables. If unset, non-production defaults on and production defaults off.
	// OpenAPIJSONEnabled controls GET /swagger/doc.json; when false, doc.json is not served (404).
	// SwaggerUIEnabled true requires OpenAPIJSONEnabled true (the UI loads doc.json).
	SwaggerUIEnabled   bool
	OpenAPIJSONEnabled bool
	// WorkerMetricsListen is the bind address for cmd/worker /metrics (Prometheus).
	// When empty and MetricsEnabled is true, cmd/worker defaults to 127.0.0.1:9091.
	WorkerMetricsListen string
	// ReconcilerMetricsListen is the bind address for cmd/reconciler /metrics when MetricsEnabled.
	// When empty, defaults to 127.0.0.1:9092.
	ReconcilerMetricsListen string
	// MQTTIngestMetricsListen is the bind address for cmd/mqtt-ingest /metrics when MetricsEnabled.
	// When empty, defaults to 127.0.0.1:9093.
	MQTTIngestMetricsListen string
	// TemporalWorkerMetricsListen is the bind address for cmd/temporal-worker ops endpoints.
	// When empty, defaults to 127.0.0.1:9094.
	TemporalWorkerMetricsListen string

	// WorkerOutboxOnly runs only outbox dispatch in cmd/worker (no payment timeout / stuck command tickers).
	WorkerOutboxOnly bool
	// WorkerOutboxLockTTLSeconds is the Postgres lease duration for outbox publish claims (SKIP LOCKED).
	WorkerOutboxLockTTLSeconds int
	// WorkerOutboxDisableLease skips row leasing (legacy single-replica behavior / tests).
	WorkerOutboxDisableLease bool

	APIWiring APIWiringRequirements

	Commerce CommerceHTTPConfig

	// CashSettlement configures admin cashbox / collection close review thresholds.
	CashSettlement CashSettlementConfig

	Reconciler ReconcilerConfig

	// Temporal configures optional Temporal client wiring (disabled by default).
	Temporal TemporalConfig

	Telemetry TelemetryConfig
	// MQTTDeviceTelemetry bounds device MQTT ingest in cmd/mqtt-ingest (payload, rate, queue).
	MQTTDeviceTelemetry MQTTDeviceTelemetryConfig
	// TelemetryJetStream bounds JetStream streams/consumers and worker projection (cmd/worker, mqtt-ingest).
	TelemetryJetStream TelemetryJetStreamConfig
	// TelemetryDataRetention configures Postgres telemetry/evidence pruning (cmd/worker periodic job).
	TelemetryDataRetention TelemetryDataRetentionConfig
	// EnterpriseRetention configures safe bounded cleanup for non-telemetry operational tables.
	EnterpriseRetention EnterpriseRetentionConfig
	// RetentionWorker gates cmd/worker retention tickers and optional RETENTION_DRY_RUN override.
	RetentionWorker RetentionWorkerConfig
	// RetentionAllowDestructiveLocal explicitly permits local/test destructive cleanup jobs when set.
	RetentionAllowDestructiveLocal bool

	// HTTPAuth selects JWT validation mode (HS256 dev secret vs RS256 PEM vs RS256 JWKS).
	HTTPAuth HTTPAuthConfig
	// AdminAuthSecurity configures MFA policy, login lockout, and password validation for interactive admins.
	AdminAuthSecurity AdminAuthSecurityConfig
	// MachineJWT selects validation mode for machine-runtime access JWTs. Defaults preserve local HS256 compatibility.
	MachineJWT MachineJWTConfig
	// HTTPRateLimit configures optional abuse protection on mutating API routes.
	HTTPRateLimit HTTPRateLimitConfig

	// Artifacts enables S3-backed backend artifact APIs (requires object store env when enabled).
	Artifacts ArtifactsConfig

	// Analytics optional cold-path sinks (ClickHouse HTTP); never required for OLTP correctness.
	Analytics AnalyticsConfig

	// SMTP is loaded from environment for provider-driven notification wiring. This repo does not
	// force SMTP usage at startup, but validates the shape when values are supplied.
	SMTP SMTPConfig

	// AuditCriticalFailOpen when true allows RecordCritical to swallow persistence errors (AUDIT_CRITICAL_FAIL_OPEN).
	// Forbidden when APP_ENV is staging or production.
	AuditCriticalFailOpen bool
	// PlatformAuditOrganizationID scopes enterprise audit_events for platform outbox admin mutations when
	// outbox_events.organization_id is NULL (PLATFORM_AUDIT_ORGANIZATION_ID). Optional in development/test.
	PlatformAuditOrganizationID uuid.UUID
}

// ArtifactsConfig gates /v1/admin/.../artifacts routes and upload limits.
type ArtifactsConfig struct {
	Enabled            bool
	MaxUploadBytes     int64
	DownloadPresignTTL time.Duration
	ListMaxKeys        int32
	Bucket             string
	PublicBaseURL      string
	AllowedTypes       []string
	ThumbSize          int
	DisplaySize        int
}

// HTTPAuthConfig configures Bearer JWT validation for /v1 (see internal/platform/auth).
type HTTPAuthConfig struct {
	Mode string // hs256 (default), rs256_pem, rs256_jwks, ed25519_pem, jwt_jwks

	// JWTAlgorithm optional enterprise label (HTTP_AUTH_JWT_ALG): HS256 | RS256 | EdDSA — cross-checked against Mode when set.
	JWTAlgorithm string

	JWTLeeway time.Duration

	// HS256 (and optional previous secret for rotation).
	JWTSecret         []byte
	JWTSecretPrevious []byte

	// LoginJWTSecret signs interactive session access tokens (HS256) when set; required for rs256_pem/rs256_jwks.
	// When empty in hs256 mode, session tokens fall back to JWTSecret so dev stacks can use a single secret.
	LoginJWTSecret []byte

	// AccessTokenTTL is the lifetime for session access JWTs issued by POST /v1/auth/login and /v1/auth/refresh.
	AccessTokenTTL time.Duration
	// RefreshTokenTTL is the server-side refresh token persistence horizon.
	RefreshTokenTTL time.Duration
	// MFAPendingTTL is the lifetime for MFA challenge JWTs issued between password verification and TOTP verification.
	MFAPendingTTL time.Duration

	// RS256 PEM (single public key; rotation = deploy new PEM / JWKS).
	RSAPublicKeyPEM []byte

	// Ed25519 PEM (PUBLIC KEY) for HTTP_AUTH_MODE=ed25519_pem (access tokens use EdDSA / Ed25519).
	Ed25519PublicKeyPEM []byte

	// RS256 JWKS
	JWKSURL             string
	JWKSCacheTTL        time.Duration
	JWKSSkipStartupWarm bool

	ExpectedIssuer   string
	ExpectedAudience string
}

// MachineJWTConfig configures Bearer JWT validation for machine-runtime gRPC.
// MACHINE_JWT_* is preferred for enterprise deployments; when unset, local/dev stacks
// retain backward-compatible HS256 validation using HTTP_AUTH_* secrets.
type MachineJWTConfig struct {
	Mode string // hs256 (default), rs256_pem, rs256_jwks, ed25519_pem, jwt_jwks

	// JWTAlgorithm optional enterprise label (MACHINE_JWT_ALG): HS256 | RS256 | EdDSA.
	JWTAlgorithm string

	JWTLeeway time.Duration

	// HS256 primary/previous secrets. AdditionalHS256Secrets carries legacy HTTP_AUTH fallbacks.
	JWTSecret              []byte
	JWTSecretPrevious      []byte
	AdditionalHS256Secrets [][]byte

	RSAPublicKeyPEM     []byte
	Ed25519PublicKeyPEM []byte

	JWKSURL             string
	JWKSCacheTTL        time.Duration
	JWKSSkipStartupWarm bool

	ExpectedIssuer   string
	ExpectedAudience string
	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration
	RequireAudience  bool
}

// HTTPRateLimitConfig enables token-bucket limits on sensitive mutating routes (POST under commerce, operator, dispatch).
type HTTPRateLimitConfig struct {
	SensitiveWritesEnabled bool
	SensitiveWritesRPS     float64
	SensitiveWritesBurst   int

	// Abuse configures fixed-window limits on login, admin mutations, machine telemetry/runtime, webhooks, and selected public routes.
	// Prefer REDIS_ADDR for distributed counting in multi-instance deployments; otherwise an in-memory backend is used (logged).
	Abuse AbuseRateLimitConfig
}

// AbuseRateLimitConfig is driven by RATE_LIMIT_* environment variables (see loadHTTPRateLimitConfig).
type AbuseRateLimitConfig struct {
	Enabled bool

	LoginPerMinute         int
	RefreshPerMinute       int
	AdminMutationPerMinute int
	MachinePerMinute       int
	WebhookPerMinute       int
	PublicPerMinute        int
	// CommandDispatchPerMinute limits POST /v1/machines/{id}/commands/dispatch per machine id + client IP.
	CommandDispatchPerMinute int
	// ReportsReadPerMinute limits heavy GET reporting endpoints per interactive user + organization scope.
	ReportsReadPerMinute int

	// LockoutWindow is the minimum Redis/memory bucket TTL (typically ≥ 1m); widens the observation window when larger than one minute.
	LockoutWindow time.Duration
}

// HTTPConfig holds the public HTTP API server settings.
type HTTPConfig struct {
	Addr              string
	ShutdownTimeout   time.Duration
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration

	// CORSAllowedOrigins lists allowed browser origins (go-chi/cors). Empty slice with CORSEnvPresent means the variable was set but empty (no CORS middleware).
	CORSAllowedOrigins   []string
	CORSEnvPresent       bool
	CORSAllowCredentials bool
}

// GRPCConfig holds gRPC server settings for cmd/api. When Enabled, the process listens on Addr and
// registers grpc.health.v1 (unless HealthEnabled is false), internal query services, machine v1 skeleton
// services, and optional reflection (non-production only by validate).
type GRPCConfig struct {
	Enabled         bool
	Addr            string
	ShutdownTimeout time.Duration
	// ReflectionEnabled registers gRPC server reflection (grpcurl list). Forced off in production by Validate.
	ReflectionEnabled bool
	// HealthEnabled registers grpc.health.v1 when true (default).
	HealthEnabled bool
	// HealthReflectsProcessReadiness wires grpc.health Check to the same probes as HTTP /health/ready (NOT_SERVING on failure; no error text on the wire).
	HealthReflectsProcessReadiness bool
	// RequireMachineJWT, when true (GRPC_REQUIRE_MACHINE_JWT, fallback GRPC_REQUIRE_MACHINE_AUTH),
	// requires Machine JWT on protected avf.machine.v1 RPCs (production mandates true).
	// Activation and refresh remain public because their credentials are carried in the request body.
	RequireMachineJWT bool
	// RequireGRPCIdempotency, when true (GRPC_REQUIRE_IDEMPOTENCY, default true), enables the Postgres-backed
	// unary replay interceptor for idempotent machine mutations. Disable only for isolated development experiments.
	RequireGRPCIdempotency bool
	// UnaryHandlerTimeout bounds handler execution when the incoming context has no deadline.
	UnaryHandlerTimeout time.Duration

	// PublicBaseURL is GRPC_PUBLIC_BASE_URL — advertised grpc/grpcs URI for vending clients (required in production when machine gRPC is enabled).
	PublicBaseURL string
	// BehindTLSProxy documents TLS termination at Caddy/Nginx/LB before traffic reaches this process (GRPC_BEHIND_TLS_PROXY).
	// Mutually exclusive with TLS.Enabled — production requires either BehindTLSProxy or TLS.Enabled so plaintext is never exposed publicly alone.
	BehindTLSProxy bool
	// MaxRecvMsgSize / MaxSendMsgSize bound grpc.Server inbound/outbound messages (GRPC_MAX_RECV_MSG_SIZE / GRPC_MAX_SEND_MSG_SIZE).
	// Zero uses Google gRPC defaults (~4 MiB) when constructing the server (explicit opts omitted).
	MaxRecvMsgSize int
	MaxSendMsgSize int

	// TLS optional server TLS + client certificate policy for machine gRPC (mTLS-ready).
	TLS GRPCServerTLSConfig
}

// TransportBoundaryConfig enforces documented splits between Admin REST, machine gRPC, MQTT commands,
// and legacy machine HTTP surfaces (docs/architecture/transport-boundary.md).
type TransportBoundaryConfig struct {
	// MachineRESTLegacyEnabled gates vending-machine REST runtime routes superseded by native gRPC.
	// Canonical env: ENABLE_LEGACY_MACHINE_HTTP (see loadTransportBoundary). MACHINE_REST_LEGACY_ENABLED is a deprecated alias when ENABLE_* is unset.
	// Defaults on for non-production; defaults off when APP_ENV=production unless explicitly enabled and allowed.
	MachineRESTLegacyEnabled bool
	// MachineRESTLegacyAllowInProduction must be true when enabling legacy machine REST in production.
	MachineRESTLegacyAllowInProduction bool
	// MQTTCommandTransport documents/enforces backend→device command delivery (MQTT_TLS + ledger). Default "mqtt".
	MQTTCommandTransport string
}

// GRPCServerTLSConfig configures the machine gRPC listener for TLS and optional client (device) certificates.
// Private keys are never loaded from the database; only file paths from environment.
type GRPCServerTLSConfig struct {
	Enabled  bool
	CertFile string
	KeyFile  string
	// ClientCAFile is the PEM bundle for verifying client certificates when ClientAuth is request or require.
	ClientCAFile string
	// ClientAuth: no | request | require (maps to tls.ClientAuthType).
	ClientAuth string
	// MachineIDFromCertURIPrefix prefixes URI SAN values that encode machine UUID (default urn:avf:machine:).
	MachineIDFromCertURIPrefix string
	// AllowMachineAuthCertOnly allows machine RPCs with a verified client cert registered in machine_device_certificates
	// and no Bearer token. Forbidden unless explicitly enabled; still requires a checker wired from Postgres.
	AllowMachineAuthCertOnly bool
}

// InternalGRPCConfig hosts a loopback-only listener for avf.internal.v1 read-only query RPCs (split-ready; monolith-only).
// Uses HS256 service bearer tokens (aud=avf-internal-grpc, typ=service); mTLS can wrap this listener later without changing contracts.
type InternalGRPCConfig struct {
	Enabled         bool
	Addr            string
	ShutdownTimeout time.Duration
	// ServiceTokenSecret signs/validates INTERNAL_GRPC bearer JWTs. In development/test only, may fall back to HTTP_AUTH_JWT_SECRET when unset.
	ServiceTokenSecret  []byte
	ReflectionEnabled   bool
	HealthEnabled       bool
	UnaryHandlerTimeout time.Duration
}

// OperationsConfig holds shared health/readiness/version/metrics settings for all processes.
type OperationsConfig struct {
	HTTPAddr              string
	ReadinessTimeout      time.Duration
	ShutdownTimeout       time.Duration
	TracerShutdownTimeout time.Duration
}

// PostgresConfig holds PostgreSQL pool settings used for readiness and future persistence.
type PostgresConfig struct {
	URL                    string
	MaxConns               int32
	MinConns               int32
	MaxConnIdleTime        time.Duration
	MaxConnLifetime        time.Duration
	APIMaxConns            *int32
	WorkerMaxConns         *int32
	MQTTIngestMaxConns     *int32
	ReconcilerMaxConns     *int32
	TemporalWorkerMaxConns *int32
	// SlowQueryLogThresholdMS logs queries exceeding this duration at WARN (0 = disabled). Uses DATABASE_SLOW_QUERY_LOG_MS.
	SlowQueryLogThresholdMS int
}

// PostgresPoolSummary is the effective pool shape for one process binary (for logging; never includes DATABASE_URL).
type PostgresPoolSummary struct {
	ProcessName     string
	MaxConns        int32
	MinConns        int32
	MaxConnIdleTime time.Duration
	MaxConnLifetime time.Duration
}

// PoolSummaryForProcess returns the effective limits applied by pgxpool for this process name.
func (p PostgresConfig) PoolSummaryForProcess(processName string) PostgresPoolSummary {
	return PostgresPoolSummary{
		ProcessName:     strings.TrimSpace(processName),
		MaxConns:        p.MaxConnsForProcess(processName),
		MinConns:        p.MinConns,
		MaxConnIdleTime: p.MaxConnIdleTime,
		MaxConnLifetime: p.MaxConnLifetime,
	}
}

// RedisConfig holds Redis client settings used for readiness and future cache usage.
type RedisConfig struct {
	Enabled               bool
	Addr                  string
	Username              string
	Password              string
	DB                    int
	TLSEnabled            bool
	TLSInsecureSkipVerify bool
	KeyPrefix             string
}

// TelemetryConfig holds OpenTelemetry exporter settings.
type TelemetryConfig struct {
	ServiceName  string
	OTLPEndpoint string
	Insecure     bool
	SDKDisabled  bool
}

// NATSConfig holds the internal event-bus endpoint used by the current async design.
type NATSConfig struct {
	URL      string
	Required bool
}

// MQTTConfig holds broker connection settings shared by API publish and ingest workers.
// For APP_ENV=production, validate() requires TLS (tls/ssl/mqtts URL or localhost) for any
// non-loopback broker; never expose plaintext MQTT publicly or set PRODUCTION_ALLOW_ANONYMOUS_MQTT
// except on a documented private-broker deployment.
type MQTTConfig struct {
	BrokerURL      string
	ClientID       string
	APIClientID    string
	IngestClientID string
	Username       string
	Password       string
	TopicPrefix    string
	// TopicLayout is legacy (default) or enterprise; see internal/platform/mqtt/topics.go.
	TopicLayout string

	TLSEnabled         bool
	CAFile             string
	CertFile           string
	KeyFile            string
	InsecureSkipVerify bool
}

// RuntimeConfig holds deploy-time identity and public base URL metadata.
type RuntimeConfig struct {
	PublicBaseURL        string
	MachinePublicBaseURL string
	Region               string
	NodeName             string
	InstanceID           string
	RuntimeRole          string
}

// BuildConfig holds release metadata exposed on health/version surfaces and logs.
type BuildConfig struct {
	Version   string
	GitSHA    string
	BuildTime string
}

// SMTPConfig holds optional outbound SMTP connection settings.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
}

// Validate checks invariants and cross-field rules after environment parsing.
func (c *Config) Validate() error {
	if c == nil {
		return errors.New("config: nil")
	}

	switch c.AppEnv {
	case AppEnvDevelopment, AppEnvTest, AppEnvStaging, AppEnvProduction:
	default:
		return fmt.Errorf("config: invalid APP_ENV %q (expected development, test, staging, or production)", c.AppEnv)
	}

	if c.AuditCriticalFailOpen && c.AppEnv != AppEnvDevelopment && c.AppEnv != AppEnvTest {
		return errors.New("config: AUDIT_CRITICAL_FAIL_OPEN is only allowed when APP_ENV is development or test")
	}

	if strings.TrimSpace(c.LogLevel) == "" {
		return errors.New("config: LOG_LEVEL is required")
	}

	switch strings.ToLower(strings.TrimSpace(c.LogFormat)) {
	case "json", "text":
	default:
		return fmt.Errorf("config: invalid LOG_FORMAT %q", c.LogFormat)
	}

	if err := c.HTTP.validate(); err != nil {
		return err
	}
	if err := c.GRPC.validate(); err != nil {
		return err
	}
	if err := c.InternalGRPC.validate(c.AppEnv, c.HTTPAuth); err != nil {
		return err
	}
	if err := c.Ops.validate(); err != nil {
		return err
	}
	if err := c.Postgres.validate(); err != nil {
		return err
	}
	if err := c.Redis.validate(); err != nil {
		return err
	}
	if err := c.validateRedisTLSInDeployedEnvironments(); err != nil {
		return err
	}
	if err := c.RedisRuntime.validate(c); err != nil {
		return err
	}
	if err := c.Capacity.validate(); err != nil {
		return err
	}
	if err := c.NATS.validate(); err != nil {
		return err
	}
	if err := c.Outbox.validate(); err != nil {
		return err
	}
	if c.Outbox.PublisherRequired && strings.TrimSpace(c.NATS.URL) == "" {
		return errors.New("config: OUTBOX_PUBLISHER_REQUIRED=true requires non-empty NATS_URL")
	}
	if err := c.MQTT.validate(c.AppEnv); err != nil {
		return err
	}
	if err := c.Runtime.validate(c.AppEnv); err != nil {
		return err
	}
	if err := c.Build.validate(); err != nil {
		return err
	}
	if err := c.Telemetry.validate(); err != nil {
		return err
	}
	if err := c.HTTPAuth.validate(); err != nil {
		return err
	}
	if err := c.HTTPAuth.validateJWTAlgorithmCrossCheck(); err != nil {
		return err
	}
	if err := c.HTTPAuth.validateDeployedJWTSecrets(c.AppEnv); err != nil {
		return err
	}
	if err := c.MachineJWT.validate(); err != nil {
		return err
	}
	if err := c.MachineJWT.validateJWTAlgorithmCrossCheck(); err != nil {
		return err
	}
	if err := c.MachineJWT.validateDeployedJWTSecrets(c.AppEnv); err != nil {
		return err
	}
	if err := c.HTTPRateLimit.validate(); err != nil {
		return err
	}
	if err := c.Artifacts.validate(c.AppEnv); err != nil {
		return err
	}
	if err := c.Analytics.validate(); err != nil {
		return err
	}
	if err := c.SMTP.validate(); err != nil {
		return err
	}
	if err := c.Temporal.validate(); err != nil {
		return err
	}
	if err := c.MQTTDeviceTelemetry.validate(); err != nil {
		return err
	}
	if err := c.TelemetryJetStream.validate(); err != nil {
		return err
	}
	if err := c.TelemetryDataRetention.validate(); err != nil {
		return err
	}
	if err := c.EnterpriseRetention.validate(); err != nil {
		return err
	}
	if (c.AppEnv == AppEnvDevelopment || c.AppEnv == AppEnvTest) &&
		!c.RetentionAllowDestructiveLocal &&
		(c.TelemetryDataRetention.CleanupEnabled || c.EnterpriseRetention.CleanupEnabled) {
		return errors.New("config: destructive retention cleanup is disabled in development/test unless RETENTION_ALLOW_DESTRUCTIVE_LOCAL=true")
	}
	if err := c.validateProductionTelemetryNATS(); err != nil {
		return err
	}
	if err := c.validateMetricsHTTPExposure(); err != nil {
		return err
	}
	if err := c.Commerce.validate(c.AppEnv); err != nil {
		return err
	}
	if err := c.TransportBoundary.validate(); err != nil {
		return err
	}
	if err := c.validateEnvironmentDeployment(); err != nil {
		return err
	}
	if err := c.validateSwaggerAndOpenAPI(); err != nil {
		return err
	}
	if err := c.validateGRPCProductionReflection(); err != nil {
		return err
	}
	if err := c.validateGRPCProductionExposure(); err != nil {
		return err
	}
	if err := c.validateInternalGRPCProductionReflection(); err != nil {
		return err
	}
	if err := c.validateGRPCProductionHealthReadiness(); err != nil {
		return err
	}
	if err := c.validateHTTPCORSDeploymentPolicy(); err != nil {
		return err
	}

	if strings.TrimSpace(c.WorkerMetricsListen) != "" {
		if _, err := net.ResolveTCPAddr("tcp", normalizeTCPAddr(c.WorkerMetricsListen)); err != nil {
			return fmt.Errorf("config: invalid WORKER_METRICS_LISTEN %q: %w", c.WorkerMetricsListen, err)
		}
	}
	if strings.TrimSpace(c.ReconcilerMetricsListen) != "" {
		if _, err := net.ResolveTCPAddr("tcp", normalizeTCPAddr(c.ReconcilerMetricsListen)); err != nil {
			return fmt.Errorf("config: invalid RECONCILER_METRICS_LISTEN %q: %w", c.ReconcilerMetricsListen, err)
		}
	}
	if strings.TrimSpace(c.MQTTIngestMetricsListen) != "" {
		if _, err := net.ResolveTCPAddr("tcp", normalizeTCPAddr(c.MQTTIngestMetricsListen)); err != nil {
			return fmt.Errorf("config: invalid MQTT_INGEST_METRICS_LISTEN %q: %w", c.MQTTIngestMetricsListen, err)
		}
	}
	if strings.TrimSpace(c.TemporalWorkerMetricsListen) != "" {
		if _, err := net.ResolveTCPAddr("tcp", normalizeTCPAddr(c.TemporalWorkerMetricsListen)); err != nil {
			return fmt.Errorf("config: invalid TEMPORAL_WORKER_METRICS_LISTEN %q: %w", c.TemporalWorkerMetricsListen, err)
		}
	}

	return nil
}

func (t TransportBoundaryConfig) validate() error {
	mt := strings.ToLower(strings.TrimSpace(t.MQTTCommandTransport))
	if mt != "" && mt != "mqtt" {
		return fmt.Errorf("config: MQTT_COMMAND_TRANSPORT must be mqtt (got %q)", t.MQTTCommandTransport)
	}
	return nil
}

func (h HTTPConfig) validate() error {
	if _, err := net.ResolveTCPAddr("tcp", normalizeTCPAddr(h.Addr)); err != nil {
		return fmt.Errorf("config: invalid HTTP_ADDR %q: %w", h.Addr, err)
	}
	if h.ShutdownTimeout <= 0 {
		return errors.New("config: HTTP_SHUTDOWN_TIMEOUT must be > 0")
	}
	if h.ReadHeaderTimeout <= 0 {
		return errors.New("config: HTTP_READ_HEADER_TIMEOUT must be > 0")
	}
	if h.ReadTimeout <= 0 {
		return errors.New("config: HTTP_READ_TIMEOUT must be > 0")
	}
	if h.WriteTimeout <= 0 {
		return errors.New("config: HTTP_WRITE_TIMEOUT must be > 0")
	}
	if h.IdleTimeout <= 0 {
		return errors.New("config: HTTP_IDLE_TIMEOUT must be > 0")
	}
	for _, o := range h.CORSAllowedOrigins {
		if strings.EqualFold(strings.TrimSpace(o), "*") {
			return errors.New("config: HTTP_CORS_ALLOWED_ORIGINS must not use the * wildcard")
		}
	}
	return nil
}

func (g GRPCConfig) validate() error {
	if !g.Enabled {
		return nil
	}
	if g.BehindTLSProxy && g.TLS.Enabled {
		return errors.New("config: GRPC_BEHIND_TLS_PROXY=true cannot be combined with GRPC_TLS_ENABLED=true (terminate TLS either at reverse proxy or at this process)")
	}
	if strings.TrimSpace(g.Addr) == "" {
		return errors.New("config: GRPC_ADDR is required when GRPC_ENABLED=true")
	}
	if _, err := net.ResolveTCPAddr("tcp", normalizeTCPAddr(g.Addr)); err != nil {
		return fmt.Errorf("config: invalid GRPC_ADDR %q: %w", g.Addr, err)
	}
	if g.ShutdownTimeout <= 0 {
		return errors.New("config: GRPC_SHUTDOWN_TIMEOUT must be > 0")
	}
	if g.UnaryHandlerTimeout <= 0 {
		return errors.New("config: GRPC_UNARY_HANDLER_TIMEOUT must be > 0 when GRPC_ENABLED=true")
	}
	if g.MaxRecvMsgSize < 0 || g.MaxSendMsgSize < 0 {
		return errors.New("config: GRPC_MAX_RECV_MSG_SIZE and GRPC_MAX_SEND_MSG_SIZE must be >= 0")
	}
	if err := g.TLS.validate(); err != nil {
		return err
	}
	return nil
}

func (t GRPCServerTLSConfig) validate() error {
	if !t.Enabled {
		if t.AllowMachineAuthCertOnly {
			return errors.New("config: GRPC_MACHINE_AUTH_CERT_ONLY_ALLOWED requires GRPC_TLS_ENABLED=true with client CA verification")
		}
		return nil
	}
	if strings.TrimSpace(t.CertFile) == "" || strings.TrimSpace(t.KeyFile) == "" {
		return errors.New("config: GRPC_TLS_ENABLED requires GRPC_TLS_CERT_FILE and GRPC_TLS_KEY_FILE")
	}
	mode := strings.ToLower(strings.TrimSpace(t.ClientAuth))
	switch mode {
	case "", "no", "none":
	case "request", "verify_if_given":
	case "require", "require_and_verify":
	default:
		return fmt.Errorf("config: invalid GRPC_TLS_CLIENT_AUTH %q (use no, request, require)", t.ClientAuth)
	}
	if mode == "require" || mode == "require_and_verify" || mode == "request" || mode == "verify_if_given" {
		if strings.TrimSpace(t.ClientCAFile) == "" {
			return errors.New("config: GRPC_TLS_CLIENT_AUTH=request|require requires GRPC_TLS_CLIENT_CA_FILE")
		}
	}
	if t.AllowMachineAuthCertOnly {
		if strings.TrimSpace(t.ClientCAFile) == "" {
			return errors.New("config: GRPC_MACHINE_AUTH_CERT_ONLY_ALLOWED requires GRPC_TLS_CLIENT_CA_FILE")
		}
		if mode == "" || mode == "no" || mode == "none" {
			return errors.New("config: GRPC_MACHINE_AUTH_CERT_ONLY_ALLOWED requires GRPC_TLS_CLIENT_AUTH=request or require")
		}
	}
	return nil
}

func (g InternalGRPCConfig) validate(appEnv AppEnvironment, httpAuth HTTPAuthConfig) error {
	if !g.Enabled {
		return nil
	}
	if strings.TrimSpace(g.Addr) == "" {
		return errors.New("config: INTERNAL_GRPC_ADDR is required when INTERNAL_GRPC_ENABLED=true")
	}
	if _, err := net.ResolveTCPAddr("tcp", normalizeTCPAddr(g.Addr)); err != nil {
		return fmt.Errorf("config: invalid INTERNAL_GRPC_ADDR %q: %w", g.Addr, err)
	}
	if g.ShutdownTimeout <= 0 {
		return errors.New("config: INTERNAL_GRPC_SHUTDOWN_TIMEOUT must be > 0 when INTERNAL_GRPC_ENABLED=true")
	}
	if g.UnaryHandlerTimeout <= 0 {
		return errors.New("config: INTERNAL_GRPC_UNARY_HANDLER_TIMEOUT must be > 0 when INTERNAL_GRPC_ENABLED=true")
	}
	hasDedicated := len(bytes.TrimSpace(g.ServiceTokenSecret)) > 0
	if !hasDedicated {
		switch appEnv {
		case AppEnvDevelopment, AppEnvTest:
			if len(bytes.TrimSpace(httpAuth.JWTSecret)) == 0 && len(bytes.TrimSpace(httpAuth.LoginJWTSecret)) == 0 {
				return errors.New("config: INTERNAL_GRPC_SERVICE_TOKEN_SECRET or HTTP_AUTH_JWT_SECRET is required when INTERNAL_GRPC_ENABLED in development/test")
			}
		default:
			return errors.New("config: INTERNAL_GRPC_SERVICE_TOKEN_SECRET is required when INTERNAL_GRPC_ENABLED outside development/test")
		}
	}
	if appEnv == AppEnvProduction || appEnv == AppEnvStaging {
		host, _, err := net.SplitHostPort(normalizeTCPAddr(g.Addr))
		if err != nil {
			return fmt.Errorf("config: INTERNAL_GRPC_ADDR: %w", err)
		}
		h := strings.ToLower(strings.TrimSpace(host))
		if h != "127.0.0.1" && h != "localhost" && h != "::1" {
			return fmt.Errorf("config: INTERNAL_GRPC_ADDR must bind to loopback in %s (got host %q)", appEnv, host)
		}
		if len(bytes.TrimSpace(g.ServiceTokenSecret)) < 32 {
			return errors.New("config: INTERNAL_GRPC_SERVICE_TOKEN_SECRET must be at least 32 bytes when INTERNAL_GRPC_ENABLED in staging/production")
		}
	}
	if appEnv == AppEnvProduction && g.ReflectionEnabled {
		return errors.New("config: INTERNAL_GRPC_REFLECTION_ENABLED must be false when APP_ENV=production")
	}
	return nil
}

func (c *Config) validateGRPCProductionReflection() error {
	if c == nil {
		return nil
	}
	if !c.GRPC.Enabled || !c.GRPC.ReflectionEnabled {
		return nil
	}
	if c.AppEnv == AppEnvProduction {
		return errors.New("config: GRPC_REFLECTION_ENABLED must be false when APP_ENV=production")
	}
	return nil
}

func (c *Config) validateGRPCProductionExposure() error {
	if c == nil || c.AppEnv != AppEnvProduction || !c.GRPC.Enabled {
		return nil
	}
	if !c.GRPC.TLS.Enabled && !c.GRPC.BehindTLSProxy {
		return errors.New("config: APP_ENV=production requires GRPC_TLS_ENABLED=true or GRPC_BEHIND_TLS_PROXY=true for machine gRPC (plaintext public exposure is not allowed)")
	}
	raw := strings.TrimSpace(c.GRPC.PublicBaseURL)
	if raw == "" {
		return errors.New("config: APP_ENV=production requires GRPC_PUBLIC_BASE_URL (e.g. grpcs://machine-api.example.com:443)")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "grpc" && u.Scheme != "grpcs") {
		return fmt.Errorf("config: GRPC_PUBLIC_BASE_URL must be a grpc:// or grpcs:// URL (got %q)", raw)
	}
	if !strings.EqualFold(u.Scheme, "grpcs") {
		return fmt.Errorf("config: APP_ENV=production requires GRPC_PUBLIC_BASE_URL to use grpcs:// (got scheme %q; TLS must be assumed for vending clients)", u.Scheme)
	}
	if strings.TrimSpace(u.Host) == "" {
		return fmt.Errorf("config: GRPC_PUBLIC_BASE_URL must include host (got %q)", raw)
	}
	return nil
}

func (c *Config) validateInternalGRPCProductionReflection() error {
	if c == nil {
		return nil
	}
	if !c.InternalGRPC.Enabled || !c.InternalGRPC.ReflectionEnabled {
		return nil
	}
	if c.AppEnv == AppEnvProduction {
		return errors.New("config: INTERNAL_GRPC_REFLECTION_ENABLED must be false when APP_ENV=production")
	}
	return nil
}

func (c *Config) validateGRPCProductionHealthReadiness() error {
	if c == nil || c.AppEnv != AppEnvProduction || !c.GRPC.Enabled {
		return nil
	}
	if !c.GRPC.HealthReflectsProcessReadiness {
		return errors.New("config: GRPC_HEALTH_USE_PROCESS_READINESS must be true when APP_ENV=production and GRPC_ENABLED=true")
	}
	return nil
}

func (c *Config) validateHTTPCORSDeploymentPolicy() error {
	if c == nil {
		return nil
	}
	switch c.AppEnv {
	case AppEnvProduction, AppEnvStaging:
		if !c.HTTP.CORSEnvPresent {
			return fmt.Errorf("config: APP_ENV=%s requires HTTP_CORS_ALLOWED_ORIGINS to be set explicitly (comma-separated origins, or an empty value to disable browser CORS middleware)", c.AppEnv)
		}
	default:
	}
	return nil
}

func (o OperationsConfig) validate() error {
	if strings.TrimSpace(o.HTTPAddr) != "" {
		if _, err := net.ResolveTCPAddr("tcp", normalizeTCPAddr(o.HTTPAddr)); err != nil {
			return fmt.Errorf("config: invalid HTTP_OPS_ADDR %q: %w", o.HTTPAddr, err)
		}
	}
	if o.ReadinessTimeout <= 0 {
		return errors.New("config: OPS_READINESS_TIMEOUT must be > 0")
	}
	if o.ShutdownTimeout <= 0 {
		return errors.New("config: OPS_SHUTDOWN_TIMEOUT must be > 0")
	}
	if o.TracerShutdownTimeout <= 0 {
		return errors.New("config: TRACER_SHUTDOWN_TIMEOUT must be > 0")
	}
	return nil
}

func (p PostgresConfig) validate() error {
	if strings.TrimSpace(p.URL) == "" {
		if p.MaxConns != 0 || p.MinConns != 0 || p.MaxConnIdleTime != 0 || p.MaxConnLifetime != 0 || len(p.overrideMaxConns()) > 0 ||
			p.SlowQueryLogThresholdMS != 0 {
			return errors.New("config: DATABASE_* pool settings require DATABASE_URL")
		}
		return nil
	}
	if p.MaxConns <= 0 {
		return errors.New("config: DATABASE_MAX_CONNS must be > 0 when DATABASE_URL is set")
	}
	const postgresMaxConnsHardCap int32 = 10000
	if p.MaxConns > postgresMaxConnsHardCap {
		return fmt.Errorf("config: DATABASE_MAX_CONNS must be <= %d (misconfiguration guard)", postgresMaxConnsHardCap)
	}
	if p.MinConns < 0 {
		return errors.New("config: DATABASE_MIN_CONNS must be >= 0")
	}
	if p.MinConns > p.MaxConns {
		return errors.New("config: DATABASE_MIN_CONNS must be <= DATABASE_MAX_CONNS")
	}
	if p.MaxConnIdleTime <= 0 {
		return errors.New("config: DATABASE_MAX_CONN_IDLE_TIME must be > 0 when DATABASE_URL is set")
	}
	if p.MaxConnLifetime <= 0 {
		return errors.New("config: DATABASE_MAX_CONN_LIFETIME must be > 0 when DATABASE_URL is set")
	}
	for envName, maxConns := range p.overrideMaxConns() {
		if maxConns <= 0 {
			return fmt.Errorf("config: %s must be > 0 when DATABASE_URL is set", envName)
		}
		if maxConns > p.MaxConns {
			return fmt.Errorf("config: %s must be <= DATABASE_MAX_CONNS (%d)", envName, p.MaxConns)
		}
		if p.MinConns > maxConns {
			return fmt.Errorf("config: DATABASE_MIN_CONNS must be <= %s", envName)
		}
	}
	if p.SlowQueryLogThresholdMS < 0 || p.SlowQueryLogThresholdMS > 600_000 {
		return errors.New("config: DATABASE_SLOW_QUERY_LOG_MS must be in [0,600000]")
	}
	return nil
}

func (r RedisConfig) validate() error {
	if strings.TrimSpace(r.Addr) == "" {
		if r.Enabled {
			return errors.New("config: REDIS_ENABLED requires REDIS_ADDR or REDIS_URL")
		}
		if strings.TrimSpace(r.Username) != "" {
			return errors.New("config: REDIS_USERNAME requires REDIS_ADDR or REDIS_URL")
		}
		if strings.TrimSpace(r.Password) != "" {
			return errors.New("config: REDIS_PASSWORD requires REDIS_ADDR or REDIS_URL")
		}
		if r.DB != 0 {
			return errors.New("config: REDIS_DB requires REDIS_ADDR or REDIS_URL")
		}
		if r.TLSEnabled || r.TLSInsecureSkipVerify {
			return errors.New("config: REDIS_TLS_* requires REDIS_ADDR or REDIS_URL")
		}
		return nil
	}
	if _, err := net.ResolveTCPAddr("tcp", normalizeTCPAddr(r.Addr)); err != nil {
		return fmt.Errorf("config: invalid REDIS_ADDR %q: %w", r.Addr, err)
	}
	if r.DB < 0 {
		return errors.New("config: REDIS_DB must be >= 0")
	}
	if strings.ContainsAny(r.KeyPrefix, " \t\r\n") {
		return errors.New("config: REDIS_KEY_PREFIX must not contain whitespace")
	}
	return nil
}

func (c *Config) validateRedisTLSInDeployedEnvironments() error {
	if c == nil {
		return nil
	}
	if c.AppEnv != AppEnvStaging && c.AppEnv != AppEnvProduction {
		return nil
	}
	if strings.TrimSpace(c.Redis.Addr) == "" {
		return nil
	}
	if c.Redis.TLSInsecureSkipVerify {
		return fmt.Errorf("config: REDIS_TLS_INSECURE_SKIP_VERIFY is not allowed when APP_ENV is %s", c.AppEnv)
	}
	return nil
}

func (n NATSConfig) validate() error {
	if strings.TrimSpace(n.URL) == "" {
		if n.Required {
			return fmt.Errorf("config: NATS_REQUIRED=true requires non-empty NATS_URL")
		}
		return nil
	}
	if _, err := url.Parse(n.URL); err != nil {
		return fmt.Errorf("config: invalid NATS_URL %q: %w", n.URL, err)
	}
	return nil
}

func (o OutboxConfig) validate() error {
	if o.MaxAttempts < 2 {
		return errors.New("config: OUTBOX_MAX_ATTEMPTS must be at least 2")
	}
	if o.BackoffMin <= 0 {
		return errors.New("config: OUTBOX_BACKOFF_MIN must be > 0")
	}
	if o.BackoffMax < o.BackoffMin {
		return errors.New("config: OUTBOX_BACKOFF_MAX must be >= OUTBOX_BACKOFF_MIN")
	}
	return nil
}

func mqttBrokerURLImpliesTLS(brokerURL string) bool {
	u := strings.TrimSpace(brokerURL)
	if u == "" {
		return false
	}
	lower := strings.ToLower(u)
	return strings.HasPrefix(lower, "ssl:") ||
		strings.HasPrefix(lower, "tls:") ||
		strings.HasPrefix(lower, "wss:") ||
		strings.HasPrefix(lower, "mqtts:") ||
		strings.HasPrefix(lower, "mqtt+ssl:") ||
		strings.HasPrefix(lower, "mqtt+tls:")
}

func mqttBrokerHostIsLoopbackLocal(host string) bool {
	switch strings.TrimSpace(strings.ToLower(host)) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func (m MQTTConfig) validate(appEnv AppEnvironment) error {
	if strings.TrimSpace(m.BrokerURL) == "" {
		if strings.TrimSpace(m.ClientID) != "" || strings.TrimSpace(m.APIClientID) != "" || strings.TrimSpace(m.IngestClientID) != "" ||
			strings.TrimSpace(m.Username) != "" || strings.TrimSpace(m.Password) != "" {
			return errors.New("config: MQTT_* credentials and client ids require MQTT_BROKER_URL")
		}
		return nil
	}
	parsedURL, err := url.Parse(strings.TrimSpace(m.BrokerURL))
	if err != nil {
		return fmt.Errorf("config: invalid MQTT_BROKER_URL %q: %w", m.BrokerURL, err)
	}
	if strings.TrimSpace(m.TopicPrefix) == "" {
		return errors.New("config: MQTT_TOPIC_PREFIX must be non-empty when MQTT_BROKER_URL is set")
	}
	if strings.TrimSpace(m.ClientID) == "" && strings.TrimSpace(m.APIClientID) == "" && strings.TrimSpace(m.IngestClientID) == "" {
		return errors.New("config: MQTT_BROKER_URL requires MQTT_CLIENT_ID and/or MQTT_CLIENT_ID_API / MQTT_CLIENT_ID_INGEST")
	}
	layout := strings.TrimSpace(strings.ToLower(m.TopicLayout))
	if layout == "" {
		layout = "legacy"
	}
	if layout != "legacy" && layout != "enterprise" {
		return fmt.Errorf("config: MQTT_TOPIC_LAYOUT must be legacy or enterprise")
	}
	if m.InsecureSkipVerify && (appEnv == AppEnvProduction || appEnv == AppEnvStaging) {
		return errors.New("config: MQTT_INSECURE_SKIP_VERIFY is not allowed when APP_ENV is staging or production")
	}
	if (strings.TrimSpace(m.CertFile) != "") != (strings.TrimSpace(m.KeyFile) != "") {
		return errors.New("config: MQTT_CERT_FILE and MQTT_KEY_FILE must both be set for mutual TLS")
	}
	brokerHostLocal := mqttBrokerHostIsLoopbackLocal(parsedURL.Hostname())
	if appEnv == AppEnvProduction && !brokerHostLocal {
		switch strings.ToLower(parsedURL.Scheme) {
		case "tcp", "mqtt":
			return errors.New("config: production MQTT_BROKER_URL must use tls:// or ssl:// when the broker host is not localhost")
		}
	}
	if appEnv == AppEnvStaging {
		if !mqttBrokerURLImpliesTLS(m.BrokerURL) && !m.TLSEnabled {
			return errors.New("config: staging MQTT requires TLS (ssl:// or tls:// MQTT_BROKER_URL, or MQTT_TLS_ENABLED=true)")
		}
	}
	if appEnv == AppEnvProduction && !brokerHostLocal {
		if !mqttBrokerURLImpliesTLS(m.BrokerURL) && !m.TLSEnabled {
			return errors.New("config: production MQTT requires TLS for non-localhost brokers (mqtts://, ssl:// or tls:// MQTT_BROKER_URL, or MQTT_TLS_ENABLED=true)")
		}
		hasUserPass := strings.TrimSpace(m.Username) != "" && strings.TrimSpace(m.Password) != ""
		hasMTLS := strings.TrimSpace(m.CertFile) != "" && strings.TrimSpace(m.KeyFile) != ""
		if !hasUserPass && !hasMTLS && !getenvBool("PRODUCTION_ALLOW_ANONYMOUS_MQTT", false) {
			return errors.New("config: production MQTT_BROKER_URL (non-localhost) requires MQTT_USERNAME and MQTT_PASSWORD, or MQTT_CERT_FILE and MQTT_KEY_FILE for mutual TLS, or set PRODUCTION_ALLOW_ANONYMOUS_MQTT=true only on a documented private broker deployment")
		}
	}
	return nil
}

func (r RuntimeConfig) validate(appEnv AppEnvironment) error {
	if strings.TrimSpace(r.PublicBaseURL) != "" {
		if _, err := url.ParseRequestURI(r.PublicBaseURL); err != nil {
			return fmt.Errorf("config: invalid PUBLIC_BASE_URL %q: %w", r.PublicBaseURL, err)
		}
	}
	if strings.TrimSpace(r.MachinePublicBaseURL) != "" {
		if _, err := url.ParseRequestURI(r.MachinePublicBaseURL); err != nil {
			return fmt.Errorf("config: invalid MACHINE_PUBLIC_BASE_URL %q: %w", r.MachinePublicBaseURL, err)
		}
	}
	if strings.TrimSpace(r.NodeName) == "" {
		return errors.New("config: APP_NODE_NAME resolved empty")
	}
	if strings.TrimSpace(r.InstanceID) == "" {
		return errors.New("config: APP_INSTANCE_ID resolved empty")
	}
	if appEnv == AppEnvProduction || appEnv == AppEnvStaging {
		if strings.TrimSpace(r.Region) == "" {
			return errors.New("config: APP_REGION (or REGION) is required in staging/production")
		}
	}
	for name, value := range map[string]string{
		"APP_REGION":      r.Region,
		"APP_NODE_NAME":   r.NodeName,
		"APP_INSTANCE_ID": r.InstanceID,
	} {
		if err := validateRuntimeIdentityPart(name, value); err != nil {
			return err
		}
	}
	return nil
}

func validateRuntimeIdentityPart(name, value string) error {
	v := strings.TrimSpace(value)
	if v == "" {
		return nil
	}
	lower := strings.ToLower(v)
	if strings.Contains(lower, "change_me") || strings.Contains(lower, "changeme") {
		return fmt.Errorf("config: %s must not contain placeholder value %q", name, value)
	}
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.', r == ':':
		default:
			return fmt.Errorf("config: %s contains invalid character %q", name, r)
		}
	}
	return nil
}

func (b BuildConfig) validate() error {
	if strings.TrimSpace(b.Version) == "" {
		return errors.New("config: APP_VERSION resolved empty")
	}
	return nil
}

func (s SMTPConfig) validate() error {
	if strings.TrimSpace(s.Host) == "" && s.Port == 0 && strings.TrimSpace(s.Username) == "" && strings.TrimSpace(s.Password) == "" {
		return nil
	}
	if strings.TrimSpace(s.Host) == "" {
		return errors.New("config: SMTP_HOST is required when SMTP settings are supplied")
	}
	if s.Port <= 0 {
		return errors.New("config: SMTP_PORT must be > 0 when SMTP_HOST is set")
	}
	return nil
}

func (t TelemetryConfig) validate() error {
	if strings.TrimSpace(t.ServiceName) == "" {
		return errors.New("config: OTEL_SERVICE_NAME is required")
	}
	if t.SDKDisabled && strings.TrimSpace(t.OTLPEndpoint) != "" {
		return errors.New("config: OTEL_EXPORTER_OTLP_ENDPOINT must be empty when OTEL_SDK_DISABLED=true")
	}
	return nil
}

func (h HTTPAuthConfig) validate() error {
	mode := strings.ToLower(strings.TrimSpace(h.Mode))
	switch mode {
	case "", "hs256", "rs256_pem", "rs256_jwks", "ed25519_pem", "jwt_jwks":
	default:
		return fmt.Errorf("config: invalid HTTP_AUTH_MODE %q", h.Mode)
	}
	if mode == "rs256_pem" && len(h.RSAPublicKeyPEM) == 0 {
		return errors.New("config: HTTP_AUTH_MODE=rs256_pem requires HTTP_AUTH_JWT_RSA_PUBLIC_KEY or HTTP_AUTH_JWT_RSA_PUBLIC_KEY_FILE")
	}
	if mode == "rs256_jwks" && strings.TrimSpace(h.JWKSURL) == "" {
		return errors.New("config: HTTP_AUTH_MODE=rs256_jwks requires HTTP_AUTH_JWT_JWKS_URL")
	}
	if mode == "ed25519_pem" && len(h.Ed25519PublicKeyPEM) == 0 {
		return errors.New("config: HTTP_AUTH_MODE=ed25519_pem requires HTTP_AUTH_JWT_ED25519_PUBLIC_KEY or HTTP_AUTH_JWT_ED25519_PUBLIC_KEY_FILE")
	}
	if mode == "jwt_jwks" && strings.TrimSpace(h.JWKSURL) == "" {
		return errors.New("config: HTTP_AUTH_MODE=jwt_jwks requires HTTP_AUTH_JWT_JWKS_URL")
	}
	if mode == "rs256_pem" || mode == "rs256_jwks" || mode == "ed25519_pem" || mode == "jwt_jwks" {
		if len(strings.TrimSpace(string(h.LoginJWTSecret))) == 0 {
			return errors.New("config: HTTP_AUTH_LOGIN_JWT_SECRET is required when HTTP_AUTH_MODE uses asymmetric access-token verification (session tokens remain HS256-signed)")
		}
	}
	if h.MFAPendingTTL <= 0 {
		return errors.New("config: ADMIN_MFA_PENDING_TTL must be > 0")
	}
	return nil
}

func (h HTTPAuthConfig) validateJWTAlgorithmCrossCheck() error {
	alg := strings.ToUpper(strings.TrimSpace(h.JWTAlgorithm))
	if alg == "" {
		return nil
	}
	mode := strings.ToLower(strings.TrimSpace(h.Mode))
	switch alg {
	case "HS256":
		if mode != "" && mode != "hs256" {
			return fmt.Errorf("config: HTTP_AUTH_JWT_ALG=HS256 conflicts with HTTP_AUTH_MODE=%s", h.Mode)
		}
	case "RS256":
		if mode != "rs256_pem" && mode != "rs256_jwks" && mode != "jwt_jwks" {
			return fmt.Errorf("config: HTTP_AUTH_JWT_ALG=RS256 requires HTTP_AUTH_MODE rs256_pem, rs256_jwks, or jwt_jwks (got %q)", h.Mode)
		}
	case "EDDSA", "ED25519":
		if mode != "ed25519_pem" && mode != "jwt_jwks" {
			return fmt.Errorf("config: HTTP_AUTH_JWT_ALG=EdDSA requires HTTP_AUTH_MODE ed25519_pem or jwt_jwks (got %q)", h.Mode)
		}
	default:
		return fmt.Errorf("config: invalid HTTP_AUTH_JWT_ALG %q (use HS256, RS256, or EdDSA)", h.JWTAlgorithm)
	}
	return nil
}

// validateDeployedJWTSecrets enforces interactive JWT signing secrets in staging and production (HS256 minimum length, login secret for asymmetric modes).
func (h HTTPAuthConfig) validateDeployedJWTSecrets(appEnv AppEnvironment) error {
	if appEnv != AppEnvProduction && appEnv != AppEnvStaging {
		return nil
	}
	mode := strings.ToLower(strings.TrimSpace(h.Mode))
	if mode == "" || mode == "hs256" {
		if len(h.JWTSecret) == 0 {
			return fmt.Errorf("config: %s requires HTTP_AUTH_JWT_SECRET when HTTP_AUTH_MODE is hs256 (default)", appEnv)
		}
		if len(bytes.TrimSpace(h.JWTSecret)) < 32 {
			return fmt.Errorf("config: %s requires HTTP_AUTH_JWT_SECRET at least 32 bytes for HS256", appEnv)
		}
		if jwtHMACSecretBytesAreTriviallyWeak(h.JWTSecret) {
			return fmt.Errorf("config: %s rejects HTTP_AUTH_JWT_SECRET that is a single repeated byte; use a high-entropy secret", appEnv)
		}
		if jwtSecretIsDocumentationPlaceholder(h.JWTSecret) {
			return jwtDocumentationPlaceholderError("HTTP_AUTH_JWT_SECRET")
		}
	}
	if mode == "rs256_pem" || mode == "rs256_jwks" || mode == "ed25519_pem" || mode == "jwt_jwks" {
		if len(strings.TrimSpace(string(h.LoginJWTSecret))) < 32 {
			return fmt.Errorf("config: %s requires HTTP_AUTH_LOGIN_JWT_SECRET at least 32 bytes when using asymmetric access-token verification", appEnv)
		}
		if jwtHMACSecretBytesAreTriviallyWeak(h.LoginJWTSecret) {
			return fmt.Errorf("config: %s rejects HTTP_AUTH_LOGIN_JWT_SECRET that is a single repeated byte; use a high-entropy secret", appEnv)
		}
		if jwtSecretIsDocumentationPlaceholder(h.LoginJWTSecret) {
			return jwtDocumentationPlaceholderError("HTTP_AUTH_LOGIN_JWT_SECRET")
		}
	}
	return nil
}

func (m MachineJWTConfig) validate() error {
	mode := strings.ToLower(strings.TrimSpace(m.Mode))
	switch mode {
	case "", "hs256", "rs256_pem", "rs256_jwks", "ed25519_pem", "jwt_jwks":
	default:
		return fmt.Errorf("config: invalid MACHINE_JWT_MODE %q", m.Mode)
	}
	if mode == "rs256_pem" && len(m.RSAPublicKeyPEM) == 0 {
		return errors.New("config: MACHINE_JWT_MODE=rs256_pem requires MACHINE_JWT_RSA_PUBLIC_KEY or MACHINE_JWT_RSA_PUBLIC_KEY_FILE")
	}
	if mode == "rs256_jwks" && strings.TrimSpace(m.JWKSURL) == "" {
		return errors.New("config: MACHINE_JWT_MODE=rs256_jwks requires MACHINE_JWT_JWKS_URL")
	}
	if mode == "ed25519_pem" && len(m.Ed25519PublicKeyPEM) == 0 {
		return errors.New("config: MACHINE_JWT_MODE=ed25519_pem requires MACHINE_JWT_ED25519_PUBLIC_KEY or MACHINE_JWT_ED25519_PUBLIC_KEY_FILE")
	}
	if mode == "jwt_jwks" && strings.TrimSpace(m.JWKSURL) == "" {
		return errors.New("config: MACHINE_JWT_MODE=jwt_jwks requires MACHINE_JWT_JWKS_URL")
	}
	return nil
}

func (m MachineJWTConfig) validateJWTAlgorithmCrossCheck() error {
	alg := strings.ToUpper(strings.TrimSpace(m.JWTAlgorithm))
	if alg == "" {
		return nil
	}
	mode := strings.ToLower(strings.TrimSpace(m.Mode))
	switch alg {
	case "HS256":
		if mode != "" && mode != "hs256" {
			return fmt.Errorf("config: MACHINE_JWT_ALG=HS256 conflicts with MACHINE_JWT_MODE=%s", m.Mode)
		}
	case "RS256":
		if mode != "rs256_pem" && mode != "rs256_jwks" && mode != "jwt_jwks" {
			return fmt.Errorf("config: MACHINE_JWT_ALG=RS256 requires MACHINE_JWT_MODE rs256_pem, rs256_jwks, or jwt_jwks (got %q)", m.Mode)
		}
	case "EDDSA", "ED25519":
		if mode != "ed25519_pem" && mode != "jwt_jwks" {
			return fmt.Errorf("config: MACHINE_JWT_ALG=EdDSA requires MACHINE_JWT_MODE ed25519_pem or jwt_jwks (got %q)", m.Mode)
		}
	default:
		return fmt.Errorf("config: invalid MACHINE_JWT_ALG %q (use HS256, RS256, or EdDSA)", m.JWTAlgorithm)
	}
	return nil
}

func (m MachineJWTConfig) validateDeployedJWTSecrets(appEnv AppEnvironment) error {
	if appEnv != AppEnvProduction && appEnv != AppEnvStaging {
		return nil
	}
	mode := strings.ToLower(strings.TrimSpace(m.Mode))
	if mode == "" || mode == "hs256" {
		if len(bytes.TrimSpace(m.JWTSecret)) == 0 && len(m.AdditionalHS256Secrets) == 0 {
			return fmt.Errorf("config: %s requires MACHINE_JWT_SECRET when MACHINE_JWT_MODE is hs256", appEnv)
		}
		if len(bytes.TrimSpace(m.JWTSecret)) > 0 && len(bytes.TrimSpace(m.JWTSecret)) < 32 {
			return fmt.Errorf("config: %s requires MACHINE_JWT_SECRET at least 32 bytes for HS256", appEnv)
		}
		if len(bytes.TrimSpace(m.JWTSecret)) > 0 && jwtHMACSecretBytesAreTriviallyWeak(m.JWTSecret) {
			return fmt.Errorf("config: %s rejects MACHINE_JWT_SECRET that is a single repeated byte; use a high-entropy secret", appEnv)
		}
		if len(bytes.TrimSpace(m.JWTSecret)) > 0 && jwtSecretIsDocumentationPlaceholder(m.JWTSecret) {
			return jwtDocumentationPlaceholderError("MACHINE_JWT_SECRET")
		}
	}
	return nil
}

func (h HTTPRateLimitConfig) validate() error {
	if h.SensitiveWritesEnabled {
		if h.SensitiveWritesRPS <= 0 {
			return errors.New("config: HTTP_RATE_LIMIT_SENSITIVE_WRITES_RPS must be > 0 when rate limiting is enabled")
		}
		if h.SensitiveWritesBurst <= 0 {
			return errors.New("config: HTTP_RATE_LIMIT_SENSITIVE_WRITES_BURST must be > 0 when rate limiting is enabled")
		}
	}
	return h.Abuse.validate()
}

func (a AbuseRateLimitConfig) validate() error {
	if !a.Enabled {
		return nil
	}
	if a.LoginPerMinute <= 0 {
		return errors.New("config: RATE_LIMIT_LOGIN_PER_MIN must be > 0 when RATE_LIMIT_ENABLED=true")
	}
	if a.RefreshPerMinute <= 0 {
		return errors.New("config: RATE_LIMIT_REFRESH_PER_MIN must be > 0 when RATE_LIMIT_ENABLED=true")
	}
	if a.AdminMutationPerMinute <= 0 {
		return errors.New("config: RATE_LIMIT_ADMIN_MUTATION_PER_MIN must be > 0 when RATE_LIMIT_ENABLED=true")
	}
	if a.MachinePerMinute <= 0 {
		return errors.New("config: RATE_LIMIT_MACHINE_PER_MIN must be > 0 when RATE_LIMIT_ENABLED=true")
	}
	if a.WebhookPerMinute <= 0 {
		return errors.New("config: RATE_LIMIT_WEBHOOK_PER_MIN must be > 0 when RATE_LIMIT_ENABLED=true")
	}
	if a.PublicPerMinute <= 0 {
		return errors.New("config: RATE_LIMIT_PUBLIC_PER_MIN must be > 0 when RATE_LIMIT_ENABLED=true")
	}
	if a.CommandDispatchPerMinute <= 0 {
		return errors.New("config: RATE_LIMIT_COMMAND_DISPATCH_PER_MIN must be > 0 when RATE_LIMIT_ENABLED=true")
	}
	if a.ReportsReadPerMinute <= 0 {
		return errors.New("config: RATE_LIMIT_REPORTS_READ_PER_MIN must be > 0 when RATE_LIMIT_ENABLED=true")
	}
	if a.LockoutWindow <= 0 {
		return errors.New("config: RATE_LIMIT_LOCKOUT_WINDOW must be > 0 when RATE_LIMIT_ENABLED=true")
	}
	return nil
}

func (c *Config) validateMetricsHTTPExposure() error {
	if !c.MetricsEnabled {
		return nil
	}
	if c.AppEnv == AppEnvProduction && c.MetricsExposeOnPublicHTTP {
		tok := strings.TrimSpace(c.MetricsScrapeToken)
		if len(tok) < 16 {
			return errors.New("config: METRICS_SCRAPE_TOKEN must be set (min 16 chars) when APP_ENV=production and METRICS_EXPOSE_ON_PUBLIC_HTTP=true")
		}
		if !getenvBool("PRODUCTION_PUBLIC_METRICS_ENDPOINT_ALLOWED", false) {
			return errors.New("config: METRICS_EXPOSE_ON_PUBLIC_HTTP=true in production requires PRODUCTION_PUBLIC_METRICS_ENDPOINT_ALLOWED=true (documented operator approval) in addition to METRICS_SCRAPE_TOKEN")
		}
	}
	return nil
}

func (c CommerceHTTPConfig) validate(appEnv AppEnvironment) error {
	mode := strings.ToLower(strings.TrimSpace(c.PaymentWebhookVerification))
	if mode == "" {
		mode = "avf_hmac"
	}
	if mode != "avf_hmac" {
		return fmt.Errorf("config: COMMERCE_PAYMENT_WEBHOOK_VERIFICATION %q is unsupported (only avf_hmac)", strings.TrimSpace(c.PaymentWebhookVerification))
	}
	sec := int(c.PaymentWebhookTimestampSkew / time.Second)
	if sec < 30 || sec > 86400 {
		return fmt.Errorf("config: COMMERCE_PAYMENT_WEBHOOK_REPLAY_WINDOW (or TIMESTAMP_SKEW alias) must be between 30 and 86400 seconds (effective %ds)", sec)
	}
	if c.PaymentWebhookAllowUnsigned && !appEnv.AllowsUnsignedCommerceWebhooksInDevelopment() {
		return errors.New("config: COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED is only allowed when APP_ENV is development or test")
	}
	hasWebhookSecretMaterial := strings.TrimSpace(c.PaymentWebhookHMACSecret) != "" ||
		(len(c.PaymentWebhookProviderSecrets) > 0)
	if appEnv.RestrictsUnsignedCommercePaymentWebhooks() &&
		!hasWebhookSecretMaterial &&
		!c.PaymentWebhookUnsafeAllowUnsignedProduction {
		return errors.New("config: COMMERCE_PAYMENT_WEBHOOK_SECRET and/or COMMERCE_PAYMENT_WEBHOOK_SECRETS_JSON is required when APP_ENV is staging or production unless COMMERCE_PAYMENT_WEBHOOK_UNSAFE_ALLOW_UNSIGNED_PRODUCTION=true")
	}
	if c.MachineOrderCheckoutMaxAge > 0 {
		if c.MachineOrderCheckoutMaxAge < time.Minute || c.MachineOrderCheckoutMaxAge > 7*24*time.Hour {
			return fmt.Errorf("config: COMMERCE_MACHINE_ORDER_CHECKOUT_MAX_AGE must be between 1m and 7d (got %s)", c.MachineOrderCheckoutMaxAge)
		}
	}
	if appEnv == AppEnvProduction || appEnv == AppEnvStaging {
		if c.PaymentWebhookUnsafeAllowUnsignedProduction {
			return errors.New("config: COMMERCE_PAYMENT_WEBHOOK_UNSAFE_ALLOW_UNSIGNED_PRODUCTION is forbidden when APP_ENV is staging or production")
		}
	}
	if appEnv == AppEnvProduction && c.PaymentWebhookAllowUnsigned {
		return errors.New("config: COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED must be false when APP_ENV=production")
	}
	return nil
}

func metricsExposeOnPublicHTTPFromEnv(appEnv AppEnvironment) (bool, error) {
	raw, ok := os.LookupEnv("METRICS_EXPOSE_ON_PUBLIC_HTTP")
	if !ok || strings.TrimSpace(raw) == "" {
		return appEnv != AppEnvProduction, nil
	}
	v, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false, fmt.Errorf("config: invalid METRICS_EXPOSE_ON_PUBLIC_HTTP: %w", err)
	}
	return v, nil
}

func (a ArtifactsConfig) validate(appEnv AppEnvironment) error {
	if !a.Enabled {
		return nil
	}
	if a.MaxUploadBytes <= 0 {
		return errors.New("config: ARTIFACTS_MAX_UPLOAD_BYTES must be > 0 when API_ARTIFACTS_ENABLED=true")
	}
	const hardMax = 5 << 30
	if a.MaxUploadBytes > hardMax {
		return fmt.Errorf("config: ARTIFACTS_MAX_UPLOAD_BYTES exceeds hard cap (%d bytes)", hardMax)
	}
	if a.DownloadPresignTTL <= 0 {
		return errors.New("config: ARTIFACTS_DOWNLOAD_PRESIGN_TTL must be > 0 when API_ARTIFACTS_ENABLED=true")
	}
	if a.ListMaxKeys < 0 || a.ListMaxKeys > 1000 {
		return errors.New("config: ARTIFACTS_LIST_MAX_KEYS must be between 0 and 1000 (0 uses service default)")
	}
	if appEnv == AppEnvProduction {
		if strings.TrimSpace(a.Bucket) == "" {
			return errors.New("config: OBJECT_STORAGE_BUCKET is required when APP_ENV=production and API_ARTIFACTS_ENABLED=true (or OBJECT_STORAGE_ENABLED=true)")
		}
		if strings.TrimSpace(a.PublicBaseURL) == "" {
			return errors.New("config: OBJECT_STORAGE_PUBLIC_BASE_URL is required when APP_ENV=production and API_ARTIFACTS_ENABLED=true (or OBJECT_STORAGE_ENABLED=true)")
		}
	}
	return nil
}

// loadSwaggerUIEnabled returns whether to mount Swagger. Explicit HTTP_SWAGGER_UI_ENABLED wins (true/1 only);
// when the variable is unset, production stays off and other APP_ENV values default on.
func loadSwaggerUIEnabled() bool {
	if _, ok := os.LookupEnv("HTTP_SWAGGER_UI_ENABLED"); ok {
		return strings.EqualFold(strings.TrimSpace(os.Getenv("HTTP_SWAGGER_UI_ENABLED")), "true") ||
			strings.TrimSpace(os.Getenv("HTTP_SWAGGER_UI_ENABLED")) == "1"
	}
	app := strings.ToLower(strings.TrimSpace(getenv("APP_ENV", string(AppEnvDevelopment))))
	return app != string(AppEnvProduction)
}

// loadOpenAPIJSONEnabled controls GET /swagger/doc.json (OpenAPI 3.0 JSON). Explicit HTTP_OPENAPI_JSON_ENABLED
// wins (true/1 only). When unset, production defaults off (fail-closed); other APP_ENV values default on for DX.
func loadOpenAPIJSONEnabled() bool {
	if _, ok := os.LookupEnv("HTTP_OPENAPI_JSON_ENABLED"); ok {
		return strings.EqualFold(strings.TrimSpace(os.Getenv("HTTP_OPENAPI_JSON_ENABLED")), "true") ||
			strings.TrimSpace(os.Getenv("HTTP_OPENAPI_JSON_ENABLED")) == "1"
	}
	app := strings.ToLower(strings.TrimSpace(getenv("APP_ENV", string(AppEnvDevelopment))))
	return app != string(AppEnvProduction)
}

func (c *Config) validateSwaggerAndOpenAPI() error {
	if c.SwaggerUIEnabled && !c.OpenAPIJSONEnabled {
		return errors.New("config: HTTP_SWAGGER_UI_ENABLED=true requires HTTP_OPENAPI_JSON_ENABLED=true (Swagger UI loads /swagger/doc.json)")
	}
	return nil
}

func loadPaymentEnv() string {
	return strings.ToLower(strings.TrimSpace(os.Getenv("PAYMENT_ENV")))
}

func loadPaymentWebhookProviderSecrets() (map[string]string, error) {
	raw := strings.TrimSpace(os.Getenv("COMMERCE_PAYMENT_WEBHOOK_SECRETS_JSON"))
	if raw == "" {
		return nil, nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, fmt.Errorf("config: COMMERCE_PAYMENT_WEBHOOK_SECRETS_JSON must be a JSON object of string secrets: %w", err)
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		kk := strings.ToLower(strings.TrimSpace(k))
		if kk == "" || strings.TrimSpace(v) == "" {
			continue
		}
		out[kk] = strings.TrimSpace(v)
	}
	return out, nil
}

func loadCORSConfigFromEnv() (origins []string, present bool, allowCreds bool) {
	raw, ok := os.LookupEnv("HTTP_CORS_ALLOWED_ORIGINS")
	if !ok {
		return nil, false, getenvBool("HTTP_CORS_ALLOW_CREDENTIALS", false)
	}
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		origins = append(origins, p)
	}
	return origins, true, getenvBool("HTTP_CORS_ALLOW_CREDENTIALS", false)
}

func loadGRPCConfig(appEnv AppEnvironment) GRPCConfig {
	reflectionDefault := appEnv == AppEnvDevelopment || appEnv == AppEnvTest
	healthReadinessDefault := appEnv == AppEnvProduction
	prefix := strings.TrimSpace(getenv("GRPC_MTLS_MACHINE_ID_URI_PREFIX", "urn:avf:machine:"))
	if prefix == "" {
		prefix = "urn:avf:machine:"
	}
	grpcEnabled := getenvBool("MACHINE_GRPC_ENABLED", getenvBool("GRPC_ENABLED", false))
	return GRPCConfig{
		Enabled:                        grpcEnabled,
		Addr:                           strings.TrimSpace(getenv("GRPC_ADDR", ":9090")),
		ShutdownTimeout:                mustParseDuration("GRPC_SHUTDOWN_TIMEOUT", getenv("GRPC_SHUTDOWN_TIMEOUT", "15s")),
		ReflectionEnabled:              getenvBool("GRPC_REFLECTION_ENABLED", reflectionDefault),
		HealthEnabled:                  getenvBool("GRPC_HEALTH_ENABLED", true),
		HealthReflectsProcessReadiness: getenvBool("GRPC_HEALTH_USE_PROCESS_READINESS", healthReadinessDefault),
		RequireMachineJWT:              getenvBool("GRPC_REQUIRE_MACHINE_JWT", getenvBool("GRPC_REQUIRE_MACHINE_AUTH", true)),
		RequireGRPCIdempotency:         getenvBool("GRPC_REQUIRE_IDEMPOTENCY", true),
		UnaryHandlerTimeout:            mustParseDuration("GRPC_UNARY_HANDLER_TIMEOUT", getenv("GRPC_UNARY_HANDLER_TIMEOUT", "60s")),
		PublicBaseURL:                  strings.TrimSpace(getenv("GRPC_PUBLIC_BASE_URL", "")),
		BehindTLSProxy:                 getenvBool("GRPC_BEHIND_TLS_PROXY", false),
		MaxRecvMsgSize:                 getenvInt("GRPC_MAX_RECV_MSG_SIZE", 0),
		MaxSendMsgSize:                 getenvInt("GRPC_MAX_SEND_MSG_SIZE", 0),
		TLS: GRPCServerTLSConfig{
			Enabled:                    getenvBool("GRPC_TLS_ENABLED", false),
			CertFile:                   strings.TrimSpace(getenv("GRPC_TLS_CERT_FILE", "")),
			KeyFile:                    strings.TrimSpace(getenv("GRPC_TLS_KEY_FILE", "")),
			ClientCAFile:               strings.TrimSpace(getenv("GRPC_TLS_CLIENT_CA_FILE", "")),
			ClientAuth:                 strings.TrimSpace(getenv("GRPC_TLS_CLIENT_AUTH", "no")),
			MachineIDFromCertURIPrefix: prefix,
			AllowMachineAuthCertOnly:   getenvBool("GRPC_MACHINE_AUTH_CERT_ONLY_ALLOWED", false),
		},
	}
}

func loadTransportBoundary(appEnv AppEnvironment) TransportBoundaryConfig {
	defaultLegacyREST := appEnv != AppEnvProduction
	legacyEnabled := defaultLegacyREST
	if _, ok := os.LookupEnv("ENABLE_LEGACY_MACHINE_HTTP"); ok {
		legacyEnabled = getenvBool("ENABLE_LEGACY_MACHINE_HTTP", false)
	} else if _, ok := os.LookupEnv("MACHINE_REST_LEGACY_ENABLED"); ok {
		legacyEnabled = getenvBool("MACHINE_REST_LEGACY_ENABLED", false)
	}
	tr := strings.TrimSpace(getenv("MQTT_COMMAND_TRANSPORT", "mqtt"))
	if tr == "" {
		tr = "mqtt"
	}
	return TransportBoundaryConfig{
		MachineRESTLegacyEnabled:           legacyEnabled,
		MachineRESTLegacyAllowInProduction: getenvBool("MACHINE_REST_LEGACY_ALLOW_IN_PRODUCTION", false),
		MQTTCommandTransport:               tr,
	}
}

func loadInternalGRPCConfig(appEnv AppEnvironment) InternalGRPCConfig {
	reflectionDefault := appEnv == AppEnvDevelopment || appEnv == AppEnvTest
	return InternalGRPCConfig{
		Enabled:             getenvBool("INTERNAL_GRPC_ENABLED", false),
		Addr:                strings.TrimSpace(getenv("INTERNAL_GRPC_ADDR", "127.0.0.1:9091")),
		ShutdownTimeout:     mustParseDuration("INTERNAL_GRPC_SHUTDOWN_TIMEOUT", getenv("INTERNAL_GRPC_SHUTDOWN_TIMEOUT", "15s")),
		ServiceTokenSecret:  []byte(strings.TrimSpace(getenvAlias("", "SERVICE_JWT_SECRET", "INTERNAL_GRPC_SERVICE_TOKEN_SECRET"))),
		ReflectionEnabled:   getenvBool("INTERNAL_GRPC_REFLECTION_ENABLED", reflectionDefault),
		HealthEnabled:       getenvBool("INTERNAL_GRPC_HEALTH_ENABLED", true),
		UnaryHandlerTimeout: mustParseDuration("INTERNAL_GRPC_UNARY_HANDLER_TIMEOUT", getenv("INTERNAL_GRPC_UNARY_HANDLER_TIMEOUT", "60s")),
	}
}

// Load reads configuration from the environment and validates it.
func Load() (*Config, error) {
	httpAuth, err := loadHTTPAuthConfig()
	if err != nil {
		return nil, err
	}
	if raw := strings.TrimSpace(os.Getenv("ADMIN_SESSION_TTL")); raw != "" {
		httpAuth.AccessTokenTTL = mustParseDuration("ADMIN_SESSION_TTL", raw)
	}
	if raw := strings.TrimSpace(os.Getenv("ADMIN_REFRESH_TTL")); raw != "" {
		httpAuth.RefreshTokenTTL = mustParseDuration("ADMIN_REFRESH_TTL", raw)
	}
	machineJWT, err := loadMachineJWTConfig(httpAuth)
	if err != nil {
		return nil, err
	}
	hostname, _ := os.Hostname()
	appEnv := AppEnvironment(strings.TrimSpace(getenv("APP_ENV", string(AppEnvDevelopment))))
	adminAuthSecurity, err := loadAdminAuthSecurityConfig(appEnv)
	if err != nil {
		return nil, err
	}
	postgresCfg, err := loadPostgresConfig(appEnv)
	if err != nil {
		return nil, err
	}
	metricsExposePublic, err := metricsExposeOnPublicHTTPFromEnv(appEnv)
	if err != nil {
		return nil, err
	}

	webhookProvSecrets, err := loadPaymentWebhookProviderSecrets()
	if err != nil {
		return nil, err
	}
	platformAuditOrgID, err := parseOptionalEnvUUID("PLATFORM_AUDIT_ORGANIZATION_ID")
	if err != nil {
		return nil, err
	}
	deployedEnv := appEnv == AppEnvStaging || appEnv == AppEnvProduction
	natsRequired := getenvBoolAlias(deployedEnv, "NATS_REQUIRED", "API_REQUIRE_NATS_RUNTIME")
	outboxRequired := getenvBoolAlias(deployedEnv, "OUTBOX_PUBLISHER_REQUIRED", "API_REQUIRE_OUTBOX_PUBLISHER")

	cfg := &Config{
		AppEnv:                    appEnv,
		RedisRuntime:              loadRedisRuntimeFeatures(appEnv),
		PaymentEnv:                loadPaymentEnv(),
		LogLevel:                  strings.TrimSpace(getenv("LOG_LEVEL", "info")),
		LogFormat:                 strings.TrimSpace(getenv("LOG_FORMAT", "json")),
		ReadinessStrict:           getenvBool("READINESS_STRICT", false),
		MetricsEnabled:            getenvBool("METRICS_ENABLED", false),
		MetricsExposeOnPublicHTTP: metricsExposePublic,
		MetricsScrapeToken:        strings.TrimSpace(os.Getenv("METRICS_SCRAPE_TOKEN")),
		SwaggerUIEnabled:          loadSwaggerUIEnabled(),
		OpenAPIJSONEnabled:        loadOpenAPIJSONEnabled(),
		Runtime: RuntimeConfig{
			PublicBaseURL:        firstNonEmptyTrimmed(os.Getenv("APP_BASE_URL"), os.Getenv("PUBLIC_BASE_URL")),
			MachinePublicBaseURL: firstNonEmptyTrimmed(os.Getenv("MACHINE_PUBLIC_BASE_URL"), os.Getenv("APP_BASE_URL"), os.Getenv("PUBLIC_BASE_URL")),
			Region:               firstNonEmptyTrimmed(os.Getenv("APP_REGION"), os.Getenv("REGION")),
			NodeName:             strings.TrimSpace(getenv("APP_NODE_NAME", hostname)),
			InstanceID:           strings.TrimSpace(getenv("APP_INSTANCE_ID", hostname)),
			RuntimeRole:          strings.TrimSpace(getenv("APP_RUNTIME_ROLE", "")),
		},
		Build: BuildConfig{
			Version:   strings.TrimSpace(getenv("APP_VERSION", version.Version)),
			GitSHA:    strings.TrimSpace(getenv("APP_GIT_SHA", version.Commit)),
			BuildTime: strings.TrimSpace(getenv("APP_BUILD_TIME", version.BuildTime)),
		},
		WorkerMetricsListen:         strings.TrimSpace(getenv("WORKER_METRICS_LISTEN", "")),
		ReconcilerMetricsListen:     strings.TrimSpace(getenv("RECONCILER_METRICS_LISTEN", "")),
		MQTTIngestMetricsListen:     strings.TrimSpace(getenv("MQTT_INGEST_METRICS_LISTEN", "")),
		TemporalWorkerMetricsListen: strings.TrimSpace(getenv("TEMPORAL_WORKER_METRICS_LISTEN", "")),
		WorkerOutboxOnly:            getenvBool("WORKER_OUTBOX_ONLY", false),
		WorkerOutboxLockTTLSeconds:  getenvInt("WORKER_OUTBOX_LOCK_TTL_SECONDS", 45),
		WorkerOutboxDisableLease:    getenvBool("WORKER_OUTBOX_DISABLE_LEASE", false),
		APIWiring: APIWiringRequirements{
			RequireAuthAdapter:             getenvBool("API_REQUIRE_AUTH_ADAPTER", false),
			RequireOutboxPublisher:         outboxRequired,
			RequireMQTTPublisher:           getenvBool("API_REQUIRE_MQTT_PUBLISHER", false),
			RequireNATSRuntime:             natsRequired,
			RequirePaymentProviderRegistry: getenvBool("API_REQUIRE_PAYMENT_PROVIDER_REGISTRY", false),
		},
		Commerce: CommerceHTTPConfig{
			PaymentOutboxTopic:         strings.TrimSpace(getenv("COMMERCE_PAYMENT_OUTBOX_TOPIC", "commerce.payments")),
			PaymentOutboxEventType:     strings.TrimSpace(getenv("COMMERCE_PAYMENT_OUTBOX_EVENT_TYPE", "payment.session_started")),
			PaymentOutboxAggregateType: strings.TrimSpace(getenv("COMMERCE_PAYMENT_OUTBOX_AGGREGATE_TYPE", "payment")),
			PaymentWebhookHMACSecret: firstNonEmptyTrimmed(
				os.Getenv("COMMERCE_PAYMENT_WEBHOOK_SECRET"),
				os.Getenv("PAYMENT_WEBHOOK_SECRET"),
				os.Getenv("COMMERCE_PAYMENT_WEBHOOK_HMAC_SECRET"),
			),
			PaymentWebhookVerification:                  strings.TrimSpace(getenv("COMMERCE_PAYMENT_WEBHOOK_VERIFICATION", "avf_hmac")),
			PaymentWebhookTimestampSkew:                 paymentWebhookReplayWindowDuration(),
			PaymentWebhookAllowUnsigned:                 getenvBool("COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED", false),
			PaymentWebhookUnsafeAllowUnsignedProduction: getenvBool("COMMERCE_PAYMENT_WEBHOOK_UNSAFE_ALLOW_UNSIGNED_PRODUCTION", false),
			PaymentWebhookProviderSecrets:               webhookProvSecrets,
			DefaultPaymentProvider:                      strings.ToLower(strings.TrimSpace(getenv("COMMERCE_PAYMENT_PROVIDER", ""))),
			MachineOrderCheckoutMaxAge:                  mustParseDuration("COMMERCE_MACHINE_ORDER_CHECKOUT_MAX_AGE", getenv("COMMERCE_MACHINE_ORDER_CHECKOUT_MAX_AGE", "30m")),
		},
		CashSettlement: CashSettlementConfig{
			VarianceReviewThresholdMinor: getenvInt64("CASH_SETTLEMENT_VARIANCE_REVIEW_THRESHOLD_MINOR", 500),
		},
		Reconciler: loadReconcilerConfig(),
		Temporal:   loadTemporalConfig(),
		HTTP: func() HTTPConfig {
			corsOrigins, corsPresent, corsCreds := loadCORSConfigFromEnv()
			return HTTPConfig{
				Addr:                 strings.TrimSpace(getenv("HTTP_ADDR", ":8080")),
				ShutdownTimeout:      mustParseDuration("HTTP_SHUTDOWN_TIMEOUT", getenv("HTTP_SHUTDOWN_TIMEOUT", "15s")),
				ReadHeaderTimeout:    mustParseDuration("HTTP_READ_HEADER_TIMEOUT", getenv("HTTP_READ_HEADER_TIMEOUT", "5s")),
				ReadTimeout:          mustParseDuration("HTTP_READ_TIMEOUT", getenv("HTTP_READ_TIMEOUT", "30s")),
				WriteTimeout:         mustParseDuration("HTTP_WRITE_TIMEOUT", getenv("HTTP_WRITE_TIMEOUT", "30s")),
				IdleTimeout:          mustParseDuration("HTTP_IDLE_TIMEOUT", getenv("HTTP_IDLE_TIMEOUT", "60s")),
				CORSAllowedOrigins:   corsOrigins,
				CORSEnvPresent:       corsPresent,
				CORSAllowCredentials: corsCreds,
			}
		}(),
		GRPC:              loadGRPCConfig(appEnv),
		TransportBoundary: loadTransportBoundary(appEnv),
		InternalGRPC:      loadInternalGRPCConfig(appEnv),
		Ops: OperationsConfig{
			HTTPAddr:              strings.TrimSpace(getenv("HTTP_OPS_ADDR", "")),
			ReadinessTimeout:      mustParseDuration("OPS_READINESS_TIMEOUT", getenv("OPS_READINESS_TIMEOUT", "2s")),
			ShutdownTimeout:       mustParseDuration("OPS_SHUTDOWN_TIMEOUT", getenv("OPS_SHUTDOWN_TIMEOUT", "5s")),
			TracerShutdownTimeout: mustParseDuration("TRACER_SHUTDOWN_TIMEOUT", getenv("TRACER_SHUTDOWN_TIMEOUT", "10s")),
		},
		Postgres: postgresCfg,
		NATS: NATSConfig{
			URL:      strings.TrimSpace(getenv("NATS_URL", "")),
			Required: natsRequired,
		},
		Outbox: OutboxConfig{
			PublisherRequired: outboxRequired,
			MaxAttempts:       getenvInt("OUTBOX_MAX_ATTEMPTS", 24),
			BackoffMin:        mustParseDuration("OUTBOX_BACKOFF_MIN", getenv("OUTBOX_BACKOFF_MIN", "1s")),
			BackoffMax:        mustParseDuration("OUTBOX_BACKOFF_MAX", getenv("OUTBOX_BACKOFF_MAX", "5m")),
			DLQEnabled:        getenvBool("OUTBOX_DLQ_ENABLED", true),
		},
		MQTT: loadMQTTConfig(),
		Telemetry: TelemetryConfig{
			ServiceName:  strings.TrimSpace(getenv("OTEL_SERVICE_NAME", "avf-vending-api")),
			OTLPEndpoint: strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")),
			Insecure:     getenvBool("OTEL_INSECURE", true),
			SDKDisabled:  getenvBool("OTEL_SDK_DISABLED", false),
		},
		MQTTDeviceTelemetry:            loadMQTTDeviceTelemetryConfig(),
		TelemetryJetStream:             loadTelemetryJetStreamConfig(),
		TelemetryDataRetention:         loadTelemetryDataRetentionConfig(appEnv),
		EnterpriseRetention:            loadEnterpriseRetentionConfig(),
		RetentionWorker:                loadRetentionWorkerConfig(appEnv),
		RetentionAllowDestructiveLocal: getenvBool("RETENTION_ALLOW_DESTRUCTIVE_LOCAL", false),
		AuditCriticalFailOpen:          getenvBool("AUDIT_CRITICAL_FAIL_OPEN", false),
		PlatformAuditOrganizationID:    platformAuditOrgID,
		HTTPAuth:                       httpAuth,
		AdminAuthSecurity:              adminAuthSecurity,
		MachineJWT:                     machineJWT,
		HTTPRateLimit:                  loadHTTPRateLimitConfig(),
		Capacity:                       loadCapacityLimitsConfig(),
		Artifacts:                      loadArtifactsConfig(),
		Analytics:                      loadAnalyticsConfig(),
		SMTP:                           loadSMTPConfig(),
	}

	cfg.Redis, err = loadRedisConfig()
	if err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func loadHTTPAuthConfig() (HTTPAuthConfig, error) {
	mode := strings.TrimSpace(getenvAlias("hs256", "USER_JWT_MODE", "HTTP_AUTH_MODE"))
	leeway := mustParseDuration("USER_JWT_LEEWAY/HTTP_AUTH_JWT_LEEWAY", getenvAlias("45s", "USER_JWT_LEEWAY", "HTTP_AUTH_JWT_LEEWAY"))

	var rsaPEM []byte
	if fp := strings.TrimSpace(getenvAlias("", "USER_JWT_RSA_PUBLIC_KEY_FILE", "HTTP_AUTH_JWT_RSA_PUBLIC_KEY_FILE")); fp != "" {
		b, err := os.ReadFile(fp)
		if err != nil {
			return HTTPAuthConfig{}, fmt.Errorf("config: USER_JWT_RSA_PUBLIC_KEY_FILE/HTTP_AUTH_JWT_RSA_PUBLIC_KEY_FILE: %w", err)
		}
		rsaPEM = b
	} else if s := strings.TrimSpace(getenvAlias("", "USER_JWT_RSA_PUBLIC_KEY", "HTTP_AUTH_JWT_RSA_PUBLIC_KEY")); s != "" {
		rsaPEM = []byte(s)
	}

	var ed25519PEM []byte
	if fp := strings.TrimSpace(getenvAlias("", "USER_JWT_ED25519_PUBLIC_KEY_FILE", "HTTP_AUTH_JWT_ED25519_PUBLIC_KEY_FILE")); fp != "" {
		b, err := os.ReadFile(fp)
		if err != nil {
			return HTTPAuthConfig{}, fmt.Errorf("config: USER_JWT_ED25519_PUBLIC_KEY_FILE/HTTP_AUTH_JWT_ED25519_PUBLIC_KEY_FILE: %w", err)
		}
		ed25519PEM = b
	} else if s := strings.TrimSpace(getenvAlias("", "USER_JWT_ED25519_PUBLIC_KEY", "HTTP_AUTH_JWT_ED25519_PUBLIC_KEY")); s != "" {
		ed25519PEM = []byte(s)
	}

	jwksTTL := mustParseDuration("USER_JWT_JWKS_CACHE_TTL/HTTP_AUTH_JWT_JWKS_CACHE_TTL", getenvAlias("5m", "USER_JWT_JWKS_CACHE_TTL", "HTTP_AUTH_JWT_JWKS_CACHE_TTL"))
	accessTTL := mustParseDuration("USER_JWT_ACCESS_TTL/HTTP_AUTH_ACCESS_TTL", getenvAlias("15m", "USER_JWT_ACCESS_TTL", "HTTP_AUTH_ACCESS_TTL"))
	refreshTTL := mustParseDuration("USER_JWT_REFRESH_TTL/HTTP_AUTH_REFRESH_TTL", getenvAlias("720h", "USER_JWT_REFRESH_TTL", "HTTP_AUTH_REFRESH_TTL"))
	mfaPendingTTL := mustParseDuration("ADMIN_MFA_PENDING_TTL", getenv("ADMIN_MFA_PENDING_TTL", "5m"))

	return HTTPAuthConfig{
		Mode:                mode,
		JWTAlgorithm:        strings.TrimSpace(getenvAlias("", "USER_JWT_ALG", "HTTP_AUTH_JWT_ALG")),
		JWTLeeway:           leeway,
		JWTSecret:           []byte(strings.TrimSpace(getenvAlias("", "USER_JWT_SECRET", "HTTP_AUTH_JWT_SECRET"))),
		JWTSecretPrevious:   []byte(strings.TrimSpace(getenvAlias("", "USER_JWT_SECRET_PREVIOUS", "HTTP_AUTH_JWT_SECRET_PREVIOUS"))),
		LoginJWTSecret:      []byte(strings.TrimSpace(getenvAlias("", "USER_JWT_LOGIN_SECRET", "HTTP_AUTH_LOGIN_JWT_SECRET"))),
		AccessTokenTTL:      accessTTL,
		RefreshTokenTTL:     refreshTTL,
		MFAPendingTTL:       mfaPendingTTL,
		RSAPublicKeyPEM:     rsaPEM,
		Ed25519PublicKeyPEM: ed25519PEM,
		JWKSURL:             strings.TrimSpace(getenvAlias("", "USER_JWT_JWKS_URL", "HTTP_AUTH_JWT_JWKS_URL")),
		JWKSCacheTTL:        jwksTTL,
		JWKSSkipStartupWarm: getenvBoolAlias(false, "USER_JWT_JWKS_SKIP_STARTUP_WARM", "HTTP_AUTH_JWT_JWKS_SKIP_STARTUP_WARM"),
		ExpectedIssuer:      strings.TrimSpace(getenvAlias("", "AUTH_ISSUER", "USER_JWT_ISSUER", "HTTP_AUTH_JWT_ISSUER")),
		ExpectedAudience:    strings.TrimSpace(getenvAlias("", "AUTH_ADMIN_AUDIENCE", "USER_JWT_AUDIENCE", "HTTP_AUTH_JWT_AUDIENCE")),
	}, nil
}

func loadMachineJWTConfig(httpAuth HTTPAuthConfig) (MachineJWTConfig, error) {
	appEnv := AppEnvironment(strings.TrimSpace(getenv("APP_ENV", string(AppEnvDevelopment))))
	mode := strings.TrimSpace(getenvAlias("hs256", "MACHINE_JWT_MODE"))
	leeway := httpAuth.JWTLeeway
	if raw := strings.TrimSpace(getenvAlias("", "MACHINE_TOKEN_CLOCK_SKEW", "MACHINE_JWT_LEEWAY")); raw != "" {
		leeway = mustParseDuration("MACHINE_TOKEN_CLOCK_SKEW/MACHINE_JWT_LEEWAY", raw)
	}
	var rsaPEM []byte
	if fp := strings.TrimSpace(getenvAlias("", "MACHINE_JWT_RSA_PUBLIC_KEY_FILE")); fp != "" {
		b, err := os.ReadFile(fp)
		if err != nil {
			return MachineJWTConfig{}, fmt.Errorf("config: MACHINE_JWT_RSA_PUBLIC_KEY_FILE: %w", err)
		}
		rsaPEM = b
	} else if s := strings.TrimSpace(getenvAlias("", "MACHINE_JWT_RSA_PUBLIC_KEY")); s != "" {
		rsaPEM = []byte(s)
	}
	var ed25519PEM []byte
	if fp := strings.TrimSpace(getenvAlias("", "MACHINE_JWT_ED25519_PUBLIC_KEY_FILE")); fp != "" {
		b, err := os.ReadFile(fp)
		if err != nil {
			return MachineJWTConfig{}, fmt.Errorf("config: MACHINE_JWT_ED25519_PUBLIC_KEY_FILE: %w", err)
		}
		ed25519PEM = b
	} else if s := strings.TrimSpace(getenvAlias("", "MACHINE_JWT_ED25519_PUBLIC_KEY")); s != "" {
		ed25519PEM = []byte(s)
	}
	secret := []byte(strings.TrimSpace(getenvAlias("", "MACHINE_JWT_SECRET")))
	if len(secret) == 0 {
		secret = firstNonEmptyBytes(httpAuth.LoginJWTSecret, httpAuth.JWTSecret)
	}
	previous := []byte(strings.TrimSpace(getenvAlias("", "MACHINE_JWT_SECRET_PREVIOUS")))
	if len(previous) == 0 {
		previous = httpAuth.JWTSecretPrevious
	}
	additional := [][]byte{}
	if len(secret) == 0 || string(bytes.TrimSpace(secret)) != string(bytes.TrimSpace(httpAuth.LoginJWTSecret)) {
		additional = appendIfSecret(additional, httpAuth.LoginJWTSecret)
	}
	if len(secret) == 0 || string(bytes.TrimSpace(secret)) != string(bytes.TrimSpace(httpAuth.JWTSecret)) {
		additional = appendIfSecret(additional, httpAuth.JWTSecret)
	}
	jwksTTL := mustParseDuration("MACHINE_JWT_JWKS_CACHE_TTL", getenvAlias("5m", "MACHINE_JWT_JWKS_CACHE_TTL"))
	accessTTL := mustParseDuration("MACHINE_ACCESS_TTL", getenvAlias(httpAuth.AccessTokenTTL.String(), "MACHINE_ACCESS_TTL"))
	refreshTTL := mustParseDuration("MACHINE_REFRESH_TTL", getenvAlias(httpAuth.RefreshTokenTTL.String(), "MACHINE_REFRESH_TTL"))
	requireAudienceDefault := appEnv != AppEnvDevelopment && appEnv != AppEnvTest
	return MachineJWTConfig{
		Mode:                   mode,
		JWTAlgorithm:           strings.TrimSpace(getenvAlias("", "MACHINE_JWT_ALG")),
		JWTLeeway:              leeway,
		JWTSecret:              secret,
		JWTSecretPrevious:      previous,
		AdditionalHS256Secrets: additional,
		RSAPublicKeyPEM:        rsaPEM,
		Ed25519PublicKeyPEM:    ed25519PEM,
		JWKSURL:                strings.TrimSpace(getenvAlias("", "MACHINE_JWT_JWKS_URL")),
		JWKSCacheTTL:           jwksTTL,
		JWKSSkipStartupWarm:    getenvBoolAlias(false, "MACHINE_JWT_JWKS_SKIP_STARTUP_WARM"),
		ExpectedIssuer:         strings.TrimSpace(getenvAlias(httpAuth.ExpectedIssuer, "AUTH_ISSUER", "MACHINE_JWT_ISSUER")),
		ExpectedAudience:       strings.TrimSpace(getenvAlias("avf-machine-grpc", "AUTH_MACHINE_AUDIENCE", "MACHINE_JWT_AUDIENCE")),
		AccessTokenTTL:         accessTTL,
		RefreshTokenTTL:        refreshTTL,
		RequireAudience:        getenvBool("MACHINE_AUTH_REQUIRE_AUDIENCE", requireAudienceDefault),
	}, nil
}

func loadHTTPRateLimitConfig() HTTPRateLimitConfig {
	lockout := mustParseDuration("RATE_LIMIT_LOCKOUT_WINDOW", getenv("RATE_LIMIT_LOCKOUT_WINDOW", "1m"))
	return HTTPRateLimitConfig{
		SensitiveWritesEnabled: getenvBool("HTTP_RATE_LIMIT_SENSITIVE_WRITES_ENABLED", false),
		SensitiveWritesRPS:     getenvFloat64("HTTP_RATE_LIMIT_SENSITIVE_WRITES_RPS", 15),
		SensitiveWritesBurst:   getenvInt("HTTP_RATE_LIMIT_SENSITIVE_WRITES_BURST", 30),
		Abuse: AbuseRateLimitConfig{
			Enabled:                  getenvBool("RATE_LIMIT_ENABLED", false),
			LoginPerMinute:           getenvInt("RATE_LIMIT_LOGIN_PER_MIN", 30),
			RefreshPerMinute:         getenvInt("RATE_LIMIT_REFRESH_PER_MIN", 120),
			AdminMutationPerMinute:   getenvInt("RATE_LIMIT_ADMIN_MUTATION_PER_MIN", 300),
			MachinePerMinute:         getenvInt("RATE_LIMIT_MACHINE_PER_MIN", 600),
			WebhookPerMinute:         getenvInt("RATE_LIMIT_WEBHOOK_PER_MIN", 1200),
			PublicPerMinute:          getenvInt("RATE_LIMIT_PUBLIC_PER_MIN", 60),
			CommandDispatchPerMinute: getenvInt("RATE_LIMIT_COMMAND_DISPATCH_PER_MIN", 120),
			ReportsReadPerMinute:     getenvInt("RATE_LIMIT_REPORTS_READ_PER_MIN", 120),
			LockoutWindow:            lockout,
		},
	}
}

func loadArtifactsConfig() ArtifactsConfig {
	lk := int32(getenvInt("ARTIFACTS_LIST_MAX_KEYS", 500))
	if lk > 1000 {
		lk = 1000
	}
	return ArtifactsConfig{
		Enabled:            getenvBool("OBJECT_STORAGE_ENABLED", getenvBool("API_ARTIFACTS_ENABLED", false)),
		MaxUploadBytes:     int64(getenvInt("PRODUCT_MEDIA_MAX_BYTES", getenvInt("ARTIFACTS_MAX_UPLOAD_BYTES", 100<<20))),
		DownloadPresignTTL: mustParseDuration("ARTIFACTS_DOWNLOAD_PRESIGN_TTL", getenv("ARTIFACTS_DOWNLOAD_PRESIGN_TTL", "15m")),
		ListMaxKeys:        lk,
		Bucket:             getenv("OBJECT_STORAGE_BUCKET", ""),
		PublicBaseURL:      getenv("OBJECT_STORAGE_PUBLIC_BASE_URL", ""),
		AllowedTypes:       splitCSV(getenv("PRODUCT_MEDIA_ALLOWED_TYPES", "image/jpeg,image/png,image/webp")),
		ThumbSize:          getenvInt("PRODUCT_MEDIA_THUMB_SIZE", 256),
		DisplaySize:        getenvInt("PRODUCT_MEDIA_DISPLAY_SIZE", 1024),
	}
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func getenvFloat64(key string, def float64) float64 {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return def
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return def
	}
	return v
}

func parseOptionalEnvUUID(key string) (uuid.UUID, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return uuid.Nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("config: invalid %s: %w", key, err)
	}
	return id, nil
}

func loadPostgresConfig(appEnv AppEnvironment) (PostgresConfig, error) {
	url := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	defaultMaxConns := int32(0)
	if url != "" {
		// Conservative defaults; override per deployment with DATABASE_MAX_CONNS.
		switch appEnv {
		case AppEnvStaging:
			defaultMaxConns = 5
		case AppEnvProduction:
			defaultMaxConns = 10
		default:
			defaultMaxConns = 3
		}
	}
	maxConns, err := getenvInt32Strict("DATABASE_MAX_CONNS", defaultMaxConns)
	if err != nil {
		return PostgresConfig{}, err
	}
	minConns, err := getenvInt32Strict("DATABASE_MIN_CONNS", 0)
	if err != nil {
		return PostgresConfig{}, err
	}
	apiMaxConns, err := getenvOptionalInt32Strict("API_DATABASE_MAX_CONNS")
	if err != nil {
		return PostgresConfig{}, err
	}
	workerMaxConns, err := getenvOptionalInt32Strict("WORKER_DATABASE_MAX_CONNS")
	if err != nil {
		return PostgresConfig{}, err
	}
	mqttIngestMaxConns, err := getenvOptionalInt32Strict("MQTT_INGEST_DATABASE_MAX_CONNS")
	if err != nil {
		return PostgresConfig{}, err
	}
	reconcilerMaxConns, err := getenvOptionalInt32Strict("RECONCILER_DATABASE_MAX_CONNS")
	if err != nil {
		return PostgresConfig{}, err
	}
	temporalWorkerMaxConns, err := getenvOptionalInt32Strict("TEMPORAL_WORKER_DATABASE_MAX_CONNS")
	if err != nil {
		return PostgresConfig{}, err
	}

	maxConnIdleTime := time.Duration(0)
	if raw := getenv("DATABASE_MAX_CONN_IDLE_TIME", ""); strings.TrimSpace(raw) != "" {
		maxConnIdleTime, err = parseDurationEnv("DATABASE_MAX_CONN_IDLE_TIME", raw)
		if err != nil {
			return PostgresConfig{}, err
		}
	} else if url != "" {
		maxConnIdleTime, err = parseDurationEnv("DATABASE_MAX_CONN_IDLE_TIME", "5m")
		if err != nil {
			return PostgresConfig{}, err
		}
	}
	maxConnLifetime := time.Duration(0)
	if raw := getenv("DATABASE_MAX_CONN_LIFETIME", ""); strings.TrimSpace(raw) != "" {
		maxConnLifetime, err = parseDurationEnv("DATABASE_MAX_CONN_LIFETIME", raw)
		if err != nil {
			return PostgresConfig{}, err
		}
	} else if url != "" {
		maxConnLifetime, err = parseDurationEnv("DATABASE_MAX_CONN_LIFETIME", "30m")
		if err != nil {
			return PostgresConfig{}, err
		}
	}

	return PostgresConfig{
		URL:                     url,
		MaxConns:                maxConns,
		MinConns:                minConns,
		MaxConnIdleTime:         maxConnIdleTime,
		MaxConnLifetime:         maxConnLifetime,
		APIMaxConns:             apiMaxConns,
		WorkerMaxConns:          workerMaxConns,
		MQTTIngestMaxConns:      mqttIngestMaxConns,
		ReconcilerMaxConns:      reconcilerMaxConns,
		TemporalWorkerMaxConns:  temporalWorkerMaxConns,
		SlowQueryLogThresholdMS: getenvInt("DATABASE_SLOW_QUERY_LOG_MS", 0),
	}, nil
}

func loadRedisConfig() (RedisConfig, error) {
	addr := strings.TrimSpace(getenv("REDIS_ADDR", ""))
	redisURL := strings.TrimSpace(os.Getenv("REDIS_URL"))
	enabled := getenvBool("REDIS_ENABLED", addr != "" || redisURL != "")
	username := strings.TrimSpace(os.Getenv("REDIS_USERNAME"))
	password := strings.TrimSpace(os.Getenv("REDIS_PASSWORD"))
	db := getenvInt("REDIS_DB", 0)
	tlsEnabled := getenvBool("REDIS_TLS_ENABLED", false)
	tlsInsecure := getenvBool("REDIS_TLS_INSECURE_SKIP_VERIFY", false)
	if addr == "" {
		if redisURL != "" {
			parsed, err := parseRedisURL(redisURL)
			if err != nil {
				return RedisConfig{}, err
			}
			addr = parsed.Addr
			if username == "" {
				username = parsed.Username
			}
			if password == "" {
				password = parsed.Password
			}
			if db == 0 {
				db = parsed.DB
			}
			if !tlsEnabled {
				tlsEnabled = parsed.TLSEnabled
			}
		}
	}
	if addr == "" {
		// Still surface password/DB so validate() can reject inconsistent combinations.
		return RedisConfig{
			Enabled:               enabled,
			Username:              username,
			Password:              password,
			DB:                    db,
			TLSEnabled:            tlsEnabled,
			TLSInsecureSkipVerify: tlsInsecure,
			KeyPrefix:             strings.TrimSpace(getenv("REDIS_KEY_PREFIX", "avf")),
		}, nil
	}

	return RedisConfig{
		Enabled:               enabled,
		Addr:                  addr,
		Username:              username,
		Password:              password,
		DB:                    db,
		TLSEnabled:            tlsEnabled,
		TLSInsecureSkipVerify: tlsInsecure,
		KeyPrefix:             strings.TrimSpace(getenv("REDIS_KEY_PREFIX", "avf")),
	}, nil
}

func loadMQTTConfig() MQTTConfig {
	return MQTTConfig{
		BrokerURL:          strings.TrimSpace(getenv("MQTT_BROKER_URL", "")),
		ClientID:           strings.TrimSpace(getenv("MQTT_CLIENT_ID", "")),
		APIClientID:        strings.TrimSpace(getenv("MQTT_CLIENT_ID_API", "")),
		IngestClientID:     strings.TrimSpace(getenv("MQTT_CLIENT_ID_INGEST", "")),
		Username:           strings.TrimSpace(getenv("MQTT_USERNAME", "")),
		Password:           os.Getenv("MQTT_PASSWORD"),
		TopicPrefix:        strings.TrimSpace(getenv("MQTT_TOPIC_PREFIX", "avf/devices")),
		TopicLayout:        strings.TrimSpace(getenv("MQTT_TOPIC_LAYOUT", "")),
		TLSEnabled:         getenvBool("MQTT_TLS_ENABLED", false),
		CAFile:             strings.TrimSpace(getenv("MQTT_CA_FILE", "")),
		CertFile:           strings.TrimSpace(getenv("MQTT_CERT_FILE", "")),
		KeyFile:            strings.TrimSpace(getenv("MQTT_KEY_FILE", "")),
		InsecureSkipVerify: getenvBool("MQTT_INSECURE_SKIP_VERIFY", false),
	}
}

func loadSMTPConfig() SMTPConfig {
	return SMTPConfig{
		Host:     strings.TrimSpace(os.Getenv("SMTP_HOST")),
		Port:     getenvInt("SMTP_PORT", 0),
		Username: strings.TrimSpace(os.Getenv("SMTP_USER")),
		Password: os.Getenv("SMTP_PASSWORD"),
	}
}

func parseRedisURL(raw string) (RedisConfig, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return RedisConfig{}, fmt.Errorf("config: invalid REDIS_URL %q: %w", raw, err)
	}
	if u.Host == "" {
		return RedisConfig{}, fmt.Errorf("config: invalid REDIS_URL %q: missing host", raw)
	}
	cfg := RedisConfig{
		Addr:       u.Host,
		TLSEnabled: strings.EqualFold(u.Scheme, "rediss"),
	}
	if u.User != nil {
		cfg.Username = u.User.Username()
		if pw, ok := u.User.Password(); ok {
			cfg.Password = pw
		}
	}
	if p := strings.TrimPrefix(strings.TrimSpace(u.Path), "/"); p != "" {
		db, err := strconv.Atoi(p)
		if err != nil {
			return RedisConfig{}, fmt.Errorf("config: invalid REDIS_URL %q: invalid database %q", raw, p)
		}
		cfg.DB = db
	}
	return cfg, nil
}

func (r RuntimeConfig) EffectiveRuntimeRole(processName string) string {
	if s := strings.TrimSpace(r.RuntimeRole); s != "" {
		return s
	}
	return strings.TrimSpace(processName)
}

func (m MQTTConfig) ClientIDForProcess(processName string) string {
	if s := strings.TrimSpace(m.ClientID); s != "" {
		return s
	}
	switch strings.TrimSpace(processName) {
	case "api":
		return firstNonEmptyTrimmed(m.APIClientID, m.IngestClientID)
	case "mqtt-ingest":
		return firstNonEmptyTrimmed(m.IngestClientID, m.APIClientID)
	default:
		return firstNonEmptyTrimmed(m.APIClientID, m.IngestClientID)
	}
}

func (r RedisConfig) TLSConfig() *tls.Config {
	if !r.TLSEnabled {
		return nil
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: r.TLSInsecureSkipVerify}
}

func firstNonEmptyTrimmed(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// paymentWebhookReplayWindowDuration loads COMMERCE_PAYMENT_WEBHOOK_REPLAY_WINDOW (seconds) when set,
// otherwise COMMERCE_PAYMENT_WEBHOOK_TIMESTAMP_SKEW_SECONDS (legacy alias), default 300.
func paymentWebhookReplayWindowDuration() time.Duration {
	if raw, ok := os.LookupEnv("COMMERCE_PAYMENT_WEBHOOK_REPLAY_WINDOW"); ok && strings.TrimSpace(raw) != "" {
		sec, err := strconv.Atoi(strings.TrimSpace(raw))
		if err == nil && sec > 0 {
			return time.Duration(sec) * time.Second
		}
	}
	return time.Duration(max(1, getenvInt("COMMERCE_PAYMENT_WEBHOOK_TIMESTAMP_SKEW_SECONDS", 300))) * time.Second
}

func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func getenvAlias(def string, keys ...string) string {
	for _, key := range keys {
		if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	return def
}

func getenvBool(key string, def bool) bool {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return def
	}
	v, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return def
	}
	return v
}

func getenvBoolAlias(def bool, keys ...string) bool {
	for _, key := range keys {
		raw, ok := os.LookupEnv(key)
		if !ok || strings.TrimSpace(raw) == "" {
			continue
		}
		v, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			return def
		}
		return v
	}
	return def
}

func firstNonEmptyBytes(vals ...[]byte) []byte {
	for _, v := range vals {
		if len(bytes.TrimSpace(v)) > 0 {
			return v
		}
	}
	return nil
}

func appendIfSecret(out [][]byte, secret []byte) [][]byte {
	if len(bytes.TrimSpace(secret)) > 0 {
		return append(out, secret)
	}
	return out
}

func getenvInt(key string, def int) int {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return def
	}
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return def
	}
	return v
}

func getenvInt32Strict(key string, def int32) (int32, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return def, nil
	}
	v, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("config: invalid %s %q: %w", key, raw, err)
	}
	return int32(v), nil
}

func getenvOptionalInt32Strict(key string) (*int32, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	v, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("config: invalid %s %q: %w", key, raw, err)
	}
	value := int32(v)
	return &value, nil
}

func (p PostgresConfig) MaxConnsForProcess(processName string) int32 {
	switch strings.TrimSpace(processName) {
	case "api":
		if p.APIMaxConns != nil {
			return *p.APIMaxConns
		}
	case "worker":
		if p.WorkerMaxConns != nil {
			return *p.WorkerMaxConns
		}
	case "mqtt-ingest":
		if p.MQTTIngestMaxConns != nil {
			return *p.MQTTIngestMaxConns
		}
	case "reconciler":
		if p.ReconcilerMaxConns != nil {
			return *p.ReconcilerMaxConns
		}
	case "temporal-worker":
		if p.TemporalWorkerMaxConns != nil {
			return *p.TemporalWorkerMaxConns
		}
	}
	return p.MaxConns
}

func (p PostgresConfig) overrideMaxConns() map[string]int32 {
	overrides := make(map[string]int32)
	if p.APIMaxConns != nil {
		overrides["API_DATABASE_MAX_CONNS"] = *p.APIMaxConns
	}
	if p.WorkerMaxConns != nil {
		overrides["WORKER_DATABASE_MAX_CONNS"] = *p.WorkerMaxConns
	}
	if p.MQTTIngestMaxConns != nil {
		overrides["MQTT_INGEST_DATABASE_MAX_CONNS"] = *p.MQTTIngestMaxConns
	}
	if p.ReconcilerMaxConns != nil {
		overrides["RECONCILER_DATABASE_MAX_CONNS"] = *p.ReconcilerMaxConns
	}
	if p.TemporalWorkerMaxConns != nil {
		overrides["TEMPORAL_WORKER_DATABASE_MAX_CONNS"] = *p.TemporalWorkerMaxConns
	}
	return overrides
}

func getenvInt64(key string, def int64) int64 {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return def
	}
	v, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return def
	}
	return v
}

func mustParseDuration(field, raw string) time.Duration {
	d, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		panic(fmt.Sprintf("config: invalid duration for %s: %q", field, raw))
	}
	return d
}

func parseDurationEnv(field, raw string) (time.Duration, error) {
	d, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("config: invalid duration for %s: %q: %w", field, raw, err)
	}
	return d, nil
}

func normalizeTCPAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "0.0.0.0" + addr
	}
	return addr
}

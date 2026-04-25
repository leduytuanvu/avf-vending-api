package config

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/version"
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
	// PaymentWebhookHMACSecret, when non-empty, enables POST .../payments/{id}/webhooks without Bearer JWT
	// using X-AVF-Webhook-Timestamp + X-AVF-Webhook-Signature (HMAC-SHA256 over "{timestamp}.{rawBody}").
	PaymentWebhookHMACSecret string
	// PaymentWebhookVerification selects webhook signature verification. Only "avf_hmac" is implemented.
	PaymentWebhookVerification string
	// PaymentWebhookTimestampSkew bounds how far X-AVF-Webhook-Timestamp may differ from server time.
	PaymentWebhookTimestampSkew time.Duration
	// PaymentWebhookAllowUnsigned, when true with an empty secret, skips HMAC verification in non-production only.
	PaymentWebhookAllowUnsigned bool
	// PaymentWebhookUnsafeAllowUnsignedProduction allows empty secret / no HMAC in production (documented unsafe).
	PaymentWebhookUnsafeAllowUnsignedProduction bool
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

// Config is the complete process configuration loaded from the environment.
type Config struct {
	AppEnv AppEnvironment
	// PaymentEnv is "sandbox" or "live" from PAYMENT_ENV; empty means unset (rules depend on APP_ENV).
	PaymentEnv string
	ProcessName string
	Runtime     RuntimeConfig
	Build       BuildConfig

	LogLevel  string
	LogFormat string

	HTTP HTTPConfig
	GRPC GRPCConfig
	Ops  OperationsConfig

	Postgres PostgresConfig
	Redis    RedisConfig
	NATS     NATSConfig
	MQTT     MQTTConfig

	ReadinessStrict bool
	MetricsEnabled  bool
	// MetricsExposeOnPublicHTTP registers GET /metrics on the main HTTP listener (HTTP_ADDR, e.g. :8080).
	// When false in production (the default when METRICS_EXPOSE_ON_PUBLIC_HTTP is unset), Prometheus must
	// scrape HTTP_OPS_ADDR/metrics on the private ops listener. When true in production, METRICS_SCRAPE_TOKEN
	// is required and protects the public /metrics route.
	MetricsExposeOnPublicHTTP bool
	// MetricsScrapeToken protects GET /metrics on the public listener when set (Authorization: Bearer <token>).
	// Required (min 16 chars) when APP_ENV=production and METRICS_EXPOSE_ON_PUBLIC_HTTP=true.
	MetricsScrapeToken string
	// SwaggerUIEnabled mounts Swagger UI (HTML) under /swagger/ when true. If HTTP_SWAGGER_UI_ENABLED is set,
	// only true/1 enables. If unset, non-production defaults on and production defaults off.
	// OpenAPIJSONEnabled controls GET /swagger/doc.json; when false, doc.json is not served (404).
	// SwaggerUIEnabled true requires OpenAPIJSONEnabled true (the UI loads doc.json).
	SwaggerUIEnabled     bool
	OpenAPIJSONEnabled   bool
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

	// HTTPAuth selects JWT validation mode (HS256 dev secret vs RS256 PEM vs RS256 JWKS).
	HTTPAuth HTTPAuthConfig
	// HTTPRateLimit configures optional abuse protection on mutating API routes.
	HTTPRateLimit HTTPRateLimitConfig

	// Artifacts enables S3-backed backend artifact APIs (requires object store env when enabled).
	Artifacts ArtifactsConfig

	// Analytics optional cold-path sinks (ClickHouse HTTP); never required for OLTP correctness.
	Analytics AnalyticsConfig

	// SMTP is loaded from environment for provider-driven notification wiring. This repo does not
	// force SMTP usage at startup, but validates the shape when values are supplied.
	SMTP SMTPConfig
}

// ArtifactsConfig gates /v1/admin/.../artifacts routes and upload limits.
type ArtifactsConfig struct {
	Enabled            bool
	MaxUploadBytes     int64
	DownloadPresignTTL time.Duration
	ListMaxKeys        int32
}

// HTTPAuthConfig configures Bearer JWT validation for /v1 (see internal/platform/auth).
type HTTPAuthConfig struct {
	Mode string // hs256 (default), rs256_pem, rs256_jwks

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

	// RS256 PEM (single public key; rotation = deploy new PEM / JWKS).
	RSAPublicKeyPEM []byte

	// RS256 JWKS
	JWKSURL             string
	JWKSCacheTTL        time.Duration
	JWKSSkipStartupWarm bool

	ExpectedIssuer   string
	ExpectedAudience string
}

// HTTPRateLimitConfig enables token-bucket limits on sensitive mutating routes (POST under commerce, operator, dispatch).
type HTTPRateLimitConfig struct {
	SensitiveWritesEnabled bool
	SensitiveWritesRPS     float64
	SensitiveWritesBurst   int
}

// HTTPConfig holds the public HTTP API server settings.
type HTTPConfig struct {
	Addr              string
	ShutdownTimeout   time.Duration
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

// GRPCConfig holds internal gRPC server settings. When Enabled, the process exposes grpc.health.v1
// only unless callers pass ServiceRegistrars into grpcserver.NewServer (bootstrap does not today).
type GRPCConfig struct {
	Enabled         bool
	Addr            string
	ShutdownTimeout time.Duration
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
	Addr                  string
	Username              string
	Password              string
	DB                    int
	TLSEnabled            bool
	TLSInsecureSkipVerify bool
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
	URL string
}

// MQTTConfig holds broker connection settings shared by API publish and ingest workers.
type MQTTConfig struct {
	BrokerURL      string
	ClientID       string
	APIClientID    string
	IngestClientID string
	Username       string
	Password       string
	TopicPrefix    string
}

// RuntimeConfig holds deploy-time identity and public base URL metadata.
type RuntimeConfig struct {
	PublicBaseURL        string
	MachinePublicBaseURL string
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
	if err := c.Ops.validate(); err != nil {
		return err
	}
	if err := c.Postgres.validate(); err != nil {
		return err
	}
	if err := c.Redis.validate(); err != nil {
		return err
	}
	if err := c.NATS.validate(); err != nil {
		return err
	}
	if err := c.MQTT.validate(); err != nil {
		return err
	}
	if err := c.Runtime.validate(); err != nil {
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
	if err := c.HTTPAuth.validateProduction(c.AppEnv); err != nil {
		return err
	}
	if err := c.HTTPRateLimit.validate(); err != nil {
		return err
	}
	if err := c.Artifacts.validate(); err != nil {
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
	if err := c.validateProductionTelemetryNATS(); err != nil {
		return err
	}
	if err := c.validateMetricsHTTPExposure(); err != nil {
		return err
	}
	if err := c.Commerce.validate(c.AppEnv); err != nil {
		return err
	}
	if err := c.validateEnvironmentDeployment(); err != nil {
		return err
	}
	if err := c.validateSwaggerAndOpenAPI(); err != nil {
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
	return nil
}

func (g GRPCConfig) validate() error {
	if !g.Enabled {
		return nil
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
		if p.MaxConns != 0 || p.MinConns != 0 || p.MaxConnIdleTime != 0 || p.MaxConnLifetime != 0 || len(p.overrideMaxConns()) > 0 {
			return errors.New("config: DATABASE_* pool settings require DATABASE_URL")
		}
		return nil
	}
	if p.MaxConns <= 0 {
		return errors.New("config: DATABASE_MAX_CONNS must be > 0 when DATABASE_URL is set")
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
		if p.MinConns > maxConns {
			return fmt.Errorf("config: DATABASE_MIN_CONNS must be <= %s", envName)
		}
	}
	return nil
}

func (r RedisConfig) validate() error {
	if strings.TrimSpace(r.Addr) == "" {
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
	return nil
}

func (n NATSConfig) validate() error {
	if strings.TrimSpace(n.URL) == "" {
		return nil
	}
	if _, err := url.Parse(n.URL); err != nil {
		return fmt.Errorf("config: invalid NATS_URL %q: %w", n.URL, err)
	}
	return nil
}

func (m MQTTConfig) validate() error {
	if strings.TrimSpace(m.BrokerURL) == "" {
		if strings.TrimSpace(m.ClientID) != "" || strings.TrimSpace(m.APIClientID) != "" || strings.TrimSpace(m.IngestClientID) != "" ||
			strings.TrimSpace(m.Username) != "" || strings.TrimSpace(m.Password) != "" {
			return errors.New("config: MQTT_* credentials and client ids require MQTT_BROKER_URL")
		}
		return nil
	}
	if _, err := url.Parse(m.BrokerURL); err != nil {
		return fmt.Errorf("config: invalid MQTT_BROKER_URL %q: %w", m.BrokerURL, err)
	}
	if strings.TrimSpace(m.TopicPrefix) == "" {
		return errors.New("config: MQTT_TOPIC_PREFIX must be non-empty when MQTT_BROKER_URL is set")
	}
	if strings.TrimSpace(m.ClientID) == "" && strings.TrimSpace(m.APIClientID) == "" && strings.TrimSpace(m.IngestClientID) == "" {
		return errors.New("config: MQTT_BROKER_URL requires MQTT_CLIENT_ID and/or MQTT_CLIENT_ID_API / MQTT_CLIENT_ID_INGEST")
	}
	return nil
}

func (r RuntimeConfig) validate() error {
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
	case "", "hs256", "rs256_pem", "rs256_jwks":
	default:
		return fmt.Errorf("config: invalid HTTP_AUTH_MODE %q", h.Mode)
	}
	if mode == "rs256_pem" && len(h.RSAPublicKeyPEM) == 0 {
		return errors.New("config: HTTP_AUTH_MODE=rs256_pem requires HTTP_AUTH_JWT_RSA_PUBLIC_KEY or HTTP_AUTH_JWT_RSA_PUBLIC_KEY_FILE")
	}
	if mode == "rs256_jwks" && strings.TrimSpace(h.JWKSURL) == "" {
		return errors.New("config: HTTP_AUTH_MODE=rs256_jwks requires HTTP_AUTH_JWT_JWKS_URL")
	}
	if mode == "rs256_pem" || mode == "rs256_jwks" {
		if len(strings.TrimSpace(string(h.LoginJWTSecret))) == 0 {
			return errors.New("config: HTTP_AUTH_LOGIN_JWT_SECRET is required when HTTP_AUTH_MODE is rs256_pem or rs256_jwks (session access tokens are HS256-signed)")
		}
	}
	return nil
}

func (h HTTPAuthConfig) validateProduction(appEnv AppEnvironment) error {
	if appEnv != AppEnvProduction {
		return nil
	}
	mode := strings.ToLower(strings.TrimSpace(h.Mode))
	if mode == "" || mode == "hs256" {
		if len(h.JWTSecret) == 0 {
			return errors.New("config: production requires HTTP_AUTH_JWT_SECRET when HTTP_AUTH_MODE is hs256 (default)")
		}
	}
	return nil
}

func (h HTTPRateLimitConfig) validate() error {
	if !h.SensitiveWritesEnabled {
		return nil
	}
	if h.SensitiveWritesRPS <= 0 {
		return errors.New("config: HTTP_RATE_LIMIT_SENSITIVE_WRITES_RPS must be > 0 when rate limiting is enabled")
	}
	if h.SensitiveWritesBurst <= 0 {
		return errors.New("config: HTTP_RATE_LIMIT_SENSITIVE_WRITES_BURST must be > 0 when rate limiting is enabled")
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
		return fmt.Errorf("config: COMMERCE_PAYMENT_WEBHOOK_TIMESTAMP_SKEW_SECONDS must be between 30 and 86400 (effective %ds)", sec)
	}
	if c.PaymentWebhookAllowUnsigned && appEnv == AppEnvProduction {
		return errors.New("config: COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED cannot be set when APP_ENV=production; use COMMERCE_PAYMENT_WEBHOOK_UNSAFE_ALLOW_UNSIGNED_PRODUCTION")
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

func (a ArtifactsConfig) validate() error {
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
// wins (true/1 only). When unset, defaults to true for all APP_ENV values so production can expose OpenAPI
// while keeping Swagger UI off.
func loadOpenAPIJSONEnabled() bool {
	if _, ok := os.LookupEnv("HTTP_OPENAPI_JSON_ENABLED"); ok {
		return strings.EqualFold(strings.TrimSpace(os.Getenv("HTTP_OPENAPI_JSON_ENABLED")), "true") ||
			strings.TrimSpace(os.Getenv("HTTP_OPENAPI_JSON_ENABLED")) == "1"
	}
	return true
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

// Load reads configuration from the environment and validates it.
func Load() (*Config, error) {
	httpAuth, err := loadHTTPAuthConfig()
	if err != nil {
		return nil, err
	}
	hostname, _ := os.Hostname()
	appEnv := AppEnvironment(strings.TrimSpace(getenv("APP_ENV", string(AppEnvDevelopment))))
	postgresCfg, err := loadPostgresConfig(appEnv)
	if err != nil {
		return nil, err
	}
	metricsExposePublic, err := metricsExposeOnPublicHTTPFromEnv(appEnv)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		AppEnv:                    appEnv,
		PaymentEnv:                loadPaymentEnv(),
		LogLevel:                  strings.TrimSpace(getenv("LOG_LEVEL", "info")),
		LogFormat:                 strings.TrimSpace(getenv("LOG_FORMAT", "json")),
		ReadinessStrict:           getenvBool("READINESS_STRICT", false),
		MetricsEnabled:            getenvBool("METRICS_ENABLED", false),
		MetricsExposeOnPublicHTTP: metricsExposePublic,
		MetricsScrapeToken:     strings.TrimSpace(os.Getenv("METRICS_SCRAPE_TOKEN")),
		SwaggerUIEnabled:       loadSwaggerUIEnabled(),
		OpenAPIJSONEnabled:     loadOpenAPIJSONEnabled(),
		Runtime: RuntimeConfig{
			PublicBaseURL:        firstNonEmptyTrimmed(os.Getenv("APP_BASE_URL"), os.Getenv("PUBLIC_BASE_URL")),
			MachinePublicBaseURL: firstNonEmptyTrimmed(os.Getenv("MACHINE_PUBLIC_BASE_URL"), os.Getenv("APP_BASE_URL"), os.Getenv("PUBLIC_BASE_URL")),
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
		APIWiring: APIWiringRequirements{
			RequireAuthAdapter:             getenvBool("API_REQUIRE_AUTH_ADAPTER", false),
			RequireOutboxPublisher:         getenvBool("API_REQUIRE_OUTBOX_PUBLISHER", false),
			RequireMQTTPublisher:           getenvBool("API_REQUIRE_MQTT_PUBLISHER", false),
			RequireNATSRuntime:             getenvBool("API_REQUIRE_NATS_RUNTIME", false),
			RequirePaymentProviderRegistry: getenvBool("API_REQUIRE_PAYMENT_PROVIDER_REGISTRY", false),
		},
		Commerce: CommerceHTTPConfig{
			PaymentOutboxTopic:                          strings.TrimSpace(getenv("COMMERCE_PAYMENT_OUTBOX_TOPIC", "commerce.payments")),
			PaymentOutboxEventType:                      strings.TrimSpace(getenv("COMMERCE_PAYMENT_OUTBOX_EVENT_TYPE", "payment.session_started")),
			PaymentOutboxAggregateType:                  strings.TrimSpace(getenv("COMMERCE_PAYMENT_OUTBOX_AGGREGATE_TYPE", "payment")),
			PaymentWebhookHMACSecret:                    firstNonEmptyTrimmed(os.Getenv("PAYMENT_WEBHOOK_SECRET"), os.Getenv("COMMERCE_PAYMENT_WEBHOOK_HMAC_SECRET")),
			PaymentWebhookVerification:                  strings.TrimSpace(getenv("COMMERCE_PAYMENT_WEBHOOK_VERIFICATION", "avf_hmac")),
			PaymentWebhookTimestampSkew:                 time.Duration(max(1, getenvInt("COMMERCE_PAYMENT_WEBHOOK_TIMESTAMP_SKEW_SECONDS", 300))) * time.Second,
			PaymentWebhookAllowUnsigned:                 getenvBool("COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED", false),
			PaymentWebhookUnsafeAllowUnsignedProduction: getenvBool("COMMERCE_PAYMENT_WEBHOOK_UNSAFE_ALLOW_UNSIGNED_PRODUCTION", false),
		},
		CashSettlement: CashSettlementConfig{
			VarianceReviewThresholdMinor: getenvInt64("CASH_SETTLEMENT_VARIANCE_REVIEW_THRESHOLD_MINOR", 500),
		},
		Reconciler: loadReconcilerConfig(),
		Temporal:   loadTemporalConfig(),
		HTTP: HTTPConfig{
			Addr:              strings.TrimSpace(getenv("HTTP_ADDR", ":8080")),
			ShutdownTimeout:   mustParseDuration("HTTP_SHUTDOWN_TIMEOUT", getenv("HTTP_SHUTDOWN_TIMEOUT", "15s")),
			ReadHeaderTimeout: mustParseDuration("HTTP_READ_HEADER_TIMEOUT", getenv("HTTP_READ_HEADER_TIMEOUT", "5s")),
			ReadTimeout:       mustParseDuration("HTTP_READ_TIMEOUT", getenv("HTTP_READ_TIMEOUT", "30s")),
			WriteTimeout:      mustParseDuration("HTTP_WRITE_TIMEOUT", getenv("HTTP_WRITE_TIMEOUT", "30s")),
			IdleTimeout:       mustParseDuration("HTTP_IDLE_TIMEOUT", getenv("HTTP_IDLE_TIMEOUT", "60s")),
		},
		GRPC: GRPCConfig{
			Enabled:         getenvBool("GRPC_ENABLED", false),
			Addr:            strings.TrimSpace(getenv("GRPC_ADDR", ":9090")),
			ShutdownTimeout: mustParseDuration("GRPC_SHUTDOWN_TIMEOUT", getenv("GRPC_SHUTDOWN_TIMEOUT", "15s")),
		},
		Ops: OperationsConfig{
			HTTPAddr:              strings.TrimSpace(getenv("HTTP_OPS_ADDR", "")),
			ReadinessTimeout:      mustParseDuration("OPS_READINESS_TIMEOUT", getenv("OPS_READINESS_TIMEOUT", "2s")),
			ShutdownTimeout:       mustParseDuration("OPS_SHUTDOWN_TIMEOUT", getenv("OPS_SHUTDOWN_TIMEOUT", "5s")),
			TracerShutdownTimeout: mustParseDuration("TRACER_SHUTDOWN_TIMEOUT", getenv("TRACER_SHUTDOWN_TIMEOUT", "10s")),
		},
		Postgres: postgresCfg,
		NATS: NATSConfig{
			URL: strings.TrimSpace(getenv("NATS_URL", "")),
		},
		MQTT: loadMQTTConfig(),
		Telemetry: TelemetryConfig{
			ServiceName:  strings.TrimSpace(getenv("OTEL_SERVICE_NAME", "avf-vending-api")),
			OTLPEndpoint: strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")),
			Insecure:     getenvBool("OTEL_INSECURE", true),
			SDKDisabled:  getenvBool("OTEL_SDK_DISABLED", false),
		},
		MQTTDeviceTelemetry: loadMQTTDeviceTelemetryConfig(),
		TelemetryJetStream:  loadTelemetryJetStreamConfig(),
		HTTPAuth:            httpAuth,
		HTTPRateLimit:       loadHTTPRateLimitConfig(),
		Artifacts:           loadArtifactsConfig(),
		Analytics:           loadAnalyticsConfig(),
		SMTP:                loadSMTPConfig(),
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
	mode := strings.TrimSpace(getenv("HTTP_AUTH_MODE", "hs256"))
	leeway := mustParseDuration("HTTP_AUTH_JWT_LEEWAY", getenv("HTTP_AUTH_JWT_LEEWAY", "45s"))

	var rsaPEM []byte
	if fp := strings.TrimSpace(getenv("HTTP_AUTH_JWT_RSA_PUBLIC_KEY_FILE", "")); fp != "" {
		b, err := os.ReadFile(fp)
		if err != nil {
			return HTTPAuthConfig{}, fmt.Errorf("config: HTTP_AUTH_JWT_RSA_PUBLIC_KEY_FILE: %w", err)
		}
		rsaPEM = b
	} else if s := strings.TrimSpace(os.Getenv("HTTP_AUTH_JWT_RSA_PUBLIC_KEY")); s != "" {
		rsaPEM = []byte(s)
	}

	jwksTTL := mustParseDuration("HTTP_AUTH_JWT_JWKS_CACHE_TTL", getenv("HTTP_AUTH_JWT_JWKS_CACHE_TTL", "5m"))
	accessTTL := mustParseDuration("HTTP_AUTH_ACCESS_TTL", getenv("HTTP_AUTH_ACCESS_TTL", "15m"))
	refreshTTL := mustParseDuration("HTTP_AUTH_REFRESH_TTL", getenv("HTTP_AUTH_REFRESH_TTL", "720h"))

	return HTTPAuthConfig{
		Mode:                mode,
		JWTLeeway:           leeway,
		JWTSecret:           []byte(strings.TrimSpace(os.Getenv("HTTP_AUTH_JWT_SECRET"))),
		JWTSecretPrevious:   []byte(strings.TrimSpace(os.Getenv("HTTP_AUTH_JWT_SECRET_PREVIOUS"))),
		LoginJWTSecret:      []byte(strings.TrimSpace(os.Getenv("HTTP_AUTH_LOGIN_JWT_SECRET"))),
		AccessTokenTTL:      accessTTL,
		RefreshTokenTTL:     refreshTTL,
		RSAPublicKeyPEM:     rsaPEM,
		JWKSURL:             strings.TrimSpace(os.Getenv("HTTP_AUTH_JWT_JWKS_URL")),
		JWKSCacheTTL:        jwksTTL,
		JWKSSkipStartupWarm: getenvBool("HTTP_AUTH_JWT_JWKS_SKIP_STARTUP_WARM", false),
		ExpectedIssuer:      strings.TrimSpace(os.Getenv("HTTP_AUTH_JWT_ISSUER")),
		ExpectedAudience:    strings.TrimSpace(os.Getenv("HTTP_AUTH_JWT_AUDIENCE")),
	}, nil
}

func loadHTTPRateLimitConfig() HTTPRateLimitConfig {
	return HTTPRateLimitConfig{
		SensitiveWritesEnabled: getenvBool("HTTP_RATE_LIMIT_SENSITIVE_WRITES_ENABLED", false),
		SensitiveWritesRPS:     getenvFloat64("HTTP_RATE_LIMIT_SENSITIVE_WRITES_RPS", 15),
		SensitiveWritesBurst:   getenvInt("HTTP_RATE_LIMIT_SENSITIVE_WRITES_BURST", 30),
	}
}

func loadArtifactsConfig() ArtifactsConfig {
	lk := int32(getenvInt("ARTIFACTS_LIST_MAX_KEYS", 500))
	if lk > 1000 {
		lk = 1000
	}
	return ArtifactsConfig{
		Enabled:            getenvBool("API_ARTIFACTS_ENABLED", false),
		MaxUploadBytes:     int64(getenvInt("ARTIFACTS_MAX_UPLOAD_BYTES", 100<<20)),
		DownloadPresignTTL: mustParseDuration("ARTIFACTS_DOWNLOAD_PRESIGN_TTL", getenv("ARTIFACTS_DOWNLOAD_PRESIGN_TTL", "15m")),
		ListMaxKeys:        lk,
	}
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
		URL:                    url,
		MaxConns:               maxConns,
		MinConns:               minConns,
		MaxConnIdleTime:        maxConnIdleTime,
		MaxConnLifetime:        maxConnLifetime,
		APIMaxConns:            apiMaxConns,
		WorkerMaxConns:         workerMaxConns,
		MQTTIngestMaxConns:     mqttIngestMaxConns,
		ReconcilerMaxConns:     reconcilerMaxConns,
		TemporalWorkerMaxConns: temporalWorkerMaxConns,
	}, nil
}

func loadRedisConfig() (RedisConfig, error) {
	addr := strings.TrimSpace(getenv("REDIS_ADDR", ""))
	username := strings.TrimSpace(os.Getenv("REDIS_USERNAME"))
	password := strings.TrimSpace(os.Getenv("REDIS_PASSWORD"))
	db := getenvInt("REDIS_DB", 0)
	tlsEnabled := getenvBool("REDIS_TLS_ENABLED", false)
	tlsInsecure := getenvBool("REDIS_TLS_INSECURE_SKIP_VERIFY", false)
	if addr == "" {
		redisURL := strings.TrimSpace(os.Getenv("REDIS_URL"))
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
			Username:              username,
			Password:              password,
			DB:                    db,
			TLSEnabled:            tlsEnabled,
			TLSInsecureSkipVerify: tlsInsecure,
		}, nil
	}

	return RedisConfig{
		Addr:                  addr,
		Username:              username,
		Password:              password,
		DB:                    db,
		TLSEnabled:            tlsEnabled,
		TLSInsecureSkipVerify: tlsInsecure,
	}, nil
}

func loadMQTTConfig() MQTTConfig {
	return MQTTConfig{
		BrokerURL:      strings.TrimSpace(getenv("MQTT_BROKER_URL", "")),
		ClientID:       strings.TrimSpace(getenv("MQTT_CLIENT_ID", "")),
		APIClientID:    strings.TrimSpace(getenv("MQTT_CLIENT_ID_API", "")),
		IngestClientID: strings.TrimSpace(getenv("MQTT_CLIENT_ID_INGEST", "")),
		Username:       strings.TrimSpace(getenv("MQTT_USERNAME", "")),
		Password:       os.Getenv("MQTT_PASSWORD"),
		TopicPrefix:    strings.TrimSpace(getenv("MQTT_TOPIC_PREFIX", "avf/devices")),
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

func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
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

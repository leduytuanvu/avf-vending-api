package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// AppEnvironment controls logging defaults and whether .env loading is expected in dev.
type AppEnvironment string

const (
	AppEnvDevelopment AppEnvironment = "development"
	AppEnvStaging     AppEnvironment = "staging"
	AppEnvProduction  AppEnvironment = "production"
)

// CommerceHTTPConfig configures durable outbox defaults for payment-session HTTP (no PSP I/O).
type CommerceHTTPConfig struct {
	PaymentOutboxTopic         string
	PaymentOutboxEventType     string
	PaymentOutboxAggregateType string
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

	LogLevel  string
	LogFormat string

	HTTP HTTPConfig
	GRPC GRPCConfig

	Postgres PostgresConfig
	Redis    RedisConfig

	ReadinessStrict bool
	MetricsEnabled  bool
	// SwaggerUIEnabled exposes /swagger/* (Swagger UI + doc.json) when true. Default is off in
	// production unless HTTP_SWAGGER_UI_ENABLED is set explicitly.
	SwaggerUIEnabled bool
	// WorkerMetricsListen is the bind address for cmd/worker /metrics (Prometheus).
	// When empty and MetricsEnabled is true, cmd/worker defaults to 127.0.0.1:9091.
	WorkerMetricsListen string
	// ReconcilerMetricsListen is the bind address for cmd/reconciler /metrics when MetricsEnabled.
	// When empty, defaults to 127.0.0.1:9092.
	ReconcilerMetricsListen string
	// MQTTIngestMetricsListen is the bind address for cmd/mqtt-ingest /metrics when MetricsEnabled.
	// When empty, defaults to 127.0.0.1:9093.
	MQTTIngestMetricsListen string

	APIWiring APIWiringRequirements

	Commerce CommerceHTTPConfig

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

// PostgresConfig holds PostgreSQL pool settings used for readiness and future persistence.
type PostgresConfig struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnIdleTime time.Duration
	MaxConnLifetime time.Duration
}

// RedisConfig holds Redis client settings used for readiness and future cache usage.
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// TelemetryConfig holds OpenTelemetry exporter settings.
type TelemetryConfig struct {
	ServiceName  string
	OTLPEndpoint string
	Insecure     bool
	SDKDisabled  bool
}

// Validate checks invariants and cross-field rules after environment parsing.
func (c *Config) Validate() error {
	if c == nil {
		return errors.New("config: nil")
	}

	switch c.AppEnv {
	case AppEnvDevelopment, AppEnvStaging, AppEnvProduction:
	default:
		return fmt.Errorf("config: invalid APP_ENV %q", c.AppEnv)
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
	if err := c.Postgres.validate(); err != nil {
		return err
	}
	if err := c.Redis.validate(); err != nil {
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

func (p PostgresConfig) validate() error {
	if strings.TrimSpace(p.URL) == "" {
		if p.MaxConns != 0 || p.MinConns != 0 {
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
	return nil
}

func (r RedisConfig) validate() error {
	if strings.TrimSpace(r.Addr) == "" {
		if strings.TrimSpace(r.Password) != "" {
			return errors.New("config: REDIS_PASSWORD requires REDIS_ADDR")
		}
		if r.DB != 0 {
			return errors.New("config: REDIS_DB requires REDIS_ADDR")
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

func loadSwaggerUIEnabled() bool {
	if _, ok := os.LookupEnv("HTTP_SWAGGER_UI_ENABLED"); ok {
		return strings.EqualFold(strings.TrimSpace(os.Getenv("HTTP_SWAGGER_UI_ENABLED")), "true") ||
			strings.TrimSpace(os.Getenv("HTTP_SWAGGER_UI_ENABLED")) == "1"
	}
	app := strings.ToLower(strings.TrimSpace(getenv("APP_ENV", string(AppEnvDevelopment))))
	return app != string(AppEnvProduction)
}

// Load reads configuration from the environment and validates it.
func Load() (*Config, error) {
	httpAuth, err := loadHTTPAuthConfig()
	if err != nil {
		return nil, err
	}
	cfg := &Config{
		AppEnv:                  AppEnvironment(strings.TrimSpace(getenv("APP_ENV", string(AppEnvDevelopment)))),
		LogLevel:                strings.TrimSpace(getenv("LOG_LEVEL", "info")),
		LogFormat:               strings.TrimSpace(getenv("LOG_FORMAT", "json")),
		ReadinessStrict:         getenvBool("READINESS_STRICT", false),
		MetricsEnabled:          getenvBool("METRICS_ENABLED", false),
		SwaggerUIEnabled:        loadSwaggerUIEnabled(),
		WorkerMetricsListen:     strings.TrimSpace(getenv("WORKER_METRICS_LISTEN", "")),
		ReconcilerMetricsListen: strings.TrimSpace(getenv("RECONCILER_METRICS_LISTEN", "")),
		MQTTIngestMetricsListen: strings.TrimSpace(getenv("MQTT_INGEST_METRICS_LISTEN", "")),
		APIWiring: APIWiringRequirements{
			RequireAuthAdapter:             getenvBool("API_REQUIRE_AUTH_ADAPTER", false),
			RequireOutboxPublisher:         getenvBool("API_REQUIRE_OUTBOX_PUBLISHER", false),
			RequireMQTTPublisher:           getenvBool("API_REQUIRE_MQTT_PUBLISHER", false),
			RequireNATSRuntime:             getenvBool("API_REQUIRE_NATS_RUNTIME", false),
			RequirePaymentProviderRegistry: getenvBool("API_REQUIRE_PAYMENT_PROVIDER_REGISTRY", false),
		},
		Commerce: CommerceHTTPConfig{
			PaymentOutboxTopic:         strings.TrimSpace(getenv("COMMERCE_PAYMENT_OUTBOX_TOPIC", "commerce.payments")),
			PaymentOutboxEventType:     strings.TrimSpace(getenv("COMMERCE_PAYMENT_OUTBOX_EVENT_TYPE", "payment.session_started")),
			PaymentOutboxAggregateType: strings.TrimSpace(getenv("COMMERCE_PAYMENT_OUTBOX_AGGREGATE_TYPE", "payment")),
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
		Postgres: loadPostgresConfig(),
		Redis:    loadRedisConfig(),
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

func loadPostgresConfig() PostgresConfig {
	url := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if url == "" {
		return PostgresConfig{}
	}

	return PostgresConfig{
		URL:             url,
		MaxConns:        int32(getenvInt("DATABASE_MAX_CONNS", 10)),
		MinConns:        int32(getenvInt("DATABASE_MIN_CONNS", 0)),
		MaxConnIdleTime: mustParseDuration("DATABASE_MAX_CONN_IDLE_TIME", getenv("DATABASE_MAX_CONN_IDLE_TIME", "30m")),
		MaxConnLifetime: mustParseDuration("DATABASE_MAX_CONN_LIFETIME", getenv("DATABASE_MAX_CONN_LIFETIME", "55m")),
	}
}

func loadRedisConfig() RedisConfig {
	addr := strings.TrimSpace(getenv("REDIS_ADDR", ""))
	password := strings.TrimSpace(os.Getenv("REDIS_PASSWORD"))
	db := getenvInt("REDIS_DB", 0)
	if addr == "" {
		// Still surface password/DB so validate() can reject inconsistent combinations.
		return RedisConfig{Password: password, DB: db}
	}

	return RedisConfig{
		Addr:     addr,
		Password: password,
		DB:       db,
	}
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

func normalizeTCPAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "0.0.0.0" + addr
	}
	return addr
}

package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"
)

// Well-known public URLs and MQTT namespace (docs + validation).
const (
	ProductionPublicBaseURL = "https://api.ldtv.dev"
	ProductionMQTTPrefix    = "avf/devices"
)

// Payment environment values for PAYMENT_ENV.
const (
	PaymentEnvSandbox = "sandbox"
	PaymentEnvLive    = "live"
)

// normalizeBaseURL removes trailing slash for comparisons.
func normalizeBaseURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	return strings.TrimRight(s, "/")
}

func publicBaseURLForValidation(c *Config) string {
	return normalizeBaseURL(c.Runtime.PublicBaseURL)
}

func hostFromDatabaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("empty database url")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	h := u.Hostname()
	if h == "" {
		return "", errors.New("database url missing host")
	}
	return strings.ToLower(h), nil
}

func isLocalDatabaseHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "localhost" || h == "127.0.0.1" || h == "::1" {
		return true
	}
	if net.ParseIP(h) != nil {
		// 127.0.0.0/8, etc.
		if ip := net.ParseIP(h); ip != nil && ip.IsLoopback() {
			return true
		}
	}
	return false
}

// productionSandboxFamilyCommercePaymentProvider reports mock/sandbox PSP keys disallowed in production.
// Keep aligned with internal/platform/payments.sandboxFamilyProviderKey (empty means runtime would fall back to sandbox).
func productionSandboxFamilyCommercePaymentProvider(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "", "mock", "sandbox", "test", "psp_fixture", "dev", "psp_grpc_int":
		return true
	default:
		return false
	}
}

func (c *Config) validateEnvironmentDeployment() error {
	if c == nil {
		return errors.New("config: nil")
	}

	pe := strings.ToLower(strings.TrimSpace(c.PaymentEnv))
	if pe != "" && pe != PaymentEnvSandbox && pe != PaymentEnvLive {
		return fmt.Errorf("config: invalid PAYMENT_ENV %q (use sandbox or live)", c.PaymentEnv)
	}

	if err := c.validateByAppEnv(pe); err != nil {
		return err
	}

	// Staging: DATABASE_URL must not equal production URL when both are in the process environment
	// (e.g. CI with both STAGING_* and PRODUCTION_* set).
	if c.AppEnv == AppEnvStaging {
		db := strings.TrimSpace(c.Postgres.URL)
		prod := strings.TrimSpace(os.Getenv("PRODUCTION_DATABASE_URL"))
		if db != "" && prod != "" && db == prod {
			return errors.New("config: staging DATABASE_URL must not equal PRODUCTION_DATABASE_URL")
		}
	}
	if c.AppEnv == AppEnvProduction {
		db := strings.TrimSpace(c.Postgres.URL)
		st := strings.TrimSpace(os.Getenv("STAGING_DATABASE_URL"))
		if db != "" && st != "" && db == st {
			return errors.New("config: production DATABASE_URL must not equal STAGING_DATABASE_URL")
		}
	}

	// Cross-environment host mismatch guard (safety when operators paste the wrong connection string).
	if err := c.validateDatabaseIdentitySeparation(); err != nil {
		return err
	}

	return nil
}

func (c *Config) validateByAppEnv(payment string) error {
	switch c.AppEnv {
	case AppEnvDevelopment, AppEnvTest:
		allowLive := getenvBool("DEVELOPMENT_ALLOW_LIVE_PAYMENT", false)
		if payment == PaymentEnvLive && !allowLive {
			return errors.New("config: PAYMENT_ENV=live in development or test requires DEVELOPMENT_ALLOW_LIVE_PAYMENT=true (unsafe; local verification only)")
		}
		return nil

	case AppEnvStaging:
		if err := c.validateStaging(payment); err != nil {
			return err
		}
		return nil

	case AppEnvProduction:
		if err := c.validateProduction(payment); err != nil {
			return err
		}
		return nil
	default:
		return nil
	}
}

func (c *Config) validateStaging(payment string) error {
	if strings.TrimSpace(c.Postgres.URL) == "" {
		return errors.New("config: APP_ENV=staging requires DATABASE_URL")
	}
	allowLocal := getenvBool("STAGING_ALLOW_LOCAL_DATABASE", false)
	host, err := hostFromDatabaseURL(c.Postgres.URL)
	if err != nil {
		return fmt.Errorf("config: DATABASE_URL: %w", err)
	}
	if isLocalDatabaseHost(host) && !allowLocal {
		return errors.New("config: staging DATABASE_URL must not use localhost/loopback; set STAGING_ALLOW_LOCAL_DATABASE=true only for local staging experiments")
	}
	if payment != PaymentEnvSandbox {
		if payment == "" {
			return errors.New("config: APP_ENV=staging requires PAYMENT_ENV=sandbox (set explicitly; do not rely on unset value)")
		}
		return errors.New("config: APP_ENV=staging requires PAYMENT_ENV=sandbox")
	}
	prodBase := ProductionPublicBaseURL
	if pub := publicBaseURLForValidation(c); pub != "" {
		if pub == prodBase {
			return fmt.Errorf("config: staging PUBLIC_BASE_URL must not be the production URL %q", ProductionPublicBaseURL)
		}
	}
	if strings.TrimSpace(c.MQTT.TopicPrefix) == ProductionMQTTPrefix {
		return errors.New("config: staging MQTT_TOPIC_PREFIX must not be the production prefix avf/devices")
	}
	return nil
}

func (c *Config) validateProduction(payment string) error {
	if strings.TrimSpace(c.Postgres.URL) == "" {
		return errors.New("config: APP_ENV=production requires DATABASE_URL")
	}
	if !c.ReadinessStrict {
		return errors.New("config: APP_ENV=production requires READINESS_STRICT=true")
	}
	host, err := hostFromDatabaseURL(c.Postgres.URL)
	if err != nil {
		return fmt.Errorf("config: DATABASE_URL: %w", err)
	}
	if isLocalDatabaseHost(host) {
		return errors.New("config: production DATABASE_URL must not use localhost/loopback")
	}
	if payment != PaymentEnvLive {
		return errors.New("config: APP_ENV=production requires PAYMENT_ENV=live")
	}
	if productionSandboxFamilyCommercePaymentProvider(c.Commerce.DefaultPaymentProvider) {
		k := strings.TrimSpace(c.Commerce.DefaultPaymentProvider)
		if k == "" {
			return errors.New("config: APP_ENV=production requires COMMERCE_PAYMENT_PROVIDER (live PSP registry key; empty is treated as sandbox at runtime)")
		}
		return fmt.Errorf("config: APP_ENV=production forbids COMMERCE_PAYMENT_PROVIDER=%q (mock/sandbox family)", strings.ToLower(k))
	}
	if pub := publicBaseURLForValidation(c); pub != ProductionPublicBaseURL {
		if pub == "" {
			return fmt.Errorf("config: APP_ENV=production requires PUBLIC_BASE_URL=%s (optionally set via APP_BASE_URL)", ProductionPublicBaseURL)
		}
		return fmt.Errorf("config: production PUBLIC_BASE_URL must be %s (got %q)", ProductionPublicBaseURL, c.Runtime.PublicBaseURL)
	}
	allowNS := getenvBool("PRODUCTION_ALLOW_NONSTANDARD_MQTT_TOPIC_PREFIX", false)
	tp := strings.TrimSpace(c.MQTT.TopicPrefix)
	if tp != "" && tp != ProductionMQTTPrefix && !allowNS {
		return errors.New("config: production MQTT_TOPIC_PREFIX must be avf/devices or set PRODUCTION_ALLOW_NONSTANDARD_MQTT_TOPIC_PREFIX=true (documented override)")
	}
	// If swagger enabled explicitly, require a production allowlist flag.
	_, swExplicit := os.LookupEnv("HTTP_SWAGGER_UI_ENABLED")
	if swExplicit && c.SwaggerUIEnabled {
		if !getenvBool("PRODUCTION_SWAGGER_UI_ALLOWED", false) {
			return errors.New("config: HTTP_SWAGGER_UI_ENABLED=true in production requires PRODUCTION_SWAGGER_UI_ALLOWED=true (documented override)")
		}
	}
	// OpenAPI JSON: explicit enable requires documented production approval (unset defaults off via loadOpenAPIJSONEnabled).
	_, oaExplicit := os.LookupEnv("HTTP_OPENAPI_JSON_ENABLED")
	if oaExplicit && c.OpenAPIJSONEnabled {
		if !getenvBool("PRODUCTION_OPENAPI_JSON_ALLOWED", false) {
			return errors.New("config: HTTP_OPENAPI_JSON_ENABLED=true in production requires PRODUCTION_OPENAPI_JSON_ALLOWED=true (documented override)")
		}
	}
	if !c.APIWiring.RequireOutboxPublisher {
		return errors.New("config: APP_ENV=production requires API_REQUIRE_OUTBOX_PUBLISHER=true (outbox publisher must be wired)")
	}
	if !c.APIWiring.RequireNATSRuntime {
		return errors.New("config: APP_ENV=production requires API_REQUIRE_NATS_RUNTIME=true (NATS/JetStream is mandatory)")
	}
	if mqttProductionRequiresMQTTPublisher(c.MQTT) && !c.APIWiring.RequireMQTTPublisher {
		return errors.New("config: APP_ENV=production with MQTT_CLIENT_ID_API set requires API_REQUIRE_MQTT_PUBLISHER=true (command publisher must be wired)")
	}
	if strings.TrimSpace(c.Redis.Addr) == "" && !getenvBool("PRODUCTION_ALLOW_MISSING_REDIS", false) {
		return errors.New("config: APP_ENV=production requires REDIS_ADDR or REDIS_URL (set PRODUCTION_ALLOW_MISSING_REDIS=true only for documented single-process or non-redis experiments)")
	}
	if !c.GRPC.RequireMachineJWT {
		return errors.New("config: APP_ENV=production requires GRPC_REQUIRE_MACHINE_JWT=true")
	}
	if !c.GRPC.RequireGRPCIdempotency {
		return errors.New("config: APP_ENV=production requires GRPC_REQUIRE_IDEMPOTENCY=true")
	}
	if mach := strings.TrimSpace(os.Getenv("MACHINE_GRPC_ENABLED")); mach != "true" {
		return errors.New("config: APP_ENV=production requires MACHINE_GRPC_ENABLED=true (set explicitly; GRPC_ENABLED alone is not sufficient)")
	}
	if !c.GRPC.Enabled {
		return errors.New("config: APP_ENV=production requires machine gRPC listener (MACHINE_GRPC_ENABLED=true)")
	}
	if c.TransportBoundary.MachineRESTLegacyEnabled && !c.TransportBoundary.MachineRESTLegacyAllowInProduction {
		return errors.New("config: APP_ENV=production with legacy machine HTTP enabled requires MACHINE_REST_LEGACY_ALLOW_IN_PRODUCTION=true (set ENABLE_LEGACY_MACHINE_HTTP or MACHINE_REST_LEGACY_ENABLED)")
	}
	mt := strings.ToLower(strings.TrimSpace(c.TransportBoundary.MQTTCommandTransport))
	if mt != "" && mt != "mqtt" {
		return fmt.Errorf("config: APP_ENV=production requires MQTT_COMMAND_TRANSPORT=mqtt (got %q)", c.TransportBoundary.MQTTCommandTransport)
	}
	if !c.MachineJWT.RequireAudience {
		return errors.New("config: APP_ENV=production requires MACHINE_AUTH_REQUIRE_AUDIENCE=true (machine JWT audience validation)")
	}
	machineMode := strings.ToLower(strings.TrimSpace(c.MachineJWT.Mode))
	if machineMode == "" || machineMode == "hs256" {
		if strings.TrimSpace(c.MachineJWT.ExpectedIssuer) == "" {
			return errors.New("config: APP_ENV=production requires AUTH_ISSUER or MACHINE_JWT_ISSUER when MACHINE_JWT_MODE is hs256 (empty issuer disables issuer validation)")
		}
	}
	if err := c.validateProductionInteractiveAuthModes(); err != nil {
		return err
	}
	if err := c.validateProductionSessionTTLs(); err != nil {
		return err
	}
	return nil
}

// validateProductionInteractiveAuthModes requires explicit operator-chosen JWT modes in production
// (no silent default HS256 from missing env).
func (c *Config) validateProductionInteractiveAuthModes() error {
	if c == nil {
		return errors.New("config: nil")
	}
	httpModeSet := envSetNonEmpty("HTTP_AUTH_MODE", "USER_JWT_MODE")
	if !httpModeSet {
		return errors.New("config: APP_ENV=production requires HTTP_AUTH_MODE or USER_JWT_MODE to be set explicitly (do not rely on implicit default hs256)")
	}
	if _, ok := os.LookupEnv("MACHINE_JWT_MODE"); !ok || strings.TrimSpace(os.Getenv("MACHINE_JWT_MODE")) == "" {
		return errors.New("config: APP_ENV=production requires MACHINE_JWT_MODE to be set explicitly")
	}
	return nil
}

func envSetNonEmpty(keys ...string) bool {
	for _, k := range keys {
		if v, ok := os.LookupEnv(k); ok && strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

func (c *Config) validateProductionSessionTTLs() error {
	if c == nil {
		return errors.New("config: nil")
	}
	access := c.HTTPAuth.AccessTokenTTL
	refresh := c.HTTPAuth.RefreshTokenTTL
	if access < 5*time.Minute || access > 24*time.Hour {
		return fmt.Errorf("config: production requires admin access token TTL (USER_JWT_ACCESS_TTL / HTTP_AUTH_ACCESS_TTL / ADMIN_SESSION_TTL) between 5m and 24h inclusive (got %s)", access)
	}
	if refresh < 1*time.Hour || refresh > 180*24*time.Hour {
		return fmt.Errorf("config: production requires admin refresh horizon (USER_JWT_REFRESH_TTL / HTTP_AUTH_REFRESH_TTL / ADMIN_REFRESH_TTL) between 1h and 180d inclusive (got %s)", refresh)
	}
	if access >= refresh {
		return errors.New("config: production requires access token TTL strictly less than refresh token TTL")
	}
	return nil
}

// mqttProductionRequiresMQTTPublisher is true when the broker is configured for API command publish
// (MQTT_CLIENT_ID_API). Ingest-only processes should use MQTT_CLIENT_ID_INGEST without MQTT_CLIENT_ID_API.
func mqttProductionRequiresMQTTPublisher(m MQTTConfig) bool {
	return strings.TrimSpace(m.BrokerURL) != "" && strings.TrimSpace(m.APIClientID) != ""
}

func (c *Config) validateDatabaseIdentitySeparation() error {
	if c.AppEnv == AppEnvStaging {
		prodRef := firstNonEmptyTrimmed(
			strings.TrimSpace(os.Getenv("PRODUCTION_DATABASE_HOST")),
			dbHostFromURLOrEmpty(os.Getenv("PRODUCTION_DATABASE_URL")),
		)
		if prodRef == "" {
			return nil
		}
		prodRef = strings.ToLower(prodRef)
		dbHost, err := hostFromDatabaseURL(c.Postgres.URL)
		if err != nil {
			return nil // URL shape already failed elsewhere; avoid duplicate
		}
		if dbHost == prodRef {
			return errors.New("config: staging DATABASE_URL host must not match PRODUCTION_DATABASE_HOST or host from PRODUCTION_DATABASE_URL")
		}
		// If full PRODUCTION_DATABASE_URL is set, also reject identical URLs (handled above) and
		// reject when staging URL string contains the production hostname as accidental paste.
	}
	if c.AppEnv == AppEnvProduction {
		stageRef := firstNonEmptyTrimmed(
			strings.TrimSpace(os.Getenv("STAGING_DATABASE_HOST")),
			dbHostFromURLOrEmpty(os.Getenv("STAGING_DATABASE_URL")),
		)
		if stageRef == "" {
			return nil
		}
		stageRef = strings.ToLower(stageRef)
		dbHost, err := hostFromDatabaseURL(c.Postgres.URL)
		if err != nil {
			return nil
		}
		if dbHost == stageRef {
			return errors.New("config: production DATABASE_URL host must not match STAGING_DATABASE_HOST or host from STAGING_DATABASE_URL")
		}
	}
	return nil
}

func dbHostFromURLOrEmpty(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	h, err := hostFromDatabaseURL(raw)
	if err != nil {
		return ""
	}
	return h
}

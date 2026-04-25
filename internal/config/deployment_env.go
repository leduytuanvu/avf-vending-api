package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
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
	return nil
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

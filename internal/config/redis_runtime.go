package config

import (
	"errors"
	"os"
	"strings"
	"time"
)

// RedisRuntimeFeatures gates optional Redis-backed behaviors in cmd/api (cache, access-token revocation, gRPC hot RPC limits).
// REDIS_ADDR remains the shared client address; these flags only enable features.
type RedisRuntimeFeatures struct {
	// CacheEnabled (CACHE_ENABLED) turns on hot-path caches (sale catalog snapshot) when Redis is available.
	CacheEnabled     bool
	RateLimitEnabled bool
	LocksEnabled     bool
	// SaleCatalogCacheTTL bounds cached JSON for sale catalog / media manifest projection.
	SaleCatalogCacheTTL time.Duration

	// AuthAccessJTIRevocationEnabled (AUTH_ACCESS_JTI_REVOCATION_ENABLED) rejects JWTs whose jti or subject was revoked in Redis.
	// Requires Redis in production unless AUTH_REVOCATION_REDIS_FAIL_OPEN=true (development-only escape hatch).
	AuthAccessJTIRevocationEnabled bool
	SessionCacheEnabled            bool
	LoginLockoutEnabled            bool
	// AuthRevocationRedisFailOpen (AUTH_REVOCATION_REDIS_FAIL_OPEN) allows requests if Redis errors during revocation checks (not allowed in production).
	AuthRevocationRedisFailOpen bool

	// GRPCMachineHotPerMinute (RATE_LIMIT_GRPC_MACHINE_HOT_PER_MIN) limits selected high-frequency machine RPCs per machine id (0 = default 900).
	GRPCMachineHotPerMinute int
}

// redisEnvConfigured reports whether REDIS_ADDR or REDIS_URL is non-empty (Redis client will be created).
func redisEnvConfigured() bool {
	return strings.TrimSpace(os.Getenv("REDIS_ADDR")) != "" || strings.TrimSpace(os.Getenv("REDIS_URL")) != ""
}

func loadRedisRuntimeFeatures(appEnv AppEnvironment) RedisRuntimeFeatures {
	grpcHot := getenvInt("RATE_LIMIT_GRPC_MACHINE_HOT_PER_MIN", 900)
	if grpcHot < 0 {
		grpcHot = 0
	}
	ttl := mustParseDuration("SALE_CATALOG_CACHE_TTL", getenv("SALE_CATALOG_CACHE_TTL", "45s"))
	failOpen := getenvBool("AUTH_REVOCATION_REDIS_FAIL_OPEN", false)
	if appEnv == AppEnvProduction {
		failOpen = false
	}
	// In staging/production with Redis configured, default Redis-backed cache/rate/session/lock behavior on.
	// Development stays opt-in so local stacks work without Redis or with READINESS_STRICT=false.
	deployedRedisDefaults := (appEnv == AppEnvStaging || appEnv == AppEnvProduction) && redisEnvConfigured()
	return RedisRuntimeFeatures{
		CacheEnabled:                   getenvBoolAlias(deployedRedisDefaults, "REDIS_CACHE_ENABLED", "CACHE_ENABLED"),
		RateLimitEnabled:               getenvBoolAlias(deployedRedisDefaults, "REDIS_RATE_LIMIT_ENABLED", "RATE_LIMIT_ENABLED"),
		LocksEnabled:                   getenvBoolAlias(deployedRedisDefaults, "REDIS_LOCKS_ENABLED"),
		SaleCatalogCacheTTL:            ttl,
		AuthAccessJTIRevocationEnabled: getenvBool("AUTH_ACCESS_JTI_REVOCATION_ENABLED", false),
		SessionCacheEnabled:            getenvBoolAlias(deployedRedisDefaults, "REDIS_SESSION_CACHE_ENABLED", "REDIS_CACHE_ENABLED"),
		LoginLockoutEnabled:            getenvBoolAlias(deployedRedisDefaults, "REDIS_LOGIN_LOCKOUT_ENABLED", "REDIS_RATE_LIMIT_ENABLED"),
		AuthRevocationRedisFailOpen:    failOpen,
		GRPCMachineHotPerMinute:        grpcHot,
	}
}

func (r RedisRuntimeFeatures) validate(c *Config) error {
	if c == nil {
		return errors.New("config: nil")
	}
	if r.CacheEnabled && r.SaleCatalogCacheTTL <= 0 {
		return errors.New("config: SALE_CATALOG_CACHE_TTL must be > 0 when CACHE_ENABLED=true")
	}
	if r.AuthAccessJTIRevocationEnabled {
		if c.AppEnv == AppEnvProduction && r.AuthRevocationRedisFailOpen {
			return errors.New("config: AUTH_REVOCATION_REDIS_FAIL_OPEN is not allowed when APP_ENV=production")
		}
		if strings.TrimSpace(c.Redis.Addr) == "" && restrictsRedisRevocation(c.AppEnv) {
			return errors.New("config: AUTH_ACCESS_JTI_REVOCATION_ENABLED requires REDIS_ADDR (or REDIS_URL) when APP_ENV is staging or production")
		}
	}
	if r.CacheEnabled && (c.AppEnv == AppEnvProduction || c.AppEnv == AppEnvStaging) && strings.TrimSpace(c.Redis.Addr) == "" {
		return errors.New("config: CACHE_ENABLED requires REDIS_ADDR (or REDIS_URL) when APP_ENV is staging or production")
	}
	if (r.RateLimitEnabled || r.SessionCacheEnabled || r.LoginLockoutEnabled || r.LocksEnabled) && (c.AppEnv == AppEnvProduction || c.AppEnv == AppEnvStaging) && strings.TrimSpace(c.Redis.Addr) == "" {
		return errors.New("config: enabled Redis runtime feature requires REDIS_ADDR (or REDIS_URL) when APP_ENV is staging or production")
	}
	return nil
}

func restrictsRedisRevocation(appEnv AppEnvironment) bool {
	switch appEnv {
	case AppEnvStaging, AppEnvProduction:
		return true
	default:
		return false
	}
}

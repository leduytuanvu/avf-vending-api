package redis

import (
	"github.com/avf/avf-vending-api/internal/config"
	goredis "github.com/redis/go-redis/v9"
)

// NewClient returns a Redis client when REDIS_ADDR is configured.
func NewClient(cfg *config.RedisConfig) (*goredis.Client, error) {
	if cfg == nil || !cfg.Enabled || cfg.Addr == "" {
		return nil, nil
	}

	opts := &goredis.Options{
		Addr:      cfg.Addr,
		Username:  cfg.Username,
		Password:  cfg.Password,
		DB:        cfg.DB,
		TLSConfig: cfg.TLSConfig(),
	}

	c := goredis.NewClient(opts)
	return c, nil
}

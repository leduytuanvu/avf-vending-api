// Package ratelimit provides fixed-window counters for HTTP abuse protection (Redis when available,
// in-memory fallback for single-node development).
package ratelimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Backend implements a fixed-window request counter per logical key.
type Backend interface {
	// Allow increments usage for key and reports whether the request is allowed.
	// limit is the maximum requests allowed per window; window is the TTL / reset period for the counter.
	Allow(ctx context.Context, key string, limit int64, window time.Duration) (allowed bool, retryAfter time.Duration)
}

// StableKey returns a short hex digest suitable for Redis keys (avoids unbounded or secret-bearing keys).
func StableKey(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte{0})
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))[:40]
}

// MemoryBackend is an in-process fixed-window limiter (not suitable for multi-instance production).
type MemoryBackend struct {
	mu sync.Mutex
	m  map[string]*memBucket
}

type memBucket struct {
	count   int64
	resetAt time.Time
}

// NewMemoryBackend constructs an empty in-memory limiter.
func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{m: make(map[string]*memBucket)}
}

// Allow implements Backend.
func (m *MemoryBackend) Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, time.Duration) {
	if limit <= 0 || window <= 0 {
		return true, 0
	}
	select {
	case <-ctx.Done():
		return false, 0
	default:
	}

	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	b := m.m[key]
	if b == nil || !now.Before(b.resetAt) {
		b = &memBucket{count: 0, resetAt: now.Add(window)}
		m.m[key] = b
	}
	b.count++
	if b.count > limit {
		retry := time.Until(b.resetAt)
		if retry < time.Second {
			retry = time.Second
		}
		return false, retry
	}
	return true, 0
}

// RedisBackend stores counters in Redis with rolling TTL (fixed window approximation via EXPIRE).
type RedisBackend struct {
	cli *goredis.Client
}

// NewRedisBackend wraps a go-redis client for distributed counters.
func NewRedisBackend(cli *goredis.Client) (*RedisBackend, error) {
	if cli == nil {
		return nil, errors.New("ratelimit: nil redis client")
	}
	return &RedisBackend{cli: cli}, nil
}

const redisKeyPrefix = "avf:rl:v1:"

// luaFixedWindow increments key and sets TTL on first increment; returns {allowed flag, pttl ms}.
const luaFixedWindow = `
local n = tonumber(redis.call('INCR', KEYS[1]))
local ttlms = tonumber(ARGV[1])
local lim = tonumber(ARGV[2])
if n == 1 then
  redis.call('PEXPIRE', KEYS[1], ttlms)
end
local pttl = tonumber(redis.call('PTTL', KEYS[1]))
if pttl < 0 then
  pttl = 0
end
if n > lim then
  return {0, pttl}
end
return {1, pttl}
`

// Allow implements Backend.
func (r *RedisBackend) Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, time.Duration) {
	if limit <= 0 || window <= 0 {
		return true, 0
	}
	k := redisKeyPrefix + key
	ms := window.Milliseconds()
	vals, err := r.cli.Eval(ctx, luaFixedWindow, []string{k}, ms, limit).Slice()
	if err != nil {
		return true, 0
	}
	if len(vals) != 2 {
		return true, 0
	}
	allowed := asInt64(vals[0])
	pttl := asInt64(vals[1])
	if allowed == 1 {
		return true, 0
	}
	retry := time.Duration(pttl) * time.Millisecond
	if retry < time.Second {
		retry = time.Second
	}
	return false, retry
}

func asInt64(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	default:
		return 0
	}
}

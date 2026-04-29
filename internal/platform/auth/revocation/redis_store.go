package revocation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	keyJTI = "avf:v1:auth:revoke:jti:"
	keySub = "avf:v1:auth:revoke:sub:"
)

// RedisStore persists revocation markers with TTL.
type RedisStore struct {
	c *goredis.Client
}

// NewRedisStore requires a non-nil go-redis client.
func NewRedisStore(c *goredis.Client) (*RedisStore, error) {
	if c == nil {
		return nil, errors.New("revocation: nil redis client")
	}
	return &RedisStore{c: c}, nil
}

func digest(s string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(s)))
	return hex.EncodeToString(h[:16])
}

// RevokeJTI implements Store.
func (r *RedisStore) RevokeJTI(ctx context.Context, jti string, ttl time.Duration) error {
	if r == nil || r.c == nil {
		return errors.New("revocation: nil store")
	}
	j := strings.TrimSpace(jti)
	if j == "" || ttl <= 0 {
		return nil
	}
	return r.c.Set(ctx, keyJTI+digest(j), "1", ttl).Err()
}

// IsJTIRevoked implements Store.
func (r *RedisStore) IsJTIRevoked(ctx context.Context, jti string) (bool, error) {
	if r == nil || r.c == nil {
		return false, errors.New("revocation: nil store")
	}
	j := strings.TrimSpace(jti)
	if j == "" {
		return false, nil
	}
	n, err := r.c.Exists(ctx, keyJTI+digest(j)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// RevokeSubject implements Store.
func (r *RedisStore) RevokeSubject(ctx context.Context, subject string, ttl time.Duration) error {
	if r == nil || r.c == nil {
		return errors.New("revocation: nil store")
	}
	s := strings.TrimSpace(subject)
	if s == "" || ttl <= 0 {
		return nil
	}
	return r.c.Set(ctx, keySub+digest(s), "1", ttl).Err()
}

// IsSubjectRevoked implements Store.
func (r *RedisStore) IsSubjectRevoked(ctx context.Context, subject string) (bool, error) {
	if r == nil || r.c == nil {
		return false, errors.New("revocation: nil store")
	}
	s := strings.TrimSpace(subject)
	if s == "" {
		return false, nil
	}
	n, err := r.c.Exists(ctx, keySub+digest(s)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

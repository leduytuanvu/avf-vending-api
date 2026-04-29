package redis

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

type LoginFailureCounter struct {
	c      *goredis.Client
	prefix string
}

var _ plauth.LoginFailureCounter = (*LoginFailureCounter)(nil)

func NewLoginFailureCounter(c *goredis.Client, prefix string) *LoginFailureCounter {
	if c == nil {
		return nil
	}
	return &LoginFailureCounter{c: c, prefix: prefix}
}

func (l *LoginFailureCounter) PeekFailureCount(ctx context.Context, orgID uuid.UUID, email string) (int64, error) {
	if l == nil || l.c == nil {
		return 0, errors.New("redis: nil login failure counter")
	}
	if orgID == uuid.Nil || strings.TrimSpace(email) == "" {
		return 0, nil
	}
	k := l.key(orgID, email)
	n, err := l.c.Get(ctx, k).Int64()
	if err == goredis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (l *LoginFailureCounter) IncrementFailure(ctx context.Context, orgID uuid.UUID, email string, threshold int32, ttl time.Duration) (bool, int64, error) {
	if l == nil || l.c == nil {
		return false, 0, errors.New("redis: nil login failure counter")
	}
	if orgID == uuid.Nil || strings.TrimSpace(email) == "" || threshold <= 0 || ttl <= 0 {
		return false, 0, nil
	}
	k := l.key(orgID, email)
	n, err := l.c.Incr(ctx, k).Result()
	if err != nil {
		return false, 0, err
	}
	if n == 1 {
		_ = l.c.Expire(ctx, k, ttl).Err()
	}
	return n >= int64(threshold), n, nil
}

func (l *LoginFailureCounter) ClearFailures(ctx context.Context, orgID uuid.UUID, email string) error {
	if l == nil || l.c == nil {
		return errors.New("redis: nil login failure counter")
	}
	if orgID == uuid.Nil || strings.TrimSpace(email) == "" {
		return nil
	}
	return l.c.Del(ctx, l.key(orgID, email)).Err()
}

func (l *LoginFailureCounter) key(orgID uuid.UUID, email string) string {
	return key(l.prefix, "auth", "login_fail", orgID.String(), digest(strings.ToLower(strings.TrimSpace(email))))
}

type memoryLoginFailure struct {
	count     int64
	expiresAt time.Time
}

type MemoryLoginFailureCounter struct {
	mu sync.Mutex
	m  map[string]memoryLoginFailure
}

func NewMemoryLoginFailureCounter() *MemoryLoginFailureCounter {
	return &MemoryLoginFailureCounter{m: make(map[string]memoryLoginFailure)}
}

func (m *MemoryLoginFailureCounter) PeekFailureCount(_ context.Context, orgID uuid.UUID, email string) (int64, error) {
	if orgID == uuid.Nil || strings.TrimSpace(email) == "" {
		return 0, nil
	}
	k := orgID.String() + ":" + strings.ToLower(strings.TrimSpace(email))
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	cur := m.m[k]
	if now.After(cur.expiresAt) {
		return 0, nil
	}
	return cur.count, nil
}

func (m *MemoryLoginFailureCounter) IncrementFailure(_ context.Context, orgID uuid.UUID, email string, threshold int32, ttl time.Duration) (bool, int64, error) {
	if orgID == uuid.Nil || strings.TrimSpace(email) == "" || threshold <= 0 || ttl <= 0 {
		return false, 0, nil
	}
	k := orgID.String() + ":" + strings.ToLower(strings.TrimSpace(email))
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	cur := m.m[k]
	if now.After(cur.expiresAt) {
		cur = memoryLoginFailure{expiresAt: now.Add(ttl)}
	}
	cur.count++
	m.m[k] = cur
	return cur.count >= int64(threshold), cur.count, nil
}

func (m *MemoryLoginFailureCounter) ClearFailures(_ context.Context, orgID uuid.UUID, email string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.m, orgID.String()+":"+strings.ToLower(strings.TrimSpace(email)))
	return nil
}

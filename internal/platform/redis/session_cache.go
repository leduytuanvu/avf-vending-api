package redis

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

// refreshRevokedTTL bounds tombstones for revoked refresh hashes (covers max refresh JWT horizons).
const refreshRevokedTTL = 720 * time.Hour

type refreshSessionRecord struct {
	AccountID string    `json:"account_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

type RefreshSessionCache struct {
	c      *goredis.Client
	prefix string
}

var _ plauth.RefreshSessionCache = (*RefreshSessionCache)(nil)

func NewRefreshSessionCache(c *goredis.Client, prefix string) *RefreshSessionCache {
	if c == nil {
		return nil
	}
	return &RefreshSessionCache{c: c, prefix: prefix}
}

func (s *RefreshSessionCache) PutRefreshSession(ctx context.Context, tokenHash []byte, accountID uuid.UUID, expiresAt time.Time) error {
	if s == nil || s.c == nil {
		return errors.New("redis: nil refresh session cache")
	}
	if len(tokenHash) == 0 || accountID == uuid.Nil {
		return nil
	}
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return nil
	}
	b, err := json.Marshal(refreshSessionRecord{AccountID: accountID.String(), ExpiresAt: expiresAt.UTC()})
	if err != nil {
		return err
	}
	pipe := s.c.Pipeline()
	pipe.Del(ctx, s.revokedKey(tokenHash))
	pipe.Set(ctx, s.refreshKey(tokenHash), b, ttl)
	pipe.SAdd(ctx, s.accountKey(accountID), digestBytes(tokenHash))
	pipe.Expire(ctx, s.accountKey(accountID), ttl)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RefreshSessionCache) InvalidateRefreshSession(ctx context.Context, tokenHash []byte) error {
	if s == nil || s.c == nil {
		return errors.New("redis: nil refresh session cache")
	}
	if len(tokenHash) == 0 {
		return nil
	}
	pipe := s.c.Pipeline()
	pipe.Set(ctx, s.revokedKey(tokenHash), "1", refreshRevokedTTL)
	pipe.Del(ctx, s.refreshKey(tokenHash))
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RefreshSessionCache) InvalidateAccountSessions(ctx context.Context, accountID uuid.UUID) error {
	if s == nil || s.c == nil {
		return errors.New("redis: nil refresh session cache")
	}
	if accountID == uuid.Nil {
		return nil
	}
	setKey := s.accountKey(accountID)
	members, err := s.c.SMembers(ctx, setKey).Result()
	if err != nil {
		return err
	}
	if len(members) == 0 {
		return s.c.Del(ctx, setKey).Err()
	}
	pipe := s.c.Pipeline()
	for _, m := range members {
		if strings.TrimSpace(m) == "" {
			continue
		}
		pipe.Set(ctx, key(s.prefix, "auth", "refresh_revoked", sanitizePart(m)), "1", refreshRevokedTTL)
		pipe.Del(ctx, key(s.prefix, "auth", "refresh", sanitizePart(m)))
	}
	pipe.Del(ctx, setKey)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RefreshSessionCache) refreshKey(tokenHash []byte) string {
	return key(s.prefix, "auth", "refresh", digestBytes(tokenHash))
}

func (s *RefreshSessionCache) revokedKey(tokenHash []byte) string {
	return key(s.prefix, "auth", "refresh_revoked", digestBytes(tokenHash))
}

// IsRefreshRevoked reports whether the refresh hash was revoked (logout / password change / admin revoke).
func (s *RefreshSessionCache) IsRefreshRevoked(ctx context.Context, tokenHash []byte) bool {
	if s == nil || s.c == nil || len(tokenHash) == 0 {
		return false
	}
	n, err := s.c.Exists(ctx, s.revokedKey(tokenHash)).Result()
	return err == nil && n > 0
}

func (s *RefreshSessionCache) accountKey(accountID uuid.UUID) string {
	return key(s.prefix, "auth", "account_sessions", accountID.String())
}

type MemoryRefreshSessionCache struct {
	mu       sync.Mutex
	sessions map[string]refreshSessionRecord
	accounts map[uuid.UUID]map[string]struct{}
	revoked  map[string]struct{}
}

func NewMemoryRefreshSessionCache() *MemoryRefreshSessionCache {
	return &MemoryRefreshSessionCache{
		sessions: make(map[string]refreshSessionRecord),
		accounts: make(map[uuid.UUID]map[string]struct{}),
		revoked:  make(map[string]struct{}),
	}
}

func (m *MemoryRefreshSessionCache) PutRefreshSession(_ context.Context, tokenHash []byte, accountID uuid.UUID, expiresAt time.Time) error {
	if len(tokenHash) == 0 || accountID == uuid.Nil || !expiresAt.After(time.Now()) {
		return nil
	}
	k := digestBytes(tokenHash)
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.revoked, k)
	m.sessions[k] = refreshSessionRecord{AccountID: accountID.String(), ExpiresAt: expiresAt.UTC()}
	if m.accounts[accountID] == nil {
		m.accounts[accountID] = make(map[string]struct{})
	}
	m.accounts[accountID][k] = struct{}{}
	return nil
}

func (m *MemoryRefreshSessionCache) InvalidateRefreshSession(_ context.Context, tokenHash []byte) error {
	k := digestBytes(tokenHash)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revoked[k] = struct{}{}
	if rec, ok := m.sessions[k]; ok {
		if aid, err := uuid.Parse(rec.AccountID); err == nil && m.accounts[aid] != nil {
			delete(m.accounts[aid], k)
		}
	}
	delete(m.sessions, k)
	return nil
}

func (m *MemoryRefreshSessionCache) InvalidateAccountSessions(_ context.Context, accountID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k := range m.accounts[accountID] {
		m.revoked[k] = struct{}{}
		delete(m.sessions, k)
	}
	delete(m.accounts, accountID)
	return nil
}

func (m *MemoryRefreshSessionCache) IsRefreshRevoked(_ context.Context, tokenHash []byte) bool {
	k := digestBytes(tokenHash)
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.revoked[k]
	return ok
}

func (m *MemoryRefreshSessionCache) Has(tokenHash []byte) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.sessions[digestBytes(tokenHash)]
	return ok
}

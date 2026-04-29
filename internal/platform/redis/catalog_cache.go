package redis

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

type CatalogCache interface {
	Get(ctx context.Context, organizationID, machineID uuid.UUID, version string) ([]byte, bool, error)
	Set(ctx context.Context, organizationID, machineID uuid.UUID, version string, payload []byte, ttl time.Duration) error
	InvalidateMachine(ctx context.Context, organizationID, machineID uuid.UUID) error
}

type RedisCatalogCache struct {
	c      *goredis.Client
	prefix string
}

func NewRedisCatalogCache(c *goredis.Client, prefix string) *RedisCatalogCache {
	if c == nil {
		return nil
	}
	return &RedisCatalogCache{c: c, prefix: prefix}
}

func (c *RedisCatalogCache) Get(ctx context.Context, orgID, machineID uuid.UUID, version string) ([]byte, bool, error) {
	if c == nil || c.c == nil {
		return nil, false, errors.New("redis: nil catalog cache")
	}
	b, err := c.c.Get(ctx, c.cacheKey(orgID, machineID, version)).Bytes()
	if err == goredis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

func (c *RedisCatalogCache) Set(ctx context.Context, orgID, machineID uuid.UUID, version string, payload []byte, ttl time.Duration) error {
	if c == nil || c.c == nil {
		return errors.New("redis: nil catalog cache")
	}
	if orgID == uuid.Nil || machineID == uuid.Nil || version == "" || ttl <= 0 {
		return nil
	}
	k := c.cacheKey(orgID, machineID, version)
	pipe := c.c.Pipeline()
	pipe.Set(ctx, k, payload, ttl)
	pipe.SAdd(ctx, c.machineIndexKey(orgID, machineID), k)
	pipe.Expire(ctx, c.machineIndexKey(orgID, machineID), ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *RedisCatalogCache) InvalidateMachine(ctx context.Context, orgID, machineID uuid.UUID) error {
	if c == nil || c.c == nil {
		return errors.New("redis: nil catalog cache")
	}
	idx := c.machineIndexKey(orgID, machineID)
	keys, err := c.c.SMembers(ctx, idx).Result()
	if err != nil {
		return err
	}
	keys = append(keys, idx)
	if len(keys) == 0 {
		return nil
	}
	return c.c.Del(ctx, keys...).Err()
}

func (c *RedisCatalogCache) cacheKey(orgID, machineID uuid.UUID, version string) string {
	return key(c.prefix, "catalog", orgID.String(), machineID.String(), digest(version))
}

func (c *RedisCatalogCache) machineIndexKey(orgID, machineID uuid.UUID) string {
	return key(c.prefix, "catalog_index", orgID.String(), machineID.String())
}

type MemoryCatalogCache struct {
	mu    sync.Mutex
	items map[string][]byte
	index map[string]map[string]struct{}
}

func NewMemoryCatalogCache() *MemoryCatalogCache {
	return &MemoryCatalogCache{items: make(map[string][]byte), index: make(map[string]map[string]struct{})}
}

func (m *MemoryCatalogCache) Get(_ context.Context, orgID, machineID uuid.UUID, version string) ([]byte, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := orgID.String() + ":" + machineID.String() + ":" + version
	b, ok := m.items[k]
	return append([]byte(nil), b...), ok, nil
}

func (m *MemoryCatalogCache) Set(_ context.Context, orgID, machineID uuid.UUID, version string, payload []byte, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := orgID.String() + ":" + machineID.String() + ":" + version
	idx := orgID.String() + ":" + machineID.String()
	m.items[k] = append([]byte(nil), payload...)
	if m.index[idx] == nil {
		m.index[idx] = make(map[string]struct{})
	}
	m.index[idx][k] = struct{}{}
	return nil
}

func (m *MemoryCatalogCache) InvalidateMachine(_ context.Context, orgID, machineID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := orgID.String() + ":" + machineID.String()
	for k := range m.index[idx] {
		delete(m.items, k)
	}
	delete(m.index, idx)
	return nil
}

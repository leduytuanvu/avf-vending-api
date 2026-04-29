package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

var ErrLockNotAcquired = errors.New("redis: lock not acquired")

type Lock struct {
	Key   string
	Owner string
}

type Locker interface {
	Acquire(ctx context.Context, key string, ttl time.Duration) (Lock, error)
	Release(ctx context.Context, lock Lock) (bool, error)
}

type RedisLocker struct {
	c      *goredis.Client
	prefix string
}

func NewRedisLocker(c *goredis.Client, prefix string) *RedisLocker {
	if c == nil {
		return nil
	}
	return &RedisLocker{c: c, prefix: prefix}
}

func (l *RedisLocker) Acquire(ctx context.Context, rawKey string, ttl time.Duration) (Lock, error) {
	if l == nil || l.c == nil {
		return Lock{}, errors.New("redis: nil locker")
	}
	if ttl <= 0 {
		return Lock{}, errors.New("redis: lock ttl must be > 0")
	}
	owner, err := randomOwnerToken()
	if err != nil {
		return Lock{}, err
	}
	k := key(l.prefix, "lock", sanitizePart(rawKey))
	ok, err := l.c.SetNX(ctx, k, owner, ttl).Result()
	if err != nil {
		return Lock{}, err
	}
	if !ok {
		return Lock{}, ErrLockNotAcquired
	}
	return Lock{Key: k, Owner: owner}, nil
}

const releaseIfOwnerLua = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`

func (l *RedisLocker) Release(ctx context.Context, lock Lock) (bool, error) {
	if l == nil || l.c == nil {
		return false, errors.New("redis: nil locker")
	}
	k := strings.TrimSpace(lock.Key)
	owner := strings.TrimSpace(lock.Owner)
	if k == "" || owner == "" {
		return false, nil
	}
	n, err := l.c.Eval(ctx, releaseIfOwnerLua, []string{k}, owner).Int64()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

func randomOwnerToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

type memoryLock struct {
	owner     string
	expiresAt time.Time
}

type MemoryLocker struct {
	mu     sync.Mutex
	prefix string
	locks  map[string]memoryLock
}

func NewMemoryLocker(prefix string) *MemoryLocker {
	return &MemoryLocker{prefix: prefix, locks: make(map[string]memoryLock)}
}

func (l *MemoryLocker) Acquire(_ context.Context, rawKey string, ttl time.Duration) (Lock, error) {
	if ttl <= 0 {
		return Lock{}, errors.New("redis: lock ttl must be > 0")
	}
	owner, err := randomOwnerToken()
	if err != nil {
		return Lock{}, err
	}
	k := key(l.prefix, "lock", sanitizePart(rawKey))
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if cur, ok := l.locks[k]; ok && now.Before(cur.expiresAt) {
		return Lock{}, ErrLockNotAcquired
	}
	l.locks[k] = memoryLock{owner: owner, expiresAt: now.Add(ttl)}
	return Lock{Key: k, Owner: owner}, nil
}

func (l *MemoryLocker) Release(_ context.Context, lock Lock) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	cur, ok := l.locks[lock.Key]
	if !ok || cur.owner != lock.Owner {
		return false, nil
	}
	delete(l.locks, lock.Key)
	return true, nil
}

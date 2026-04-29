package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStableKey_deterministic(t *testing.T) {
	t.Parallel()
	a := StableKey("login", "a@b.com", "127.0.0.1")
	b := StableKey("login", "a@b.com", "127.0.0.1")
	c := StableKey("login", "b@b.com", "127.0.0.1")
	require.Equal(t, a, b)
	require.NotEqual(t, a, c)
}

func TestMemoryBackend_fixedWindow(t *testing.T) {
	t.Parallel()
	m := NewMemoryBackend()
	ctx := context.Background()
	key := "k1"
	win := 500 * time.Millisecond
	limit := int64(2)

	ok1, _ := m.Allow(ctx, key, limit, win)
	ok2, _ := m.Allow(ctx, key, limit, win)
	ok3, retry := m.Allow(ctx, key, limit, win)
	require.True(t, ok1)
	require.True(t, ok2)
	require.False(t, ok3)
	require.Greater(t, retry, time.Duration(0))

	time.Sleep(win + 50*time.Millisecond)
	ok4, _ := m.Allow(ctx, key, limit, win)
	require.True(t, ok4)
}

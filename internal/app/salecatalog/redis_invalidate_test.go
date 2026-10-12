package salecatalog

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestInvalidateRedisSnapshotCache_deletesHeadAndVersionedKeys(t *testing.T) {
	t.Parallel()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	mid := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	headKey := saleCatalogRedisHeadKey(mid, false, true)
	versioned := saleCatalogRedisCacheKey(mid, 1, 2, 3, false, true)
	require.NoError(t, rdb.Set(ctx, headKey, "{}", time.Minute).Err())
	require.NoError(t, rdb.Set(ctx, versioned, "{}", time.Minute).Err())

	InvalidateRedisSnapshotCache(ctx, rdb, mid)

	require.False(t, mr.Exists(headKey))
	require.False(t, mr.Exists(versioned))
}

func TestSaleCatalogHeadEntry_matchesVersions(t *testing.T) {
	t.Parallel()
	head := saleCatalogHeadEntry{ShadowVer: 1, MaxCfgRev: 2, MediaEpoch: 3}
	require.True(t, saleCatalogHeadMatchesVersions(head, 1, 2, 3))
	require.False(t, saleCatalogHeadMatchesVersions(head, 2, 2, 3))
}

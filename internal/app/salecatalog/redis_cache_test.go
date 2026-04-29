package salecatalog

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type stubInnerBuilder struct {
	called int
	snap   Snapshot
	err    error
}

func (s *stubInnerBuilder) BuildSnapshot(ctx context.Context, machineID uuid.UUID, opts Options) (Snapshot, error) {
	s.called++
	out := s.snap
	if out.MachineID == uuid.Nil {
		out.MachineID = machineID
	}
	return out, s.err
}

func TestSaleCatalogRedisCache_keyChangesWithConfigRevision(t *testing.T) {
	t.Parallel()
	mid := uuid.MustParse("88888888-8888-8888-8888-888888888888")
	k1 := saleCatalogRedisCacheKey(mid, 3, 1, 0, false, true)
	k2 := saleCatalogRedisCacheKey(mid, 3, 2, 0, false, true)
	require.NotEqual(t, k1, k2)
}

func TestRedisCachedSnapshotBuilder_nilRedisDelegatesToInner(t *testing.T) {
	t.Parallel()
	mid := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	inner := &stubInnerBuilder{snap: Snapshot{MachineID: mid}}
	c := RedisCachedSnapshotBuilder{Inner: inner, RDB: nil, TTL: 0}
	snap, err := c.BuildSnapshot(context.Background(), mid, Options{})
	require.NoError(t, err)
	require.Equal(t, mid, snap.MachineID)
	require.Equal(t, 1, inner.called)
}

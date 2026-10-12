package salecatalog

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSnapshotMediaURLsStillValid_skipsWhenAllFresh(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	exp := now.Add(5 * time.Minute)
	snap := Snapshot{
		Items: []Item{
			{Image: &ImageMeta{URLExpiresAt: exp}},
		},
	}
	require.True(t, snapshotMediaURLsStillValid(&snap, now))
}

func TestSnapshotMediaURLsStillValid_falseWhenExpired(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	snap := Snapshot{
		Items: []Item{
			{Image: &ImageMeta{URLExpiresAt: now.Add(30 * time.Second)}},
		},
	}
	require.False(t, snapshotMediaURLsStillValid(&snap, now))
}

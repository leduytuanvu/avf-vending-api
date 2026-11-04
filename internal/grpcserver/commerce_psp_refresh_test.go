package grpcserver

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestShouldRunOrderStatusPSPRefresh_graceSkipsWithoutRecording(t *testing.T) {
	t.Parallel()
	orderID := uuid.New()
	createdAt := time.Date(2026, 11, 5, 2, 0, 0, 0, time.UTC)
	now := createdAt.Add(2 * time.Second)

	require.False(t, shouldRunOrderStatusPSPRefresh(orderID, createdAt, now))
	require.True(t, shouldRunOrderStatusPSPRefresh(orderID, createdAt, createdAt.Add(3*time.Second)))
}

func TestShouldRunOrderStatusPSPRefresh_hotCooldownFiveSeconds(t *testing.T) {
	t.Parallel()
	orderID := uuid.New()
	createdAt := time.Date(2026, 11, 5, 2, 0, 0, 0, time.UTC)
	first := createdAt.Add(4 * time.Second)

	require.True(t, shouldRunOrderStatusPSPRefresh(orderID, createdAt, first))
	require.False(t, shouldRunOrderStatusPSPRefresh(orderID, createdAt, first.Add(4*time.Second)))
	require.True(t, shouldRunOrderStatusPSPRefresh(orderID, createdAt, first.Add(5*time.Second)))
}

func TestShouldRunOrderStatusPSPRefresh_coldCooldownFifteenSeconds(t *testing.T) {
	t.Parallel()
	orderID := uuid.New()
	createdAt := time.Date(2026, 11, 5, 2, 0, 0, 0, time.UTC)
	first := createdAt.Add(2*time.Minute + time.Second)

	require.True(t, shouldRunOrderStatusPSPRefresh(orderID, createdAt, first))
	require.False(t, shouldRunOrderStatusPSPRefresh(orderID, createdAt, first.Add(14*time.Second)))
	require.True(t, shouldRunOrderStatusPSPRefresh(orderID, createdAt, first.Add(15*time.Second)))
}

func TestShouldRunOrderStatusPSPRefresh_rejectsNilOrder(t *testing.T) {
	t.Parallel()
	now := time.Now()
	require.False(t, shouldRunOrderStatusPSPRefresh(uuid.Nil, now.Add(-time.Minute), now))
}

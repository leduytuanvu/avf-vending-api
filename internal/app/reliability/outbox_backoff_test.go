package reliability_test

import (
	"testing"
	"time"

	appreliability "github.com/avf/avf-vending-api/internal/app/reliability"
	"github.com/stretchr/testify/require"
)

func TestOutboxPublishBackoffAfterFailure_Deterministic(t *testing.T) {
	base := time.Second
	max := 10 * time.Second
	require.Equal(t, time.Second, appreliability.OutboxPublishBackoffAfterFailure(1, base, max))
	require.Equal(t, 2*time.Second, appreliability.OutboxPublishBackoffAfterFailure(2, base, max))
	require.Equal(t, 4*time.Second, appreliability.OutboxPublishBackoffAfterFailure(3, base, max))
	require.Equal(t, 8*time.Second, appreliability.OutboxPublishBackoffAfterFailure(4, base, max))
	require.Equal(t, 10*time.Second, appreliability.OutboxPublishBackoffAfterFailure(5, base, max))
	require.Equal(t, 10*time.Second, appreliability.OutboxPublishBackoffAfterFailure(100, base, max))
}

package clockskew

import (
	"errors"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestValidateCheckIn_rejectsFutureAndPast(t *testing.T) {
	cfg := config.DeviceClockSkewConfig{
		CheckInFutureMax: 5 * time.Minute,
		CheckInPastMax:   time.Hour,
	}
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

	err := ValidateCheckIn(now.Add(10*time.Minute), now, cfg)
	var v Violation
	require.True(t, errors.As(err, &v))
	require.Equal(t, ReasonFuture, v.Reason)

	err = ValidateCheckIn(now.Add(-2*time.Hour), now, cfg)
	require.True(t, errors.As(err, &v))
	require.Equal(t, ReasonPast, v.Reason)

	require.NoError(t, ValidateCheckIn(now.Add(-30*time.Minute), now, cfg))
}

func TestValidateOfflineEvent_rejectsImplausibleYear(t *testing.T) {
	cfg := config.DeviceClockSkewConfig{OfflineMinYear: 2020, OfflineFutureMax: 5 * time.Minute}
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	epoch := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	err := ValidateOfflineEvent(epoch, now, time.Time{}, cfg)
	var v Violation
	require.True(t, errors.As(err, &v))
	require.Equal(t, ReasonImplausible, v.Reason)
}

func TestDriftSeconds_positiveWhenEventInPast(t *testing.T) {
	occurred := time.Date(2026, 9, 1, 11, 0, 0, 0, time.UTC)
	received := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	require.InDelta(t, 3600, DriftSeconds(occurred, received), 0.001)
}

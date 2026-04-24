package telemetry

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestStableCriticalIdempotencyKey(t *testing.T) {
	t.Parallel()
	mid := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	boot := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	seq := int64(7)

	t.Run("dedupe_key", func(t *testing.T) {
		t.Parallel()
		dk := "device-chosen-key"
		got, err := StableCriticalIdempotencyKey(mid, "events.vend", CriticalIngestIdentity{DedupeKey: &dk})
		require.NoError(t, err)
		require.Equal(t, "device-chosen-key", got)
	})

	t.Run("event_id", func(t *testing.T) {
		t.Parallel()
		got, err := StableCriticalIdempotencyKey(mid, "events.vend", CriticalIngestIdentity{EventID: "evt-1"})
		require.NoError(t, err)
		require.Equal(t, mid.String()+":events.vend:evt-1", got)
	})

	t.Run("boot_seq_includes_event_type", func(t *testing.T) {
		t.Parallel()
		got, err := StableCriticalIdempotencyKey(mid, "payment.captured", CriticalIngestIdentity{BootID: &boot, SeqNo: &seq})
		require.NoError(t, err)
		require.Equal(t, mid.String()+":"+boot.String()+":7:payment.captured", got)
	})

	t.Run("missing", func(t *testing.T) {
		t.Parallel()
		_, err := StableCriticalIdempotencyKey(mid, "events.vend", CriticalIngestIdentity{})
		require.ErrorIs(t, err, ErrCriticalTelemetryMissingIdentity)
	})

	t.Run("nil_machine", func(t *testing.T) {
		t.Parallel()
		_, err := StableCriticalIdempotencyKey(uuid.Nil, "events.vend", CriticalIngestIdentity{EventID: "x"})
		require.True(t, err != nil && !errors.Is(err, ErrCriticalTelemetryMissingIdentity))
	})
}

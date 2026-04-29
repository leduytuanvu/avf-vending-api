package postgres_test

import (
	"context"
	"testing"

	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/stretchr/testify/require"
)

func TestMessagingConsumerDeduper_TryClaim_requiresFields(t *testing.T) {
	ctx := context.Background()
	d := postgres.NewMessagingConsumerDeduper(nil)
	_, err := d.TryClaim(ctx, "", "s", "m")
	require.Error(t, err)
	_, err = d.TryClaim(ctx, "c", "", "m")
	require.Error(t, err)
	_, err = d.TryClaim(ctx, "c", "s", "")
	require.Error(t, err)
}

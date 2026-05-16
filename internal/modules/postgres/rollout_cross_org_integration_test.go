package postgres_test

import (
	"context"
	"testing"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestRollout_GetCampaignByID_SingleCompany(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	q := db.New(pool)
	row, err := q.InsertRolloutCampaign(ctx, db.InsertRolloutCampaignParams{
		RolloutType:   "config_version",
		TargetVersion: "v-cross-org",
		Status:        "pending",
		Strategy:      []byte(`{"confirm_full_rollout":true}`),
		CreatedBy:     pgtype.UUID{},
	})
	require.NoError(t, err)
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM rollout_campaigns WHERE id = $1`, row.ID)
	}()

	_, err = q.GetRolloutCampaignByID(ctx, row.ID)
	require.NoError(t, err)
}

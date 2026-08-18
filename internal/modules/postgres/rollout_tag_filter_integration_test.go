package postgres_test

import (
	"context"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"testing"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/pgxutil"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRollout_TagFilter_SelectsIntersection(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	q := pgxutil.NewQueries(pool)

	slugA := "rollout-tf-a-" + uuid.NewString()
	slugB := "rollout-tf-b-" + uuid.NewString()
	tagA := id.NewUUIDV7()
	tagB := id.NewUUIDV7()
	_, err := pool.Exec(ctx, `INSERT INTO tags (id, slug, name) VALUES ($1,$2,'A'), ($3,$4,'B')`, tagA, slugA, tagB, slugB)
	require.NoError(t, err)
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM machine_tag_assignments WHERE tag_id = ANY($1)`, []uuid.UUID{tagA, tagB})
		_, _ = pool.Exec(ctx, `DELETE FROM tags WHERE id = ANY($1)`, []uuid.UUID{tagA, tagB})
	}()

	m1 := testfixtures.DevMachineID
	hw := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	m2 := id.NewUUIDV7()
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, site_id, hardware_profile_id, serial_number, name, status, command_sequence, credential_version)
VALUES ($1, $2, $3, $4, 'rollout-tf-b', 'online', 0, 0)`,
		m2, testfixtures.DevSiteID, hw, "sn-rollout-tf-"+m2.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM machines WHERE id = $1`, m2)
	})

	_, err = pool.Exec(ctx, `INSERT INTO machine_tag_assignments (machine_id, tag_id) VALUES ($1,$2), ($1,$3), ($4,$2)`,
		m1, tagA, tagB, m2)
	require.NoError(t, err)

	machines, err := q.RolloutListMachineIDsWithAllTags(ctx, db.RolloutListMachineIDsWithAllTagsParams{
		TagIds:        []uuid.UUID{tagA, tagB},
		RequiredCount: 2,
	})
	require.NoError(t, err)
	require.Len(t, machines, 1)
	require.Equal(t, m1, machines[0])
}

package postgres_test

import (
	"context"
	"testing"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRollout_TagFilter_SelectsIntersection(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	q := db.New(pool)
	org := testfixtures.DevOrganizationID

	slugA := "rollout-tf-a-" + uuid.NewString()
	slugB := "rollout-tf-b-" + uuid.NewString()
	tagA := uuid.New()
	tagB := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO tags (id, organization_id, slug, name) VALUES ($1,$2,$3,'A'), ($4,$2,$5,'B')`, tagA, org, slugA, tagB, slugB)
	require.NoError(t, err)
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM machine_tag_assignments WHERE tag_id = ANY($1)`, []uuid.UUID{tagA, tagB})
		_, _ = pool.Exec(ctx, `DELETE FROM tags WHERE id = ANY($1)`, []uuid.UUID{tagA, tagB})
	}()

	m1 := testfixtures.DevMachineID
	var m2 uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT id FROM machines WHERE organization_id = $1 AND id <> $2 LIMIT 1`, org, m1).Scan(&m2))

	_, err = pool.Exec(ctx, `INSERT INTO machine_tag_assignments (organization_id, machine_id, tag_id) VALUES ($1,$2,$3), ($1,$2,$4), ($1,$5,$3)`,
		org, m1, tagA, tagB, m2)
	require.NoError(t, err)

	machines, err := q.RolloutListMachineIDsWithAllTags(ctx, db.RolloutListMachineIDsWithAllTagsParams{
		OrganizationID: org,
		TagIds:         []uuid.UUID{tagA, tagB},
		RequiredCount:  2,
	})
	require.NoError(t, err)
	require.Len(t, machines, 1)
	require.Equal(t, m1, machines[0])
}

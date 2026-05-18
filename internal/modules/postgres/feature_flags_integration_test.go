package postgres_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/avf/avf-vending-api/internal/app/featureflags"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestFeatureFlags_siteTargetOverridesMasterDisabled(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := db.New(pool)
	svc, err := featureflags.NewService(q, pool, nil)
	require.NoError(t, err)

	key := "test.flag." + uuid.NewString()
	f, err := svc.CreateFlag(ctx, featureflags.CreateFlagParams{
		FlagKey:     key,
		DisplayName: "integration",
		Description: "test",
		Enabled:     false,
		Metadata:    json.RawMessage("{}"),
	})
	require.NoError(t, err)

	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM feature_flag_targets WHERE feature_flag_id = $1`, f.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM feature_flags WHERE id = $1`, f.ID)
	}()

	sid := testfixtures.DevSiteID
	_, err = svc.ReplaceTargets(ctx, featureflags.PutTargetsParams{
		FlagID: f.ID,
		Targets: []featureflags.TargetInput{
			{
				TargetType: "site",
				SiteID:     &sid,
				Priority:   10,
				Enabled:    true,
			},
		},
	})
	require.NoError(t, err)

	m, err := svc.ResolveEffectiveFlags(ctx, testfixtures.DevMachineID)
	require.NoError(t, err)
	require.True(t, m[key])
}

func TestMachineConfig_versionAndRolloutRoundTrip(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := db.New(pool)
	svc, err := featureflags.NewService(q, pool, nil)
	require.NoError(t, err)

	label := "v-test-" + uuid.NewString()
	ver, err := svc.CreateMachineConfigVersion(ctx, featureflags.CreateMachineConfigVersionParams{
		VersionLabel:  label,
		ConfigPayload: json.RawMessage(`{"k":true}`),
	})
	require.NoError(t, err)

	roll, err := svc.CreateRollout(ctx, featureflags.CreateRolloutParams{
		TargetVersionID:    ver.ID,
		Status:             "pending",
		RolloutTargetLevel: "global",
		Metadata:           json.RawMessage("{}"),
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM machine_config_rollouts WHERE id = $1`, roll.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM machine_config_versions WHERE id = $1`, ver.ID)
	})

	got, err := svc.GetRollout(ctx, uuid.Nil, roll.ID)
	require.NoError(t, err)
	require.Equal(t, roll.ID, got.ID)
	require.Equal(t, ver.ID, got.TargetVersionID)
}

func TestMachineConfig_rolloutRollbackCreatesPendingPrevious(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := db.New(pool)
	svc, err := featureflags.NewService(q, pool, nil)
	require.NoError(t, err)

	v1, err := svc.CreateMachineConfigVersion(ctx, featureflags.CreateMachineConfigVersionParams{
		VersionLabel:  "rb-a-" + uuid.NewString(),
		ConfigPayload: json.RawMessage(`{}`),
	})
	require.NoError(t, err)
	v2, err := svc.CreateMachineConfigVersion(ctx, featureflags.CreateMachineConfigVersionParams{
		VersionLabel:    "rb-b-" + uuid.NewString(),
		ConfigPayload:   json.RawMessage(`{"step":2}`),
		ParentVersionID: &v1.ID,
	})
	require.NoError(t, err)

	r1, err := svc.CreateRollout(ctx, featureflags.CreateRolloutParams{
		TargetVersionID:    v2.ID,
		PreviousVersionID:  &v1.ID,
		Status:             "pending",
		RolloutTargetLevel: "global",
		Metadata:           json.RawMessage("{}"),
	})
	require.NoError(t, err)

	rb, err := svc.RollbackRollout(ctx, uuid.Nil, r1.ID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM machine_config_rollouts WHERE id IN ($1, $2)`, r1.ID, rb.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM machine_config_versions WHERE id IN ($1, $2)`, v1.ID, v2.ID)
	})
	require.Equal(t, v1.ID, rb.TargetVersionID)
	require.NotNil(t, rb.PreviousVersionID)
	require.Equal(t, v2.ID, *rb.PreviousVersionID)
}

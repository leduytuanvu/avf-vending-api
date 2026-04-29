package rollout

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMergeRolloutDesired_AppVersion(t *testing.T) {
	t.Parallel()
	b, err := mergeRolloutDesired(nil, "app_version", "2.3.1")
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	require.Equal(t, "2.3.1", m["app_version"])
}

func TestMergeRolloutDesired_UnknownType(t *testing.T) {
	t.Parallel()
	_, err := mergeRolloutDesired(nil, "not_a_type", "x")
	require.Error(t, err)
}

func TestApplyRolloutSelection_FullRequiresConfirm(t *testing.T) {
	t.Parallel()
	cand := []uuid.UUID{uuid.MustParse("00000000-0000-4000-8000-000000000001"), uuid.MustParse("00000000-0000-4000-8000-000000000002")}
	_, err := applyRolloutSelection(cand, Strategy{})
	require.ErrorIs(t, err, ErrInvalidArgument)
	out, err := applyRolloutSelection(cand, Strategy{ConfirmFullRollout: true})
	require.NoError(t, err)
	require.Len(t, out, 2)
}

func TestApplyRolloutSelection_CanarySubset(t *testing.T) {
	t.Parallel()
	cand := make([]uuid.UUID, 100)
	for i := range cand {
		var b [16]byte
		b[15] = byte(i + 1)
		b[6] = 0x40
		b[8] = 0x80
		cand[i] = uuid.UUID(b)
	}
	p := 10.0
	out, err := applyRolloutSelection(cand, Strategy{CanaryPercent: &p})
	require.NoError(t, err)
	require.Equal(t, 10, len(out))
}

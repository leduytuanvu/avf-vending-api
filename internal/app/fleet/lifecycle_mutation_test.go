package fleet_test

import (
	"testing"

	appfleet "github.com/avf/avf-vending-api/internal/app/fleet"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestValidateLifecycleMutation_requiresReason(t *testing.T) {
	err := appfleet.ValidateLifecycleMutation("suspend", appfleet.LifecycleMutationInput{}, false)
	require.ErrorIs(t, err, appfleet.ErrLifecycleReasonRequired)

	err = appfleet.ValidateLifecycleMutation("suspend", appfleet.LifecycleMutationInput{Reason: "maintenance"}, false)
	require.NoError(t, err)
}

func TestValidateLifecycleMutation_technicianRequiresOperatorSession(t *testing.T) {
	err := appfleet.ValidateLifecycleMutation("suspend", appfleet.LifecycleMutationInput{Reason: "x"}, true)
	require.ErrorIs(t, err, appfleet.ErrOperatorSessionRequired)

	sid := uuid.New()
	err = appfleet.ValidateLifecycleMutation("suspend", appfleet.LifecycleMutationInput{Reason: "x", OperatorSessionID: &sid}, true)
	require.NoError(t, err)
}

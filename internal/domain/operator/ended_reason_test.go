package operator_test

import (
	"testing"

	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
	"github.com/stretchr/testify/require"
)

func TestNormalizeEndedReason(t *testing.T) {
	require.Equal(t, domainoperator.EndedReasonSupersededBySameOperator, domainoperator.NormalizeEndedReason("stale_session_reclaimed"))
	require.Equal(t, domainoperator.EndedReasonClientLogout, domainoperator.NormalizeEndedReason("client_logout"))
	require.Equal(t, domainoperator.EndedReasonUnknown, domainoperator.NormalizeEndedReason(""))
	require.Equal(t, domainoperator.EndedReasonUnknown, domainoperator.NormalizeEndedReason("random_legacy"))
}

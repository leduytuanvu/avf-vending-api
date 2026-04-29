package machineruntime

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestOfflineSequenceOutOfOrder_retryableCode(t *testing.T) {
	t.Parallel()
	err := OfflineSequenceOutOfOrder(2, 5)
	require.Equal(t, codes.Aborted, status.Code(err))
	require.Contains(t, err.Error(), "expected 2 got 5")
}

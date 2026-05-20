package payments

import (
	"context"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminService_ListPaymentReconciliationDrift_guardClauses(t *testing.T) {
	t.Parallel()
	var s *AdminService
	_, err := s.ListPaymentReconciliationDrift(context.Background(), id.NewUUIDV7(), 3600, 10)
	require.Error(t, err)

	s = &AdminService{}
	_, err = s.ListPaymentReconciliationDrift(context.Background(), id.NewUUIDV7(), 3600, 10)
	require.Error(t, err)
}

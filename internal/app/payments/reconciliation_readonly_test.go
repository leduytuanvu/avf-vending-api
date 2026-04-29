package payments

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAdminService_ListPaymentReconciliationDrift_guardClauses(t *testing.T) {
	t.Parallel()
	var s *AdminService
	_, err := s.ListPaymentReconciliationDrift(context.Background(), uuid.New(), 3600, 10)
	require.Error(t, err)

	s = &AdminService{}
	_, err = s.ListPaymentReconciliationDrift(context.Background(), uuid.New(), 3600, 10)
	require.Error(t, err)
}

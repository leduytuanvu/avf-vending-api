package correctness

import (
	"context"
	"testing"

	"github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestP06_E2E_Reconciliation_paidVendFailedCaseIsIdempotent(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := postgres.NewCommerceReconcileRepository(pool)

	orderID := uuid.New()
	in := commerce.ReconciliationCaseInput{
		OrganizationID: testfixtures.DevOrganizationID,
		CaseType:       "payment_paid_vend_failed",
		Severity:       "critical",
		OrderID:        &orderID,
		Reason:         "p06 e2e: payment terminal + vend failure",
		Metadata:       []byte(`{"source":"p06_e2e"}`),
	}

	r1, err := repo.UpsertReconciliationCase(ctx, in)
	require.NoError(t, err)

	r2, err := repo.UpsertReconciliationCase(ctx, in)
	require.NoError(t, err)
	require.Equal(t, r1.ID, r2.ID)
}

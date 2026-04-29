package postgres_test

import (
	"context"
	"sync"
	"testing"

	"github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUpsertReconciliationCase_idempotentUnderConcurrentRuns(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := postgres.NewCommerceReconcileRepository(pool)

	in := commerce.ReconciliationCaseInput{
		OrganizationID: testfixtures.DevOrganizationID,
		CaseType:       "payment_paid_vend_not_started",
		Severity:       "critical",
		Reason:         "test concurrent upsert",
		Metadata:       []byte(`{"test":true}`),
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	ids := make(chan uuid.UUID, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			row, err := repo.UpsertReconciliationCase(ctx, in)
			if err != nil {
				errs <- err
				return
			}
			ids <- row.ID
		}()
	}
	wg.Wait()
	close(errs)
	close(ids)
	for err := range errs {
		require.NoError(t, err)
	}
	var got []uuid.UUID
	for id := range ids {
		got = append(got, id)
	}
	require.Len(t, got, 2)
	require.Equal(t, got[0], got[1])
}

package commerce

import (
	"context"
	"testing"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type resolvePaymentFinancialStub struct {
	refreshPaymentsFinancialStub
	winner    domaincommerce.Payment
	winnerErr error
}

func (s *resolvePaymentFinancialStub) GetWinningPaymentForOrder(context.Context, uuid.UUID) (domaincommerce.Payment, error) {
	if s.winnerErr != nil {
		return domaincommerce.Payment{}, s.winnerErr
	}
	if s.winner.ID != uuid.Nil {
		return s.winner, nil
	}
	return domaincommerce.Payment{}, ErrNotFound
}

func TestResolveAuthoritativePayment_prefersWinner(t *testing.T) {
	t.Parallel()
	orderID := uuid.New()
	winner := domaincommerce.Payment{ID: uuid.New(), OrderID: orderID, State: "captured"}
	latest := domaincommerce.Payment{ID: uuid.New(), OrderID: orderID, State: "created", CreatedAt: time.Now()}
	svc := NewService(Deps{
		OrderVend: stubOrderVendWorkflow{},
		SaleLines: stubSaleLineResolver{},
		Lifecycle: &orderStatusLifecycleStub{payment: latest},
		FinancialCorrectness: &resolvePaymentFinancialStub{
			winner: winner,
			refreshPaymentsFinancialStub: refreshPaymentsFinancialStub{
				payments: []domaincommerce.Payment{winner, latest},
			},
		},
	})

	got, err := svc.resolveAuthoritativePayment(context.Background(), orderID)
	require.NoError(t, err)
	require.Equal(t, winner.ID, got.ID)
}

func TestResolveAuthoritativePayment_prefersOldestCapturedWhenWinnerMissing(t *testing.T) {
	t.Parallel()
	orderID := uuid.New()
	olderCaptured := domaincommerce.Payment{
		ID:        uuid.New(),
		OrderID:   orderID,
		State:     "captured",
		CreatedAt: time.Now().Add(-2 * time.Minute),
	}
	newerCreated := domaincommerce.Payment{
		ID:        uuid.New(),
		OrderID:   orderID,
		State:     "created",
		CreatedAt: time.Now(),
	}
	svc := NewService(Deps{
		OrderVend: stubOrderVendWorkflow{},
		SaleLines: stubSaleLineResolver{},
		Lifecycle: &orderStatusLifecycleStub{payment: newerCreated},
		FinancialCorrectness: &resolvePaymentFinancialStub{
			winnerErr: ErrNotFound,
			refreshPaymentsFinancialStub: refreshPaymentsFinancialStub{
				payments: []domaincommerce.Payment{olderCaptured, newerCreated},
			},
		},
	})

	got, err := svc.resolveAuthoritativePayment(context.Background(), orderID)
	require.NoError(t, err)
	require.Equal(t, olderCaptured.ID, got.ID)
}

func TestResolveAuthoritativePayment_latestCapturedOnly(t *testing.T) {
	t.Parallel()
	orderID := uuid.New()
	captured := domaincommerce.Payment{
		ID:        uuid.New(),
		OrderID:   orderID,
		State:     "captured",
		CreatedAt: time.Now().Add(-time.Minute),
	}
	svc := NewService(Deps{
		OrderVend: stubOrderVendWorkflow{},
		SaleLines: stubSaleLineResolver{},
		Lifecycle: &orderStatusLifecycleStub{payment: captured},
		FinancialCorrectness: &resolvePaymentFinancialStub{
			winnerErr: ErrNotFound,
			refreshPaymentsFinancialStub: refreshPaymentsFinancialStub{
				payments: []domaincommerce.Payment{captured},
			},
		},
	})

	got, err := svc.resolveAuthoritativePayment(context.Background(), orderID)
	require.NoError(t, err)
	require.Equal(t, captured.ID, got.ID)
}

func TestResolveAuthoritativePayment_noCapturedFallsBackToLatest(t *testing.T) {
	t.Parallel()
	orderID := uuid.New()
	older := domaincommerce.Payment{
		ID:        uuid.New(),
		OrderID:   orderID,
		State:     "created",
		CreatedAt: time.Now().Add(-time.Minute),
	}
	latest := domaincommerce.Payment{
		ID:        uuid.New(),
		OrderID:   orderID,
		State:     "created",
		CreatedAt: time.Now(),
	}
	svc := NewService(Deps{
		OrderVend: stubOrderVendWorkflow{},
		SaleLines: stubSaleLineResolver{},
		Lifecycle: &orderStatusLifecycleStub{payment: latest},
		FinancialCorrectness: &resolvePaymentFinancialStub{
			winnerErr: ErrNotFound,
			refreshPaymentsFinancialStub: refreshPaymentsFinancialStub{
				payments: []domaincommerce.Payment{older, latest},
			},
		},
	})

	got, err := svc.resolveAuthoritativePayment(context.Background(), orderID)
	require.NoError(t, err)
	require.Equal(t, latest.ID, got.ID)
}

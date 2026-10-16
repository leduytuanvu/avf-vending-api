package commerce

import (
	"context"
	"testing"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type markOrderUnknownLife struct {
	orderStatusLifecycleStub
}

func (s *markOrderUnknownLife) UpdateOrderStatus(_ context.Context, orderID, _ uuid.UUID, status string) (domaincommerce.Order, error) {
	if s.order.ID != orderID {
		return domaincommerce.Order{}, ErrNotFound
	}
	s.order.Status = status
	return s.order, nil
}

func TestMarkOrderUnknown_FromVending(t *testing.T) {
	orderID := uuid.Must(uuid.NewV7())
	life := &markOrderUnknownLife{
		orderStatusLifecycleStub: orderStatusLifecycleStub{
			order: domaincommerce.Order{ID: orderID, Status: "vending"},
		},
	}
	svc := &Service{life: life}
	out, err := svc.MarkOrderUnknown(context.Background(), uuid.Nil, orderID, "crash_recovery")
	require.NoError(t, err)
	require.Equal(t, "unknown", out.Status)
}

func TestMarkOrderUnknown_Idempotent(t *testing.T) {
	orderID := uuid.Must(uuid.NewV7())
	life := &markOrderUnknownLife{
		orderStatusLifecycleStub: orderStatusLifecycleStub{
			order: domaincommerce.Order{ID: orderID, Status: "unknown"},
		},
	}
	svc := &Service{life: life}
	out, err := svc.MarkOrderUnknown(context.Background(), uuid.Nil, orderID, "crash_recovery")
	require.NoError(t, err)
	require.Equal(t, "unknown", out.Status)
}

func TestMarkOrderUnknown_TerminalRejected(t *testing.T) {
	orderID := uuid.Must(uuid.NewV7())
	life := &markOrderUnknownLife{
		orderStatusLifecycleStub: orderStatusLifecycleStub{
			order: domaincommerce.Order{ID: orderID, Status: "completed"},
		},
	}
	svc := &Service{life: life}
	_, err := svc.MarkOrderUnknown(context.Background(), uuid.Nil, orderID, "crash_recovery")
	require.ErrorIs(t, err, ErrIllegalTransition)
}

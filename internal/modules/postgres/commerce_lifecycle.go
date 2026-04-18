package postgres

import (
	"context"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

var _ appcommerce.CommerceLifecycleStore = (*Store)(nil)

func (s *Store) GetOrderByID(ctx context.Context, orderID uuid.UUID) (domaincommerce.Order, error) {
	row, err := db.New(s.pool).GetOrderByID(ctx, orderID)
	if err != nil {
		if isNoRows(err) {
			return domaincommerce.Order{}, appcommerce.ErrNotFound
		}
		return domaincommerce.Order{}, err
	}
	return mapOrder(row), nil
}

func (s *Store) UpdateOrderStatus(ctx context.Context, orderID, organizationID uuid.UUID, status string) (domaincommerce.Order, error) {
	row, err := db.New(s.pool).UpdateOrderStatusByOrg(ctx, db.UpdateOrderStatusByOrgParams{
		ID:             orderID,
		OrganizationID: organizationID,
		Status:         status,
	})
	if err != nil {
		if isNoRows(err) {
			return domaincommerce.Order{}, appcommerce.ErrNotFound
		}
		return domaincommerce.Order{}, err
	}
	return mapOrder(row), nil
}

func (s *Store) GetVendSessionByOrderAndSlot(ctx context.Context, orderID uuid.UUID, slotIndex int32) (domaincommerce.VendSession, error) {
	row, err := db.New(s.pool).GetVendSessionByOrderAndSlot(ctx, db.GetVendSessionByOrderAndSlotParams{
		OrderID:   orderID,
		SlotIndex: slotIndex,
	})
	if err != nil {
		if isNoRows(err) {
			return domaincommerce.VendSession{}, appcommerce.ErrNotFound
		}
		return domaincommerce.VendSession{}, err
	}
	return mapVend(row), nil
}

func (s *Store) UpdateVendSessionState(ctx context.Context, p appcommerce.UpdateVendSessionParams) (domaincommerce.VendSession, error) {
	row, err := db.New(s.pool).UpdateVendSessionStateByOrderSlot(ctx, db.UpdateVendSessionStateByOrderSlotParams{
		OrderID:       p.OrderID,
		SlotIndex:     p.SlotIndex,
		State:         p.ToState,
		FailureReason: p.FailureReason,
	})
	if err != nil {
		if isNoRows(err) {
			return domaincommerce.VendSession{}, appcommerce.ErrNotFound
		}
		return domaincommerce.VendSession{}, err
	}
	return mapVend(row), nil
}

func (s *Store) GetLatestPaymentForOrder(ctx context.Context, orderID uuid.UUID) (domaincommerce.Payment, error) {
	row, err := db.New(s.pool).GetLatestPaymentForOrder(ctx, orderID)
	if err != nil {
		if isNoRows(err) {
			return domaincommerce.Payment{}, appcommerce.ErrNotFound
		}
		return domaincommerce.Payment{}, err
	}
	return mapPayment(row), nil
}

func (s *Store) GetPaymentByID(ctx context.Context, paymentID uuid.UUID) (domaincommerce.Payment, error) {
	row, err := db.New(s.pool).GetPaymentByID(ctx, paymentID)
	if err != nil {
		if isNoRows(err) {
			return domaincommerce.Payment{}, appcommerce.ErrNotFound
		}
		return domaincommerce.Payment{}, err
	}
	return mapPayment(row), nil
}

func (s *Store) InsertPaymentAttempt(ctx context.Context, in appcommerce.InsertPaymentAttemptParams) (appcommerce.PaymentAttemptView, error) {
	if in.Payload == nil {
		in.Payload = []byte("{}")
	}
	row, err := db.New(s.pool).InsertPaymentAttempt(ctx, db.InsertPaymentAttemptParams{
		PaymentID:         in.PaymentID,
		ProviderReference: in.ProviderReference,
		State:             in.State,
		Payload:           in.Payload,
	})
	if err != nil {
		return appcommerce.PaymentAttemptView{}, err
	}
	return mapPaymentAttemptView(row), nil
}

func mapPaymentAttemptView(row db.PaymentAttempt) appcommerce.PaymentAttemptView {
	return appcommerce.PaymentAttemptView{
		ID:        row.ID,
		PaymentID: row.PaymentID,
		State:     row.State,
		CreatedAt: row.CreatedAt,
	}
}

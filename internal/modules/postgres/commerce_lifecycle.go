package postgres

import (
	"context"
	"strings"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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
	var fr pgtype.Text
	if p.FailureReason != nil {
		fr = pgtype.Text{String: *p.FailureReason, Valid: true}
	}
	row, err := db.New(s.pool).UpdateVendSessionStateByOrderSlot(ctx, db.UpdateVendSessionStateByOrderSlotParams{
		OrderID:       p.OrderID,
		SlotIndex:     p.SlotIndex,
		State:         p.ToState,
		FailureReason: fr,
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
	var pref pgtype.Text
	if in.ProviderReference != nil {
		pref = pgtype.Text{String: *in.ProviderReference, Valid: true}
	}
	row, err := db.New(s.pool).InsertPaymentAttempt(ctx, db.InsertPaymentAttemptParams{
		PaymentID:         in.PaymentID,
		ProviderReference: pref,
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

func (s *Store) InsertRefundRow(ctx context.Context, in appcommerce.InsertRefundRowInput) (appcommerce.RefundRowView, error) {
	meta := in.Metadata
	if len(meta) == 0 {
		meta = []byte("{}")
	}
	var reason pgtype.Text
	if strings.TrimSpace(in.Reason) != "" {
		reason = pgtype.Text{String: strings.TrimSpace(in.Reason), Valid: true}
	}
	var idem pgtype.Text
	if strings.TrimSpace(in.IdempotencyKey) != "" {
		idem = pgtype.Text{String: strings.TrimSpace(in.IdempotencyKey), Valid: true}
	}
	row, err := db.New(s.pool).InsertRefundRow(ctx, db.InsertRefundRowParams{
		PaymentID:      in.PaymentID,
		OrderID:        in.OrderID,
		AmountMinor:    in.AmountMinor,
		Currency:       strings.ToUpper(strings.TrimSpace(in.Currency)),
		State:          in.State,
		Reason:         reason,
		IdempotencyKey: idem,
		Metadata:       meta,
	})
	if err != nil {
		return appcommerce.RefundRowView{}, err
	}
	return mapRefundRowView(row), nil
}

func (s *Store) ListRefundsForOrder(ctx context.Context, orderID uuid.UUID) ([]appcommerce.RefundRowView, error) {
	rows, err := db.New(s.pool).ListRefundsForOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	out := make([]appcommerce.RefundRowView, 0, len(rows))
	for _, r := range rows {
		out = append(out, mapRefundListRow(r))
	}
	return out, nil
}

func (s *Store) GetRefundByIDForOrder(ctx context.Context, orderID, refundID uuid.UUID) (appcommerce.RefundRowView, error) {
	row, err := db.New(s.pool).GetRefundByIDForOrder(ctx, db.GetRefundByIDForOrderParams{
		ID:      refundID,
		OrderID: orderID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return appcommerce.RefundRowView{}, appcommerce.ErrNotFound
		}
		return appcommerce.RefundRowView{}, err
	}
	return mapRefundGetRow(row), nil
}

func (s *Store) GetRefundByOrderIdempotency(ctx context.Context, orderID uuid.UUID, idempotencyKey string) (appcommerce.RefundRowView, error) {
	row, err := db.New(s.pool).GetRefundByOrderIdempotency(ctx, db.GetRefundByOrderIdempotencyParams{
		OrderID:        orderID,
		IdempotencyKey: pgtype.Text{String: strings.TrimSpace(idempotencyKey), Valid: strings.TrimSpace(idempotencyKey) != ""},
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return appcommerce.RefundRowView{}, appcommerce.ErrNotFound
		}
		return appcommerce.RefundRowView{}, err
	}
	return mapRefundIdemRow(row), nil
}

func (s *Store) SumNonFailedRefundAmountForPayment(ctx context.Context, paymentID uuid.UUID) (int64, error) {
	return db.New(s.pool).SumNonFailedRefundAmountForPayment(ctx, paymentID)
}

func mapRefundRowView(row db.InsertRefundRowRow) appcommerce.RefundRowView {
	return appcommerce.RefundRowView{
		ID:             row.ID,
		PaymentID:      row.PaymentID,
		OrderID:        row.OrderID,
		AmountMinor:    row.AmountMinor,
		Currency:       row.Currency,
		State:          row.State,
		Reason:         textPtr(row.Reason),
		IdempotencyKey: textPtr(row.IdempotencyKey),
		Metadata:       row.Metadata,
		CreatedAt:      row.CreatedAt,
	}
}

func mapRefundListRow(row db.ListRefundsForOrderRow) appcommerce.RefundRowView {
	return appcommerce.RefundRowView{
		ID:             row.ID,
		PaymentID:      row.PaymentID,
		OrderID:        row.OrderID,
		AmountMinor:    row.AmountMinor,
		Currency:       row.Currency,
		State:          row.State,
		Reason:         textPtr(row.Reason),
		IdempotencyKey: textPtr(row.IdempotencyKey),
		Metadata:       row.Metadata,
		CreatedAt:      row.CreatedAt,
	}
}

func mapRefundGetRow(row db.GetRefundByIDForOrderRow) appcommerce.RefundRowView {
	return appcommerce.RefundRowView{
		ID:             row.ID,
		PaymentID:      row.PaymentID,
		OrderID:        row.OrderID,
		AmountMinor:    row.AmountMinor,
		Currency:       row.Currency,
		State:          row.State,
		Reason:         textPtr(row.Reason),
		IdempotencyKey: textPtr(row.IdempotencyKey),
		Metadata:       row.Metadata,
		CreatedAt:      row.CreatedAt,
	}
}

func mapRefundIdemRow(row db.GetRefundByOrderIdempotencyRow) appcommerce.RefundRowView {
	return appcommerce.RefundRowView{
		ID:             row.ID,
		PaymentID:      row.PaymentID,
		OrderID:        row.OrderID,
		AmountMinor:    row.AmountMinor,
		Currency:       row.Currency,
		State:          row.State,
		Reason:         textPtr(row.Reason),
		IdempotencyKey: textPtr(row.IdempotencyKey),
		Metadata:       row.Metadata,
		CreatedAt:      row.CreatedAt,
	}
}

func textPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

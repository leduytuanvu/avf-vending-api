package commerce

import (
	"context"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/workfloworch"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type auditRecorderSpy struct {
	last compliance.EnterpriseAuditRecord
}

func (a *auditRecorderSpy) Record(_ context.Context, in compliance.EnterpriseAuditRecord) error {
	a.last = in
	return nil
}

func (a *auditRecorderSpy) RecordCritical(_ context.Context, in compliance.EnterpriseAuditRecord) error {
	a.last = in
	return nil
}

func (a *auditRecorderSpy) RecordCriticalTx(_ context.Context, _ pgx.Tx, in compliance.EnterpriseAuditRecord) error {
	a.last = in
	return nil
}

type refundLifecycleStub struct {
	order   domaincommerce.Order
	payment domaincommerce.Payment
	rid     uuid.UUID
}

func (s *refundLifecycleStub) GetOrderByID(context.Context, uuid.UUID) (domaincommerce.Order, error) {
	return s.order, nil
}

func (s *refundLifecycleStub) UpdateOrderStatus(context.Context, uuid.UUID, uuid.UUID, string) (domaincommerce.Order, error) {
	return domaincommerce.Order{}, ErrNotConfigured
}

func (s *refundLifecycleStub) GetVendSessionByOrderAndSlot(context.Context, uuid.UUID, int32) (domaincommerce.VendSession, error) {
	return domaincommerce.VendSession{}, ErrNotFound
}

func (s *refundLifecycleStub) UpdateVendSessionState(context.Context, UpdateVendSessionParams) (domaincommerce.VendSession, error) {
	return domaincommerce.VendSession{}, ErrNotConfigured
}

func (s *refundLifecycleStub) GetLatestPaymentForOrder(context.Context, uuid.UUID) (domaincommerce.Payment, error) {
	return s.payment, nil
}

func (s *refundLifecycleStub) GetPaymentByID(context.Context, uuid.UUID) (domaincommerce.Payment, error) {
	return s.payment, nil
}

func (s *refundLifecycleStub) InsertPaymentAttempt(context.Context, InsertPaymentAttemptParams) (PaymentAttemptView, error) {
	return PaymentAttemptView{}, ErrNotConfigured
}

func (s *refundLifecycleStub) InsertRefundRow(_ context.Context, in InsertRefundRowInput) (RefundRowView, error) {
	id := s.rid
	if id == uuid.Nil {
		id = uuid.New()
	}
	reason := in.Reason
	ik := in.IdempotencyKey
	return RefundRowView{
		ID:             id,
		PaymentID:      in.PaymentID,
		OrderID:        in.OrderID,
		AmountMinor:    in.AmountMinor,
		Currency:       in.Currency,
		State:          in.State,
		Reason:         &reason,
		IdempotencyKey: &ik,
		Metadata:       in.Metadata,
		CreatedAt:      time.Now().UTC(),
	}, nil
}

func (s *refundLifecycleStub) ListRefundsForOrder(context.Context, uuid.UUID) ([]RefundRowView, error) {
	return nil, nil
}

func (s *refundLifecycleStub) GetRefundByIDForOrder(context.Context, uuid.UUID, uuid.UUID) (RefundRowView, error) {
	return RefundRowView{}, ErrNotFound
}

func (s *refundLifecycleStub) GetRefundByOrderIdempotency(context.Context, uuid.UUID, string) (RefundRowView, error) {
	return RefundRowView{}, ErrNotFound
}

func (s *refundLifecycleStub) SumNonFailedRefundAmountForPayment(context.Context, uuid.UUID) (int64, error) {
	return 0, nil
}

func (s *refundLifecycleStub) FulfillSuccessfulVendAtomically(context.Context, FulfillSuccessfulVendInput) (FulfillSuccessfulVendResult, error) {
	return FulfillSuccessfulVendResult{}, ErrNotConfigured
}

func (s *refundLifecycleStub) FulfillFailedVendAtomically(context.Context, FulfillFailedVendInput) (FulfillFailedVendResult, error) {
	return FulfillFailedVendResult{}, ErrNotConfigured
}

func TestCreateRefund_recordsEnterpriseAudit(t *testing.T) {
	t.Parallel()
	org := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	orderID := uuid.New()
	payID := uuid.New()
	refundID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	rec := &auditRecorderSpy{}
	svc := NewService(Deps{
		OrderVend:     stubOrderVendWorkflow{},
		PaymentOutbox: nil,
		Lifecycle: &refundLifecycleStub{
			order: domaincommerce.Order{
				ID:             orderID,
				OrganizationID: org,
				Status:         "completed",
				MachineID:      uuid.New(),
				Currency:       "USD",
				CreatedAt:      time.Now().UTC(),
				UpdatedAt:      time.Now().UTC(),
			},
			payment: domaincommerce.Payment{
				ID:          payID,
				OrderID:     orderID,
				State:       "captured",
				AmountMinor: 500,
				Currency:    "USD",
				CreatedAt:   time.Now().UTC(),
			},
			rid: refundID,
		},
		WebhookPersist:              nil,
		SaleLines:                   stubSaleLineResolver{},
		WorkflowOrchestration:       workfloworch.NewDisabled(),
		EnterpriseAudit:             rec,
		ScheduleVendFailureFollowUp: false,
	})

	ctx := context.Background()
	row, err := svc.CreateRefund(ctx, CreateRefundInput{
		OrganizationID: org,
		OrderID:        orderID,
		AmountMinor:    100,
		Currency:       "USD",
		Reason:         "test",
		IdempotencyKey: "idem-refund-audit-1",
		Metadata:       []byte(`{}`),
	})
	require.NoError(t, err)
	require.Equal(t, refundID, row.ID)
	require.Equal(t, compliance.ActionRefundRequested, rec.last.Action)
	require.Equal(t, "commerce.refund", rec.last.ResourceType)
}

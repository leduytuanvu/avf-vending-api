package commerce

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/avf/avf-vending-api/internal/config"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var errMachinePayNotImpl = errors.New("not implemented")

type mpayOutbox struct {
	t   *testing.T
	pay domaincommerce.Payment
}

func (m *mpayOutbox) CreatePaymentWithOutbox(ctx context.Context, in domaincommerce.PaymentOutboxInput) (domaincommerce.PaymentOutboxResult, error) {
	require.Equal(m.t, int64(500), in.AmountMinor)
	require.Equal(m.t, "USD", in.Currency)
	_ = ctx
	return domaincommerce.PaymentOutboxResult{
		Payment: m.pay,
		Outbox:  domaincommerce.OutboxEvent{ID: 42},
	}, nil
}

type mpayLife struct {
	order       domaincommerce.Order
	lastAttempt *InsertPaymentAttemptParams
}

func (m *mpayLife) GetOrderByID(ctx context.Context, orderID uuid.UUID) (domaincommerce.Order, error) {
	_ = ctx
	if orderID != m.order.ID {
		return domaincommerce.Order{}, ErrNotFound
	}
	return m.order, nil
}

func (m *mpayLife) UpdateOrderStatus(ctx context.Context, orderID, organizationID uuid.UUID, status string) (domaincommerce.Order, error) {
	return domaincommerce.Order{}, errMachinePayNotImpl
}

func (m *mpayLife) GetVendSessionByOrderAndSlot(ctx context.Context, orderID uuid.UUID, slotIndex int32) (domaincommerce.VendSession, error) {
	return domaincommerce.VendSession{}, ErrNotFound
}

func (m *mpayLife) UpdateVendSessionState(ctx context.Context, p UpdateVendSessionParams) (domaincommerce.VendSession, error) {
	return domaincommerce.VendSession{}, errMachinePayNotImpl
}

func (m *mpayLife) GetLatestPaymentForOrder(ctx context.Context, orderID uuid.UUID) (domaincommerce.Payment, error) {
	return domaincommerce.Payment{}, ErrNotFound
}

func (m *mpayLife) GetPaymentByID(ctx context.Context, paymentID uuid.UUID) (domaincommerce.Payment, error) {
	return domaincommerce.Payment{}, ErrNotFound
}

func (m *mpayLife) InsertPaymentAttempt(ctx context.Context, in InsertPaymentAttemptParams) (PaymentAttemptView, error) {
	if m.lastAttempt != nil {
		*m.lastAttempt = in
	}
	return PaymentAttemptView{ID: uuid.New()}, nil
}

func (m *mpayLife) InsertRefundRow(context.Context, InsertRefundRowInput) (RefundRowView, error) {
	return RefundRowView{}, errMachinePayNotImpl
}

func (m *mpayLife) ListRefundsForOrder(context.Context, uuid.UUID) ([]RefundRowView, error) {
	return nil, errMachinePayNotImpl
}

func (m *mpayLife) GetRefundByIDForOrder(context.Context, uuid.UUID, uuid.UUID) (RefundRowView, error) {
	return RefundRowView{}, ErrNotFound
}

func (m *mpayLife) GetRefundByOrderIdempotency(context.Context, uuid.UUID, string) (RefundRowView, error) {
	return RefundRowView{}, ErrNotFound
}

func (m *mpayLife) SumNonFailedRefundAmountForPayment(context.Context, uuid.UUID) (int64, error) {
	return 0, nil
}

func (m *mpayLife) FulfillSuccessfulVendAtomically(context.Context, FulfillSuccessfulVendInput) (FulfillSuccessfulVendResult, error) {
	return FulfillSuccessfulVendResult{}, ErrNotConfigured
}

func (m *mpayLife) FulfillFailedVendAtomically(context.Context, FulfillFailedVendInput) (FulfillFailedVendResult, error) {
	return FulfillFailedVendResult{}, ErrNotConfigured
}

type sessionReg struct {
	inner platformpayments.PaymentProvider
	key   string
}

func (r sessionReg) ResolveForPaymentSession(appEnv config.AppEnvironment, clientDeclared string) (platformpayments.PaymentProvider, string, error) {
	_ = appEnv
	_ = clientDeclared
	return r.inner, r.key, nil
}

type captureProvider struct {
	*platformpayments.SandboxProvider
	last *platformpayments.CreatePaymentSessionInput
}

func (c *captureProvider) CreatePaymentSession(ctx context.Context, in platformpayments.CreatePaymentSessionInput) (platformpayments.CreatePaymentSessionResult, error) {
	inCopy := in
	c.last = &inCopy
	return c.SandboxProvider.CreatePaymentSession(ctx, in)
}

func TestCreateMachinePaymentSession_rejectsClientAmountMismatchVsOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	org := uuid.New()
	mid := uuid.New()
	oid := uuid.New()
	payID := uuid.New()

	life := &mpayLife{order: domaincommerce.Order{
		ID:             oid,
		OrganizationID: org,
		MachineID:      mid,
		TotalMinor:     500,
		Currency:       "USD",
		Status:         "created",
	}}
	outbox := &mpayOutbox{t: t, pay: domaincommerce.Payment{
		ID:          payID,
		State:       "created",
		AmountMinor: 500,
		Currency:    "USD",
		Provider:    "psp_fixture",
	}}
	cap := &captureProvider{SandboxProvider: platformpayments.NewSandboxProvider("psp_fixture")}
	svc := NewService(Deps{
		OrderVend:              stubOrderVend{},
		PaymentOutbox:          outbox,
		Lifecycle:              life,
		SaleLines:              stubSaleLines{},
		PaymentSessionRegistry: sessionReg{inner: cap, key: "psp_fixture"},
	})

	_, err := svc.CreateMachinePaymentSession(ctx, CreateMachinePaymentSessionInput{
		OrganizationID:  org,
		OrderID:         oid,
		MachineID:       mid,
		IdempotencyKey:  "ik-1",
		AmountMinor:     499,
		Currency:        "USD",
		AppEnv:          config.AppEnvDevelopment,
		OutboxTopic:     "t",
		OutboxEventType: "e",
		OutboxAggregate: "a",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidArgument), "got %v", err)
}

func TestCreateMachinePaymentSession_adapterUsesServerOrderTotalsAndBindsProviderReference(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	org := uuid.New()
	mid := uuid.New()
	oid := uuid.New()
	payID := uuid.New()

	var attempt InsertPaymentAttemptParams
	life := &mpayLife{
		order: domaincommerce.Order{
			ID:             oid,
			OrganizationID: org,
			MachineID:      mid,
			TotalMinor:     500,
			Currency:       "USD",
			Status:         "created",
		},
		lastAttempt: &attempt,
	}
	outbox := &mpayOutbox{t: t, pay: domaincommerce.Payment{
		ID:          payID,
		State:       "created",
		AmountMinor: 500,
		Currency:    "USD",
		Provider:    "psp_fixture",
	}}
	cap := &captureProvider{SandboxProvider: platformpayments.NewSandboxProvider("psp_fixture")}
	svc := NewService(Deps{
		OrderVend:              stubOrderVend{},
		PaymentOutbox:          outbox,
		Lifecycle:              life,
		SaleLines:              stubSaleLines{},
		PaymentSessionRegistry: sessionReg{inner: cap, key: "psp_fixture"},
	})

	res, err := svc.CreateMachinePaymentSession(ctx, CreateMachinePaymentSessionInput{
		OrganizationID:  org,
		OrderID:         oid,
		MachineID:       mid,
		IdempotencyKey:  "ik-ps-1",
		AmountMinor:     500,
		Currency:        "USD",
		AppEnv:          config.AppEnvDevelopment,
		OutboxTopic:     "commerce.payments",
		OutboxEventType: "payment.session_started",
		OutboxAggregate: "payment",
	})
	require.NoError(t, err)
	require.False(t, res.Replay)
	require.NotNil(t, cap.last)
	require.Equal(t, int64(500), cap.last.AmountMinor)
	require.Equal(t, payID, cap.last.PaymentID)
	require.Equal(t, oid, cap.last.OrderID)
	require.True(t, strings.HasPrefix(strings.TrimSpace(res.QRPayloadOrURL), "https://"))
	require.NotNil(t, attempt.ProviderReference)
	require.Equal(t, "sb_"+payID.String(), strings.TrimSpace(*attempt.ProviderReference))
}

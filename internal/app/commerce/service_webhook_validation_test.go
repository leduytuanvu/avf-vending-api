package commerce

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
)

type stubOrderVend struct{}

func (stubOrderVend) CreateOrderWithVendSession(ctx context.Context, in domaincommerce.CreateOrderVendInput) (domaincommerce.CreateOrderVendResult, error) {
	return domaincommerce.CreateOrderVendResult{}, errors.New("not implemented")
}

func (stubOrderVend) TryReplayCreateOrderWithVend(ctx context.Context, organizationID uuid.UUID, idempotencyKey string) (domaincommerce.CreateOrderVendResult, bool, error) {
	return domaincommerce.CreateOrderVendResult{}, false, nil
}

type stubPaymentOutbox struct{}

func (stubPaymentOutbox) CreatePaymentWithOutbox(ctx context.Context, in domaincommerce.PaymentOutboxInput) (domaincommerce.PaymentOutboxResult, error) {
	return domaincommerce.PaymentOutboxResult{}, errors.New("not implemented")
}

type stubLifecycle struct{}

func (stubLifecycle) GetOrderByID(ctx context.Context, orderID uuid.UUID) (domaincommerce.Order, error) {
	return domaincommerce.Order{}, ErrNotFound
}
func (stubLifecycle) UpdateOrderStatus(ctx context.Context, orderID, organizationID uuid.UUID, status string) (domaincommerce.Order, error) {
	return domaincommerce.Order{}, errors.New("not implemented")
}
func (stubLifecycle) GetVendSessionByOrderAndSlot(ctx context.Context, orderID uuid.UUID, slotIndex int32) (domaincommerce.VendSession, error) {
	return domaincommerce.VendSession{}, ErrNotFound
}
func (stubLifecycle) UpdateVendSessionState(ctx context.Context, p UpdateVendSessionParams) (domaincommerce.VendSession, error) {
	return domaincommerce.VendSession{}, errors.New("not implemented")
}
func (stubLifecycle) GetLatestPaymentForOrder(ctx context.Context, orderID uuid.UUID) (domaincommerce.Payment, error) {
	return domaincommerce.Payment{}, ErrNotFound
}
func (stubLifecycle) GetPaymentByID(ctx context.Context, paymentID uuid.UUID) (domaincommerce.Payment, error) {
	return domaincommerce.Payment{}, ErrNotFound
}
func (stubLifecycle) InsertPaymentAttempt(ctx context.Context, in InsertPaymentAttemptParams) (PaymentAttemptView, error) {
	return PaymentAttemptView{}, errors.New("not implemented")
}
func (stubLifecycle) InsertRefundRow(context.Context, InsertRefundRowInput) (RefundRowView, error) {
	return RefundRowView{}, errors.New("not implemented")
}
func (stubLifecycle) ListRefundsForOrder(context.Context, uuid.UUID) ([]RefundRowView, error) {
	return nil, errors.New("not implemented")
}
func (stubLifecycle) GetRefundByIDForOrder(context.Context, uuid.UUID, uuid.UUID) (RefundRowView, error) {
	return RefundRowView{}, ErrNotFound
}
func (stubLifecycle) GetRefundByOrderIdempotency(context.Context, uuid.UUID, string) (RefundRowView, error) {
	return RefundRowView{}, ErrNotFound
}
func (stubLifecycle) SumNonFailedRefundAmountForPayment(context.Context, uuid.UUID) (int64, error) {
	return 0, nil
}

func (stubLifecycle) FulfillSuccessfulVendAtomically(context.Context, FulfillSuccessfulVendInput) (FulfillSuccessfulVendResult, error) {
	return FulfillSuccessfulVendResult{}, ErrNotConfigured
}

func (stubLifecycle) FulfillFailedVendAtomically(context.Context, FulfillFailedVendInput) (FulfillFailedVendResult, error) {
	return FulfillFailedVendResult{}, ErrNotConfigured
}

type stubSaleLines struct{}

func (stubSaleLines) ResolveSaleLine(ctx context.Context, in ResolveSaleLineInput) (ResolvedSaleLine, error) {
	return ResolvedSaleLine{}, errors.New("not implemented")
}
func (stubSaleLines) LookupSlotDisplay(ctx context.Context, organizationID, machineID, productID uuid.UUID, slotIndex int32) (ResolvedSaleLine, error) {
	return ResolvedSaleLine{}, errors.New("not implemented")
}

type captureWebhookPersistence struct {
	got ApplyPaymentProviderWebhookInput
}

func (s *captureWebhookPersistence) ApplyPaymentProviderWebhook(ctx context.Context, in ApplyPaymentProviderWebhookInput) (ApplyPaymentProviderWebhookResult, error) {
	s.got = in
	return ApplyPaymentProviderWebhookResult{}, nil
}

func TestApplyPaymentProviderWebhook_requiresWebhookPersistence(t *testing.T) {
	s := &Service{
		orders:    stubOrderVend{},
		payments:  stubPaymentOutbox{},
		life:      stubLifecycle{},
		webhook:   nil,
		saleLines: stubSaleLines{},
	}
	_, err := s.ApplyPaymentProviderWebhook(context.Background(), ApplyPaymentProviderWebhookInput{
		OrganizationID:         uuid.New(),
		OrderID:                uuid.New(),
		PaymentID:              uuid.New(),
		Provider:               "stripe",
		ProviderReference:      "evt_1",
		WebhookEventID:         "wh_evt_test_webhook_not_configured",
		EventType:              "charge.succeeded",
		NormalizedPaymentState: "captured",
		Payload:                []byte(`{}`),
	})
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("got %v want ErrNotConfigured", err)
	}
}

func TestApplyPaymentProviderWebhook_requiresWebhookEventID(t *testing.T) {
	s := &Service{
		orders:    stubOrderVend{},
		payments:  stubPaymentOutbox{},
		life:      stubLifecycle{},
		webhook:   nil,
		saleLines: stubSaleLines{},
	}
	_, err := s.ApplyPaymentProviderWebhook(context.Background(), ApplyPaymentProviderWebhookInput{
		OrganizationID:         uuid.New(),
		OrderID:                uuid.New(),
		PaymentID:              uuid.New(),
		Provider:               "stripe",
		ProviderReference:      "evt_1",
		EventType:              "charge.succeeded",
		NormalizedPaymentState: "captured",
		Payload:                []byte(`{}`),
	})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("got %v want ErrInvalidArgument", err)
	}
}

func TestApplyPaymentProviderWebhook_webhookEventIDTooLong(t *testing.T) {
	s := &Service{
		orders:    stubOrderVend{},
		payments:  stubPaymentOutbox{},
		life:      stubLifecycle{},
		webhook:   nil,
		saleLines: stubSaleLines{},
	}
	long := make([]byte, 300)
	for i := range long {
		long[i] = 'a'
	}
	_, err := s.ApplyPaymentProviderWebhook(context.Background(), ApplyPaymentProviderWebhookInput{
		OrganizationID:         uuid.New(),
		OrderID:                uuid.New(),
		PaymentID:              uuid.New(),
		Provider:               "stripe",
		ProviderReference:      "evt_1",
		WebhookEventID:         string(long),
		EventType:              "charge.succeeded",
		NormalizedPaymentState: "captured",
		Payload:                []byte(`{}`),
	})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("got %v want ErrInvalidArgument", err)
	}
}

func TestApplyPaymentProviderWebhook_redactsSensitivePaymentPayload(t *testing.T) {
	persist := &captureWebhookPersistence{}
	s := &Service{
		orders:    stubOrderVend{},
		payments:  stubPaymentOutbox{},
		life:      stubLifecycle{},
		webhook:   persist,
		saleLines: stubSaleLines{},
	}
	_, err := s.ApplyPaymentProviderWebhook(context.Background(), ApplyPaymentProviderWebhookInput{
		OrganizationID:         uuid.New(),
		OrderID:                uuid.New(),
		PaymentID:              uuid.New(),
		Provider:               "stripe",
		ProviderReference:      "evt_sensitive",
		WebhookEventID:         "wh_evt_sensitive",
		EventType:              "charge.succeeded",
		NormalizedPaymentState: "captured",
		Payload:                []byte(`{"card_number":"4111111111111111","note":"card 4111 1111 1111 1111","amount":100}`),
		ProviderMetadata:       []byte(`{"hmac":"secret","safe":"ok"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(persist.got.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["card_number"] != "[REDACTED]" || payload["note"] != "card [REDACTED]" {
		t.Fatalf("payload not redacted: %s", string(persist.got.Payload))
	}
	var meta map[string]any
	if err := json.Unmarshal(persist.got.ProviderMetadata, &meta); err != nil {
		t.Fatal(err)
	}
	if meta["hmac"] != "[REDACTED]" || meta["safe"] != "ok" {
		t.Fatalf("metadata not redacted: %s", string(persist.got.ProviderMetadata))
	}
}

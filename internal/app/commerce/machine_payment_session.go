package commerce

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	"github.com/avf/avf-vending-api/internal/platform/payments/psp/ref"
	"github.com/google/uuid"
)

// CreateMachinePaymentSessionInput is the app-layer contract for vending gRPC payment sessions.
// Untrusted vending fields (QR URLs, provider references, outbox JSON) must never be passed here.
type CreateMachinePaymentSessionInput struct {
	OrderID             uuid.UUID
	MachineID           uuid.UUID
	IdempotencyKey      string
	ClientProvider      string
	ClientPayState      string
	AmountMinor         int64
	Currency            string
	AppEnv              config.AppEnvironment
	OutboxTopic         string
	OutboxEventType     string
	OutboxAggregate     string
	MachineExternalCode string // machine_code for AVF/TFO tenant selection
	StoreID             string // terminal/store hint for PSP
	ProviderReference   string // optional pre-assigned ref (legacy order_code)
	PreferredMethod     string // e.g. vietqr when using zalopay adapter
}

// CreateMachinePaymentSessionResult returns provider-owned display material for the kiosk.
type CreateMachinePaymentSessionResult struct {
	Replay         bool
	Payment        domaincommerce.Payment
	Outbox         domaincommerce.OutboxEvent
	ProviderKey        string
	ProviderReference  string
	ProviderSessionID  string
	CallbackURLHost    string
	QRPayloadOrURL string
	PaymentURL     string
	CheckoutURL    string
	ExpiresAt      *time.Time
}

// CreateMachinePaymentSession provisions a PSP-backed payment session with server-side adapter I/O.
func (s *Service) CreateMachinePaymentSession(ctx context.Context, in CreateMachinePaymentSessionInput) (CreateMachinePaymentSessionResult, error) {
	out := CreateMachinePaymentSessionResult{}
	if s == nil || s.payments == nil || s.life == nil {
		return out, ErrNotConfigured
	}
	if s.paymentSessionReg == nil {
		return out, ErrNotConfigured
	}
	if in.OrderID == uuid.Nil || in.MachineID == uuid.Nil {
		return out, errors.Join(ErrInvalidArgument, errors.New("order_id and machine_id are required"))
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	if key == "" {
		return out, errors.Join(ErrInvalidArgument, errors.New("idempotency_key is required"))
	}
	ps := strings.TrimSpace(in.ClientPayState)
	if ps != "" && strings.ToLower(ps) != "created" {
		return out, errors.Join(ErrInvalidArgument, errors.New("payment_state must be empty or created for PSP sessions"))
	}
	o, err := s.life.GetOrderByID(ctx, in.OrderID)
	if err != nil {
		return out, err
	}
	if o.MachineID != in.MachineID {
		return out, errors.Join(ErrInvalidArgument, errors.New("order machine mismatch"))
	}
	if o.TotalMinor != in.AmountMinor {
		return out, errors.Join(ErrInvalidArgument, errors.New("amount_minor does not match order total"))
	}
	if strings.ToUpper(strings.TrimSpace(o.Currency)) != strings.ToUpper(strings.TrimSpace(in.Currency)) {
		return out, errors.Join(ErrInvalidArgument, errors.New("currency does not match order"))
	}
	if orderStatusTerminal(o.Status) {
		return out, errors.Join(ErrIllegalTransition, errors.New("order is terminal"))
	}

	prov, pkey, err := s.paymentSessionReg.ResolveForPaymentSession(in.AppEnv, in.ClientProvider)
	if err != nil {
		return out, err
	}

	outboxPayload, _ := json.Marshal(map[string]any{
		"source":         "machine_payment_session",
		"order_id":       in.OrderID.String(),
		"provider":       pkey,
		"idempotency":    key,
		"schema_version": 1,
	})
	outboxIdem := key + ":outbox:" + in.OrderID.String()
	payRes, err := s.StartPaymentWithOutbox(ctx, StartPaymentInput{
		OrderID:              in.OrderID,
		Provider:             pkey,
		PaymentState:         "created",
		AmountMinor:          o.TotalMinor,
		Currency:             o.Currency,
		IdempotencyKey:       key,
		OutboxTopic:          in.OutboxTopic,
		OutboxEventType:      in.OutboxEventType,
		OutboxPayload:        outboxPayload,
		OutboxAggregateType:  in.OutboxAggregate,
		OutboxAggregateID:    in.OrderID,
		OutboxIdempotencyKey: outboxIdem,
		Simulated:            o.Simulated,
		SimulationRunID:      derefString(o.SimulationRunID),
		SimulationScenario:   derefString(o.SimulationScenario),
		FakeBill:             o.FakeBill,
		FakeBoard:            o.FakeBoard,
	})
	if err != nil {
		return out, err
	}
	out.Replay = payRes.Replay
	out.Payment = payRes.Payment
	out.Outbox = payRes.Outbox
	out.ProviderKey = pkey

	if payRes.Replay {
		if strings.TrimSpace(payRes.Payment.Provider) != "" && !strings.EqualFold(strings.TrimSpace(payRes.Payment.Provider), pkey) {
			return out, ErrIdempotencyPayloadConflict
		}
		if payRes.Payment.AmountMinor != o.TotalMinor ||
			strings.ToUpper(strings.TrimSpace(payRes.Payment.Currency)) != strings.ToUpper(strings.TrimSpace(o.Currency)) ||
			payRes.Payment.State != "created" {
			return out, ErrIdempotencyPayloadConflict
		}
		qr := replayQRPayloadFromStoredAttempt(ctx, s.life, payRes.Payment.ID)
		out.QRPayloadOrURL = qr
		return out, nil
	}

	providerRef := strings.TrimSpace(in.ProviderReference)
	if providerRef == "" {
		providerRef = ref.GenerateFromUUID(payRes.Payment.ID)
	}
	preCreatePayload, _ := json.Marshal(map[string]any{
		"provider_reference": providerRef,
		"bind_phase":         "pre_create",
	})
	if _, err := s.BindPaymentAttempt(ctx, InsertPaymentAttemptParams{
		PaymentID:         payRes.Payment.ID,
		State:             "created",
		ProviderReference: &providerRef,
		Payload:           preCreatePayload,
	}); err != nil {
		return out, err
	}

	sess, err := prov.CreatePaymentSession(ctx, platformpayments.CreatePaymentSessionInput{
		OrderID:             in.OrderID,
		PaymentID:           payRes.Payment.ID,
		AmountMinor:         o.TotalMinor,
		Currency:            o.Currency,
		IdempotencyKey:      key,
		MachineExternalCode: strings.TrimSpace(in.MachineExternalCode),
		StoreID:             strings.TrimSpace(in.StoreID),
		ProviderReference:   providerRef,
		PreferredMethod:     strings.TrimSpace(in.PreferredMethod),
	})
	if err != nil {
		return out, err
	}
	boundRef := strings.TrimSpace(sess.ProviderReference)
	if boundRef == "" {
		boundRef = providerRef
	}
	if boundRef == "" {
		return out, errors.Join(ErrNotConfigured, errors.New("payment provider returned empty provider_reference"))
	}
	attemptPayload := sess.ProviderDisplayJSON
	if len(attemptPayload) == 0 {
		attemptPayload, _ = json.Marshal(map[string]any{
			"provider_reference":  boundRef,
			"provider_session_id": sess.ProviderSessionID,
			"qr_url":              sess.QRPayloadOrURL,
			"payment_url":         sess.PaymentURL,
			"checkout_url":        sess.CheckoutURL,
		})
	}
	if !json.Valid(attemptPayload) {
		return out, errors.Join(ErrNotConfigured, errors.New("payment provider returned invalid attempt payload json"))
	}
	if _, err := s.BindPaymentAttempt(ctx, InsertPaymentAttemptParams{
		PaymentID:         payRes.Payment.ID,
		State:             "created",
		ProviderReference: &boundRef,
		Payload:           attemptPayload,
	}); err != nil {
		return out, err
	}
	qr := strings.TrimSpace(sess.QRPayloadOrURL)
	if qr == "" {
		qr = strings.TrimSpace(sess.PaymentURL)
	}
	out.QRPayloadOrURL = qr
	out.PaymentURL = strings.TrimSpace(sess.PaymentURL)
	out.CheckoutURL = strings.TrimSpace(sess.CheckoutURL)
	out.ExpiresAt = sess.ExpiresAt
	out.ProviderReference = boundRef
	out.ProviderSessionID = strings.TrimSpace(sess.ProviderSessionID)
	return out, nil
}

func orderStatusTerminal(st string) bool {
	switch strings.ToLower(strings.TrimSpace(st)) {
	case "completed", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func replayQRPayloadFromStoredAttempt(ctx context.Context, life CommerceLifecycleStore, paymentID uuid.UUID) string {
	if life == nil || paymentID == uuid.Nil {
		return ""
	}
	getter, ok := life.(interface {
		GetLatestPaymentAttemptPayload(ctx context.Context, paymentID uuid.UUID) ([]byte, error)
	})
	if !ok {
		return ""
	}
	payload, err := getter.GetLatestPaymentAttemptPayload(ctx, paymentID)
	if err != nil || len(payload) == 0 {
		return ""
	}
	return qrFromAttemptPayload(payload)
}

func qrFromAttemptPayload(b []byte) string {
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return ""
	}
	for _, k := range []string{"qr_code_url", "qr_url", "qr_payload_or_url"} {
		if s, ok := m[k].(string); ok {
			if v := strings.TrimSpace(s); v != "" {
				return v
			}
		}
	}
	return ""
}

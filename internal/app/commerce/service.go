// Ledger follow-up: emit financial_ledger_entries on payment/refund transitions and from reconcilers (see migrations/00007_financial_ledger_reconciliation.sql).
//
// Long-running follow-up (delayed compensation, human review) belongs on the optional Temporal workflow
// boundary (internal/app/workfloworch, wired from bootstrap when enabled)—not in synchronous HTTP handlers.
package commerce

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
)

// Service coordinates order creation, payment persistence, vend progression, and completion rules.
type Service struct {
	orders   domaincommerce.OrderVendWorkflow
	payments domaincommerce.PaymentOutboxWorkflow
	life     CommerceLifecycleStore
	webhook  PaymentWebhookPersistence
}

// NewService returns a commerce orchestrator. OrderVend workflow is required.
func NewService(d Deps) *Service {
	if d.OrderVend == nil {
		panic("commerce.NewService: nil OrderVendWorkflow")
	}
	return &Service{
		orders:   d.OrderVend,
		payments: d.PaymentOutbox,
		life:     d.Lifecycle,
		webhook:  d.WebhookPersist,
	}
}

var _ Orchestrator = (*Service)(nil)

var (
	paymentStates = map[string]struct{}{
		"created": {}, "authorized": {}, "captured": {}, "failed": {}, "refunded": {},
	}
	vendStates = map[string]struct{}{
		"pending": {}, "in_progress": {}, "success": {}, "failed": {},
	}
	vendTransitions = map[string]map[string]struct{}{
		"pending":     {"in_progress": {}},
		"in_progress": {"success": {}, "failed": {}},
	}
)

// CreateOrder provisions an order in "created" with an initial vend session in "pending".
func (s *Service) CreateOrder(ctx context.Context, in CreateOrderInput) (CreateOrderResult, error) {
	if err := validateCreateOrder(in); err != nil {
		return CreateOrderResult{}, err
	}
	return s.orders.CreateOrderWithVendSession(ctx, domaincommerce.CreateOrderVendInput{
		OrganizationID: in.OrganizationID,
		MachineID:      in.MachineID,
		ProductID:      in.ProductID,
		SlotIndex:      in.SlotIndex,
		Currency:       strings.ToUpper(strings.TrimSpace(in.Currency)),
		SubtotalMinor:  in.SubtotalMinor,
		TaxMinor:       in.TaxMinor,
		TotalMinor:     in.TotalMinor,
		IdempotencyKey: strings.TrimSpace(in.IdempotencyKey),
		OrderStatus:    "created",
		VendState:      "pending",
	})
}

// StartPaymentWithOutbox records a payment and optional outbox event; provider and outbox fields are caller-supplied.
func (s *Service) StartPaymentWithOutbox(ctx context.Context, in StartPaymentInput) (domaincommerce.PaymentOutboxResult, error) {
	if s.payments == nil {
		return domaincommerce.PaymentOutboxResult{}, ErrNotConfigured
	}
	if err := validateStartPayment(in); err != nil {
		return domaincommerce.PaymentOutboxResult{}, err
	}
	if s.life != nil {
		o, err := s.life.GetOrderByID(ctx, in.OrderID)
		if err != nil {
			return domaincommerce.PaymentOutboxResult{}, err
		}
		if o.OrganizationID != in.OrganizationID {
			return domaincommerce.PaymentOutboxResult{}, ErrOrgMismatch
		}
	}
	return s.payments.CreatePaymentWithOutbox(ctx, domaincommerce.PaymentOutboxInput{
		OrganizationID:       in.OrganizationID,
		OrderID:              in.OrderID,
		Provider:             strings.TrimSpace(in.Provider),
		PaymentState:         in.PaymentState,
		AmountMinor:          in.AmountMinor,
		Currency:             strings.ToUpper(strings.TrimSpace(in.Currency)),
		IdempotencyKey:       strings.TrimSpace(in.IdempotencyKey),
		OutboxTopic:          strings.TrimSpace(in.OutboxTopic),
		OutboxEventType:      strings.TrimSpace(in.OutboxEventType),
		OutboxPayload:        in.OutboxPayload,
		OutboxAggregateType:  strings.TrimSpace(in.OutboxAggregateType),
		OutboxAggregateID:    in.OutboxAggregateID,
		OutboxIdempotencyKey: strings.TrimSpace(in.OutboxIdempotencyKey),
	})
}

// BindPaymentAttempt appends a payment_attempt row for an existing payment aggregate.
func (s *Service) BindPaymentAttempt(ctx context.Context, in InsertPaymentAttemptParams) (PaymentAttemptView, error) {
	if s.life == nil {
		return PaymentAttemptView{}, ErrNotConfigured
	}
	if in.PaymentID == uuid.Nil {
		return PaymentAttemptView{}, errors.Join(ErrInvalidArgument, errors.New("payment_id must be set"))
	}
	if strings.TrimSpace(in.State) == "" {
		return PaymentAttemptView{}, errors.Join(ErrInvalidArgument, errors.New("state is required"))
	}
	return s.life.InsertPaymentAttempt(ctx, in)
}

// MarkOrderPaidAfterPaymentCapture moves the order to "paid" when the latest payment is captured.
func (s *Service) MarkOrderPaidAfterPaymentCapture(ctx context.Context, organizationID, orderID uuid.UUID) (domaincommerce.Order, error) {
	if s.life == nil {
		return domaincommerce.Order{}, ErrNotConfigured
	}
	if organizationID == uuid.Nil || orderID == uuid.Nil {
		return domaincommerce.Order{}, errors.Join(ErrInvalidArgument, errors.New("organization_id and order_id must be set"))
	}
	o, err := s.life.GetOrderByID(ctx, orderID)
	if err != nil {
		return domaincommerce.Order{}, err
	}
	if o.OrganizationID != organizationID {
		return domaincommerce.Order{}, ErrOrgMismatch
	}
	pay, err := s.life.GetLatestPaymentForOrder(ctx, orderID)
	if err != nil {
		return domaincommerce.Order{}, err
	}
	if pay.State != "captured" {
		return domaincommerce.Order{}, ErrPaymentNotSettled
	}
	if o.Status == "paid" || o.Status == "vending" || o.Status == "completed" {
		return o, nil
	}
	if o.Status != "created" && o.Status != "quoted" {
		return domaincommerce.Order{}, errors.Join(ErrIllegalTransition, errors.New("order cannot move to paid from current status"))
	}
	return s.life.UpdateOrderStatus(ctx, orderID, organizationID, "paid")
}

// AdvanceVend validates and persists a non-terminal vend transition; may move order to "vending" when appropriate.
func (s *Service) AdvanceVend(ctx context.Context, in AdvanceVendInput) (domaincommerce.VendSession, error) {
	if s.life == nil {
		return domaincommerce.VendSession{}, ErrNotConfigured
	}
	if err := validateAdvanceVend(in); err != nil {
		return domaincommerce.VendSession{}, err
	}
	o, err := s.life.GetOrderByID(ctx, in.OrderID)
	if err != nil {
		return domaincommerce.VendSession{}, err
	}
	if o.OrganizationID != in.OrganizationID {
		return domaincommerce.VendSession{}, ErrOrgMismatch
	}
	v, err := s.life.GetVendSessionByOrderAndSlot(ctx, in.OrderID, in.SlotIndex)
	if err != nil {
		return domaincommerce.VendSession{}, err
	}
	if _, ok := vendStates[in.ToState]; !ok {
		return domaincommerce.VendSession{}, errors.Join(ErrInvalidArgument, errors.New("invalid vend target state"))
	}
	next, ok := vendTransitions[v.State]
	if _, allowed := next[in.ToState]; !ok || !allowed {
		return domaincommerce.VendSession{}, ErrIllegalTransition
	}
	updated, err := s.life.UpdateVendSessionState(ctx, UpdateVendSessionParams{
		OrderID:       in.OrderID,
		SlotIndex:     in.SlotIndex,
		ToState:       in.ToState,
		FailureReason: in.FailureReason,
	})
	if err != nil {
		return domaincommerce.VendSession{}, err
	}
	if in.ToState == "in_progress" && o.Status == "paid" {
		if _, err := s.life.UpdateOrderStatus(ctx, in.OrderID, in.OrganizationID, "vending"); err != nil {
			return domaincommerce.VendSession{}, err
		}
	}
	return updated, nil
}

// FinalizeOrderAfterVend applies a terminal vend outcome and sets order status; payment must be captured to reach "completed".
func (s *Service) FinalizeOrderAfterVend(ctx context.Context, in FinalizeAfterVendInput) (FinalizeOutcome, error) {
	if s.life == nil {
		return FinalizeOutcome{}, ErrNotConfigured
	}
	if in.OrganizationID == uuid.Nil || in.OrderID == uuid.Nil {
		return FinalizeOutcome{}, errors.Join(ErrInvalidArgument, errors.New("organization_id and order_id must be set"))
	}
	if in.TerminalVendState != "success" && in.TerminalVendState != "failed" {
		return FinalizeOutcome{}, errors.Join(ErrInvalidArgument, errors.New("terminal vend state must be success or failed"))
	}
	o, err := s.life.GetOrderByID(ctx, in.OrderID)
	if err != nil {
		return FinalizeOutcome{}, err
	}
	if o.OrganizationID != in.OrganizationID {
		return FinalizeOutcome{}, ErrOrgMismatch
	}
	v, err := s.life.GetVendSessionByOrderAndSlot(ctx, in.OrderID, in.SlotIndex)
	if err != nil {
		return FinalizeOutcome{}, err
	}
	if v.State == "success" || v.State == "failed" {
		o2, oErr := s.life.GetOrderByID(ctx, in.OrderID)
		if oErr != nil {
			return FinalizeOutcome{}, oErr
		}
		return FinalizeOutcome{Order: o2, Vend: v}, nil
	}
	next, ok := vendTransitions[v.State]
	if _, allowed := next[in.TerminalVendState]; !ok || !allowed {
		return FinalizeOutcome{}, ErrIllegalTransition
	}

	pay, pErr := s.life.GetLatestPaymentForOrder(ctx, in.OrderID)
	if pErr != nil && !errors.Is(pErr, ErrNotFound) {
		return FinalizeOutcome{}, pErr
	}

	vendUpdated, err := s.life.UpdateVendSessionState(ctx, UpdateVendSessionParams{
		OrderID:       in.OrderID,
		SlotIndex:     in.SlotIndex,
		ToState:       in.TerminalVendState,
		FailureReason: in.FailureReason,
	})
	if err != nil {
		return FinalizeOutcome{}, err
	}

	if in.TerminalVendState == "success" {
		if pErr != nil || pay.State != "captured" {
			return FinalizeOutcome{}, ErrPaymentNotSettled
		}
		orderUpdated, err := s.life.UpdateOrderStatus(ctx, in.OrderID, in.OrganizationID, "completed")
		if err != nil {
			return FinalizeOutcome{}, err
		}
		return FinalizeOutcome{Order: orderUpdated, Vend: vendUpdated}, nil
	}
	orderUpdated, err := s.life.UpdateOrderStatus(ctx, in.OrderID, in.OrganizationID, "failed")
	if err != nil {
		return FinalizeOutcome{}, err
	}
	return FinalizeOutcome{Order: orderUpdated, Vend: vendUpdated}, nil
}

// EvaluateRefundEligibility returns an advisory outcome from persisted payment and vend state (no provider calls).
func (s *Service) EvaluateRefundEligibility(ctx context.Context, orderID uuid.UUID, slotIndex int32) (RefundEligibilityAssessment, error) {
	if s.life == nil {
		return RefundEligibilityAssessment{}, ErrNotConfigured
	}
	if orderID == uuid.Nil {
		return RefundEligibilityAssessment{}, errors.Join(ErrInvalidArgument, errors.New("order_id must be set"))
	}
	if slotIndex < 0 {
		return RefundEligibilityAssessment{}, errors.Join(ErrInvalidArgument, errors.New("slot_index must be non-negative"))
	}
	o, err := s.life.GetOrderByID(ctx, orderID)
	if err != nil {
		return RefundEligibilityAssessment{}, err
	}
	pay, pErr := s.life.GetLatestPaymentForOrder(ctx, orderID)
	if pErr != nil {
		if errors.Is(pErr, ErrNotFound) {
			return RefundEligibilityAssessment{
				Eligible:     false,
				Reason:       "no payment on file",
				PaymentState: "",
				VendState:    "",
			}, nil
		}
		return RefundEligibilityAssessment{}, pErr
	}
	v, vErr := s.life.GetVendSessionByOrderAndSlot(ctx, orderID, slotIndex)
	vendState := ""
	switch {
	case vErr == nil:
		vendState = v.State
	case errors.Is(vErr, ErrNotFound):
		vendState = ""
	default:
		return RefundEligibilityAssessment{}, vErr
	}
	out := RefundEligibilityAssessment{
		PaymentState: pay.State,
		VendState:    vendState,
	}
	switch {
	case pay.State == "refunded":
		out.Eligible = false
		out.Reason = "payment already refunded"
	case pay.State == "captured" && vendState == "failed":
		out.Eligible = true
		out.Reason = "captured payment with failed vend"
	case pay.State == "captured" && o.Status == "failed":
		out.Eligible = true
		out.Reason = "order failed after capture"
	case pay.State == "captured" && vendState == "success" && o.Status == "completed":
		out.Eligible = false
		out.Reason = "successful fulfillment"
	default:
		out.Eligible = false
		out.Reason = "no refund policy match for current states"
	}
	return out, nil
}

// EnsureOrderOrganization verifies the order belongs to the caller organization.
func (s *Service) EnsureOrderOrganization(ctx context.Context, organizationID, orderID uuid.UUID) error {
	if s.life == nil {
		return ErrNotConfigured
	}
	o, err := s.life.GetOrderByID(ctx, orderID)
	if err != nil {
		return err
	}
	if o.OrganizationID != organizationID {
		return ErrOrgMismatch
	}
	return nil
}

// GetCheckoutStatus returns authoritative order, vend, and latest payment state for an organization-scoped read.
func (s *Service) GetCheckoutStatus(ctx context.Context, organizationID, orderID uuid.UUID, slotIndex int32) (CheckoutStatusView, error) {
	if s.life == nil {
		return CheckoutStatusView{}, ErrNotConfigured
	}
	o, err := s.life.GetOrderByID(ctx, orderID)
	if err != nil {
		return CheckoutStatusView{}, err
	}
	if o.OrganizationID != organizationID {
		return CheckoutStatusView{}, ErrOrgMismatch
	}
	v, err := s.life.GetVendSessionByOrderAndSlot(ctx, orderID, slotIndex)
	if err != nil {
		return CheckoutStatusView{}, err
	}
	out := CheckoutStatusView{Order: o, Vend: v}
	pay, err := s.life.GetLatestPaymentForOrder(ctx, orderID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return out, nil
		}
		return CheckoutStatusView{}, err
	}
	out.Payment = pay
	out.PaymentPresent = true
	return out, nil
}

// ApplyPaymentProviderWebhook persists an auditable provider notification and advances payment state when legal.
// Duplicate deliveries for the same (provider, provider_reference) are idempotent replays.
func (s *Service) ApplyPaymentProviderWebhook(ctx context.Context, in ApplyPaymentProviderWebhookInput) (ApplyPaymentProviderWebhookResult, error) {
	if s.webhook == nil {
		return ApplyPaymentProviderWebhookResult{}, ErrNotConfigured
	}
	target := strings.TrimSpace(strings.ToLower(in.NormalizedPaymentState))
	if _, ok := paymentStates[target]; !ok {
		return ApplyPaymentProviderWebhookResult{}, errors.Join(ErrInvalidArgument, errors.New("invalid normalized_payment_state"))
	}
	if strings.TrimSpace(in.Provider) == "" || strings.TrimSpace(in.ProviderReference) == "" {
		return ApplyPaymentProviderWebhookResult{}, errors.Join(ErrInvalidArgument, errors.New("provider and provider_reference are required"))
	}
	if strings.TrimSpace(in.EventType) == "" {
		return ApplyPaymentProviderWebhookResult{}, errors.Join(ErrInvalidArgument, errors.New("event_type is required"))
	}
	payload := in.Payload
	if len(payload) == 0 {
		payload = []byte("{}")
	}
	if !json.Valid(payload) {
		return ApplyPaymentProviderWebhookResult{}, errors.Join(ErrInvalidArgument, errors.New("payload must be JSON"))
	}
	in2 := ApplyPaymentProviderWebhookInput{
		OrganizationID:         in.OrganizationID,
		OrderID:                in.OrderID,
		PaymentID:              in.PaymentID,
		Provider:               strings.TrimSpace(in.Provider),
		ProviderReference:      strings.TrimSpace(in.ProviderReference),
		EventType:              strings.TrimSpace(in.EventType),
		NormalizedPaymentState: target,
		Payload:                payload,
		ProviderAmountMinor:    in.ProviderAmountMinor,
		Currency:               in.Currency,
	}
	return s.webhook.ApplyPaymentProviderWebhook(ctx, in2)
}

func validateCreateOrder(in CreateOrderInput) error {
	if in.OrganizationID == uuid.Nil || in.MachineID == uuid.Nil || in.ProductID == uuid.Nil {
		return errors.Join(ErrInvalidArgument, errors.New("organization_id, machine_id, and product_id must be set"))
	}
	if strings.TrimSpace(in.IdempotencyKey) == "" {
		return errors.Join(ErrInvalidArgument, errors.New("idempotency_key is required"))
	}
	cur := strings.ToUpper(strings.TrimSpace(in.Currency))
	if len(cur) != 3 || !isAllAlpha(cur) {
		return errors.Join(ErrInvalidArgument, errors.New("currency must be a 3-letter ISO code"))
	}
	if in.SubtotalMinor < 0 || in.TaxMinor < 0 || in.TotalMinor < 0 {
		return errors.Join(ErrInvalidArgument, errors.New("amounts must be non-negative"))
	}
	if in.SubtotalMinor+in.TaxMinor != in.TotalMinor {
		return errors.Join(ErrInvalidArgument, errors.New("subtotal_minor plus tax_minor must equal total_minor"))
	}
	if in.SlotIndex < 0 {
		return errors.Join(ErrInvalidArgument, errors.New("slot_index must be non-negative"))
	}
	return nil
}

func validateStartPayment(in StartPaymentInput) error {
	if in.OrganizationID == uuid.Nil || in.OrderID == uuid.Nil {
		return errors.Join(ErrInvalidArgument, errors.New("organization_id and order_id must be set"))
	}
	if strings.TrimSpace(in.Provider) == "" {
		return errors.Join(ErrInvalidArgument, errors.New("provider is required"))
	}
	if _, ok := paymentStates[in.PaymentState]; !ok {
		return errors.Join(ErrInvalidArgument, errors.New("invalid payment state"))
	}
	if in.AmountMinor < 0 {
		return errors.Join(ErrInvalidArgument, errors.New("amount_minor must be non-negative"))
	}
	cur := strings.ToUpper(strings.TrimSpace(in.Currency))
	if len(cur) != 3 || !isAllAlpha(cur) {
		return errors.Join(ErrInvalidArgument, errors.New("currency must be a 3-letter ISO code"))
	}
	if strings.TrimSpace(in.IdempotencyKey) == "" || strings.TrimSpace(in.OutboxIdempotencyKey) == "" {
		return errors.Join(ErrInvalidArgument, errors.New("idempotency keys are required"))
	}
	if strings.TrimSpace(in.OutboxTopic) == "" || strings.TrimSpace(in.OutboxEventType) == "" || strings.TrimSpace(in.OutboxAggregateType) == "" {
		return errors.Join(ErrInvalidArgument, errors.New("outbox topic, event type, and aggregate type are required"))
	}
	if in.OutboxAggregateID == uuid.Nil {
		return errors.Join(ErrInvalidArgument, errors.New("outbox_aggregate_id must be set"))
	}
	return nil
}

func validateAdvanceVend(in AdvanceVendInput) error {
	if in.OrganizationID == uuid.Nil || in.OrderID == uuid.Nil {
		return errors.Join(ErrInvalidArgument, errors.New("organization_id and order_id must be set"))
	}
	if in.ToState == "success" || in.ToState == "failed" {
		return errors.Join(ErrInvalidArgument, errors.New("use FinalizeOrderAfterVend for terminal vend states"))
	}
	if _, ok := vendStates[in.ToState]; !ok {
		return errors.Join(ErrInvalidArgument, errors.New("invalid vend state"))
	}
	return nil
}

func isAllAlpha(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

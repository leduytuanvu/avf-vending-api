package commerce

import (
	"context"

	"github.com/avf/avf-vending-api/internal/app/workfloworch"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
)

// CommerceLifecycleStore covers order/vend/payment reads and updates not yet modeled as single workflows.
// Postgres/sqlc should implement this contract.
type CommerceLifecycleStore interface {
	GetOrderByID(ctx context.Context, orderID uuid.UUID) (domaincommerce.Order, error)
	UpdateOrderStatus(ctx context.Context, orderID, organizationID uuid.UUID, status string) (domaincommerce.Order, error)

	GetVendSessionByOrderAndSlot(ctx context.Context, orderID uuid.UUID, slotIndex int32) (domaincommerce.VendSession, error)
	UpdateVendSessionState(ctx context.Context, p UpdateVendSessionParams) (domaincommerce.VendSession, error)

	GetLatestPaymentForOrder(ctx context.Context, orderID uuid.UUID) (domaincommerce.Payment, error)
	GetPaymentByID(ctx context.Context, paymentID uuid.UUID) (domaincommerce.Payment, error)

	InsertPaymentAttempt(ctx context.Context, in InsertPaymentAttemptParams) (PaymentAttemptView, error)

	InsertRefundRow(ctx context.Context, in InsertRefundRowInput) (RefundRowView, error)
	ListRefundsForOrder(ctx context.Context, orderID uuid.UUID) ([]RefundRowView, error)
	GetRefundByIDForOrder(ctx context.Context, orderID, refundID uuid.UUID) (RefundRowView, error)
	GetRefundByOrderIdempotency(ctx context.Context, orderID uuid.UUID, idempotencyKey string) (RefundRowView, error)
	SumNonFailedRefundAmountForPayment(ctx context.Context, paymentID uuid.UUID) (int64, error)
}

// PaymentWebhookPersistence applies idempotent provider notifications in a single database transaction.
type PaymentWebhookPersistence interface {
	ApplyPaymentProviderWebhook(ctx context.Context, in ApplyPaymentProviderWebhookInput) (ApplyPaymentProviderWebhookResult, error)
}

// Deps wires workflows and optional lifecycle persistence.
type Deps struct {
	OrderVend                   domaincommerce.OrderVendWorkflow
	PaymentOutbox               domaincommerce.PaymentOutboxWorkflow
	Lifecycle                   CommerceLifecycleStore
	WebhookPersist              PaymentWebhookPersistence
	SaleLines                   SaleLineResolver
	WorkflowOrchestration       workfloworch.Boundary
	ScheduleVendFailureFollowUp bool
}

// Orchestrator is the application surface for HTTP/workers.
type Orchestrator interface {
	CreateOrder(ctx context.Context, in CreateOrderInput) (CreateOrderResult, error)
	StartPaymentWithOutbox(ctx context.Context, in StartPaymentInput) (domaincommerce.PaymentOutboxResult, error)
	BindPaymentAttempt(ctx context.Context, in InsertPaymentAttemptParams) (PaymentAttemptView, error)
	MarkOrderPaidAfterPaymentCapture(ctx context.Context, organizationID, orderID uuid.UUID) (domaincommerce.Order, error)
	AdvanceVend(ctx context.Context, in AdvanceVendInput) (domaincommerce.VendSession, error)
	FinalizeOrderAfterVend(ctx context.Context, in FinalizeAfterVendInput) (FinalizeOutcome, error)
	EvaluateRefundEligibility(ctx context.Context, orderID uuid.UUID, slotIndex int32) (RefundEligibilityAssessment, error)

	GetCheckoutStatus(ctx context.Context, organizationID, orderID uuid.UUID, slotIndex int32) (CheckoutStatusView, error)
	ApplyPaymentProviderWebhook(ctx context.Context, in ApplyPaymentProviderWebhookInput) (ApplyPaymentProviderWebhookResult, error)

	EnsureCommerceCallerOrderAccess(ctx context.Context, organizationID, orderID uuid.UUID, p plauth.Principal) error
	CancelOrder(ctx context.Context, organizationID, orderID uuid.UUID, reason string) (domaincommerce.Order, error)
	CreateRefund(ctx context.Context, in CreateRefundInput) (RefundRowView, error)
	ListRefundsForOrder(ctx context.Context, organizationID, orderID uuid.UUID) ([]RefundRowView, error)
	GetRefundForOrder(ctx context.Context, organizationID, orderID, refundID uuid.UUID) (RefundRowView, error)
}

package commerce

import (
	"github.com/google/uuid"
)

// ReconciledPaymentTransitionInput is a reconciler-driven terminal transition from created|authorized.
// ProviderHint is persisted on payment_attempts (JSON); Reason is human/diagnostic text for audit.
type ReconciledPaymentTransitionInput struct {
	PaymentID    uuid.UUID
	ToState      string // captured | failed
	Reason       string
	ProviderHint []byte
	DryRun       bool
}

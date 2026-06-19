package commerce

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrVendEvidenceRequired = errors.New("vend hardware evidence required")
	ErrVendEvidenceInvalid  = errors.New("vend hardware evidence invalid")
)

// VerificationStatus values persisted on vend_sessions.verification_status.
const (
	VerificationUnverified         = "unverified"
	VerificationVerified           = "verified"
	VerificationHardwareUnverified = "hardware_unverified"
)

// HardwareCommandRef links vend success to command ledger / wire trace.
type HardwareCommandRef struct {
	CommandID  string
	TxRxDigest string
	RawRef     string
}

// BillFinalRecord is the BILL acceptor final credit for cash checkout.
type BillFinalRecord struct {
	EventID     string
	AmountMinor int64
	Currency    string
	RawRef      string
}

// TcnDispenseRecord is the TCN board dispense/drop outcome.
type TcnDispenseRecord struct {
	Slot    string
	Result  string
	Dropped bool
	Digest  string
	RawRef  string
}

// VendHardwareEvidence is structured hardware correlation for vend finalization.
type VendHardwareEvidence struct {
	VendAttemptID uuid.UUID
	CorrelationID uuid.UUID
	Command       HardwareCommandRef
	BillFinal     *BillFinalRecord
	TcnDispense   TcnDispenseRecord
}

// Validate checks evidence completeness. requireSuccessEvidence enforces TCN dropped for success paths.
func (e *VendHardwareEvidence) Validate(cashFlow bool, requireSuccessEvidence bool) error {
	if e == nil {
		return ErrVendEvidenceRequired
	}
	if e.VendAttemptID == uuid.Nil {
		return fmt.Errorf("%w: vend_attempt_id required", ErrVendEvidenceInvalid)
	}
	if e.CorrelationID == uuid.Nil {
		return fmt.Errorf("%w: correlation_id required", ErrVendEvidenceInvalid)
	}
	cmdID := strings.TrimSpace(e.Command.CommandID)
	if cmdID == "" {
		return fmt.Errorf("%w: command.command_id required", ErrVendEvidenceInvalid)
	}
	digest := strings.TrimSpace(e.Command.TxRxDigest)
	rawRef := strings.TrimSpace(e.Command.RawRef)
	if digest == "" && rawRef == "" {
		return fmt.Errorf("%w: command tx_rx_digest or raw_ref required", ErrVendEvidenceInvalid)
	}
	if cashFlow {
		if e.BillFinal == nil {
			return fmt.Errorf("%w: bill_final required for cash flow", ErrVendEvidenceInvalid)
		}
		if strings.TrimSpace(e.BillFinal.EventID) == "" {
			return fmt.Errorf("%w: bill_final.event_id required", ErrVendEvidenceInvalid)
		}
		if e.BillFinal.AmountMinor <= 0 {
			return fmt.Errorf("%w: bill_final.amount_minor must be positive", ErrVendEvidenceInvalid)
		}
		cur := strings.ToUpper(strings.TrimSpace(e.BillFinal.Currency))
		if len(cur) != 3 {
			return fmt.Errorf("%w: bill_final.currency must be 3-letter ISO", ErrVendEvidenceInvalid)
		}
	}
	if strings.TrimSpace(e.TcnDispense.Slot) == "" {
		return fmt.Errorf("%w: tcn_dispense.slot required", ErrVendEvidenceInvalid)
	}
	if strings.TrimSpace(e.TcnDispense.Result) == "" {
		return fmt.Errorf("%w: tcn_dispense.result required", ErrVendEvidenceInvalid)
	}
	if requireSuccessEvidence && !e.TcnDispense.Dropped {
		return fmt.Errorf("%w: tcn_dispense.dropped must be true for vend success", ErrVendEvidenceInvalid)
	}
	tdDigest := strings.TrimSpace(e.TcnDispense.Digest)
	tdRaw := strings.TrimSpace(e.TcnDispense.RawRef)
	if tdDigest == "" && tdRaw == "" {
		return fmt.Errorf("%w: tcn_dispense digest or raw_ref required", ErrVendEvidenceInvalid)
	}
	return nil
}

// EvidenceDigest returns a stable fingerprint for persistence (command digest preferred).
func (e *VendHardwareEvidence) EvidenceDigest() string {
	if e == nil {
		return ""
	}
	if d := strings.TrimSpace(e.Command.TxRxDigest); d != "" {
		return d
	}
	if d := strings.TrimSpace(e.TcnDispense.Digest); d != "" {
		return d
	}
	return strings.TrimSpace(e.Command.RawRef)
}

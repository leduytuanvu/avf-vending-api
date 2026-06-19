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
	// Self-attested integrity: any present digest must be a well-formed SHA-256 hex string, and when
	// both the command and TCN dispense carry a digest they must agree (the device derives both from
	// the same dispense wire bytes). This binds the evidence to a real exchange without an
	// independent backend copy of the serial frames; independent ledger cross-check is a tracked
	// follow-up that requires a new app->backend frame-ingest path.
	if digest != "" && !isSHA256Hex(digest) {
		return fmt.Errorf("%w: command.tx_rx_digest must be sha-256 hex", ErrVendEvidenceInvalid)
	}
	if tdDigest != "" && !isSHA256Hex(tdDigest) {
		return fmt.Errorf("%w: tcn_dispense.digest must be sha-256 hex", ErrVendEvidenceInvalid)
	}
	if digest != "" && tdDigest != "" && !strings.EqualFold(digest, tdDigest) {
		return fmt.Errorf("%w: tcn_dispense.digest must match command.tx_rx_digest", ErrVendEvidenceInvalid)
	}
	return nil
}

// isSHA256Hex reports whether s is exactly 64 hexadecimal characters (a SHA-256 digest).
func isSHA256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
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

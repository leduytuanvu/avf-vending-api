package commerce

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// validDigestHex is a well-formed 64-char SHA-256 hex digest used across evidence tests.
const validDigestHex = "4d7a1c1f2b3e4a5d6c7b8a9f0e1d2c3b4a5f6e7d8c9b0a1f2e3d4c5b6a7f8e9d"

func validEvidence() *VendHardwareEvidence {
	return &VendHardwareEvidence{
		VendAttemptID: uuid.New(),
		CorrelationID: uuid.New(),
		Command: HardwareCommandRef{
			CommandID:  "cmd-1",
			TxRxDigest: validDigestHex,
		},
		BillFinal: &BillFinalRecord{
			EventID:     "bill-1",
			AmountMinor: 15000,
			Currency:    "VND",
		},
		TcnDispense: TcnDispenseRecord{
			Slot:    "A1",
			Result:  "ok",
			Dropped: true,
			Digest:  validDigestHex,
		},
	}
}

func TestVendHardwareEvidence_Validate_HappyPath(t *testing.T) {
	require.NoError(t, validEvidence().Validate(true, true))
}

func TestVendHardwareEvidence_Validate_MissingVendAttempt(t *testing.T) {
	ev := validEvidence()
	ev.VendAttemptID = uuid.Nil
	require.ErrorIs(t, ev.Validate(true, true), ErrVendEvidenceInvalid)
}

func TestVendHardwareEvidence_Validate_CashWithoutBill(t *testing.T) {
	ev := validEvidence()
	ev.BillFinal = nil
	require.ErrorIs(t, ev.Validate(true, true), ErrVendEvidenceInvalid)
}

func TestVendHardwareEvidence_Validate_TcnNotDropped(t *testing.T) {
	ev := validEvidence()
	ev.TcnDispense.Dropped = false
	require.ErrorIs(t, ev.Validate(true, true), ErrVendEvidenceInvalid)
}

func TestVendHardwareEvidence_Validate_NilEvidence(t *testing.T) {
	require.ErrorIs(t, (*VendHardwareEvidence)(nil).Validate(true, true), ErrVendEvidenceRequired)
}

func TestVendHardwareEvidence_Validate_NonCashSkipsBill(t *testing.T) {
	ev := validEvidence()
	ev.BillFinal = nil
	require.NoError(t, ev.Validate(false, true))
}

func TestVendHardwareEvidence_Validate_MalformedDigest(t *testing.T) {
	ev := validEvidence()
	ev.Command.TxRxDigest = "not-a-sha256-hex"
	ev.TcnDispense.Digest = "not-a-sha256-hex"
	require.ErrorIs(t, ev.Validate(true, true), ErrVendEvidenceInvalid)
}

func TestVendHardwareEvidence_Validate_DigestMismatch(t *testing.T) {
	ev := validEvidence()
	// Both well-formed hex but inconsistent (command vs tcn dispense) must be rejected.
	ev.TcnDispense.Digest = "0000000000000000000000000000000000000000000000000000000000000000"
	require.ErrorIs(t, ev.Validate(true, true), ErrVendEvidenceInvalid)
}

func TestVendHardwareEvidence_Validate_RawRefOnlyNoDigest(t *testing.T) {
	// XY/hybrid path: no digest, only an honest rawRef -> still valid (never forces a fake digest).
	ev := validEvidence()
	ev.Command.TxRxDigest = ""
	ev.Command.RawRef = "dispense:order-1"
	ev.TcnDispense.Digest = ""
	ev.TcnDispense.RawRef = "dispense:order-1"
	require.NoError(t, ev.Validate(true, true))
}

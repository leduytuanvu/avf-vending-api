package commerce

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func validEvidence() *VendHardwareEvidence {
	return &VendHardwareEvidence{
		VendAttemptID: uuid.New(),
		CorrelationID: uuid.New(),
		Command: HardwareCommandRef{
			CommandID:  "cmd-1",
			TxRxDigest: "abc123",
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
			Digest:  "tcn-digest",
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

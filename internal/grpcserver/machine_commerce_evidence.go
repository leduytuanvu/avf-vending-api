package grpcserver

import (
	"fmt"
	"strings"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
)

const vendHardwareEvidenceRequiredMsg = "vend_hardware_evidence_required"

func mapProtoVendHardwareEvidence(ev *machinev1.VendHardwareEvidence, legacyCorr *uuid.UUID) (*domaincommerce.VendHardwareEvidence, error) {
	if ev == nil {
		return nil, domaincommerce.ErrVendEvidenceRequired
	}
	out := &domaincommerce.VendHardwareEvidence{}
	if vid := strings.TrimSpace(ev.GetVendAttemptId()); vid != "" {
		u, err := uuid.Parse(vid)
		if err != nil || u == uuid.Nil {
			return nil, domaincommerce.ErrVendEvidenceInvalid
		}
		out.VendAttemptID = u
	}
	corrStr := strings.TrimSpace(ev.GetCorrelationId())
	if corrStr == "" && legacyCorr != nil {
		corrStr = legacyCorr.String()
	}
	if corrStr != "" {
		u, err := uuid.Parse(corrStr)
		if err != nil || u == uuid.Nil {
			return nil, domaincommerce.ErrVendEvidenceInvalid
		}
		out.CorrelationID = u
	}
	if cmd := ev.GetCommand(); cmd != nil {
		out.Command = domaincommerce.HardwareCommandRef{
			CommandID:  strings.TrimSpace(cmd.GetCommandId()),
			TxRxDigest: strings.TrimSpace(cmd.GetTxRxDigest()),
			RawRef:     strings.TrimSpace(cmd.GetRawRef()),
		}
	}
	if bill := ev.GetBillFinal(); bill != nil {
		out.BillFinal = &domaincommerce.BillFinalRecord{
			EventID:     strings.TrimSpace(bill.GetEventId()),
			AmountMinor: bill.GetAmountMinor(),
			Currency:    strings.TrimSpace(bill.GetCurrency()),
		}
		if bill.RawRef != nil {
			out.BillFinal.RawRef = strings.TrimSpace(bill.GetRawRef())
		}
	}
	if tcn := ev.GetTcnDispense(); tcn != nil {
		out.TcnDispense = domaincommerce.TcnDispenseRecord{
			Slot:    strings.TrimSpace(tcn.GetSlot()),
			Result:  strings.TrimSpace(tcn.GetResult()),
			Dropped: tcn.GetDropped(),
		}
		if tcn.Digest != nil {
			out.TcnDispense.Digest = strings.TrimSpace(tcn.GetDigest())
		}
		if tcn.RawRef != nil {
			out.TcnDispense.RawRef = strings.TrimSpace(tcn.GetRawRef())
		}
	}
	return out, nil
}

// resolveVendEvidenceFromRequest maps + validates client evidence and reconciles the self-attested
// cash amount against the backend's authoritative authorized payment. authorizedAmountMinor is the
// order's authorized cash amount for cash flows (0 when unknown/non-cash); when positive it must
// equal bill_final.amount_minor. With requireEvidence off, any failure degrades to
// hardware_unverified for backward compatibility.
func resolveVendEvidenceFromRequest(ev *machinev1.VendHardwareEvidence, legacyCorr *uuid.UUID, cashFlow bool, requireEvidence bool, requireSuccessEvidence bool, authorizedAmountMinor int64) (*domaincommerce.VendHardwareEvidence, string, error) {
	if ev == nil {
		if requireEvidence {
			return nil, "", domaincommerce.ErrVendEvidenceRequired
		}
		return nil, domaincommerce.VerificationHardwareUnverified, nil
	}
	mapped, err := mapProtoVendHardwareEvidence(ev, legacyCorr)
	if err != nil {
		if requireEvidence {
			return nil, "", err
		}
		return nil, domaincommerce.VerificationHardwareUnverified, nil
	}
	if err := mapped.Validate(cashFlow, requireSuccessEvidence); err != nil {
		if requireEvidence {
			return nil, "", err
		}
		return nil, domaincommerce.VerificationHardwareUnverified, nil
	}
	// Amount reconcile against the backend's own authorized payment (independent of the device): the
	// self-attested bill_final.amount_minor must equal the authorized cash amount.
	if cashFlow && authorizedAmountMinor > 0 && mapped.BillFinal != nil && mapped.BillFinal.AmountMinor != authorizedAmountMinor {
		if requireEvidence {
			return nil, "", fmt.Errorf("%w: bill_final.amount_minor %d does not match authorized %d", domaincommerce.ErrVendEvidenceInvalid, mapped.BillFinal.AmountMinor, authorizedAmountMinor)
		}
		return nil, domaincommerce.VerificationHardwareUnverified, nil
	}
	return mapped, domaincommerce.VerificationVerified, nil
}

func machineCommerceVendOutboxConfig(deps MachineGRPCServicesDeps) (topic, succeeded, failed, reconciliation, aggregate string) {
	if deps.Config == nil {
		return "", "", "", "", "order"
	}
	c := deps.Config.Commerce
	return c.VendOutboxTopic, c.VendOutboxEventTypeSucceeded, c.VendOutboxEventTypeFailed, c.VendOutboxEventTypeReconciliation, c.VendOutboxAggregateType
}

func commerceRequireVendHardwareEvidence(deps MachineGRPCServicesDeps, machineID uuid.UUID) bool {
	if deps.Config == nil {
		return false
	}
	if deps.Config.Commerce.RequireVendHardwareEvidence {
		return true
	}
	if machineID == uuid.Nil {
		return false
	}
	for _, id := range deps.Config.Commerce.RequireVendHardwareEvidenceMachineIDs {
		if id == machineID {
			return true
		}
	}
	return false
}

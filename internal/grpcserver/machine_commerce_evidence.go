package grpcserver

import (
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

func resolveVendEvidenceFromRequest(ev *machinev1.VendHardwareEvidence, legacyCorr *uuid.UUID, cashFlow bool, requireEvidence bool, requireSuccessEvidence bool) (*domaincommerce.VendHardwareEvidence, string, error) {
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
	return mapped, domaincommerce.VerificationVerified, nil
}

func machineCommerceVendOutboxConfig(deps MachineGRPCServicesDeps) (topic, succeeded, failed, reconciliation, aggregate string) {
	if deps.Config == nil {
		return "", "", "", "", "order"
	}
	c := deps.Config.Commerce
	return c.VendOutboxTopic, c.VendOutboxEventTypeSucceeded, c.VendOutboxEventTypeFailed, c.VendOutboxEventTypeReconciliation, c.VendOutboxAggregateType
}

func commerceRequireVendHardwareEvidence(deps MachineGRPCServicesDeps) bool {
	return deps.Config != nil && deps.Config.Commerce.RequireVendHardwareEvidence
}

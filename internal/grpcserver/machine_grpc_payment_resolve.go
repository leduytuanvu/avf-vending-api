package grpcserver

import (
	"context"

	appmachinepaymentmethods "github.com/avf/avf-vending-api/internal/app/machinepaymentmethods"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	"github.com/google/uuid"
)

func paymentRegistryFromRuntime(src MachinePaymentRuntimeSource) *platformpayments.Registry {
	if src == nil {
		return nil
	}
	if r, ok := src.(*platformpayments.Registry); ok {
		return r
	}
	return nil
}

func resolveMachinePaymentMethods(ctx context.Context, deps MachineGRPCServicesDeps, machineID uuid.UUID, flags map[string]bool) platformpayments.MachinePaymentMethodsView {
	override := machinePaymentOverride(ctx, deps.MachinePaymentMethods, machineID)
	reg := paymentRegistryFromRuntime(deps.PaymentRuntime)
	return platformpayments.ResolveMachinePaymentMethodsWithOverride(deps.Config, reg, flags, override)
}

func machinePaymentOverride(ctx context.Context, svc *appmachinepaymentmethods.Service, machineID uuid.UUID) platformpayments.MachineMethodOverride {
	if svc == nil || machineID == uuid.Nil {
		return platformpayments.MachineMethodOverride{}
	}
	return svc.OverrideForMachine(ctx, machineID)
}

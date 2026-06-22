package grpcserver

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/google/uuid"
)

func TestCommerceRequireVendHardwareEvidence(t *testing.T) {
	machineA := uuid.MustParse("019e702c-11c6-7ab0-89c7-5eb32f0b12cb")
	machineB := uuid.MustParse("55555555-5555-5555-5555-555555555555")

	depsWith := func(global bool, ids ...uuid.UUID) MachineGRPCServicesDeps {
		return MachineGRPCServicesDeps{
			Config: &config.Config{
				Commerce: config.CommerceHTTPConfig{
					RequireVendHardwareEvidence:           global,
					RequireVendHardwareEvidenceMachineIDs: ids,
				},
			},
		}
	}

	if commerceRequireVendHardwareEvidence(depsWith(false), machineA) {
		t.Fatal("expected false when global off and empty list")
	}
	if !commerceRequireVendHardwareEvidence(depsWith(true), machineA) {
		t.Fatal("expected true when global on")
	}
	if !commerceRequireVendHardwareEvidence(depsWith(false, machineA), machineA) {
		t.Fatal("expected true when machine in allowlist")
	}
	if commerceRequireVendHardwareEvidence(depsWith(false, machineA), machineB) {
		t.Fatal("expected false when machine not in allowlist")
	}
	if commerceRequireVendHardwareEvidence(MachineGRPCServicesDeps{}, uuid.Nil) {
		t.Fatal("expected false when config nil or machine nil")
	}
}

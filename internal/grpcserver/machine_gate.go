package grpcserver

import (
	"context"
	"strings"

	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func machineCredentialGate(ctx context.Context, q *db.Queries, claims plauth.MachineAccessClaims) error {
	row, err := q.GetMachineCredentialGate(ctx, claims.MachineID)
	if err != nil {
		return status.Errorf(codes.NotFound, "machine not found")
	}
	if row.OrganizationID != claims.OrganizationID {
		return status.Errorf(codes.PermissionDenied, "machine organization mismatch")
	}
	if row.CredentialRevokedAt.Valid {
		return status.Errorf(codes.Unauthenticated, "machine credentials revoked")
	}
	st := strings.ToLower(strings.TrimSpace(row.Status))
	switch st {
	case "active", "draft", "provisioned", "provisioning", "online", "offline":
		// allowed for authenticated vending flows
	case "retired", "decommissioned":
		return status.Errorf(codes.PermissionDenied, "machine retired")
	case "maintenance":
		return status.Errorf(codes.Unauthenticated, "machine disabled")
	case "suspended":
		return status.Errorf(codes.PermissionDenied, "machine suspended")
	case "compromised":
		return status.Errorf(codes.Unauthenticated, "machine compromised")
	default:
		return status.Errorf(codes.PermissionDenied, "machine not operational")
	}
	if row.CredentialVersion != claims.CredentialVersion {
		return status.Errorf(codes.Unauthenticated, "machine credential version mismatch")
	}
	return nil
}

// machineRuntimeInventoryGate enforces credential version plus operational status for inventory / operator RPCs.
// Only machines in online or offline lifecycle may mutate or read inventory snapshots over gRPC.
func machineRuntimeInventoryGate(ctx context.Context, q *db.Queries, claims plauth.MachineAccessClaims) error {
	row, err := q.GetMachineCredentialGate(ctx, claims.MachineID)
	if err != nil {
		return status.Errorf(codes.NotFound, "machine not found")
	}
	if row.OrganizationID != claims.OrganizationID {
		return status.Errorf(codes.PermissionDenied, "machine organization mismatch")
	}
	if row.CredentialRevokedAt.Valid {
		return status.Errorf(codes.Unauthenticated, "machine credentials revoked")
	}
	if strings.EqualFold(strings.TrimSpace(row.Status), "retired") || strings.EqualFold(strings.TrimSpace(row.Status), "decommissioned") {
		return status.Errorf(codes.PermissionDenied, "machine retired")
	}
	if row.CredentialVersion != claims.CredentialVersion {
		return status.Errorf(codes.Unauthenticated, "machine credential version mismatch")
	}
	st := strings.ToLower(strings.TrimSpace(row.Status))
	switch st {
	case "online", "offline":
		return nil
	default:
		return status.Errorf(codes.PermissionDenied, "machine not operational for inventory")
	}
}

package grpcserver

import (
	"context"
	"strings"

	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// machineCredentialGate is retained for call-site clarity; unary auth interceptor already validated credentials.
func machineCredentialGate(ctx context.Context, q *db.Queries, claims plauth.MachineAccessClaims) error {
	_ = ctx
	_ = q
	_ = claims
	return nil
}

func snapshotFromCredentialGate(row db.GetMachineCredentialGateRow) machineCredentialGateSnapshot {
	return machineCredentialGateSnapshot{
		loaded:            true,
		status:            row.Status,
		credentialVersion: row.CredentialVersion,
		credentialRevoked: row.CredentialRevokedAt.Valid,
	}
}

func snapshotFromSessionGate(sg db.GetMachineSessionGateRow) machineCredentialGateSnapshot {
	return machineCredentialGateSnapshot{
		loaded:            true,
		status:            sg.MachineStatus,
		credentialVersion: sg.MachineCredentialVersion,
		credentialRevoked: sg.MachineCredentialRevokedAt.Valid,
	}
}

func evaluateMachineRuntimeInventoryGate(machineStatus string, credentialVersion int64, revoked bool, claims plauth.MachineAccessClaims) error {
	if revoked {
		return status.Errorf(codes.Unauthenticated, "machine credentials revoked")
	}
	if strings.EqualFold(strings.TrimSpace(machineStatus), "retired") || strings.EqualFold(strings.TrimSpace(machineStatus), "decommissioned") {
		return status.Errorf(codes.PermissionDenied, "machine retired")
	}
	if credentialVersion != claims.CredentialVersion {
		return status.Errorf(codes.Unauthenticated, "machine credential version mismatch")
	}
	st := strings.ToLower(strings.TrimSpace(machineStatus))
	switch st {
	case "online", "offline":
		return nil
	default:
		return status.Errorf(codes.PermissionDenied, "machine not operational for inventory")
	}
}

// machineRuntimeInventoryGate enforces credential version plus operational status for inventory / operator RPCs.
// Only machines in online or offline lifecycle may mutate or read inventory snapshots over gRPC.
func machineRuntimeInventoryGate(ctx context.Context, q *db.Queries, claims plauth.MachineAccessClaims) error {
	if snap, ok := machineCredentialGateSnapshotFromContext(ctx); ok {
		return evaluateMachineRuntimeInventoryGate(snap.status, snap.credentialVersion, snap.credentialRevoked, claims)
	}
	row, err := q.GetMachineCredentialGate(ctx, claims.MachineID)
	if err != nil {
		return status.Errorf(codes.NotFound, "machine not found")
	}
	return evaluateMachineRuntimeInventoryGate(row.Status, row.CredentialVersion, row.CredentialRevokedAt.Valid, claims)
}

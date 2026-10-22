package grpcserver

import (
	"context"
	"testing"

	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestEvaluateMachineRuntimeInventoryGate(t *testing.T) {
	t.Parallel()
	claims := plauth.MachineAccessClaims{MachineID: uuid.New(), CredentialVersion: 3}
	require.NoError(t, evaluateMachineRuntimeInventoryGate("online", 3, false, claims))
	require.NoError(t, evaluateMachineRuntimeInventoryGate("offline", 3, false, claims))
	require.Equal(t, codes.Unauthenticated, status.Code(evaluateMachineRuntimeInventoryGate("online", 3, true, claims)))
	require.Equal(t, codes.PermissionDenied, status.Code(evaluateMachineRuntimeInventoryGate("retired", 3, false, claims)))
	require.Equal(t, codes.Unauthenticated, status.Code(evaluateMachineRuntimeInventoryGate("online", 2, false, claims)))
	require.Equal(t, codes.PermissionDenied, status.Code(evaluateMachineRuntimeInventoryGate("active", 3, false, claims)))
}

func TestMachineRuntimeInventoryGate_ReusesContextSnapshot(t *testing.T) {
	t.Parallel()
	claims := plauth.MachineAccessClaims{MachineID: uuid.New(), CredentialVersion: 1}
	ctx := withMachineCredentialGateSnapshotBox(context.Background())
	storeMachineCredentialGateSnapshot(ctx, machineCredentialGateSnapshot{
		loaded:            true,
		status:            "online",
		credentialVersion: 1,
		credentialRevoked: false,
	})
	err := machineRuntimeInventoryGate(ctx, nil, claims)
	require.NoError(t, err)
}

func TestRuntimeHintsFlagsCache_Hit(t *testing.T) {
	t.Parallel()
	var cache runtimeHintsFlagsCache
	id := uuid.New()
	_, ok := cache.get(id)
	require.False(t, ok)
	cache.put(id, map[string]bool{"qr_card_enabled": true})
	got, ok := cache.get(id)
	require.True(t, ok)
	require.True(t, got["qr_card_enabled"])
}

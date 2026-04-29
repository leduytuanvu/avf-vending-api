package correctness

import (
	"context"
	"testing"

	"github.com/avf/avf-vending-api/internal/grpcserver"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestP06_E2E_MachineCredentialChecker_rejectsSuspendedMachine(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	insertOrganizationSiteMachine(t, ctx, pool, orgID, siteID, machineID, "suspended", 1)

	chk := grpcserver.NewSQLMachineTokenCredentialChecker(pool)
	err := chk.ValidateMachineAccessClaims(ctx, plauth.MachineAccessClaims{
		OrganizationID:    orgID,
		MachineID:         machineID,
		CredentialVersion: 1,
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestP06_E2E_MachineCredentialChecker_rejectsCompromisedMachine(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	insertOrganizationSiteMachine(t, ctx, pool, orgID, siteID, machineID, "compromised", 1)

	chk := grpcserver.NewSQLMachineTokenCredentialChecker(pool)
	err := chk.ValidateMachineAccessClaims(ctx, plauth.MachineAccessClaims{
		OrganizationID:    orgID,
		MachineID:         machineID,
		CredentialVersion: 1,
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

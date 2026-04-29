package postgres_test

import (
	"context"
	"testing"

	appfleet "github.com/avf/avf-vending-api/internal/app/fleet"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestFleetAdminP05_CredentialLifecycleAndSoftArchive(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	insertAuditOrganization(t, pool, orgID)
	_, err := pool.Exec(ctx, `
INSERT INTO sites (id, organization_id, name, code, status)
VALUES ($1, $2, 'P05 Site', $3, 'active')
`, siteID, orgID, "p05-site-"+siteID.String()[:8])
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, organization_id, site_id, serial_number, code, name, status, credential_version)
VALUES ($1, $2, $3, $4, $5, 'P05 Machine', 'active', 7)
`, machineID, orgID, siteID, "p05-sn-"+machineID.String(), "p05-"+machineID.String()[:8])
	require.NoError(t, err)

	svc := appfleet.NewService(postgres.NewFleetRepository(pool))
	rotated, err := svc.RotateMachineCredential(ctx, orgID, machineID)
	require.NoError(t, err)
	require.Equal(t, int64(8), rotated.CredentialVersion)
	require.NotNil(t, rotated.RotatedAt)
	require.Nil(t, rotated.RevokedAt)

	revoked, err := svc.RevokeMachineCredential(ctx, orgID, machineID)
	require.NoError(t, err)
	require.Equal(t, int64(9), revoked.CredentialVersion)
	require.NotNil(t, revoked.RevokedAt)

	compromised, err := svc.MarkMachineCompromised(ctx, orgID, machineID)
	require.NoError(t, err)
	require.Equal(t, "compromised", compromised.Status)
	require.NotNil(t, compromised.RevokedAt)

	retired, err := svc.RetireMachine(ctx, orgID, machineID)
	require.NoError(t, err)
	require.Equal(t, "decommissioned", retired.Status)

	var exists bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM machines WHERE id = $1)`, machineID).Scan(&exists))
	require.True(t, exists, "archive must be soft and preserve machine history")
}

func TestFleetAdminP05_TechnicianAssignmentExplicitRelease(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	techID := uuid.New()
	insertAuditOrganization(t, pool, orgID)
	_, err := pool.Exec(ctx, `INSERT INTO sites (id, organization_id, name, code, status) VALUES ($1, $2, 'P05 Assign Site', $3, 'active')`, siteID, orgID, "p05-assign-"+siteID.String()[:8])
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO machines (id, organization_id, site_id, serial_number, code, name, status) VALUES ($1, $2, $3, $4, $5, 'P05 Assign Machine', 'active')`, machineID, orgID, siteID, "p05-assign-sn-"+machineID.String(), "p05m-"+machineID.String()[:8])
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO technicians (id, organization_id, display_name, status) VALUES ($1, $2, 'Tech P05', 'active')`, techID, orgID)
	require.NoError(t, err)

	svc := appfleet.NewService(postgres.NewFleetRepository(pool))
	assignment, err := svc.AssignTechnicianToMachine(ctx, appfleet.AssignTechnicianInput{
		OrganizationID: orgID,
		MachineID:      machineID,
		TechnicianID:   techID,
		Role:           "field_service",
		Scope:          "maintenance",
	})
	require.NoError(t, err)
	require.Equal(t, "active", assignment.Status)
	require.Equal(t, "maintenance", assignment.Scope)

	released, err := svc.ReleaseTechnicianAssignmentForMachineUser(ctx, orgID, machineID, techID)
	require.NoError(t, err)
	require.Equal(t, "released", released.Status)
	require.NotNil(t, released.ValidTo)
}

func TestFleetAdminP05_TechnicianSelfAssignmentForbidden(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	techID := uuid.New()
	insertAuditOrganization(t, pool, orgID)
	_, err := pool.Exec(ctx, `INSERT INTO sites (id, organization_id, name, code, status) VALUES ($1, $2, 'Self Site', $3, 'active')`, siteID, orgID, "self-site-"+siteID.String()[:8])
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO machines (id, organization_id, site_id, serial_number, code, name, status) VALUES ($1, $2, $3, $4, $5, 'Self Machine', 'active')`, machineID, orgID, siteID, "self-sn-"+machineID.String(), "selfm-"+machineID.String()[:8])
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO technicians (id, organization_id, display_name, status) VALUES ($1, $2, 'Self Tech', 'active')`, techID, orgID)
	require.NoError(t, err)

	svc := appfleet.NewService(postgres.NewFleetRepository(pool))
	_, err = svc.AssignTechnicianToMachine(ctx, appfleet.AssignTechnicianInput{
		OrganizationID:    orgID,
		MachineID:         machineID,
		TechnicianID:      techID,
		Role:              "field_service",
		ActorTechnicianID: techID,
	})
	require.ErrorIs(t, err, appfleet.ErrForbiddenTechnicianSelfAssignment)
}

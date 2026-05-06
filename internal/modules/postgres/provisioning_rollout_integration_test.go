package postgres_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	appactivation "github.com/avf/avf-vending-api/internal/app/activation"
	appfleet "github.com/avf/avf-vending-api/internal/app/fleet"
	approvisioning "github.com/avf/avf-vending-api/internal/app/provisioning"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestP21_BulkProvisioning100Machines_NoActivationCodes(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	insertAuditOrganization(t, pool, orgID)
	_, err := pool.Exec(ctx, `
INSERT INTO sites (id, organization_id, name, code, status)
VALUES ($1, $2, 'Bulk Site', $3, 'active')
`, siteID, orgID, "bulk-site-"+siteID.String()[:8])
	require.NoError(t, err)

	fleetSvc := appfleet.NewService(postgres.NewFleetRepository(pool))
	issuer := mustTestIssuer(t)
	actSvc := appactivation.NewService(pool, issuer, plauth.TrimSecret(bytes.Repeat([]byte("z"), 32)), nil)
	provSvc := approvisioning.NewService(approvisioning.Deps{
		Pool:       pool,
		Fleet:      fleetSvc,
		Activation: actSvc,
		Audit:      nil,
	})

	rows := make([]approvisioning.BulkMachineRow, 100)
	for i := range rows {
		rows[i] = approvisioning.BulkMachineRow{
			SerialNumber: fmt.Sprintf("%s-P21-%04d", orgID.String()[:8], i),
			Name:         fmt.Sprintf("Bulk %d", i),
			Model:        "AVF-TEST",
		}
	}
	res, err := provSvc.BulkCreateMachines(ctx, orgID, approvisioning.BulkCreateInput{
		SiteID:                  siteID,
		CabinetType:             "ambient",
		Machines:                rows,
		GenerateActivationCodes: false,
	})
	require.NoError(t, err)
	require.Equal(t, 100, res.MachineCount)
	require.Len(t, res.Machines, 100)

	var n int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM machines WHERE organization_id = $1 AND status = 'provisioning'`, orgID).Scan(&n))
	require.Equal(t, 100, n)
}

func mustTestIssuer(t *testing.T) *plauth.SessionIssuer {
	t.Helper()
	cfg := config.HTTPAuthConfig{
		Mode:            plauth.HTTPAuthModeHS256,
		JWTSecret:       bytes.Repeat([]byte("z"), 32),
		JWTLeeway:       30 * time.Second,
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 720 * time.Hour,
	}
	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(cfg)
	require.NoError(t, err)
	return issuer
}

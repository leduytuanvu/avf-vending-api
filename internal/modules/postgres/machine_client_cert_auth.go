package postgres

import (
	"context"
	"crypto/x509"
	"fmt"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// MachineGRPCClientCertAuth implements auth.MachineGRPCClientCertChecker using machine_device_certificates.
type MachineGRPCClientCertAuth struct {
	q         *db.Queries
	uriPrefix string
}

// NewMachineGRPCClientCertAuth builds a checker; uriPrefix should match GRPC_MTLS_MACHINE_ID_URI_PREFIX (e.g. urn:avf:machine:).
func NewMachineGRPCClientCertAuth(q *db.Queries, uriPrefix string) *MachineGRPCClientCertAuth {
	return &MachineGRPCClientCertAuth{q: q, uriPrefix: uriPrefix}
}

// ResolveMachineAccessFromClientCert implements auth.MachineGRPCClientCertChecker.
func (a *MachineGRPCClientCertAuth) ResolveMachineAccessFromClientCert(ctx context.Context, cert *x509.Certificate) (auth.MachineAccessClaims, error) {
	if a == nil || a.q == nil || cert == nil {
		return auth.MachineAccessClaims{}, auth.ErrUnauthenticated
	}
	fp := auth.ClientCertificateFingerprintSHA256(cert)
	row, err := a.q.DeviceCertificateActiveByFingerprint(ctx, fp[:])
	if err != nil {
		if err == pgx.ErrNoRows {
			return auth.MachineAccessClaims{}, auth.ErrUnauthenticated
		}
		return auth.MachineAccessClaims{}, fmt.Errorf("device cert lookup: %w", err)
	}
	if mid, ok := auth.ParseMachineIDFromClientCertURIs(cert, a.uriPrefix); ok && mid != row.MachineID {
		return auth.MachineAccessClaims{}, auth.ErrUnauthenticated
	}
	scopes := make([]string, len(auth.DefaultMachineAccessScopes))
	copy(scopes, auth.DefaultMachineAccessScopes)
	return auth.MachineAccessClaims{
		MachineID:         row.MachineID,
		OrganizationID:    row.OrganizationID,
		SiteID:            uuid.Nil,
		CredentialVersion: 0,
		Scopes:            scopes,
		Subject:           fmt.Sprintf("machine:%s", row.MachineID.String()),
		ExpiresAt:         cert.NotAfter.UTC(),
		Audience:          auth.AudienceMachineGRPC,
		Type:              auth.JWTClaimTypeMachine,
		JTI:               "",
	}, nil
}

package auth

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"strings"

	"github.com/google/uuid"
)

// MachineGRPCClientCertChecker resolves a verified TLS peer leaf certificate into machine runtime
// claims after checking Postgres-registered device certificate metadata (fingerprint, status, validity window).
// Optional on the machine gRPC listener; when nil, mTLS may still be enabled for transport security only.
type MachineGRPCClientCertChecker interface {
	ResolveMachineAccessFromClientCert(ctx context.Context, cert *x509.Certificate) (MachineAccessClaims, error)
}

// MachineTokenCredentialChecker validates token claims against authoritative
// machine credential state after signature/claim validation succeeds.
type MachineTokenCredentialChecker interface {
	ValidateMachineAccessClaims(ctx context.Context, claims MachineAccessClaims) error
}

// ClientCertificateFingerprintSHA256 returns the SHA-256 fingerprint of the DER-encoded certificate.
func ClientCertificateFingerprintSHA256(cert *x509.Certificate) [32]byte {
	if cert == nil {
		return [32]byte{}
	}
	return sha256.Sum256(cert.Raw)
}

// ParseMachineIDFromClientCertURIs extracts a machine UUID from URI SANs using prefix (e.g. "urn:avf:machine:").
// Returns false when no matching URI is present.
func ParseMachineIDFromClientCertURIs(cert *x509.Certificate, uriPrefix string) (uuid.UUID, bool) {
	if cert == nil {
		return uuid.Nil, false
	}
	p := strings.TrimSpace(uriPrefix)
	if p == "" {
		return uuid.Nil, false
	}
	for _, u := range cert.URIs {
		if u == nil {
			continue
		}
		s := u.String()
		if !strings.HasPrefix(s, p) {
			continue
		}
		id, err := uuid.Parse(strings.TrimSpace(strings.TrimPrefix(s, p)))
		if err == nil && id != uuid.Nil {
			return id, true
		}
	}
	return uuid.Nil, false
}

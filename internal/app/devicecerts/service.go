package devicecerts

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"fmt"

	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Service registers and revokes device client certificate metadata (no private keys).
type Service struct {
	q     *db.Queries
	audit compliance.EnterpriseRecorder
}

func NewService(q *db.Queries) *Service {
	return &Service{q: q}
}

// WithAudit enables enterprise audit hooks for certificate lifecycle operations.
func (s *Service) WithAudit(rec compliance.EnterpriseRecorder) *Service {
	if s != nil {
		s.audit = rec
	}
	return s
}

// Register inserts active certificate metadata for a machine (call after CA issued cert to device).
func (s *Service) Register(ctx context.Context, organizationID, machineID uuid.UUID, cert *x509.Certificate, serial string) (db.MachineDeviceCertificate, error) {
	if s == nil || s.q == nil || cert == nil {
		return db.MachineDeviceCertificate{}, fmt.Errorf("devicecerts: invalid args")
	}
	fp := sha256.Sum256(cert.Raw)
	sans, _ := json.Marshal(sanSummary(cert))
	issuer := pgtype.Text{}
	if cert.Issuer.String() != "" {
		issuer = pgtype.Text{String: cert.Issuer.String(), Valid: true}
	}
	row, err := s.q.DeviceCertificateInsert(ctx, db.DeviceCertificateInsertParams{
		OrganizationID:    organizationID,
		MachineID:         machineID,
		FingerprintSha256: fp[:],
		SerialNumber:      serial,
		SubjectDn:         cert.Subject.String(),
		IssuerDn:          issuer,
		SansJson:          sans,
		NotBefore:         cert.NotBefore.UTC(),
		NotAfter:          cert.NotAfter.UTC(),
		Status:            "active",
		Metadata:          []byte("{}"),
	})
	if err != nil {
		return db.MachineDeviceCertificate{}, err
	}
	_ = s.recordAudit(ctx, organizationID, row.ID, compliance.ActionMachineDeviceCertRegistered, map[string]any{
		"machine_id": machineID.String(),
		"serial":     serial,
		"not_after":  cert.NotAfter.UTC().Format("2006-01-02T15:04:05Z07:00"),
	})
	return row, nil
}

// Revoke marks a certificate inactive by fingerprint.
func (s *Service) Revoke(ctx context.Context, organizationID uuid.UUID, cert *x509.Certificate, reason string) (int64, error) {
	if s == nil || s.q == nil || cert == nil {
		return 0, fmt.Errorf("devicecerts: invalid args")
	}
	fp := sha256.Sum256(cert.Raw)
	rows, err := s.q.DeviceCertificateRevokeByFingerprint(ctx, db.DeviceCertificateRevokeByFingerprintParams{
		OrganizationID:    organizationID,
		FingerprintSha256: fp[:],
		RevokeReason:      pgtype.Text{String: reason, Valid: reason != ""},
	})
	if err != nil {
		return 0, err
	}
	_ = s.recordAudit(ctx, organizationID, uuid.Nil, compliance.ActionMachineDeviceCertRevoked, map[string]any{
		"fingerprint_sha256": fmt.Sprintf("%x", fp[:]),
		"reason":             reason,
		"rows":               rows,
	})
	return rows, nil
}

// Rotate registers a new active certificate and supersedes the previous row (by old cert id).
func (s *Service) Rotate(ctx context.Context, organizationID, machineID, oldCertID uuid.UUID, oldCert, newCert *x509.Certificate, newSerial string) (db.MachineDeviceCertificate, error) {
	if s == nil || s.q == nil || oldCert == nil || newCert == nil {
		return db.MachineDeviceCertificate{}, fmt.Errorf("devicecerts: invalid args")
	}
	row, err := s.Register(ctx, organizationID, machineID, newCert, newSerial)
	if err != nil {
		return db.MachineDeviceCertificate{}, err
	}
	if err := s.q.DeviceCertificateSupersede(ctx, db.DeviceCertificateSupersedeParams{
		ID:             oldCertID,
		OrganizationID: organizationID,
		SupersededBy:   pgtype.UUID{Bytes: row.ID, Valid: true},
	}); err != nil {
		return db.MachineDeviceCertificate{}, err
	}
	_ = s.recordAudit(ctx, organizationID, row.ID, compliance.ActionMachineDeviceCertRotated, map[string]any{
		"machine_id":      machineID.String(),
		"old_cert_id":     oldCertID.String(),
		"new_cert_id":     row.ID.String(),
		"new_serial":      newSerial,
		"old_fingerprint": certFingerprintHex(oldCert),
		"new_fingerprint": certFingerprintHex(newCert),
	})
	return row, nil
}

func sanSummary(cert *x509.Certificate) []string {
	var out []string
	for _, d := range cert.DNSNames {
		if s := d; s != "" {
			out = append(out, "dns:"+s)
		}
	}
	for _, u := range cert.URIs {
		if u != nil {
			out = append(out, "uri:"+u.String())
		}
	}
	return out
}

func (s *Service) recordAudit(ctx context.Context, organizationID, certID uuid.UUID, action string, metadata map[string]any) error {
	if s == nil || s.audit == nil {
		return nil
	}
	md, _ := json.Marshal(metadata)
	var resourceID *string
	if certID != uuid.Nil {
		v := certID.String()
		resourceID = &v
	}
	return s.audit.Record(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: organizationID,
		ActorType:      compliance.ActorService,
		Action:         action,
		ResourceType:   "machine_device_certificate",
		ResourceID:     resourceID,
		Metadata:       md,
		Outcome:        compliance.OutcomeSuccess,
	})
}

func certFingerprintHex(cert *x509.Certificate) string {
	if cert == nil {
		return ""
	}
	fp := sha256.Sum256(cert.Raw)
	return fmt.Sprintf("%x", fp[:])
}

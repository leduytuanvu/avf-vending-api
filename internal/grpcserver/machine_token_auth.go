package grpcserver

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type sqlMachineTokenCredentialChecker struct {
	q     *db.Queries
	audit compliance.EnterpriseRecorder
}

func NewSQLMachineTokenCredentialChecker(pool *pgxpool.Pool, audit ...compliance.EnterpriseRecorder) auth.MachineTokenCredentialChecker {
	if pool == nil {
		return nil
	}
	var rec compliance.EnterpriseRecorder
	if len(audit) > 0 {
		rec = audit[0]
	}
	return &sqlMachineTokenCredentialChecker{q: db.New(pool), audit: rec}
}

func (c *sqlMachineTokenCredentialChecker) ValidateMachineAccessClaims(ctx context.Context, claims auth.MachineAccessClaims) error {
	if c == nil || c.q == nil {
		return nil
	}
	if claims.MachineID == uuid.Nil || claims.OrganizationID == uuid.Nil {
		return status.Error(codes.Unauthenticated, "machine identity claims required")
	}
	if claims.SessionID != uuid.Nil {
		sg, err := c.q.GetMachineSessionGate(ctx, db.GetMachineSessionGateParams{
			ID:        claims.SessionID,
			MachineID: claims.MachineID,
		})
		if err != nil {
			c.recordFailure(ctx, claims, "session_not_found")
			return status.Error(codes.Unauthenticated, "machine session not found")
		}
		if sg.OrganizationID != claims.OrganizationID {
			c.recordFailure(ctx, claims, "session_organization_mismatch")
			return status.Error(codes.PermissionDenied, "machine organization mismatch")
		}
		now := time.Now().UTC()
		if strings.ToLower(strings.TrimSpace(sg.SessionStatus)) != "active" || sg.SessionRevokedAt.Valid || !now.Before(sg.SessionExpiresAt.UTC()) {
			c.recordFailure(ctx, claims, "session_inactive")
			return status.Error(codes.Unauthenticated, "machine session revoked or expired")
		}
		switch strings.ToLower(strings.TrimSpace(sg.CredentialStatus)) {
		case "active":
		default:
			c.recordFailure(ctx, claims, "credential_inactive")
			return status.Error(codes.Unauthenticated, "machine credential inactive")
		}
		if sg.MachineCredentialRevokedAt.Valid {
			c.recordFailure(ctx, claims, "credentials_revoked")
			return status.Error(codes.Unauthenticated, "machine credentials revoked")
		}
		if sg.MachineCredentialVersion != claims.CredentialVersion || sg.SessionCredentialVersion != claims.CredentialVersion {
			c.recordFailure(ctx, claims, "credential_version_mismatch")
			return status.Error(codes.Unauthenticated, "machine credential version mismatch")
		}
		st := strings.ToLower(strings.TrimSpace(sg.MachineStatus))
		switch st {
		case "active", "draft", "provisioned", "provisioning", "online", "offline":
		default:
			c.recordFailure(ctx, claims, "machine_not_active")
			return status.Error(codes.PermissionDenied, "machine not active")
		}
		_ = c.q.MarkMachineCredentialUsed(ctx, db.MarkMachineCredentialUsedParams{
			ID:             claims.MachineID,
			OrganizationID: claims.OrganizationID,
		})
		return nil
	}
	row, err := c.q.GetMachineCredentialGate(ctx, claims.MachineID)
	if err != nil {
		c.recordFailure(ctx, claims, "machine_not_found")
		return status.Error(codes.Unauthenticated, "machine not found")
	}
	if row.OrganizationID != claims.OrganizationID {
		c.recordFailure(ctx, claims, "organization_mismatch")
		return status.Error(codes.PermissionDenied, "machine organization mismatch")
	}
	if row.CredentialRevokedAt.Valid {
		c.recordFailure(ctx, claims, "credentials_revoked")
		return status.Error(codes.Unauthenticated, "machine credentials revoked")
	}
	if row.CredentialVersion != claims.CredentialVersion {
		c.recordFailure(ctx, claims, "credential_version_mismatch")
		return status.Error(codes.Unauthenticated, "machine credential version mismatch")
	}
	switch strings.ToLower(strings.TrimSpace(row.Status)) {
	case "active", "draft", "provisioned", "provisioning", "online", "offline":
	default:
		c.recordFailure(ctx, claims, "machine_not_active")
		return status.Error(codes.PermissionDenied, "machine not active")
	}
	_ = c.q.MarkMachineCredentialUsed(ctx, db.MarkMachineCredentialUsedParams{
		ID:             claims.MachineID,
		OrganizationID: claims.OrganizationID,
	})
	return nil
}

func (c *sqlMachineTokenCredentialChecker) recordFailure(ctx context.Context, claims auth.MachineAccessClaims, reason string) {
	if c == nil || c.audit == nil || claims.OrganizationID == uuid.Nil {
		return
	}
	mid := claims.MachineID.String()
	meta := map[string]any{
		"reason":             reason,
		"credential_version": claims.CredentialVersion,
		"token_use":          claims.TokenUse,
	}
	if claims.SessionID != uuid.Nil {
		meta["session_id"] = claims.SessionID.String()
	}
	md, _ := json.Marshal(meta)
	if len(md) == 0 {
		md = []byte("{}")
	}
	_ = c.audit.Record(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: claims.OrganizationID,
		ActorType:      compliance.ActorMachine,
		ActorID:        &mid,
		Action:         compliance.ActionMachineAuthFailed,
		ResourceType:   "machine",
		ResourceID:     &mid,
		Metadata:       md,
	})
}

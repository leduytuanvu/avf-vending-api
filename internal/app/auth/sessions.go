package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AdminAuthSessionView is a non-secret projection of admin_sessions for HTTP APIs.
type AdminAuthSessionView struct {
	SessionID  string  `json:"sessionId"`
	IPAddress  *string `json:"ipAddress,omitempty"`
	UserAgent  *string `json:"userAgent,omitempty"`
	CreatedAt  string  `json:"createdAt"`
	LastUsedAt *string `json:"lastUsedAt,omitempty"`
	ExpiresAt  string  `json:"expiresAt"`
	Status     string  `json:"status"`
}

func mapAdminSessionRow(row db.AdminSession) AdminAuthSessionView {
	out := AdminAuthSessionView{
		SessionID: row.ID.String(),
		CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339Nano),
		ExpiresAt: row.ExpiresAt.UTC().Format(time.RFC3339Nano),
		Status:    row.Status,
	}
	if row.IpAddress.Valid && strings.TrimSpace(row.IpAddress.String) != "" {
		v := strings.TrimSpace(row.IpAddress.String)
		out.IPAddress = &v
	}
	if row.UserAgent.Valid && strings.TrimSpace(row.UserAgent.String) != "" {
		v := strings.TrimSpace(row.UserAgent.String)
		out.UserAgent = &v
	}
	if row.LastUsedAt.Valid {
		v := row.LastUsedAt.Time.UTC().Format(time.RFC3339Nano)
		out.LastUsedAt = &v
	}
	return out
}

// ListMySessions returns device sessions for the authenticated account.
func (s *Service) ListMySessions(ctx context.Context, accountID uuid.UUID) ([]AdminAuthSessionView, error) {
	if s == nil {
		return nil, errors.New("auth service: nil")
	}
	if accountID == uuid.Nil {
		return nil, ErrInvalidRequest
	}
	_, err := s.q.AuthGetAccountByID(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	return s.listSessionsForAccount(ctx, uuid.Nil, accountID)
}

func (s *Service) listSessionsForAccount(ctx context.Context, companyID, userID uuid.UUID) ([]AdminAuthSessionView, error) {
	rows, err := s.q.AuthAdminListSessionsForAccount(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]AdminAuthSessionView, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapAdminSessionRow(row))
	}
	return out, nil
}

// AdminListUserSessions lists sessions for a target user (admin directory); actor must be authorized for scopeID.
func (s *Service) AdminListUserSessions(ctx context.Context, companyID uuid.UUID, targetUserID uuid.UUID) ([]AdminAuthSessionView, error) {
	if s == nil {
		return nil, errors.New("auth service: nil")
	}
	if companyID == uuid.Nil || targetUserID == uuid.Nil {
		return nil, ErrInvalidRequest
	}
	if _, err := s.q.AuthAdminGetAccountByOrgAndID(ctx, targetUserID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}
	return s.listSessionsForAccount(ctx, companyID, targetUserID)
}

// RevokeMySession revokes one refresh token / session belonging to the caller.
func (s *Service) RevokeMySession(ctx context.Context, accountID uuid.UUID, sessionID uuid.UUID) error {
	if s == nil {
		return errors.New("auth service: nil")
	}
	if accountID == uuid.Nil || sessionID == uuid.Nil {
		return ErrInvalidRequest
	}
	_, err := s.q.AuthGetAccountByID(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidCredentials
		}
		return err
	}
	sess, err := s.q.AuthAdminGetAdminSessionByUserAndID(ctx, db.AuthAdminGetAdminSessionByUserAndIDParams{
		ID:     sessionID,
		UserID: accountID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidRequest
		}
		return err
	}
	if err := s.revokeRefreshPair(ctx, accountID, sess.RefreshTokenID, sess.RefreshTokenHash); err != nil {
		return err
	}
	return s.auditMFASecurity(ctx, auditActionSessionRevokeSelf, uuid.Nil, accountID, map[string]any{
		"sessionId": sessionID.String(),
	}, compliance.OutcomeSuccess)
}

// RevokeMyOtherSessions revokes every active session except the one identified by the current refresh token.
func (s *Service) RevokeMyOtherSessions(ctx context.Context, accountID uuid.UUID, exceptRefreshToken string) error {
	if s == nil {
		return errors.New("auth service: nil")
	}
	if accountID == uuid.Nil || strings.TrimSpace(exceptRefreshToken) == "" {
		return ErrInvalidRequest
	}
	_, err := s.q.AuthGetAccountByID(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidCredentials
		}
		return err
	}
	exceptHash := plauth.HashRefreshToken(exceptRefreshToken)
	rows, err := s.q.AuthAdminListSessionsForAccount(ctx, accountID)
	if err != nil {
		return err
	}
	var revoked int
	for _, row := range rows {
		if !strings.EqualFold(strings.TrimSpace(row.Status), "active") {
			continue
		}
		if len(row.RefreshTokenHash) == len(exceptHash) && subtle.ConstantTimeCompare(row.RefreshTokenHash, exceptHash) == 1 {
			continue
		}
		if err := s.revokeRefreshPair(ctx, accountID, row.RefreshTokenID, row.RefreshTokenHash); err != nil {
			return err
		}
		revoked++
	}
	return s.auditMFASecurity(ctx, auditActionSessionsRevokedOthers, uuid.Nil, accountID, map[string]any{
		"revokedCount": revoked,
	}, compliance.OutcomeSuccess)
}

func (s *Service) revokeRefreshPair(ctx context.Context, accountID, refreshTokenID uuid.UUID, refreshHash []byte) error {
	if err := s.q.AuthRevokeRefreshToken(ctx, refreshTokenID); err != nil {
		return err
	}
	if _, err := s.q.AuthAdminRevokeAdminSessionByRefreshTokenID(ctx, db.AuthAdminRevokeAdminSessionByRefreshTokenIDParams{
		RefreshTokenID: refreshTokenID,
		UserID:         accountID,
	}); err != nil {
		return err
	}
	if s.sessionCache != nil && len(refreshHash) > 0 {
		_ = s.sessionCache.InvalidateRefreshSession(ctx, refreshHash)
	}
	return nil
}

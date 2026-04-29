package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

// MFATOTPEnrollResponse is returned when a pending TOTP factor is created.
type MFATOTPEnrollResponse struct {
	OTPAuthURI string `json:"otpauthUri"`
	Secret     string `json:"secret"`
}

// MFATOTPVerifyRequest completes MFA enrollment or an interactive login challenge.
type MFATOTPVerifyRequest struct {
	Code string `json:"code"`
}

// MFATOTPDisableRequest disables active TOTP after password + code verification.
type MFATOTPDisableRequest struct {
	CurrentPassword string `json:"currentPassword"`
	TOTPCode        string `json:"totpCode"`
}

func (s *Service) mfaAESKey() ([]byte, error) {
	if s == nil {
		return nil, errors.New("auth service: nil")
	}
	if len(s.adminSec.MFAEncryptionKey) == 32 {
		return s.adminSec.MFAEncryptionKey, nil
	}
	return nil, ErrMFANotConfigured
}

// MFATOTPEnrollBegin creates a pending TOTP factor. Allowed with a normal access token (optional MFA)
// or with an MFA enrollment challenge JWT (production-required MFA).
func (s *Service) MFATOTPEnrollBegin(ctx context.Context, p plauth.Principal) (*MFATOTPEnrollResponse, error) {
	if s == nil {
		return nil, errors.New("auth service: nil")
	}
	accountID, err := uuid.Parse(strings.TrimSpace(p.Subject))
	if err != nil || accountID == uuid.Nil {
		return nil, ErrInvalidRequest
	}
	tu := strings.ToLower(strings.TrimSpace(p.TokenUse))
	if tu == plauth.TokenUseMFAPending {
		if !p.MFAEnrollment {
			return nil, ErrInvalidRequest
		}
	} else if tu != "" && tu != plauth.TokenUseInteractiveAccess {
		return nil, ErrInvalidRequest
	}
	key, err := s.mfaAESKey()
	if err != nil {
		return nil, err
	}
	acct, err := s.q.AuthGetAccountByID(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if acct.OrganizationID != p.OrganizationID {
		return nil, ErrInvalidRequest
	}
	active, err := s.q.AuthAdminMFACountActiveForUser(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if active > 0 {
		return nil, ErrMFAConflict
	}
	if err := s.q.AuthAdminMFADeletePendingForUser(ctx, accountID); err != nil {
		return nil, err
	}
	otpKey, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "AVF Admin",
		AccountName: strings.TrimSpace(acct.Email),
	})
	if err != nil {
		return nil, err
	}
	ct, err := plauth.EncryptMFASecret(key, []byte(otpKey.Secret()))
	if err != nil {
		return nil, err
	}
	if _, err := s.q.AuthAdminMFAInsertPending(ctx, db.AuthAdminMFAInsertPendingParams{
		OrganizationID:   acct.OrganizationID,
		UserID:           accountID,
		SecretCiphertext: ct,
	}); err != nil {
		return nil, err
	}
	if err := s.auditMFASecurity(ctx, auditActionMFATOTPEnrollBegin, acct.OrganizationID, accountID, map[string]any{"email": acct.Email}, compliance.OutcomeSuccess); err != nil {
		return nil, err
	}
	return &MFATOTPEnrollResponse{
		OTPAuthURI: otpKey.URL(),
		Secret:     otpKey.Secret(),
	}, nil
}

// MFATOTPVerify activates enrollment or completes login after password verification.
func (s *Service) MFATOTPVerify(ctx context.Context, p plauth.Principal, req MFATOTPVerifyRequest) (*LoginResponse, error) {
	if s == nil {
		return nil, errors.New("auth service: nil")
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		return nil, ErrInvalidRequest
	}
	tu := strings.ToLower(strings.TrimSpace(p.TokenUse))
	isChallenge := tu == plauth.TokenUseMFAPending
	isInteractive := tu == "" || tu == plauth.TokenUseInteractiveAccess
	if !isChallenge && !isInteractive {
		return nil, ErrInvalidRequest
	}
	accountID, err := uuid.Parse(strings.TrimSpace(p.Subject))
	if err != nil || accountID == uuid.Nil {
		return nil, ErrInvalidRequest
	}
	keyMaterial, err := s.mfaAESKey()
	if err != nil {
		return nil, err
	}

	acct, err := s.q.AuthGetAccountByID(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if acct.OrganizationID != p.OrganizationID {
		return nil, ErrInvalidRequest
	}
	if strings.ToLower(strings.TrimSpace(acct.Status)) != "active" {
		s.auditLoginFailure(ctx, acct.OrganizationID, acct.Email, "account_disabled")
		return nil, ErrInvalidCredentials
	}

	var verifyPending bool
	var pendingFactorID uuid.UUID
	var secretStr string

	switch {
	case isChallenge && p.MFAEnrollment:
		pending, err := s.q.AuthAdminMFAPendingFactor(ctx, accountID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				s.auditMFATOTPFailure(ctx, acct, "no_pending_factor")
				return nil, ErrInvalidCredentials
			}
			return nil, err
		}
		verifyPending = true
		pendingFactorID = pending.ID
		plain, err := plauth.DecryptMFASecret(keyMaterial, pending.SecretCiphertext)
		if err != nil {
			return nil, err
		}
		secretStr = string(plain)
	case isInteractive:
		pending, err := s.q.AuthAdminMFAPendingFactor(ctx, accountID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrInvalidRequest
			}
			return nil, err
		}
		verifyPending = true
		pendingFactorID = pending.ID
		plain, err := plauth.DecryptMFASecret(keyMaterial, pending.SecretCiphertext)
		if err != nil {
			return nil, err
		}
		secretStr = string(plain)
	default:
		row, err := s.q.AuthAdminMFAActiveFactorCiphertext(ctx, accountID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				s.auditMFATOTPFailure(ctx, acct, "no_active_factor")
				return nil, ErrInvalidCredentials
			}
			return nil, err
		}
		plain, err := plauth.DecryptMFASecret(keyMaterial, row.SecretCiphertext)
		if err != nil {
			return nil, err
		}
		secretStr = string(plain)
	}

	if ok := totp.Validate(code, secretStr); !ok {
		s.auditMFATOTPFailure(ctx, acct, "invalid_totp")
		return nil, ErrInvalidCredentials
	}

	if verifyPending {
		if _, err := s.q.AuthAdminMFAActivateFactor(ctx, db.AuthAdminMFAActivateFactorParams{
			ID:     pendingFactorID,
			UserID: accountID,
		}); err != nil {
			return nil, err
		}
		if err := s.auditMFASecurity(ctx, auditActionMFATOTPActivated, acct.OrganizationID, accountID, map[string]any{"email": acct.Email}, compliance.OutcomeSuccess); err != nil {
			return nil, err
		}
	} else {
		if err := s.auditMFASecurity(ctx, auditActionMFALoginMFACompleted, acct.OrganizationID, accountID, map[string]any{"email": acct.Email}, compliance.OutcomeSuccess); err != nil {
			return nil, err
		}
	}

	_ = s.q.AuthRecordLoginSuccess(ctx, accountID)
	if s.loginFailures != nil {
		email := strings.TrimSpace(strings.ToLower(acct.Email))
		_ = s.loginFailures.ClearFailures(ctx, acct.OrganizationID, email)
	}
	out, err := s.issueLoginResponse(ctx, acct)
	if err != nil {
		return nil, err
	}
	if err := s.auditLoginSuccess(ctx, acct); err != nil {
		_ = s.q.AuthRevokeAllRefreshForAccount(ctx, acct.ID)
		return nil, fmt.Errorf("audit: %w", err)
	}
	return out, nil
}

// MFATOTPDisable turns off the active TOTP factor and revokes refresh tokens.
func (s *Service) MFATOTPDisable(ctx context.Context, accountID uuid.UUID, req MFATOTPDisableRequest) error {
	if s == nil {
		return errors.New("auth service: nil")
	}
	if accountID == uuid.Nil || strings.TrimSpace(req.CurrentPassword) == "" || strings.TrimSpace(req.TOTPCode) == "" {
		return ErrInvalidRequest
	}
	keyMaterial, err := s.mfaAESKey()
	if err != nil {
		return err
	}
	acct, err := s.q.AuthGetAccountByID(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidCredentials
		}
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(acct.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		return ErrInvalidCredentials
	}
	row, err := s.q.AuthAdminMFAActiveFactorCiphertext(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMFAConflict
		}
		return err
	}
	plain, err := plauth.DecryptMFASecret(keyMaterial, row.SecretCiphertext)
	if err != nil {
		return err
	}
	if ok := totp.Validate(strings.TrimSpace(req.TOTPCode), string(plain)); !ok {
		s.auditMFATOTPFailure(ctx, acct, "invalid_totp_disable")
		return ErrInvalidCredentials
	}
	orgID := acct.OrganizationID
	err = s.runPasswordTxn(ctx, func(tx pgx.Tx, q *db.Queries) error {
		if _, err := q.AuthAdminMFADisableActiveTOTP(ctx, accountID); err != nil {
			return err
		}
		if err := q.AuthRevokeAllRefreshForAccount(ctx, accountID); err != nil {
			return err
		}
		return q.AuthAdminRevokeAllAdminSessionsForUser(ctx, db.AuthAdminRevokeAllAdminSessionsForUserParams{
			OrganizationID: orgID,
			UserID:         accountID,
		})
	})
	if err != nil {
		return err
	}
	if s.sessionCache != nil {
		_ = s.sessionCache.InvalidateAccountSessions(ctx, accountID)
	}
	if s.accessRevocation != nil {
		ttl := s.i.AccessTokenTTL()
		if ttl > 0 {
			_ = s.accessRevocation.RevokeSubject(ctx, accountID.String(), ttl)
		}
	}
	return s.auditMFASecurity(ctx, auditActionMFATOTPDisabled, orgID, accountID, map[string]any{"email": acct.Email}, compliance.OutcomeSuccess)
}

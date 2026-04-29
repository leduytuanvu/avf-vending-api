package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/listscope"
	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

// AllowedRoles enumerates platform_auth_accounts.roles values accepted by admin APIs (explicit whitelist).
var AllowedRoles = []string{
	plauth.RolePlatformAdmin,
	plauth.RoleOrgAdmin,
	plauth.RoleOrgMember,
	"catalog_manager",
	"fleet_manager",
	"technician_manager",
	"inventory_manager",
	plauth.RoleTechnician,
	"finance",
	"finance_admin",
	"support",
	"viewer",
}

var allowedRoleSet map[string]struct{}

func init() {
	allowedRoleSet = make(map[string]struct{}, len(AllowedRoles))
	for _, r := range AllowedRoles {
		allowedRoleSet[r] = struct{}{}
	}
}

const (
	authAuditCreateUser         = "auth_admin.create_user"
	authAuditPatchUser          = "auth_admin.patch_user"
	authAuditActivateUser       = "auth_admin.activate_user"
	authAuditDeactivateUser     = "auth_admin.deactivate_user"
	authAuditResetPassword      = "auth_admin.reset_password"
	authAuditRequestReset       = "auth.request_password_reset"
	authAuditConfirmReset       = "auth.confirm_password_reset"
	authAuditRevokeSessions     = "auth_admin.revoke_sessions"
	authAuditSetRoles           = "auth_admin.set_roles"
	authAuditRemoveRole         = "auth_admin.remove_role"
	authAuditSelfChangePassword = "auth.change_password"
)

// --- Requests / responses ---

// AdminCreateUserRequest is the application payload for POST /v1/admin/auth/users.
type AdminCreateUserRequest struct {
	Email    string
	Password string
	Roles    []string
	Status   string
}

// AdminPatchUserRequest carries optional updates for PATCH /v1/admin/auth/users/{accountId}.
type AdminPatchUserRequest struct {
	Email  *string
	Roles  *[]string
	Status *string
}

// AdminAccountView is a safe projection without credential material.
type AdminAccountView struct {
	AccountID      string   `json:"accountId"`
	OrganizationID string   `json:"organizationId"`
	Email          string   `json:"email"`
	Roles          []string `json:"roles"`
	Status         string   `json:"status"`
	CreatedAt      string   `json:"createdAt"`
	UpdatedAt      string   `json:"updatedAt"`
}

// AdminUsersListResponse is returned by GET /v1/admin/auth/users.
type AdminUsersListResponse struct {
	Items []AdminAccountView       `json:"items"`
	Meta  listscope.CollectionMeta `json:"meta"`
}

// ResetPasswordRequest is the JSON body for admin reset-password.
type ResetPasswordRequest struct {
	Password string `json:"password"`
}

// ChangePasswordRequest is POST /v1/auth/change-password (caller authenticated).
type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// PasswordResetRequest starts password reset without leaking whether the email exists.
type PasswordResetRequest struct {
	OrganizationID uuid.UUID `json:"organizationId"`
	Email          string    `json:"email"`
}

// PasswordResetIssueResult is returned by the application service. HTTP handlers intentionally omit ResetToken.
type PasswordResetIssueResult struct {
	Accepted   bool      `json:"accepted"`
	ResetToken string    `json:"-"`
	ExpiresAt  time.Time `json:"-"`
}

// PasswordResetConfirmRequest consumes a short-lived one-time token.
type PasswordResetConfirmRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

func mapAcctPublic(a db.PlatformAuthAccount) AdminAccountView {
	roles := append([]string(nil), a.Roles...)
	sort.Strings(roles)
	return AdminAccountView{
		AccountID:      a.ID.String(),
		OrganizationID: a.OrganizationID.String(),
		Email:          a.Email,
		Roles:          roles,
		Status:         a.Status,
		CreatedAt:      a.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:      a.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func normalizeEmail(s string) (string, error) {
	v := strings.TrimSpace(strings.ToLower(s))
	if v == "" {
		return "", ErrInvalidEmail
	}
	if _, err := mail.ParseAddress(v); err != nil {
		return "", ErrInvalidEmail
	}
	return v, nil
}

func validateRolesNonEmpty(roles []string) ([]string, error) {
	if len(roles) == 0 {
		return nil, ErrInvalidRequest
	}
	out := make([]string, 0, len(roles))
	seen := map[string]struct{}{}
	for _, r := range roles {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if r == plauth.RoleMachine {
			return nil, fmt.Errorf("%w: %s", ErrInvalidRole, r)
		}
		if _, ok := allowedRoleSet[r]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrInvalidRole, r)
		}
		if _, dup := seen[r]; dup {
			continue
		}
		seen[r] = struct{}{}
		out = append(out, r)
	}
	if len(out) == 0 {
		return nil, ErrInvalidRequest
	}
	sort.Strings(out)
	return out, nil
}

func statusAllowed(s string) bool {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "active", "disabled", "locked", "invited":
		return true
	default:
		return false
	}
}

func accountContributesOrgAdmin(status string, roles []string) bool {
	if strings.TrimSpace(strings.ToLower(status)) != "active" {
		return false
	}
	for _, r := range roles {
		if r == plauth.RoleOrgAdmin {
			return true
		}
	}
	return false
}

func (s *Service) emitAdminMutation(ctx context.Context, tx pgx.Tx, e AuthAdminMutationEvent) error {
	if s == nil || s.onAdminMutation == nil {
		return nil
	}
	return s.onAdminMutation(ctx, tx, e)
}

func (s *Service) runPasswordTxn(ctx context.Context, fn func(tx pgx.Tx, q *db.Queries) error) error {
	if s.pool == nil {
		return errors.New("auth service: Postgres pool required for password mutations")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	if err := fn(tx, q); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) guardRemoveFinalOrgAdmin(ctx context.Context, orgID uuid.UUID, excludeAccountID uuid.UUID, beforeContrib, afterContrib bool) error {
	if !beforeContrib || afterContrib {
		return nil
	}
	others, err := s.q.AuthAdminCountActiveOrgAdminsExcluding(ctx, db.AuthAdminCountActiveOrgAdminsExcludingParams{
		OrganizationID: orgID,
		ID:             excludeAccountID,
	})
	if err != nil {
		return err
	}
	if others == 0 {
		return ErrForbiddenLastOrgAdmin
	}
	return nil
}

// AdminListUsers returns paginated auth accounts for an organization.
func (s *Service) AdminListUsers(ctx context.Context, organizationID uuid.UUID, limit, offset int32) (*AdminUsersListResponse, error) {
	if s == nil {
		return nil, errors.New("auth service: nil")
	}
	if organizationID == uuid.Nil {
		return nil, ErrInvalidRequest
	}
	total, err := s.q.AuthAdminCountAccounts(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.AuthAdminListAccounts(ctx, db.AuthAdminListAccountsParams{
		OrganizationID: organizationID,
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		return nil, err
	}
	items := make([]AdminAccountView, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapAcctPublic(row))
	}
	return &AdminUsersListResponse{
		Items: items,
		Meta: listscope.CollectionMeta{
			Limit:    limit,
			Offset:   offset,
			Returned: len(items),
			Total:    total,
		},
	}, nil
}

// AdminCreateUser creates an auth account for an organization.
func (s *Service) AdminCreateUser(ctx context.Context, actorAccountID uuid.UUID, organizationID uuid.UUID, req AdminCreateUserRequest) (*AdminAccountView, error) {
	if s == nil {
		return nil, errors.New("auth service: nil")
	}
	if organizationID == uuid.Nil || actorAccountID == uuid.Nil {
		return nil, ErrInvalidRequest
	}
	if err := s.validatePassword(req.Password); err != nil {
		return nil, err
	}
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return nil, err
	}
	roles, err := validateRolesNonEmpty(req.Roles)
	if err != nil {
		return nil, err
	}
	status := strings.TrimSpace(strings.ToLower(req.Status))
	if status == "" {
		status = "active"
	}
	if !statusAllowed(status) {
		return nil, ErrInvalidRequest
	}
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	if s.pool == nil {
		return nil, errors.New("auth service: Postgres pool required for AdminCreateUser")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.q.WithTx(tx)
	row, err := qtx.AuthAdminInsertAccount(ctx, db.AuthAdminInsertAccountParams{
		OrganizationID: organizationID,
		Email:          email,
		PasswordHash:   string(hashBytes),
		Roles:          roles,
		Status:         status,
	})
	if err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23505" {
			return nil, ErrConflictDuplicateEmail
		}
		return nil, err
	}
	if err := s.emitAdminMutation(ctx, tx, AuthAdminMutationEvent{
		Action:          authAuditCreateUser,
		OrganizationID:  organizationID,
		ActorAccountID:  actorAccountID,
		TargetAccountID: row.ID,
		Details:         map[string]any{"email": row.Email},
	}); err != nil {
		return nil, fmt.Errorf("audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	out := mapAcctPublic(row)
	return &out, nil
}

// AdminReplaceUserRoles is PUT /v1/admin/users/{id}/roles — replaces the role list only.
func (s *Service) AdminReplaceUserRoles(ctx context.Context, actorAccountID uuid.UUID, organizationID uuid.UUID, accountID uuid.UUID, roles []string) (*AdminAccountView, error) {
	if s == nil {
		return nil, errors.New("auth service: nil")
	}
	cur, err := s.q.AuthAdminGetAccountByOrgAndID(ctx, db.AuthAdminGetAccountByOrgAndIDParams{
		ID:             accountID,
		OrganizationID: organizationID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}
	before := accountContributesOrgAdmin(cur.Status, cur.Roles)
	newRoles, err := validateRolesNonEmpty(roles)
	if err != nil {
		return nil, err
	}
	after := accountContributesOrgAdmin(cur.Status, newRoles)
	if err := s.guardRemoveFinalOrgAdmin(ctx, organizationID, accountID, before, after); err != nil {
		return nil, err
	}
	if s.pool == nil {
		return nil, errors.New("auth service: Postgres pool required for AdminReplaceUserRoles")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.q.WithTx(tx)
	row, err := qtx.AuthAdminUpdateAccount(ctx, db.AuthAdminUpdateAccountParams{
		ID:             accountID,
		OrganizationID: organizationID,
		Email:          cur.Email,
		Roles:          newRoles,
		Status:         cur.Status,
	})
	if err != nil {
		return nil, err
	}
	if err := qtx.AuthRevokeAllRefreshForAccount(ctx, accountID); err != nil {
		return nil, err
	}
	if err := s.emitAdminMutation(ctx, tx, AuthAdminMutationEvent{
		Action:          authAuditSetRoles,
		OrganizationID:  organizationID,
		ActorAccountID:  actorAccountID,
		TargetAccountID: accountID,
		Details: map[string]any{
			"roles_before": append([]string(nil), cur.Roles...),
			"roles_after":  append([]string(nil), row.Roles...),
		},
	}); err != nil {
		return nil, fmt.Errorf("audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if s.accessRevocation != nil {
		ttl := s.i.AccessTokenTTL()
		if ttl > 0 {
			_ = s.accessRevocation.RevokeSubject(ctx, accountID.String(), ttl)
		}
	}
	out := mapAcctPublic(row)
	return &out, nil
}

// AdminRemoveUserRole removes a single role (DELETE .../roles/{role}). At least one role must remain.
func (s *Service) AdminRemoveUserRole(ctx context.Context, actorAccountID uuid.UUID, organizationID uuid.UUID, accountID uuid.UUID, role string) (*AdminAccountView, error) {
	if s == nil {
		return nil, errors.New("auth service: nil")
	}
	role = strings.TrimSpace(role)
	if role == "" {
		return nil, ErrInvalidRequest
	}
	if role == plauth.RoleMachine {
		return nil, fmt.Errorf("%w: %s", ErrInvalidRole, role)
	}
	if _, ok := allowedRoleSet[role]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrInvalidRole, role)
	}
	cur, err := s.q.AuthAdminGetAccountByOrgAndID(ctx, db.AuthAdminGetAccountByOrgAndIDParams{
		ID:             accountID,
		OrganizationID: organizationID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}
	before := accountContributesOrgAdmin(cur.Status, cur.Roles)
	newRoles := make([]string, 0, len(cur.Roles))
	removed := false
	for _, r := range cur.Roles {
		if r == role {
			removed = true
			continue
		}
		newRoles = append(newRoles, r)
	}
	if !removed {
		v := mapAcctPublic(cur)
		return &v, nil
	}
	if len(newRoles) == 0 {
		return nil, errors.Join(ErrInvalidRequest, errors.New("at least one role is required"))
	}
	sort.Strings(newRoles)
	after := accountContributesOrgAdmin(cur.Status, newRoles)
	if err := s.guardRemoveFinalOrgAdmin(ctx, organizationID, accountID, before, after); err != nil {
		return nil, err
	}
	if s.pool == nil {
		return nil, errors.New("auth service: Postgres pool required for AdminRemoveUserRole")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.q.WithTx(tx)
	row, err := qtx.AuthAdminUpdateAccount(ctx, db.AuthAdminUpdateAccountParams{
		ID:             accountID,
		OrganizationID: organizationID,
		Email:          cur.Email,
		Roles:          newRoles,
		Status:         cur.Status,
	})
	if err != nil {
		return nil, err
	}
	if err := qtx.AuthRevokeAllRefreshForAccount(ctx, accountID); err != nil {
		return nil, err
	}
	if err := s.emitAdminMutation(ctx, tx, AuthAdminMutationEvent{
		Action:          authAuditRemoveRole,
		OrganizationID:  organizationID,
		ActorAccountID:  actorAccountID,
		TargetAccountID: accountID,
		Details: map[string]any{
			"role_removed": role,
			"roles_before": append([]string(nil), cur.Roles...),
			"roles_after":  append([]string(nil), row.Roles...),
		},
	}); err != nil {
		return nil, fmt.Errorf("audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if s.accessRevocation != nil {
		ttl := s.i.AccessTokenTTL()
		if ttl > 0 {
			_ = s.accessRevocation.RevokeSubject(ctx, accountID.String(), ttl)
		}
	}
	out := mapAcctPublic(row)
	return &out, nil
}

// AdminGetUser returns one account scoped by organization.
func (s *Service) AdminGetUser(ctx context.Context, organizationID uuid.UUID, accountID uuid.UUID) (*AdminAccountView, error) {
	if s == nil {
		return nil, errors.New("auth service: nil")
	}
	row, err := s.q.AuthAdminGetAccountByOrgAndID(ctx, db.AuthAdminGetAccountByOrgAndIDParams{
		ID:             accountID,
		OrganizationID: organizationID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}
	v := mapAcctPublic(row)
	return &v, nil
}

// AdminPatchUser applies partial updates.
func (s *Service) AdminPatchUser(ctx context.Context, actorAccountID uuid.UUID, organizationID uuid.UUID, accountID uuid.UUID, req AdminPatchUserRequest) (*AdminAccountView, error) {
	if s == nil {
		return nil, errors.New("auth service: nil")
	}
	cur, err := s.q.AuthAdminGetAccountByOrgAndID(ctx, db.AuthAdminGetAccountByOrgAndIDParams{
		ID:             accountID,
		OrganizationID: organizationID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}
	before := accountContributesOrgAdmin(cur.Status, cur.Roles)

	email := cur.Email
	if req.Email != nil {
		email, err = normalizeEmail(*req.Email)
		if err != nil {
			return nil, err
		}
	}
	roles := append([]string(nil), cur.Roles...)
	if req.Roles != nil {
		roles, err = validateRolesNonEmpty(*req.Roles)
		if err != nil {
			return nil, err
		}
	}
	status := cur.Status
	if req.Status != nil {
		status = strings.TrimSpace(strings.ToLower(*req.Status))
		if !statusAllowed(status) {
			return nil, ErrInvalidRequest
		}
	}

	after := accountContributesOrgAdmin(status, roles)
	if err := s.guardRemoveFinalOrgAdmin(ctx, organizationID, accountID, before, after); err != nil {
		return nil, err
	}

	if s.pool == nil {
		return nil, errors.New("auth service: Postgres pool required for AdminPatchUser")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.q.WithTx(tx)
	row, err := qtx.AuthAdminUpdateAccount(ctx, db.AuthAdminUpdateAccountParams{
		ID:             accountID,
		OrganizationID: organizationID,
		Email:          email,
		Roles:          roles,
		Status:         status,
	})
	if err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23505" {
			return nil, ErrConflictDuplicateEmail
		}
		return nil, err
	}
	if req.Roles != nil {
		if err := qtx.AuthRevokeAllRefreshForAccount(ctx, accountID); err != nil {
			return nil, err
		}
	}
	if err := s.emitAdminMutation(ctx, tx, AuthAdminMutationEvent{
		Action:          authAuditPatchUser,
		OrganizationID:  organizationID,
		ActorAccountID:  actorAccountID,
		TargetAccountID: accountID,
		Details: map[string]any{
			"roles_before": append([]string(nil), cur.Roles...),
			"roles_after":  append([]string(nil), row.Roles...),
		},
	}); err != nil {
		return nil, fmt.Errorf("audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if req.Roles != nil && s.accessRevocation != nil {
		ttl := s.i.AccessTokenTTL()
		if ttl > 0 {
			_ = s.accessRevocation.RevokeSubject(ctx, accountID.String(), ttl)
		}
	}
	out := mapAcctPublic(row)
	return &out, nil
}

// AdminActivate sets status to active.
func (s *Service) AdminActivateUser(ctx context.Context, actorAccountID uuid.UUID, organizationID uuid.UUID, accountID uuid.UUID) (*AdminAccountView, error) {
	if s == nil {
		return nil, errors.New("auth service: nil")
	}
	cur, err := s.q.AuthAdminGetAccountByOrgAndID(ctx, db.AuthAdminGetAccountByOrgAndIDParams{
		ID:             accountID,
		OrganizationID: organizationID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}
	before := accountContributesOrgAdmin(cur.Status, cur.Roles)
	after := accountContributesOrgAdmin("active", cur.Roles)
	if err := s.guardRemoveFinalOrgAdmin(ctx, organizationID, accountID, before, after); err != nil {
		return nil, err
	}
	if s.pool == nil {
		return nil, errors.New("auth service: Postgres pool required for AdminActivateUser")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.q.WithTx(tx)
	row, err := qtx.AuthAdminUpdateAccount(ctx, db.AuthAdminUpdateAccountParams{
		ID:             accountID,
		OrganizationID: organizationID,
		Email:          cur.Email,
		Roles:          append([]string(nil), cur.Roles...),
		Status:         "active",
	})
	if err != nil {
		return nil, err
	}
	if err := s.emitAdminMutation(ctx, tx, AuthAdminMutationEvent{
		Action:          authAuditActivateUser,
		OrganizationID:  organizationID,
		ActorAccountID:  actorAccountID,
		TargetAccountID: accountID,
	}); err != nil {
		return nil, fmt.Errorf("audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	out := mapAcctPublic(row)
	return &out, nil
}

// AdminDeactivate sets status to disabled.
func (s *Service) AdminDeactivateUser(ctx context.Context, actorAccountID uuid.UUID, organizationID uuid.UUID, accountID uuid.UUID) (*AdminAccountView, error) {
	if s == nil {
		return nil, errors.New("auth service: nil")
	}
	cur, err := s.q.AuthAdminGetAccountByOrgAndID(ctx, db.AuthAdminGetAccountByOrgAndIDParams{
		ID:             accountID,
		OrganizationID: organizationID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}
	before := accountContributesOrgAdmin(cur.Status, cur.Roles)
	if err := s.guardRemoveFinalOrgAdmin(ctx, organizationID, accountID, before, false); err != nil {
		return nil, err
	}
	if s.pool == nil {
		return nil, errors.New("auth service: Postgres pool required for AdminDeactivateUser")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.q.WithTx(tx)
	row, err := qtx.AuthAdminUpdateAccount(ctx, db.AuthAdminUpdateAccountParams{
		ID:             accountID,
		OrganizationID: organizationID,
		Email:          cur.Email,
		Roles:          append([]string(nil), cur.Roles...),
		Status:         "disabled",
	})
	if err != nil {
		return nil, err
	}
	if err := qtx.AuthRevokeAllRefreshForAccount(ctx, accountID); err != nil {
		return nil, err
	}
	if err := s.emitAdminMutation(ctx, tx, AuthAdminMutationEvent{
		Action:          authAuditDeactivateUser,
		OrganizationID:  organizationID,
		ActorAccountID:  actorAccountID,
		TargetAccountID: accountID,
	}); err != nil {
		return nil, fmt.Errorf("audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if s.accessRevocation != nil {
		ttl := s.i.AccessTokenTTL()
		if ttl > 0 {
			_ = s.accessRevocation.RevokeSubject(ctx, accountID.String(), ttl)
		}
	}
	out := mapAcctPublic(row)
	return &out, nil
}

// AdminResetPassword sets a new bcrypt hash and revokes refresh tokens.
func (s *Service) AdminResetPassword(ctx context.Context, actorAccountID uuid.UUID, organizationID uuid.UUID, accountID uuid.UUID, req ResetPasswordRequest) (*AdminAccountView, error) {
	if s == nil {
		return nil, errors.New("auth service: nil")
	}
	if err := s.validatePassword(req.Password); err != nil {
		return nil, err
	}
	if _, err := s.q.AuthAdminGetAccountByOrgAndID(ctx, db.AuthAdminGetAccountByOrgAndIDParams{
		ID:             accountID,
		OrganizationID: organizationID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	hash := string(hashBytes)
	err = s.runPasswordTxn(ctx, func(tx pgx.Tx, q *db.Queries) error {
		if err := q.AuthAdminSetPasswordHash(ctx, db.AuthAdminSetPasswordHashParams{
			ID:           accountID,
			PasswordHash: hash,
		}); err != nil {
			return err
		}
		if err := q.AuthRevokeAllRefreshForAccount(ctx, accountID); err != nil {
			return err
		}
		return s.emitAdminMutation(ctx, tx, AuthAdminMutationEvent{
			Action:          authAuditResetPassword,
			OrganizationID:  organizationID,
			ActorAccountID:  actorAccountID,
			TargetAccountID: accountID,
		})
	})
	if err != nil {
		return nil, err
	}
	row, err := s.q.AuthAdminGetAccountByOrgAndID(ctx, db.AuthAdminGetAccountByOrgAndIDParams{
		ID:             accountID,
		OrganizationID: organizationID,
	})
	if err != nil {
		return nil, err
	}
	out := mapAcctPublic(row)
	return &out, nil
}

// AdminRevokeUserSessions revokes all refresh tokens for a target account.
func (s *Service) AdminRevokeUserSessions(ctx context.Context, actorAccountID uuid.UUID, organizationID uuid.UUID, accountID uuid.UUID) error {
	if s == nil {
		return errors.New("auth service: nil")
	}
	if actorAccountID == uuid.Nil || organizationID == uuid.Nil || accountID == uuid.Nil {
		return ErrInvalidRequest
	}
	if _, err := s.q.AuthAdminGetAccountByOrgAndID(ctx, db.AuthAdminGetAccountByOrgAndIDParams{
		ID:             accountID,
		OrganizationID: organizationID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrAccountNotFound
		}
		return err
	}
	if s.pool == nil {
		return errors.New("auth service: Postgres pool required for AdminRevokeUserSessions")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.q.WithTx(tx)
	if err := qtx.AuthRevokeAllRefreshForAccount(ctx, accountID); err != nil {
		return err
	}
	if err := qtx.AuthAdminRevokeAllAdminSessionsForUser(ctx, db.AuthAdminRevokeAllAdminSessionsForUserParams{
		OrganizationID: organizationID,
		UserID:         accountID,
	}); err != nil {
		return err
	}
	if err := s.emitAdminMutation(ctx, tx, AuthAdminMutationEvent{
		Action:          authAuditRevokeSessions,
		OrganizationID:  organizationID,
		ActorAccountID:  actorAccountID,
		TargetAccountID: accountID,
	}); err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
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
	return nil
}

// ChangePassword lets an authenticated user rotate their password and invalidates refresh tokens.
func (s *Service) ChangePassword(ctx context.Context, accountID uuid.UUID, req ChangePasswordRequest) error {
	if s == nil {
		return errors.New("auth service: nil")
	}
	if accountID == uuid.Nil {
		return ErrInvalidRequest
	}
	if err := s.validatePassword(req.NewPassword); err != nil {
		return err
	}
	if strings.TrimSpace(req.CurrentPassword) == "" {
		return ErrInvalidRequest
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
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	hash := string(hashBytes)
	orgID := acct.OrganizationID
	err = s.runPasswordTxn(ctx, func(tx pgx.Tx, q *db.Queries) error {
		if err := q.AuthAdminSetPasswordHash(ctx, db.AuthAdminSetPasswordHashParams{
			ID:           accountID,
			PasswordHash: hash,
		}); err != nil {
			return err
		}
		if err := q.AuthRevokeAllRefreshForAccount(ctx, accountID); err != nil {
			return err
		}
		if err := q.AuthAdminRevokeAllAdminSessionsForUser(ctx, db.AuthAdminRevokeAllAdminSessionsForUserParams{
			OrganizationID: orgID,
			UserID:         accountID,
		}); err != nil {
			return err
		}
		return s.emitAdminMutation(ctx, tx, AuthAdminMutationEvent{
			Action:          authAuditSelfChangePassword,
			OrganizationID:  orgID,
			ActorAccountID:  accountID,
			TargetAccountID: accountID,
		})
	})
	return err
}

// RequestPasswordReset creates a one-time reset token when the account exists and always returns Accepted.
func (s *Service) RequestPasswordReset(ctx context.Context, req PasswordResetRequest) (*PasswordResetIssueResult, error) {
	if s == nil {
		return nil, errors.New("auth service: nil")
	}
	if req.OrganizationID == uuid.Nil {
		return nil, ErrInvalidRequest
	}
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return nil, ErrInvalidRequest
	}
	out := &PasswordResetIssueResult{Accepted: true}
	acct, err := s.q.AuthLookupAccountByOrgEmailAnyStatus(ctx, db.AuthLookupAccountByOrgEmailAnyStatusParams{
		OrganizationID: req.OrganizationID,
		Lower:          email,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return out, nil
		}
		return nil, err
	}
	if strings.TrimSpace(strings.ToLower(acct.Status)) == "disabled" {
		return out, nil
	}
	raw, hash, err := plauth.NewRefreshToken()
	if err != nil {
		return nil, err
	}
	exp := time.Now().UTC().Add(s.adminSec.PasswordResetTTL)
	tokID := uuid.New()
	if s.pool == nil {
		return nil, errors.New("auth service: Postgres pool required for RequestPasswordReset")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.q.WithTx(tx)
	if err := qtx.AuthInsertPasswordResetToken(ctx, db.AuthInsertPasswordResetTokenParams{
		ID:             tokID,
		UserID:         acct.ID,
		OrganizationID: acct.OrganizationID,
		TokenHash:      hash,
		ExpiresAt:      exp,
	}); err != nil {
		return nil, err
	}
	if err := s.emitAdminMutation(ctx, tx, AuthAdminMutationEvent{
		Action:          authAuditRequestReset,
		OrganizationID:  acct.OrganizationID,
		ActorAccountID:  acct.ID,
		TargetAccountID: acct.ID,
	}); err != nil {
		return nil, fmt.Errorf("audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	out.ResetToken = raw
	out.ExpiresAt = exp
	return out, nil
}

// ConfirmPasswordReset consumes a reset token, sets a new password hash, and revokes refresh sessions.
func (s *Service) ConfirmPasswordReset(ctx context.Context, req PasswordResetConfirmRequest) error {
	if s == nil {
		return errors.New("auth service: nil")
	}
	if strings.TrimSpace(req.Token) == "" {
		return ErrInvalidRequest
	}
	if err := s.validatePassword(req.NewPassword); err != nil {
		return err
	}
	row, err := s.q.AuthGetPasswordResetTokenByHash(ctx, plauth.HashRefreshToken(req.Token))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidResetToken
		}
		return err
	}
	acct, err := s.q.AuthGetAccountByID(ctx, row.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidResetToken
		}
		return err
	}
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	hash := string(hashBytes)
	err = s.runPasswordTxn(ctx, func(tx pgx.Tx, q *db.Queries) error {
		if err := q.AuthMarkPasswordResetTokenUsed(ctx, row.ID); err != nil {
			return err
		}
		if err := q.AuthAdminSetPasswordHash(ctx, db.AuthAdminSetPasswordHashParams{
			ID:           row.UserID,
			PasswordHash: hash,
		}); err != nil {
			return err
		}
		if err := q.AuthRevokeAllRefreshForAccount(ctx, row.UserID); err != nil {
			return err
		}
		if err := q.AuthAdminRevokeAllAdminSessionsForUser(ctx, db.AuthAdminRevokeAllAdminSessionsForUserParams{
			OrganizationID: acct.OrganizationID,
			UserID:         row.UserID,
		}); err != nil {
			return err
		}
		return s.emitAdminMutation(ctx, tx, AuthAdminMutationEvent{
			Action:          authAuditConfirmReset,
			OrganizationID:  acct.OrganizationID,
			ActorAccountID:  acct.ID,
			TargetAccountID: acct.ID,
		})
	})
	return err
}

package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	appauth "github.com/avf/avf-vending-api/internal/app/auth"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func mountAdminAuthUserRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.Auth == nil {
		return
	}
	if writeRL == nil {
		writeRL = func(h http.Handler) http.Handler { return h }
	}
	registerAdminAuthUserRoutes(r, app.Auth, writeRL, "/auth/users", "accountId")
	registerAdminAuthUserRoutes(r, app.Auth, writeRL, "/users", "userId")
	registerAdminAuthUserRoutes(r, app.Auth, writeRL, "/organizations/{organizationId}/users", "userId")
}

func registerAdminAuthUserRoutes(r chi.Router, svc *appauth.Service, writeRL func(http.Handler) http.Handler, base, idParam string) {
	r.Route(base, func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAnyPermission(auth.PermUserRead))
			r.Get("/", getAdminAuthUsers(svc))
			r.Get("/{"+idParam+"}", getAdminAuthUserByID(svc, idParam))
			r.Get("/{"+idParam+"}/sessions", getAdminAuthUserSessions(svc, idParam))
		})
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAnyPermission(auth.PermUserWrite))
			r.With(writeRL).Post("/", postAdminAuthUsersCreate(svc))
			r.With(writeRL).Patch("/{"+idParam+"}", patchAdminAuthUser(svc, idParam))
			r.With(writeRL).Patch("/{"+idParam+"}/status", patchAdminAuthUserStatus(svc, idParam))
			r.With(writeRL).Post("/{"+idParam+"}/activate", postAdminAuthUserActivate(svc, idParam))
			r.With(writeRL).Post("/{"+idParam+"}/deactivate", postAdminAuthUserDeactivate(svc, idParam))
			r.With(writeRL).Post("/{"+idParam+"}/enable", postAdminAuthUserActivate(svc, idParam))
			r.With(writeRL).Post("/{"+idParam+"}/disable", postAdminAuthUserDeactivate(svc, idParam))
			r.With(writeRL).Post("/{"+idParam+"}/reset-password", postAdminAuthUserResetPassword(svc, idParam))
		})
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAnyPermission(auth.PermUserSessionsRevoke))
			r.With(writeRL).Post("/{"+idParam+"}/revoke-sessions", postAdminAuthUserRevokeSessions(svc, idParam))
		})
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAnyPermission(auth.PermUserRoles))
			r.With(writeRL).Post("/{"+idParam+"}/roles", putAdminAuthUserRoles(svc, idParam))
			r.With(writeRL).Put("/{"+idParam+"}/roles", putAdminAuthUserRoles(svc, idParam))
			r.With(writeRL).Patch("/{"+idParam+"}/roles", putAdminAuthUserRoles(svc, idParam))
			r.With(writeRL).Delete("/{"+idParam+"}/roles/{role}", deleteAdminAuthUserRole(svc, idParam))
		})
	})
}

func getAdminAuthUsers(svc *appauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminAuthOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_pagination", err.Error())
			return
		}
		out, err := svc.AdminListUsers(r.Context(), orgID, limit, offset)
		if err != nil {
			writeAuthAdminMutationError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func postAdminAuthUsersCreate(svc *appauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminAuthOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		actorID, ok := principalAccountID(r)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		var body struct {
			Email    string   `json:"email"`
			Password string   `json:"password"`
			Roles    []string `json:"roles"`
			Status   string   `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		out, err := svc.AdminCreateUser(r.Context(), actorID, orgID, appauth.AdminCreateUserRequest{
			Email:    body.Email,
			Password: body.Password,
			Roles:    body.Roles,
			Status:   body.Status,
		})
		if err != nil {
			writeAuthAdminMutationError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusCreated, out)
	}
}

func getAdminAuthUserByID(svc *appauth.Service, idParam string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminAuthOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		id, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, idParam)))
		if err != nil || id == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_account_id", "invalid user id")
			return
		}
		out, err := svc.AdminGetUser(r.Context(), orgID, id)
		if err != nil {
			writeAuthAdminMutationError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func patchAdminAuthUser(svc *appauth.Service, idParam string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminAuthOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		actorID, ok := principalAccountID(r)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		id, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, idParam)))
		if err != nil || id == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_account_id", "invalid user id")
			return
		}
		var body appauth.AdminPatchUserRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		if body.Roles != nil {
			p, ok := auth.PrincipalFromContext(r.Context())
			if !ok || !auth.HasPermission(p, auth.PermUserRoles) {
				writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", "role changes require user:roles")
				return
			}
		}
		out, err := svc.AdminPatchUser(r.Context(), actorID, orgID, id, body)
		if err != nil {
			writeAuthAdminMutationError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func patchAdminAuthUserStatus(svc *appauth.Service, idParam string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminAuthOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		actorID, ok := principalAccountID(r)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		id, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, idParam)))
		if err != nil || id == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_account_id", "invalid user id")
			return
		}
		var body struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		st := strings.TrimSpace(body.Status)
		out, err := svc.AdminPatchUser(r.Context(), actorID, orgID, id, appauth.AdminPatchUserRequest{Status: &st})
		if err != nil {
			writeAuthAdminMutationError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func putAdminAuthUserRoles(svc *appauth.Service, idParam string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminAuthOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		actorID, ok := principalAccountID(r)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		id, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, idParam)))
		if err != nil || id == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_account_id", "invalid user id")
			return
		}
		var body struct {
			Roles []string `json:"roles"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		out, err := svc.AdminReplaceUserRoles(r.Context(), actorID, orgID, id, body.Roles)
		if err != nil {
			writeAuthAdminMutationError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func deleteAdminAuthUserRole(svc *appauth.Service, idParam string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminAuthOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		actorID, ok := principalAccountID(r)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		id, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, idParam)))
		if err != nil || id == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_account_id", "invalid user id")
			return
		}
		rawRole := strings.TrimSpace(chi.URLParam(r, "role"))
		role, err := url.PathUnescape(rawRole)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_role", "invalid role")
			return
		}
		out, err := svc.AdminRemoveUserRole(r.Context(), actorID, orgID, id, role)
		if err != nil {
			writeAuthAdminMutationError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func postAdminAuthUserActivate(svc *appauth.Service, idParam string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminAuthOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		actorID, ok := principalAccountID(r)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		id, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, idParam)))
		if err != nil || id == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_account_id", "invalid user id")
			return
		}
		out, err := svc.AdminActivateUser(r.Context(), actorID, orgID, id)
		if err != nil {
			writeAuthAdminMutationError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func postAdminAuthUserDeactivate(svc *appauth.Service, idParam string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminAuthOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		actorID, ok := principalAccountID(r)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		id, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, idParam)))
		if err != nil || id == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_account_id", "invalid user id")
			return
		}
		out, err := svc.AdminDeactivateUser(r.Context(), actorID, orgID, id)
		if err != nil {
			writeAuthAdminMutationError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func postAdminAuthUserResetPassword(svc *appauth.Service, idParam string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminAuthOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		actorID, ok := principalAccountID(r)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		id, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, idParam)))
		if err != nil || id == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_account_id", "invalid user id")
			return
		}
		var body appauth.ResetPasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		out, err := svc.AdminResetPassword(r.Context(), actorID, orgID, id, body)
		if err != nil {
			writeAuthAdminMutationError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func postAdminAuthUserRevokeSessions(svc *appauth.Service, idParam string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminAuthOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		actorID, ok := principalAccountID(r)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		id, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, idParam)))
		if err != nil || id == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_account_id", "invalid user id")
			return
		}
		if err := svc.AdminRevokeUserSessions(r.Context(), actorID, orgID, id); err != nil {
			writeAuthAdminMutationError(w, r.Context(), err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func getAdminAuthUserSessions(svc *appauth.Service, idParam string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminAuthOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		uid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, idParam)))
		if err != nil || uid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_account_id", "invalid user id")
			return
		}
		out, err := svc.AdminListUserSessions(r.Context(), orgID, uid)
		if err != nil {
			writeAuthAdminMutationError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"sessions": out})
	}
}

func adminAuthOrganizationID(r *http.Request) (uuid.UUID, error) {
	if raw := strings.TrimSpace(chi.URLParam(r, "organizationId")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil {
			return uuid.Nil, errors.New("invalid organization id")
		}
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			return uuid.Nil, errors.New("missing principal")
		}
		if !auth.CanAccessOrganizationAdminData(p, id) {
			return uuid.Nil, errors.New("organization scope mismatch")
		}
		return id, nil
	}
	return adminCatalogOrganizationID(r)
}

func principalAccountID(r *http.Request) (uuid.UUID, bool) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(strings.TrimSpace(p.Subject))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, false
	}
	return id, true
}

func postAuthChangePassword(svc *appauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := principalAccountID(r)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		var body appauth.ChangePasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		if err := svc.ChangePassword(r.Context(), id, body); err != nil {
			writeAuthChangePasswordError(w, r.Context(), err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postAuthPasswordResetRequest(svc *appauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body appauth.PasswordResetRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		out, err := svc.RequestPasswordReset(attachTransportMeta(r.Context(), r), body)
		if err != nil {
			writeAuthChangePasswordError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]bool{"accepted": out.Accepted})
	}
}

func postAuthPasswordResetConfirm(svc *appauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body appauth.PasswordResetConfirmRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		if err := svc.ConfirmPasswordReset(attachTransportMeta(r.Context(), r), body); err != nil {
			writeAuthChangePasswordError(w, r.Context(), err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func writeAuthAdminMutationError(w http.ResponseWriter, ctx context.Context, err error) {
	switch {
	case errors.Is(err, appauth.ErrInvalidRequest):
		writeAPIError(w, ctx, http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, appauth.ErrInvalidEmail):
		writeAPIError(w, ctx, http.StatusBadRequest, "invalid_email", err.Error())
	case errors.Is(err, appauth.ErrWeakPassword):
		writeAPIError(w, ctx, http.StatusBadRequest, "weak_password", err.Error())
	case errors.Is(err, appauth.ErrInvalidRole):
		writeAPIError(w, ctx, http.StatusBadRequest, "invalid_role", err.Error())
	case errors.Is(err, appauth.ErrAccountNotFound):
		writeAPIError(w, ctx, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, appauth.ErrConflictDuplicateEmail):
		writeAPIError(w, ctx, http.StatusConflict, "duplicate_email", err.Error())
	case errors.Is(err, appauth.ErrForbiddenLastOrgAdmin):
		writeAPIError(w, ctx, http.StatusForbidden, "last_org_admin", err.Error())
	default:
		writeAPIError(w, ctx, http.StatusInternalServerError, "internal", err.Error())
	}
}

func writeAuthChangePasswordError(w http.ResponseWriter, ctx context.Context, err error) {
	switch {
	case errors.Is(err, appauth.ErrInvalidRequest):
		writeAPIError(w, ctx, http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, appauth.ErrWeakPassword):
		writeAPIError(w, ctx, http.StatusBadRequest, "weak_password", err.Error())
	case errors.Is(err, appauth.ErrMFAConflict):
		writeAPIError(w, ctx, http.StatusConflict, "mfa_conflict", err.Error())
	case errors.Is(err, appauth.ErrMFANotConfigured):
		writeAPIError(w, ctx, http.StatusServiceUnavailable, "mfa_misconfigured", "MFA encryption is not configured")
	case errors.Is(err, appauth.ErrInvalidCredentials):
		writeAPIError(w, ctx, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
	case errors.Is(err, appauth.ErrInvalidResetToken):
		writeAPIError(w, ctx, http.StatusUnauthorized, "invalid_reset_token", "invalid reset token")
	default:
		writeAPIError(w, ctx, http.StatusInternalServerError, "internal", err.Error())
	}
}

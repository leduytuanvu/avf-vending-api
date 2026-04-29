package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	appauth "github.com/avf/avf-vending-api/internal/app/auth"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	appmw "github.com/avf/avf-vending-api/internal/middleware"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func httpClientIP(r *http.Request) string {
	addr := strings.TrimSpace(r.RemoteAddr)
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

func attachTransportMeta(ctx context.Context, r *http.Request) context.Context {
	rid := appmw.RequestIDFromContext(ctx)
	corr := appmw.CorrelationIDFromContext(ctx)
	return compliance.WithTransportMeta(ctx, compliance.TransportMeta{
		RequestID: rid,
		TraceID:   corr,
		IP:        httpClientIP(r),
		UserAgent: r.UserAgent(),
	})
}

func auditTransportMetaMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(attachTransportMeta(r.Context(), r)))
	})
}

func mountAuthRoutes(r chi.Router, app *api.HTTPApplication, abuse *AbuseProtection, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.Auth == nil {
		return
	}
	svc := app.Auth
	if abuse == nil {
		abuse = &AbuseProtection{}
	}
	if writeRL == nil {
		writeRL = func(next http.Handler) http.Handler { return next }
	}
	r.With(writeRL, abuse.LoginPOST()).Post("/login", postAuthLogin(svc))
	r.With(writeRL, abuse.RefreshPOST()).Post("/refresh", postAuthRefresh(svc))
	r.With(writeRL, abuse.PasswordResetRequestPOST()).Post("/password/reset/request", postAuthPasswordResetRequest(svc))
	r.With(writeRL, abuse.PasswordResetConfirmPOST()).Post("/password/reset/confirm", postAuthPasswordResetConfirm(svc))
}

// mountAuthBearerSessionRoutes registers MFA challenge routes, interactive session routes, and session management.
// Caller must wrap this router with bearer auth + observability middleware.
func mountAuthBearerSessionRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.Auth == nil {
		return
	}
	svc := app.Auth
	if writeRL == nil {
		writeRL = func(next http.Handler) http.Handler { return next }
	}
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireInteractiveAccountActive)
		r.Use(auth.RequireMFAPendingOrInteractiveAccess)
		r.With(writeRL).Post("/mfa/totp/enroll", postAuthMFATOTPEnroll(svc))
		r.With(writeRL).Post("/mfa/totp/verify", postAuthMFATOTPVerify(svc))
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireInteractiveAccountActive)
		r.Use(auth.RequireInteractiveAccessToken)
		mountAuthBearerRoutes(r, app)
		r.Get("/sessions", getAuthSessions(svc))
		r.Delete("/sessions/{sessionId}", deleteAuthSession(svc))
		r.With(writeRL).Delete("/sessions", deleteAuthSessionsExceptCurrent(svc))
		r.With(writeRL).Post("/mfa/totp/disable", postAuthMFATOTPDisable(svc))
	})
}

func mountAuthBearerRoutes(r chi.Router, app *api.HTTPApplication) {
	if app == nil || app.Auth == nil {
		return
	}
	svc := app.Auth
	r.Get("/me", getAuthMe(svc))
	r.Post("/logout", postAuthLogout(svc))
	r.Post("/change-password", postAuthChangePassword(svc))
	r.Post("/password/change", postAuthChangePassword(svc))
}

func postAuthMFATOTPEnroll(svc *appauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		ctx := attachTransportMeta(r.Context(), r)
		out, err := svc.MFATOTPEnrollBegin(ctx, p)
		if err != nil {
			writeAuthServiceError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func postAuthMFATOTPVerify(svc *appauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		var req appauth.MFATOTPVerifyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		ctx := attachTransportMeta(r.Context(), r)
		out, err := svc.MFATOTPVerify(ctx, p, req)
		if err != nil {
			writeAuthServiceError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func postAuthMFATOTPDisable(svc *appauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := principalAccountID(r)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		var req appauth.MFATOTPDisableRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		ctx := attachTransportMeta(r.Context(), r)
		if err := svc.MFATOTPDisable(ctx, id, req); err != nil {
			writeAuthServiceError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func getAuthSessions(svc *appauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := principalAccountID(r)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		out, err := svc.ListMySessions(r.Context(), id)
		if err != nil {
			writeAuthServiceError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"sessions": out})
	}
}

func deleteAuthSession(svc *appauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := principalAccountID(r)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		sid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "sessionId")))
		if err != nil || sid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_session_id", "invalid session id")
			return
		}
		ctx := attachTransportMeta(r.Context(), r)
		if err := svc.RevokeMySession(ctx, id, sid); err != nil {
			writeAuthServiceError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteAuthSessionsExceptCurrent(svc *appauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := principalAccountID(r)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		var body struct {
			ExceptRefreshToken string `json:"exceptRefreshToken"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		ctx := attachTransportMeta(r.Context(), r)
		if err := svc.RevokeMyOtherSessions(ctx, id, body.ExceptRefreshToken); err != nil {
			writeAuthServiceError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postAuthLogin(svc *appauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req appauth.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		ctx := attachTransportMeta(r.Context(), r)
		out, err := svc.Login(ctx, req)
		if err != nil {
			writeAuthServiceError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func postAuthRefresh(svc *appauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req appauth.RefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		out, err := svc.Refresh(r.Context(), req)
		if err != nil {
			writeAuthServiceError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getAuthMe(svc *appauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		id, err := uuid.Parse(strings.TrimSpace(p.Subject))
		if err != nil || id == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		out, err := svc.Me(r.Context(), id)
		if err != nil {
			writeAuthServiceError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func postAuthLogout(svc *appauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		id, err := uuid.Parse(strings.TrimSpace(p.Subject))
		if err != nil || id == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		var req appauth.LogoutRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		ctx := attachTransportMeta(r.Context(), r)
		if err := svc.Logout(ctx, id, strings.TrimSpace(p.JTI), p.ExpiresAt, req); err != nil {
			writeAuthServiceError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func writeAuthServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, appauth.ErrInvalidRequest):
		writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, appauth.ErrMFAConflict):
		writeAPIError(w, r.Context(), http.StatusConflict, "mfa_conflict", err.Error())
	case errors.Is(err, appauth.ErrMFANotConfigured):
		writeAPIError(w, r.Context(), http.StatusServiceUnavailable, "mfa_misconfigured", "MFA encryption is not configured")
	case errors.Is(err, appauth.ErrInvalidCredentials):
		writeAPIError(w, r.Context(), http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
	case errors.Is(err, appauth.ErrInvalidRefreshToken):
		writeAPIError(w, r.Context(), http.StatusUnauthorized, "invalid_refresh_token", "invalid refresh token")
	case errors.Is(err, appauth.ErrInvalidResetToken):
		writeAPIError(w, r.Context(), http.StatusUnauthorized, "invalid_reset_token", "invalid reset token")
	default:
		writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
	}
}

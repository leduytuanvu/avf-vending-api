package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	appauth "github.com/avf/avf-vending-api/internal/app/auth"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func mountAuthRoutes(r chi.Router, app *api.HTTPApplication) {
	if app == nil || app.Auth == nil {
		return
	}
	svc := app.Auth
	r.Post("/login", postAuthLogin(svc))
	r.Post("/refresh", postAuthRefresh(svc))
}

func mountAuthBearerRoutes(r chi.Router, app *api.HTTPApplication) {
	if app == nil || app.Auth == nil {
		return
	}
	svc := app.Auth
	r.Get("/me", getAuthMe(svc))
	r.Post("/logout", postAuthLogout(svc))
}

func postAuthLogin(svc *appauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req appauth.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		out, err := svc.Login(r.Context(), req)
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
		if err := svc.Logout(r.Context(), id, req); err != nil {
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
	case errors.Is(err, appauth.ErrInvalidCredentials):
		writeAPIError(w, r.Context(), http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
	case errors.Is(err, appauth.ErrInvalidRefreshToken):
		writeAPIError(w, r.Context(), http.StatusUnauthorized, "invalid_refresh_token", "invalid refresh token")
	default:
		writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
	}
}

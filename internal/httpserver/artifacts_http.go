package httpserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/app/artifacts"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// mountArtifactAdminRoutes registers S3-backed artifact APIs under /v1/admin/artifacts/...
// when app.Artifacts is configured. Mutating routes use writeRL when rate limiting is enabled.
func mountArtifactAdminRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.Artifacts == nil {
		return
	}
	svc := app.Artifacts
	r.Route("/artifacts", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAnyPermission(auth.PermCatalogRead))
			r.Get("/", artifactListHandler(svc))
			r.Get("/{artifactId}", artifactGetHandler(svc))
			r.Get("/{artifactId}/download", artifactDownloadURLHandler(svc))
		})
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAnyPermission(auth.PermCatalogWrite))
			r.With(writeRL).Post("/", artifactReserveHandler(svc))
			r.With(writeRL).Put("/{artifactId}/content", artifactPutContentHandler(svc))
			r.With(writeRL).Delete("/{artifactId}", artifactDeleteHandler(svc))
		})
	})
}

func parseArtifactIDs(r *http.Request) (scopeID uuid.UUID, artifactID uuid.UUID, ok bool) {
	scopeID = uuid.Nil
	artRaw := strings.TrimSpace(chi.URLParam(r, "artifactId"))
	if artRaw == "" {
		return scopeID, uuid.Nil, true
	}
	a, err := uuid.Parse(artRaw)
	if err != nil {
		return uuid.Nil, uuid.Nil, false
	}
	return scopeID, a, true
}

func artifactScopeAllowed(p auth.Principal, scopeID uuid.UUID) bool {
	_ = scopeID
	return p.HasRole(auth.RolePlatformAdmin) || auth.HasPermission(p, auth.PermCatalogRead) || auth.HasPermission(p, auth.PermCatalogWrite)
}

func artifactReserveHandler(svc *artifacts.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
			return
		}
		scopeID, _, ok := parseArtifactIDs(r)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope_id", "invalid artifact route context")
			return
		}
		if !artifactScopeAllowed(p, scopeID) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		id, err := svc.ReserveArtifact(r.Context())
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"artifact_id": id.String(),
			"upload_path": "/v1/admin/artifacts/" + id.String() + "/content",
		})
	}
}

func artifactListHandler(svc *artifacts.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
			return
		}
		scopeID, _, ok := parseArtifactIDs(r)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope_id", "invalid artifact route context")
			return
		}
		if !artifactScopeAllowed(p, scopeID) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		items, err := svc.ListArtifacts(r.Context(), scopeID)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		out := make([]map[string]any, 0, len(items))
		for _, it := range items {
			out = append(out, artifactInfoView(it))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": out})
	}
}

func artifactGetHandler(svc *artifacts.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
			return
		}
		scopeID, artID, ok := parseArtifactIDs(r)
		if !ok || artID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_ids", "invalid artifactId")
			return
		}
		if !artifactScopeAllowed(p, scopeID) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		info, err := svc.GetInfo(r.Context(), scopeID, artID)
		if err != nil {
			writeArtifactAPIError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, artifactInfoView(info))
	}
}

func artifactDownloadURLHandler(svc *artifacts.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
			return
		}
		scopeID, artID, ok := parseArtifactIDs(r)
		if !ok || artID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_ids", "invalid artifactId")
			return
		}
		if !artifactScopeAllowed(p, scopeID) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		signed, exp, err := svc.PresignDownload(r.Context(), scopeID, artID)
		if err != nil {
			writeArtifactAPIError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"method":     signed.Method,
			"url":        signed.URL,
			"headers":    signed.Headers,
			"expires_at": exp.UTC().Format(time.RFC3339Nano),
		})
	}
}

func artifactPutContentHandler(svc *artifacts.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
			return
		}
		scopeID, artID, ok := parseArtifactIDs(r)
		if !ok || artID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_ids", "invalid artifactId")
			return
		}
		if !artifactScopeAllowed(p, scopeID) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		cl := strings.TrimSpace(r.Header.Get("Content-Length"))
		if cl == "" {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_content_length", "Content-Length is required")
			return
		}
		size, err := strconv.ParseInt(cl, 10, 64)
		if err != nil || size <= 0 {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_content_length", "Content-Length must be a positive integer")
			return
		}
		sha := strings.TrimSpace(r.Header.Get("X-Artifact-SHA256"))
		ct := strings.TrimSpace(r.Header.Get("Content-Type"))
		fn := strings.TrimSpace(r.Header.Get("X-Artifact-Filename"))
		if err := svc.PutContent(r.Context(), scopeID, artID, r.Body, size, ct, sha, fn); err != nil {
			writeArtifactAPIError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "stored", "artifact_id": artID.String()})
	}
}

func artifactDeleteHandler(svc *artifacts.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
			return
		}
		scopeID, artID, ok := parseArtifactIDs(r)
		if !ok || artID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_ids", "invalid artifactId")
			return
		}
		if !artifactScopeAllowed(p, scopeID) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		if err := svc.DeleteArtifact(r.Context(), scopeID, artID); err != nil {
			writeArtifactAPIError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "deleted", "artifact_id": artID.String()})
	}
}

func artifactInfoView(it artifacts.ArtifactInfo) map[string]any {
	m := map[string]any{
		"artifact_id":  it.ArtifactID.String(),
		"size_bytes":   it.Size,
		"content_type": it.ContentType,
		"etag":         it.ETag,
		"object_key":   it.ObjectKey,
	}
	if !it.LastModifiedUTC.IsZero() {
		m["updated_at"] = it.LastModifiedUTC.Format(time.RFC3339Nano)
	}
	if it.SHA256Hex != "" {
		m["sha256"] = it.SHA256Hex
	}
	if it.OriginalFilename != "" {
		m["original_filename"] = it.OriginalFilename
	}
	return m
}

func writeArtifactAPIError(w http.ResponseWriter, ctx context.Context, err error) {
	switch {
	case errors.Is(err, artifacts.ErrNotFound):
		writeAPIError(w, ctx, http.StatusNotFound, "artifact_not_found", err.Error())
	case errors.Is(err, artifacts.ErrInvalidArgument):
		writeAPIError(w, ctx, http.StatusBadRequest, "invalid_argument", err.Error())
	case errors.Is(err, artifacts.ErrChecksumMismatch), errors.Is(err, artifacts.ErrTrailingBytes):
		writeAPIError(w, ctx, http.StatusBadRequest, "artifact_integrity", err.Error())
	default:
		writeAPIError(w, ctx, http.StatusInternalServerError, "internal", err.Error())
	}
}

package httpserver

import (
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

// mountArtifactAdminRoutes registers S3-backed artifact APIs under /v1/admin/organizations/{orgId}/artifacts/...
// when app.Artifacts is configured. Mutating routes use writeRL when rate limiting is enabled.
func mountArtifactAdminRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.Artifacts == nil {
		return
	}
	svc := app.Artifacts
	r.With(writeRL).Route("/organizations/{orgId}/artifacts", func(r chi.Router) {
		r.Post("/", artifactReserveHandler(svc))
		r.Get("/", artifactListHandler(svc))
		r.Get("/{artifactId}", artifactGetHandler(svc))
		r.Get("/{artifactId}/download", artifactDownloadURLHandler(svc))
		r.Put("/{artifactId}/content", artifactPutContentHandler(svc))
		r.Delete("/{artifactId}", artifactDeleteHandler(svc))
	})
}

func parseOrgArtifactIDs(r *http.Request) (orgID uuid.UUID, artifactID uuid.UUID, ok bool) {
	orgRaw := strings.TrimSpace(chi.URLParam(r, "orgId"))
	o, err := uuid.Parse(orgRaw)
	if err != nil {
		return uuid.Nil, uuid.Nil, false
	}
	artRaw := strings.TrimSpace(chi.URLParam(r, "artifactId"))
	if artRaw == "" {
		return o, uuid.Nil, true
	}
	a, err := uuid.Parse(artRaw)
	if err != nil {
		return uuid.Nil, uuid.Nil, false
	}
	return o, a, true
}

func artifactOrgAllowed(p auth.Principal, orgID uuid.UUID) bool {
	if p.HasRole(auth.RolePlatformAdmin) {
		return true
	}
	return p.HasOrganization() && p.OrganizationID == orgID && p.HasRole(auth.RoleOrgAdmin)
}

func artifactReserveHandler(svc *artifacts.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, http.StatusUnauthorized, "unauthenticated", "unauthenticated")
			return
		}
		orgID, _, ok := parseOrgArtifactIDs(r)
		if !ok {
			writeAPIError(w, http.StatusBadRequest, "invalid_organization_id", "invalid orgId")
			return
		}
		if !artifactOrgAllowed(p, orgID) {
			writeAPIError(w, http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		id, err := svc.ReserveArtifact(r.Context())
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"artifact_id": id.String(),
			"upload_path": "/v1/admin/organizations/" + orgID.String() + "/artifacts/" + id.String() + "/content",
		})
	}
}

func artifactListHandler(svc *artifacts.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, http.StatusUnauthorized, "unauthenticated", "unauthenticated")
			return
		}
		orgID, _, ok := parseOrgArtifactIDs(r)
		if !ok {
			writeAPIError(w, http.StatusBadRequest, "invalid_organization_id", "invalid orgId")
			return
		}
		if !artifactOrgAllowed(p, orgID) {
			writeAPIError(w, http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		items, err := svc.ListArtifacts(r.Context(), orgID)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "internal", err.Error())
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
			writeAPIError(w, http.StatusUnauthorized, "unauthenticated", "unauthenticated")
			return
		}
		orgID, artID, ok := parseOrgArtifactIDs(r)
		if !ok || artID == uuid.Nil {
			writeAPIError(w, http.StatusBadRequest, "invalid_ids", "invalid orgId or artifactId")
			return
		}
		if !artifactOrgAllowed(p, orgID) {
			writeAPIError(w, http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		info, err := svc.GetInfo(r.Context(), orgID, artID)
		if err != nil {
			writeArtifactAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, artifactInfoView(info))
	}
}

func artifactDownloadURLHandler(svc *artifacts.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, http.StatusUnauthorized, "unauthenticated", "unauthenticated")
			return
		}
		orgID, artID, ok := parseOrgArtifactIDs(r)
		if !ok || artID == uuid.Nil {
			writeAPIError(w, http.StatusBadRequest, "invalid_ids", "invalid orgId or artifactId")
			return
		}
		if !artifactOrgAllowed(p, orgID) {
			writeAPIError(w, http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		signed, exp, err := svc.PresignDownload(r.Context(), orgID, artID)
		if err != nil {
			writeArtifactAPIError(w, err)
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
			writeAPIError(w, http.StatusUnauthorized, "unauthenticated", "unauthenticated")
			return
		}
		orgID, artID, ok := parseOrgArtifactIDs(r)
		if !ok || artID == uuid.Nil {
			writeAPIError(w, http.StatusBadRequest, "invalid_ids", "invalid orgId or artifactId")
			return
		}
		if !artifactOrgAllowed(p, orgID) {
			writeAPIError(w, http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		cl := strings.TrimSpace(r.Header.Get("Content-Length"))
		if cl == "" {
			writeAPIError(w, http.StatusBadRequest, "missing_content_length", "Content-Length is required")
			return
		}
		size, err := strconv.ParseInt(cl, 10, 64)
		if err != nil || size <= 0 {
			writeAPIError(w, http.StatusBadRequest, "invalid_content_length", "Content-Length must be a positive integer")
			return
		}
		sha := strings.TrimSpace(r.Header.Get("X-Artifact-SHA256"))
		ct := strings.TrimSpace(r.Header.Get("Content-Type"))
		fn := strings.TrimSpace(r.Header.Get("X-Artifact-Filename"))
		if err := svc.PutContent(r.Context(), orgID, artID, r.Body, size, ct, sha, fn); err != nil {
			writeArtifactAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "stored", "artifact_id": artID.String()})
	}
}

func artifactDeleteHandler(svc *artifacts.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, http.StatusUnauthorized, "unauthenticated", "unauthenticated")
			return
		}
		orgID, artID, ok := parseOrgArtifactIDs(r)
		if !ok || artID == uuid.Nil {
			writeAPIError(w, http.StatusBadRequest, "invalid_ids", "invalid orgId or artifactId")
			return
		}
		if !artifactOrgAllowed(p, orgID) {
			writeAPIError(w, http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		if err := svc.DeleteArtifact(r.Context(), orgID, artID); err != nil {
			writeArtifactAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "deleted", "artifact_id": artID.String()})
	}
}

func artifactInfoView(it artifacts.ArtifactInfo) map[string]any {
	m := map[string]any{
		"organization_id": it.OrganizationID.String(),
		"artifact_id":     it.ArtifactID.String(),
		"size_bytes":      it.Size,
		"content_type":    it.ContentType,
		"etag":            it.ETag,
		"object_key":      it.ObjectKey,
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

func writeArtifactAPIError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, artifacts.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "artifact_not_found", err.Error())
	case errors.Is(err, artifacts.ErrInvalidArgument):
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", err.Error())
	case errors.Is(err, artifacts.ErrChecksumMismatch), errors.Is(err, artifacts.ErrTrailingBytes):
		writeAPIError(w, http.StatusBadRequest, "artifact_integrity", err.Error())
	default:
		writeAPIError(w, http.StatusInternalServerError, "internal", err.Error())
	}
}

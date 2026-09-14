package httpserver

// rbac:inherited-mount — bootstrap routes are mounted under PermCatalogRead in admin_catalog_http.go.

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	appassignmentcatalog "github.com/avf/avf-vending-api/internal/app/assignmentcatalog"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func mountAdminProductBootstrapRoutes(r chi.Router, app *api.HTTPApplication) {
	if app == nil || app.CatalogAdmin == nil {
		return
	}
	svc := appassignmentcatalog.NewService(app.CatalogAdmin, nil, 0)
	r.Get("/products/bootstrap-manifest", getAdminProductBootstrapManifest(app, svc))
	r.Get("/products/bootstrap-bundle", getAdminProductBootstrapBundle(app, svc))
	r.Get("/products/delta", getAdminProductCatalogDelta(app, svc))
}

func getAdminProductBootstrapManifest(app *api.HTTPApplication, svc *appassignmentcatalog.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireCatalogPrincipalUUID(r, app.MediaAdmin); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		machineID := resolveBootstrapMachineID(r)
		snap, err := svc.BuildSnapshot(r.Context(), machineID)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		baseURL := requestPublicBaseURL(r)
		writeJSON(w, http.StatusOK, appassignmentcatalog.BuildManifestHeader(baseURL, snap))
	}
}

func getAdminProductBootstrapBundle(app *api.HTTPApplication, svc *appassignmentcatalog.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireCatalogPrincipalUUID(r, app.MediaAdmin); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		machineID := resolveBootstrapMachineID(r)
		catalogVersion, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("catalogVersion")), 10, 32)
		if err != nil || catalogVersion <= 0 {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_catalog_version", "catalogVersion is required")
			return
		}
		variant := strings.TrimSpace(r.URL.Query().Get("variant"))
		if variant == "" {
			variant = "thumb"
		}
		snap, err := svc.BuildSnapshot(r.Context(), machineID)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		if int32(catalogVersion) != snap.CatalogVersion {
			writeAPIError(w, r.Context(), http.StatusConflict, "catalog_version_mismatch", "catalogVersion is stale")
			return
		}
		data, err := appassignmentcatalog.BuildBootstrapBundleZip(snap, variant)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		appassignmentcatalog.WriteBootstrapBundleResponse(w, data)
	}
}

func getAdminProductCatalogDelta(app *api.HTTPApplication, svc *appassignmentcatalog.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireCatalogPrincipalUUID(r, app.MediaAdmin); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		machineID := resolveBootstrapMachineID(r)
		since, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("sinceCatalogVersion")), 10, 32)
		if err != nil {
			since = 0
		}
		delta, err := svc.BuildDelta(r.Context(), machineID, int32(since))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, appassignmentcatalog.BuildDeltaDTO(delta))
	}
}

func resolveBootstrapMachineID(r *http.Request) uuid.UUID {
	raw := strings.TrimSpace(r.URL.Query().Get("machine_id"))
	if raw == "" {
		return uuid.Nil
	}
	machineID, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil
	}
	return machineID
}

func requestPublicBaseURL(r *http.Request) string {
	scheme := "https"
	if r.TLS == nil {
		if xf := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); xf != "" {
			scheme = xf
		} else {
			scheme = "http"
		}
	}
	host := strings.TrimSpace(r.Host)
	if host == "" {
		host = "localhost"
	}
	return scheme + "://" + host + "/v1/admin"
}

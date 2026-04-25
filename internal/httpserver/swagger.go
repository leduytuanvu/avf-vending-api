package httpserver

import (
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/docs/swagger" // register OpenAPI with swag (init) + OpenAPIJSON()
	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger"
	"go.uber.org/zap"
)

// MountOpenAPIJSON serves only GET /swagger/doc.json (OpenAPI 3.0 JSON) without Bearer auth.
// Use when HTTP_OPENAPI_JSON_ENABLED=true and HTTP_SWAGGER_UI_ENABLED=false (e.g. production + Postman import).
func MountOpenAPIJSON(r chi.Router, log *zap.Logger) {
	if log != nil {
		log.Info("openapi_json_mount", zap.String("path", "/swagger/doc.json"))
	}
	r.Get("/swagger/doc.json", func(w http.ResponseWriter, _ *http.Request) {
		b := swagger.OpenAPIJSON()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	})
}

// MountSwaggerUI serves Swagger UI (HTML) and /swagger/doc.json (OpenAPI 3.0) without Bearer auth.
// Wire when config.Config.SwaggerUIEnabled is true.
//
// Chi mounts /swagger/* to the swag handler; bare GET /swagger would otherwise 404. Redirect to the UI entrypoint.
func MountSwaggerUI(r chi.Router, log *zap.Logger) {
	if log != nil {
		log.Info("swagger_ui_mount", zap.String("path_prefix", "/swagger"))
	}
	inner := httpSwagger.WrapHandler
	r.Mount("/swagger", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tail := strings.TrimPrefix(r.URL.Path, "/swagger")
		if tail == "" || tail == "/" {
			http.Redirect(w, r, "/swagger/index.html", http.StatusTemporaryRedirect)
			return
		}
		inner.ServeHTTP(w, r)
	}))
}

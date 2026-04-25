package httpserver

import (
	"net/http"
	"strings"

	_ "github.com/avf/avf-vending-api/docs/swagger" // register OpenAPI document with swag (side effect in init)
	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger"
	"go.uber.org/zap"
)

// MountSwaggerUI serves Swagger UI and /swagger/doc.json (OpenAPI 3.0) without Bearer auth.
// Wire only when config.Config.SwaggerUIEnabled is true (production defaults off when HTTP_SWAGGER_UI_ENABLED is unset).
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

package httpserver

import (
	"net/http"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
)

// mountAdminPaymentProviderRoutes registers read-only PSP registry metadata.
// Provider secrets and PATCH configuration remain environment-driven (no DB-backed provider config in this phase).
func mountAdminPaymentProviderRoutes(r chi.Router, app *api.HTTPApplication) {
	if app == nil || app.ListPaymentProviders == nil {
		return
	}
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermPaymentRead, auth.PermCommerceRead))
		r.Get("/payment/providers", getAdminPaymentProviders(app))
	})
}

func getAdminPaymentProviders(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items := app.ListPaymentProviders()
		writeJSON(w, http.StatusOK, map[string]any{
			"items": items,
		})
	}
}

package httpserver

import (
	"fmt"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/config"
)

// ValidateP0HTTPApplication ensures the API process has non-nil wiring for pilot P0 HTTP surfaces.
// Call from bootstrap before NewHTTPServer when serving real production traffic.
func ValidateP0HTTPApplication(cfg *config.Config, app *api.HTTPApplication) error {
	if cfg == nil {
		return fmt.Errorf("httpserver.ValidateP0HTTPApplication: nil config")
	}
	if cfg.AppEnv != config.AppEnvProduction {
		return nil
	}
	if app == nil {
		return fmt.Errorf("httpserver.ValidateP0HTTPApplication: nil HTTPApplication")
	}
	if app.Auth == nil {
		return fmt.Errorf("httpserver.ValidateP0HTTPApplication: nil Auth (session APIs)")
	}
	if app.CatalogAdmin == nil {
		return fmt.Errorf("httpserver.ValidateP0HTTPApplication: nil CatalogAdmin")
	}
	if app.InventoryAdmin == nil {
		return fmt.Errorf("httpserver.ValidateP0HTTPApplication: nil InventoryAdmin")
	}
	if app.Commerce == nil {
		return fmt.Errorf("httpserver.ValidateP0HTTPApplication: nil Commerce")
	}
	if app.Activation == nil {
		return fmt.Errorf("httpserver.ValidateP0HTTPApplication: nil Activation")
	}
	if app.TelemetryStore == nil {
		return fmt.Errorf("httpserver.ValidateP0HTTPApplication: nil TelemetryStore")
	}
	if app.MachineShadow == nil {
		return fmt.Errorf("httpserver.ValidateP0HTTPApplication: nil MachineShadow")
	}
	if app.RemoteCommands == nil {
		return fmt.Errorf("httpserver.ValidateP0HTTPApplication: nil RemoteCommands")
	}
	return nil
}

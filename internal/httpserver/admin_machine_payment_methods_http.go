package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/avf/avf-vending-api/internal/app/api"
	appmachinepaymentmethods "github.com/avf/avf-vending-api/internal/app/machinepaymentmethods"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
)

func mountAdminMachinePaymentMethodsRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.MachinePaymentMethods == nil {
		return
	}
	svc := app.MachinePaymentMethods
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermFleetRead))
		r.Get("/machines/{machineId}/payment-methods", getAdminMachinePaymentMethods(svc, app))
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermFleetWrite))
		r.With(writeRL).Put("/machines/{machineId}/payment-methods", putAdminMachinePaymentMethods(svc, app))
	})
}

type machinePaymentMethodWire struct {
	MethodKey string `json:"methodKey"`
	Enabled   bool   `json:"enabled"`
	SortOrder int32  `json:"sortOrder"`
}

type putMachinePaymentMethodsBody struct {
	Methods []machinePaymentMethodWire `json:"methods"`
}

func getAdminMachinePaymentMethods(svc *appmachinepaymentmethods.Service, app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, ok := resolveAdminMachineRef(w, r, app, "machineId")
		if !ok {
			return
		}
		view, err := svc.Get(r.Context(), identity.MachineID)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, mapMachinePaymentMethodsView(view))
	}
}

func putAdminMachinePaymentMethods(svc *appmachinepaymentmethods.Service, app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, ok := resolveAdminMachineRef(w, r, app, "machineId")
		if !ok {
			return
		}
		var body putMachinePaymentMethodsBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		methods := make([]appmachinepaymentmethods.MethodRow, 0, len(body.Methods))
		for _, m := range body.Methods {
			methods = append(methods, appmachinepaymentmethods.MethodRow{
				MethodKey: m.MethodKey,
				Enabled:   m.Enabled,
				SortOrder: m.SortOrder,
			})
		}
		view, err := svc.Replace(r.Context(), appmachinepaymentmethods.ReplaceInput{
			MachineID: identity.MachineID,
			Methods:   methods,
		})
		if err != nil {
			writeMachinePaymentMethodsPutError(w, r.Context(), err)
			return
		}
		if app.EnterpriseAudit != nil {
			mid := identity.MachineID.String()
			md, _ := json.Marshal(map[string]any{
				"machine_id": mid,
				"methods":    view.Methods,
			})
			at, aid := compliance.ActorUser, ""
			if p, ok := auth.PrincipalFromContext(r.Context()); ok {
				at, aid = p.Actor()
			}
			_ = app.EnterpriseAudit.Record(r.Context(), compliance.EnterpriseAuditRecord{
				ActorType:    at,
				ActorID:      stringPtrOrNil(aid),
				Action:       compliance.ActionMachineUpdated,
				ResourceType: "fleet.machine",
				ResourceID:   &mid,
				Metadata:     md,
				Outcome:      compliance.OutcomeSuccess,
			})
		}
		writeJSON(w, http.StatusOK, mapMachinePaymentMethodsView(view))
	}
}

func writeMachinePaymentMethodsPutError(w http.ResponseWriter, ctx context.Context, err error) {
	switch {
	case errors.Is(err, appmachinepaymentmethods.ErrInvalidMethodKey):
		writeAPIError(w, ctx, http.StatusBadRequest, "invalid_method_key", "invalid payment method key")
	case errors.Is(err, appmachinepaymentmethods.ErrUnsupportedMethodKey):
		writeAPIError(w, ctx, http.StatusBadRequest, "unsupported_method_key", "payment method not supported by deployment")
	default:
		writeAPIError(w, ctx, http.StatusInternalServerError, "internal", err.Error())
	}
}

func mapMachinePaymentMethodsView(view appmachinepaymentmethods.GetView) map[string]any {
	methods := make([]map[string]any, 0, len(view.Methods))
	for _, m := range view.Methods {
		methods = append(methods, map[string]any{
			"methodKey": m.MethodKey,
			"enabled":   m.Enabled,
			"sortOrder": m.SortOrder,
		})
	}
	return map[string]any{
		"machineId":           view.MachineID.String(),
		"configured":          view.Configured,
		"methods":             methods,
		"deploymentSupported": view.DeploymentSupported,
	}
}

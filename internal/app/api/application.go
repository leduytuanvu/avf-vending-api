package api

import (
	appartifacts "github.com/avf/avf-vending-api/internal/app/artifacts"
	appauth "github.com/avf/avf-vending-api/internal/app/auth"
	appcatalogadmin "github.com/avf/avf-vending-api/internal/app/catalogadmin"
	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	appcommerceadmin "github.com/avf/avf-vending-api/internal/app/commerceadmin"
	appdevice "github.com/avf/avf-vending-api/internal/app/device"
	appfleet "github.com/avf/avf-vending-api/internal/app/fleet"
	appfleetadmin "github.com/avf/avf-vending-api/internal/app/fleetadmin"
	appinventoryadmin "github.com/avf/avf-vending-api/internal/app/inventoryadmin"
	appoperator "github.com/avf/avf-vending-api/internal/app/operator"
	appreporting "github.com/avf/avf-vending-api/internal/app/reporting"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
)

// HTTPApplication groups versioned HTTP API application services (ports only at the HTTP boundary).
type HTTPApplication struct {
	Auth             *appauth.Service
	CatalogAdmin     *appcatalogadmin.Service
	InventoryAdmin   *appinventoryadmin.Service
	AdminMachines    MachinesAdminService
	AdminTechnicians TechniciansAdminService
	AdminAssignments AssignmentsAdminService
	AdminCommands    CommandsAdminService
	AdminOTA         OTAAdminService
	Payments         PaymentsService
	Orders           OrdersService
	MachineShadow    MachineShadowService
	MachineOperator  *appoperator.Service
	Commerce         *appcommerce.Service
	RemoteCommands   *appdevice.MQTTCommandDispatcher
	Artifacts        *appartifacts.Service
	// TelemetryStore serves read-only telemetry projection endpoints (rollups / incidents / snapshot).
	TelemetryStore *postgres.Store
	Reporting        ReportingService
}

// HTTPApplicationDeps wires real domain services for the HTTP API process.
type HTTPApplicationDeps struct {
	Store              *postgres.Store
	Fleet              *appfleet.Service
	Commerce           *appcommerce.Service
	MQTTCommandPublish appdevice.MQTTDispatchPublisher
	Artifacts          *appartifacts.Service
	HTTPAuth           config.HTTPAuthConfig
}

// NewHTTPApplication constructs HTTP ports backed by real adapters where they exist.
// Callers must supply a non-nil Store and Fleet service when DATABASE_URL is enabled for this process.
func NewHTTPApplication(deps HTTPApplicationDeps) *HTTPApplication {
	if deps.Store == nil {
		panic("api.NewHTTPApplication: nil Store")
	}
	if deps.Fleet == nil {
		panic("api.NewHTTPApplication: nil Fleet service")
	}
	if deps.Commerce == nil {
		panic("api.NewHTTPApplication: nil Commerce service")
	}
	pool := deps.Store.Pool()
	queries := db.New(pool)
	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(deps.HTTPAuth)
	if err != nil {
		panic("api.NewHTTPApplication: session issuer: " + err.Error())
	}
	authSvc, err := appauth.NewService(appauth.Deps{Queries: queries, Issuer: issuer})
	if err != nil {
		panic("api.NewHTTPApplication: auth service: " + err.Error())
	}
	catSvc, err := appcatalogadmin.NewService(queries)
	if err != nil {
		panic("api.NewHTTPApplication: catalog admin: " + err.Error())
	}
	invSvc, err := appinventoryadmin.NewService(queries)
	if err != nil {
		panic("api.NewHTTPApplication: inventory admin: " + err.Error())
	}
	machineOp := appoperator.NewServiceFromDeps(appoperator.Deps{
		Sessions:    postgres.NewOperatorRepository(pool),
		Machines:    postgres.NewMachineRepository(pool),
		Technicians: postgres.NewTechnicianRepository(pool),
		Assignments: postgres.NewTechnicianAssignmentRepository(pool),
	})
	fleetAdm, err := appfleetadmin.NewService(queries)
	if err != nil {
		panic("api.NewHTTPApplication: fleet admin: " + err.Error())
	}
	commerceAdm, err := appcommerceadmin.NewService(queries)
	if err != nil {
		panic("api.NewHTTPApplication: commerce admin: " + err.Error())
	}
	remoteCmd := appdevice.NewMQTTCommandDispatcher(appdevice.MQTTCommandDispatcherDeps{
		Workflow:  deps.Store,
		Store:     deps.Store,
		Publisher: deps.MQTTCommandPublish,
	})
	reportingSvc := appreporting.NewService(queries)

	return &HTTPApplication{
		Auth:             authSvc,
		CatalogAdmin:     catSvc,
		InventoryAdmin:   invSvc,
		AdminMachines:    fleetAdm,
		AdminTechnicians: fleetAdm,
		AdminAssignments: fleetAdm,
		AdminCommands:    fleetAdm,
		AdminOTA:         fleetAdm,
		Payments:         commerceAdm,
		Orders:           commerceAdm,
		MachineShadow:    NewSQLMachineShadow(pool),
		MachineOperator:  machineOp,
		Commerce:         deps.Commerce,
		RemoteCommands:   remoteCmd,
		Artifacts:        deps.Artifacts,
		TelemetryStore:   deps.Store,
		Reporting:        reportingSvc,
	}
}

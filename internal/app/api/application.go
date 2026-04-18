package api

import (
	appartifacts "github.com/avf/avf-vending-api/internal/app/artifacts"
	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	appdevice "github.com/avf/avf-vending-api/internal/app/device"
	appfleet "github.com/avf/avf-vending-api/internal/app/fleet"
	appoperator "github.com/avf/avf-vending-api/internal/app/operator"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
)

// HTTPApplication groups versioned HTTP API application services (ports only at the HTTP boundary).
type HTTPApplication struct {
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
}

// HTTPApplicationDeps wires real domain services for the HTTP API process.
type HTTPApplicationDeps struct {
	Store              *postgres.Store
	Fleet              *appfleet.Service
	Commerce           *appcommerce.Service
	MQTTCommandPublish appdevice.MQTTDispatchPublisher
	Artifacts          *appartifacts.Service
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
	machineOp := appoperator.NewServiceFromDeps(appoperator.Deps{
		Sessions:    postgres.NewOperatorRepository(pool),
		Machines:    postgres.NewMachineRepository(pool),
		Technicians: postgres.NewTechnicianRepository(pool),
		Assignments: postgres.NewTechnicianAssignmentRepository(pool),
	})
	var adminListsNotImplemented unimplementedV1AdminCollections
	var commerceListsNotImplemented unimplementedV1CommerceLists
	remoteCmd := appdevice.NewMQTTCommandDispatcher(appdevice.MQTTCommandDispatcherDeps{
		Workflow:  deps.Store,
		Store:     deps.Store,
		Publisher: deps.MQTTCommandPublish,
	})

	return &HTTPApplication{
		AdminMachines:    NewFleetMachinesAdmin(deps.Fleet),
		AdminTechnicians: adminListsNotImplemented,
		AdminAssignments: adminListsNotImplemented,
		AdminCommands:    adminListsNotImplemented,
		AdminOTA:         adminListsNotImplemented,
		Payments:         commerceListsNotImplemented,
		Orders:           commerceListsNotImplemented,
		MachineShadow:    NewSQLMachineShadow(pool),
		MachineOperator:  machineOp,
		Commerce:         deps.Commerce,
		RemoteCommands:   remoteCmd,
		Artifacts:        deps.Artifacts,
		TelemetryStore:   deps.Store,
	}
}

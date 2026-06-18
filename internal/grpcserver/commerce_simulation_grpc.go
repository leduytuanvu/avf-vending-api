package grpcserver

import (
	"os"
	"strings"

	"github.com/avf/avf-vending-api/internal/config"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Enterprise HIL test machine — simulated commerce allowed in production when explicitly tagged.
var simulationTestMachineID = uuid.MustParse("019ec0d7-0a68-7bb0-ace0-f1d4dd3b0054")

type simulationMeta struct {
	Simulated          bool
	SimulationRunID    string
	SimulationScenario string
	FakeBill           bool
	FakeBoard          bool
}

func parseSimulationContext(sim *machinev1.SimulationContext) simulationMeta {
	if sim == nil {
		return simulationMeta{}
	}
	return simulationMeta{
		Simulated:          sim.GetSimulated(),
		SimulationRunID:    strings.TrimSpace(sim.GetSimulationRunId()),
		SimulationScenario: strings.TrimSpace(sim.GetSimulationScenario()),
		FakeBill:           sim.GetFakeBill(),
		FakeBoard:          sim.GetFakeBoard(),
	}
}

func simulationMetaFromOrder(
	simulated bool,
	runID, scenario *string,
	fakeBill, fakeBoard bool,
) simulationMeta {
	run := ""
	if runID != nil {
		run = strings.TrimSpace(*runID)
	}
	scenarioText := ""
	if scenario != nil {
		scenarioText = strings.TrimSpace(*scenario)
	}
	return simulationMeta{
		Simulated:          simulated,
		SimulationRunID:    run,
		SimulationScenario: scenarioText,
		FakeBill:           fakeBill,
		FakeBoard:          fakeBoard,
	}
}

func validateSimulationCommerce(machineID uuid.UUID, meta simulationMeta, appEnv config.AppEnvironment) error {
	if !meta.Simulated {
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("ALLOW_SIMULATED_COMMERCE")), "true") {
		return nil
	}
	if appEnv != config.AppEnvProduction {
		return nil
	}
	if machineID == simulationTestMachineID {
		return nil
	}
	return status.Error(codes.PermissionDenied, "simulated commerce not allowed for this machine")
}

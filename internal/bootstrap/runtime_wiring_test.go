package bootstrap

import (
	"context"
	"strings"
	"testing"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/google/uuid"
)

func TestValidateRuntimeWiring_RequiresAuthAdapter(t *testing.T) {
	cfg := &config.Config{
		APIWiring: config.APIWiringRequirements{RequireAuthAdapter: true},
	}
	rt := &Runtime{}
	err := ValidateRuntimeWiring(cfg, rt)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "API_REQUIRE_AUTH_ADAPTER") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestValidateRuntimeWiring_AllowsNilDepsWhenNotRequired(t *testing.T) {
	cfg := &config.Config{}
	rt := &Runtime{}
	if err := ValidateRuntimeWiring(cfg, rt); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRuntimeWiring_AllowsRequiredMQTTPublisherWhenPresent(t *testing.T) {
	cfg := &config.Config{
		APIWiring: config.APIWiringRequirements{RequireMQTTPublisher: true},
	}
	rt := &Runtime{
		Deps: RuntimeDeps{
			MQTTPublisher: stubMQTTPublisher{},
		},
	}
	if err := ValidateRuntimeWiring(cfg, rt); err != nil {
		t.Fatal(err)
	}
}

type stubMQTTPublisher struct{}

func (stubMQTTPublisher) PublishDeviceDispatch(_ context.Context, _ uuid.UUID, _ []byte) error {
	return nil
}

func (stubMQTTPublisher) Health(_ context.Context) error {
	return nil
}

package device

import "testing"

func TestNewMQTTCommandDispatcher_nilStoreOrWorkflow(t *testing.T) {
	if NewMQTTCommandDispatcher(MQTTCommandDispatcherDeps{}) != nil {
		t.Fatal("expected nil")
	}
}

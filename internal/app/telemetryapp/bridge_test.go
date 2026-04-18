package telemetryapp

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"go.uber.org/zap"
)

func TestSelectIngestLegacyWithoutJetStream(t *testing.T) {
	log := zap.NewNop()
	st := &postgres.Store{} // nil pool — ingest not executed; only mode selection
	ing := SelectIngest(log, st, nil)
	if _, ok := ing.(*LegacyStoreIngest); !ok {
		t.Fatalf("expected LegacyStoreIngest, got %T", ing)
	}
}

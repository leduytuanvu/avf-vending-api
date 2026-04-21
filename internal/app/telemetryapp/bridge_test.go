//go:build !windows

package telemetryapp

import (
	"strings"
	"testing"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"go.uber.org/zap"
)

func TestSelectIngestLegacyWithoutJetStream(t *testing.T) {
	log := zap.NewNop()
	st := &postgres.Store{} // nil pool — ingest not executed; only mode selection
	ing, err := SelectIngest(log, st, nil, config.AppEnvDevelopment, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ing.(*LegacyStoreIngest); !ok {
		t.Fatalf("expected LegacyStoreIngest, got %T", ing)
	}
}

func TestSelectIngestProductionRequiresJetStream(t *testing.T) {
	log := zap.NewNop()
	st := &postgres.Store{}
	_, err := SelectIngest(log, st, nil, config.AppEnvProduction, false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "production") {
		t.Fatalf("unexpected: %v", err)
	}
}

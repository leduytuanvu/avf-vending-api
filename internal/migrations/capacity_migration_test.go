package migrations_test

import (
	"os"
	"strings"
	"testing"
)

func TestMigration00066_indexes_are_additive_if_present(t *testing.T) {
	b, err := os.ReadFile("../../migrations/00066_p25_capacity_indexes.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	raw := string(b)
	if !strings.Contains(raw, "CREATE INDEX IF NOT EXISTS") {
		t.Fatal("expected CREATE INDEX IF NOT EXISTS statements")
	}
	if strings.Contains(strings.ToUpper(raw), "DROP TABLE") || strings.Contains(strings.ToUpper(raw), "ALTER TABLE DROP") {
		t.Fatal("migration must not drop tables/columns")
	}
}

func TestMigration00071_cost_indexes_are_additive_if_present(t *testing.T) {
	b, err := os.ReadFile("../../migrations/00071_p23_capacity_cost_indexes.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	raw := string(b)
	if !strings.Contains(raw, "CREATE INDEX IF NOT EXISTS") {
		t.Fatal("expected CREATE INDEX IF NOT EXISTS statements")
	}
	if strings.Contains(strings.ToUpper(raw), "DROP TABLE") || strings.Contains(strings.ToUpper(raw), "ALTER TABLE DROP") {
		t.Fatal("migration must not drop tables/columns")
	}
}

func TestMigration00072_operational_anomaly_types_is_safe(t *testing.T) {
	b, err := os.ReadFile("../../migrations/00072_p24_operational_anomaly_types.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	raw := string(b)
	if !strings.Contains(raw, "ALTER TABLE inventory_anomalies") {
		t.Fatal("expected ALTER TABLE inventory_anomalies")
	}
	if strings.Contains(strings.ToUpper(raw), "DROP TABLE") {
		t.Fatal("migration must not drop tables")
	}
}

package migrations_test

import (
	"os"
	"strings"
	"testing"
)

func TestBaselinePlatformMigration_is_irreversible_down(t *testing.T) {
	b, err := os.ReadFile("../../migrations/00002_platform_schema.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	raw := string(b)
	if !strings.Contains(raw, "-- +goose Down") {
		t.Fatal("expected goose Down section")
	}
	if !strings.Contains(raw, "baseline schema migration irreversible") {
		t.Fatal("expected irreversible Down marker")
	}
}

func TestBaselinePlatformMigration_defines_core_tables(t *testing.T) {
	b, err := os.ReadFile("../../migrations/00002_platform_schema.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	raw := strings.ToUpper(string(b))
	for _, needle := range []string{
		"CREATE TABLE REGIONS",
		"CREATE TABLE SITES",
		"CREATE TABLE MACHINES",
		"CREATE TABLE PRODUCTS",
	} {
		if !strings.Contains(raw, needle) {
			t.Fatalf("expected %s in baseline migration", needle)
		}
	}
}

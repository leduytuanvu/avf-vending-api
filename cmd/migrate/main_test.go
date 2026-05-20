package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateMigrations(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "00001_test.sql"), []byte("-- +goose Up\nSELECT 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := validateMigrations(dir); code != 0 {
		t.Fatalf("validateMigrations() = %d, want 0", code)
	}
}

func TestValidateMigrationsMissingDir(t *testing.T) {
	if code := validateMigrations(filepath.Join(t.TempDir(), "missing")); code == 0 {
		t.Fatal("expected failure for missing dir")
	}
}

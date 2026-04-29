package loadtest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestParseManifestTSV(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "m.tsv")
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	if err := os.WriteFile(p, []byte(id.String()+"\teyJ0b2tlbiJ9Cg==\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rows, err := ParseManifestTSV(p)
	if err != nil || len(rows) != 1 || rows[0].MachineID != id {
		t.Fatalf("got %#v err=%v", rows, err)
	}
}

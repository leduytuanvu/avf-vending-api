// Command telemetry-contract validates JSON files under testdata/telemetry.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	root, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "telemetry-contract: %v\n", err)
		os.Exit(1)
	}
	dir := filepath.Join(root, "testdata", "telemetry")
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "telemetry-contract: %v\n", err)
		os.Exit(1)
	}
	var failed bool
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: read: %v\n", path, err)
			failed = true
			continue
		}
		if !json.Valid(b) {
			fmt.Fprintf(os.Stderr, "%s: invalid JSON\n", path)
			failed = true
			continue
		}
		if err := checkCriticalIdentity(e.Name(), b); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			failed = true
			continue
		}
		fmt.Println("ok", path)
	}
	if failed {
		os.Exit(1)
	}
}

func findRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("go.mod not found from %s", wd)
}

// checkCriticalIdentity enforces that "valid_*" fixtures intended for critical paths carry ingest-stable identity.
func checkCriticalIdentity(base string, raw []byte) error {
	if strings.HasPrefix(base, "invalid_") || base == "duplicate_replay_vend.json" {
		return nil
	}
	if !strings.HasPrefix(base, "valid_") {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}

	switch base {
	case "valid_heartbeat_metrics.json":
		return nil
	case "valid_command_ack.json":
		dk, _ := m["dedupe_key"].(string)
		if strings.TrimSpace(dk) == "" {
			return fmt.Errorf("valid command ack must set non-empty dedupe_key (application idempotency for receipt)")
		}
		return nil
	default:
		// Envelope-style critical telemetry: dedupe_key and/or event_id and/or boot_id+seq_no.
		dk, _ := m["dedupe_key"].(string)
		eid, _ := m["event_id"].(string)
		boot, _ := m["boot_id"].(string)
		var seq float64
		if v, ok := m["seq_no"]; ok {
			seq, _ = v.(float64)
		}
		if strings.TrimSpace(dk) != "" || strings.TrimSpace(eid) != "" || (strings.TrimSpace(boot) != "" && seq != 0) {
			return nil
		}
		return fmt.Errorf("valid critical sample must set dedupe_key and/or event_id and/or boot_id+seq_no")
	}
}

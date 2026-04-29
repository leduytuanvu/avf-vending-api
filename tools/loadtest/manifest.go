package loadtest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
)

// MachineRow is one activated machine credential pair for fleet-shaped load tests.
type MachineRow struct {
	MachineID uuid.UUID
	JWT       string
}

// ParseManifestTSV reads UTF-8 lines: machine_uuid<TAB>jwt (# comments and blanks skipped).
func ParseManifestTSV(path string) ([]MachineRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var rows []MachineRow
	sc := bufio.NewScanner(f)
	lineNum := 0
	for sc.Scan() {
		lineNum++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("%s:%d: expected machine_uuid<TAB>jwt", path, lineNum)
		}
		mid, err := uuid.Parse(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("%s:%d: invalid machine uuid: %w", path, lineNum, err)
		}
		jwt := strings.TrimSpace(parts[1])
		if jwt == "" {
			return nil, fmt.Errorf("%s:%d: empty jwt", path, lineNum)
		}
		rows = append(rows, MachineRow{MachineID: mid, JWT: jwt})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%s: no machine rows", path)
	}
	return rows, nil
}

type manifestJSONRow struct {
	MachineID string `json:"machine_id"`
	JWT       string `json:"jwt"`
}

// ParseManifestJSON reads [{"machine_id":"...","jwt":"..."},...].
func ParseManifestJSON(path string) ([]MachineRow, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var js []manifestJSONRow
	if err := json.Unmarshal(raw, &js); err != nil {
		return nil, err
	}
	out := make([]MachineRow, 0, len(js))
	for i, row := range js {
		mid, err := uuid.Parse(strings.TrimSpace(row.MachineID))
		if err != nil {
			return nil, fmt.Errorf("row %d: machine_id: %w", i, err)
		}
		jwt := strings.TrimSpace(row.JWT)
		if jwt == "" {
			return nil, fmt.Errorf("row %d: empty jwt", i)
		}
		out = append(out, MachineRow{MachineID: mid, JWT: jwt})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty manifest")
	}
	return out, nil
}

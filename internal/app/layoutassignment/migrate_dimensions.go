package layoutassignment

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgtype"
)

var gridKeyRe = regexp.MustCompile(`^grid-(\d+)x(\d+)$`)

// ClassifyLegacyLayoutDimensions audits machine_slot_layouts and records PROVEN / INFERRED_SAFE / REQUIRES_REVIEW.
func ClassifyLegacyLayoutDimensions(ctx context.Context, q *db.Queries) error {
	rows, err := q.ListMachineSlotLayoutsForDimensionAudit(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		class, evidence := classifyLayoutRow(row)
		evBytes, _ := json.Marshal(evidence)
		if err := q.InsertLayoutDimensionAudit(ctx, db.InsertLayoutDimensionAuditParams{
			MachineSlotLayoutID: row.ID,
			Class:               class,
			Evidence:            evBytes,
		}); err != nil {
			return err
		}
		if class == "PROVEN" || class == "INFERRED_SAFE" {
			gr, gc := evidenceRowsCols(evidence)
			if gr > 0 && gc > 0 {
				if err := q.UpdateMachineSlotLayoutGridDimensions(ctx, db.UpdateMachineSlotLayoutGridDimensionsParams{
					ID:       row.ID,
					GridRows: pgtype.Int4{Int32: gr, Valid: true},
					GridCols: pgtype.Int4{Int32: gc, Valid: true},
				}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func classifyLayoutRow(row db.MachineSlotLayout) (class string, evidence map[string]any) {
	evidence = map[string]any{"layout_key": row.LayoutKey}
	var spec map[string]any
	_ = json.Unmarshal(row.LayoutSpec, &spec)
	if rows, cols, ok := provenFromSpec(spec); ok {
		evidence["rows"] = rows
		evidence["cols"] = cols
		evidence["source"] = "layout_spec"
		return "PROVEN", evidence
	}
	if rows, cols, ok := inferredFromLayoutKey(row.LayoutKey); ok {
		evidence["rows"] = rows
		evidence["cols"] = cols
		evidence["source"] = "layout_key"
		return "INFERRED_SAFE", evidence
	}
	evidence["layout_spec"] = spec
	return "REQUIRES_REVIEW", evidence
}

func provenFromSpec(spec map[string]any) (rows, cols int32, ok bool) {
	if spec == nil {
		return 0, 0, false
	}
	rf, rOK := spec["rows"].(float64)
	cf, cOK := spec["cols"].(float64)
	if !rOK || !cOK {
		return 0, 0, false
	}
	rows = int32(rf)
	cols = int32(cf)
	if rows < 1 || rows > MaxGridRows || cols < 1 || cols > MaxGridCols {
		return 0, 0, false
	}
	return rows, cols, true
}

func inferredFromLayoutKey(key string) (rows, cols int32, ok bool) {
	m := gridKeyRe.FindStringSubmatch(strings.TrimSpace(key))
	if len(m) != 3 {
		return 0, 0, false
	}
	var c, r int32
	if _, err := fmtSscanf(m[1], &c); err != nil {
		return 0, 0, false
	}
	if _, err := fmtSscanf(m[2], &r); err != nil {
		return 0, 0, false
	}
	if r < 1 || r > MaxGridRows || c < 1 || c > MaxGridCols {
		return 0, 0, false
	}
	return r, c, true
}

func fmtSscanf(s string, out *int32) (int, error) {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, err
	}
	*out = int32(v)
	return 1, nil
}

func evidenceRowsCols(evidence map[string]any) (rows, cols int32) {
	if r, ok := evidence["rows"].(int32); ok {
		rows = r
	}
	if c, ok := evidence["cols"].(int32); ok {
		cols = c
	}
	return rows, cols
}

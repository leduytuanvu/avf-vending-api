package layoutassignment

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/avf/avf-vending-api/internal/platform/pgxutil"
	"github.com/google/uuid"
)

// LayoutDimensionAuditSummary is a per-class count from layout_dimension_migration_audit.
type LayoutDimensionAuditSummary struct {
	Class string `json:"class"`
	Count int64  `json:"count"`
}

// LayoutDimensionAuditItem is one audited legacy layout row.
type LayoutDimensionAuditItem struct {
	MachineSlotLayoutID uuid.UUID      `json:"machineSlotLayoutId"`
	Class               string         `json:"class"`
	Evidence            map[string]any `json:"evidence"`
	AuditedAt           string         `json:"auditedAt"`
	LayoutKey           string         `json:"layoutKey,omitempty"`
	GridRows            *int32         `json:"gridRows,omitempty"`
	GridCols            *int32         `json:"gridCols,omitempty"`
}

// LayoutDimensionMigrationAuditReport is the read model for the admin audit endpoint.
type LayoutDimensionMigrationAuditReport struct {
	Summary           []LayoutDimensionAuditSummary `json:"summary"`
	RequiresReview    []LayoutDimensionAuditItem    `json:"requiresReview"`
	UnauditedCount    int64                         `json:"unauditedCount"`
	MissingDimensions int64                         `json:"missingDimensions"`
}

// GetLayoutDimensionMigrationAuditReport returns classification coverage for legacy layouts.
func (s *Service) GetLayoutDimensionMigrationAuditReport(ctx context.Context) (LayoutDimensionMigrationAuditReport, error) {
	if s.Pool == nil {
		return LayoutDimensionMigrationAuditReport{}, fmt.Errorf("database pool is not configured")
	}
	q := pgxutil.NewQueries(s.Pool)
	summaryRows, err := q.ListLayoutDimensionMigrationAuditSummary(ctx)
	if err != nil {
		return LayoutDimensionMigrationAuditReport{}, err
	}
	reviewRows, err := q.ListLayoutDimensionMigrationAuditRequiresReview(ctx)
	if err != nil {
		return LayoutDimensionMigrationAuditReport{}, err
	}
	unaudited, err := q.CountMachineSlotLayoutsMissingDimensionAudit(ctx)
	if err != nil {
		return LayoutDimensionMigrationAuditReport{}, err
	}
	missingDims, err := q.CountMachineSlotLayoutsMissingDimensions(ctx)
	if err != nil {
		return LayoutDimensionMigrationAuditReport{}, err
	}

	report := LayoutDimensionMigrationAuditReport{
		Summary:           make([]LayoutDimensionAuditSummary, 0, len(summaryRows)),
		RequiresReview:    make([]LayoutDimensionAuditItem, 0, len(reviewRows)),
		UnauditedCount:    unaudited,
		MissingDimensions: missingDims,
	}
	for _, row := range summaryRows {
		report.Summary = append(report.Summary, LayoutDimensionAuditSummary{
			Class: row.Class,
			Count: row.Count,
		})
	}
	for _, row := range reviewRows {
		item := LayoutDimensionAuditItem{
			MachineSlotLayoutID: row.MachineSlotLayoutID,
			Class:               row.Class,
			AuditedAt:           row.AuditedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			LayoutKey:           row.LayoutKey,
		}
		if row.GridRows.Valid {
			v := row.GridRows.Int32
			item.GridRows = &v
		}
		if row.GridCols.Valid {
			v := row.GridCols.Int32
			item.GridCols = &v
		}
		var evidence map[string]any
		if len(row.Evidence) > 0 {
			_ = json.Unmarshal(row.Evidence, &evidence)
		}
		if evidence == nil {
			evidence = map[string]any{}
		}
		item.Evidence = evidence
		report.RequiresReview = append(report.RequiresReview, item)
	}
	return report, nil
}

// RunLayoutDimensionMigrationClassify executes ClassifyLegacyLayoutDimensions against the pool.
func (s *Service) RunLayoutDimensionMigrationClassify(ctx context.Context) error {
	if s.Pool == nil {
		return fmt.Errorf("database pool is not configured")
	}
	return ClassifyLegacyLayoutDimensions(ctx, pgxutil.NewQueries(s.Pool))
}

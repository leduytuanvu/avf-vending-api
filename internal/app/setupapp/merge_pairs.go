package setupapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrMergePairOverlap      = errors.New("merge pair overlaps an active pair")
	ErrMergePairInvalidSlots = errors.New("invalid merge slot codes")
	ErrMergePairNotFound     = errors.New("active merge pair not found")
)

// LaneMergePair is one active double-wide TCN lane merge.
type LaneMergePair struct {
	LeftSlotCode    string
	RightSlotCode   string
	CabinetCode     string
	LayoutKey       string
	LayoutRevision  int32
	Revision        int32
	OperatorSession *uuid.UUID
	MergedAt        time.Time
}

// MergePairApplyItem is one merge or split operation in a batch request.
type MergePairApplyItem struct {
	LeftSlotCode   string
	RightSlotCode  string
	CabinetCode    string
	LayoutKey      string
	LayoutRevision int32
	Merge          bool
}

// MergePairBatchInput applies merge/split operations atomically for a machine.
type MergePairBatchInput struct {
	MachineID         uuid.UUID
	OperatorSessionID uuid.UUID
	Items             []MergePairApplyItem
}

// MergePairBatchResult is returned after a successful batch apply.
type MergePairBatchResult struct {
	Revision int32
	Pairs    []LaneMergePair
}

func mapMergePairRow(row db.MachineLaneMergePair) LaneMergePair {
	out := LaneMergePair{
		LeftSlotCode:   strings.TrimSpace(row.LeftSlotCode),
		RightSlotCode:  strings.TrimSpace(row.RightSlotCode),
		CabinetCode:    strings.TrimSpace(row.CabinetCode),
		LayoutKey:      strings.TrimSpace(row.LayoutKey),
		LayoutRevision: row.LayoutRevision,
		Revision:       row.Revision,
		MergedAt:        row.MergedAt,
	}
	if row.OperatorSessionID.Valid {
		sid := uuid.UUID(row.OperatorSessionID.Bytes)
		out.OperatorSession = &sid
	}
	return out
}

// ListActiveMergePairs returns active merge pairs for a machine.
func ListActiveMergePairs(ctx context.Context, pool *pgxpool.Pool, machineID uuid.UUID) ([]LaneMergePair, error) {
	if pool == nil || machineID == uuid.Nil {
		return nil, nil
	}
	rows, err := db.New(pool).PlanogramListActiveMergePairsForMachine(ctx, machineID)
	if err != nil {
		return nil, err
	}
	out := make([]LaneMergePair, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapMergePairRow(row))
	}
	return out, nil
}

// ApplyMergePairBatch upserts merge pairs and splits removed pairs in one transaction.
func ApplyMergePairBatch(ctx context.Context, pool *pgxpool.Pool, in MergePairBatchInput) (MergePairBatchResult, error) {
	var zero MergePairBatchResult
	if pool == nil || in.MachineID == uuid.Nil {
		return zero, fmt.Errorf("pool and machine_id required")
	}
	if in.OperatorSessionID == uuid.Nil {
		return zero, fmt.Errorf("operator_session_id required")
	}
	if len(in.Items) == 0 {
		pairs, err := ListActiveMergePairs(ctx, pool, in.MachineID)
		if err != nil {
			return zero, err
		}
		return MergePairBatchResult{Pairs: pairs}, nil
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return zero, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)

	nextRev, err := q.PlanogramNextMergePairRevision(ctx, in.MachineID)
	if err != nil {
		return zero, err
	}
	revision := nextRev

	active, err := q.PlanogramListActiveMergePairsForMachine(ctx, in.MachineID)
	if err != nil {
		return zero, err
	}
	activeByLeft := make(map[string]db.MachineLaneMergePair, len(active))
	for _, row := range active {
		activeByLeft[strings.TrimSpace(row.LeftSlotCode)] = row
	}

	desired := make(map[string]LaneMergePair)
	for _, item := range in.Items {
		left := strings.TrimSpace(item.LeftSlotCode)
		right := strings.TrimSpace(item.RightSlotCode)
		if left == "" || right == "" || left == right {
			return zero, ErrMergePairInvalidSlots
		}
		if !item.Merge {
			if _, ok := activeByLeft[left]; ok {
				if _, err := q.PlanogramSplitActiveMergePair(ctx, db.PlanogramSplitActiveMergePairParams{
					MachineID:    in.MachineID,
					LeftSlotCode: left,
				}); err != nil {
					return zero, err
				}
				delete(activeByLeft, left)
			}
			continue
		}
		layoutKey := strings.TrimSpace(item.LayoutKey)
		if layoutKey == "" {
			layoutKey = "default"
		}
		layoutRevision := item.LayoutRevision
		if layoutRevision < 1 {
			layoutRevision = 1
		}
		cabinetCode := strings.TrimSpace(item.CabinetCode)
		if err := ensureNoMergeOverlap(activeByLeft, left, right, left); err != nil {
			return zero, err
		}
		if existing, ok := activeByLeft[left]; ok {
			if _, err := q.PlanogramSplitActiveMergePair(ctx, db.PlanogramSplitActiveMergePairParams{
				MachineID:    in.MachineID,
				LeftSlotCode: left,
			}); err != nil {
				return zero, err
			}
			_ = existing
			delete(activeByLeft, left)
		}
		row, err := q.PlanogramInsertActiveMergePair(ctx, db.PlanogramInsertActiveMergePairParams{
			MachineID:         in.MachineID,
			LeftSlotCode:      left,
			RightSlotCode:     right,
			CabinetCode:       cabinetCode,
			LayoutKey:         layoutKey,
			LayoutRevision:    layoutRevision,
			Revision:          revision,
			OperatorSessionID: pgtype.UUID{Bytes: in.OperatorSessionID, Valid: true},
		})
		if err != nil {
			return zero, err
		}
		pair := mapMergePairRow(row)
		desired[left] = pair
		activeByLeft[left] = row
		if err := mirrorMergeMetadata(ctx, q, in.MachineID, pair); err != nil {
			return zero, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return zero, err
	}
	pairs, err := ListActiveMergePairs(ctx, pool, in.MachineID)
	if err != nil {
		return zero, err
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].LeftSlotCode < pairs[j].LeftSlotCode })
	return MergePairBatchResult{Revision: revision, Pairs: pairs}, nil
}

func ensureNoMergeOverlap(active map[string]db.MachineLaneMergePair, left, right, skipLeft string) error {
	for _, row := range active {
		l := strings.TrimSpace(row.LeftSlotCode)
		r := strings.TrimSpace(row.RightSlotCode)
		if l == skipLeft {
			continue
		}
		if l == left || l == right || r == left || r == right {
			return ErrMergePairOverlap
		}
	}
	return nil
}

func mirrorMergeMetadata(ctx context.Context, q *db.Queries, machineID uuid.UUID, pair LaneMergePair) error {
	leftMeta, _ := json.Marshal(map[string]any{
		"laneSpan":  2,
		"mergeRole": "left",
		"mergeWith": pair.RightSlotCode,
	})
	rightMeta, _ := json.Marshal(map[string]any{
		"laneSpan":  2,
		"mergeRole": "hidden_companion",
		"mergeWith": pair.LeftSlotCode,
	})
	if err := q.PlanogramMirrorSlotConfigMetadata(ctx, db.PlanogramMirrorSlotConfigMetadataParams{
		MachineID: machineID,
		SlotCode:  pair.LeftSlotCode,
		Metadata:  leftMeta,
	}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err := q.PlanogramMirrorSlotConfigMetadata(ctx, db.PlanogramMirrorSlotConfigMetadataParams{
		MachineID: machineID,
		SlotCode:  pair.RightSlotCode,
		Metadata:  rightMeta,
	}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	return nil
}

package layoutassignment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/pgjson"
	"github.com/avf/avf-vending-api/internal/platform/pgxutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	SnapshotReasonPeriodic30M  = "PERIODIC_30M"
	SnapshotReasonManualSync   = "MANUAL_SYNC"
	SnapshotReasonReconnect    = "RECONNECT"
	SnapshotReasonActivation   = "ACTIVATION"
	SnapshotReasonLegacyReport = "LEGACY_REPORT"

	DefaultSnapshotPayloadVersion = 1
)

var (
	ErrLayoutNotFound            = errors.New("layout not found")
	ErrSnapshotSchemaUnsupported = errors.New("snapshot schema unsupported")
	ErrStaleLayoutRevision       = errors.New("stale layout revision")
)

// ReportLayoutSnapshotInput is the canonical device snapshot ingest command.
type ReportLayoutSnapshotInput struct {
	MachineID          uuid.UUID
	SnapshotID         uuid.UUID
	LayoutID           uuid.UUID
	LayoutName         string
	DeviceInstanceID   string
	CaptureSequence    int64
	DeviceGeneration   int64
	CapturedAt         time.Time
	IntervalKey        string
	BaseServerRevision *int32
	Fingerprint        string
	PayloadVersion     int32
	SnapshotReason     string
	GridRows           int32
	GridCols           int32
	ActiveLayoutID     uuid.UUID
	MergePairs         []SnapshotMergePairInput
	Slots              []SnapshotSlotInput
	SlotsJSON          []byte
}

type SnapshotMergePairInput struct {
	LeftSlotCode  string
	RightSlotCode string
}

type SnapshotSlotInput struct {
	SlotCode             string
	SlotOrdinal          int32
	LogicalCoordinate    string
	PhysicalLane         int32
	ProductID            string
	MaxQuantity          int32
	PriceMinor           int64
	LocalPricingRevision int64
	CurrentInventory     int32
	Enabled              bool
	OperationalState     string
}

// ReportLayoutSnapshotResult is returned after ingest.
type ReportLayoutSnapshotResult struct {
	Accepted              bool
	Duplicate             bool
	StoredSnapshotID      uuid.UUID
	StoredCaptureSequence int64
	StoredFingerprint     string
	StoredRevision        int32
	StoredGeneration      int64
}

// ReportLayoutSnapshot ingests one immutable history row and reconciles current device state.
func (s *Service) ReportLayoutSnapshot(ctx context.Context, auth MachineAuthContext, in ReportLayoutSnapshotInput) (ReportLayoutSnapshotResult, error) {
	if s.Pool == nil {
		return ReportLayoutSnapshotResult{}, fmt.Errorf("database pool is not configured")
	}
	if auth.MachineID == uuid.Nil {
		return ReportLayoutSnapshotResult{}, fmt.Errorf("machine auth context is required")
	}
	if in.CapturedAt.IsZero() {
		in.CapturedAt = time.Now().UTC()
	}
	if len(in.SlotsJSON) == 0 && len(in.Slots) > 0 {
		slotsJSON, err := marshalSnapshotSlotsJSON(in.Slots)
		if err != nil {
			return ReportLayoutSnapshotResult{}, err
		}
		in.SlotsJSON = slotsJSON
	}
	if err := validateReportLayoutSnapshotInput(auth, in); err != nil {
		return ReportLayoutSnapshotResult{}, err
	}

	readQ := pgxutil.NewQueries(s.Pool)
	if dup, err := readQ.GetMachineLayoutSnapshotHistoryByDeviceSequence(ctx, db.GetMachineLayoutSnapshotHistoryByDeviceSequenceParams{
		MachineID:        in.MachineID,
		DeviceInstanceID: strings.TrimSpace(in.DeviceInstanceID),
		CaptureSequence:  in.CaptureSequence,
	}); err == nil {
		return ReportLayoutSnapshotResult{
			Accepted:              true,
			Duplicate:             true,
			StoredSnapshotID:      dup.SnapshotID,
			StoredCaptureSequence: dup.CaptureSequence,
			StoredFingerprint:     strings.TrimSpace(dup.Fingerprint),
		}, nil
	} else if err != pgx.ErrNoRows {
		return ReportLayoutSnapshotResult{}, err
	}

	layoutRow, layoutErr := readQ.GetMachineLayoutByID(ctx, db.GetMachineLayoutByIDParams{
		ID:        in.LayoutID,
		MachineID: in.MachineID,
	})
	if layoutErr != nil {
		if layoutErr == pgx.ErrNoRows {
			return ReportLayoutSnapshotResult{}, ErrLayoutNotFound
		}
		return ReportLayoutSnapshotResult{}, layoutErr
	}
	_ = layoutRow

	payloadJSON, err := buildSnapshotPayloadJSON(in)
	if err != nil {
		return ReportLayoutSnapshotResult{}, err
	}

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ReportLayoutSnapshotResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := pgxutil.NewQueries(tx)
	now := time.Now().UTC()
	var baseRev pgtype.Int4
	if in.BaseServerRevision != nil {
		baseRev = pgtype.Int4{Int32: *in.BaseServerRevision, Valid: true}
	}
	histRow, err := q.InsertMachineLayoutSnapshotHistory(ctx, db.InsertMachineLayoutSnapshotHistoryParams{
		SnapshotID:         in.SnapshotID,
		MachineID:          in.MachineID,
		LayoutID:           in.LayoutID,
		DeviceInstanceID:   strings.TrimSpace(in.DeviceInstanceID),
		CaptureSequence:    in.CaptureSequence,
		IntervalKey:        pgtype.Text{String: strings.TrimSpace(in.IntervalKey), Valid: strings.TrimSpace(in.IntervalKey) != ""},
		CapturedAt:         in.CapturedAt.UTC(),
		DeviceGeneration:   in.DeviceGeneration,
		BaseServerRevision: baseRev,
		Fingerprint:        strings.TrimSpace(in.Fingerprint),
		SnapshotReason:     normalizeSnapshotReason(in.SnapshotReason),
		PayloadVersion:     in.PayloadVersion,
		Payload:            payloadJSON,
	})
	if err != nil {
		return ReportLayoutSnapshotResult{}, err
	}

	deviceState, err := q.UpsertMachineLayoutDeviceState(ctx, db.UpsertMachineLayoutDeviceStateParams{
		MachineID:             in.MachineID,
		LayoutID:              in.LayoutID,
		LatestSnapshotID:      pgtype.UUID{Bytes: in.SnapshotID, Valid: true},
		LatestCaptureSequence: in.CaptureSequence,
		DeviceGeneration:      in.DeviceGeneration,
		Fingerprint:           strings.TrimSpace(in.Fingerprint),
		ReportedAt:            now,
		DeviceInstanceID:      strings.TrimSpace(in.DeviceInstanceID),
	})
	if err != nil {
		return ReportLayoutSnapshotResult{}, err
	}

	activeLayoutID := in.ActiveLayoutID
	if activeLayoutID == uuid.Nil {
		activeLayoutID = in.LayoutID
	}
	if deviceState.LatestCaptureSequence == in.CaptureSequence {
		if err := q.SetMachineActiveLayoutPointers(ctx, db.SetMachineActiveLayoutPointersParams{
			ID:                     in.MachineID,
			ReportedActiveLayoutID: pgtype.UUID{Bytes: activeLayoutID, Valid: true},
		}); err != nil {
			return ReportLayoutSnapshotResult{}, err
		}
	}

	if err := s.upsertLegacyMirrorFromSnapshot(ctx, q, in, payloadJSON, now); err != nil {
		return ReportLayoutSnapshotResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return ReportLayoutSnapshotResult{}, err
	}

	storedRev := int32(in.CaptureSequence)
	if in.BaseServerRevision != nil {
		storedRev = *in.BaseServerRevision
	}
	return ReportLayoutSnapshotResult{
		Accepted:              true,
		StoredSnapshotID:      histRow.SnapshotID,
		StoredCaptureSequence: histRow.CaptureSequence,
		StoredFingerprint:     strings.TrimSpace(histRow.Fingerprint),
		StoredRevision:        storedRev,
		StoredGeneration:      in.DeviceGeneration,
	}, nil
}

func (s *Service) upsertLegacyMirrorFromSnapshot(ctx context.Context, q *db.Queries, in ReportLayoutSnapshotInput, payloadJSON []byte, now time.Time) error {
	slotsJSON := in.SlotsJSON
	if len(slotsJSON) == 0 {
		slotsJSON = payloadJSON
	}
	revision := int32(in.CaptureSequence)
	if in.BaseServerRevision != nil {
		revision = *in.BaseServerRevision
	}
	return q.UpsertMachineLocalLayoutMirror(ctx, db.UpsertMachineLocalLayoutMirrorParams{
		MachineID:        in.MachineID,
		LocalLayoutID:    in.LayoutID,
		Revision:         revision,
		GridRows:         in.GridRows,
		GridCols:         in.GridCols,
		Slots:            pgjson.RequiredString(slotsJSON),
		Fingerprint:      strings.TrimSpace(in.Fingerprint),
		ReportedAt:       now,
		DeviceInstanceID: strings.TrimSpace(in.DeviceInstanceID),
	})
}

func validateReportLayoutSnapshotInput(auth MachineAuthContext, in ReportLayoutSnapshotInput) error {
	if auth.MachineID == uuid.Nil {
		return fmt.Errorf("machine auth context is required")
	}
	if in.MachineID == uuid.Nil || in.MachineID != auth.MachineID {
		return fmt.Errorf("machine auth context does not match report")
	}
	if in.SnapshotID == uuid.Nil {
		return fmt.Errorf("snapshotId is required")
	}
	if in.LayoutID == uuid.Nil {
		return fmt.Errorf("layoutId is required")
	}
	if in.CaptureSequence < 1 {
		return fmt.Errorf("captureSequence must be >= 1")
	}
	if strings.TrimSpace(in.Fingerprint) == "" {
		return fmt.Errorf("fingerprint is required")
	}
	if strings.TrimSpace(in.DeviceInstanceID) == "" {
		return fmt.Errorf("deviceInstanceId is required")
	}
	if in.PayloadVersion < 1 {
		return fmt.Errorf("%w: payloadVersion=%d", ErrSnapshotSchemaUnsupported, in.PayloadVersion)
	}
	if in.CapturedAt.IsZero() {
		return fmt.Errorf("capturedAt is required")
	}
	if err := ValidateGridDimensions(in.GridRows, in.GridCols); err != nil {
		return err
	}
	if len(in.SlotsJSON) == 0 && len(in.Slots) == 0 {
		return fmt.Errorf("slots payload is required")
	}
	if len(in.SlotsJSON) > 0 {
		if !json.Valid(in.SlotsJSON) {
			return fmt.Errorf("invalid slots payload")
		}
		return validateReportedSlotsUnique(in.SlotsJSON)
	}
	return fmt.Errorf("slots payload is required")
}

func normalizeSnapshotReason(reason string) string {
	r := strings.TrimSpace(strings.ToUpper(reason))
	switch r {
	case SnapshotReasonPeriodic30M, SnapshotReasonManualSync, SnapshotReasonReconnect, SnapshotReasonActivation, SnapshotReasonLegacyReport:
		return r
	default:
		return SnapshotReasonLegacyReport
	}
}

func buildSnapshotPayloadJSON(in ReportLayoutSnapshotInput) ([]byte, error) {
	if len(in.SlotsJSON) > 0 && json.Valid(in.SlotsJSON) {
		body := map[string]any{
			"slots":          json.RawMessage(in.SlotsJSON),
			"mergePairs":     in.MergePairs,
			"layoutName":     strings.TrimSpace(in.LayoutName),
			"gridRows":       in.GridRows,
			"gridCols":       in.GridCols,
			"activeLayoutId": in.ActiveLayoutID.String(),
		}
		return json.Marshal(body)
	}
	slotsJSON, err := marshalSnapshotSlotsJSON(in.Slots)
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"slots":          json.RawMessage(slotsJSON),
		"mergePairs":     in.MergePairs,
		"layoutName":     strings.TrimSpace(in.LayoutName),
		"gridRows":       in.GridRows,
		"gridCols":       in.GridCols,
		"activeLayoutId": in.ActiveLayoutID.String(),
	}
	return json.Marshal(body)
}

func marshalSnapshotSlotsJSON(slots []SnapshotSlotInput) ([]byte, error) {
	if len(slots) == 0 {
		return nil, fmt.Errorf("slots required")
	}
	type slotJSON struct {
		SlotCode             string `json:"slotCode"`
		SlotOrdinal          int32  `json:"slotOrdinal,omitempty"`
		LogicalCoordinate    string `json:"logicalCoordinate,omitempty"`
		PhysicalLane         int32  `json:"physicalLane,omitempty"`
		ProductID            string `json:"productId,omitempty"`
		MaxQuantity          int32  `json:"maxQuantity,omitempty"`
		PriceMinor           int64  `json:"priceMinor,omitempty"`
		LocalPricingRevision int64  `json:"localPricingRevision,omitempty"`
		CurrentInventory     int32  `json:"currentInventory,omitempty"`
		Enabled              *bool  `json:"enabled,omitempty"`
		OperationalState     string `json:"operationalState,omitempty"`
	}
	out := make([]slotJSON, 0, len(slots))
	for _, sl := range slots {
		code := strings.TrimSpace(sl.SlotCode)
		if code == "" {
			return nil, fmt.Errorf("slotCode is required for each slot")
		}
		enabled := sl.Enabled
		out = append(out, slotJSON{
			SlotCode:             code,
			SlotOrdinal:          sl.SlotOrdinal,
			LogicalCoordinate:    strings.TrimSpace(sl.LogicalCoordinate),
			PhysicalLane:         sl.PhysicalLane,
			ProductID:            strings.TrimSpace(sl.ProductID),
			MaxQuantity:          sl.MaxQuantity,
			PriceMinor:           sl.PriceMinor,
			LocalPricingRevision: sl.LocalPricingRevision,
			CurrentInventory:     sl.CurrentInventory,
			Enabled:              &enabled,
			OperationalState:     strings.TrimSpace(sl.OperationalState),
		})
	}
	return json.Marshal(out)
}

// ReportLayoutSnapshotFromLocalLayout adapts legacy ReportLocalLayout into snapshot ingest.
func ReportLayoutSnapshotFromLocalLayout(in ReportLocalLayoutInput) ReportLayoutSnapshotInput {
	capturedAt := time.Now().UTC()
	return ReportLayoutSnapshotInput{
		MachineID:          in.MachineID,
		SnapshotID:         in.LocalLayoutID,
		LayoutID:           in.LocalLayoutID,
		DeviceInstanceID:   in.DeviceInstanceID,
		CaptureSequence:    int64(in.Revision),
		DeviceGeneration:   in.LocalGeneration,
		CapturedAt:         capturedAt,
		BaseServerRevision: &in.Revision,
		Fingerprint:        in.Fingerprint,
		PayloadVersion:     DefaultSnapshotPayloadVersion,
		SnapshotReason:     SnapshotReasonLegacyReport,
		GridRows:           in.Rows,
		GridCols:           in.Columns,
		ActiveLayoutID:     in.LocalLayoutID,
		SlotsJSON:          in.SlotsJSON,
	}
}

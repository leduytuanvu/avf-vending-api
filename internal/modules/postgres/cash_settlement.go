package postgres

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	cashdomain "github.com/avf/avf-vending-api/internal/domain/cash"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// MachineCashboxSummary is a read model for GET .../cashbox (no mutation; no hardware side effects).
type MachineCashboxSummary struct {
	ExpectedAmountMinor          int64
	ExpectedCashboxMinor         int64 // vault / loose cash (commerce-derived today)
	ExpectedRecyclerMinor        int64 // bill recycler expectation (0 until recycler telemetry is wired)
	Currency                     string
	LastClosedAt                 *time.Time
	OpenCollectionID             *uuid.UUID
	VarianceReviewThresholdMinor int64
}

// StartMachineCashCollectionInput begins an open collection session on the machine.
type StartMachineCashCollectionInput struct {
	OrganizationID      uuid.UUID
	MachineID           uuid.UUID
	OperatorSessionID   *uuid.UUID
	Currency            string
	Notes               string
	StartIdempotencyKey string
	CorrelationID       *uuid.UUID
	OpenedAt            time.Time
}

// CloseMachineCashCollectionInput closes an open session with a physical count (idempotent on payload hash).
type CloseMachineCashCollectionInput struct {
	OrganizationID          uuid.UUID
	MachineID               uuid.UUID
	CollectionID            uuid.UUID
	OperatorSessionID       *uuid.UUID
	CountedAmountMinor      int64 // total counted (cashbox + recycler) for variance
	CountedCashboxMinor     int64
	CountedRecyclerMinor    int64
	Currency                string
	Notes                   string
	EvidenceArtifactURL     string
	EvidencePhotoArtifactID string
	// Denominations is persisted in metadata and included in the extended close hash when non-empty.
	Denominations []CloseDenominationCount
	// Client-declared close instant for idempotency hashing only (server records closed_at = now()).
	ClosedAtRFC3339         string
	CorrelationID           *uuid.UUID
	VarianceReviewThreshold int64
	// UsesExtendedCloseHash selects P1 hash when any extended field is used (non-zero recycler, denoms, photo id, closedAt).
	UsesExtendedCloseHash bool
}

// CloseDenominationCount is a counted denomination row at close (audit / hash input).
type CloseDenominationCount struct {
	DenominationMinor int64 `json:"denominationMinor"`
	Count             int64 `json:"count"`
}

func legacyCloseHash(counted int64, currency, notes, evidenceURL string) []byte {
	type canon struct {
		CountedAmountMinor  int64  `json:"counted_amount_minor"`
		Currency            string `json:"currency"`
		EvidenceArtifactURL string `json:"evidence_artifact_url"`
		Notes               string `json:"notes"`
	}
	raw, err := json.Marshal(canon{
		CountedAmountMinor:  counted,
		Currency:            strings.TrimSpace(currency),
		EvidenceArtifactURL: strings.TrimSpace(evidenceURL),
		Notes:               strings.TrimSpace(notes),
	})
	if err != nil {
		raw = []byte{}
	}
	sum := sha256.Sum256(raw)
	return sum[:]
}

func extendedCloseHash(in CloseMachineCashCollectionInput) []byte {
	type denom struct {
		DenominationMinor int64 `json:"denominationMinor"`
		Count             int64 `json:"count"`
	}
	d := make([]denom, 0, len(in.Denominations))
	for _, x := range in.Denominations {
		d = append(d, denom{DenominationMinor: x.DenominationMinor, Count: x.Count})
	}
	type canon struct {
		CountedAmountMinor      int64   `json:"counted_amount_minor"`
		CountedCashboxMinor     int64   `json:"counted_cashbox_minor"`
		CountedRecyclerMinor    int64   `json:"counted_recycler_minor"`
		Currency                string  `json:"currency"`
		EvidenceArtifactURL     string  `json:"evidence_artifact_url"`
		EvidencePhotoArtifactID string  `json:"evidence_photo_artifact_id"`
		Notes                   string  `json:"notes"`
		Denominations           []denom `json:"denominations,omitempty"`
		ClosedAt                string  `json:"closed_at,omitempty"`
	}
	raw, err := json.Marshal(canon{
		CountedAmountMinor:      in.CountedAmountMinor,
		CountedCashboxMinor:     in.CountedCashboxMinor,
		CountedRecyclerMinor:    in.CountedRecyclerMinor,
		Currency:                strings.TrimSpace(in.Currency),
		EvidenceArtifactURL:     strings.TrimSpace(in.EvidenceArtifactURL),
		EvidencePhotoArtifactID: strings.TrimSpace(in.EvidencePhotoArtifactID),
		Notes:                   strings.TrimSpace(in.Notes),
		Denominations:           d,
		ClosedAt:                strings.TrimSpace(in.ClosedAtRFC3339),
	})
	if err != nil {
		raw = []byte{}
	}
	sum := sha256.Sum256(raw)
	return sum[:]
}

func computeCloseRequestHash(in CloseMachineCashCollectionInput) []byte {
	if in.UsesExtendedCloseHash {
		return extendedCloseHash(in)
	}
	return legacyCloseHash(in.CountedAmountMinor, in.Currency, in.Notes, in.EvidenceArtifactURL)
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// GetMachineCashboxSummary returns expected vault cash from commerce since the last closed collection.
func (s *Store) GetMachineCashboxSummary(ctx context.Context, organizationID, machineID uuid.UUID, currency string, varianceReviewThresholdMinor int64) (MachineCashboxSummary, error) {
	if s == nil || s.pool == nil {
		return MachineCashboxSummary{}, errors.New("postgres: nil store")
	}
	cur := strings.TrimSpace(currency)
	if cur == "" {
		cur = "USD"
	}
	if varianceReviewThresholdMinor <= 0 {
		varianceReviewThresholdMinor = 500
	}
	q := db.New(s.pool)
	m, err := q.GetMachineByID(ctx, machineID)
	if err != nil {
		if isNoRows(err) {
			return MachineCashboxSummary{}, fmt.Errorf("cash settlement summary: %w", pgx.ErrNoRows)
		}
		return MachineCashboxSummary{}, err
	}
	if m.OrganizationID != organizationID {
		return MachineCashboxSummary{}, ErrMachineOrganizationMismatch
	}
	exp, err := q.CashSettlementNetExpectedMinor(ctx, db.CashSettlementNetExpectedMinorParams{
		MachineID:      machineID,
		OrganizationID: organizationID,
		Currency:       cur,
	})
	if err != nil {
		return MachineCashboxSummary{}, err
	}
	last, err := q.CashSettlementLastClosedAt(ctx, db.CashSettlementLastClosedAtParams{
		MachineID:      machineID,
		OrganizationID: organizationID,
	})
	if err != nil {
		return MachineCashboxSummary{}, err
	}
	var lastPtr *time.Time
	if !last.IsZero() && last.Unix() > 0 {
		t := last.UTC()
		lastPtr = &t
	}
	var openID *uuid.UUID
	openRow, err := q.GetOpenCashCollectionByMachine(ctx, db.GetOpenCashCollectionByMachineParams{
		MachineID:      machineID,
		OrganizationID: organizationID,
	})
	if err == nil {
		id := openRow.ID
		openID = &id
	} else if !isNoRows(err) {
		return MachineCashboxSummary{}, err
	}
	return MachineCashboxSummary{
		ExpectedAmountMinor:          exp,
		ExpectedCashboxMinor:         exp,
		ExpectedRecyclerMinor:        0,
		Currency:                     cur,
		LastClosedAt:                 lastPtr,
		OpenCollectionID:             openID,
		VarianceReviewThresholdMinor: varianceReviewThresholdMinor,
	}, nil
}

// StartMachineCashCollection inserts an open cash_collections row (operator session required).
func (s *Store) StartMachineCashCollection(ctx context.Context, in StartMachineCashCollectionInput) (db.CashCollection, error) {
	if s == nil || s.pool == nil {
		return db.CashCollection{}, errors.New("postgres: nil store")
	}
	cur := strings.TrimSpace(in.Currency)
	if cur == "" {
		return db.CashCollection{}, fmt.Errorf("postgres: currency required")
	}
	idem := strings.TrimSpace(in.StartIdempotencyKey)
	if idem == "" {
		return db.CashCollection{}, fmt.Errorf("postgres: start idempotency key required")
	}
	opened := in.OpenedAt
	if opened.IsZero() {
		opened = time.Now().UTC()
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return db.CashCollection{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)

	machineRow, err := q.GetMachineByIDForUpdate(ctx, in.MachineID)
	if err != nil {
		if isNoRows(err) {
			return db.CashCollection{}, fmt.Errorf("cash collection start: %w", pgx.ErrNoRows)
		}
		return db.CashCollection{}, err
	}
	if machineRow.OrganizationID != in.OrganizationID {
		return db.CashCollection{}, ErrMachineOrganizationMismatch
	}

	replay, err := q.FindCashCollectionOpenByStartIdempotencyKey(ctx, db.FindCashCollectionOpenByStartIdempotencyKeyParams{
		Column1:        idem,
		MachineID:      in.MachineID,
		OrganizationID: in.OrganizationID,
	})
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return db.CashCollection{}, err
		}
		return replay, nil
	}
	if !isNoRows(err) {
		return db.CashCollection{}, err
	}

	_, err = q.GetOpenCashCollectionByMachine(ctx, db.GetOpenCashCollectionByMachineParams{
		MachineID:      in.MachineID,
		OrganizationID: in.OrganizationID,
	})
	if err == nil {
		return db.CashCollection{}, cashdomain.ErrOpenCollectionExists
	}
	if !isNoRows(err) {
		return db.CashCollection{}, err
	}

	meta := map[string]any{}
	if strings.TrimSpace(in.Notes) != "" {
		meta["notes"] = strings.TrimSpace(in.Notes)
	}
	meta["start_idempotency_key"] = idem
	meta["disclosure"] = "Accounting-only API: does not command bill recycler or other cash hardware."
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return db.CashCollection{}, err
	}

	row, err := q.InsertCashCollection(ctx, db.InsertCashCollectionParams{
		OrganizationID:      in.OrganizationID,
		MachineID:           in.MachineID,
		CollectedAt:         opened,
		OpenedAt:            opened,
		ClosedAt:            pgtype.Timestamptz{Valid: false},
		LifecycleStatus:     "open",
		AmountMinor:         0,
		ExpectedAmountMinor: 0,
		VarianceAmountMinor: 0,
		RequiresReview:      false,
		CloseRequestHash:    nil,
		Currency:            cur,
		Metadata:            metaBytes,
		OperatorSessionID:   optionalUUIDToPg(in.OperatorSessionID),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return db.CashCollection{}, cashdomain.ErrOpenCollectionExists
		}
		return db.CashCollection{}, err
	}

	if err := insertOperatorSessionAttribution(ctx, q, operatorAttributionSpec{
		MachineID:         in.MachineID,
		OrganizationID:    in.OrganizationID,
		OperatorSessionID: in.OperatorSessionID,
		ActionDomain:      "cash",
		ActionType:        "cash.collection.open",
		ResourceTable:     "cash_collections",
		ResourceID:        row.ID.String(),
		CorrelationID:     in.CorrelationID,
		OccurredAt:        &opened,
	}); err != nil {
		return db.CashCollection{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return db.CashCollection{}, err
	}
	return row, nil
}

// CloseMachineCashCollection closes an open collection with counted cash vs commerce-expected vault.
func (s *Store) CloseMachineCashCollection(ctx context.Context, in CloseMachineCashCollectionInput) (db.CashCollection, error) {
	if s == nil || s.pool == nil {
		return db.CashCollection{}, errors.New("postgres: nil store")
	}
	thr := in.VarianceReviewThreshold
	if thr <= 0 {
		thr = 500
	}
	cur := strings.TrimSpace(in.Currency)
	if cur == "" {
		return db.CashCollection{}, fmt.Errorf("postgres: currency required")
	}
	if in.CountedAmountMinor < 0 || in.CountedCashboxMinor < 0 || in.CountedRecyclerMinor < 0 {
		return db.CashCollection{}, cashdomain.ErrInvalidCountedAmount
	}
	hash := computeCloseRequestHash(in)

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return db.CashCollection{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)

	existing, err := q.GetCashCollectionByIDForOrgMachine(ctx, db.GetCashCollectionByIDForOrgMachineParams{
		ID:             in.CollectionID,
		OrganizationID: in.OrganizationID,
		MachineID:      in.MachineID,
	})
	if err != nil {
		if isNoRows(err) {
			return db.CashCollection{}, cashdomain.ErrCollectionNotFound
		}
		return db.CashCollection{}, err
	}

	if existing.LifecycleStatus == "closed" {
		if len(existing.CloseRequestHash) == 0 {
			return db.CashCollection{}, cashdomain.ErrClosePayloadConflict
		}
		if bytes.Equal(hash, existing.CloseRequestHash) {
			if err := tx.Commit(ctx); err != nil {
				return db.CashCollection{}, err
			}
			return existing, nil
		}
		if !in.UsesExtendedCloseHash {
			legacy := legacyCloseHash(in.CountedAmountMinor, cur, in.Notes, in.EvidenceArtifactURL)
			if bytes.Equal(legacy, existing.CloseRequestHash) {
				if err := tx.Commit(ctx); err != nil {
					return db.CashCollection{}, err
				}
				return existing, nil
			}
		}
		return db.CashCollection{}, cashdomain.ErrClosePayloadConflict
	}

	if existing.LifecycleStatus != "open" {
		return db.CashCollection{}, cashdomain.ErrCollectionNotFound
	}

	if !strings.EqualFold(strings.TrimSpace(existing.Currency), cur) {
		return db.CashCollection{}, cashdomain.ErrCurrencyMismatch
	}

	expected, err := q.CashSettlementNetExpectedMinor(ctx, db.CashSettlementNetExpectedMinorParams{
		MachineID:      in.MachineID,
		OrganizationID: in.OrganizationID,
		Currency:       strings.TrimSpace(existing.Currency),
	})
	if err != nil {
		return db.CashCollection{}, err
	}

	variance := in.CountedAmountMinor - expected
	needsReview := abs64(variance) > thr

	var collRecon string
	switch {
	case needsReview:
		collRecon = "pending"
	case variance == 0:
		collRecon = "matched"
	default:
		collRecon = "mismatch"
	}

	var reconLedger string
	switch {
	case needsReview:
		reconLedger = "review"
	case variance == 0:
		reconLedger = "matched"
	default:
		reconLedger = "mismatch"
	}

	meta := map[string]any{}
	if len(existing.Metadata) > 0 {
		_ = json.Unmarshal(existing.Metadata, &meta)
	}
	if strings.TrimSpace(in.Notes) != "" {
		meta["close_notes"] = strings.TrimSpace(in.Notes)
	}
	if strings.TrimSpace(in.EvidenceArtifactURL) != "" {
		meta["evidence_artifact_url"] = strings.TrimSpace(in.EvidenceArtifactURL)
	}
	if strings.TrimSpace(in.EvidencePhotoArtifactID) != "" {
		meta["evidence_photo_artifact_id"] = strings.TrimSpace(in.EvidencePhotoArtifactID)
	}
	if in.CountedCashboxMinor != 0 || in.CountedRecyclerMinor != 0 {
		meta["counted_cashbox_minor"] = in.CountedCashboxMinor
		meta["counted_recycler_minor"] = in.CountedRecyclerMinor
	}
	if len(in.Denominations) > 0 {
		meta["denominations"] = in.Denominations
	}
	if strings.TrimSpace(in.ClosedAtRFC3339) != "" {
		meta["client_closed_at"] = strings.TrimSpace(in.ClosedAtRFC3339)
	}
	meta["disclosure"] = "Accounting-only API: does not command bill recycler or other cash hardware."
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return db.CashCollection{}, err
	}

	closed, err := q.CloseCashCollection(ctx, db.CloseCashCollectionParams{
		ID:                   in.CollectionID,
		OrganizationID:       in.OrganizationID,
		MachineID:            in.MachineID,
		AmountMinor:          in.CountedAmountMinor,
		ExpectedAmountMinor:  expected,
		VarianceAmountMinor:  variance,
		RequiresReview:       needsReview,
		CloseRequestHash:     hash,
		ReconciliationStatus: collRecon,
		Metadata:             metaBytes,
	})
	if err != nil {
		if isNoRows(err) {
			return db.CashCollection{}, cashdomain.ErrCollectionNotFound
		}
		return db.CashCollection{}, err
	}

	reconMeta, _ := json.Marshal(map[string]any{
		"cash_collection_id": closed.ID.String(),
		"operator_close":     in.OperatorSessionID != nil,
	})
	_, err = q.InsertCashReconciliation(ctx, db.InsertCashReconciliationParams{
		MachineID:           in.MachineID,
		CashCollectionID:    uuidToPg(closed.ID),
		ExpectedAmountMinor: expected,
		CountedAmountMinor:  in.CountedAmountMinor,
		VarianceAmountMinor: variance,
		Status:              reconLedger,
		Metadata:            reconMeta,
	})
	if err != nil {
		return db.CashCollection{}, err
	}

	now := time.Now().UTC()
	if err := insertOperatorSessionAttribution(ctx, q, operatorAttributionSpec{
		MachineID:         in.MachineID,
		OrganizationID:    in.OrganizationID,
		OperatorSessionID: in.OperatorSessionID,
		ActionDomain:      "cash",
		ActionType:        "cash.collection.close",
		ResourceTable:     "cash_collections",
		ResourceID:        closed.ID.String(),
		CorrelationID:     in.CorrelationID,
		OccurredAt:        &now,
	}); err != nil {
		return db.CashCollection{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return db.CashCollection{}, err
	}
	return closed, nil
}

// ListMachineCashCollections returns recent cash collection rows for the machine (tenant-scoped).
func (s *Store) ListMachineCashCollections(ctx context.Context, organizationID, machineID uuid.UUID, limit, offset int32) ([]db.CashCollection, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("postgres: nil store")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	q := db.New(s.pool)
	return q.ListCashCollectionsForMachine(ctx, db.ListCashCollectionsForMachineParams{
		MachineID:      machineID,
		OrganizationID: organizationID,
		Limit:          limit,
		Offset:         offset,
	})
}

// GetMachineCashCollection returns one collection if it belongs to the org and machine.
func (s *Store) GetMachineCashCollection(ctx context.Context, organizationID, machineID, collectionID uuid.UUID) (db.CashCollection, error) {
	if s == nil || s.pool == nil {
		return db.CashCollection{}, errors.New("postgres: nil store")
	}
	q := db.New(s.pool)
	return q.GetCashCollectionByIDForOrgMachine(ctx, db.GetCashCollectionByIDForOrgMachineParams{
		ID:             collectionID,
		OrganizationID: organizationID,
		MachineID:      machineID,
	})
}

// FormatCloseRequestHashHex returns a stable hex representation for API responses.
func FormatCloseRequestHashHex(h []byte) string {
	if len(h) == 0 {
		return ""
	}
	return hex.EncodeToString(h)
}

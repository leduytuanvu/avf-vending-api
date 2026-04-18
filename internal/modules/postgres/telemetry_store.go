package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/avf/avf-vending-api/internal/gen/db"
)

// MachineOrgSite is tenant + site for a machine row.
type MachineOrgSite struct {
	OrganizationID uuid.UUID
	SiteID         uuid.UUID
}

// GetMachineOrgSite returns organization_id and site_id for envelope routing.
func (s *Store) GetMachineOrgSite(ctx context.Context, machineID uuid.UUID) (MachineOrgSite, error) {
	if s == nil || s.pool == nil {
		return MachineOrgSite{}, errors.New("postgres: nil store")
	}
	const q = `SELECT organization_id, site_id FROM machines WHERE id = $1`
	var row MachineOrgSite
	err := s.pool.QueryRow(ctx, q, machineID).Scan(&row.OrganizationID, &row.SiteID)
	if err != nil {
		return MachineOrgSite{}, err
	}
	return row, nil
}

// TouchMachineConnectivityFast updates machine online marker without heavy locks (used from telemetry bridge).
func (s *Store) TouchMachineConnectivityFast(ctx context.Context, machineID uuid.UUID) error {
	if s == nil || s.pool == nil {
		return errors.New("postgres: nil store")
	}
	const q = `UPDATE machines SET updated_at = now(),
		status = CASE WHEN status = 'offline' THEN 'online' ELSE status END
		WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, machineID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func jsonFingerprint(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// UpsertMachineCurrentSnapshotRow upserts the denormalized snapshot row (worker).
func (s *Store) UpsertMachineCurrentSnapshotRow(ctx context.Context, machineID, orgID, siteID uuid.UUID, reported, metrics []byte, repFp, metFp *string, hbAt *time.Time, appVer, fwVer *string) error {
	if s == nil || s.pool == nil {
		return errors.New("postgres: nil store")
	}
	const q = `
INSERT INTO machine_current_snapshot (
	machine_id, organization_id, site_id,
	reported_fingerprint, metrics_fingerprint,
	reported_state, metrics_state,
	last_heartbeat_at, app_version, firmware_version, updated_at
) VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7::jsonb,$8,$9,$10, now())
ON CONFLICT (machine_id) DO UPDATE SET
	reported_fingerprint = COALESCE(EXCLUDED.reported_fingerprint, machine_current_snapshot.reported_fingerprint),
	metrics_fingerprint = COALESCE(EXCLUDED.metrics_fingerprint, machine_current_snapshot.metrics_fingerprint),
	reported_state = EXCLUDED.reported_state,
	metrics_state = EXCLUDED.metrics_state,
	last_heartbeat_at = COALESCE(EXCLUDED.last_heartbeat_at, machine_current_snapshot.last_heartbeat_at),
	app_version = COALESCE(EXCLUDED.app_version, machine_current_snapshot.app_version),
	firmware_version = COALESCE(EXCLUDED.firmware_version, machine_current_snapshot.firmware_version),
	updated_at = now()
`
	_, err := s.pool.Exec(ctx, q, machineID, orgID, siteID, repFp, metFp, jsonOrEmpty(reported), jsonOrEmpty(metrics), hbAt, appVer, fwVer)
	return err
}

func jsonOrEmpty(b []byte) []byte {
	if len(b) == 0 {
		return []byte("{}")
	}
	return b
}

// ApplyShadowReportedProjection updates machine_shadow, machine_current_snapshot, and optional state transition.
func (s *Store) ApplyShadowReportedProjection(ctx context.Context, machineID uuid.UUID, reported []byte, appVer, fwVer *string) error {
	if s == nil || s.pool == nil {
		return errors.New("postgres: nil store")
	}
	fp := jsonFingerprint(reported)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)
	if _, err := q.GetMachineByIDForUpdate(ctx, machineID); err != nil {
		return err
	}
	if _, err := q.UpsertMachineShadowReported(ctx, db.UpsertMachineShadowReportedParams{
		MachineID:     machineID,
		ReportedState: reported,
	}); err != nil {
		return err
	}
	var loc MachineOrgSite
	if err := tx.QueryRow(ctx, `SELECT organization_id, site_id FROM machines WHERE id = $1`, machineID).Scan(&loc.OrganizationID, &loc.SiteID); err != nil {
		return err
	}
	var prev *string
	_ = tx.QueryRow(ctx, `SELECT reported_fingerprint FROM machine_current_snapshot WHERE machine_id = $1`, machineID).Scan(&prev)
	var fromJSON []byte
	if prev != nil {
		_ = tx.QueryRow(ctx, `SELECT reported_state FROM machine_current_snapshot WHERE machine_id = $1`, machineID).Scan(&fromJSON)
	}
	if prev == nil || *prev != fp {
		meta, _ := json.Marshal(map[string]any{"fingerprint": fp})
		if _, err := tx.Exec(ctx, `
INSERT INTO machine_state_transitions (machine_id, transition_key, from_value, to_value, metadata, occurred_at)
VALUES ($1,'shadow.reported',$2::jsonb,$3::jsonb,$4::jsonb, now())`,
			machineID, jsonOrEmpty(fromJSON), jsonOrEmpty(reported), meta); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO machine_current_snapshot (
	machine_id, organization_id, site_id, reported_fingerprint, reported_state, app_version, firmware_version, updated_at
) VALUES ($1,$2,$3,$4,$5::jsonb,$6,$7, now())
ON CONFLICT (machine_id) DO UPDATE SET
	reported_fingerprint = EXCLUDED.reported_fingerprint,
	reported_state = EXCLUDED.reported_state,
	app_version = COALESCE(EXCLUDED.app_version, machine_current_snapshot.app_version),
	firmware_version = COALESCE(EXCLUDED.firmware_version, machine_current_snapshot.firmware_version),
	updated_at = now()
`, machineID, loc.OrganizationID, loc.SiteID, fp, jsonOrEmpty(reported), appVer, fwVer); err != nil {
		return err
	}
	if err := q.TouchMachineConnectivity(ctx, machineID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) insertMachineIncident(ctx context.Context, machineID uuid.UUID, severity, code, title string, detail []byte, dedupe *string) error {
	const q = `INSERT INTO machine_incidents (machine_id, severity, code, title, detail, dedupe_key, opened_at, updated_at)
VALUES ($1,$2,$3,$4,$5::jsonb,$6, now(), now())`
	_, err := s.pool.Exec(ctx, q, machineID, severity, code, title, jsonOrEmpty(detail), dedupe)
	return err
}

// UpsertMachineIncidentDeduped inserts or updates by (machine_id, dedupe_key) when dedupe is set.
func (s *Store) UpsertMachineIncidentDeduped(ctx context.Context, machineID uuid.UUID, severity, code, title string, detail []byte, dedupe string) error {
	if strings.TrimSpace(dedupe) == "" {
		return s.insertMachineIncident(ctx, machineID, severity, code, title, detail, nil)
	}
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT id FROM machine_incidents WHERE machine_id = $1 AND dedupe_key = $2`, machineID, dedupe).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return s.insertMachineIncident(ctx, machineID, severity, code, title, detail, &dedupe)
	}
	if err != nil {
		return err
	}
	var titleArg any
	if strings.TrimSpace(title) != "" {
		titleArg = title
	}
	_, err = s.pool.Exec(ctx, `UPDATE machine_incidents SET severity = $1, code = $2, title = COALESCE($3, title), detail = $4::jsonb, updated_at = now() WHERE id = $5`,
		severity, code, titleArg, jsonOrEmpty(detail), id)
	return err
}

// UpsertHeartbeatSnapshot bumps last_heartbeat_at and ensures a snapshot row exists without clobbering reported state.
func (s *Store) UpsertHeartbeatSnapshot(ctx context.Context, machineID uuid.UUID, at time.Time) error {
	loc, err := s.GetMachineOrgSite(ctx, machineID)
	if err != nil {
		return err
	}
	const q = `
INSERT INTO machine_current_snapshot (
	machine_id, organization_id, site_id,
	reported_state, metrics_state, last_heartbeat_at, updated_at
) VALUES ($1,$2,$3,'{}'::jsonb,'{}'::jsonb,$4, now())
ON CONFLICT (machine_id) DO UPDATE SET
	last_heartbeat_at = EXCLUDED.last_heartbeat_at,
	updated_at = now()
`
	_, err = s.pool.Exec(ctx, q, machineID, loc.OrganizationID, loc.SiteID, at)
	return err
}

// MergeTelemetryRollupMinute adds samples into the 1-minute rollup bucket.
func (s *Store) MergeTelemetryRollupMinute(ctx context.Context, machineID uuid.UUID, bucketStart time.Time, metricKey string, sampleCount int64, sum, min, max, last *float64, extra []byte) error {
	if s == nil || s.pool == nil {
		return errors.New("postgres: nil store")
	}
	if extra == nil {
		extra = []byte("{}")
	}
	const q = `
INSERT INTO telemetry_rollups (machine_id, bucket_start, granularity, metric_key, sample_count, sum_val, min_val, max_val, last_val, extra)
VALUES ($1, date_trunc('minute', $2::timestamptz), '1m', $3, $4, $5, $6, $7, $8, $9::jsonb)
ON CONFLICT (machine_id, bucket_start, granularity, metric_key) DO UPDATE SET
	sample_count = telemetry_rollups.sample_count + EXCLUDED.sample_count,
	sum_val = COALESCE(telemetry_rollups.sum_val,0) + COALESCE(EXCLUDED.sum_val,0),
	min_val = CASE
		WHEN telemetry_rollups.min_val IS NULL THEN EXCLUDED.min_val
		WHEN EXCLUDED.min_val IS NULL THEN telemetry_rollups.min_val
		ELSE LEAST(telemetry_rollups.min_val, EXCLUDED.min_val) END,
	max_val = CASE
		WHEN telemetry_rollups.max_val IS NULL THEN EXCLUDED.max_val
		WHEN EXCLUDED.max_val IS NULL THEN telemetry_rollups.max_val
		ELSE GREATEST(telemetry_rollups.max_val, EXCLUDED.max_val) END,
	last_val = COALESCE(EXCLUDED.last_val, telemetry_rollups.last_val),
	extra = telemetry_rollups.extra || EXCLUDED.extra
`
	bucket := bucketStart.UTC().Truncate(time.Minute)
	_, err := s.pool.Exec(ctx, q, machineID, bucket, metricKey, sampleCount, sum, min, max, last, extra)
	return err
}

// InsertDiagnosticBundleManifestRow records cold-path metadata (blobs in object storage).
func (s *Store) InsertDiagnosticBundleManifestRow(ctx context.Context, machineID uuid.UUID, storageKey, provider, contentType string, size *int64, sha string, meta []byte, expires *time.Time) error {
	if s == nil || s.pool == nil {
		return errors.New("postgres: nil store")
	}
	if meta == nil {
		meta = []byte("{}")
	}
	const q = `
INSERT INTO diagnostic_bundle_manifests (machine_id, storage_key, storage_provider, content_type, size_bytes, sha256_hex, metadata, created_at, expires_at)
VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb, now(), $8)`
	_, err := s.pool.Exec(ctx, q, machineID, storageKey, provider, contentType, size, sha, meta, expires)
	return err
}

// TelemetrySnapshotRow is a read model for admin APIs.
type TelemetrySnapshotRow struct {
	MachineID         uuid.UUID
	OrganizationID    uuid.UUID
	SiteID            uuid.UUID
	ReportedState     []byte
	MetricsState      []byte
	LastHeartbeatAt   *time.Time
	AppVersion        *string
	FirmwareVersion   *string
	UpdatedAt         time.Time
}

// GetTelemetrySnapshot returns current snapshot or ErrNoRows.
func (s *Store) GetTelemetrySnapshot(ctx context.Context, machineID uuid.UUID) (TelemetrySnapshotRow, error) {
	const q = `SELECT machine_id, organization_id, site_id, reported_state, metrics_state, last_heartbeat_at, app_version, firmware_version, updated_at
FROM machine_current_snapshot WHERE machine_id = $1`
	var r TelemetrySnapshotRow
	err := s.pool.QueryRow(ctx, q, machineID).Scan(
		&r.MachineID, &r.OrganizationID, &r.SiteID, &r.ReportedState, &r.MetricsState,
		&r.LastHeartbeatAt, &r.AppVersion, &r.FirmwareVersion, &r.UpdatedAt,
	)
	return r, err
}

// MachineIncidentRow is a persisted incident read model.
type MachineIncidentRow struct {
	ID         uuid.UUID
	MachineID  uuid.UUID
	Severity   string
	Code       string
	Title      *string
	Detail     []byte
	DedupeKey  *string
	OpenedAt   time.Time
	UpdatedAt  time.Time
}

// TelemetryRollupRow is a persisted rollup read model.
type TelemetryRollupRow struct {
	MachineID    uuid.UUID
	BucketStart  time.Time
	Granularity  string
	MetricKey    string
	SampleCount  int64
	SumVal       *float64
	MinVal       *float64
	MaxVal       *float64
	LastVal      *float64
	Extra        []byte
}

// ListTelemetryRollupsInRange returns rollups for charts (not raw MQTT history).
func (s *Store) ListTelemetryRollupsInRange(ctx context.Context, machineID uuid.UUID, from, to time.Time, granularity string, limit int32) ([]TelemetryRollupRow, error) {
	if limit <= 0 || limit > 2000 {
		limit = 200
	}
	if granularity == "" {
		granularity = "1m"
	}
	rows, err := s.pool.Query(ctx, `
SELECT machine_id, bucket_start, granularity, metric_key, sample_count, sum_val, min_val, max_val, last_val, extra
FROM telemetry_rollups
WHERE machine_id = $1 AND granularity = $2 AND bucket_start >= $3 AND bucket_start < $4
ORDER BY bucket_start ASC, metric_key ASC
LIMIT $5`, machineID, granularity, from.UTC(), to.UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TelemetryRollupRow
	for rows.Next() {
		var r TelemetryRollupRow
		if err := rows.Scan(&r.MachineID, &r.BucketStart, &r.Granularity, &r.MetricKey, &r.SampleCount, &r.SumVal, &r.MinVal, &r.MaxVal, &r.LastVal, &r.Extra); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListMachineIncidentsRecent returns recent incidents for a machine.
func (s *Store) ListMachineIncidentsRecent(ctx context.Context, machineID uuid.UUID, limit int32) ([]MachineIncidentRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
SELECT id, machine_id, severity, code, title, detail, dedupe_key, opened_at, updated_at
FROM machine_incidents WHERE machine_id = $1 ORDER BY opened_at DESC LIMIT $2`, machineID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MachineIncidentRow
	for rows.Next() {
		var r MachineIncidentRow
		if err := rows.Scan(&r.ID, &r.MachineID, &r.Severity, &r.Code, &r.Title, &r.Detail, &r.DedupeKey, &r.OpenedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RunTelemetryRetention deletes aged telemetry projection rows (not financial OLTP).
func RunTelemetryRetention(ctx context.Context, pool *pgxpool.Pool, now time.Time) error {
	if pool == nil {
		return errors.New("postgres: nil pool")
	}
	// Defaults per ops/TELEMETRY_PIPELINE.md
	st := now.Add(-60 * 24 * time.Hour)  // state transitions 60d
	incLow := now.Add(-90 * 24 * time.Hour)
	incHi := now.Add(-180 * 24 * time.Hour)
	r1m := now.Add(-30 * 24 * time.Hour)
	r1h := now.Add(-180 * 24 * time.Hour)
	diag := now.Add(-365 * 24 * time.Hour)

	batch := func(name, q string, args ...any) error {
		tag, err := pool.Exec(ctx, q, args...)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if tag.RowsAffected() > 0 {
			// optional: log deleted count via caller
			_ = tag.RowsAffected()
		}
		return nil
	}
	if err := batch("prune_state_transitions", `DELETE FROM machine_state_transitions WHERE occurred_at < $1`, st); err != nil {
		return err
	}
	if err := batch("prune_incidents_low", `DELETE FROM machine_incidents WHERE severity IN ('low','medium','info') AND opened_at < $1`, incLow); err != nil {
		return err
	}
	if err := batch("prune_incidents_high", `DELETE FROM machine_incidents WHERE severity IN ('high','critical') AND opened_at < $1`, incHi); err != nil {
		return err
	}
	if err := batch("prune_rollups_1m", `DELETE FROM telemetry_rollups WHERE granularity = '1m' AND bucket_start < $1`, r1m); err != nil {
		return err
	}
	if err := batch("prune_rollups_1h", `DELETE FROM telemetry_rollups WHERE granularity = '1h' AND bucket_start < $1`, r1h); err != nil {
		return err
	}
	if err := batch("prune_diagnostic_manifests", `DELETE FROM diagnostic_bundle_manifests WHERE created_at < $1`, diag); err != nil {
		return err
	}
	return nil
}

// ParseMetricsPayload extracts numeric samples from a generic JSON payload for rollups.
func ParseMetricsPayload(payload []byte) map[string]float64 {
	out := make(map[string]float64)
	var root map[string]json.RawMessage
	if err := json.Unmarshal(payload, &root); err != nil {
		return out
	}
	if raw, ok := root["samples"]; ok {
		var samples map[string]float64
		if err := json.Unmarshal(raw, &samples); err == nil {
			return samples
		}
	}
	for k, v := range root {
		if k == "schema_version" || k == "event_type" {
			continue
		}
		var f float64
		if err := json.Unmarshal(v, &f); err == nil {
			out[k] = f
		}
	}
	return out
}

// ParseIncidentPayload extracts incident fields from device JSON.
func ParseIncidentPayload(payload []byte) (severity, code, title, dedupe string, err error) {
	var m struct {
		Severity string `json:"severity"`
		Code     string `json:"code"`
		Title    string `json:"title"`
		Dedupe   string `json:"dedupe_key"`
	}
	if err = json.Unmarshal(payload, &m); err != nil {
		return "", "", "", "", err
	}
	if strings.TrimSpace(m.Severity) == "" {
		m.Severity = "medium"
	}
	if strings.TrimSpace(m.Code) == "" {
		return "", "", "", "", fmt.Errorf("telemetry: incident.code required")
	}
	return strings.TrimSpace(m.Severity), strings.TrimSpace(m.Code), strings.TrimSpace(m.Title), strings.TrimSpace(m.Dedupe), nil
}

// ParseDiagnosticManifestPayload extracts cold storage pointer from MQTT JSON.
func ParseDiagnosticManifestPayload(payload []byte) (storageKey, provider, contentType string, size *int64, sha string, err error) {
	var m struct {
		StorageKey   string `json:"storage_key"`
		Provider     string `json:"storage_provider"`
		ContentType  string `json:"content_type"`
		SizeBytes    *int64 `json:"size_bytes"`
		SHA256       string `json:"sha256_hex"`
	}
	if err = json.Unmarshal(payload, &m); err != nil {
		return "", "", "", nil, "", err
	}
	if strings.TrimSpace(m.StorageKey) == "" {
		return "", "", "", nil, "", fmt.Errorf("telemetry: storage_key required")
	}
	if strings.TrimSpace(m.Provider) == "" {
		m.Provider = "s3"
	}
	return m.StorageKey, m.Provider, m.ContentType, m.SizeBytes, strings.TrimSpace(m.SHA256), nil
}

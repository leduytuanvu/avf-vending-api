package featureflags

import (
	"context"
	"encoding/json"
	"errors"
	"hash/crc32"
	"sort"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const listFlagsMax = 10000

// Service manages feature flags and machine config rollouts (tenant-scoped).
type Service struct {
	q     *db.Queries
	pool  *pgxpool.Pool
	audit compliance.EnterpriseRecorder
}

// NewService constructs the feature flag / rollout admin API backing service.
func NewService(q *db.Queries, pool *pgxpool.Pool, audit compliance.EnterpriseRecorder) (*Service, error) {
	if q == nil {
		return nil, errors.New("featureflags: nil queries")
	}
	if pool == nil {
		return nil, errors.New("featureflags: nil pool")
	}
	return &Service{q: q, pool: pool, audit: audit}, nil
}

func (s *Service) auditRec(ctx context.Context, org uuid.UUID, action, resourceType, resourceID string, before, after any, md map[string]any) {
	if s == nil || s.audit == nil {
		return
	}
	var b, a []byte
	if before != nil {
		b, _ = json.Marshal(before)
	}
	if after != nil {
		a, _ = json.Marshal(after)
	}
	meta, _ := json.Marshal(md)
	_ = s.audit.Record(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: org,
		ActorType:      "user",
		Action:         action,
		ResourceType:   resourceType,
		ResourceID:     strPtr(resourceID),
		BeforeJSON:     b,
		AfterJSON:      a,
		Metadata:       meta,
	})
}

func strPtr(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

// --- Feature flags ---

// FlagDTO is an API-facing feature flag row.
type FlagDTO struct {
	ID             uuid.UUID       `json:"id"`
	OrganizationID uuid.UUID       `json:"organizationId"`
	FlagKey        string          `json:"flagKey"`
	DisplayName    string          `json:"displayName"`
	Description    string          `json:"description"`
	Enabled        bool            `json:"enabled"`
	Metadata       json.RawMessage `json:"metadata"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

// TargetDTO is a scoped override row.
type TargetDTO struct {
	ID                uuid.UUID       `json:"id"`
	TargetType        string          `json:"targetType"`
	SiteID            *uuid.UUID      `json:"siteId,omitempty"`
	MachineID         *uuid.UUID      `json:"machineId,omitempty"`
	HardwareProfileID *uuid.UUID      `json:"hardwareProfileId,omitempty"`
	CanaryPercent     *float64        `json:"canaryPercent,omitempty"`
	Priority          int32           `json:"priority"`
	Enabled           bool            `json:"enabled"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
	CreatedAt         time.Time       `json:"createdAt"`
}

// FlagDetail combines a flag and optional targets.
type FlagDetail struct {
	Flag    FlagDTO     `json:"flag"`
	Targets []TargetDTO `json:"targets,omitempty"`
}

type ListFlagsParams struct {
	OrganizationID uuid.UUID
	Limit          int32
	Offset         int32
}

type ListFlagsResponse struct {
	Items []FlagDTO `json:"items"`
	Total int64     `json:"total"`
}

func flagDTO(row db.FeatureFlag) FlagDTO {
	md := json.RawMessage(row.Metadata)
	if len(md) == 0 || string(md) == "null" {
		md = json.RawMessage("{}")
	}
	return FlagDTO{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		FlagKey:        row.FlagKey,
		DisplayName:    row.DisplayName,
		Description:    row.Description,
		Enabled:        row.Enabled,
		Metadata:       md,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}

func targetDTO(t db.FeatureFlagTarget) TargetDTO {
	out := TargetDTO{
		ID:         t.ID,
		TargetType: t.TargetType,
		Priority:   t.Priority,
		Enabled:    t.Enabled,
		CreatedAt:  t.CreatedAt,
	}
	if t.SiteID.Valid {
		u := uuid.UUID(t.SiteID.Bytes)
		out.SiteID = &u
	}
	if t.MachineID.Valid {
		u := uuid.UUID(t.MachineID.Bytes)
		out.MachineID = &u
	}
	if t.HardwareProfileID.Valid {
		u := uuid.UUID(t.HardwareProfileID.Bytes)
		out.HardwareProfileID = &u
	}
	if cp, ok := numericToFloat64(t.CanaryPercent); ok {
		out.CanaryPercent = &cp
	}
	md := json.RawMessage(t.Metadata)
	if len(md) > 0 && string(md) != "null" {
		out.Metadata = md
	}
	return out
}

// ListFlags paginates flags for an organization.
func (s *Service) ListFlags(ctx context.Context, p ListFlagsParams) (*ListFlagsResponse, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("featureflags: nil service")
	}
	lim := p.Limit
	if lim <= 0 {
		lim = 50
	}
	if lim > 500 {
		lim = 500
	}
	off := p.Offset
	if off < 0 {
		off = 0
	}
	total, err := s.q.FeatureFlagsCountByOrganization(ctx, p.OrganizationID)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.FeatureFlagsListByOrganization(ctx, db.FeatureFlagsListByOrganizationParams{
		OrganizationID: p.OrganizationID,
		Limit:          lim,
		Offset:         off,
	})
	if err != nil {
		return nil, err
	}
	items := make([]FlagDTO, 0, len(rows))
	for _, r := range rows {
		items = append(items, flagDTO(r))
	}
	return &ListFlagsResponse{Items: items, Total: total}, nil
}

// GetFlag loads one flag and targets.
func (s *Service) GetFlag(ctx context.Context, orgID, flagID uuid.UUID) (*FlagDetail, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("featureflags: nil service")
	}
	row, err := s.q.FeatureFlagsGetByID(ctx, db.FeatureFlagsGetByIDParams{
		ID:             flagID,
		OrganizationID: orgID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	tg, err := s.q.FeatureFlagTargetsByFlagID(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	out := &FlagDetail{Flag: flagDTO(row)}
	for _, t := range tg {
		out.Targets = append(out.Targets, targetDTO(t))
	}
	return out, nil
}

type CreateFlagParams struct {
	OrganizationID uuid.UUID
	FlagKey        string
	DisplayName    string
	Description    string
	Enabled        bool
	Metadata       json.RawMessage
}

// CreateFlag inserts a flag (unique key per org).
func (s *Service) CreateFlag(ctx context.Context, p CreateFlagParams) (*FlagDTO, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("featureflags: nil service")
	}
	key := strings.TrimSpace(p.FlagKey)
	if key == "" {
		return nil, ErrInvalidTarget
	}
	meta := []byte(p.Metadata)
	if len(meta) == 0 || !json.Valid(meta) {
		meta = []byte("{}")
	}
	row, err := s.q.FeatureFlagsInsert(ctx, db.FeatureFlagsInsertParams{
		OrganizationID: p.OrganizationID,
		FlagKey:        key,
		DisplayName:    strings.TrimSpace(p.DisplayName),
		Description:    strings.TrimSpace(p.Description),
		Enabled:        p.Enabled,
		Metadata:       meta,
	})
	if err != nil {
		return nil, err
	}
	d := flagDTO(row)
	s.auditRec(ctx, p.OrganizationID, "feature_flag.create", "feature_flag", row.ID.String(), nil, d, nil)
	return &d, nil
}

type PatchFlagParams struct {
	OrganizationID uuid.UUID
	FlagID         uuid.UUID
	DisplayName    *string
	Description    *string
	Enabled        *bool
	MetadataJSON   *[]byte
}

// PatchFlag updates mutable fields.
func (s *Service) PatchFlag(ctx context.Context, p PatchFlagParams) (*FlagDTO, error) {
	cur, err := s.q.FeatureFlagsGetByID(ctx, db.FeatureFlagsGetByIDParams{
		ID:             p.FlagID,
		OrganizationID: p.OrganizationID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	before := flagDTO(cur)
	dn := cur.DisplayName
	if p.DisplayName != nil {
		dn = strings.TrimSpace(*p.DisplayName)
	}
	ds := cur.Description
	if p.Description != nil {
		ds = strings.TrimSpace(*p.Description)
	}
	en := cur.Enabled
	if p.Enabled != nil {
		en = *p.Enabled
	}
	meta := cur.Metadata
	if p.MetadataJSON != nil {
		m := *p.MetadataJSON
		if len(m) == 0 || !json.Valid(m) {
			return nil, ErrInvalidTarget
		}
		meta = m
	}
	row, err := s.q.FeatureFlagsUpdate(ctx, db.FeatureFlagsUpdateParams{
		ID:             p.FlagID,
		OrganizationID: p.OrganizationID,
		DisplayName:    dn,
		Description:    ds,
		Enabled:        en,
		Metadata:       meta,
	})
	if err != nil {
		return nil, err
	}
	after := flagDTO(row)
	s.auditRec(ctx, p.OrganizationID, "feature_flag.patch", "feature_flag", row.ID.String(), before, after, nil)
	return &after, nil
}

// SetEnabled sets master enabled bit (POST enable/disable shortcuts).
func (s *Service) SetEnabled(ctx context.Context, orgID, flagID uuid.UUID, enabled bool) (*FlagDTO, error) {
	return s.PatchFlag(ctx, PatchFlagParams{
		OrganizationID: orgID,
		FlagID:         flagID,
		Enabled:        &enabled,
	})
}

type PutTargetsParams struct {
	OrganizationID uuid.UUID
	FlagID         uuid.UUID
	Targets        []TargetInput
}

// TargetInput is the PUT body row shape.
type TargetInput struct {
	TargetType        string
	SiteID            *uuid.UUID
	MachineID         *uuid.UUID
	HardwareProfileID *uuid.UUID
	CanaryPercent     *float64
	Priority          int32
	Enabled           bool
	Metadata          json.RawMessage
}

// ReplaceTargets replaces all targets for a flag inside one transaction.
func (s *Service) ReplaceTargets(ctx context.Context, p PutTargetsParams) ([]TargetDTO, error) {
	if s == nil || s.pool == nil || s.q == nil {
		return nil, errors.New("featureflags: nil service")
	}
	_, err := s.q.FeatureFlagsGetByID(ctx, db.FeatureFlagsGetByIDParams{
		ID:             p.FlagID,
		OrganizationID: p.OrganizationID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	for _, t := range p.Targets {
		if err := validateTargetInput(t); err != nil {
			return nil, err
		}
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)
	if err := q.FeatureFlagTargetsDeleteByFlag(ctx, db.FeatureFlagTargetsDeleteByFlagParams{
		FeatureFlagID:  p.FlagID,
		OrganizationID: p.OrganizationID,
	}); err != nil {
		return nil, err
	}
	out := make([]TargetDTO, 0, len(p.Targets))
	for _, t := range p.Targets {
		meta := []byte(t.Metadata)
		if len(meta) == 0 || !json.Valid(meta) {
			meta = []byte("{}")
		}
		row, ierr := q.FeatureFlagTargetsInsert(ctx, db.FeatureFlagTargetsInsertParams{
			OrganizationID:    p.OrganizationID,
			FeatureFlagID:     p.FlagID,
			TargetType:        strings.TrimSpace(strings.ToLower(t.TargetType)),
			SiteID:            uuidPtrPg(t.SiteID),
			MachineID:         uuidPtrPg(t.MachineID),
			HardwareProfileID: uuidPtrPg(t.HardwareProfileID),
			CanaryPercent:     numericFromFloat64(t.CanaryPercent),
			Priority:          t.Priority,
			Enabled:           t.Enabled,
			Metadata:          meta,
		})
		if ierr != nil {
			return nil, ierr
		}
		out = append(out, targetDTO(row))
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.auditRec(ctx, p.OrganizationID, "feature_flag.targets_replace", "feature_flag", p.FlagID.String(), nil, out, map[string]any{"count": len(out)})
	return out, nil
}

func validateTargetInput(t TargetInput) error {
	tt := strings.TrimSpace(strings.ToLower(t.TargetType))
	switch tt {
	case "organization":
		if t.SiteID != nil || t.MachineID != nil || t.HardwareProfileID != nil || t.CanaryPercent != nil {
			return ErrInvalidTarget
		}
	case "site":
		if t.SiteID == nil || *t.SiteID == uuid.Nil || t.MachineID != nil || t.HardwareProfileID != nil || t.CanaryPercent != nil {
			return ErrInvalidTarget
		}
	case "machine":
		if t.MachineID == nil || *t.MachineID == uuid.Nil || t.SiteID != nil || t.HardwareProfileID != nil || t.CanaryPercent != nil {
			return ErrInvalidTarget
		}
	case "hardware_profile":
		if t.HardwareProfileID == nil || *t.HardwareProfileID == uuid.Nil || t.SiteID != nil || t.MachineID != nil || t.CanaryPercent != nil {
			return ErrInvalidTarget
		}
	case "canary":
		if t.CanaryPercent == nil || t.MachineID != nil || t.HardwareProfileID != nil {
			return ErrInvalidTarget
		}
	default:
		return ErrInvalidTarget
	}
	return nil
}

func uuidPtrPg(p *uuid.UUID) pgtype.UUID {
	if p == nil || *p == uuid.Nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *p, Valid: true}
}

// ResolveEffectiveFlags returns evaluated flag_key → enabled for a machine.
func (s *Service) ResolveEffectiveFlags(ctx context.Context, machineID uuid.UUID) (map[string]bool, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("featureflags: nil service")
	}
	ctxRow, err := s.q.FeatureFlagsResolveMachineContext(ctx, machineID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	flags, err := s.q.FeatureFlagsListByOrganization(ctx, db.FeatureFlagsListByOrganizationParams{
		OrganizationID: ctxRow.OrganizationID,
		Limit:          listFlagsMax,
		Offset:         0,
	})
	if err != nil {
		return nil, err
	}
	allTargets, err := s.q.FeatureFlagTargetsByOrganization(ctx, ctxRow.OrganizationID)
	if err != nil {
		return nil, err
	}
	byFlag := map[uuid.UUID][]db.FeatureFlagTarget{}
	for _, t := range allTargets {
		byFlag[t.FeatureFlagID] = append(byFlag[t.FeatureFlagID], t)
	}
	out := make(map[string]bool, len(flags))
	for _, f := range flags {
		tlist := byFlag[f.ID]
		sort.SliceStable(tlist, func(i, j int) bool {
			if tlist[i].Priority != tlist[j].Priority {
				return tlist[i].Priority > tlist[j].Priority
			}
			return tlist[i].CreatedAt.Before(tlist[j].CreatedAt)
		})
		effective := f.Enabled
		for _, tgt := range tlist {
			if !targetMatchesMachine(tgt, ctxRow, f.ID, machineID) {
				continue
			}
			effective = tgt.Enabled
			break
		}
		out[f.FlagKey] = effective
	}
	return out, nil
}

func targetMatchesMachine(t db.FeatureFlagTarget, mc db.FeatureFlagsResolveMachineContextRow, flagID uuid.UUID, machineID uuid.UUID) bool {
	switch strings.ToLower(strings.TrimSpace(t.TargetType)) {
	case "organization":
		return true
	case "site":
		return t.SiteID.Valid && uuid.UUID(t.SiteID.Bytes) == mc.SiteID
	case "machine":
		return t.MachineID.Valid && uuid.UUID(t.MachineID.Bytes) == machineID
	case "hardware_profile":
		if !t.HardwareProfileID.Valid || !mc.HardwareProfileID.Valid {
			return false
		}
		return uuid.UUID(t.HardwareProfileID.Bytes) == uuid.UUID(mc.HardwareProfileID.Bytes)
	case "canary":
		if t.SiteID.Valid && uuid.UUID(t.SiteID.Bytes) != mc.SiteID {
			return false
		}
		cp, ok := numericToFloat64(t.CanaryPercent)
		if !ok {
			return false
		}
		return CanaryHit(machineID, flagID, cp)
	default:
		return false
	}
}

// CanaryHit implements deterministic bucket sampling for canary_percent targets (exported for tests).
func CanaryHit(machineID, flagID uuid.UUID, pct float64) bool {
	if pct <= 0 {
		return false
	}
	if pct >= 100 {
		return true
	}
	h := crc32.ChecksumIEEE([]byte(machineID.String() + "|" + flagID.String()))
	return float64(h%100) < pct
}

// RuntimeHints bundles bootstrap/check-in extras without breaking legacy clients.
type RuntimeHints struct {
	FeatureFlags                 map[string]bool      `json:"featureFlags,omitempty"`
	AppliedMachineConfigRevision int32                `json:"appliedMachineConfigRevision,omitempty"`
	PendingMachineConfigRollouts []PendingRolloutHint `json:"pendingMachineConfigRollouts,omitempty"`
}

// PendingRolloutHint summarizes an active rollout affecting this machine.
type PendingRolloutHint struct {
	RolloutID          string `json:"rolloutId"`
	TargetVersionID    string `json:"targetVersionId"`
	TargetVersionLabel string `json:"targetVersionLabel,omitempty"`
	Status             string `json:"status"`
}

// RuntimeHintsForMachine builds hints for machine bootstrap / check-in responses.
func (s *Service) RuntimeHintsForMachine(ctx context.Context, machineID uuid.UUID) (*RuntimeHints, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("featureflags: nil service")
	}
	ff, err := s.ResolveEffectiveFlags(ctx, machineID)
	if err != nil {
		return nil, err
	}
	rev, err := s.q.MachineAppliedConfigRevision(ctx, machineID)
	if err != nil {
		return nil, err
	}
	pending, err := s.q.MachineConfigRolloutsPendingForMachine(ctx, machineID)
	if err != nil {
		return nil, err
	}
	hints := make([]PendingRolloutHint, 0, len(pending))
	for _, r := range pending {
		mc, err := s.q.MachineConfigVersionsGetByID(ctx, db.MachineConfigVersionsGetByIDParams{
			ID:             r.TargetVersionID,
			OrganizationID: r.OrganizationID,
		})
		label := ""
		if err == nil {
			label = mc.VersionLabel
		}
		hints = append(hints, PendingRolloutHint{
			RolloutID:          r.ID.String(),
			TargetVersionID:    r.TargetVersionID.String(),
			TargetVersionLabel: label,
			Status:             r.Status,
		})
	}
	return &RuntimeHints{
		FeatureFlags:                 ff,
		AppliedMachineConfigRevision: rev,
		PendingMachineConfigRollouts: hints,
	}, nil
}

// --- Machine config versions / rollouts ---

type MachineConfigVersionDTO struct {
	ID              uuid.UUID       `json:"id"`
	OrganizationID  uuid.UUID       `json:"organizationId"`
	VersionLabel    string          `json:"versionLabel"`
	ConfigPayload   json.RawMessage `json:"configPayload"`
	ParentVersionID *uuid.UUID      `json:"parentVersionId,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
}

func versionDTO(v db.MachineConfigVersion) MachineConfigVersionDTO {
	md := json.RawMessage(v.ConfigPayload)
	if len(md) == 0 || string(md) == "null" {
		md = json.RawMessage("{}")
	}
	out := MachineConfigVersionDTO{
		ID:             v.ID,
		OrganizationID: v.OrganizationID,
		VersionLabel:   v.VersionLabel,
		ConfigPayload:  md,
		CreatedAt:      v.CreatedAt,
	}
	if v.ParentVersionID.Valid {
		u := uuid.UUID(v.ParentVersionID.Bytes)
		out.ParentVersionID = &u
	}
	return out
}

type CreateMachineConfigVersionParams struct {
	OrganizationID  uuid.UUID
	VersionLabel    string
	ConfigPayload   json.RawMessage
	ParentVersionID *uuid.UUID
}

// CreateMachineConfigVersion inserts a logical config bundle row.
func (s *Service) CreateMachineConfigVersion(ctx context.Context, p CreateMachineConfigVersionParams) (*MachineConfigVersionDTO, error) {
	label := strings.TrimSpace(p.VersionLabel)
	if label == "" {
		return nil, ErrInvalidRollout
	}
	payload := []byte(p.ConfigPayload)
	if len(payload) == 0 || !json.Valid(payload) {
		payload = []byte("{}")
	}
	row, err := s.q.MachineConfigVersionsInsert(ctx, db.MachineConfigVersionsInsertParams{
		OrganizationID:  p.OrganizationID,
		VersionLabel:    label,
		ConfigPayload:   payload,
		ParentVersionID: uuidPtrPg(p.ParentVersionID),
	})
	if err != nil {
		return nil, err
	}
	d := versionDTO(row)
	s.auditRec(ctx, p.OrganizationID, "machine_config_version.create", "machine_config_version", row.ID.String(), nil, d, nil)
	return &d, nil
}

type MachineConfigRolloutDTO struct {
	ID                uuid.UUID       `json:"id"`
	OrganizationID    uuid.UUID       `json:"organizationId"`
	TargetVersionID   uuid.UUID       `json:"targetVersionId"`
	PreviousVersionID *uuid.UUID      `json:"previousVersionId,omitempty"`
	Status            string          `json:"status"`
	CanaryPercent     *float64        `json:"canaryPercent,omitempty"`
	ScopeType         string          `json:"scopeType"`
	SiteID            *uuid.UUID      `json:"siteId,omitempty"`
	MachineID         *uuid.UUID      `json:"machineId,omitempty"`
	HardwareProfileID *uuid.UUID      `json:"hardwareProfileId,omitempty"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
	CreatedAt         time.Time       `json:"createdAt"`
	UpdatedAt         time.Time       `json:"updatedAt"`
}

func rolloutDTO(r db.MachineConfigRollout) MachineConfigRolloutDTO {
	md := json.RawMessage(r.Metadata)
	if len(md) == 0 || string(md) == "null" {
		md = json.RawMessage("{}")
	}
	out := MachineConfigRolloutDTO{
		ID:              r.ID,
		OrganizationID:  r.OrganizationID,
		TargetVersionID: r.TargetVersionID,
		Status:          r.Status,
		ScopeType:       r.ScopeType,
		Metadata:        md,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
	if r.PreviousVersionID.Valid {
		u := uuid.UUID(r.PreviousVersionID.Bytes)
		out.PreviousVersionID = &u
	}
	if cp, ok := numericToFloat64(r.CanaryPercent); ok {
		out.CanaryPercent = &cp
	}
	if r.SiteID.Valid {
		u := uuid.UUID(r.SiteID.Bytes)
		out.SiteID = &u
	}
	if r.MachineID.Valid {
		u := uuid.UUID(r.MachineID.Bytes)
		out.MachineID = &u
	}
	if r.HardwareProfileID.Valid {
		u := uuid.UUID(r.HardwareProfileID.Bytes)
		out.HardwareProfileID = &u
	}
	return out
}

type CreateRolloutParams struct {
	OrganizationID    uuid.UUID
	TargetVersionID   uuid.UUID
	PreviousVersionID *uuid.UUID
	Status            string
	CanaryPercent     *float64
	ScopeType         string
	SiteID            *uuid.UUID
	MachineID         *uuid.UUID
	HardwareProfileID *uuid.UUID
	Metadata          json.RawMessage
}

// CreateRollout starts a rollout row (typically status=pending or in_progress).
func (s *Service) CreateRollout(ctx context.Context, p CreateRolloutParams) (*MachineConfigRolloutDTO, error) {
	if err := validateRolloutScope(p.ScopeType, p.SiteID, p.MachineID, p.HardwareProfileID); err != nil {
		return nil, err
	}
	meta := []byte(p.Metadata)
	if len(meta) == 0 || !json.Valid(meta) {
		meta = []byte("{}")
	}
	st := strings.TrimSpace(p.Status)
	if st == "" {
		st = "pending"
	}
	row, err := s.q.MachineConfigRolloutsInsert(ctx, db.MachineConfigRolloutsInsertParams{
		OrganizationID:    p.OrganizationID,
		TargetVersionID:   p.TargetVersionID,
		PreviousVersionID: uuidPtrPg(p.PreviousVersionID),
		Status:            st,
		CanaryPercent:     numericFromFloat64(p.CanaryPercent),
		ScopeType:         strings.TrimSpace(strings.ToLower(p.ScopeType)),
		SiteID:            uuidPtrPg(p.SiteID),
		MachineID:         uuidPtrPg(p.MachineID),
		HardwareProfileID: uuidPtrPg(p.HardwareProfileID),
		Metadata:          meta,
	})
	if err != nil {
		return nil, err
	}
	d := rolloutDTO(row)
	s.auditRec(ctx, p.OrganizationID, "machine_config_rollout.create", "machine_config_rollout", row.ID.String(), nil, d, nil)
	return &d, nil
}

func validateRolloutScope(scope string, siteID, machineID, hpID *uuid.UUID) error {
	sc := strings.TrimSpace(strings.ToLower(scope))
	switch sc {
	case "organization":
		if siteID != nil || machineID != nil || hpID != nil {
			return ErrInvalidRollout
		}
	case "site":
		if siteID == nil || *siteID == uuid.Nil || machineID != nil || hpID != nil {
			return ErrInvalidRollout
		}
	case "machine":
		if machineID == nil || *machineID == uuid.Nil || siteID != nil || hpID != nil {
			return ErrInvalidRollout
		}
	case "hardware_profile":
		if hpID == nil || *hpID == uuid.Nil || siteID != nil || machineID != nil {
			return ErrInvalidRollout
		}
	default:
		return ErrInvalidRollout
	}
	return nil
}

type ListRolloutsParams struct {
	OrganizationID uuid.UUID
	Limit          int32
	Offset         int32
}

type ListRolloutsResponse struct {
	Items []MachineConfigRolloutDTO `json:"items"`
	Total int64                     `json:"total"`
}

// ListRollouts paginates rollouts for an organization.
func (s *Service) ListRollouts(ctx context.Context, p ListRolloutsParams) (*ListRolloutsResponse, error) {
	lim := p.Limit
	if lim <= 0 {
		lim = 50
	}
	if lim > 500 {
		lim = 500
	}
	off := p.Offset
	if off < 0 {
		off = 0
	}
	total, err := s.q.MachineConfigRolloutsCountByOrganization(ctx, p.OrganizationID)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.MachineConfigRolloutsListByOrganization(ctx, db.MachineConfigRolloutsListByOrganizationParams{
		OrganizationID: p.OrganizationID,
		Limit:          lim,
		Offset:         off,
	})
	if err != nil {
		return nil, err
	}
	items := make([]MachineConfigRolloutDTO, 0, len(rows))
	for _, r := range rows {
		items = append(items, rolloutDTO(r))
	}
	return &ListRolloutsResponse{Items: items, Total: total}, nil
}

// GetRollout returns one rollout.
func (s *Service) GetRollout(ctx context.Context, orgID, rolloutID uuid.UUID) (*MachineConfigRolloutDTO, error) {
	row, err := s.q.MachineConfigRolloutsGetByID(ctx, db.MachineConfigRolloutsGetByIDParams{
		ID:             rolloutID,
		OrganizationID: orgID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	d := rolloutDTO(row)
	return &d, nil
}

// RollbackRollout marks rollout rolled_back and creates a compensating rollout targeting previous_version_id when present.
func (s *Service) RollbackRollout(ctx context.Context, orgID, rolloutID uuid.UUID) (*MachineConfigRolloutDTO, error) {
	cur, err := s.q.MachineConfigRolloutsGetByID(ctx, db.MachineConfigRolloutsGetByIDParams{
		ID:             rolloutID,
		OrganizationID: orgID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if !cur.PreviousVersionID.Valid {
		return nil, ErrInvalidRollout
	}
	prev := uuid.UUID(cur.PreviousVersionID.Bytes)

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)

	if _, err := q.MachineConfigRolloutsUpdateStatus(ctx, db.MachineConfigRolloutsUpdateStatusParams{
		ID:             rolloutID,
		OrganizationID: orgID,
		Status:         "rolled_back",
	}); err != nil {
		return nil, err
	}
	meta := []byte(`{"reason":"rollback"}`)
	row, err := q.MachineConfigRolloutsInsert(ctx, db.MachineConfigRolloutsInsertParams{
		OrganizationID:    orgID,
		TargetVersionID:   prev,
		PreviousVersionID: uuidPtrPg(&cur.TargetVersionID),
		Status:            "pending",
		CanaryPercent:     cur.CanaryPercent,
		ScopeType:         cur.ScopeType,
		SiteID:            cur.SiteID,
		MachineID:         cur.MachineID,
		HardwareProfileID: cur.HardwareProfileID,
		Metadata:          meta,
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	d := rolloutDTO(row)
	s.auditRec(ctx, orgID, "machine_config_rollout.rollback", "machine_config_rollout", row.ID.String(),
		map[string]string{"fromRolloutId": rolloutID.String()}, d, nil)
	return &d, nil
}

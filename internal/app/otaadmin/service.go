package otaadmin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	appfleetadmin "github.com/avf/avf-vending-api/internal/app/fleetadmin"
	"github.com/avf/avf-vending-api/internal/app/listscope"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	domaindevice "github.com/avf/avf-vending-api/internal/domain/device"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	cmdOTAApply    = "OTA_APPLY"
	cmdOTARollback = "OTA_ROLLBACK"

	strategyImmediate = "immediate"
	strategyCanary    = "canary"

	statusDraft      = "draft"
	statusApproved   = "approved"
	statusRunning    = "running"
	statusPaused     = "paused"
	statusCompleted  = "completed"
	statusFailed     = "failed"
	statusCancelled  = "cancelled"
	statusRolledBack = "rolled_back"

	waveForward  = "forward"
	waveRollback = "rollback"

	resDispatched = "dispatched"
)

// Service manages OTA campaigns and rollout via command_ledger.
type Service struct {
	q     *db.Queries
	pool  *pgxpool.Pool
	wf    domaindevice.CommandShadowWorkflow
	audit compliance.EnterpriseRecorder
}

// NewService wires OTA admin APIs. wf must append commands + shadow (typically postgres.Store).
func NewService(q *db.Queries, pool *pgxpool.Pool, wf domaindevice.CommandShadowWorkflow, audit compliance.EnterpriseRecorder) (*Service, error) {
	if q == nil {
		return nil, errors.New("otaadmin: nil queries")
	}
	if pool == nil {
		return nil, errors.New("otaadmin: nil pool")
	}
	if wf == nil {
		return nil, errors.New("otaadmin: nil workflow")
	}
	return &Service{q: q, pool: pool, wf: wf, audit: audit}, nil
}

func timeRangeOrAll(from, to *time.Time) (time.Time, time.Time) {
	start := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)
	if from != nil {
		start = from.UTC()
	}
	if to != nil {
		end = to.UTC()
	}
	return start, end
}

func pgText(s string) pgtype.Text {
	t := strings.TrimSpace(s)
	if t == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: t, Valid: true}
}

func textPtrFromPg(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

func uuidPg(u uuid.UUID) pgtype.UUID {
	if u == uuid.Nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: u, Valid: true}
}

func uuidPtrPg(u *uuid.UUID) pgtype.UUID {
	if u == nil || *u == uuid.Nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *u, Valid: true}
}

func canaryFirstWaveSize(n int, pct int32, strategy string) int {
	if n == 0 {
		return 0
	}
	if strategy == strategyImmediate || pct >= 100 {
		return n
	}
	if pct <= 0 {
		return n
	}
	k := (int64(n)*int64(pct) + 99) / 100
	if k < 1 {
		k = 1
	}
	if int(k) > n {
		return n
	}
	return int(k)
}

// ListOTA implements legacy GET /v1/admin/ota (backward compatible).
func (s *Service) ListOTA(ctx context.Context, scope listscope.AdminFleet) (*appfleetadmin.OTAListResponse, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("otaadmin: nil service")
	}
	if scope.OrganizationID == uuid.Nil {
		return nil, listscope.ErrAdminOrganizationRequired
	}
	st, en := timeRangeOrAll(scope.From, scope.To)
	filterStatus := strings.TrimSpace(scope.Status) != ""

	listArg := db.FleetAdminListOTACampaignsParams{
		OrganizationID: scope.OrganizationID,
		Column2:        filterStatus,
		Column3:        strings.TrimSpace(scope.Status),
		Column4:        st,
		Column5:        en,
		Limit:          scope.Limit,
		Offset:         scope.Offset,
	}
	countArg := db.FleetAdminCountOTACampaignsParams{
		OrganizationID: scope.OrganizationID,
		Column2:        filterStatus,
		Column3:        strings.TrimSpace(scope.Status),
		Column4:        st,
		Column5:        en,
	}
	rows, err := s.q.FleetAdminListOTACampaigns(ctx, listArg)
	if err != nil {
		return nil, err
	}
	total, err := s.q.FleetAdminCountOTACampaigns(ctx, countArg)
	if err != nil {
		return nil, err
	}
	items := make([]appfleetadmin.AdminOTAListItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, appfleetadmin.AdminOTAListItem{
			CampaignID:         r.CampaignID.String(),
			OrganizationID:     r.OrganizationID.String(),
			CampaignName:       r.CampaignName,
			Strategy:           r.Strategy,
			CampaignStatus:     r.CampaignStatus,
			CreatedAt:          r.CreatedAt.UTC(),
			ArtifactID:         r.ArtifactID.String(),
			ArtifactSemver:     textPtrFromPg(r.ArtifactSemver),
			ArtifactStorageKey: r.ArtifactStorageKey,
		})
	}
	return &appfleetadmin.OTAListResponse{
		Items: items,
		Meta: listscope.CollectionMeta{
			Limit:    scope.Limit,
			Offset:   scope.Offset,
			Returned: len(items),
			Total:    total,
		},
	}, nil
}

// ListCampaigns returns richer rows for GET /v1/admin/ota/campaigns.
func (s *Service) ListCampaigns(ctx context.Context, p CampaignListParams) (*CampaignListResponse, error) {
	if s == nil {
		return nil, errors.New("otaadmin: nil service")
	}
	if p.OrganizationID == uuid.Nil {
		return nil, listscope.ErrAdminOrganizationRequired
	}
	st, en := timeRangeOrAll(p.From, p.To)
	filter := strings.TrimSpace(p.Status) != ""
	rows, err := s.q.OtaAdminListCampaigns(ctx, db.OtaAdminListCampaignsParams{
		OrganizationID: p.OrganizationID,
		Column2:        filter,
		Column3:        strings.TrimSpace(p.Status),
		Column4:        st,
		Column5:        en,
		Limit:          p.Limit,
		Offset:         p.Offset,
	})
	if err != nil {
		return nil, err
	}
	total, err := s.q.OtaAdminCountCampaigns(ctx, db.OtaAdminCountCampaignsParams{
		OrganizationID: p.OrganizationID,
		Column2:        filter,
		Column3:        strings.TrimSpace(p.Status),
		Column4:        st,
		Column5:        en,
	})
	if err != nil {
		return nil, err
	}
	items := make([]CampaignListItem, 0, len(rows))
	for _, r := range rows {
		var appr *time.Time
		if r.ApprovedAt.Valid {
			t := r.ApprovedAt.Time.UTC()
			appr = &t
		}
		items = append(items, CampaignListItem{
			CampaignID:         r.CampaignID.String(),
			OrganizationID:     r.OrganizationID.String(),
			Name:               r.CampaignName,
			RolloutStrategy:    r.RolloutStrategy,
			Status:             r.CampaignStatus,
			CampaignType:       r.CampaignType,
			CanaryPercent:      r.CanaryPercent,
			RolloutNextOffset:  r.RolloutNextOffset,
			CreatedAt:          r.CreatedAt.UTC(),
			UpdatedAt:          r.UpdatedAt.UTC(),
			ApprovedAt:         appr,
			ArtifactID:         r.ArtifactID.String(),
			ArtifactSemver:     textPtrFromPg(r.ArtifactSemver),
			ArtifactStorageKey: r.ArtifactStorageKey,
			ArtifactVersion:    textPtrFromPg(r.ArtifactVersion),
			RollbackArtifactID: uuidPtrToStrPtr(r.RollbackArtifactID),
		})
	}
	return &CampaignListResponse{
		Items: items,
		Meta: listscope.CollectionMeta{
			Limit:    p.Limit,
			Offset:   p.Offset,
			Returned: len(items),
			Total:    total,
		},
	}, nil
}

func uuidPtrToStrPtr(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := uuid.UUID(u.Bytes).String()
	return &s
}

func uuidPgPtrStr(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := uuid.UUID(u.Bytes).String()
	return &s
}

func listItemFromCampaignAndArtifact(c db.OtaCampaign, art db.OtaArtifact) CampaignListItem {
	var appr *time.Time
	if c.ApprovedAt.Valid {
		t := c.ApprovedAt.Time.UTC()
		appr = &t
	}
	return CampaignListItem{
		CampaignID:         c.ID.String(),
		OrganizationID:     c.OrganizationID.String(),
		Name:               c.Name,
		RolloutStrategy:    c.RolloutStrategy,
		Status:             c.Status,
		CampaignType:       c.CampaignType,
		CanaryPercent:      c.CanaryPercent,
		RolloutNextOffset:  c.RolloutNextOffset,
		CreatedAt:          c.CreatedAt.UTC(),
		UpdatedAt:          c.UpdatedAt.UTC(),
		ApprovedAt:         appr,
		ArtifactID:         c.ArtifactID.String(),
		ArtifactSemver:     textPtrFromPg(art.Semver),
		ArtifactStorageKey: art.StorageKey,
		ArtifactVersion:    textPtrFromPg(c.ArtifactVersion),
		RollbackArtifactID: uuidPtrToStrPtr(c.RollbackArtifactID),
	}
}

func detailFromCampaignAndArtifact(c db.OtaCampaign, art db.OtaArtifact) CampaignDetail {
	d := CampaignDetail{CampaignListItem: listItemFromCampaignAndArtifact(c, art)}
	d.CreatedBy = uuidPgPtrStr(c.CreatedBy)
	d.ApprovedBy = uuidPgPtrStr(c.ApprovedBy)
	if c.PausedAt.Valid {
		t := c.PausedAt.Time.UTC()
		d.PausedAt = &t
	}
	return d
}

func (s *Service) campaignDetail(ctx context.Context, orgID uuid.UUID, c db.OtaCampaign) (CampaignDetail, error) {
	art, err := s.q.OtaAdminGetArtifactForOrg(ctx, db.OtaAdminGetArtifactForOrgParams{
		OrganizationID: orgID,
		ID:             c.ArtifactID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CampaignDetail{}, ErrNotFound
		}
		return CampaignDetail{}, err
	}
	return detailFromCampaignAndArtifact(c, art), nil
}

func (s *Service) emitEvent(ctx context.Context, q *db.Queries, orgID, campaignID uuid.UUID, typ string, payload map[string]any, actor pgtype.UUID) {
	b, _ := json.Marshal(payload)
	_, _ = q.OtaAdminInsertCampaignEvent(ctx, db.OtaAdminInsertCampaignEventParams{
		OrganizationID: orgID,
		CampaignID:     campaignID,
		EventType:      typ,
		Payload:        b,
		ActorID:        actor,
	})
	s.maybeEnterpriseCampaignAudit(ctx, orgID, campaignID, typ, payload)
}

func (s *Service) maybeEnterpriseCampaignAudit(ctx context.Context, orgID, campaignID uuid.UUID, typ string, payload map[string]any) {
	if s == nil || s.audit == nil {
		return
	}
	action := mapOTACampaignEnterpriseAction(typ)
	if action == "" {
		return
	}
	cid := campaignID.String()
	md, err := json.Marshal(payload)
	if err != nil {
		md = []byte("{}")
	}
	md = compliance.SanitizeJSONBytes(md)
	_ = s.audit.Record(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: orgID,
		ActorType:      compliance.ActorSystem,
		Action:         action,
		ResourceType:   "ota.campaign",
		ResourceID:     &cid,
		Metadata:       md,
	})
}

func mapOTACampaignEnterpriseAction(typ string) string {
	switch typ {
	case "campaign_started":
		return compliance.ActionOTACampaignStarted
	case "campaign_paused":
		return compliance.ActionOTACampaignPaused
	case "campaign_resumed":
		return compliance.ActionOTACampaignResumed
	case "campaign_cancelled":
		return compliance.ActionOTACampaignCancelled
	case "campaign_rollback":
		return compliance.ActionOTACampaignRollback
	default:
		return ""
	}
}

func (s *Service) getCampaign(ctx context.Context, orgID, id uuid.UUID) (db.OtaCampaign, error) {
	row, err := s.q.OtaAdminGetCampaign(ctx, db.OtaAdminGetCampaignParams{OrganizationID: orgID, ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.OtaCampaign{}, ErrNotFound
		}
		return db.OtaCampaign{}, err
	}
	return row, nil
}

func mutableTargets(status string) bool {
	switch status {
	case statusDraft, statusApproved:
		return true
	default:
		return false
	}
}

func (s *Service) CreateCampaign(ctx context.Context, in CreateCampaignInput) (CampaignDetail, error) {
	if s == nil {
		return CampaignDetail{}, errors.New("otaadmin: nil service")
	}
	if in.OrganizationID == uuid.Nil || strings.TrimSpace(in.Name) == "" || in.ArtifactID == uuid.Nil {
		return CampaignDetail{}, ErrInvalidArgument
	}
	if _, err := s.q.OtaAdminGetArtifactForOrg(ctx, db.OtaAdminGetArtifactForOrgParams{
		OrganizationID: in.OrganizationID,
		ID:             in.ArtifactID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CampaignDetail{}, ErrNotFound
		}
		return CampaignDetail{}, err
	}
	ct := strings.TrimSpace(strings.ToLower(in.CampaignType))
	if ct == "" {
		ct = "app"
	}
	rs := strings.TrimSpace(strings.ToLower(in.RolloutStrategy))
	if rs == "" {
		rs = strategyCanary
	}
	if rs != strategyImmediate && rs != strategyCanary {
		return CampaignDetail{}, fmt.Errorf("%w: rolloutStrategy must be immediate or canary", ErrInvalidArgument)
	}
	av := pgText("")
	if in.ArtifactVersion != nil {
		av = pgText(*in.ArtifactVersion)
	}
	rb := uuidPtrPg(in.RollbackArtifactID)
	if rb.Valid {
		if _, err := s.q.OtaAdminGetArtifactForOrg(ctx, db.OtaAdminGetArtifactForOrgParams{
			OrganizationID: in.OrganizationID,
			ID:             uuid.UUID(rb.Bytes),
		}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return CampaignDetail{}, ErrNotFound
			}
			return CampaignDetail{}, err
		}
	}
	row, err := s.q.OtaAdminInsertCampaign(ctx, db.OtaAdminInsertCampaignParams{
		OrganizationID:     in.OrganizationID,
		Name:               strings.TrimSpace(in.Name),
		ArtifactID:         in.ArtifactID,
		ArtifactVersion:    av,
		CampaignType:       ct,
		RolloutStrategy:    rs,
		CanaryPercent:      in.CanaryPercent,
		RollbackArtifactID: rb,
		CreatedBy:          uuidPg(in.CreatedBy),
		Status:             statusDraft,
	})
	if err != nil {
		return CampaignDetail{}, err
	}
	s.emitEvent(ctx, s.q, in.OrganizationID, row.ID, "campaign_created", map[string]any{"name": row.Name}, uuidPg(in.CreatedBy))
	return s.campaignDetail(ctx, in.OrganizationID, row)
}

// GetCampaignDetail returns one campaign with artifact metadata.
func (s *Service) GetCampaignDetail(ctx context.Context, orgID, id uuid.UUID) (CampaignDetail, error) {
	row, err := s.getCampaign(ctx, orgID, id)
	if err != nil {
		return CampaignDetail{}, err
	}
	return s.campaignDetail(ctx, orgID, row)
}

// PatchCampaign updates editable fields on draft/approved campaigns only.
func (s *Service) PatchCampaign(ctx context.Context, orgID, id uuid.UUID, patch PatchCampaignInput) (CampaignDetail, error) {
	cur, err := s.getCampaign(ctx, orgID, id)
	if err != nil {
		return CampaignDetail{}, err
	}
	if !mutableTargets(cur.Status) {
		return CampaignDetail{}, ErrTargetsLocked
	}
	name := cur.Name
	if patch.Name != nil && strings.TrimSpace(*patch.Name) != "" {
		name = strings.TrimSpace(*patch.Name)
	}
	av := cur.ArtifactVersion
	if patch.ArtifactVersion != nil {
		av = pgText(*patch.ArtifactVersion)
	}
	ct := cur.CampaignType
	if patch.CampaignType != nil {
		ct = strings.TrimSpace(strings.ToLower(*patch.CampaignType))
	}
	rs := cur.RolloutStrategy
	if patch.RolloutStrategy != nil {
		rs = strings.TrimSpace(strings.ToLower(*patch.RolloutStrategy))
		if rs != strategyImmediate && rs != strategyCanary {
			return CampaignDetail{}, fmt.Errorf("%w: rolloutStrategy must be immediate or canary", ErrInvalidArgument)
		}
	}
	cp := cur.CanaryPercent
	if patch.CanaryPercent != nil {
		cp = *patch.CanaryPercent
	}
	rb := cur.RollbackArtifactID
	if patch.RollbackArtifactID != nil {
		if *patch.RollbackArtifactID != uuid.Nil {
			if _, err := s.q.OtaAdminGetArtifactForOrg(ctx, db.OtaAdminGetArtifactForOrgParams{
				OrganizationID: orgID,
				ID:             *patch.RollbackArtifactID,
			}); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return CampaignDetail{}, ErrNotFound
				}
				return CampaignDetail{}, err
			}
		}
		rb = uuidPtrPg(patch.RollbackArtifactID)
	}
	row, err := s.q.OtaAdminUpdateCampaignPatch(ctx, db.OtaAdminUpdateCampaignPatchParams{
		OrganizationID:     orgID,
		ID:                 id,
		Name:               name,
		ArtifactVersion:    av,
		CampaignType:       ct,
		RolloutStrategy:    rs,
		CanaryPercent:      cp,
		RollbackArtifactID: rb,
	})
	if err != nil {
		return CampaignDetail{}, err
	}
	s.emitEvent(ctx, s.q, orgID, id, "campaign_patched", map[string]any{}, pgtype.UUID{})
	return s.campaignDetail(ctx, orgID, row)
}

// PutCampaignTargets replaces targets (draft/approved).
func (s *Service) PutCampaignTargets(ctx context.Context, in PutTargetsInput) error {
	cur, err := s.getCampaign(ctx, in.OrganizationID, in.CampaignID)
	if err != nil {
		return err
	}
	if !mutableTargets(cur.Status) {
		return ErrTargetsLocked
	}
	if len(in.MachineIDs) == 0 {
		return ErrNoTargets
	}
	valid, err := s.q.OtaAdminValidateMachinesBelongToOrg(ctx, db.OtaAdminValidateMachinesBelongToOrgParams{
		OrganizationID: in.OrganizationID,
		Column2:        in.MachineIDs,
	})
	if err != nil {
		return err
	}
	if len(valid) != len(in.MachineIDs) {
		return ErrMachinesNotInOrg
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := db.New(tx)
	if err := qtx.OtaAdminDeleteTargetsForCampaign(ctx, in.CampaignID); err != nil {
		return err
	}
	for _, mid := range in.MachineIDs {
		if _, err := qtx.OtaAdminInsertCampaignTarget(ctx, db.OtaAdminInsertCampaignTargetParams{
			CampaignID: in.CampaignID,
			MachineID:  mid,
		}); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.emitEvent(ctx, s.q, in.OrganizationID, in.CampaignID, "targets_replaced", map[string]any{"count": len(in.MachineIDs)}, pgtype.UUID{})
	return nil
}

func (s *Service) ListCampaignTargets(ctx context.Context, orgID, campaignID uuid.UUID) ([]CampaignTargetItem, error) {
	if _, err := s.getCampaign(ctx, orgID, campaignID); err != nil {
		return nil, err
	}
	rows, err := s.q.OtaAdminListCampaignTargetsSorted(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	out := make([]CampaignTargetItem, 0, len(rows))
	for _, t := range rows {
		it := CampaignTargetItem{
			MachineID: t.MachineID.String(),
			State:     t.State,
			UpdatedAt: t.UpdatedAt.UTC(),
		}
		it.LastError = textPtrFromPg(t.LastError)
		out = append(out, it)
	}
	return out, nil
}

func (s *Service) ListCampaignResults(ctx context.Context, orgID, campaignID uuid.UUID) ([]MachineResultItem, error) {
	if _, err := s.getCampaign(ctx, orgID, campaignID); err != nil {
		return nil, err
	}
	rows, err := s.q.OtaAdminListMachineResultsForCampaign(ctx, db.OtaAdminListMachineResultsForCampaignParams{
		OrganizationID: orgID,
		CampaignID:     campaignID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]MachineResultItem, 0, len(rows))
	for _, r := range rows {
		it := MachineResultItem{
			MachineID: r.MachineID.String(),
			Wave:      r.Wave,
			Status:    r.Status,
			UpdatedAt: r.UpdatedAt.UTC(),
			CreatedAt: r.CreatedAt.UTC(),
		}
		it.LastError = textPtrFromPg(r.LastError)
		if r.CommandID.Valid {
			s := uuid.UUID(r.CommandID.Bytes).String()
			it.CommandID = &s
		}
		out = append(out, it)
	}
	return out, nil
}

// PublishCampaign moves a draft campaign through approval (when needed) and starts the first rollout wave.
func (s *Service) PublishCampaign(ctx context.Context, orgID, campaignID, actor uuid.UUID) (CampaignDetail, error) {
	cur, err := s.getCampaign(ctx, orgID, campaignID)
	if err != nil {
		return CampaignDetail{}, err
	}
	switch cur.Status {
	case statusDraft:
		if _, err := s.ApproveCampaign(ctx, orgID, campaignID, actor); err != nil {
			return CampaignDetail{}, err
		}
		return s.StartCampaign(ctx, orgID, campaignID)
	case statusApproved:
		return s.StartCampaign(ctx, orgID, campaignID)
	default:
		return CampaignDetail{}, ErrInvalidTransition
	}
}

func (s *Service) ApproveCampaign(ctx context.Context, orgID, campaignID, actor uuid.UUID) (CampaignDetail, error) {
	cur, err := s.getCampaign(ctx, orgID, campaignID)
	if err != nil {
		return CampaignDetail{}, err
	}
	if cur.Status != statusDraft {
		return CampaignDetail{}, ErrInvalidTransition
	}
	now := time.Now().UTC()
	row, err := s.q.OtaAdminUpdateCampaignStatusFields(ctx, db.OtaAdminUpdateCampaignStatusFieldsParams{
		OrganizationID:    orgID,
		ID:                campaignID,
		Status:            statusApproved,
		ApprovedBy:        uuidPg(actor),
		ApprovedAt:        pgtype.Timestamptz{Time: now, Valid: true},
		RolloutNextOffset: cur.RolloutNextOffset,
		PausedAt:          cur.PausedAt,
	})
	if err != nil {
		return CampaignDetail{}, err
	}
	s.emitEvent(ctx, s.q, orgID, campaignID, "campaign_approved", map[string]any{}, uuidPg(actor))
	return s.campaignDetail(ctx, orgID, row)
}

func (s *Service) StartCampaign(ctx context.Context, orgID, campaignID uuid.UUID) (CampaignDetail, error) {
	cur, err := s.getCampaign(ctx, orgID, campaignID)
	if err != nil {
		return CampaignDetail{}, err
	}
	if cur.Status != statusApproved {
		return CampaignDetail{}, ErrNeedsApproval
	}
	targets, err := s.q.OtaAdminListCampaignTargetsSorted(ctx, campaignID)
	if err != nil {
		return CampaignDetail{}, err
	}
	mids := make([]uuid.UUID, 0, len(targets))
	for _, t := range targets {
		mids = append(mids, t.MachineID)
	}
	n := len(mids)
	if n == 0 {
		return CampaignDetail{}, ErrNoTargets
	}
	primary, err := s.q.OtaAdminGetArtifactForOrg(ctx, db.OtaAdminGetArtifactForOrgParams{
		OrganizationID: orgID,
		ID:             cur.ArtifactID,
	})
	if err != nil {
		return CampaignDetail{}, err
	}
	first := canaryFirstWaveSize(n, cur.CanaryPercent, cur.RolloutStrategy)
	wave := mids[:first]
	phase := "canary"
	if cur.RolloutStrategy == strategyImmediate || first >= n {
		phase = "full"
	}
	if err := s.dispatchOTACommands(ctx, orgID, cur, primary, wave, waveForward, phase); err != nil {
		return CampaignDetail{}, err
	}
	nextSt := statusRunning
	nextOff := int32(first)
	if first >= n {
		nextSt = statusCompleted
		nextOff = int32(n)
	}
	row, err := s.q.OtaAdminUpdateCampaignStatusFields(ctx, db.OtaAdminUpdateCampaignStatusFieldsParams{
		OrganizationID:    orgID,
		ID:                campaignID,
		Status:            nextSt,
		ApprovedBy:        cur.ApprovedBy,
		ApprovedAt:        cur.ApprovedAt,
		RolloutNextOffset: nextOff,
		PausedAt:          pgtype.Timestamptz{},
	})
	if err != nil {
		return CampaignDetail{}, err
	}
	s.emitEvent(ctx, s.q, orgID, campaignID, "campaign_started", map[string]any{"wave_size": len(wave)}, pgtype.UUID{})
	return s.campaignDetail(ctx, orgID, row)
}

func (s *Service) dispatchOTACommands(ctx context.Context, orgID uuid.UUID, camp db.OtaCampaign, art db.OtaArtifact, machines []uuid.UUID, wave, phase string) error {
	semver := ""
	if art.Semver.Valid {
		semver = art.Semver.String
	}
	ver := semver
	if camp.ArtifactVersion.Valid && strings.TrimSpace(camp.ArtifactVersion.String) != "" {
		ver = strings.TrimSpace(camp.ArtifactVersion.String)
	}
	for i, mid := range machines {
		payload, err := json.Marshal(map[string]any{
			"campaign_id":      camp.ID.String(),
			"artifact_id":      art.ID.String(),
			"storage_key":      art.StorageKey,
			"semver":           semver,
			"artifact_version": ver,
			"campaign_type":    camp.CampaignType,
			"wave":             wave,
			"phase":            phase,
			"wave_index":       i,
		})
		if err != nil {
			return err
		}
		idem := fmt.Sprintf("ota:%s:%s:%s:%d", camp.ID.String(), mid.String(), wave, camp.RolloutNextOffset+int32(i))
		res, err := s.wf.AppendCommandUpdateShadow(ctx, domaindevice.AppendCommandInput{
			MachineID:      mid,
			CommandType:    cmdOTAApply,
			Payload:        payload,
			IdempotencyKey: idem,
			DesiredState:   []byte(`{}`),
		})
		if err != nil {
			return err
		}
		cmdID := uuidPg(res.CommandID)
		_, err = s.q.OtaAdminUpsertMachineResult(ctx, db.OtaAdminUpsertMachineResultParams{
			OrganizationID: orgID,
			CampaignID:     camp.ID,
			MachineID:      mid,
			Wave:           wave,
			CommandID:      cmdID,
			Status:         resDispatched,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

package rollout

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	appaudit "github.com/avf/avf-vending-api/internal/app/audit"
	appdevice "github.com/avf/avf-vending-api/internal/app/device"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	domaindevice "github.com/avf/avf-vending-api/internal/domain/device"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	commandFleetRolloutApply = "fleet_rollout_apply"
	dispatchChunk            = 50
	maxDispatchLoops         = 5000
)

func rolloutCampaignDispatchShouldStop(status string) bool {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "paused", "cancelled":
		return true
	default:
		return false
	}
}

func computeCanaryCount(n int, p float64) int {
	k := int(math.Ceil(float64(n) * p / 100.0))
	if k < 1 {
		k = 1
	}
	if k > n {
		k = n
	}
	return k
}

// Service orchestrates rollout campaigns backed by MQTT command dispatch.
type Service struct {
	pool       *pgxpool.Pool
	dispatcher *appdevice.MQTTCommandDispatcher
	audit      *appaudit.Service
}

type Deps struct {
	Pool       *pgxpool.Pool
	Dispatcher *appdevice.MQTTCommandDispatcher
	Audit      *appaudit.Service
}

func NewService(d Deps) *Service {
	if d.Pool == nil || d.Dispatcher == nil {
		return nil
	}
	return &Service{pool: d.Pool, dispatcher: d.Dispatcher, audit: d.Audit}
}

// Strategy describes targeting encoded into rollout_campaigns.strategy JSON.
type Strategy struct {
	SiteIDs            []uuid.UUID `json:"site_ids,omitempty"`
	Statuses           []string    `json:"statuses,omitempty"`
	ModelSubstring     string      `json:"model,omitempty"`
	MachineIDs         []uuid.UUID `json:"machine_ids,omitempty"`
	CanaryPercent      *float64    `json:"canary_percent,omitempty"`
	ConfirmFullRollout bool        `json:"confirm_full_rollout,omitempty"`
	RollbackVersion    string      `json:"rollback_version,omitempty"`
	ResolvedMachineIDs []uuid.UUID `json:"resolved_machine_ids,omitempty"`
	ActivePhase        string      `json:"active_phase,omitempty"`
	// TagIDs and TagSlugs constrain targets to machines that have all listed tags (catalog tags via machine_tag_assignments).
	TagIDs   []uuid.UUID `json:"tag_ids,omitempty"`
	TagSlugs []string    `json:"tag_slugs,omitempty"`
}

func normalizeStrategy(raw []byte) (Strategy, error) {
	var st Strategy
	if len(raw) == 0 {
		return st, nil
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return Strategy{}, err
	}
	return st, nil
}

func marshalStrategy(st Strategy) ([]byte, error) {
	return json.Marshal(st)
}

// CreateCampaign inserts a pending rollout definition without dispatching targets yet.
func (s *Service) CreateCampaign(ctx context.Context, organizationID uuid.UUID, rolloutType, targetVersion string, strategyJSON []byte, createdBy pgtype.UUID) (db.RolloutCampaign, error) {
	if s == nil || s.pool == nil {
		return db.RolloutCampaign{}, ErrInvalidArgument
	}
	if organizationID == uuid.Nil || strings.TrimSpace(rolloutType) == "" || strings.TrimSpace(targetVersion) == "" {
		return db.RolloutCampaign{}, ErrInvalidArgument
	}
	if _, err := normalizeStrategy(strategyJSON); err != nil {
		return db.RolloutCampaign{}, ErrInvalidArgument
	}
	q := db.New(s.pool)
	row, err := q.InsertRolloutCampaign(ctx, db.InsertRolloutCampaignParams{
		OrganizationID: organizationID,
		RolloutType:    rolloutType,
		TargetVersion:  strings.TrimSpace(targetVersion),
		Status:         "pending",
		Strategy:       strategyJSON,
		CreatedBy:      createdBy,
	})
	if err != nil {
		return db.RolloutCampaign{}, err
	}
	s.recordAudit(ctx, organizationID, compliance.ActionFleetRolloutCreated, row.ID.String(), map[string]any{
		"rollout_type": rolloutType,
		"target":       targetVersion,
	})
	return row, nil
}

func (s *Service) ListCampaigns(ctx context.Context, organizationID uuid.UUID, limit, offset int32) ([]db.RolloutCampaign, error) {
	if s == nil || s.pool == nil {
		return nil, ErrInvalidArgument
	}
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	q := db.New(s.pool)
	return q.ListRolloutCampaigns(ctx, db.ListRolloutCampaignsParams{
		OrganizationID: organizationID,
		Limit:          limit,
		Offset:         offset,
	})
}

func (s *Service) GetCampaign(ctx context.Context, organizationID, campaignID uuid.UUID) (db.RolloutCampaign, []db.RolloutTarget, error) {
	if s == nil || s.pool == nil {
		return db.RolloutCampaign{}, nil, ErrInvalidArgument
	}
	q := db.New(s.pool)
	c, err := q.GetRolloutCampaignByID(ctx, db.GetRolloutCampaignByIDParams{
		ID:             campaignID,
		OrganizationID: organizationID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.RolloutCampaign{}, nil, ErrNotFound
		}
		return db.RolloutCampaign{}, nil, err
	}
	_ = q.RolloutRefreshTargetFromLatestAttempt(ctx, db.RolloutRefreshTargetFromLatestAttemptParams{
		CampaignID:     campaignID,
		OrganizationID: organizationID,
	})
	tgts, err := q.ListRolloutTargetsByCampaign(ctx, db.ListRolloutTargetsByCampaignParams{
		CampaignID:     campaignID,
		OrganizationID: organizationID,
	})
	if err != nil {
		return db.RolloutCampaign{}, nil, err
	}
	return c, tgts, nil
}

// Start resolves targets then enters running state and begins MQTT-backed dispatch rounds.
func (s *Service) Start(ctx context.Context, organizationID, campaignID uuid.UUID) error {
	if s == nil || s.pool == nil || s.dispatcher == nil {
		return ErrInvalidArgument
	}
	q := db.New(s.pool)
	c, err := q.GetRolloutCampaignByID(ctx, db.GetRolloutCampaignByIDParams{
		ID:             campaignID,
		OrganizationID: organizationID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if c.Status != "pending" && c.Status != "paused" {
		return ErrForbiddenState
	}

	st, err := normalizeStrategy(c.Strategy)
	if err != nil {
		return ErrInvalidArgument
	}
	resolved, err := s.resolveMachines(ctx, organizationID, st)
	if err != nil {
		return err
	}
	st.ResolvedMachineIDs = resolved
	st.ActivePhase = "forward"
	nextStrat, err := marshalStrategy(st)
	if err != nil {
		return err
	}
	if err := q.UpdateRolloutCampaignStrategy(ctx, db.UpdateRolloutCampaignStrategyParams{
		ID:             campaignID,
		OrganizationID: organizationID,
		Strategy:       nextStrat,
	}); err != nil {
		return err
	}

	for _, mid := range resolved {
		if _, ierr := q.InsertRolloutTargetRow(ctx, db.InsertRolloutTargetRowParams{
			OrganizationID: organizationID,
			CampaignID:     campaignID,
			MachineID:      mid,
		}); ierr != nil {
			return ierr
		}
	}

	if _, err := q.MarkRolloutCampaignStarted(ctx, db.MarkRolloutCampaignStartedParams{
		ID:             campaignID,
		OrganizationID: organizationID,
	}); err != nil {
		return err
	}
	if _, err := q.UpdateRolloutCampaignStatusOnly(ctx, db.UpdateRolloutCampaignStatusOnlyParams{
		ID:             campaignID,
		OrganizationID: organizationID,
		Status:         "running",
	}); err != nil {
		return err
	}

	s.recordAudit(ctx, organizationID, compliance.ActionFleetRolloutStarted, campaignID.String(), map[string]any{
		"machines": len(resolved),
	})

	return s.dispatchUntilQuiet(ctx, organizationID, campaignID)
}

func (s *Service) Pause(ctx context.Context, organizationID, campaignID uuid.UUID) error {
	return s.patchStatus(ctx, organizationID, campaignID, "paused", compliance.ActionFleetRolloutPaused)
}

func (s *Service) Resume(ctx context.Context, organizationID, campaignID uuid.UUID) error {
	if err := s.patchStatus(ctx, organizationID, campaignID, "running", compliance.ActionFleetRolloutResumed); err != nil {
		return err
	}
	return s.dispatchUntilQuiet(ctx, organizationID, campaignID)
}

func (s *Service) Cancel(ctx context.Context, organizationID, campaignID uuid.UUID) error {
	q := db.New(s.pool)
	if _, err := q.MarkRolloutCampaignCancelled(ctx, db.MarkRolloutCampaignCancelledParams{
		ID:             campaignID,
		OrganizationID: organizationID,
	}); err != nil {
		return err
	}
	if err := q.RolloutSkipPendingTargets(ctx, db.RolloutSkipPendingTargetsParams{
		CampaignID:     campaignID,
		OrganizationID: organizationID,
	}); err != nil {
		return err
	}
	s.recordAudit(ctx, organizationID, compliance.ActionFleetRolloutCancelled, campaignID.String(), nil)
	return nil
}

func (s *Service) Rollback(ctx context.Context, organizationID, campaignID uuid.UUID) error {
	if s == nil || s.pool == nil || s.dispatcher == nil {
		return ErrInvalidArgument
	}
	q := db.New(s.pool)
	c, err := q.GetRolloutCampaignByID(ctx, db.GetRolloutCampaignByIDParams{
		ID:             campaignID,
		OrganizationID: organizationID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	st, err := normalizeStrategy(c.Strategy)
	if err != nil {
		return err
	}
	if strings.TrimSpace(st.RollbackVersion) == "" {
		return ErrInvalidArgument
	}
	st.ActivePhase = "rollback"
	nextStrat, err := marshalStrategy(st)
	if err != nil {
		return err
	}
	if err := q.UpdateRolloutCampaignStrategy(ctx, db.UpdateRolloutCampaignStrategyParams{
		ID:             campaignID,
		OrganizationID: organizationID,
		Strategy:       nextStrat,
	}); err != nil {
		return err
	}
	if err := q.RolloutPrepareRollbackWave(ctx, db.RolloutPrepareRollbackWaveParams{
		CampaignID:     campaignID,
		OrganizationID: organizationID,
	}); err != nil {
		return err
	}
	if _, err := q.MarkRolloutCampaignStarted(ctx, db.MarkRolloutCampaignStartedParams{
		ID:             campaignID,
		OrganizationID: organizationID,
	}); err != nil {
		return err
	}
	if _, err := q.UpdateRolloutCampaignStatusOnly(ctx, db.UpdateRolloutCampaignStatusOnlyParams{
		ID:             campaignID,
		OrganizationID: organizationID,
		Status:         "running",
	}); err != nil {
		return err
	}
	s.recordAudit(ctx, organizationID, compliance.ActionFleetRolloutRollbackDispatched, campaignID.String(), map[string]any{
		"rollback_version": st.RollbackVersion,
	})
	if err := s.dispatchUntilQuiet(ctx, organizationID, campaignID); err != nil {
		return err
	}
	if _, err := q.MarkRolloutCampaignRolledBack(ctx, db.MarkRolloutCampaignRolledBackParams{
		ID:             campaignID,
		OrganizationID: organizationID,
	}); err != nil {
		return err
	}
	return nil
}

func (s *Service) patchStatus(ctx context.Context, organizationID, campaignID uuid.UUID, status string, action string) error {
	q := db.New(s.pool)
	if _, err := q.UpdateRolloutCampaignStatusOnly(ctx, db.UpdateRolloutCampaignStatusOnlyParams{
		ID:             campaignID,
		OrganizationID: organizationID,
		Status:         status,
	}); err != nil {
		return err
	}
	s.recordAudit(ctx, organizationID, action, campaignID.String(), map[string]any{"status": status})
	return nil
}

func (s *Service) dispatchUntilQuiet(ctx context.Context, organizationID, campaignID uuid.UUID) error {
	q := db.New(s.pool)
	for iter := 0; iter < maxDispatchLoops; iter++ {
		camp, err := q.GetRolloutCampaignByID(ctx, db.GetRolloutCampaignByIDParams{
			ID:             campaignID,
			OrganizationID: organizationID,
		})
		if err != nil {
			return err
		}
		if rolloutCampaignDispatchShouldStop(camp.Status) {
			return nil
		}

		targets, err := q.ListRolloutPendingTargets(ctx, db.ListRolloutPendingTargetsParams{
			CampaignID:     campaignID,
			OrganizationID: organizationID,
			Limit:          dispatchChunk,
		})
		if err != nil {
			return err
		}

		st, err := normalizeStrategy(camp.Strategy)
		if err != nil {
			return err
		}
		ver := camp.TargetVersion
		if strings.EqualFold(strings.TrimSpace(st.ActivePhase), "rollback") {
			ver = strings.TrimSpace(st.RollbackVersion)
		}

		for _, t := range targets {
			if err := s.dispatchOne(ctx, organizationID, camp, st, t, ver); err != nil {
				return err
			}
		}

		_ = q.RolloutRefreshTargetFromLatestAttempt(ctx, db.RolloutRefreshTargetFromLatestAttemptParams{
			CampaignID:     campaignID,
			OrganizationID: organizationID,
		})

		pending, err := q.CountRolloutTargetsByStatus(ctx, db.CountRolloutTargetsByStatusParams{
			CampaignID:     campaignID,
			OrganizationID: organizationID,
			Status:         "pending",
		})
		if err != nil {
			return err
		}
		dispatched, err := q.CountRolloutTargetsByStatus(ctx, db.CountRolloutTargetsByStatusParams{
			CampaignID:     campaignID,
			OrganizationID: organizationID,
			Status:         "dispatched",
		})
		if err != nil {
			return err
		}

		if pending == 0 && len(targets) == 0 && dispatched == 0 {
			_, err := q.MarkRolloutCampaignCompleted(ctx, db.MarkRolloutCampaignCompletedParams{
				ID:             campaignID,
				OrganizationID: organizationID,
			})
			return err
		}
		if pending == 0 && len(targets) == 0 && dispatched > 0 {
			continue
		}
		if pending == 0 && len(targets) == 0 {
			_, err := q.MarkRolloutCampaignCompleted(ctx, db.MarkRolloutCampaignCompletedParams{
				ID:             campaignID,
				OrganizationID: organizationID,
			})
			return err
		}
	}
	return ErrInvalidArgument
}

func (s *Service) dispatchOne(ctx context.Context, organizationID uuid.UUID, camp db.RolloutCampaign, st Strategy, tgt db.RolloutTarget, version string) error {
	q := db.New(s.pool)
	if s.dispatcher == nil {
		return ErrInvalidArgument
	}
	m, err := q.GetMachineShadowByMachineID(ctx, tgt.MachineID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			_, err := q.UpdateRolloutTargetDispatch(ctx, db.UpdateRolloutTargetDispatchParams{
				ID:             tgt.ID,
				OrganizationID: organizationID,
				CampaignID:     camp.ID,
				Status:         "failed",
				CommandID:      pgtype.UUID{},
				ErrMessage:     pgtype.Text{String: "shadow_missing", Valid: true},
			})
			return err
		}
		return err
	}

	desired, err := mergeRolloutDesired(m.DesiredState, camp.RolloutType, version)
	if err != nil {
		return err
	}
	corr := tgt.ID
	payload := map[string]any{
		"rollout_campaign_id": camp.ID.String(),
		"rollout_target_id":   tgt.ID.String(),
		"rollout_type":        camp.RolloutType,
		"target_version":      version,
		"phase":               st.ActivePhase,
	}
	pb, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	idem := fmt.Sprintf("fleet_rollout:%s:%s:%s", camp.ID, tgt.ID, strings.TrimSpace(st.ActivePhase))

	res, err := s.dispatcher.DispatchRemoteMQTTCommand(ctx, appdevice.RemoteCommandDispatchInput{
		Append: domaindevice.AppendCommandInput{
			MachineID:      tgt.MachineID,
			CommandType:    commandFleetRolloutApply,
			Payload:        pb,
			CorrelationID:  &corr,
			IdempotencyKey: idem,
			DesiredState:   desired,
		},
	})
	if err != nil {
		_, uerr := q.UpdateRolloutTargetDispatch(ctx, db.UpdateRolloutTargetDispatchParams{
			ID:             tgt.ID,
			OrganizationID: organizationID,
			CampaignID:     camp.ID,
			Status:         "failed",
			CommandID:      pgtype.UUID{},
			ErrMessage:     pgtype.Text{String: truncateErr(err), Valid: true},
		})
		return errors.Join(err, uerr)
	}

	cmdID := pgtype.UUID{Bytes: res.CommandID, Valid: true}
	_, err = q.UpdateRolloutTargetDispatch(ctx, db.UpdateRolloutTargetDispatchParams{
		ID:             tgt.ID,
		OrganizationID: organizationID,
		CampaignID:     camp.ID,
		Status:         "dispatched",
		CommandID:      cmdID,
		ErrMessage:     pgtype.Text{},
	})
	return err
}

func truncateErr(err error) string {
	es := err.Error()
	if len(es) > 512 {
		return es[:512]
	}
	return es
}

func mergeRolloutDesired(desired []byte, rolloutType, version string) ([]byte, error) {
	base := desired
	if len(base) == 0 {
		base = []byte("{}")
	}
	var m map[string]any
	if err := json.Unmarshal(base, &m); err != nil {
		return nil, err
	}
	key := map[string]string{
		"config_version":    "config_version",
		"catalog_version":   "catalog_version",
		"media_version":     "media_version",
		"planogram_version": "planogram_version",
		"app_version":       "app_version",
	}[rolloutType]
	if key == "" {
		return nil, ErrInvalidArgument
	}
	m[key] = version
	return json.Marshal(m)
}

func (s *Service) resolveMachines(ctx context.Context, organizationID uuid.UUID, st Strategy) ([]uuid.UUID, error) {
	q := db.New(s.pool)
	if len(st.MachineIDs) > 0 {
		out := make([]uuid.UUID, 0, len(st.MachineIDs))
		for _, id := range st.MachineIDs {
			m, err := q.GetMachineByID(ctx, id)
			if err != nil {
				return nil, err
			}
			if m.OrganizationID != organizationID {
				return nil, ErrInvalidArgument
			}
			out = append(out, id)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
		return out, nil
	}

	rows, err := q.RolloutListOrgMachines(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var cand []uuid.UUID
	for _, r := range rows {
		if len(st.SiteIDs) > 0 && !containsUUID(st.SiteIDs, r.SiteID) {
			continue
		}
		if len(st.Statuses) > 0 && !containsStringFold(st.Statuses, r.Status) {
			continue
		}
		if ms := strings.TrimSpace(st.ModelSubstring); ms != "" {
			mv := ""
			if r.Model.Valid {
				mv = r.Model.String
			}
			if !strings.Contains(strings.ToLower(mv), strings.ToLower(ms)) {
				continue
			}
		}
		cand = append(cand, r.ID)
	}
	sort.Slice(cand, func(i, j int) bool { return cand[i].String() < cand[j].String() })

	tagIDs, err := s.mergeAndValidateRolloutTagFilters(ctx, q, organizationID, st)
	if err != nil {
		return nil, err
	}
	if len(tagIDs) > 0 {
		machineWithTags, err := q.RolloutListMachineIDsWithAllTags(ctx, db.RolloutListMachineIDsWithAllTagsParams{
			OrganizationID: organizationID,
			TagIds:         tagIDs,
			RequiredCount:  int32(len(tagIDs)),
		})
		if err != nil {
			return nil, err
		}
		allowed := make(map[uuid.UUID]struct{}, len(machineWithTags))
		for _, mid := range machineWithTags {
			allowed[mid] = struct{}{}
		}
		filtered := cand[:0]
		for _, id := range cand {
			if _, ok := allowed[id]; ok {
				filtered = append(filtered, id)
			}
		}
		cand = filtered
	}

	return applyRolloutSelection(cand, st)
}

func applyRolloutSelection(cand []uuid.UUID, st Strategy) ([]uuid.UUID, error) {
	n := len(cand)
	if n == 0 {
		return nil, ErrInvalidArgument
	}
	if st.CanaryPercent != nil {
		p := *st.CanaryPercent
		if p >= 100 || p <= 0 {
			return nil, ErrInvalidArgument
		}
		k := computeCanaryCount(n, p)
		return cand[:k], nil
	}
	if !st.ConfirmFullRollout {
		return nil, ErrInvalidArgument
	}
	return cand, nil
}

func (s *Service) mergeAndValidateRolloutTagFilters(ctx context.Context, q *db.Queries, organizationID uuid.UUID, st Strategy) ([]uuid.UUID, error) {
	slugSeen := make(map[string]struct{})
	var slugList []string
	for _, raw := range st.TagSlugs {
		ls := strings.ToLower(strings.TrimSpace(raw))
		if ls == "" {
			continue
		}
		if _, ok := slugSeen[ls]; ok {
			continue
		}
		slugSeen[ls] = struct{}{}
		slugList = append(slugList, ls)
	}
	idSet := make(map[uuid.UUID]struct{})
	if len(slugList) > 0 {
		rows, err := q.RolloutResolveTagIDsBySlugs(ctx, db.RolloutResolveTagIDsBySlugsParams{
			OrganizationID: organizationID,
			Slugs:          slugList,
		})
		if err != nil {
			return nil, err
		}
		bySlug := make(map[string]uuid.UUID, len(rows))
		for _, r := range rows {
			bySlug[strings.ToLower(strings.TrimSpace(r.Slug))] = r.ID
		}
		for _, ls := range slugList {
			id, ok := bySlug[ls]
			if !ok {
				return nil, ErrInvalidArgument
			}
			idSet[id] = struct{}{}
		}
	}
	for _, id := range st.TagIDs {
		if id == uuid.Nil {
			continue
		}
		idSet[id] = struct{}{}
	}
	if len(idSet) == 0 {
		return nil, nil
	}
	merged := make([]uuid.UUID, 0, len(idSet))
	for id := range idSet {
		merged = append(merged, id)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].String() < merged[j].String() })
	matched, err := q.RolloutMatchTagIDsForOrg(ctx, db.RolloutMatchTagIDsForOrgParams{
		OrganizationID: organizationID,
		TagIds:         merged,
	})
	if err != nil {
		return nil, err
	}
	if len(matched) != len(merged) {
		return nil, ErrInvalidArgument
	}
	return merged, nil
}

func containsUUID(xs []uuid.UUID, v uuid.UUID) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func containsStringFold(xs []string, v string) bool {
	vv := strings.ToLower(strings.TrimSpace(v))
	for _, x := range xs {
		if strings.ToLower(strings.TrimSpace(x)) == vv {
			return true
		}
	}
	return false
}

func (s *Service) recordAudit(ctx context.Context, org uuid.UUID, action string, resourceID string, meta map[string]any) {
	if s == nil || s.audit == nil {
		return
	}
	md, _ := json.Marshal(meta)
	if len(md) == 0 {
		md = []byte("{}")
	}
	rid := resourceID
	at, aid := compliance.ActorUser, ""
	if p, ok := auth.PrincipalFromContext(ctx); ok {
		at, aid = p.Actor()
	}
	var actorPtr *string
	if strings.TrimSpace(aid) != "" {
		actorPtr = &aid
	}
	_ = s.audit.RecordCritical(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: org,
		ActorType:      at,
		ActorID:        actorPtr,
		Action:         action,
		ResourceType:   "rollout_campaigns",
		ResourceID:     &rid,
		Metadata:       md,
	})
}

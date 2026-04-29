package otaadmin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	domaindevice "github.com/avf/avf-vending-api/internal/domain/device"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// PauseCampaign freezes rollout (no further commands until resume).
func (s *Service) PauseCampaign(ctx context.Context, orgID, campaignID uuid.UUID) (CampaignDetail, error) {
	cur, err := s.getCampaign(ctx, orgID, campaignID)
	if err != nil {
		return CampaignDetail{}, err
	}
	if cur.Status != statusRunning {
		return CampaignDetail{}, ErrRolloutNotActive
	}
	now := time.Now().UTC()
	row, err := s.q.OtaAdminUpdateCampaignStatusFields(ctx, db.OtaAdminUpdateCampaignStatusFieldsParams{
		OrganizationID:    orgID,
		ID:                campaignID,
		Status:            statusPaused,
		ApprovedBy:        cur.ApprovedBy,
		ApprovedAt:        cur.ApprovedAt,
		RolloutNextOffset: cur.RolloutNextOffset,
		PausedAt:          pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return CampaignDetail{}, err
	}
	s.emitEvent(ctx, s.q, orgID, campaignID, "campaign_paused", map[string]any{}, pgtype.UUID{})
	return s.campaignDetail(ctx, orgID, row)
}

// ResumeCampaign continues the next deterministic wave after canary (or completes if nothing remains).
func (s *Service) ResumeCampaign(ctx context.Context, orgID, campaignID uuid.UUID) (CampaignDetail, error) {
	cur, err := s.getCampaign(ctx, orgID, campaignID)
	if err != nil {
		return CampaignDetail{}, err
	}
	if cur.Status != statusPaused {
		return CampaignDetail{}, ErrInvalidTransition
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
	off := int(cur.RolloutNextOffset)
	if off >= n {
		return CampaignDetail{}, ErrNothingLeftToRollout
	}
	primary, err := s.q.OtaAdminGetArtifactForOrg(ctx, db.OtaAdminGetArtifactForOrgParams{
		OrganizationID: orgID,
		ID:             cur.ArtifactID,
	})
	if err != nil {
		return CampaignDetail{}, err
	}
	wave := mids[off:n]
	if err := s.dispatchOTACommands(ctx, orgID, cur, primary, wave, waveForward, "full"); err != nil {
		return CampaignDetail{}, err
	}
	row, err := s.q.OtaAdminUpdateCampaignStatusFields(ctx, db.OtaAdminUpdateCampaignStatusFieldsParams{
		OrganizationID:    orgID,
		ID:                campaignID,
		Status:            statusCompleted,
		ApprovedBy:        cur.ApprovedBy,
		ApprovedAt:        cur.ApprovedAt,
		RolloutNextOffset: int32(n),
		PausedAt:          pgtype.Timestamptz{},
	})
	if err != nil {
		return CampaignDetail{}, err
	}
	s.emitEvent(ctx, s.q, orgID, campaignID, "campaign_resumed", map[string]any{"wave_size": len(wave)}, pgtype.UUID{})
	return s.campaignDetail(ctx, orgID, row)
}

// CancelCampaign stops the campaign without deploying further waves.
func (s *Service) CancelCampaign(ctx context.Context, orgID, campaignID uuid.UUID) (CampaignDetail, error) {
	cur, err := s.getCampaign(ctx, orgID, campaignID)
	if err != nil {
		return CampaignDetail{}, err
	}
	switch cur.Status {
	case statusDraft, statusApproved, statusRunning, statusPaused:
	default:
		return CampaignDetail{}, ErrInvalidTransition
	}
	row, err := s.q.OtaAdminUpdateCampaignStatusFields(ctx, db.OtaAdminUpdateCampaignStatusFieldsParams{
		OrganizationID:    orgID,
		ID:                campaignID,
		Status:            statusCancelled,
		ApprovedBy:        cur.ApprovedBy,
		ApprovedAt:        cur.ApprovedAt,
		RolloutNextOffset: cur.RolloutNextOffset,
		PausedAt:          cur.PausedAt,
	})
	if err != nil {
		return CampaignDetail{}, err
	}
	s.emitEvent(ctx, s.q, orgID, campaignID, "campaign_cancelled", map[string]any{}, pgtype.UUID{})
	return s.campaignDetail(ctx, orgID, row)
}

// RollbackCampaign issues rollback commands using rollback_artifact_id (campaign field or override).
func (s *Service) RollbackCampaign(ctx context.Context, orgID, campaignID uuid.UUID, rollbackArtifactID *uuid.UUID) (CampaignDetail, error) {
	cur, err := s.getCampaign(ctx, orgID, campaignID)
	if err != nil {
		return CampaignDetail{}, err
	}
	switch cur.Status {
	case statusRunning, statusPaused, statusCompleted:
	default:
		return CampaignDetail{}, ErrInvalidTransition
	}
	rid := uuid.Nil
	if rollbackArtifactID != nil && *rollbackArtifactID != uuid.Nil {
		rid = *rollbackArtifactID
	} else if cur.RollbackArtifactID.Valid {
		rid = uuid.UUID(cur.RollbackArtifactID.Bytes)
	}
	if rid == uuid.Nil {
		return CampaignDetail{}, ErrRollbackArtifact
	}
	rbArt, err := s.q.OtaAdminGetArtifactForOrg(ctx, db.OtaAdminGetArtifactForOrgParams{
		OrganizationID: orgID,
		ID:             rid,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CampaignDetail{}, ErrNotFound
		}
		return CampaignDetail{}, err
	}
	targets, err := s.q.OtaAdminListCampaignTargetsSorted(ctx, campaignID)
	if err != nil {
		return CampaignDetail{}, err
	}
	mids := make([]uuid.UUID, 0, len(targets))
	for _, t := range targets {
		mids = append(mids, t.MachineID)
	}
	if len(mids) == 0 {
		return CampaignDetail{}, ErrNoTargets
	}
	if err := s.dispatchRollbackCommands(ctx, orgID, cur, rbArt, mids); err != nil {
		return CampaignDetail{}, err
	}
	row, err := s.q.OtaAdminUpdateCampaignStatusFields(ctx, db.OtaAdminUpdateCampaignStatusFieldsParams{
		OrganizationID:    orgID,
		ID:                campaignID,
		Status:            statusRolledBack,
		ApprovedBy:        cur.ApprovedBy,
		ApprovedAt:        cur.ApprovedAt,
		RolloutNextOffset: cur.RolloutNextOffset,
		PausedAt:          pgtype.Timestamptz{},
	})
	if err != nil {
		return CampaignDetail{}, err
	}
	s.emitEvent(ctx, s.q, orgID, campaignID, "campaign_rollback", map[string]any{"rollback_artifact_id": rid.String()}, pgtype.UUID{})
	return s.campaignDetail(ctx, orgID, row)
}

func (s *Service) dispatchRollbackCommands(ctx context.Context, orgID uuid.UUID, camp db.OtaCampaign, art db.OtaArtifact, machines []uuid.UUID) error {
	semver := ""
	if art.Semver.Valid {
		semver = art.Semver.String
	}
	for i, mid := range machines {
		payload, err := json.Marshal(map[string]any{
			"campaign_id":   camp.ID.String(),
			"artifact_id":   art.ID.String(),
			"storage_key":   art.StorageKey,
			"semver":        semver,
			"campaign_type": camp.CampaignType,
			"rollback":      true,
			"wave_index":    i,
		})
		if err != nil {
			return err
		}
		idem := fmt.Sprintf("ota-rollback:%s:%s:%d", camp.ID.String(), mid.String(), i)
		res, err := s.wf.AppendCommandUpdateShadow(ctx, domaindevice.AppendCommandInput{
			MachineID:      mid,
			CommandType:    cmdOTARollback,
			Payload:        payload,
			IdempotencyKey: idem,
			DesiredState:   []byte(`{}`),
		})
		if err != nil {
			return err
		}
		_, err = s.q.OtaAdminUpsertMachineResult(ctx, db.OtaAdminUpsertMachineResultParams{
			OrganizationID: orgID,
			CampaignID:     camp.ID,
			MachineID:      mid,
			Wave:           waveRollback,
			CommandID:      uuidPg(res.CommandID),
			Status:         resDispatched,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

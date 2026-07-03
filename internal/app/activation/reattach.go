package activation

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/machineruntime"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	// ErrReattachDenied is returned when reattach policy blocks recovery.
	ErrReattachDenied = errors.New("activation: reattach denied")
)

// ClaimContext optional accountability fields for activation/reattach.
type ClaimContext struct {
	ActivatedByAccountID *uuid.UUID
	OperatorSessionID    *uuid.UUID
	RequestID            string
	CorrelationID        *uuid.UUID
	AppVersion           string
	BootID               string
	DeviceSerial         string
	Reason               string
	ActivationSource     string
}

// ReattachInput is an admin/technician device reattach after reinstall.
type ReattachInput struct {
	MachineID            uuid.UUID
	DeviceFingerprint    DeviceFingerprint
	RawDeviceFingerprint json.RawMessage
	ClaimContext
	AdminReattach bool
	TechnicianID  *uuid.UUID
	ClientIP      string
	UserAgent     string
}

// ReattachResult includes fresh machine credentials.
type ReattachResult struct {
	ClaimResult
	Reattached        bool
	SessionID         uuid.UUID
	OperatorSessionID *uuid.UUID
	CorrelationID     *uuid.UUID
}

// ReattachDevice issues new machine credentials after authorized reinstall recovery.
func (s *Service) ReattachDevice(ctx context.Context, in ReattachInput, mqttBrokerURL, mqttTopicPrefix, mqttTopicLayout string) (ReattachResult, error) {
	if in.MachineID == uuid.Nil {
		return ReattachResult{}, fmt.Errorf("activation: machine required")
	}
	fpJSON := in.RawDeviceFingerprint
	if len(fpJSON) == 0 {
		var err error
		fpJSON, err = json.Marshal(in.DeviceFingerprint)
		if err != nil {
			return ReattachResult{}, err
		}
	}
	fpHash := sha256.Sum256(fpJSON)
	st := strings.TrimSpace(in.ActivationSource)
	if st == "" {
		if in.AdminReattach {
			st = "admin_reattach"
		} else {
			st = "technician_reattach"
		}
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ReattachResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := db.New(tx)

	m, err := qtx.GetMachineByID(ctx, in.MachineID)
	if err != nil {
		return ReattachResult{}, err
	}
	status := strings.ToLower(strings.TrimSpace(m.Status))
	switch status {
	case "compromised", "retired", "decommissioned":
		return ReattachResult{}, ErrReattachDenied
	}
	if !machineEligibleForClaim(m) && status != "suspended" {
		return ReattachResult{}, ErrReattachDenied
	}

	cred, err := s.ensureActiveMachineCredential(ctx, qtx, uuid.Nil, in.MachineID, m.CredentialVersion)
	if err != nil {
		return ReattachResult{}, err
	}
	plainRefresh, refreshExp, sessionID, err := s.provisionMachineRefreshSession(ctx, qtx, in.MachineID, uuid.Nil, m, cred)
	if err != nil {
		return ReattachResult{}, err
	}
	tok, exp, err := s.issuer.IssueMachineAccessJWT(in.MachineID, m.SiteID, m.CredentialVersion, sessionID)
	if err != nil {
		return ReattachResult{}, err
	}

	claimParams := db.InsertMachineActivationClaimExtendedParams{
		MachineID:        in.MachineID,
		FingerprintHash:  fpHash[:],
		IpAddress:        strings.TrimSpace(in.ClientIP),
		UserAgent:        strings.TrimSpace(in.UserAgent),
		Result:           "succeeded",
		FailureReason:    "",
		AppVersion:       pgtype.Text{String: strings.TrimSpace(in.AppVersion), Valid: strings.TrimSpace(in.AppVersion) != ""},
		BootID:           pgtype.Text{String: strings.TrimSpace(in.BootID), Valid: strings.TrimSpace(in.BootID) != ""},
		DeviceSerial:     pgtype.Text{String: strings.TrimSpace(in.DeviceSerial), Valid: strings.TrimSpace(in.DeviceSerial) != ""},
		Reason:           pgtype.Text{String: strings.TrimSpace(in.Reason), Valid: strings.TrimSpace(in.Reason) != ""},
		ActivationSource: pgtype.Text{String: st, Valid: true},
		RequestID:        pgtype.Text{String: strings.TrimSpace(in.RequestID), Valid: strings.TrimSpace(in.RequestID) != ""},
	}
	if in.ActivatedByAccountID != nil {
		claimParams.ActivatedByAccountID = pgtype.UUID{Bytes: *in.ActivatedByAccountID, Valid: true}
	}
	if in.OperatorSessionID != nil {
		claimParams.OperatorSessionID = pgtype.UUID{Bytes: *in.OperatorSessionID, Valid: true}
	}
	if in.CorrelationID != nil {
		claimParams.CorrelationID = pgtype.UUID{Bytes: *in.CorrelationID, Valid: true}
	}
	if _, err := qtx.InsertMachineActivationClaimExtended(ctx, claimParams); err != nil {
		return ReattachResult{}, err
	}

	if s.runtime != nil {
		attachReason := "technician_reattach"
		if in.AdminReattach {
			attachReason = "admin_reattach"
		}
		idMeta := machineruntime.DeviceIdentityFromFingerprint(fpJSON, in.ClientIP, in.UserAgent, nil)
		idMeta.BootID = strings.TrimSpace(in.BootID)
		if idMeta.AppBuildSHA == "" {
			idMeta.AppBuildSHA = strings.TrimSpace(in.AppVersion)
		}
		_, err := s.runtime.AttachOrReplaceDeviceInTx(ctx, qtx, machineruntime.AttachInput{
			MachineID:           in.MachineID,
			Reason:              attachReason,
			AttachedByAccountID: in.ActivatedByAccountID,
			OperatorSessionID:   in.OperatorSessionID,
			CorrelationID:       in.CorrelationID,
			Identity:            idMeta,
			RequireOperator:     !in.AdminReattach,
			TechnicianID:        in.TechnicianID,
		})
		if err != nil {
			return ReattachResult{}, err
		}
	}

	if s.audit != nil {
		mid := in.MachineID.String()
		meta, _ := json.Marshal(map[string]any{
			"activation_source": st,
			"session_id":        sessionID.String(),
			"reason":            in.Reason,
		})
		_ = s.audit.RecordCriticalTx(ctx, tx, compliance.EnterpriseAuditRecord{
			ActorType:    compliance.ActorUser,
			ActorID:      stringPtr(in.ActivatedByAccountID),
			Action:       compliance.ActionMachineActivationClaimed,
			ResourceType: "machine",
			ResourceID:   &mid,
			MachineID:    &in.MachineID,
			Metadata:     meta,
		})
	}
	mqttUser, mqttPass, err := s.provisionMachineMQTT(ctx, qtx, in.MachineID)
	if err != nil {
		return ReattachResult{}, err
	}
	if s.emqx != nil && (mqttUser == "" || mqttPass == "") {
		return ReattachResult{}, ErrMQTTProvisioning
	}
	if s.audit != nil && mqttUser != "" {
		mid := in.MachineID.String()
		meta, _ := json.Marshal(map[string]any{
			"mqtt_username": mqttUser,
			"action":        "mqtt_credential_rotated",
		})
		_ = s.audit.RecordCriticalTx(ctx, tx, compliance.EnterpriseAuditRecord{
			ActorType:    compliance.ActorUser,
			ActorID:      stringPtr(in.ActivatedByAccountID),
			Action:       compliance.ActionMachineActivationClaimed,
			ResourceType: "machine_mqtt_credentials",
			ResourceID:   &mid,
			MachineID:    &in.MachineID,
			Metadata:     meta,
		})
	}
	if err := tx.Commit(ctx); err != nil {
		if s.emqx != nil && mqttUser != "" {
			_ = s.emqx.DeleteUser(context.WithoutCancel(ctx), mqttUser)
		}
		return ReattachResult{}, err
	}
	cr := ClaimResult{
		MachineID:         in.MachineID,
		SiteID:            m.SiteID,
		MachineName:       m.Name,
		MachineToken:      tok,
		TokenExpiresAt:    exp.UTC(),
		RefreshToken:      plainRefresh,
		RefreshExpiresAt:  refreshExp,
		MQTTBrokerURL:     mqttBrokerURL,
		MQTTTopicPrefix:   mqttTopicPrefix,
		MQTTTopicLayout:   mqttTopicLayout,
		MQTTUsername:      mqttUser,
		MQTTPassword:      mqttPass,
		BootstrapPath:     fmt.Sprintf("/v1/setup/machines/%s/bootstrap", in.MachineID),
		BootstrapRequired: true,
	}
	return ReattachResult{
		ClaimResult:       cr,
		Reattached:        true,
		SessionID:         sessionID,
		OperatorSessionID: in.OperatorSessionID,
		CorrelationID:     in.CorrelationID,
	}, nil
}

func stringPtr(id *uuid.UUID) *string {
	if id == nil || *id == uuid.Nil {
		return nil
	}
	s := id.String()
	return &s
}

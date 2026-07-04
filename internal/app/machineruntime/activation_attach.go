package machineruntime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ActivationAttachInput binds a device fingerprint during activation-code claim.
type ActivationAttachInput struct {
	MachineID         uuid.UUID
	FingerprintJSON   json.RawMessage
	ClientIP          string
	UserAgent         string
	Reason            string
	ActivationSource  string
}

// DeviceIdentityMatchesAttachment returns true when the active attachment matches the submitted identity.
// Matching uses android_id, board_serial, sim_iccid, android_serial, and device_serial when present.
// When all discriminating fields are empty (incomplete fingerprint), attachments are reused on replay.
func DeviceIdentityMatchesAttachment(att db.MachineDeviceAttachment, id DeviceIdentity) bool {
	checks := []struct {
		attVal string
		idVal  string
	}{
		{pgTextVal(att.AndroidID), strings.TrimSpace(id.AndroidID)},
		{pgTextVal(att.BoardSerial), strings.TrimSpace(id.BoardSerial)},
		{pgTextVal(att.SimIccid), strings.TrimSpace(id.SimICCID)},
		{pgTextVal(att.AndroidSerial), strings.TrimSpace(id.AndroidSerial)},
		{pgTextVal(att.DeviceSerial), strings.TrimSpace(id.DeviceSerial)},
	}
	compared := false
	for _, c := range checks {
		if c.attVal == "" && c.idVal == "" {
			continue
		}
		compared = true
		if c.attVal != c.idVal {
			return false
		}
	}
	if !compared {
		aid := strings.TrimSpace(id.AndroidID)
		if aid != "" && pgTextVal(att.AndroidID) != "" {
			return pgTextVal(att.AndroidID) == aid
		}
		return true
	}
	return true
}

// EnsureActivationDeviceAttachmentInTx creates, reuses, or replaces a device attachment for activation-code claim.
// Idempotent replay with the same fingerprint reuses the active attachment without closing runtime sessions.
// A different fingerprint replaces the active attachment and closes the current runtime app session as BOARD_REPLACED.
func (s *Service) EnsureActivationDeviceAttachmentInTx(ctx context.Context, qtx *db.Queries, in ActivationAttachInput) (db.MachineDeviceAttachment, error) {
	if s == nil {
		return db.MachineDeviceAttachment{}, errors.New("machineruntime: nil service")
	}
	if in.MachineID == uuid.Nil {
		return db.MachineDeviceAttachment{}, errors.New("machineruntime: machine required")
	}
	meta := json.RawMessage("{}")
	if src := strings.TrimSpace(in.ActivationSource); src != "" {
		b, _ := json.Marshal(map[string]string{"activation_source": src})
		meta = b
	}
	identity := DeviceIdentityFromFingerprint(in.FingerprintJSON, in.ClientIP, in.UserAgent, meta)
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		reason = "first_install"
	}

	cur, err := qtx.GetActiveMachineDeviceAttachment(ctx, in.MachineID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return s.attachOrReplaceDeviceTx(ctx, qtx, AttachInput{
				MachineID:       in.MachineID,
				Reason:          reason,
				Identity:        identity,
				RequireOperator: false,
			})
		}
		return db.MachineDeviceAttachment{}, err
	}
	if DeviceIdentityMatchesAttachment(cur, identity) {
		return cur, nil
	}
	return s.attachOrReplaceDeviceTx(ctx, qtx, AttachInput{
		MachineID:       in.MachineID,
		Reason:          reason,
		Identity:        identity,
		RequireOperator: false,
	})
}

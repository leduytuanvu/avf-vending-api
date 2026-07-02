package fleet

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/emqxadmin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SetEMQXProvisioner wires broker lifecycle hooks for credential revoke/rotate.
func (s *Service) SetEMQXProvisioner(c *emqxadmin.Client, pool *pgxpool.Pool) {
	if s == nil {
		return
	}
	s.emqx = c
	s.dbPool = pool
}

func (s *Service) emqxConfigured() bool {
	return s != nil && s.emqx != nil && s.dbPool != nil
}

func (s *Service) revokeMachineMQTT(ctx context.Context, machineID uuid.UUID) error {
	if !s.emqxConfigured() || machineID == uuid.Nil {
		return nil
	}
	if err := s.emqx.DeleteUser(ctx, machineID.String()); err != nil {
		return fmt.Errorf("fleet: revoke mqtt user: %w", err)
	}
	return db.New(s.dbPool).RevokeMachineMQTTCredentials(ctx, machineID)
}

func (s *Service) rotateMachineMQTT(ctx context.Context, machineID uuid.UUID) error {
	if !s.emqxConfigured() || machineID == uuid.Nil {
		return nil
	}
	password, err := randomMQTTPassword()
	if err != nil {
		return err
	}
	username := machineID.String()
	if err := s.emqx.UpsertUser(ctx, username, password); err != nil {
		return fmt.Errorf("fleet: rotate mqtt user: %w", err)
	}
	return db.New(s.dbPool).UpsertMachineMQTTCredentials(ctx, db.UpsertMachineMQTTCredentialsParams{
		MachineID:       machineID,
		MqttBrokerShard: "default",
		Username:        pgtype.Text{String: username, Valid: true},
		SecretRef:       pgtype.Text{String: fmt.Sprintf("emqx:v1:%d", time.Now().UTC().UnixNano()), Valid: true},
	})
}

func randomMQTTPassword() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", buf), nil
}

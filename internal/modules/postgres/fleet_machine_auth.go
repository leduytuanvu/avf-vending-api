package postgres

import (
	"context"
	"errors"

	appfleet "github.com/avf/avf-vending-api/internal/app/fleet"
	domainfleet "github.com/avf/avf-vending-api/internal/domain/fleet"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// RotateMachineCredentialLifecycle bumps the machine credential version, marks prior credential rows rotated,
// inserts the new active credential row, revokes runtime sessions, and revokes outstanding activation codes.
func (r *fleetRepository) RotateMachineCredentialLifecycle(ctx context.Context, companyID, machineID uuid.UUID) (domainfleet.Machine, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domainfleet.Machine{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)
	cur, err := q.GetMachineByIDForUpdate(ctx, machineID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainfleet.Machine{}, appfleet.ErrNotFound
		}
		return domainfleet.Machine{}, err
	}
	if uuid.Nil != companyID {
		return domainfleet.Machine{}, appfleet.ErrOrgMismatch
	}
	oldVer := cur.CredentialVersion
	newVer, err := q.BumpMachineCredentialVersion(ctx, machineID)
	if err != nil {
		return domainfleet.Machine{}, err
	}
	if err := q.MarkMachineCredentialRotatedByVersion(ctx, db.MarkMachineCredentialRotatedByVersionParams{
		MachineID:         machineID,
		CredentialVersion: oldVer,
	}); err != nil {
		return domainfleet.Machine{}, err
	}
	if _, err := q.InsertMachineCredential(ctx, db.InsertMachineCredentialParams{
		MachineID:         machineID,
		CredentialVersion: newVer,
		SecretHash:        nil,
		Status:            "active",
	}); err != nil {
		return domainfleet.Machine{}, err
	}
	if err := q.RevokeAllMachineSessionsForMachine(ctx, machineID); err != nil {
		return domainfleet.Machine{}, err
	}
	if err := q.AdminRevokeActiveMachineActivationCodes(ctx, machineID); err != nil {
		return domainfleet.Machine{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domainfleet.Machine{}, err
	}
	return r.GetMachine(ctx, machineID)
}

// RevokeMachineCredentialLifecycle revokes runtime sessions, marks active credential rows, then applies machines.revoke credential bump.
// When compromiseMachineCredentials is true, active credential rows are marked compromised instead of revoked before the bump.
func (r *fleetRepository) RevokeMachineCredentialLifecycle(ctx context.Context, companyID, machineID uuid.UUID, compromiseMachineCredentials bool) (domainfleet.Machine, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domainfleet.Machine{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)
	_, err = q.GetMachineByID(ctx, machineID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainfleet.Machine{}, appfleet.ErrNotFound
		}
		return domainfleet.Machine{}, err
	}
	if uuid.Nil != companyID {
		return domainfleet.Machine{}, appfleet.ErrOrgMismatch
	}
	if err := q.RevokeAllMachineSessionsForMachine(ctx, machineID); err != nil {
		return domainfleet.Machine{}, err
	}
	if compromiseMachineCredentials {
		if err := q.MarkMachineCredentialsCompromised(ctx, machineID); err != nil {
			return domainfleet.Machine{}, err
		}
	} else {
		if err := q.MarkMachineCredentialsRevokedActive(ctx, machineID); err != nil {
			return domainfleet.Machine{}, err
		}
	}
	if _, err := q.RevokeMachineCredentials(ctx, machineID); err != nil {
		return domainfleet.Machine{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domainfleet.Machine{}, err
	}
	return r.GetMachine(ctx, machineID)
}

func (r *fleetRepository) RevokeAllMachineSessionsOnly(ctx context.Context, companyID, machineID uuid.UUID) error {
	return db.New(r.pool).RevokeAllMachineSessionsForMachine(ctx, machineID)
}

package postgres

import (
	"context"
	"strings"
	"time"

	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OperatorRepository implements domainoperator.Repository.
type OperatorRepository struct {
	pool *pgxpool.Pool
}

func NewOperatorRepository(pool *pgxpool.Pool) *OperatorRepository {
	return &OperatorRepository{pool: pool}
}

func defaultJSONB(b []byte) []byte {
	if len(b) == 0 {
		return []byte("{}")
	}
	return b
}

func sameOperatorSessionActor(active db.MachineOperatorSession, in domainoperator.StartOperatorSessionParams) bool {
	if active.OrganizationID != in.OrganizationID {
		return false
	}
	if active.ActorType != in.ActorType {
		return false
	}
	switch in.ActorType {
	case domainoperator.ActorTypeTechnician:
		if in.TechnicianID == nil || !active.TechnicianID.Valid {
			return false
		}
		return uuid.UUID(active.TechnicianID.Bytes) == *in.TechnicianID
	case domainoperator.ActorTypeUser:
		if in.UserPrincipal == nil || !active.UserPrincipal.Valid {
			return false
		}
		return strings.TrimSpace(active.UserPrincipal.String) == strings.TrimSpace(*in.UserPrincipal)
	default:
		return false
	}
}

// StartOperatorSession runs in a single DB transaction: machine row FOR UPDATE, optional active
// session FOR UPDATE, then INSERT. Together with the partial unique index
// ux_machine_operator_sessions_one_active (see migrations/00008_machine_operator_sessions.sql),
// this prevents two ACTIVE rows for the same machine under concurrent logins. If you remove the
// pre-insert SELECT FOR UPDATE or the partial unique index, you can reintroduce a race where two
// transactions both insert ACTIVE — operators would see inconsistent "current operator" state.
//
// Same-actor login resumes the ACTIVE row (crash/reconnect). A different actor may claim the
// machine after StaleIdleReclaimAfter of silence on last_activity_at, or immediately when
// ForceAdminTakeover is authorized.
func (r *OperatorRepository) StartOperatorSession(ctx context.Context, in domainoperator.StartOperatorSessionParams) (domainoperator.Session, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domainoperator.Session{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)

	machine, err := q.GetMachineByIDForUpdate(ctx, in.MachineID)
	if err != nil {
		return domainoperator.Session{}, err
	}
	if machine.OrganizationID != in.OrganizationID {
		return domainoperator.Session{}, domainoperator.ErrOrganizationMismatch
	}

	active, err := q.GetActiveOperatorSessionByMachineIDForUpdate(ctx, in.MachineID)
	if err != nil && !isNoRows(err) {
		return domainoperator.Session{}, err
	}
	if err == nil {
		if sameOperatorSessionActor(active, in) {
			row, rerr := q.ResumeActiveOperatorSessionForActor(ctx, db.ResumeActiveOperatorSessionForActorParams{
				MachineID:      in.MachineID,
				OrganizationID: in.OrganizationID,
				ActorType:      in.ActorType,
				TechnicianID:   optionalUUIDToPg(in.TechnicianID),
				UserPrincipal:  optionalStringPtrToPgText(in.UserPrincipal),
				ExpiresAt:      optionalTimeToPgTimestamptz(in.ExpiresAt),
				ClientMetadata: defaultJSONB(in.ClientMetadata),
			})
			if rerr != nil {
				if isNoRows(rerr) {
					return domainoperator.Session{}, domainoperator.ErrActiveSessionExists
				}
				return domainoperator.Session{}, rerr
			}
			if in.InitialAuth != nil {
				if err := domainoperator.ValidateAuthEventSemantics(domainoperator.AuthEventSessionRefresh, in.InitialAuth.AuthMethod); err != nil {
					return domainoperator.Session{}, err
				}
				meta := defaultJSONB(in.InitialAuth.Metadata)
				sid := row.ID
				_, err = q.InsertMachineOperatorAuthEvent(ctx, db.InsertMachineOperatorAuthEventParams{
					OperatorSessionID: optionalUUIDToPg(&sid),
					MachineID:         row.MachineID,
					EventType:         domainoperator.AuthEventSessionRefresh,
					AuthMethod:        in.InitialAuth.AuthMethod,
					Column5:           ptrTimeOrNow(nil),
					CorrelationID:     optionalUUIDToPg(in.InitialAuth.CorrelationID),
					Metadata:          meta,
				})
				if err != nil {
					return domainoperator.Session{}, err
				}
			}
			if err := tx.Commit(ctx); err != nil {
				return domainoperator.Session{}, err
			}
			return mapOperatorSession(row), nil
		}

		staleAfter := in.StaleIdleReclaimAfter
		if staleAfter <= 0 {
			staleAfter = domainoperator.DefaultStaleIdleReclaimForDifferentOperator
		}
		idle := time.Now().UTC().Sub(active.LastActivityAt.UTC())

		var endReason *string
		var endStatus string
		switch {
		case in.ForceAdminTakeover && in.AdminTakeoverAuthorized:
			r := domainoperator.EndedReasonAdminForcedTakeover
			endReason = &r
			endStatus = domainoperator.SessionStatusRevoked
		case idle >= staleAfter:
			r := domainoperator.EndedReasonStaleSessionReclaimed
			endReason = &r
			endStatus = domainoperator.SessionStatusEnded
		default:
			return domainoperator.Session{}, domainoperator.ErrActiveSessionExists
		}

		endedAt := time.Now().UTC()
		_, err = q.EndMachineOperatorSession(ctx, db.EndMachineOperatorSessionParams{
			ID:          active.ID,
			Status:      endStatus,
			EndedAt:     pgtype.Timestamptz{Time: endedAt, Valid: true},
			EndedReason: optionalStringPtrToPgText(endReason),
		})
		if err != nil {
			if isNoRows(err) {
				return domainoperator.Session{}, domainoperator.ErrActiveSessionExists
			}
			return domainoperator.Session{}, err
		}
	}

	row, err := q.InsertMachineOperatorSession(ctx, db.InsertMachineOperatorSessionParams{
		OrganizationID: in.OrganizationID,
		MachineID:      in.MachineID,
		ActorType:      in.ActorType,
		TechnicianID:   optionalUUIDToPg(in.TechnicianID),
		UserPrincipal:  optionalStringPtrToPgText(in.UserPrincipal),
		Status:         domainoperator.SessionStatusActive,
		ExpiresAt:      optionalTimeToPgTimestamptz(in.ExpiresAt),
		ClientMetadata: defaultJSONB(in.ClientMetadata),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domainoperator.Session{}, domainoperator.ErrActiveSessionExists
		}
		return domainoperator.Session{}, err
	}

	if in.InitialAuth != nil {
		meta := defaultJSONB(in.InitialAuth.Metadata)
		eventType := in.InitialAuth.EventType
		if eventType == "" {
			eventType = domainoperator.AuthEventLoginSuccess
		}
		sid := row.ID
		_, err = q.InsertMachineOperatorAuthEvent(ctx, db.InsertMachineOperatorAuthEventParams{
			OperatorSessionID: optionalUUIDToPg(&sid),
			MachineID:         row.MachineID,
			EventType:         eventType,
			AuthMethod:        in.InitialAuth.AuthMethod,
			Column5:           ptrTimeOrNow(nil),
			CorrelationID:     optionalUUIDToPg(in.InitialAuth.CorrelationID),
			Metadata:          meta,
		})
		if err != nil {
			return domainoperator.Session{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domainoperator.Session{}, err
	}
	return mapOperatorSession(row), nil
}

func (r *OperatorRepository) GetOperatorSessionByID(ctx context.Context, id uuid.UUID) (domainoperator.Session, error) {
	row, err := db.New(r.pool).GetOperatorSessionByID(ctx, id)
	if err != nil {
		if isNoRows(err) {
			return domainoperator.Session{}, domainoperator.ErrSessionNotFound
		}
		return domainoperator.Session{}, err
	}
	return mapOperatorSession(row), nil
}

func (r *OperatorRepository) GetActiveSessionByMachineID(ctx context.Context, machineID uuid.UUID) (domainoperator.Session, error) {
	row, err := db.New(r.pool).GetActiveOperatorSessionByMachineID(ctx, machineID)
	if err != nil {
		if isNoRows(err) {
			return domainoperator.Session{}, domainoperator.ErrNoActiveSession
		}
		return domainoperator.Session{}, err
	}
	return mapOperatorSession(row), nil
}

// EndOperatorSession locks the session row, verifies ACTIVE, ends it, and optionally appends a
// logout auth event in the same transaction so the session cannot close without the audit row.
func (r *OperatorRepository) EndOperatorSession(ctx context.Context, in domainoperator.EndOperatorSessionParams) (domainoperator.Session, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domainoperator.Session{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)

	sess, err := q.GetOperatorSessionByIDForUpdate(ctx, in.SessionID)
	if err != nil {
		if isNoRows(err) {
			return domainoperator.Session{}, domainoperator.ErrSessionNotFound
		}
		return domainoperator.Session{}, err
	}
	if sess.Status != domainoperator.SessionStatusActive {
		return domainoperator.Session{}, domainoperator.ErrSessionNotActive
	}

	row, err := q.EndMachineOperatorSession(ctx, db.EndMachineOperatorSessionParams{
		ID:          in.SessionID,
		Status:      in.Status,
		EndedAt:     pgtype.Timestamptz{Time: in.EndedAt.UTC(), Valid: true},
		EndedReason: optionalStringPtrToPgText(in.EndedReason),
	})
	if err != nil {
		if isNoRows(err) {
			return domainoperator.Session{}, domainoperator.ErrSessionNotActive
		}
		return domainoperator.Session{}, err
	}

	if in.Logout != nil {
		log := *in.Logout
		sid := row.ID
		log.OperatorSessionID = &sid
		log.MachineID = row.MachineID
		_, err = q.InsertMachineOperatorAuthEvent(ctx, db.InsertMachineOperatorAuthEventParams{
			OperatorSessionID: optionalUUIDToPg(log.OperatorSessionID),
			MachineID:         log.MachineID,
			EventType:         log.EventType,
			AuthMethod:        log.AuthMethod,
			Column5:           ptrTimeOrNow(log.OccurredAt),
			CorrelationID:     optionalUUIDToPg(log.CorrelationID),
			Metadata:          defaultJSONB(log.Metadata),
		})
		if err != nil {
			return domainoperator.Session{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domainoperator.Session{}, err
	}
	return mapOperatorSession(row), nil
}

// TouchOperatorSessionActivity updates last activity for an ACTIVE session under row lock.
func (r *OperatorRepository) TouchOperatorSessionActivity(ctx context.Context, sessionID uuid.UUID) (domainoperator.Session, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domainoperator.Session{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)

	sess, err := q.GetOperatorSessionByIDForUpdate(ctx, sessionID)
	if err != nil {
		if isNoRows(err) {
			return domainoperator.Session{}, domainoperator.ErrSessionNotFound
		}
		return domainoperator.Session{}, err
	}
	if sess.Status != domainoperator.SessionStatusActive {
		return domainoperator.Session{}, domainoperator.ErrSessionNotActive
	}

	row, err := q.TouchMachineOperatorSessionActivity(ctx, sessionID)
	if err != nil {
		if isNoRows(err) {
			return domainoperator.Session{}, domainoperator.ErrSessionNotActive
		}
		return domainoperator.Session{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domainoperator.Session{}, err
	}
	return mapOperatorSession(row), nil
}

// TimeoutOperatorSessionIfExpired marks EXPIRED when expires_at is set and in the past, under row lock.
func (r *OperatorRepository) TimeoutOperatorSessionIfExpired(ctx context.Context, sessionID uuid.UUID) (domainoperator.Session, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domainoperator.Session{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)

	sess, err := q.GetOperatorSessionByIDForUpdate(ctx, sessionID)
	if err != nil {
		if isNoRows(err) {
			return domainoperator.Session{}, domainoperator.ErrSessionNotFound
		}
		return domainoperator.Session{}, err
	}
	if sess.Status != domainoperator.SessionStatusActive {
		return domainoperator.Session{}, domainoperator.ErrSessionNotActive
	}
	now := time.Now().UTC()
	if !sess.ExpiresAt.Valid || !sess.ExpiresAt.Time.Before(now) {
		return domainoperator.Session{}, domainoperator.ErrTimeoutNotApplicable
	}

	row, err := q.TimeoutMachineOperatorSessionIfExpired(ctx, sessionID)
	if err != nil {
		if isNoRows(err) {
			return domainoperator.Session{}, domainoperator.ErrTimeoutNotApplicable
		}
		return domainoperator.Session{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domainoperator.Session{}, err
	}
	return mapOperatorSession(row), nil
}

func (r *OperatorRepository) InsertAuthEvent(ctx context.Context, in domainoperator.InsertAuthEventParams) (domainoperator.AuthEvent, error) {
	row, err := db.New(r.pool).InsertMachineOperatorAuthEvent(ctx, db.InsertMachineOperatorAuthEventParams{
		OperatorSessionID: optionalUUIDToPg(in.OperatorSessionID),
		MachineID:         in.MachineID,
		EventType:         in.EventType,
		AuthMethod:        in.AuthMethod,
		Column5:           ptrTimeOrNow(in.OccurredAt),
		CorrelationID:     optionalUUIDToPg(in.CorrelationID),
		Metadata:          defaultJSONB(in.Metadata),
	})
	if err != nil {
		return domainoperator.AuthEvent{}, err
	}
	return mapOperatorAuthEvent(row), nil
}

func (r *OperatorRepository) InsertActionAttribution(ctx context.Context, in domainoperator.InsertActionAttributionParams) (domainoperator.ActionAttribution, error) {
	row, err := db.New(r.pool).InsertMachineActionAttribution(ctx, db.InsertMachineActionAttributionParams{
		OperatorSessionID: optionalUUIDToPg(in.OperatorSessionID),
		MachineID:         in.MachineID,
		ActionOriginType:  in.ActionOriginType,
		ResourceType:      in.ResourceType,
		ResourceID:        in.ResourceID,
		Column6:           ptrTimeOrNow(in.OccurredAt),
		Metadata:          defaultJSONB(in.Metadata),
		CorrelationID:     optionalUUIDToPg(in.CorrelationID),
	})
	if err != nil {
		return domainoperator.ActionAttribution{}, err
	}
	return mapOperatorActionAttribution(row), nil
}

func (r *OperatorRepository) ListSessionsByMachineID(ctx context.Context, machineID uuid.UUID, limit int32) ([]domainoperator.Session, error) {
	rows, err := db.New(r.pool).ListOperatorSessionsByMachineID(ctx, db.ListOperatorSessionsByMachineIDParams{
		MachineID: machineID,
		Limit:     limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domainoperator.Session, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapOperatorSession(row))
	}
	return out, nil
}

func (r *OperatorRepository) ListSessionsByTechnicianID(ctx context.Context, technicianID uuid.UUID, limit int32) ([]domainoperator.Session, error) {
	rows, err := db.New(r.pool).ListOperatorSessionsByTechnicianID(ctx, db.ListOperatorSessionsByTechnicianIDParams{
		TechnicianID: uuidToPg(technicianID),
		Limit:        limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domainoperator.Session, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapOperatorSession(row))
	}
	return out, nil
}

func (r *OperatorRepository) ListSessionsByUserPrincipal(ctx context.Context, in domainoperator.ListSessionsParams) ([]domainoperator.Session, error) {
	rows, err := db.New(r.pool).ListOperatorSessionsByUserPrincipal(ctx, db.ListOperatorSessionsByUserPrincipalParams{
		OrganizationID: in.OrganizationID,
		UserPrincipal:  optionalStringToPgText(in.UserPrincipal),
		Limit:          in.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domainoperator.Session, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapOperatorSession(row))
	}
	return out, nil
}

func (r *OperatorRepository) ListAuthEventsByMachineID(ctx context.Context, machineID uuid.UUID, limit int32) ([]domainoperator.AuthEvent, error) {
	rows, err := db.New(r.pool).ListMachineOperatorAuthEventsByMachineID(ctx, db.ListMachineOperatorAuthEventsByMachineIDParams{
		MachineID: machineID,
		Limit:     limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domainoperator.AuthEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapOperatorAuthEvent(row))
	}
	return out, nil
}

func (r *OperatorRepository) ListActionAttributionsByMachineID(ctx context.Context, machineID uuid.UUID, limit int32) ([]domainoperator.ActionAttribution, error) {
	rows, err := db.New(r.pool).ListMachineActionAttributionsByMachineID(ctx, db.ListMachineActionAttributionsByMachineIDParams{
		MachineID: machineID,
		Limit:     limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domainoperator.ActionAttribution, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapOperatorActionAttribution(row))
	}
	return out, nil
}

func (r *OperatorRepository) ListActionAttributionsByMachineAndResource(ctx context.Context, machineID uuid.UUID, resourceType, resourceID string, limit int32) ([]domainoperator.ActionAttribution, error) {
	rows, err := db.New(r.pool).ListMachineActionAttributionsByMachineAndResource(ctx, db.ListMachineActionAttributionsByMachineAndResourceParams{
		MachineID:    machineID,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Limit:        limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domainoperator.ActionAttribution, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapOperatorActionAttribution(row))
	}
	return out, nil
}

func (r *OperatorRepository) ListActionAttributionsForTechnician(ctx context.Context, organizationID, technicianID uuid.UUID, limit int32) ([]domainoperator.ActionAttribution, error) {
	rows, err := db.New(r.pool).ListMachineActionAttributionsForTechnician(ctx, db.ListMachineActionAttributionsForTechnicianParams{
		TechnicianID:   uuidToPg(technicianID),
		OrganizationID: organizationID,
		Limit:          limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domainoperator.ActionAttribution, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapOperatorActionAttribution(row))
	}
	return out, nil
}

func (r *OperatorRepository) ListActionAttributionsForUserPrincipal(ctx context.Context, organizationID uuid.UUID, userPrincipal string, limit int32) ([]domainoperator.ActionAttribution, error) {
	rows, err := db.New(r.pool).ListMachineActionAttributionsForUserPrincipal(ctx, db.ListMachineActionAttributionsForUserPrincipalParams{
		OrganizationID: organizationID,
		UserPrincipal:  optionalStringToPgText(userPrincipal),
		Limit:          limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domainoperator.ActionAttribution, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapOperatorActionAttribution(row))
	}
	return out, nil
}

var _ domainoperator.Repository = (*OperatorRepository)(nil)

package postgres

import (
	"context"
	"errors"
	"strings"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/domain/fleet"
	"github.com/avf/avf-vending-api/internal/domain/org"
	"github.com/avf/avf-vending-api/internal/domain/reliability"
	"github.com/avf/avf-vending-api/internal/domain/retail"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OrgRepository implements org.OrganizationRepository using sqlc queries.
type OrgRepository struct {
	pool *pgxpool.Pool
}

func NewOrgRepository(pool *pgxpool.Pool) *OrgRepository {
	return &OrgRepository{pool: pool}
}

func (r *OrgRepository) GetByID(ctx context.Context, id uuid.UUID) (org.Organization, error) {
	row, err := db.New(r.pool).GetOrganizationByID(ctx, id)
	if err != nil {
		return org.Organization{}, err
	}
	return mapOrganization(row), nil
}

var _ org.OrganizationRepository = (*OrgRepository)(nil)

// SiteRepository implements org.SiteRepository using sqlc queries.
type SiteRepository struct {
	pool *pgxpool.Pool
}

func NewSiteRepository(pool *pgxpool.Pool) *SiteRepository {
	return &SiteRepository{pool: pool}
}

func (r *SiteRepository) GetByID(ctx context.Context, id uuid.UUID) (org.Site, error) {
	row, err := db.New(r.pool).GetSiteByID(ctx, id)
	if err != nil {
		return org.Site{}, err
	}
	return mapSite(row), nil
}

var _ org.SiteRepository = (*SiteRepository)(nil)

// MachineRepository implements fleet.MachineRepository using sqlc queries.
type MachineRepository struct {
	pool *pgxpool.Pool
}

func NewMachineRepository(pool *pgxpool.Pool) *MachineRepository {
	return &MachineRepository{pool: pool}
}

func (r *MachineRepository) GetByID(ctx context.Context, id uuid.UUID) (fleet.Machine, error) {
	row, err := db.New(r.pool).GetMachineByID(ctx, id)
	if err != nil {
		return fleet.Machine{}, err
	}
	return mapMachine(row), nil
}

var _ fleet.MachineRepository = (*MachineRepository)(nil)

// TechnicianRepository implements fleet.TechnicianRepository using sqlc queries.
type TechnicianRepository struct {
	pool *pgxpool.Pool
}

func NewTechnicianRepository(pool *pgxpool.Pool) *TechnicianRepository {
	return &TechnicianRepository{pool: pool}
}

func (r *TechnicianRepository) GetByID(ctx context.Context, id uuid.UUID) (fleet.Technician, error) {
	row, err := db.New(r.pool).GetTechnicianByID(ctx, id)
	if err != nil {
		return fleet.Technician{}, err
	}
	return mapTechnician(row), nil
}

var _ fleet.TechnicianRepository = (*TechnicianRepository)(nil)

// TechnicianAssignmentRepository implements fleet.TechnicianMachineAssignmentChecker.
type TechnicianAssignmentRepository struct {
	pool *pgxpool.Pool
}

func NewTechnicianAssignmentRepository(pool *pgxpool.Pool) *TechnicianAssignmentRepository {
	return &TechnicianAssignmentRepository{pool: pool}
}

func (r *TechnicianAssignmentRepository) HasActiveAssignment(ctx context.Context, technicianID, machineID uuid.UUID) (bool, error) {
	return db.New(r.pool).TechnicianActiveAssignmentExists(ctx, technicianID, machineID)
}

var _ fleet.TechnicianMachineAssignmentChecker = (*TechnicianAssignmentRepository)(nil)

// ProductRepository implements retail.ProductRepository using sqlc queries.
type ProductRepository struct {
	pool *pgxpool.Pool
}

func NewProductRepository(pool *pgxpool.Pool) *ProductRepository {
	return &ProductRepository{pool: pool}
}

func (r *ProductRepository) GetByID(ctx context.Context, id uuid.UUID) (retail.Product, error) {
	row, err := db.New(r.pool).GetProductByID(ctx, id)
	if err != nil {
		return retail.Product{}, err
	}
	return mapProduct(row), nil
}

var _ retail.ProductRepository = (*ProductRepository)(nil)

// AuditRepository implements compliance.AuditRepository using sqlc queries.
type AuditRepository struct {
	pool *pgxpool.Pool
}

func NewAuditRepository(pool *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{pool: pool}
}

func (r *AuditRepository) Record(ctx context.Context, in compliance.AuditRecord) (compliance.AuditLog, error) {
	row, err := db.New(r.pool).InsertAuditLog(ctx, db.InsertAuditLogParams{
		OrganizationID: in.OrganizationID,
		ActorType:      in.ActorType,
		ActorID:        in.ActorID,
		Action:         in.Action,
		ResourceType:   in.ResourceType,
		ResourceID:     in.ResourceID,
		Payload:        in.Payload,
		Ip:             in.IP,
	})
	if err != nil {
		return compliance.AuditLog{}, err
	}
	return mapAuditLog(row), nil
}

var _ compliance.AuditRepository = (*AuditRepository)(nil)

// OutboxRepository implements reliability.OutboxRepository using sqlc queries.
//
// Publish path invariant (worker): transport publish happens before MarkOutboxPublished; JetStream
// dedupe (Nats-Msg-Id) is the safety net if the process dies between publish and mark, or if mark
// fails and a later cycle retries. Do not add a code path that sets published_at before a
// successful broker ack — that would acknowledge delivery without consumers ever seeing the event.
//
// Ops: table outbox_events, worker log fields outbox_* and worker_job_summary — see ops/RUNBOOK.md.
type OutboxRepository struct {
	pool *pgxpool.Pool
}

func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{pool: pool}
}

func (r *OutboxRepository) ListUnpublished(ctx context.Context, limit int32) ([]reliability.OutboxEvent, error) {
	rows, err := db.New(r.pool).ListOutboxUnpublished(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]reliability.OutboxEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapReliabilityOutbox(row))
	}
	return out, nil
}

const outboxPublishErrMsgMax = 512

func truncateOutboxPublishErrMsg(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= outboxPublishErrMsgMax {
		return s
	}
	return s[:outboxPublishErrMsgMax]
}

// RecordOutboxPublishFailure increments attempts and schedules the next try or dead-letters the row.
func (r *OutboxRepository) RecordOutboxPublishFailure(ctx context.Context, rec reliability.OutboxPublishFailureRecord) error {
	msg := truncateOutboxPublishErrMsg(rec.ErrorMessage)
	return db.New(r.pool).RecordOutboxPublishFailure(ctx, db.RecordOutboxPublishFailureParams{
		ID:               rec.EventID,
		LastPublishError: msg,
		NextPublishAfter: rec.NextPublishAfter,
		DeadLettered:     rec.DeadLettered,
	})
}

// GetOutboxPipelineStats returns aggregate counters for observability (one cheap snapshot query).
func (r *OutboxRepository) GetOutboxPipelineStats(ctx context.Context) (reliability.OutboxPipelineStats, error) {
	row, err := db.New(r.pool).GetOutboxPipelineStats(ctx)
	if err != nil {
		return reliability.OutboxPipelineStats{}, err
	}
	return reliability.OutboxPipelineStats{
		PendingTotal:           row.PendingTotal,
		PendingDueNow:          row.PendingDueNow,
		DeadLetteredTotal:      row.DeadLetteredTotal,
		OldestPendingCreatedAt: row.OldestPendingCreatedAt,
		MaxPendingAttempts:     row.MaxPendingAttempts,
	}, nil
}

// MarkOutboxPublished sets published_at when still null.
// If no row matched, marked is false: another worker may have completed the row, or the id is unknown.
// Callers should treat false as success when JetStream deduplication makes a duplicate publish safe.
func (r *OutboxRepository) MarkOutboxPublished(ctx context.Context, outboxID int64) (bool, error) {
	_, err := db.New(r.pool).MarkOutboxEventPublished(ctx, outboxID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

var _ reliability.OutboxRepository = (*OutboxRepository)(nil)
var _ domaincommerce.OutboxMarkPublishedWriter = (*OutboxRepository)(nil)

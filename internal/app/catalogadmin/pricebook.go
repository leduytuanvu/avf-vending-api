package catalogadmin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func normalizeCurrency(c string) string {
	return strings.ToUpper(strings.TrimSpace(c))
}

func validatePriceBookWindow(effectiveFrom time.Time, effectiveTo pgtype.Timestamptz) error {
	if effectiveTo.Valid && !effectiveTo.Time.After(effectiveFrom) {
		return fmt.Errorf("%w: effective_to must be after effective_from", ErrInvalidArgument)
	}
	return nil
}

func validateScopeColumns(scopeType string, siteID, machineID pgtype.UUID) error {
	switch scopeType {
	case "organization":
		if siteID.Valid || machineID.Valid {
			return fmt.Errorf("%w: organization scope must not set site_id or machine_id", ErrInvalidArgument)
		}
	case "site":
		if !siteID.Valid || machineID.Valid {
			return fmt.Errorf("%w: site scope requires site_id and no machine_id", ErrInvalidArgument)
		}
	case "machine":
		if siteID.Valid || !machineID.Valid {
			return fmt.Errorf("%w: machine scope requires machine_id and no site_id", ErrInvalidArgument)
		}
	default:
		return fmt.Errorf("%w: invalid scope_type", ErrInvalidArgument)
	}
	return nil
}

// CreatePriceBookInput validates and inserts a price book row.
type CreatePriceBookInput struct {
	OrganizationID uuid.UUID
	Name           string
	Currency       string
	EffectiveFrom  time.Time
	EffectiveTo    pgtype.Timestamptz
	IsDefault      bool
	ScopeType      string
	SiteID         pgtype.UUID
	MachineID      pgtype.UUID
	Priority       int32
}

// CreatePriceBook creates an active price book.
func (s *Service) CreatePriceBook(ctx context.Context, in CreatePriceBookInput) (db.PriceBook, error) {
	if s == nil {
		return db.PriceBook{}, errors.New("catalogadmin: nil service")
	}
	if in.OrganizationID == uuid.Nil {
		return db.PriceBook{}, ErrOrganizationRequired
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return db.PriceBook{}, fmt.Errorf("%w: name required", ErrInvalidArgument)
	}
	cur := normalizeCurrency(in.Currency)
	if len(cur) != 3 {
		return db.PriceBook{}, fmt.Errorf("%w: currency must be ISO 4217 (3 letters)", ErrInvalidArgument)
	}
	scope := strings.TrimSpace(strings.ToLower(in.ScopeType))
	if scope == "" {
		scope = "organization"
	}
	if err := validatePriceBookWindow(in.EffectiveFrom, in.EffectiveTo); err != nil {
		return db.PriceBook{}, err
	}
	if err := validateScopeColumns(scope, in.SiteID, in.MachineID); err != nil {
		return db.PriceBook{}, err
	}
	row, err := s.q.CatalogWriteInsertPriceBook(ctx, db.CatalogWriteInsertPriceBookParams{
		OrganizationID: in.OrganizationID,
		Name:           name,
		Currency:       cur,
		EffectiveFrom:  in.EffectiveFrom,
		EffectiveTo:    in.EffectiveTo,
		IsDefault:      in.IsDefault,
		Active:         true,
		ScopeType:      scope,
		SiteID:         in.SiteID,
		MachineID:      in.MachineID,
		Priority:       in.Priority,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return db.PriceBook{}, fmt.Errorf("%w: duplicate name/effective window for this scope", ErrConflict)
		}
		return db.PriceBook{}, err
	}
	s.recordCatalogWriteAudit(ctx, in.OrganizationID, compliance.ActionPriceBookCreated, "catalog.price_book", row.ID, priceBookAuditSnapshot(row))
	s.bumpCatalogCache(ctx, in.OrganizationID)
	return row, nil
}

// UpdatePriceBook persists merged catalog fields (HTTP layer applies PATCH semantics).
func (s *Service) UpdatePriceBook(ctx context.Context, p db.CatalogWriteUpdatePriceBookParams) (db.PriceBook, error) {
	if s == nil {
		return db.PriceBook{}, errors.New("catalogadmin: nil service")
	}
	if p.OrganizationID == uuid.Nil || p.ID == uuid.Nil {
		return db.PriceBook{}, ErrOrganizationRequired
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return db.PriceBook{}, fmt.Errorf("%w: name cannot be empty", ErrInvalidArgument)
	}
	p.Currency = normalizeCurrency(p.Currency)
	if len(p.Currency) != 3 {
		return db.PriceBook{}, fmt.Errorf("%w: currency must be ISO 4217 (3 letters)", ErrInvalidArgument)
	}
	p.ScopeType = strings.TrimSpace(strings.ToLower(p.ScopeType))
	if err := validatePriceBookWindow(p.EffectiveFrom, p.EffectiveTo); err != nil {
		return db.PriceBook{}, err
	}
	if err := validateScopeColumns(p.ScopeType, p.SiteID, p.MachineID); err != nil {
		return db.PriceBook{}, err
	}
	row, err := s.q.CatalogWriteUpdatePriceBook(ctx, p)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.PriceBook{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return db.PriceBook{}, fmt.Errorf("%w: duplicate name/effective window for this scope", ErrConflict)
		}
		return db.PriceBook{}, err
	}
	s.recordCatalogWriteAudit(ctx, p.OrganizationID, compliance.ActionPriceBookChanged, "catalog.price_book", row.ID, priceBookAuditSnapshot(row))
	s.bumpCatalogCache(ctx, p.OrganizationID)
	return row, nil
}

// ActivatePriceBook sets active=true (enterprise “reactivate” after deactivation).
func (s *Service) ActivatePriceBook(ctx context.Context, organizationID, priceBookID uuid.UUID) (db.PriceBook, error) {
	if s == nil {
		return db.PriceBook{}, errors.New("catalogadmin: nil service")
	}
	if organizationID == uuid.Nil || priceBookID == uuid.Nil {
		return db.PriceBook{}, ErrOrganizationRequired
	}
	cur, err := s.GetPriceBook(ctx, organizationID, priceBookID)
	if err != nil {
		return db.PriceBook{}, err
	}
	return s.UpdatePriceBook(ctx, db.CatalogWriteUpdatePriceBookParams{
		OrganizationID: cur.OrganizationID,
		ID:             cur.ID,
		Name:           cur.Name,
		Currency:       cur.Currency,
		EffectiveFrom:  cur.EffectiveFrom,
		EffectiveTo:    cur.EffectiveTo,
		IsDefault:      cur.IsDefault,
		Active:         true,
		ScopeType:      cur.ScopeType,
		SiteID:         cur.SiteID,
		MachineID:      cur.MachineID,
		Priority:       cur.Priority,
	})
}

// DeactivatePriceBook sets active=false.
func (s *Service) DeactivatePriceBook(ctx context.Context, organizationID, priceBookID uuid.UUID) (db.PriceBook, error) {
	if s == nil {
		return db.PriceBook{}, errors.New("catalogadmin: nil service")
	}
	if organizationID == uuid.Nil || priceBookID == uuid.Nil {
		return db.PriceBook{}, ErrOrganizationRequired
	}
	row, err := s.q.CatalogWriteDeactivatePriceBook(ctx, db.CatalogWriteDeactivatePriceBookParams{
		OrganizationID: organizationID,
		ID:             priceBookID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.PriceBook{}, ErrNotFound
		}
		return db.PriceBook{}, err
	}
	s.bumpCatalogCache(ctx, organizationID)
	return row, nil
}

// GetPriceBook returns a single price book for the tenant.
func (s *Service) GetPriceBook(ctx context.Context, organizationID, priceBookID uuid.UUID) (db.PriceBook, error) {
	if s == nil {
		return db.PriceBook{}, errors.New("catalogadmin: nil service")
	}
	if organizationID == uuid.Nil || priceBookID == uuid.Nil {
		return db.PriceBook{}, ErrOrganizationRequired
	}
	row, err := s.q.CatalogAdminGetPriceBook(ctx, db.CatalogAdminGetPriceBookParams{
		OrganizationID: organizationID,
		ID:             priceBookID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.PriceBook{}, ErrNotFound
		}
		return db.PriceBook{}, err
	}
	return row, nil
}

// PriceBookItemRow is one catalog price line.
type PriceBookItemRow struct {
	ProductID      uuid.UUID
	UnitPriceMinor int64
}

// ListPriceBookItems returns items for a book (tenant-scoped).
func (s *Service) ListPriceBookItems(ctx context.Context, organizationID, priceBookID uuid.UUID) ([]db.PriceBookItem, error) {
	if s == nil {
		return nil, errors.New("catalogadmin: nil service")
	}
	if organizationID == uuid.Nil || priceBookID == uuid.Nil {
		return nil, ErrOrganizationRequired
	}
	if _, err := s.q.CatalogAdminGetPriceBook(ctx, db.CatalogAdminGetPriceBookParams{
		OrganizationID: organizationID,
		ID:             priceBookID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return s.q.CatalogAdminListPriceBookItems(ctx, db.CatalogAdminListPriceBookItemsParams{
		OrganizationID: organizationID,
		PriceBookID:    priceBookID,
	})
}

// ReplacePriceBookItems replaces all items for the book (PUT semantics).
func (s *Service) ReplacePriceBookItems(ctx context.Context, organizationID, priceBookID uuid.UUID, items []PriceBookItemRow) error {
	if s == nil {
		return errors.New("catalogadmin: nil service")
	}
	if organizationID == uuid.Nil || priceBookID == uuid.Nil {
		return ErrOrganizationRequired
	}
	for _, it := range items {
		if it.ProductID == uuid.Nil || it.UnitPriceMinor < 0 {
			return fmt.Errorf("%w: invalid item row", ErrInvalidArgument)
		}
	}
	if _, err := s.q.CatalogAdminGetPriceBook(ctx, db.CatalogAdminGetPriceBookParams{
		OrganizationID: organizationID,
		ID:             priceBookID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := s.q.WithTx(tx)
	if err := qtx.CatalogWriteDeleteAllPriceBookItems(ctx, db.CatalogWriteDeleteAllPriceBookItemsParams{
		OrganizationID: organizationID,
		PriceBookID:    priceBookID,
	}); err != nil {
		return err
	}
	for _, it := range items {
		if _, err := qtx.CatalogWriteUpsertPriceBookItem(ctx, db.CatalogWriteUpsertPriceBookItemParams{
			OrganizationID: organizationID,
			PriceBookID:    priceBookID,
			ProductID:      it.ProductID,
			UnitPriceMinor: it.UnitPriceMinor,
		}); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.bumpCatalogCache(ctx, organizationID)
	return nil
}

// UpsertPriceBookItem PATCHes a single item.
func (s *Service) UpsertPriceBookItem(ctx context.Context, organizationID, priceBookID, productID uuid.UUID, unitPriceMinor int64) (db.PriceBookItem, error) {
	if s == nil {
		return db.PriceBookItem{}, errors.New("catalogadmin: nil service")
	}
	if organizationID == uuid.Nil || priceBookID == uuid.Nil || productID == uuid.Nil {
		return db.PriceBookItem{}, ErrOrganizationRequired
	}
	if unitPriceMinor < 0 {
		return db.PriceBookItem{}, fmt.Errorf("%w: unit_price_minor must be >= 0", ErrInvalidArgument)
	}
	if _, err := s.q.CatalogAdminGetPriceBook(ctx, db.CatalogAdminGetPriceBookParams{
		OrganizationID: organizationID,
		ID:             priceBookID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.PriceBookItem{}, ErrNotFound
		}
		return db.PriceBookItem{}, err
	}
	row, err := s.q.CatalogWriteUpsertPriceBookItem(ctx, db.CatalogWriteUpsertPriceBookItemParams{
		OrganizationID: organizationID,
		PriceBookID:    priceBookID,
		ProductID:      productID,
		UnitPriceMinor: unitPriceMinor,
	})
	if err != nil {
		return db.PriceBookItem{}, err
	}
	s.bumpCatalogCache(ctx, organizationID)
	return row, nil
}

// DeletePriceBookItem removes one item line.
func (s *Service) DeletePriceBookItem(ctx context.Context, organizationID, priceBookID, productID uuid.UUID) error {
	if s == nil {
		return errors.New("catalogadmin: nil service")
	}
	if organizationID == uuid.Nil || priceBookID == uuid.Nil || productID == uuid.Nil {
		return ErrOrganizationRequired
	}
	if _, err := s.q.CatalogAdminGetPriceBook(ctx, db.CatalogAdminGetPriceBookParams{
		OrganizationID: organizationID,
		ID:             priceBookID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	n, err := s.q.CatalogWriteDeletePriceBookItem(ctx, db.CatalogWriteDeletePriceBookItemParams{
		OrganizationID: organizationID,
		PriceBookID:    priceBookID,
		ProductID:      productID,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	s.bumpCatalogCache(ctx, organizationID)
	return nil
}

// AssignPriceBookTargetInput binds an organization-scoped book to a machine or site.
type AssignPriceBookTargetInput struct {
	OrganizationID uuid.UUID
	PriceBookID    uuid.UUID
	SiteID         *uuid.UUID
	MachineID      *uuid.UUID
}

// AssignPriceBookTarget inserts a target row (organization-scoped books only).
func (s *Service) AssignPriceBookTarget(ctx context.Context, in AssignPriceBookTargetInput) (db.PriceBookTarget, error) {
	if s == nil {
		return db.PriceBookTarget{}, errors.New("catalogadmin: nil service")
	}
	if in.OrganizationID == uuid.Nil || in.PriceBookID == uuid.Nil {
		return db.PriceBookTarget{}, ErrOrganizationRequired
	}
	pb, err := s.q.CatalogAdminGetPriceBook(ctx, db.CatalogAdminGetPriceBookParams{
		OrganizationID: in.OrganizationID,
		ID:             in.PriceBookID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.PriceBookTarget{}, ErrNotFound
		}
		return db.PriceBookTarget{}, err
	}
	if pb.ScopeType != "organization" {
		return db.PriceBookTarget{}, fmt.Errorf("%w: assign-target only supports organization-scoped price books", ErrInvalidArgument)
	}
	var sitePg pgtype.UUID
	var machPg pgtype.UUID
	switch {
	case in.SiteID != nil && in.MachineID != nil:
		return db.PriceBookTarget{}, fmt.Errorf("%w: specify exactly one of site_id or machine_id", ErrInvalidArgument)
	case in.MachineID != nil && *in.MachineID != uuid.Nil:
		machPg = pgtype.UUID{Bytes: *in.MachineID, Valid: true}
	case in.SiteID != nil && *in.SiteID != uuid.Nil:
		sitePg = pgtype.UUID{Bytes: *in.SiteID, Valid: true}
	default:
		return db.PriceBookTarget{}, fmt.Errorf("%w: site_id or machine_id required", ErrInvalidArgument)
	}

	row, err := s.q.CatalogWriteInsertPriceBookTarget(ctx, db.CatalogWriteInsertPriceBookTargetParams{
		OrganizationID: in.OrganizationID,
		PriceBookID:    in.PriceBookID,
		SiteID:         sitePg,
		MachineID:      machPg,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return db.PriceBookTarget{}, fmt.Errorf("%w: duplicate assignment for this book and machine/site", ErrConflict)
		}
		return db.PriceBookTarget{}, err
	}
	return row, nil
}

// DeletePriceBookTarget removes a target assignment.
func (s *Service) DeletePriceBookTarget(ctx context.Context, organizationID, priceBookID, targetID uuid.UUID) error {
	if s == nil {
		return errors.New("catalogadmin: nil service")
	}
	if organizationID == uuid.Nil || priceBookID == uuid.Nil || targetID == uuid.Nil {
		return ErrOrganizationRequired
	}
	tgt, err := s.q.CatalogAdminGetPriceBookTarget(ctx, db.CatalogAdminGetPriceBookTargetParams{
		OrganizationID: organizationID,
		ID:             targetID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if tgt.PriceBookID != priceBookID {
		return ErrNotFound
	}
	n, err := s.q.CatalogWriteDeletePriceBookTarget(ctx, db.CatalogWriteDeletePriceBookTargetParams{
		OrganizationID: organizationID,
		ID:             targetID,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListPriceBookTargets lists assignments for a price book.
func (s *Service) ListPriceBookTargets(ctx context.Context, organizationID, priceBookID uuid.UUID) ([]db.PriceBookTarget, error) {
	if s == nil {
		return nil, errors.New("catalogadmin: nil service")
	}
	if organizationID == uuid.Nil || priceBookID == uuid.Nil {
		return nil, ErrOrganizationRequired
	}
	if _, err := s.q.CatalogAdminGetPriceBook(ctx, db.CatalogAdminGetPriceBookParams{
		OrganizationID: organizationID,
		ID:             priceBookID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return s.q.CatalogAdminListPriceBookTargetsByBook(ctx, db.CatalogAdminListPriceBookTargetsByBookParams{
		OrganizationID: organizationID,
		PriceBookID:    priceBookID,
	})
}

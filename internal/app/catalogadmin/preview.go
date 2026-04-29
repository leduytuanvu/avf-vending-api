package catalogadmin

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PricingPreviewParams drives POST /v1/admin/pricing/preview.
type PricingPreviewParams struct {
	OrganizationID uuid.UUID
	MachineID      *uuid.UUID
	SiteID         *uuid.UUID
	ProductIDs     []uuid.UUID
	At             time.Time
}

// PricingPreviewLine is one resolved price for a product.
type PricingPreviewLine struct {
	ProductID      uuid.UUID
	BasePriceMinor int64
	EffectiveMinor int64
	Currency       string
	PriceBookID    uuid.UUID
	AppliedRuleIDs []string
	Reasons        []string
}

// PricingPreviewResult is the preview envelope body.
type PricingPreviewResult struct {
	At       time.Time            `json:"at"`
	Currency string               `json:"currency"`
	Lines    []PricingPreviewLine `json:"lines"`
}

type pricedCandidate struct {
	tier     int // 3 machine > 2 site > 1 org
	book     db.PriceBook
	targetID uuid.UUID
	hasTgt   bool
	price    int64
}

func cmpCandidates(a, b pricedCandidate) int {
	if a.tier != b.tier {
		if a.tier < b.tier {
			return -1
		}
		return 1
	}
	if a.book.Priority != b.book.Priority {
		if a.book.Priority < b.book.Priority {
			return -1
		}
		return 1
	}
	if !a.book.EffectiveFrom.Equal(b.book.EffectiveFrom) {
		if a.book.EffectiveFrom.Before(b.book.EffectiveFrom) {
			return -1
		}
		return 1
	}
	if a.book.ID.String() < b.book.ID.String() {
		return -1
	}
	if a.book.ID.String() > b.book.ID.String() {
		return 1
	}
	return 0
}

func pickWinner(cands []pricedCandidate) *pricedCandidate {
	if len(cands) == 0 {
		return nil
	}
	sort.SliceStable(cands, func(i, j int) bool {
		return cmpCandidates(cands[i], cands[j]) < 0
	})
	w := cands[len(cands)-1]
	return &w
}

func applicability(machineID, siteID *uuid.UUID, b db.PriceBook, tgts []db.PriceBookTarget) (tier int, targetID uuid.UUID, ok bool) {
	switch b.ScopeType {
	case "machine":
		if machineID != nil && b.MachineID.Valid && uuid.UUID(b.MachineID.Bytes) == *machineID {
			return 3, uuid.Nil, true
		}
	case "site":
		if siteID != nil && b.SiteID.Valid && uuid.UUID(b.SiteID.Bytes) == *siteID {
			return 2, uuid.Nil, true
		}
	case "organization":
		if len(tgts) == 0 {
			return 1, uuid.Nil, true
		}
		if machineID != nil {
			for _, t := range tgts {
				if t.MachineID.Valid && uuid.UUID(t.MachineID.Bytes) == *machineID {
					return 3, t.ID, true
				}
			}
		}
		if siteID != nil {
			for _, t := range tgts {
				if t.SiteID.Valid && uuid.UUID(t.SiteID.Bytes) == *siteID {
					return 2, t.ID, true
				}
			}
		}
	default:
		return 0, uuid.Nil, false
	}
	return 0, uuid.Nil, false
}

// PreviewPricing resolves effective prices for products at At under tenant scope.
func (s *Service) PreviewPricing(ctx context.Context, p PricingPreviewParams) (*PricingPreviewResult, error) {
	if s == nil {
		return nil, errors.New("catalogadmin: nil service")
	}
	if p.OrganizationID == uuid.Nil {
		return nil, ErrOrganizationRequired
	}
	if len(p.ProductIDs) == 0 {
		return nil, fmt.Errorf("%w: product_ids required", ErrInvalidArgument)
	}
	at := p.At
	if at.IsZero() {
		at = time.Now().UTC()
	} else {
		at = at.UTC()
	}

	nProd, err := s.q.CatalogAdminCountProductsInOrgByIDs(ctx, db.CatalogAdminCountProductsInOrgByIDsParams{
		OrganizationID: p.OrganizationID,
		Column2:        p.ProductIDs,
	})
	if err != nil {
		return nil, err
	}
	if int64(len(p.ProductIDs)) != nProd {
		return nil, fmt.Errorf("%w: one or more products not found in organization", ErrInvalidArgument)
	}

	var machineID *uuid.UUID
	var siteID *uuid.UUID
	switch {
	case p.MachineID != nil && *p.MachineID != uuid.Nil:
		machineID = p.MachineID
		sid, err := s.q.CatalogAdminGetMachineSiteForOrg(ctx, db.CatalogAdminGetMachineSiteForOrgParams{
			OrganizationID: p.OrganizationID,
			ID:             *machineID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("%w: machine not found", ErrInvalidArgument)
			}
			return nil, err
		}
		siteID = &sid
		if p.SiteID != nil && *p.SiteID != uuid.Nil && *p.SiteID != sid {
			return nil, fmt.Errorf("%w: site_id does not match machine site", ErrInvalidArgument)
		}
	case p.SiteID != nil && *p.SiteID != uuid.Nil:
		siteID = p.SiteID
	default:
		machineID = nil
		siteID = nil
	}

	books, err := s.q.CatalogAdminPricingPreviewBooksActiveAt(ctx, db.CatalogAdminPricingPreviewBooksActiveAtParams{
		OrganizationID: p.OrganizationID,
		Column2:        at,
	})
	if err != nil {
		return nil, err
	}
	allTargets, err := s.q.CatalogAdminListPriceBookTargetsByOrg(ctx, p.OrganizationID)
	if err != nil {
		return nil, err
	}
	tgtByBook := map[uuid.UUID][]db.PriceBookTarget{}
	for _, t := range allTargets {
		tgtByBook[t.PriceBookID] = append(tgtByBook[t.PriceBookID], t)
	}

	bookIDs := make([]uuid.UUID, 0, len(books))
	for _, b := range books {
		bookIDs = append(bookIDs, b.ID)
	}

	itemRows, err := s.q.CatalogAdminPriceBookItemsForPreview(ctx, db.CatalogAdminPriceBookItemsForPreviewParams{
		OrganizationID: p.OrganizationID,
		Column2:        bookIDs,
		Column3:        p.ProductIDs,
	})
	if err != nil {
		return nil, err
	}
	priceByBookProduct := map[uuid.UUID]map[uuid.UUID]int64{}
	for _, row := range itemRows {
		if _, ok := priceByBookProduct[row.PriceBookID]; !ok {
			priceByBookProduct[row.PriceBookID] = map[uuid.UUID]int64{}
		}
		priceByBookProduct[row.PriceBookID][row.ProductID] = row.UnitPriceMinor
	}

	var currency string
	for _, b := range books {
		if currency == "" {
			currency = normalizeCurrency(b.Currency)
			continue
		}
		if normalizeCurrency(b.Currency) != currency {
			return nil, fmt.Errorf("%w: conflicting currencies among active price books", ErrInvalidArgument)
		}
	}
	if currency == "" {
		currency = "USD"
	}

	out := &PricingPreviewResult{At: at, Currency: currency, Lines: make([]PricingPreviewLine, 0, len(p.ProductIDs))}

	for _, pid := range p.ProductIDs {
		var effCands []pricedCandidate
		var baseCands []pricedCandidate
		for _, bk := range books {
			tier, tgtID, ok := applicability(machineID, siteID, bk, tgtByBook[bk.ID])
			if !ok {
				continue
			}
			px, has := priceByBookProduct[bk.ID][pid]
			if !has {
				continue
			}
			pc := pricedCandidate{tier: tier, book: bk, price: px}
			if tgtID != uuid.Nil {
				pc.targetID = tgtID
				pc.hasTgt = true
			}
			effCands = append(effCands, pc)
			if tier == 1 {
				baseCands = append(baseCands, pc)
			}
		}

		win := pickWinner(effCands)
		baseWin := pickWinner(baseCands)

		line := PricingPreviewLine{ProductID: pid, Currency: currency}
		if win != nil {
			line.EffectiveMinor = win.price
			line.PriceBookID = win.book.ID
			line.AppliedRuleIDs = append(line.AppliedRuleIDs, "price_book:"+win.book.ID.String())
			if win.hasTgt {
				line.AppliedRuleIDs = append(line.AppliedRuleIDs, "price_book_target:"+win.targetID.String())
			}
			line.Reasons = append(line.Reasons,
				fmt.Sprintf("tier_%d", win.tier),
				fmt.Sprintf("priority_%d", win.book.Priority),
			)
		} else {
			line.Reasons = append(line.Reasons, "no_matching_price_book")
		}
		if baseWin != nil {
			line.BasePriceMinor = baseWin.price
			line.Reasons = append(line.Reasons, "base_from_org_wide_book")
		} else if win != nil {
			line.BasePriceMinor = win.price
			line.Reasons = append(line.Reasons, "base_fallback_effective")
		}

		out.Lines = append(out.Lines, line)
	}

	return out, nil
}

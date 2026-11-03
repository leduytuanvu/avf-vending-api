package catalogadmin

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/pgxutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	ProductListKindCommon = "common"
	ProductListKindCustom = "custom"
)

// ProductListSummary is one named product list (assortment) with membership count.
type ProductListSummary struct {
	List      db.Assortment
	ItemCount int64
}

// ProductListDetail is a list plus membership rows.
type ProductListDetail struct {
	List  db.Assortment
	Items []db.ProductListItemsRow
}

// ListProductListsParams pages and filters named product lists.
type ListProductListsParams struct {
	Limit  int32
	Offset int32
	Search string
	Status string
	Kind   string
}

// ListProductListsResult is a paged product-list collection.
type ListProductListsResult struct {
	Items      []ProductListSummary
	TotalCount int64
}

// CreateProductListInput creates a named list. Kind is common or custom.
type CreateProductListInput struct {
	Name        string
	Status      string
	Kind        string
	Description string
}

// UpdateProductListInput patches list metadata (not membership).
type UpdateProductListInput struct {
	ListID      uuid.UUID
	Name        *string
	Status      *string
	Description *string
}

// MachineProductListsAssignment is the active primary plus supplemental lists on a machine.
type MachineProductListsAssignment struct {
	MachineID           uuid.UUID
	PrimaryListID       *uuid.UUID
	SupplementalListIDs []uuid.UUID
	Bindings            []db.ProductListMachineBindingsRow
}

// PutMachineProductListsInput assigns one primary and zero or more supplemental lists.
type PutMachineProductListsInput struct {
	MachineID           uuid.UUID
	PrimaryListID       uuid.UUID
	SupplementalListIDs []uuid.UUID
}

// ProductListItemInput is one membership row for PUT /product-lists/{id}/items.
// Nil UnitPriceMinor inherits the previous list price, then the default price book.
type ProductListItemInput struct {
	ProductID      uuid.UUID
	UnitPriceMinor *int64
}

// ImportProductListResult reports created SKUs and membership after CSV/JSON import.
type ImportProductListResult struct {
	Detail        ProductListDetail
	CreatedCount  int
	AddedCount    int
	ExistingCount int
	SkippedCount  int
}

// ListProductLists returns named assortments for admin CRUD.
func (s *Service) ListProductLists(ctx context.Context, p ListProductListsParams) (*ListProductListsResult, error) {
	if s == nil {
		return nil, errors.New("catalogadmin: nil service")
	}
	search := strings.TrimSpace(p.Search)
	status := strings.ToLower(strings.TrimSpace(p.Status))
	kind := strings.ToLower(strings.TrimSpace(p.Kind))
	if kind != "" && kind != ProductListKindCommon && kind != ProductListKindCustom {
		return nil, fmt.Errorf("%w: kind must be common or custom", ErrInvalidArgument)
	}
	total, err := s.q.ProductListCount(ctx, db.ProductListCountParams{
		Column1: search,
		Column2: status,
		Column3: kind,
	})
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ProductListList(ctx, db.ProductListListParams{
		Column1: search,
		Column2: status,
		Column3: kind,
		Limit:   p.Limit,
		Offset:  p.Offset,
	})
	if err != nil {
		return nil, err
	}
	items := make([]ProductListSummary, 0, len(rows))
	for _, row := range rows {
		items = append(items, ProductListSummary{
			List: db.Assortment{
				ID:          row.ID,
				Name:        row.Name,
				Status:      row.Status,
				Kind:        row.Kind,
				Description: row.Description,
				Meta:        row.Meta,
				CreatedAt:   row.CreatedAt,
				UpdatedAt:   row.UpdatedAt,
			},
			ItemCount: row.ItemCount,
		})
	}
	return &ListProductListsResult{Items: items, TotalCount: total}, nil
}

// GetProductList returns a list and its membership.
func (s *Service) GetProductList(ctx context.Context, listID uuid.UUID) (*ProductListDetail, error) {
	if s == nil {
		return nil, errors.New("catalogadmin: nil service")
	}
	if listID == uuid.Nil {
		return nil, fmt.Errorf("%w: list id required", ErrInvalidArgument)
	}
	list, err := s.q.ProductListGet(ctx, listID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	items, err := s.q.ProductListItems(ctx, listID)
	if err != nil {
		return nil, err
	}
	return &ProductListDetail{List: list, Items: items}, nil
}

// CreateProductList inserts a named list. Kind common is unique per org.
func (s *Service) CreateProductList(ctx context.Context, in CreateProductListInput) (db.Assortment, error) {
	if s == nil {
		return db.Assortment{}, errors.New("catalogadmin: nil service")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return db.Assortment{}, fmt.Errorf("%w: name required", ErrInvalidArgument)
	}
	kind := strings.ToLower(strings.TrimSpace(in.Kind))
	if kind == "" {
		kind = ProductListKindCustom
	}
	if kind != ProductListKindCommon && kind != ProductListKindCustom {
		return db.Assortment{}, fmt.Errorf("%w: kind must be common or custom", ErrInvalidArgument)
	}
	status := strings.ToLower(strings.TrimSpace(in.Status))
	if status == "" {
		status = "published"
	}
	if err := validateAssortmentStatus(status); err != nil {
		return db.Assortment{}, err
	}
	row, err := s.q.ProductListInsert(ctx, db.ProductListInsertParams{
		Name:        name,
		Status:      status,
		Kind:        kind,
		Description: strings.TrimSpace(in.Description),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return db.Assortment{}, fmt.Errorf("%w: common product list already exists", ErrConflict)
		}
		return db.Assortment{}, err
	}
	return row, nil
}

// UpdateProductList patches name/status/description.
func (s *Service) UpdateProductList(ctx context.Context, in UpdateProductListInput) (db.Assortment, error) {
	if s == nil {
		return db.Assortment{}, errors.New("catalogadmin: nil service")
	}
	cur, err := s.q.ProductListGet(ctx, in.ListID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Assortment{}, ErrNotFound
		}
		return db.Assortment{}, err
	}
	name := cur.Name
	if in.Name != nil {
		name = strings.TrimSpace(*in.Name)
		if name == "" {
			return db.Assortment{}, fmt.Errorf("%w: name required", ErrInvalidArgument)
		}
	}
	status := cur.Status
	if in.Status != nil {
		status = strings.ToLower(strings.TrimSpace(*in.Status))
		if err := validateAssortmentStatus(status); err != nil {
			return db.Assortment{}, err
		}
	}
	desc := cur.Description
	if in.Description != nil {
		desc = strings.TrimSpace(*in.Description)
	}
	row, err := s.q.ProductListUpdate(ctx, db.ProductListUpdateParams{
		ID:          cur.ID,
		Name:        name,
		Status:      status,
		Description: desc,
	})
	if err != nil {
		return db.Assortment{}, err
	}
	return row, nil
}

// ReplaceProductListItems overwrites membership (order preserved) with optional per-list prices.
func (s *Service) ReplaceProductListItems(ctx context.Context, listID uuid.UUID, items []ProductListItemInput) (*ProductListDetail, error) {
	if s == nil {
		return nil, errors.New("catalogadmin: nil service")
	}
	if listID == uuid.Nil {
		return nil, fmt.Errorf("%w: list id required", ErrInvalidArgument)
	}
	normalized, err := normalizeProductListItems(items)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(normalized))
	for _, it := range normalized {
		ids = append(ids, it.ProductID)
	}
	if len(ids) > 0 {
		existing, err := s.q.ProductListExistingProductIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		if len(existing) != len(ids) {
			return nil, fmt.Errorf("%w: one or more product ids are not in the company catalog", ErrInvalidArgument)
		}
	}
	previous, err := s.q.ProductListItems(ctx, listID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	prevPrice := make(map[uuid.UUID]pgtype.Int8, len(previous))
	for _, row := range previous {
		prevPrice[row.ProductID] = row.UnitPriceMinor
	}
	needDefault := make([]uuid.UUID, 0, len(ids))
	for _, it := range normalized {
		if it.UnitPriceMinor != nil {
			continue
		}
		if prev, ok := prevPrice[it.ProductID]; ok && prev.Valid {
			continue
		}
		needDefault = append(needDefault, it.ProductID)
	}
	defaultPrice := map[uuid.UUID]int64{}
	if len(needDefault) > 0 {
		rows, err := s.q.ProductMasterDefaultPriceByProductIDs(ctx, needDefault)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			defaultPrice[row.ProductID] = row.UnitPriceMinor
		}
	}
	resolved := make([]pgtype.Int8, len(normalized))
	for i, it := range normalized {
		def, hasDefault := defaultPrice[it.ProductID]
		price, rerr := resolveListUnitPrice(it.UnitPriceMinor, prevPrice[it.ProductID], def, hasDefault)
		if rerr != nil {
			return nil, rerr
		}
		resolved[i] = price
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := pgxutil.NewQueries(tx)
	if _, err := qtx.ProductListGet(ctx, listID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if err := qtx.ProductListDeleteItems(ctx, listID); err != nil {
		return nil, err
	}
	for i, it := range normalized {
		if _, err := qtx.ProductListInsertItem(ctx, db.ProductListInsertItemParams{
			AssortmentID:   listID,
			ProductID:      it.ProductID,
			SortOrder:      int32(i),
			UnitPriceMinor: resolved[i],
		}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetProductList(ctx, listID)
}

// ImportProductListCSV creates missing SKUs (inactive) then appends membership.
func (s *Service) ImportProductListCSV(ctx context.Context, companyID, listID uuid.UUID, body io.Reader) (*ImportProductListResult, error) {
	rows, err := parseProductListCSV(body)
	if err != nil {
		return nil, err
	}
	return s.importProductListRows(ctx, companyID, listID, rows, nil)
}

// ImportProductListJSON creates missing SKUs and/or adds productIds then appends membership.
func (s *Service) ImportProductListJSON(ctx context.Context, companyID, listID uuid.UUID, raw json.RawMessage) (*ImportProductListResult, error) {
	var payload struct {
		Products   []importProductRow `json:"products"`
		Items      []importProductRow `json:"items"`
		ProductIDs []string           `json:"productIds"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("%w: invalid json", ErrInvalidArgument)
	}
	rows := payload.Products
	if len(rows) == 0 {
		rows = payload.Items
	}
	ids := make([]uuid.UUID, 0, len(payload.ProductIDs))
	for _, rawID := range payload.ProductIDs {
		id, err := uuid.Parse(strings.TrimSpace(rawID))
		if err != nil || id == uuid.Nil {
			return nil, fmt.Errorf("%w: invalid productIds entry", ErrInvalidArgument)
		}
		ids = append(ids, id)
	}
	return s.importProductListRows(ctx, companyID, listID, rows, ids)
}

type importProductRow struct {
	Sku         string `json:"sku"`
	Name        string `json:"name"`
	Barcode     string `json:"barcode"`
	Description string `json:"description"`
}

func (s *Service) importProductListRows(
	ctx context.Context,
	companyID, listID uuid.UUID,
	rows []importProductRow,
	extraIDs []uuid.UUID,
) (*ImportProductListResult, error) {
	if s == nil {
		return nil, errors.New("catalogadmin: nil service")
	}
	if _, err := s.q.ProductListGet(ctx, listID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	existingItems, err := s.q.ProductListItems(ctx, listID)
	if err != nil {
		return nil, err
	}
	member := make(map[uuid.UUID]struct{}, len(existingItems)+len(rows)+len(extraIDs))
	ordered := make([]uuid.UUID, 0, len(existingItems)+len(rows)+len(extraIDs))
	for _, it := range existingItems {
		if _, ok := member[it.ProductID]; ok {
			continue
		}
		member[it.ProductID] = struct{}{}
		ordered = append(ordered, it.ProductID)
	}
	created := 0
	existingHits := 0
	skipped := 0
	added := 0
	addID := func(id uuid.UUID) {
		if id == uuid.Nil {
			return
		}
		if _, ok := member[id]; ok {
			return
		}
		member[id] = struct{}{}
		ordered = append(ordered, id)
		added++
	}
	for _, row := range rows {
		sku := strings.TrimSpace(row.Sku)
		name := strings.TrimSpace(row.Name)
		if sku == "" || name == "" {
			skipped++
			continue
		}
		found, err := s.q.ProductListGetProductBySku(ctx, sku)
		if err == nil {
			existingHits++
			addID(found.ID)
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		var barcode *string
		if b := strings.TrimSpace(row.Barcode); b != "" {
			barcode = &b
		}
		createdRow, cerr := s.CreateProduct(ctx, CreateProductInput{
			Sku:         sku,
			Barcode:     barcode,
			Name:        name,
			Description: strings.TrimSpace(row.Description),
			Active:      false,
			CompanyID:   companyID,
		})
		if cerr != nil {
			return nil, cerr
		}
		created++
		addID(createdRow.ID)
	}
	if len(extraIDs) > 0 {
		existing, err := s.q.ProductListExistingProductIDs(ctx, extraIDs)
		if err != nil {
			return nil, err
		}
		if len(existing) != len(normalizeUUIDSlice(extraIDs)) {
			return nil, fmt.Errorf("%w: one or more product ids are not in the company catalog", ErrInvalidArgument)
		}
		for _, id := range extraIDs {
			addID(id)
		}
	}
	inputs := make([]ProductListItemInput, 0, len(ordered))
	for _, id := range ordered {
		inputs = append(inputs, ProductListItemInput{ProductID: id})
	}
	detail, err := s.ReplaceProductListItems(ctx, listID, inputs)
	if err != nil {
		return nil, err
	}
	return &ImportProductListResult{
		Detail:        *detail,
		CreatedCount:  created,
		AddedCount:    added,
		ExistingCount: existingHits,
		SkippedCount:  skipped,
	}, nil
}

// GetMachineProductLists returns the current primary + supplemental assignment.
func (s *Service) GetMachineProductLists(ctx context.Context, machineID uuid.UUID) (*MachineProductListsAssignment, error) {
	if s == nil {
		return nil, errors.New("catalogadmin: nil service")
	}
	if err := s.requireMachine(ctx, machineID); err != nil {
		return nil, err
	}
	bindings, err := s.q.ProductListMachineBindings(ctx, machineID)
	if err != nil {
		return nil, err
	}
	out := &MachineProductListsAssignment{
		MachineID:           machineID,
		SupplementalListIDs: make([]uuid.UUID, 0),
		Bindings:            bindings,
	}
	for _, b := range bindings {
		id := b.AssortmentID
		if b.IsPrimary {
			out.PrimaryListID = &id
			continue
		}
		out.SupplementalListIDs = append(out.SupplementalListIDs, id)
	}
	return out, nil
}

// PutMachineProductLists replaces active primary and supplemental bindings.
// Supplemental assignment does not change the commerce primary assortment allow-list.
func (s *Service) PutMachineProductLists(ctx context.Context, in PutMachineProductListsInput) (*MachineProductListsAssignment, error) {
	if s == nil {
		return nil, errors.New("catalogadmin: nil service")
	}
	if err := s.requireMachine(ctx, in.MachineID); err != nil {
		return nil, err
	}
	if in.PrimaryListID == uuid.Nil {
		return nil, fmt.Errorf("%w: primaryListId required", ErrInvalidArgument)
	}
	if err := s.requirePublishedList(ctx, in.PrimaryListID); err != nil {
		return nil, err
	}
	supp, err := normalizeProductIDs(in.SupplementalListIDs)
	if err != nil {
		return nil, err
	}
	seen := map[uuid.UUID]struct{}{in.PrimaryListID: {}}
	for _, id := range supp {
		if _, ok := seen[id]; ok {
			return nil, fmt.Errorf("%w: duplicate list id in assignment", ErrInvalidArgument)
		}
		seen[id] = struct{}{}
		if err := s.requirePublishedList(ctx, id); err != nil {
			return nil, err
		}
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := pgxutil.NewQueries(tx)
	if err := qtx.ProductListCloseMachineBindings(ctx, db.ProductListCloseMachineBindingsParams{
		MachineID: in.MachineID,
		Column2:   true,
	}); err != nil {
		return nil, err
	}
	if err := qtx.ProductListCloseMachineBindings(ctx, db.ProductListCloseMachineBindingsParams{
		MachineID: in.MachineID,
		Column2:   false,
	}); err != nil {
		return nil, err
	}
	if err := qtx.ProductListInsertBinding(ctx, db.ProductListInsertBindingParams{
		MachineID:    in.MachineID,
		AssortmentID: in.PrimaryListID,
		IsPrimary:    true,
	}); err != nil {
		return nil, err
	}
	for _, id := range supp {
		if err := qtx.ProductListInsertBinding(ctx, db.ProductListInsertBindingParams{
			MachineID:    in.MachineID,
			AssortmentID: id,
			IsPrimary:    false,
		}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetMachineProductLists(ctx, in.MachineID)
}

func (s *Service) requireMachine(ctx context.Context, machineID uuid.UUID) error {
	if machineID == uuid.Nil {
		return fmt.Errorf("%w: machine id required", ErrInvalidArgument)
	}
	if _, err := s.q.FleetAdminGetMachineDetail(ctx, machineID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (s *Service) requirePublishedList(ctx context.Context, listID uuid.UUID) error {
	list, err := s.q.ProductListGet(ctx, listID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: product list not found", ErrNotFound)
		}
		return err
	}
	if list.Status != "published" {
		return fmt.Errorf("%w: product list must be published", ErrInvalidArgument)
	}
	return nil
}

func validateAssortmentStatus(status string) error {
	switch status {
	case "draft", "published", "archived":
		return nil
	default:
		return fmt.Errorf("%w: status must be draft, published, or archived", ErrInvalidArgument)
	}
}

func normalizeProductIDs(ids []uuid.UUID) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, 0, len(ids))
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			return nil, fmt.Errorf("%w: product id required", ErrInvalidArgument)
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

func normalizeProductListItems(items []ProductListItemInput) ([]ProductListItemInput, error) {
	out := make([]ProductListItemInput, 0, len(items))
	seen := make(map[uuid.UUID]struct{}, len(items))
	for _, it := range items {
		if it.ProductID == uuid.Nil {
			return nil, fmt.Errorf("%w: product id required", ErrInvalidArgument)
		}
		if _, ok := seen[it.ProductID]; ok {
			continue
		}
		seen[it.ProductID] = struct{}{}
		out = append(out, it)
	}
	return out, nil
}

func resolveListUnitPrice(explicit *int64, existing pgtype.Int8, defaultMinor int64, hasDefault bool) (pgtype.Int8, error) {
	if explicit != nil {
		if *explicit < 0 {
			return pgtype.Int8{}, fmt.Errorf("%w: unitPriceMinor must be >= 0", ErrInvalidArgument)
		}
		return pgtype.Int8{Int64: *explicit, Valid: true}, nil
	}
	if existing.Valid {
		return existing, nil
	}
	if hasDefault {
		if defaultMinor < 0 {
			return pgtype.Int8{}, fmt.Errorf("%w: unitPriceMinor must be >= 0", ErrInvalidArgument)
		}
		return pgtype.Int8{Int64: defaultMinor, Valid: true}, nil
	}
	return pgtype.Int8{}, nil
}

func normalizeUUIDSlice(ids []uuid.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func parseProductListCSV(body io.Reader) ([]importProductRow, error) {
	r := csv.NewReader(body)
	r.TrimLeadingSpace = true
	r.ReuseRecord = true
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("%w: csv header required", ErrInvalidArgument)
	}
	idx := map[string]int{}
	for i, col := range header {
		idx[strings.ToLower(strings.TrimSpace(col))] = i
	}
	skuIdx, ok := idx["sku"]
	if !ok {
		return nil, fmt.Errorf("%w: csv requires sku column", ErrInvalidArgument)
	}
	nameIdx, ok := idx["name"]
	if !ok {
		return nil, fmt.Errorf("%w: csv requires name column", ErrInvalidArgument)
	}
	barcodeIdx := -1
	if i, ok := idx["barcode"]; ok {
		barcodeIdx = i
	}
	descIdx := -1
	if i, ok := idx["description"]; ok {
		descIdx = i
	}
	var rows []importProductRow
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidArgument, err)
		}
		row := importProductRow{
			Sku:  csvCell(rec, skuIdx),
			Name: csvCell(rec, nameIdx),
		}
		if barcodeIdx >= 0 {
			row.Barcode = csvCell(rec, barcodeIdx)
		}
		if descIdx >= 0 {
			row.Description = csvCell(rec, descIdx)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func csvCell(rec []string, idx int) string {
	if idx < 0 || idx >= len(rec) {
		return ""
	}
	return strings.TrimSpace(rec[idx])
}

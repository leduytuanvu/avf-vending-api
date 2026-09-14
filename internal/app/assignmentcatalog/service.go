package assignmentcatalog

import (
	"context"
	"fmt"
	"hash/fnv"
	"log"
	"sort"
	"strings"
	"time"

	appcatalogadmin "github.com/avf/avf-vending-api/internal/app/catalogadmin"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/objectstore"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const maxAssignmentProducts int32 = 500

// Service builds company-wide assignment catalogs for technician product pickers.
type Service struct {
	catalogAdmin *appcatalogadmin.Service
	mediaStore   objectstore.Store
	presignTTL   time.Duration
}

func NewService(catalogAdmin *appcatalogadmin.Service, mediaStore objectstore.Store, presignTTL time.Duration) *Service {
	if presignTTL <= 0 {
		presignTTL = 15 * time.Minute
	}
	return &Service{
		catalogAdmin: catalogAdmin,
		mediaStore:   mediaStore,
		presignTTL:   presignTTL,
	}
}

// Product is one assignable company product row for technician sync.
type Product struct {
	ID                   uuid.UUID
	SKU                  string
	Barcode              string
	Name                 string
	ShortName            string
	Description          string
	Active               bool
	BasePriceMinor       int64
	Currency             string
	ImageKey             string
	ImageHash            string
	ThumbURL             string
	DisplayURL           string
	ImageContentRevision int64
	UpdatedAt            time.Time
}

// Snapshot is the full assignment catalog projection for one machine.
type Snapshot struct {
	MachineID      uuid.UUID
	CatalogVersion int32
	GeneratedAt    time.Time
	Products       []Product
}

// Delta is an assignment-catalog change set since a prior catalog version.
type Delta struct {
	FromCatalogVersion int32
	ToCatalogVersion   int32
	GeneratedAt        time.Time
	Upserts            []Product
	DeletedProductIDs  []uuid.UUID
}

// BuildSnapshot loads the company-wide product master for technician assignment.
func (s *Service) BuildSnapshot(ctx context.Context, machineID uuid.UUID) (*Snapshot, error) {
	if s == nil || s.catalogAdmin == nil {
		return nil, fmt.Errorf("assignmentcatalog: service not configured")
	}
	if machineID == uuid.Nil {
		// REST admin bootstrap is company-scoped; machine id is optional and used for logging only.
	}
	res, err := s.catalogAdmin.ListProducts(ctx, appcatalogadmin.ListProductsParams{
		Limit:      maxAssignmentProducts,
		Offset:     0,
		ActiveOnly: false,
	})
	if err != nil {
		return nil, err
	}
	products, err := s.enrichProducts(ctx, res.Items)
	if err != nil {
		return nil, err
	}
	version := CatalogVersionInt(products)
	log.Printf("ASSIGNMENT_CATALOG_SNAPSHOT machineId=%s productCount=%d catalogVersion=%d", machineID, len(products), version)
	return &Snapshot{
		MachineID:      machineID,
		CatalogVersion: version,
		GeneratedAt:    time.Now().UTC(),
		Products:       products,
	}, nil
}

// BuildDelta compares basisCatalogVersion to the current snapshot and returns upserts/tombstones.
func (s *Service) BuildDelta(ctx context.Context, machineID uuid.UUID, basisCatalogVersion int32) (*Delta, error) {
	snap, err := s.BuildSnapshot(ctx, machineID)
	if err != nil {
		return nil, err
	}
	out := &Delta{
		FromCatalogVersion: basisCatalogVersion,
		ToCatalogVersion:   snap.CatalogVersion,
		GeneratedAt:        snap.GeneratedAt,
	}
	if basisCatalogVersion > 0 && basisCatalogVersion == snap.CatalogVersion {
		return out, nil
	}
	out.Upserts = snap.Products
	return out, nil
}

func (s *Service) enrichProducts(ctx context.Context, rows []db.CatalogAdminListProductsRow) ([]Product, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	pids := make([]uuid.UUID, len(rows))
	for i := range rows {
		pids[i] = rows[i].ID
	}
	assetByProd, err := s.catalogAdmin.PrimaryMediaAssetByProductIDs(ctx, pids)
	if err != nil {
		return nil, err
	}
	products := make([]Product, 0, len(rows))
	for _, row := range rows {
		p := Product{
			ID:          row.ID,
			SKU:         row.Sku,
			Barcode:     textFromPg(row.Barcode),
			Name:        row.Name,
			Description: row.Description,
			Active:      row.Active,
			Currency:    "VND",
			UpdatedAt:   row.UpdatedAt.UTC(),
		}
		if aid, ok := assetByProd[row.ID]; ok {
			p.ImageKey = fmt.Sprintf("product:%s", row.ID)
			p.ImageHash = aid.String()
			p.ImageContentRevision = row.UpdatedAt.UTC().Unix()
		}
		products = append(products, p)
	}
	sort.Slice(products, func(i, j int) bool {
		if products[i].Name == products[j].Name {
			return products[i].ID.String() < products[j].ID.String()
		}
		return products[i].Name < products[j].Name
	})
	return products, nil
}

func textFromPg(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return strings.TrimSpace(v.String)
}

// CatalogVersionInt derives a stable positive catalog version from product rows.
func CatalogVersionInt(products []Product) int32 {
	h := fnv.New32a()
	for _, p := range products {
		_, _ = h.Write([]byte(p.ID.String()))
		_, _ = h.Write([]byte("|"))
		_, _ = h.Write([]byte(p.UpdatedAt.UTC().Format(time.RFC3339Nano)))
		_, _ = h.Write([]byte("|"))
		_, _ = h.Write([]byte(p.ImageHash))
		_, _ = h.Write([]byte("\n"))
	}
	v := int32(h.Sum32() & 0x7fffffff)
	if v <= 0 {
		return 1
	}
	return v
}

package mediaadmin

import (
	"context"
	"time"

	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/platform/objectstore"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CatalogMediaCacheBumper optional hook when Redis (or similar) caches machine catalog / media manifest hints.
type CatalogMediaCacheBumper interface {
	BumpOrganizationMedia(ctx context.Context, organizationID uuid.UUID)
}

// Deps wires Postgres, object storage, and optional audit / cache hooks.
type Deps struct {
	Pool           *pgxpool.Pool
	Store          objectstore.Store
	Audit          compliance.EnterpriseRecorder
	Variants       VariantGenerator
	PresignPutTTL  time.Duration
	MaxUploadBytes int64
	// ThumbMaxPixels and DisplayMaxPixels bound generated WebP variants (defaults 256 / 1024 when zero).
	ThumbMaxPixels   int
	DisplayMaxPixels int
	Cache            CatalogMediaCacheBumper
}

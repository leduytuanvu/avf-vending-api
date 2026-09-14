package catalogbootstrap

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Lookup maps natural keys to UUIDs in the database.
type Lookup struct {
	pool *pgxpool.Pool
}

func NewLookup(pool *pgxpool.Pool) *Lookup {
	return &Lookup{pool: pool}
}

func (l *Lookup) CategoryIDBySlug(ctx context.Context, slug string) (uuid.UUID, error) {
	var id uuid.UUID
	err := l.pool.QueryRow(ctx,
		`SELECT id FROM categories WHERE lower(slug) = lower($1) LIMIT 1`, strings.TrimSpace(slug)).Scan(&id)
	return id, err
}

func (l *Lookup) BrandIDBySlug(ctx context.Context, slug string) (uuid.UUID, error) {
	var id uuid.UUID
	err := l.pool.QueryRow(ctx,
		`SELECT id FROM brands WHERE lower(slug) = lower($1) LIMIT 1`, strings.TrimSpace(slug)).Scan(&id)
	return id, err
}

func (l *Lookup) TagIDBySlug(ctx context.Context, slug string) (uuid.UUID, error) {
	var id uuid.UUID
	err := l.pool.QueryRow(ctx,
		`SELECT id FROM tags WHERE lower(slug) = lower($1) LIMIT 1`, strings.TrimSpace(slug)).Scan(&id)
	return id, err
}

func (l *Lookup) ProductIDBySKU(ctx context.Context, sku string) (uuid.UUID, error) {
	var id uuid.UUID
	err := l.pool.QueryRow(ctx,
		`SELECT id FROM products WHERE sku = $1 LIMIT 1`, strings.TrimSpace(sku)).Scan(&id)
	return id, err
}

func (l *Lookup) MediaAssetByProviderPublicID(ctx context.Context, publicID string) (uuid.UUID, error) {
	var id uuid.UUID
	err := l.pool.QueryRow(ctx,
		`SELECT id FROM media_assets WHERE provider_public_id = $1 LIMIT 1`, strings.TrimSpace(publicID)).Scan(&id)
	return id, err
}

func (l *Lookup) DefaultPriceBookID(ctx context.Context) (uuid.UUID, error) {
	var id uuid.UUID
	err := l.pool.QueryRow(ctx,
		`SELECT id FROM price_books WHERE active = true AND is_default = true AND price_book_level = 'global' ORDER BY effective_from DESC LIMIT 1`).Scan(&id)
	return id, err
}

type CatalogCounts struct {
	Products       int64
	Categories     int64
	Brands         int64
	Tags           int64
	ProductTags    int64
	MediaAssets    int64
	ProductImages  int64
	ProductMedia   int64
	PriceBooks     int64
	PriceBookItems int64
}

func (l *Lookup) CatalogCounts(ctx context.Context) (CatalogCounts, error) {
	var c CatalogCounts
	queries := []struct {
		dest *int64
		sql  string
	}{
		{&c.Products, `SELECT count(*) FROM products`},
		{&c.Categories, `SELECT count(*) FROM categories`},
		{&c.Brands, `SELECT count(*) FROM brands`},
		{&c.Tags, `SELECT count(*) FROM tags`},
		{&c.ProductTags, `SELECT count(*) FROM product_tags`},
		{&c.MediaAssets, `SELECT count(*) FROM media_assets`},
		{&c.ProductImages, `SELECT count(*) FROM product_images`},
		{&c.ProductMedia, `SELECT count(*) FROM product_media`},
		{&c.PriceBooks, `SELECT count(*) FROM price_books`},
		{&c.PriceBookItems, `SELECT count(*) FROM price_book_items`},
	}
	for _, q := range queries {
		if err := l.pool.QueryRow(ctx, q.sql).Scan(q.dest); err != nil {
			return c, err
		}
	}
	return c, nil
}

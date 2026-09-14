package catalogbootstrap

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/catalogadmin"
	"github.com/google/uuid"
)

// ImportTaxonomy idempotently creates categories, brands, and tags.
func ImportTaxonomy(ctx context.Context, catSvc *catalogadmin.Service, lookup *Lookup, m *Manifest, dryRun bool) (map[string]uuid.UUID, map[string]uuid.UUID, map[string]uuid.UUID, error) {
	if m == nil {
		return nil, nil, nil, errors.New("nil manifest")
	}
	catIDs := map[string]uuid.UUID{}
	brandIDs := map[string]uuid.UUID{}
	tagIDs := map[string]uuid.UUID{}

	// Parents first.
	for _, c := range m.Taxonomy.Categories {
		if c.ParentSlug != nil && strings.TrimSpace(*c.ParentSlug) != "" {
			continue
		}
		slug := strings.TrimSpace(c.Slug)
		if slug == "" {
			continue
		}
		if dryRun {
			catIDs[slug] = uuid.Nil
			continue
		}
		row, err := catSvc.CreateCategory(ctx, catalogadmin.CreateCategoryInput{
			Slug:   slug,
			Name:   strings.TrimSpace(c.Name),
			Active: c.Active || true,
		})
		if err != nil {
			if errors.Is(err, catalogadmin.ErrDuplicateSlug) {
				id, lerr := lookup.CategoryIDBySlug(ctx, slug)
				if lerr != nil {
					return nil, nil, nil, lerr
				}
				catIDs[slug] = id
				continue
			}
			return nil, nil, nil, fmt.Errorf("category %s: %w", slug, err)
		}
		catIDs[slug] = row.ID
	}
	// Children.
	for _, c := range m.Taxonomy.Categories {
		if c.ParentSlug == nil || strings.TrimSpace(*c.ParentSlug) == "" {
			continue
		}
		slug := strings.TrimSpace(c.Slug)
		parentSlug := strings.TrimSpace(*c.ParentSlug)
		parentID := catIDs[parentSlug]
		if parentID == uuid.Nil && !dryRun {
			id, err := lookup.CategoryIDBySlug(ctx, parentSlug)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("parent category %s: %w", parentSlug, err)
			}
			parentID = id
		}
		if dryRun {
			catIDs[slug] = uuid.Nil
			continue
		}
		pid := parentID
		row, err := catSvc.CreateCategory(ctx, catalogadmin.CreateCategoryInput{
			Slug:     slug,
			Name:     strings.TrimSpace(c.Name),
			ParentID: &pid,
			Active:   c.Active || true,
		})
		if err != nil {
			if errors.Is(err, catalogadmin.ErrDuplicateSlug) {
				id, lerr := lookup.CategoryIDBySlug(ctx, slug)
				if lerr != nil {
					return nil, nil, nil, lerr
				}
				catIDs[slug] = id
				continue
			}
			return nil, nil, nil, fmt.Errorf("category %s: %w", slug, err)
		}
		catIDs[slug] = row.ID
	}

	for _, b := range m.Taxonomy.Brands {
		slug := strings.TrimSpace(b.Slug)
		if slug == "" {
			continue
		}
		if dryRun {
			brandIDs[slug] = uuid.Nil
			continue
		}
		row, err := catSvc.CreateBrand(ctx, catalogadmin.CreateBrandInput{
			Slug:   slug,
			Name:   strings.TrimSpace(b.Name),
			Active: b.Active || true,
		})
		if err != nil {
			if errors.Is(err, catalogadmin.ErrDuplicateSlug) {
				id, lerr := lookup.BrandIDBySlug(ctx, slug)
				if lerr != nil {
					return nil, nil, nil, lerr
				}
				brandIDs[slug] = id
				continue
			}
			return nil, nil, nil, fmt.Errorf("brand %s: %w", slug, err)
		}
		brandIDs[slug] = row.ID
	}

	for _, t := range m.Taxonomy.Tags {
		slug := strings.TrimSpace(t.Slug)
		if slug == "" {
			continue
		}
		if dryRun {
			tagIDs[slug] = uuid.Nil
			continue
		}
		row, err := catSvc.CreateTag(ctx, catalogadmin.CreateTagInput{
			Slug:   slug,
			Name:   strings.TrimSpace(t.Name),
			Active: t.Active || true,
		})
		if err != nil {
			if errors.Is(err, catalogadmin.ErrDuplicateSlug) {
				id, lerr := lookup.TagIDBySlug(ctx, slug)
				if lerr != nil {
					return nil, nil, nil, lerr
				}
				tagIDs[slug] = id
				continue
			}
			return nil, nil, nil, fmt.Errorf("tag %s: %w", slug, err)
		}
		tagIDs[slug] = row.ID
	}
	return catIDs, brandIDs, tagIDs, nil
}

package api

import (
	"context"

	appsalecatalog "github.com/avf/avf-vending-api/internal/app/salecatalog"
	appmediaadmin "github.com/avf/avf-vending-api/internal/app/mediaadmin"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

type catalogMediaCacheBumper struct {
	inner appmediaadmin.CatalogMediaCacheBumper
	rdb   *goredis.Client
}

func wrapCatalogMediaCacheBumper(inner appmediaadmin.CatalogMediaCacheBumper, rdb *goredis.Client) appmediaadmin.CatalogMediaCacheBumper {
	if inner == nil && rdb == nil {
		return nil
	}
	return &catalogMediaCacheBumper{inner: inner, rdb: rdb}
}

func (w *catalogMediaCacheBumper) BumpCompanyMedia(ctx context.Context, companyID uuid.UUID) {
	if w == nil {
		return
	}
	if w.inner != nil {
		w.inner.BumpCompanyMedia(ctx, companyID)
	}
	if w.rdb != nil {
		appsalecatalog.InvalidateAllRedisSnapshotCaches(ctx, w.rdb)
	}
}

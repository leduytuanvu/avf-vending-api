package salecatalog

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/avf/avf-vending-api/internal/app/mediaadmin"
	"github.com/avf/avf-vending-api/internal/domain/org"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	goredis "github.com/redis/go-redis/v9"
)

func saleCatalogRedisCacheKey(machineID uuid.UUID, shadowVer, maxCfgRev, mediaEpoch int64, inclU, inclI bool) string {
	return fmt.Sprintf("avf:v1:cache:salecat:%s:%d:%d:%d:%t:%t",
		machineID.String(), shadowVer, maxCfgRev, mediaEpoch, inclU, inclI)
}

type saleCatalogHeadEntry struct {
	ShadowVer  int64    `json:"shadowVer"`
	MaxCfgRev  int64    `json:"maxCfgRev"`
	MediaEpoch int64    `json:"mediaEpoch"`
	Snap       Snapshot `json:"snap"`
}

var catalogCacheHits = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: "avf",
	Subsystem: "sale_catalog_cache",
	Name:      "hits_total",
	Help:      "Redis-backed sale catalog snapshot cache hits.",
})
var catalogCacheMiss = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: "avf",
	Subsystem: "sale_catalog_cache",
	Name:      "miss_total",
	Help:      "Redis-backed sale catalog snapshot cache misses (falls through to Postgres builder).",
})

// RedisCachedSnapshotBuilder wraps SnapshotBuilder with a short JSON cache keyed by machine shadow version,
// machine_configs.config_revision (planogram publish), org media epoch, and snapshot flags.
// Invalidation is implicit when shadow version, applied config revision, or media epoch bumps (TTL is a backstop).
type RedisCachedSnapshotBuilder struct {
	Inner           SnapshotBuilder
	Pool            *pgxpool.Pool
	RDB             *goredis.Client
	TTL             time.Duration
	MediaCompanyID  uuid.UUID
}

// BuildSnapshot implements SnapshotBuilder.
func (c *RedisCachedSnapshotBuilder) BuildSnapshot(ctx context.Context, machineID uuid.UUID, opts Options) (Snapshot, error) {
	if c == nil || c.Inner == nil {
		return Snapshot{}, fmt.Errorf("salecatalog: nil cached builder")
	}
	if c.RDB == nil || c.Pool == nil || c.TTL <= 0 {
		return c.Inner.BuildSnapshot(ctx, machineID, opts)
	}
	if opts.IfNoneMatchConfigVersion != nil {
		return c.Inner.BuildSnapshot(ctx, machineID, opts)
	}

	headKey := saleCatalogRedisHeadKey(machineID, opts.IncludeUnavailable, opts.IncludeImages)
	if raw, err := c.RDB.Get(ctx, headKey).Bytes(); err == nil && len(raw) > 0 {
		var head saleCatalogHeadEntry
		if json.Unmarshal(raw, &head) == nil && head.Snap.MachineID == machineID && !head.Snap.NotModified {
			shadowVer, maxCfgRev, mediaEpoch, verr := c.currentCacheVersions(ctx, machineID)
			if verr == nil && saleCatalogHeadMatchesVersions(head, shadowVer, maxCfgRev, mediaEpoch) {
				catalogCacheHits.Inc()
				return head.Snap, nil
			}
			_ = c.RDB.Del(ctx, headKey).Err()
		}
	}

	shadowVer, maxCfgRev, mediaEpoch, err := c.currentCacheVersions(ctx, machineID)
	if err != nil {
		if err != pgx.ErrNoRows {
			return Snapshot{}, err
		}
		shadowVer, maxCfgRev, mediaEpoch = 0, 0, 0
	}
	cacheKey := saleCatalogRedisCacheKey(machineID, shadowVer, maxCfgRev, mediaEpoch, opts.IncludeUnavailable, opts.IncludeImages)

	raw, err := c.RDB.Get(ctx, cacheKey).Bytes()
	if err == nil && len(raw) > 0 {
		var snap Snapshot
		if json.Unmarshal(raw, &snap) == nil && snap.MachineID == machineID {
			catalogCacheHits.Inc()
			c.storeHead(ctx, headKey, shadowVer, maxCfgRev, mediaEpoch, snap)
			return snap, nil
		}
	}
	catalogCacheMiss.Inc()

	snap, err := c.Inner.BuildSnapshot(ctx, machineID, opts)
	if err != nil {
		return snap, err
	}
	if !snap.NotModified {
		if b, mErr := json.Marshal(snap); mErr == nil {
			_ = c.RDB.Set(ctx, cacheKey, b, c.TTL).Err()
			c.storeHead(ctx, headKey, shadowVer, maxCfgRev, mediaEpoch, snap)
		}
	}
	return snap, nil
}

func (c *RedisCachedSnapshotBuilder) mediaCompanyID() uuid.UUID {
	if c == nil {
		return uuid.Nil
	}
	if c.MediaCompanyID != uuid.Nil {
		return c.MediaCompanyID
	}
	return org.DefaultCompanyID
}

func (c *RedisCachedSnapshotBuilder) currentCacheVersions(ctx context.Context, machineID uuid.UUID) (shadowVer, maxCfgRev, mediaEpoch int64, err error) {
	q := db.New(c.Pool)
	cfgVer, err := q.GetMachineShadowVersion(ctx, machineID)
	if err != nil {
		if err == pgx.ErrNoRows {
			cfgVer = 0
		} else {
			return 0, 0, 0, err
		}
	}
	shadowVer = cfgVer
	if err := c.Pool.QueryRow(ctx, `SELECT COALESCE(MAX(config_revision), 0) FROM machine_configs WHERE machine_id = $1`, machineID).Scan(&maxCfgRev); err != nil {
		return 0, 0, 0, err
	}
	mediaEpoch = mediaadmin.ReadMediaEpoch(ctx, c.RDB, c.mediaCompanyID())
	return shadowVer, maxCfgRev, mediaEpoch, nil
}

func (c *RedisCachedSnapshotBuilder) storeHead(ctx context.Context, headKey string, shadowVer, maxCfgRev, mediaEpoch int64, snap Snapshot) {
	if c == nil || c.RDB == nil || c.TTL <= 0 {
		return
	}
	head := saleCatalogHeadEntry{
		ShadowVer:  shadowVer,
		MaxCfgRev:  maxCfgRev,
		MediaEpoch: mediaEpoch,
		Snap:       snap,
	}
	if b, err := json.Marshal(head); err == nil {
		_ = c.RDB.Set(ctx, headKey, b, c.TTL).Err()
	}
}

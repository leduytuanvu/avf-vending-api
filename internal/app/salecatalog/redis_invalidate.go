package salecatalog

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

func saleCatalogHeadMatchesVersions(head saleCatalogHeadEntry, shadowVer, maxCfgRev, mediaEpoch int64) bool {
	return head.ShadowVer == shadowVer && head.MaxCfgRev == maxCfgRev && head.MediaEpoch == mediaEpoch
}

func saleCatalogRedisHeadKey(machineID uuid.UUID, inclU, inclI bool) string {
	return fmt.Sprintf("avf:v1:cache:salecat:head:%s:%t:%t", machineID.String(), inclU, inclI)
}

func saleCatalogRedisMachineScanPatterns(machineID uuid.UUID) []string {
	if machineID == uuid.Nil {
		return nil
	}
	id := machineID.String()
	return []string{
		fmt.Sprintf("avf:v1:cache:salecat:head:%s:*", id),
		fmt.Sprintf("avf:v1:cache:salecat:%s:*", id),
	}
}

func deleteRedisKeysByScan(ctx context.Context, rdb *goredis.Client, patterns []string) {
	if rdb == nil || len(patterns) == 0 {
		return
	}
	for _, pattern := range patterns {
		iter := rdb.Scan(ctx, 0, pattern, 128).Iterator()
		var batch []string
		for iter.Next(ctx) {
			batch = append(batch, iter.Val())
			if len(batch) >= 64 {
				_ = rdb.Del(ctx, batch...).Err()
				batch = nil
			}
		}
		if len(batch) > 0 {
			_ = rdb.Del(ctx, batch...).Err()
		}
		_ = iter.Err()
	}
}

// InvalidateRedisSnapshotCache drops head + versioned snapshot keys for a machine (call after slot/planogram/layout mutations).
func InvalidateRedisSnapshotCache(ctx context.Context, rdb *goredis.Client, machineID uuid.UUID) {
	if rdb == nil || machineID == uuid.Nil {
		return
	}
	keys := []string{
		saleCatalogRedisHeadKey(machineID, false, false),
		saleCatalogRedisHeadKey(machineID, false, true),
		saleCatalogRedisHeadKey(machineID, true, false),
		saleCatalogRedisHeadKey(machineID, true, true),
	}
	_ = rdb.Del(ctx, keys...).Err()
	deleteRedisKeysByScan(ctx, rdb, saleCatalogRedisMachineScanPatterns(machineID))
}

// InvalidateAllRedisSnapshotCaches drops every sale-catalog Redis entry (company media epoch bump).
func InvalidateAllRedisSnapshotCaches(ctx context.Context, rdb *goredis.Client) {
	deleteRedisKeysByScan(ctx, rdb, []string{"avf:v1:cache:salecat:*"})
}

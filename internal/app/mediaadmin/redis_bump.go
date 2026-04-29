package mediaadmin

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

const redisCatalogMediaEpochKeyPrefix = "avf:v1:catalog:media_epoch:"

// RedisCatalogMediaBumper increments a monotonic epoch per organization (optional machine-side cache hint).
type RedisCatalogMediaBumper struct {
	Client *goredis.Client
}

// NewRedisCatalogMediaBumper returns a bumper or nil if client is nil.
func NewRedisCatalogMediaBumper(c *goredis.Client) *RedisCatalogMediaBumper {
	if c == nil {
		return nil
	}
	return &RedisCatalogMediaBumper{Client: c}
}

func (r *RedisCatalogMediaBumper) BumpOrganizationMedia(ctx context.Context, organizationID uuid.UUID) {
	if r == nil || r.Client == nil || organizationID == uuid.Nil {
		return
	}
	_ = r.Client.Incr(ctx, redisCatalogMediaEpochKeyPrefix+organizationID.String())
}

// ReadMediaEpoch is a test/debug helper; returns 0 on miss or nil client.
func ReadMediaEpoch(ctx context.Context, c *goredis.Client, organizationID uuid.UUID) int64 {
	if c == nil || organizationID == uuid.Nil {
		return 0
	}
	n, err := c.Get(ctx, redisCatalogMediaEpochKeyPrefix+organizationID.String()).Int64()
	if err != nil {
		return 0
	}
	return n
}

// FormatMediaEpochHeader is a suggested HTTP response header value for edge caches (optional).
func FormatMediaEpochHeader(epoch int64) string {
	return fmt.Sprintf("%d", epoch)
}

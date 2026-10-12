package grpcserver

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

const orderStatusPSPRefreshCooldown = 2 * time.Second
const orderStatusPSPRefreshTimeout = 800 * time.Millisecond

var orderStatusPSPRefreshLast sync.Map // uuid.UUID -> time.Time

func shouldRunOrderStatusPSPRefresh(orderID uuid.UUID) bool {
	if orderID == uuid.Nil {
		return false
	}
	now := time.Now()
	if v, ok := orderStatusPSPRefreshLast.Load(orderID); ok {
		if now.Sub(v.(time.Time)) < orderStatusPSPRefreshCooldown {
			return false
		}
	}
	orderStatusPSPRefreshLast.Store(orderID, now)
	return true
}

func contextForOrderStatusPSPRefresh(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		return context.WithTimeout(context.Background(), orderStatusPSPRefreshTimeout)
	}
	if _, ok := parent.Deadline(); ok {
		return parent, func() {}
	}
	return context.WithTimeout(parent, orderStatusPSPRefreshTimeout)
}

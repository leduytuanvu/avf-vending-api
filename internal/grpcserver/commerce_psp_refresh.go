package grpcserver

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	orderStatusPSPRefreshGrace        = 3 * time.Second
	orderStatusPSPRefreshHotWindow    = 2 * time.Minute
	orderStatusPSPRefreshHotCooldown  = 5 * time.Second
	orderStatusPSPRefreshColdCooldown = 15 * time.Second
	orderStatusPSPRefreshTimeout      = 800 * time.Millisecond
)

var orderStatusPSPRefreshLast sync.Map // uuid.UUID -> time.Time

func shouldRunOrderStatusPSPRefresh(orderID uuid.UUID, oldestPendingCreatedAt, now time.Time) bool {
	if orderID == uuid.Nil {
		return false
	}
	if now.IsZero() {
		now = time.Now()
	}
	if !oldestPendingCreatedAt.IsZero() && now.Sub(oldestPendingCreatedAt) < orderStatusPSPRefreshGrace {
		return false
	}
	cooldown := orderStatusPSPRefreshColdCooldown
	if oldestPendingCreatedAt.IsZero() || now.Sub(oldestPendingCreatedAt) <= orderStatusPSPRefreshHotWindow {
		cooldown = orderStatusPSPRefreshHotCooldown
	}
	if v, ok := orderStatusPSPRefreshLast.Load(orderID); ok {
		if last, ok := v.(time.Time); ok && now.Sub(last) < cooldown {
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

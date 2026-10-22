package grpcserver

import (
	"context"
	"strings"
)

type ctxKey int

const (
	skipCredentialUsedTouchKey       ctxKey = 1
	machineCredentialGateSnapshotKey ctxKey = 2
)

type machineCredentialGateSnapshot struct {
	loaded            bool
	status            string
	credentialVersion int64
	credentialRevoked bool
}

func withSkipCredentialUsedTouch(ctx context.Context) context.Context {
	return context.WithValue(ctx, skipCredentialUsedTouchKey, true)
}

func withMachineCredentialGateSnapshotBox(ctx context.Context) context.Context {
	return context.WithValue(ctx, machineCredentialGateSnapshotKey, &machineCredentialGateSnapshot{})
}

func storeMachineCredentialGateSnapshot(ctx context.Context, snap machineCredentialGateSnapshot) {
	box, _ := ctx.Value(machineCredentialGateSnapshotKey).(*machineCredentialGateSnapshot)
	if box == nil {
		return
	}
	*box = snap
}

func machineCredentialGateSnapshotFromContext(ctx context.Context) (machineCredentialGateSnapshot, bool) {
	box, _ := ctx.Value(machineCredentialGateSnapshotKey).(*machineCredentialGateSnapshot)
	if box == nil || !box.loaded {
		return machineCredentialGateSnapshot{}, false
	}
	return *box, true
}

func skipCredentialUsedTouchFromContext(ctx context.Context) bool {
	v, ok := ctx.Value(skipCredentialUsedTouchKey).(bool)
	return ok && v
}

func skipCredentialUsedTouchForMethod(fullMethod string) bool {
	m := strings.ToLower(fullMethod)
	switch {
	case strings.Contains(m, "heartbeat"):
		return true
	case strings.Contains(m, "checkin"):
		return true
	case strings.Contains(m, "telemetry"):
		return true
	case strings.Contains(m, "pushtelemetry"):
		return true
	case strings.Contains(m, "createpaymentsession"):
		return true
	case strings.Contains(m, "attachpaymentresult"):
		return true
	default:
		return false
	}
}

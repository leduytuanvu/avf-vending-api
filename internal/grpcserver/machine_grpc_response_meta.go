package grpcserver

import (
	"context"
	"strings"
	"time"

	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// responseMetaCtx builds MachineResponseMeta with server_time, optional request echo, and trace_id from unary metadata.
func responseMetaCtx(ctx context.Context, requestID string, st machinev1.MachineResponseStatus) *machinev1.MachineResponseMeta {
	m := &machinev1.MachineResponseMeta{
		ServerTime: timestamppb.New(time.Now().UTC()),
		RequestId:  strings.TrimSpace(requestID),
		Status:     st,
	}
	if g, ok := GRPCRequestMetaFromContext(ctx); ok {
		tid := strings.TrimSpace(g.CorrelationID)
		if tid == "" {
			tid = strings.TrimSpace(g.RequestID)
		}
		m.TraceId = tid
	}
	return m
}

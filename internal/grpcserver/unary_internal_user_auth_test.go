package grpcserver

import (
	"context"
	"testing"

	"github.com/avf/avf-vending-api/internal/platform/auth"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type userJWTOkValidator struct{}

func (userJWTOkValidator) ValidateAccessToken(context.Context, string) (auth.Principal, error) {
	return auth.Principal{Subject: "user:fixture"}, nil
}

func TestUnaryInternalUserAuth_rejectsMachineNamespace(t *testing.T) {
	t.Parallel()

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer user.jwt"))
	meta := GRPCRequestMeta{RequestID: "r1", CorrelationID: "c1"}
	md, _ := metadata.FromIncomingContext(ctx)

	handler := func(context.Context, any) (any, error) {
		t.Fatal("handler must not run")
		return nil, nil
	}

	_, err := unaryInternalUserAuth(ctx, nil, &grpc.UnaryServerInfo{
		FullMethod: "/avf.machine.v1.MachineCommerceService/CreateOrder",
	}, handler, zap.NewNop(), userJWTOkValidator{}, meta, md)
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}
}

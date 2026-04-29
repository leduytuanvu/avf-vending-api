package grpcserver

import (
	"strings"
	"testing"

	internalv1 "github.com/avf/avf-vending-api/internal/gen/avfinternalv1"
	"google.golang.org/grpc"
)

func TestInternalGRPCServiceDescriptors_NoStreaming(t *testing.T) {
	t.Parallel()
	descs := []*grpc.ServiceDesc{
		&internalv1.InternalMachineQueryService_ServiceDesc,
		&internalv1.InternalTelemetryQueryService_ServiceDesc,
		&internalv1.InternalCommerceQueryService_ServiceDesc,
		&internalv1.InternalPaymentQueryService_ServiceDesc,
		&internalv1.InternalCatalogQueryService_ServiceDesc,
		&internalv1.InternalInventoryQueryService_ServiceDesc,
		&internalv1.InternalReportingQueryService_ServiceDesc,
	}
	for _, d := range descs {
		if d == nil {
			t.Fatal("nil service desc")
		}
		if len(d.Streams) != 0 {
			t.Fatalf("service %q exposes streaming handlers", d.ServiceName)
		}
	}
}

func TestInternalGRPCUnaryMethodNames_AreReadOnlyByConvention(t *testing.T) {
	t.Parallel()
	descs := []*grpc.ServiceDesc{
		&internalv1.InternalMachineQueryService_ServiceDesc,
		&internalv1.InternalTelemetryQueryService_ServiceDesc,
		&internalv1.InternalCommerceQueryService_ServiceDesc,
		&internalv1.InternalPaymentQueryService_ServiceDesc,
		&internalv1.InternalCatalogQueryService_ServiceDesc,
		&internalv1.InternalInventoryQueryService_ServiceDesc,
		&internalv1.InternalReportingQueryService_ServiceDesc,
	}
	for _, d := range descs {
		for _, m := range d.Methods {
			if !strings.HasPrefix(m.MethodName, "Get") {
				t.Fatalf("service %q method %q: expected Get* only (read-only internal contract)", d.ServiceName, m.MethodName)
			}
		}
	}
}

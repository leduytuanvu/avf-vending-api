package grpcserver

import (
	"errors"
	"fmt"
	"testing"

	"github.com/avf/avf-vending-api/internal/app/activation"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMapActivationErrorMQTTProvisioningUnavailable(t *testing.T) {
	t.Parallel()
	wrapped := fmt.Errorf("%w: emqxadmin: HTTP 401", activation.ErrMQTTProvisioning)
	got := mapActivationError(wrapped)
	st, ok := status.FromError(got)
	if !ok {
		t.Fatal("expected grpc status")
	}
	if st.Code() != codes.Unavailable {
		t.Fatalf("code=%s", st.Code())
	}
	if st.Message() != "mqtt_provisioning_failed" {
		t.Fatalf("message=%q", st.Message())
	}
	if !errors.Is(wrapped, activation.ErrMQTTProvisioning) {
		t.Fatal("wrapped error must remain errors.Is MQTT provisioning")
	}
}

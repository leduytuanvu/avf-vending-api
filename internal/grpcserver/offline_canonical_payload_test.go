package grpcserver

import (
	"testing"

	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestCanonicalAndroidCreateOrderJSONUnmarshals(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"context":{"idempotencyKey":"idem-1","clientEventId":"event-1"},"productId":"prod-9","currency":"VND","slot":{"slotIndex":3}}`)
	var req machinev1.CreateOrderRequest
	require.NoError(t, protojson.Unmarshal(raw, &req))
	require.Equal(t, "prod-9", req.GetProductId())
	require.Equal(t, "VND", req.GetCurrency())
	require.Equal(t, int32(3), req.GetSlot().GetSlotIndex())
	require.Equal(t, "idem-1", req.GetContext().GetIdempotencyKey())
}

func TestCanonicalAndroidPaymentSessionJSONUnmarshals(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"context":{"idempotencyKey":"idem-1","clientEventId":"event-1"},"orderId":"ord-1","provider":"CASH"}`)
	var req machinev1.CreatePaymentSessionRequest
	require.NoError(t, protojson.Unmarshal(raw, &req))
	require.Equal(t, "ord-1", req.GetOrderId())
	require.Equal(t, "CASH", req.GetProvider())
}

func TestCanonicalAndroidConfirmCashJSONUnmarshals(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"context":{"idempotencyKey":"idem-1","clientEventId":"event-1"},"orderId":"ord-1"}`)
	var req machinev1.ConfirmCashPaymentRequest
	require.NoError(t, protojson.Unmarshal(raw, &req))
	require.Equal(t, "ord-1", req.GetOrderId())
}

func TestCanonicalAndroidStartVendJSONUnmarshals(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"context":{"idempotencyKey":"idem-1","clientEventId":"event-1"},"orderId":"ord-1","slotIndex":3}`)
	var req machinev1.StartVendRequest
	require.NoError(t, protojson.Unmarshal(raw, &req))
	require.Equal(t, "ord-1", req.GetOrderId())
	require.Equal(t, int32(3), req.GetSlotIndex())
}

func TestCanonicalAndroidCancelOrderJSONUnmarshals(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"context":{"idempotencyKey":"idem-1","clientEventId":"event-1"},"orderId":"ord-1","reason":"customer_cancel"}`)
	var req machinev1.CancelOrderRequest
	require.NoError(t, protojson.Unmarshal(raw, &req))
	require.Equal(t, "ord-1", req.GetOrderId())
	require.Equal(t, "customer_cancel", req.GetReason())
}

func TestCanonicalAndroidConfirmVendSuccessJSONUnmarshals(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"context":{"idempotencyKey":"idem-1","clientEventId":"event-1"},"orderId":"ord-1","slotIndex":3}`)
	var req machinev1.ConfirmVendSuccessRequest
	require.NoError(t, protojson.Unmarshal(raw, &req))
	require.Equal(t, "ord-1", req.GetOrderId())
	require.Equal(t, int32(3), req.GetSlotIndex())
}

func TestCanonicalAndroidReportVendFailureJSONUnmarshals(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"context":{"idempotencyKey":"idem-1","clientEventId":"event-1"},"orderId":"ord-1","failureReason":"motor_jam","slotIndex":4}`)
	var req machinev1.ReportVendFailureRequest
	require.NoError(t, protojson.Unmarshal(raw, &req))
	require.Equal(t, "ord-1", req.GetOrderId())
	require.Equal(t, "motor_jam", req.GetFailureReason())
	require.Equal(t, int32(4), req.GetSlotIndex())
}

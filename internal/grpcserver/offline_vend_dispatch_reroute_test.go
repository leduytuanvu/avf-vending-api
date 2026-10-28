package grpcserver

import (
	"testing"

	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/stretchr/testify/require"
)

func TestVendConfirmRerouteFromOfflineMeta_successKey(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"orderId":"ord-1","slotIndex":10,"evidence":{"vendAttemptId":"va"}}`)
	meta := &machinev1.MachineRequestMeta{IdempotencyKey: "offline-vend-success:ord-1:10:1-recover-9"}
	require.Equal(t, "commerce.confirm_vend_success", vendConfirmRerouteFromOfflineMeta(payload, meta))
}

func TestVendConfirmRerouteFromOfflineMeta_failureStatus(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"orderId":"ord-1","status":"FAILED","canonical_operation":"commerce.confirm_vend_failure"}`)
	require.Equal(t, "commerce.confirm_vend_failure", vendConfirmRerouteFromOfflineMeta(payload, nil))
}

func TestVendConfirmRerouteFromOfflineMeta_trueStartVend(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"orderId":"ord-1","slotIndex":10,"canonical_operation":"commerce.start_vend"}`)
	meta := &machinev1.MachineRequestMeta{IdempotencyKey: "offline-vend-start:ord-1:10:1"}
	require.Equal(t, "", vendConfirmRerouteFromOfflineMeta(payload, meta))
}

func TestOfflineLedgerSkipRedispatch_failedTerminalReplays(t *testing.T) {
	t.Parallel()
	require.False(t, offlineLedgerSkipRedispatch("failed_terminal"))
	require.False(t, offlineLedgerSkipRedispatch("rejected"))
	require.True(t, offlineLedgerSkipRedispatch("processed"))
	require.True(t, offlineLedgerSkipRedispatch("replayed"))
}

func TestPushOfflineEventsIsNotOuterLedgerMutation(t *testing.T) {
	t.Parallel()
	require.False(t, isMachineIdempotentMutation(machinev1.MachineOfflineSyncService_PushOfflineEvents_FullMethodName))
	require.True(t, isMachineIdempotentMutation(machinev1.MachineCommerceService_StartVend_FullMethodName))
}

package grpcserver

import (
	"testing"

	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/stretchr/testify/require"
)

func TestSkipCredentialUsedTouchForMethod_CreatePaymentSession(t *testing.T) {
	t.Parallel()
	require.True(t, skipCredentialUsedTouchForMethod(machinev1.MachineCommerceService_CreatePaymentSession_FullMethodName))
	require.True(t, skipCredentialUsedTouchForMethod(machinev1.MachineCommerceService_AttachPaymentResult_FullMethodName))
	require.True(t, skipCredentialUsedTouchForMethod(machinev1.MachineRuntimeSessionService_HeartbeatRuntimeSession_FullMethodName))
	require.False(t, skipCredentialUsedTouchForMethod(machinev1.MachineCommerceService_CreateQuote_FullMethodName))
}

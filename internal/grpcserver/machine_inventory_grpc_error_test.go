package grpcserver

import (
	"errors"
	"testing"

	"github.com/avf/avf-vending-api/internal/app/inventoryapp"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMapMachineInventoryLedgerError(t *testing.T) {
	t.Parallel()
	require.Equal(t, codes.FailedPrecondition, status.Code(mapMachineInventoryLedgerError(inventoryapp.ErrQuantityBeforeMismatch)))
	require.Equal(t, codes.NotFound, status.Code(mapMachineInventoryLedgerError(inventoryapp.ErrAdjustmentSlotNotFound)))
	require.Equal(t, codes.InvalidArgument, status.Code(mapMachineInventoryLedgerError(inventoryapp.ErrInvalidStockAdjustmentReason)))
	require.Equal(t, codes.Aborted, status.Code(mapMachineInventoryLedgerError(inventoryapp.ErrIdempotencyKeyConflict)))
	require.Equal(t, codes.PermissionDenied, status.Code(mapMachineInventoryLedgerError(postgres.ErrMachineOrganizationMismatch)))
	require.Equal(t, codes.Internal, status.Code(mapMachineInventoryLedgerError(errors.New("other"))))
}

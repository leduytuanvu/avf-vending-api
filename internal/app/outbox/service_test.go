package outbox_test

import (
	"context"
	"testing"
	"time"

	appoutbox "github.com/avf/avf-vending-api/internal/app/outbox"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/stretchr/testify/require"
)

func TestAdminService_NilReceiver_ReturnsErrors(t *testing.T) {
	var svc *appoutbox.AdminService
	_, err := svc.ReplayDeadLetter(context.Background(), 1)
	require.Error(t, err)
	_, err = svc.MarkManualDLQ(context.Background(), 1, "x")
	require.Error(t, err)
	_, err = svc.ReplayDeadLetterTx(context.Background(), 1, nil, compliance.EnterpriseAuditRecord{})
	require.Error(t, err)
	_, err = svc.MarkManualDLQTx(context.Background(), 1, "x", nil, compliance.EnterpriseAuditRecord{})
	require.Error(t, err)
	_, err = svc.GetByID(context.Background(), 1)
	require.Error(t, err)
	_, err = svc.ListPendingWindow(context.Background(), time.Now(), time.Now().Add(time.Hour), "", 10)
	require.Error(t, err)
	_, err = svc.RequeuePendingByID(context.Background(), 1, "")
	require.Error(t, err)
	_, err = svc.ReplayDeadLetterAfterConfirm(context.Background(), 1, true)
	require.Error(t, err)
}

func TestAdminService_New_nilPool_ReturnsNil(t *testing.T) {
	require.Nil(t, appoutbox.NewAdminService(nil))
}

func TestRequirePoisonReplayConfirmation(t *testing.T) {
	t.Parallel()
	require.ErrorIs(t, appoutbox.RequirePoisonReplayConfirmation(false), appoutbox.ErrPoisonReplayRequiresConfirm)
	require.NoError(t, appoutbox.RequirePoisonReplayConfirmation(true))
}

func TestAdminService_ListPendingWindow_nilService(t *testing.T) {
	t.Parallel()
	var svc *appoutbox.AdminService
	_, err := svc.ListPendingWindow(context.Background(), time.Now(), time.Now().Add(time.Hour), "", 10)
	require.Error(t, err)
}

func TestAdminService_RequeuePendingByID_nilService(t *testing.T) {
	t.Parallel()
	var svc *appoutbox.AdminService
	_, err := svc.RequeuePendingByID(context.Background(), 1, "")
	require.Error(t, err)
}

func TestAdminService_ReplayDeadLetterAfterConfirm_false(t *testing.T) {
	t.Parallel()
	var svc *appoutbox.AdminService
	_, err := svc.ReplayDeadLetterAfterConfirm(context.Background(), 1, false)
	require.ErrorIs(t, err, appoutbox.ErrPoisonReplayRequiresConfirm)
}

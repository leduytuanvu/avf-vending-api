package background

import (
	"context"
	"testing"
	"time"

	appreliability "github.com/avf/avf-vending-api/internal/app/reliability"
	"github.com/avf/avf-vending-api/internal/app/workfloworch"
	"github.com/google/uuid"
)

type stubWorkflowBoundary struct {
	starts []workfloworch.StartInput
}

func (s *stubWorkflowBoundary) Enabled() bool { return true }

func (s *stubWorkflowBoundary) Start(_ context.Context, in workfloworch.StartInput) error {
	s.starts = append(s.starts, in)
	return nil
}

func (s *stubWorkflowBoundary) Close() error { return nil }

type stubStuckPaymentFinder struct {
	rows []appreliability.StuckPaymentCandidate
}

func (s stubStuckPaymentFinder) FindStuckPayments(context.Context, appreliability.ScanRunContext, appreliability.RecoveryPolicy, appreliability.ScanLimits) ([]appreliability.StuckPaymentCandidate, error) {
	return s.rows, nil
}

func TestPaymentTimeoutScanTick_SchedulesWorkflowWhenEnabled(t *testing.T) {
	t.Parallel()
	paymentID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	orderID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	boundary := &stubWorkflowBoundary{}
	svc := appreliability.NewService(appreliability.Deps{
		Payments: stubStuckPaymentFinder{
			rows: []appreliability.StuckPaymentCandidate{{
				PaymentID: paymentID,
				OrderID:   orderID,
				State:     "created",
				CreatedAt: time.Now().UTC().Add(-time.Hour),
			}},
		},
	})
	err := PaymentTimeoutScanTick(context.Background(), WorkerDeps{
		Reliability:                   svc,
		Policy:                        appreliability.NormalizeRecoveryPolicy(appreliability.RecoveryPolicy{}),
		Limits:                        appreliability.ScanLimits{MaxItems: 10},
		WorkflowOrchestration:         boundary,
		SchedulePaymentPendingTimeout: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(boundary.starts) != 1 {
		t.Fatalf("starts=%d", len(boundary.starts))
	}
	if boundary.starts[0].Kind != workfloworch.KindPaymentPendingTimeoutFollowUp {
		t.Fatalf("kind=%q", boundary.starts[0].Kind)
	}
}

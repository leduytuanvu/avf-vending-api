package background

import (
	"context"
	"fmt"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"go.uber.org/zap"
)

// FinancialCorrectnessReconcileTick opens reconciliation cases for unclaimed captures,
// cash gross mismatches, and unresolved change liability.
func FinancialCorrectnessReconcileTick(ctx context.Context, deps ReconcilerDeps) error {
	if deps.ObservabilityPool == nil {
		return nil
	}
	q := db.New(deps.ObservabilityPool)
	lim := deps.limits()
	before := deps.beforeBoundary()

	unclaimed, err := q.ListCapturedPaymentsWithoutWinner(ctx, db.ListCapturedPaymentsWithoutWinnerParams{
		CreatedAt: before,
		Limit:     lim,
	})
	if err != nil {
		return fmt.Errorf("financial_correctness: list unclaimed captures: %w", err)
	}
	for _, pay := range unclaimed {
		if deps.CaseWriter == nil {
			continue
		}
		_, _ = deps.CaseWriter.UpsertReconciliationCase(ctx, domaincommerce.ReconciliationCaseInput{
			CaseType:       "unclaimed_capture",
			Severity:       "warning",
			OrderID:        &pay.OrderID,
			PaymentID:      &pay.ID,
			CorrelationKey: fmt.Sprintf("unclaimed_capture:%s", pay.ID.String()),
			Reason:         "captured payment without winning_payment_id claim",
			Metadata:       reconciliationCaseMetadata(map[string]any{"payment_id": pay.ID.String()}),
		})
		recordCommerceReconciliationCase("unclaimed_capture")
	}

	mismatches, err := q.ListOrdersWithCashGrossMismatch(ctx, db.ListOrdersWithCashGrossMismatchParams{
		UpdatedAt: before,
		Limit:     lim,
	})
	if err != nil {
		return fmt.Errorf("financial_correctness: list gross mismatch: %w", err)
	}
	for _, orderID := range mismatches {
		if deps.CaseWriter == nil {
			continue
		}
		oid := orderID
		_, _ = deps.CaseWriter.UpsertReconciliationCase(ctx, domaincommerce.ReconciliationCaseInput{
			CaseType:       "cash_gross_mismatch",
			Severity:       "critical",
			OrderID:        &oid,
			CorrelationKey: fmt.Sprintf("cash_gross_mismatch:%s", orderID.String()),
			Reason:         "allocated + change + liability does not reconcile to gross accepted",
			Metadata:       reconciliationCaseMetadata(map[string]any{"order_id": orderID.String()}),
		})
		recordCommerceReconciliationCase("cash_gross_mismatch")
	}

	liabilities, err := q.ListUnresolvedChangeLiability(ctx, db.ListUnresolvedChangeLiabilityParams{
		CreatedAt: before,
		Limit:     lim,
	})
	if err != nil {
		return fmt.Errorf("financial_correctness: list change liability: %w", err)
	}
	for _, chg := range liabilities {
		if deps.CaseWriter == nil {
			continue
		}
		orderID := chg.OrderID
		_, _ = deps.CaseWriter.UpsertReconciliationCase(ctx, domaincommerce.ReconciliationCaseInput{
			CaseType:       "change_liability_unresolved",
			Severity:       "warning",
			OrderID:        &orderID,
			CorrelationKey: fmt.Sprintf("change_liability:%s", chg.ID.String()),
			Reason:         "change liability remains unresolved",
			Metadata: reconciliationCaseMetadata(map[string]any{
				"order_id":        orderID.String(),
				"liability_minor": chg.LiabilityMinor,
				"change_outcome":  chg.Outcome,
			}),
		})
		recordCommerceReconciliationCase("change_liability_unresolved")
	}

	selected := len(unclaimed) + len(mismatches) + len(liabilities)
	atLimit := int32(len(unclaimed)) >= lim || int32(len(mismatches)) >= lim || int32(len(liabilities)) >= lim
	if deps.Log != nil {
		deps.Log.Info("reconciler_job_summary",
			zap.String("job", "financial_correctness"),
			zap.Int("selected", selected),
			zap.Int("unclaimed_captures", len(unclaimed)),
			zap.Int("gross_mismatch", len(mismatches)),
			zap.Int("change_liability", len(liabilities)),
			zap.Bool("at_batch_limit", atLimit),
		)
	}
	if deps.Telemetry != nil {
		deps.Telemetry.JobSummary("financial_correctness", selected, 0, atLimit)
	}
	return nil
}

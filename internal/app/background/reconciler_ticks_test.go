package background

import (
	"context"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/workfloworch"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	domainreliability "github.com/avf/avf-vending-api/internal/domain/reliability"
	"github.com/google/uuid"
)

type stubReconReader struct {
	pendingTimeout []domaincommerce.Payment
	refundReview   []domaincommerce.Payment
	duplicates     []domaincommerce.Payment
	paidNoVend     []domaincommerce.PaidOrderVendStartCandidate
	paidFailures   []domaincommerce.PaidVendFailureCandidate
	pendingRefunds []domaincommerce.RefundPendingCandidate
}

func (s *stubReconReader) ListPaymentsPendingTimeout(ctx context.Context, before time.Time, limit int32) ([]domaincommerce.Payment, error) {
	_ = ctx
	_ = before
	if limit > 0 && int32(len(s.pendingTimeout)) > limit {
		return s.pendingTimeout[:limit], nil
	}
	return append([]domaincommerce.Payment(nil), s.pendingTimeout...), nil
}

func (s *stubReconReader) ListOrdersWithUnresolvedPayment(ctx context.Context, before time.Time, limit int32) ([]domaincommerce.Order, error) {
	_ = ctx
	_ = before
	_ = limit
	return nil, nil
}

func (s *stubReconReader) ListVendSessionsStuckForReconciliation(ctx context.Context, before time.Time, limit int32) ([]domaincommerce.VendReconciliationCandidate, error) {
	_ = ctx
	_ = before
	_ = limit
	return nil, nil
}

func (s *stubReconReader) ListPotentialDuplicatePayments(ctx context.Context, before time.Time, limit int32) ([]domaincommerce.Payment, error) {
	_ = ctx
	_ = before
	if limit > 0 && int32(len(s.duplicates)) > limit {
		return s.duplicates[:limit], nil
	}
	return append([]domaincommerce.Payment(nil), s.duplicates...), nil
}

func (s *stubReconReader) ListPaymentsForRefundReview(ctx context.Context, before time.Time, limit int32) ([]domaincommerce.Payment, error) {
	_ = ctx
	_ = before
	if limit > 0 && int32(len(s.refundReview)) > limit {
		return s.refundReview[:limit], nil
	}
	return append([]domaincommerce.Payment(nil), s.refundReview...), nil
}

func (s *stubReconReader) ListStaleCommandLedgerEntries(ctx context.Context, before time.Time, limit int32) ([]domaincommerce.CommandLedgerSummary, error) {
	_ = ctx
	_ = before
	_ = limit
	return nil, nil
}

func (s *stubReconReader) ListPaidOrdersWithoutVendStart(ctx context.Context, before time.Time, limit int32) ([]domaincommerce.PaidOrderVendStartCandidate, error) {
	_ = ctx
	_ = before
	_ = limit
	return append([]domaincommerce.PaidOrderVendStartCandidate(nil), s.paidNoVend...), nil
}

func (s *stubReconReader) ListPaidVendFailuresForReview(ctx context.Context, before time.Time, limit int32) ([]domaincommerce.PaidVendFailureCandidate, error) {
	_ = ctx
	_ = before
	_ = limit
	return append([]domaincommerce.PaidVendFailureCandidate(nil), s.paidFailures...), nil
}

func (s *stubReconReader) ListRefundsPendingTooLong(ctx context.Context, before time.Time, limit int32) ([]domaincommerce.RefundPendingCandidate, error) {
	_ = ctx
	_ = before
	_ = limit
	return append([]domaincommerce.RefundPendingCandidate(nil), s.pendingRefunds...), nil
}

type stubPaymentGateway struct {
	normalized string
	err        error
}

func (g *stubPaymentGateway) FetchPaymentStatus(ctx context.Context, lookup domaincommerce.PaymentProviderLookup) (domaincommerce.PaymentStatusSnapshot, error) {
	_ = ctx
	_ = lookup
	if g.err != nil {
		return domaincommerce.PaymentStatusSnapshot{}, g.err
	}
	return domaincommerce.PaymentStatusSnapshot{
		NormalizedState: g.normalized,
		ProviderHint:    []byte(`{"stub":"gateway"}`),
	}, nil
}

type stubPaymentApplier struct {
	calls []domaincommerce.ReconciledPaymentTransitionInput
	// OrderByPayment optionally maps payment id -> order id for returned Payment rows.
	OrderByPayment map[uuid.UUID]uuid.UUID
}

func (a *stubPaymentApplier) ApplyReconciledPaymentTransition(ctx context.Context, in domaincommerce.ReconciledPaymentTransitionInput) (domaincommerce.Payment, error) {
	_ = ctx
	a.calls = append(a.calls, in)
	oid := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	if a.OrderByPayment != nil {
		if v, ok := a.OrderByPayment[in.PaymentID]; ok {
			oid = v
		}
	}
	out := domaincommerce.Payment{
		ID:      in.PaymentID,
		OrderID: oid,
		State:   in.ToState,
	}
	if in.DryRun {
		out.State = "created"
	}
	return out, nil
}

type stubOrderReader struct {
	org uuid.UUID
}

func (o stubOrderReader) GetOrderByID(ctx context.Context, id uuid.UUID) (domaincommerce.Order, error) {
	_ = ctx
	return domaincommerce.Order{
		ID:             id,
		OrganizationID: o.org,
		Status:         "failed",
	}, nil
}

type stubRefundSink struct {
	tickets []domaincommerce.RefundReviewTicket
}

func (s *stubRefundSink) EnqueueRefundReview(ctx context.Context, ticket domaincommerce.RefundReviewTicket) error {
	_ = ctx
	s.tickets = append(s.tickets, ticket)
	return nil
}

type stubCaseWriter struct {
	cases []domaincommerce.ReconciliationCaseInput
}

func (s *stubCaseWriter) UpsertReconciliationCase(ctx context.Context, in domaincommerce.ReconciliationCaseInput) (domaincommerce.ReconciliationCase, error) {
	_ = ctx
	s.cases = append(s.cases, in)
	return domaincommerce.ReconciliationCase{ID: uuid.New(), OrganizationID: in.OrganizationID, CaseType: in.CaseType}, nil
}

func TestPaymentProviderReconcileTick_settledAfterProbe(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	oid := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	reader := &stubReconReader{
		pendingTimeout: []domaincommerce.Payment{{
			ID: pid, OrderID: oid, Provider: "psp", State: "created",
		}},
	}
	gw := &stubPaymentGateway{normalized: "settled"}
	ap := &stubPaymentApplier{}
	ctx := context.Background()
	deps := ReconcilerDeps{
		Reader:                     reader,
		Gateway:                    gw,
		PaymentApplier:             ap,
		ActionsEnabled:             true,
		DryRun:                     false,
		StableAge:                  time.Minute,
		Limits:                     10,
		PaymentOutboxTopic:         "commerce.payments",
		PaymentOutboxAggregateType: "payment",
	}
	if err := PaymentProviderReconcileTick(ctx, deps); err != nil {
		t.Fatal(err)
	}
	if len(ap.calls) != 1 {
		t.Fatalf("applier calls: %d", len(ap.calls))
	}
	if ap.calls[0].ToState != "captured" || ap.calls[0].DryRun {
		t.Fatalf("unexpected apply input: %+v", ap.calls[0])
	}
	if ap.calls[0].OutboxEventType != domainreliability.OutboxEventPaymentConfirmed ||
		ap.calls[0].OutboxTopic != "commerce.payments" ||
		ap.calls[0].OutboxAggregateType != "payment" ||
		ap.calls[0].OutboxAggregateID != pid ||
		ap.calls[0].OutboxIdempotencyKey == "" {
		t.Fatalf("missing reconciler outbox metadata: %+v", ap.calls[0])
	}
}

func TestPaymentProviderReconcileTick_failedAfterProbe(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	oid := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	reader := &stubReconReader{
		pendingTimeout: []domaincommerce.Payment{{
			ID: pid, OrderID: oid, Provider: "psp", State: "authorized",
		}},
	}
	gw := &stubPaymentGateway{normalized: "declined"}
	ap := &stubPaymentApplier{}
	ctx := context.Background()
	deps := ReconcilerDeps{
		Reader:         reader,
		Gateway:        gw,
		PaymentApplier: ap,
		ActionsEnabled: true,
		DryRun:         false,
		StableAge:      time.Minute,
		Limits:         10,
	}
	if err := PaymentProviderReconcileTick(ctx, deps); err != nil {
		t.Fatal(err)
	}
	if len(ap.calls) != 1 || ap.calls[0].ToState != "failed" {
		t.Fatalf("unexpected apply: %+v", ap.calls)
	}
}

func TestPaymentProviderReconcileTick_dryRunDoesNotApplyTerminalState(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
	oid := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	reader := &stubReconReader{
		pendingTimeout: []domaincommerce.Payment{{
			ID: pid, OrderID: oid, Provider: "psp", State: "created",
		}},
	}
	gw := &stubPaymentGateway{normalized: "paid"}
	ap := &stubPaymentApplier{}
	ctx := context.Background()
	deps := ReconcilerDeps{
		Reader:         reader,
		Gateway:        gw,
		PaymentApplier: ap,
		ActionsEnabled: true,
		DryRun:         true,
		StableAge:      time.Minute,
		Limits:         10,
	}
	if err := PaymentProviderReconcileTick(ctx, deps); err != nil {
		t.Fatal(err)
	}
	if len(ap.calls) != 1 || !ap.calls[0].DryRun {
		t.Fatalf("expected dry_run apply, got %+v", ap.calls)
	}
}

func TestRefundReviewDecisionTick_routesToSink(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("12121212-1212-1212-1212-121212121212")
	oid := uuid.MustParse("34343434-3434-3434-3434-343434343434")
	org := uuid.MustParse("56565656-5656-5656-5656-565656565656")
	reader := &stubReconReader{
		refundReview: []domaincommerce.Payment{{
			ID: pid, OrderID: oid, Provider: "psp", State: "captured",
		}},
	}
	sink := &stubRefundSink{}
	ctx := context.Background()
	deps := ReconcilerDeps{
		Reader:         reader,
		OrderRead:      stubOrderReader{org: org},
		RefundSink:     sink,
		ActionsEnabled: true,
		DryRun:         false,
		StableAge:      time.Minute,
		Limits:         10,
	}
	if err := RefundReviewDecisionTick(ctx, deps); err != nil {
		t.Fatal(err)
	}
	if len(sink.tickets) != 1 {
		t.Fatalf("tickets: %+v", sink.tickets)
	}
	tk := sink.tickets[0]
	if tk.PaymentID != pid || tk.OrderID != oid || tk.OrganizationID != org || tk.Reason != "captured_payment_failed_order" {
		t.Fatalf("unexpected ticket: %+v", tk)
	}
}

func TestDuplicatePaymentRecoveryTick_routesToSink(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("abababab-abab-abab-abab-abababababab")
	oid := uuid.MustParse("cdcdcdcd-cdcd-cdcd-cdcd-cdcdcdcdcdcd")
	org := uuid.MustParse("efefefef-efef-efef-efef-efefefefefef")
	reader := &stubReconReader{
		duplicates: []domaincommerce.Payment{{
			ID: pid, OrderID: oid, Provider: "psp", State: "captured",
		}},
	}
	sink := &stubRefundSink{}
	ctx := context.Background()
	deps := ReconcilerDeps{
		Reader:         reader,
		OrderRead:      stubOrderReader{org: org},
		RefundSink:     sink,
		ActionsEnabled: true,
		DryRun:         false,
		StableAge:      time.Minute,
		Limits:         10,
	}
	if err := DuplicatePaymentRecoveryTick(ctx, deps); err != nil {
		t.Fatal(err)
	}
	if len(sink.tickets) != 1 || sink.tickets[0].Reason != "potential_duplicate_payment" {
		t.Fatalf("unexpected tickets: %+v", sink.tickets)
	}
}

type stubMarkOrderPaid struct {
	calls int
}

func (s *stubMarkOrderPaid) MarkOrderPaidAfterPaymentCapture(ctx context.Context, organizationID, orderID uuid.UUID) (domaincommerce.Order, error) {
	_ = ctx
	_ = organizationID
	_ = orderID
	s.calls++
	return domaincommerce.Order{}, nil
}

func TestPaymentProviderReconcileTick_captureCallsMarkOrderPaid(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("13131313-1313-1313-1313-131313131313")
	oid := uuid.MustParse("24242424-2424-2424-2424-242424242424")
	org := uuid.MustParse("35353535-3535-3535-3535-353535353535")
	reader := &stubReconReader{
		pendingTimeout: []domaincommerce.Payment{{
			ID: pid, OrderID: oid, Provider: "psp", State: "created",
		}},
	}
	gw := &stubPaymentGateway{normalized: "captured"}
	ap := &stubPaymentApplier{OrderByPayment: map[uuid.UUID]uuid.UUID{pid: oid}}
	mp := &stubMarkOrderPaid{}
	ctx := context.Background()
	deps := ReconcilerDeps{
		Reader:         reader,
		Gateway:        gw,
		PaymentApplier: ap,
		MarkOrderPaid:  mp,
		OrderRead:      stubOrderReader{org: org},
		ActionsEnabled: true,
		DryRun:         false,
		StableAge:      time.Minute,
		Limits:         10,
	}
	if err := PaymentProviderReconcileTick(ctx, deps); err != nil {
		t.Fatal(err)
	}
	if mp.calls != 1 {
		t.Fatalf("mark paid calls: %d", mp.calls)
	}
}

func TestRefundReviewDecisionTick_actionsEnabledMissingSinkErrors(t *testing.T) {
	t.Parallel()
	reader := &stubReconReader{
		refundReview: []domaincommerce.Payment{{ID: uuid.New(), OrderID: uuid.New()}},
	}
	ctx := context.Background()
	deps := ReconcilerDeps{
		Reader:         reader,
		OrderRead:      stubOrderReader{org: uuid.New()},
		RefundSink:     nil,
		ActionsEnabled: true,
		StableAge:      time.Minute,
		Limits:         10,
	}
	if err := RefundReviewDecisionTick(ctx, deps); err == nil {
		t.Fatal("expected error when actions enabled but refund sink nil")
	}
}

func TestRefundReviewDecisionTick_schedulesWorkflowWhenEnabled(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("91919191-9191-9191-9191-919191919191")
	oid := uuid.MustParse("92929292-9292-9292-9292-929292929292")
	org := uuid.MustParse("93939393-9393-9393-9393-939393939393")
	reader := &stubReconReader{
		refundReview: []domaincommerce.Payment{{ID: pid, OrderID: oid, State: "captured"}},
	}
	wf := &stubWorkflowBoundary{}
	err := RefundReviewDecisionTick(context.Background(), ReconcilerDeps{
		Reader:                      reader,
		OrderRead:                   stubOrderReader{org: org},
		WorkflowOrchestration:       wf,
		ScheduleRefundOrchestration: true,
		StableAge:                   time.Minute,
		Limits:                      10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(wf.starts) != 1 || wf.starts[0].Kind != workfloworch.KindRefundOrchestration {
		t.Fatalf("starts=%+v", wf.starts)
	}
}

func TestDuplicatePaymentRecoveryTick_schedulesWorkflowWhenEnabled(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("94949494-9494-9494-9494-949494949494")
	oid := uuid.MustParse("95959595-9595-9595-9595-959595959595")
	org := uuid.MustParse("96969696-9696-9696-9696-969696969696")
	reader := &stubReconReader{
		duplicates: []domaincommerce.Payment{{ID: pid, OrderID: oid, State: "captured"}},
	}
	wf := &stubWorkflowBoundary{}
	err := DuplicatePaymentRecoveryTick(context.Background(), ReconcilerDeps{
		Reader:                         reader,
		OrderRead:                      stubOrderReader{org: org},
		WorkflowOrchestration:          wf,
		ScheduleManualReviewEscalation: true,
		StableAge:                      time.Minute,
		Limits:                         10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(wf.starts) != 1 || wf.starts[0].Kind != workfloworch.KindManualReviewEscalation {
		t.Fatalf("starts=%+v", wf.starts)
	}
}

func TestUnresolvedOrdersTick_paidNoVendStartCreatesCase(t *testing.T) {
	t.Parallel()
	org := uuid.New()
	orderID := uuid.New()
	paymentID := uuid.New()
	vendID := uuid.New()
	cases := &stubCaseWriter{}
	reader := &stubReconReader{
		paidNoVend: []domaincommerce.PaidOrderVendStartCandidate{{
			OrderID:        orderID,
			OrganizationID: org,
			PaymentID:      paymentID,
			VendSessionID:  vendID,
			Provider:       "psp",
			PaymentState:   "captured",
			VendState:      "pending",
			MachineID:      uuid.New(),
		}},
	}
	if err := UnresolvedOrdersTick(context.Background(), ReconcilerDeps{Reader: reader, CaseWriter: cases, StableAge: time.Minute, Limits: 10}); err != nil {
		t.Fatal(err)
	}
	if len(cases.cases) != 1 || cases.cases[0].CaseType != "payment_paid_vend_not_started" {
		t.Fatalf("cases=%+v", cases.cases)
	}
}

func TestP06_Reconciler_VendTimeoutPaidVendFailedCreatesReviewCase(t *testing.T) {
	t.Parallel()
	org := uuid.New()
	cases := &stubCaseWriter{}
	reader := &stubReconReader{
		paidFailures: []domaincommerce.PaidVendFailureCandidate{{
			OrderID:        uuid.New(),
			OrganizationID: org,
			PaymentID:      uuid.New(),
			VendSessionID:  uuid.New(),
			Provider:       "psp",
			PaymentState:   "captured",
			VendState:      "failed",
			MachineID:      uuid.New(),
		}},
	}
	if err := VendTimeoutReconcileTick(context.Background(), ReconcilerDeps{Reader: reader, CaseWriter: cases, StableAge: time.Minute, Limits: 10}); err != nil {
		t.Fatal(err)
	}
	if len(cases.cases) != 1 || cases.cases[0].CaseType != "payment_paid_vend_failed" {
		t.Fatalf("cases=%+v", cases.cases)
	}
}

func TestRefundReviewDecisionTick_pendingRefundCreatesCase(t *testing.T) {
	t.Parallel()
	org := uuid.New()
	cases := &stubCaseWriter{}
	reader := &stubReconReader{
		pendingRefunds: []domaincommerce.RefundPendingCandidate{{
			RefundID:       uuid.New(),
			PaymentID:      uuid.New(),
			OrderID:        uuid.New(),
			OrganizationID: org,
			Provider:       "psp",
			RefundState:    "processing",
			AmountMinor:    100,
			Currency:       "USD",
		}},
	}
	if err := RefundReviewDecisionTick(context.Background(), ReconcilerDeps{Reader: reader, CaseWriter: cases, StableAge: time.Minute, Limits: 10}); err != nil {
		t.Fatal(err)
	}
	if len(cases.cases) != 1 || cases.cases[0].CaseType != "refund_pending_too_long" {
		t.Fatalf("cases=%+v", cases.cases)
	}
}

func TestDuplicatePaymentRecoveryTick_safeConcurrentCaseUpserts(t *testing.T) {
	t.Parallel()
	org := uuid.New()
	reader := &stubReconReader{duplicates: []domaincommerce.Payment{{ID: uuid.New(), OrderID: uuid.New(), State: "captured", AmountMinor: 100, Currency: "USD"}}}
	run := func() error {
		return DuplicatePaymentRecoveryTick(context.Background(), ReconcilerDeps{
			Reader:     reader,
			CaseWriter: &stubCaseWriter{},
			OrderRead:  stubOrderReader{org: org},
			StableAge:  time.Minute,
			Limits:     10,
		})
	}
	errc := make(chan error, 2)
	go func() { errc <- run() }()
	go func() { errc <- run() }()
	for i := 0; i < 2; i++ {
		if err := <-errc; err != nil {
			t.Fatal(err)
		}
	}
}

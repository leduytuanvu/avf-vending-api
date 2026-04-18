package workfloworch

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestTemporalBoundary_Start_NotImplemented(t *testing.T) {
	t.Parallel()
	// White-box: Start does not invoke the SDK client yet; nil client is safe for this assertion.
	b := &temporalBoundary{c: nil, taskQueue: "avf-q"}
	err := b.Start(context.Background(), StartInput{Kind: KindHumanReviewEscalation})
	if !errors.Is(err, ErrWorkflowNotImplemented) {
		t.Fatalf("got %v want ErrWorkflowNotImplemented", err)
	}
	if !strings.Contains(err.Error(), string(KindHumanReviewEscalation)) {
		t.Fatalf("expected kind in error: %v", err)
	}
}

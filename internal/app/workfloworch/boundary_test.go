package workfloworch

import (
	"context"
	"errors"
	"testing"
)

func TestNewDisabled(t *testing.T) {
	t.Parallel()
	b := NewDisabled()
	if b.Enabled() {
		t.Fatal("expected disabled")
	}
	err := b.Start(context.Background(), StartInput{Kind: KindReconciliationFollowUp})
	if !errors.Is(err, ErrDisabled) {
		t.Fatalf("got %v want ErrDisabled", err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewTemporal_EmptyQueue(t *testing.T) {
	t.Parallel()
	_, err := NewTemporal(nil, "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewTemporal_NilClient(t *testing.T) {
	t.Parallel()
	_, err := NewTemporal(nil, "avf-workflows")
	if err == nil {
		t.Fatal("expected error")
	}
}

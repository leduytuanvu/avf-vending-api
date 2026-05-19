package artifacts

import (
	"context"
	"testing"

	"github.com/avf/avf-vending-api/internal/testfixtures"
)

func TestReserveArtifactReturnsUUIDV7(t *testing.T) {
	svc := NewService(Deps{Store: &stubStore{}})
	got, err := svc.ReserveArtifact(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	testfixtures.AssertResourceUUIDV7(t, got)
}

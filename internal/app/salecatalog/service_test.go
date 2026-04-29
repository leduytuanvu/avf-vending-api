package salecatalog

import (
	"testing"
)

func TestNewService_PanicsOnNilPool(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = NewService(nil)
}

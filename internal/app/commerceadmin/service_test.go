//go:build !windows

package commerceadmin

import "testing"

func TestNewService_nilQueries(t *testing.T) {
	_, err := NewService(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

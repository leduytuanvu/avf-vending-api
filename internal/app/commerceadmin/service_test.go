//go:build !windows

package commerceadmin

import "testing"

func TestNewService_nilDeps(t *testing.T) {
	_, err := NewService(nil, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

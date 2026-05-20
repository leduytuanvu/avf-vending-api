package auth

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/platform/id"
	"github.com/avf/avf-vending-api/internal/testfixtures"
)

func TestNewRefreshTokenRowIDIsUUIDV7(t *testing.T) {
	testfixtures.AssertResourceUUIDV7(t, id.NewUUIDV7())
}

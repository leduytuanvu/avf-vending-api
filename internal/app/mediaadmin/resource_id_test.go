package mediaadmin

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/platform/id"
	"github.com/avf/avf-vending-api/internal/testfixtures"
)

func TestNewMediaAssetIDIsUUIDV7(t *testing.T) {
	testfixtures.AssertResourceUUIDV7(t, id.NewUUIDV7())
}

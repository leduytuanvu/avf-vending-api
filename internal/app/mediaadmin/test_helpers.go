package mediaadmin

import (
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/google/uuid"
)

// TestServiceWithMediaCompanyID constructs a minimal Service for handler unit tests.
func TestServiceWithMediaCompanyID(companyID uuid.UUID) *Service {
	return &Service{uploadCfg: config.MediaUploadConfig{CompanyID: companyID}}
}

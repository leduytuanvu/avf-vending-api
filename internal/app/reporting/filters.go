package reporting

import (
	"github.com/avf/avf-vending-api/internal/app/listscope"
	"github.com/google/uuid"
)

func reportingFiltersActive(q listscope.ReportingQuery) bool {
	return q.SiteIDFilter != uuid.Nil || q.MachineIDFilter != uuid.Nil || q.ProductIDFilter != uuid.Nil
}

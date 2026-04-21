package inventoryadmin

import "testing"

func TestDefaultAdminCabinetCodeMatchesSQLFallback(t *testing.T) {
	t.Parallel()
	if defaultAdminCabinetCode != "CAB-A" {
		t.Fatalf(
			"defaultAdminCabinetCode must stay aligned with db/queries/inventory_admin.sql coalesce(..., 'CAB-A'); got %q",
			defaultAdminCabinetCode,
		)
	}
}

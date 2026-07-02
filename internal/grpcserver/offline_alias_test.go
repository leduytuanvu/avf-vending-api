package grpcserver

import "testing"

func TestMapOfflineEventAlias(t *testing.T) {
	cases := map[string]string{
		"order_created_offline":       "commerce.create_order",
		"cash_accepted":               "commerce.confirm_cash_payment",
		"vend_success":                "commerce.confirm_vend_success",
		"inventory_decrement_pending": "inventory.report_delta",
		"hardware_error":              "telemetry.critical",
		"commerce.create_order":       "commerce.create_order",
	}
	for in, want := range cases {
		if got := mapOfflineEventAlias(in); got != want {
			t.Fatalf("%s => %s, want %s", in, got, want)
		}
	}
}

package payments

import "testing"

func TestZalopayAppTransIDFromPayload(t *testing.T) {
	t.Run("reads app_trans_id from provider display json", func(t *testing.T) {
		payload := []byte(`{"app_trans_id":"260906_abc-order","qr_payload":"..."}`)
		got := zalopayAppTransIDFromPayload(payload)
		if got != "260906_abc-order" {
			t.Fatalf("expected stored app_trans_id, got %q", got)
		}
	})

	t.Run("empty payload returns empty", func(t *testing.T) {
		if got := zalopayAppTransIDFromPayload(nil); got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})

	t.Run("malformed json returns empty", func(t *testing.T) {
		if got := zalopayAppTransIDFromPayload([]byte("{")); got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})
}

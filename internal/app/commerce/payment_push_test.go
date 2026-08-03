package commerce

import "testing"

func TestPaymentCapturedMQTTIdempotencyKey(t *testing.T) {
	t.Parallel()
	got := PaymentCapturedMQTTIdempotencyKey("pay-1", "evt-9")
	want := "payment.captured:pay-1:evt-9"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	got = PaymentCapturedMQTTIdempotencyKey("  pay-1  ", "  evt-9 ")
	if got != want {
		t.Fatalf("trim: got %q want %q", got, want)
	}
}

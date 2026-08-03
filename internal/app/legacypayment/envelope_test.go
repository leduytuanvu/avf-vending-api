package legacypayment

import (
	"encoding/json"
	"testing"
)

func TestCreateSuccessEnvelope_exactKeys(t *testing.T) {
	t.Parallel()

	env := CreateSuccessEnvelope(CreateSessionResult{
		QRCodeURL:         "https://pay.example/qr",
		PaymentProviderID: "zalopay",
		OrderCode:         "ORD-1",
		PaymentRefcode:    "",
	})
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m["code"] != float64(200) {
		t.Fatalf("code=%v", m["code"])
	}
	if m["message"] != "Thành công" {
		t.Fatalf("message=%v", m["message"])
	}
	data, ok := m["data"].(map[string]any)
	if !ok {
		t.Fatalf("data type %T", m["data"])
	}
	for _, k := range []string{"qrCodeUrl", "Payment_Provider_id", "Order_code", "Payment_Refcode"} {
		if _, exists := data[k]; !exists {
			t.Fatalf("missing data key %q in %v", k, data)
		}
	}
	if data["Payment_Provider_id"] != "zalopay" {
		t.Fatalf("Payment_Provider_id=%v", data["Payment_Provider_id"])
	}
	if data["qrCodeUrl"] != "https://pay.example/qr" {
		t.Fatalf("qrCodeUrl=%v", data["qrCodeUrl"])
	}
}

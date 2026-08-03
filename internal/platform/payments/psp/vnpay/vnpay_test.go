package vnpay

import (
	"crypto/md5"
	"encoding/hex"
	"strings"
	"testing"
)

func TestCreateChecksum_Golden(t *testing.T) {
	f := CreateChecksumFields{
		AppID:         "APP1",
		MasterMerCode: "MASTER",
		MerchantCode:  "MERCH",
		TerminalID:    "TERM1",
		ProductID:     "",
		TxnID:         "ORDER001",
		Amount:        "25000",
		TipAndFee:     "",
		ExpDate:       "2508011530",
		Secret:        "secretKey",
	}
	raw := "APP1|AVF VIET NAM|03|VN|MASTER|5045|MERCH|TERM1|03||ORDER001|25000||704|2508011530|secretKey"
	want := md5UpperTest(raw)
	got := CreateChecksum(f)
	if got != want {
		t.Fatalf("CreateChecksum = %q, want %q\nraw=%q", got, want, raw)
	}
	if got != strings.ToUpper(got) {
		t.Fatal("checksum must be uppercase")
	}
}

func TestQueryChecksum_Golden(t *testing.T) {
	raw := "01/08/2025|ORDER001|MERCH|TERM1|checkSecret"
	want := md5UpperTest(raw)
	got := QueryChecksum("01/08/2025", "ORDER001", "MERCH", "TERM1", "checkSecret")
	if got != want {
		t.Fatalf("QueryChecksum = %q, want %q", got, want)
	}
}

func TestMapCode(t *testing.T) {
	st, lg := MapCode("00")
	if st != StatusCaptured || lg != 0 {
		t.Fatalf("00 → (%q,%d)", st, lg)
	}
	st, lg = MapCode("01")
	if st != StatusFailed || lg != -1 {
		t.Fatalf("01 → (%q,%d)", st, lg)
	}
	st, lg = MapCode("99")
	if st != StatusFailed || lg != -1 {
		t.Fatalf("99 → (%q,%d)", st, lg)
	}
}

func TestConstants(t *testing.T) {
	if MerchantName != "AVF VIET NAM" || ServiceCode != "03" || PayType != "03" ||
		CCY != "704" || MerchantType != "5045" || CountryCode != "VN" {
		t.Fatal("unexpected constant values")
	}
}

func md5UpperTest(raw string) string {
	sum := md5.Sum([]byte(raw))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

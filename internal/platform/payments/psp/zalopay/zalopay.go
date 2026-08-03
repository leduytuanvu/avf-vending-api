// Package zalopay implements ZaloPay create/query/callback MAC and status mapping
// matching the legacy Python AVF payment service.
package zalopay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Status values returned by MapReturnCode.
const (
	StatusCaptured = "captured"
	StatusPending  = "pending"
	StatusFailed   = "failed"
)

// Credentials holds ZaloPay gateway credentials.
type Credentials struct {
	AppID       string
	Key1        string
	Key2        string
	Endpoint    string
	CallbackURL string
}

// CreateOrderMAC returns HMAC-SHA256 hex of KEY1 over:
//
//	{app_id}|{app_trans_id}|{app_user}|{amount}|{app_time}|{embed_data}|{item}
func CreateOrderMAC(key1 string, appID, appTransID, appUser, amount string, appTime int64, embedData, item string) string {
	raw := fmt.Sprintf("%s|%s|%s|%s|%d|%s|%s",
		appID, appTransID, appUser, amount, appTime, embedData, item)
	return hmacSHA256Hex(key1, raw)
}

// QueryMAC returns HMAC-SHA256 hex of KEY1 over:
//
//	{app_id}|{app_trans_id}|{KEY1}
func QueryMAC(key1, appID, appTransID string) string {
	raw := fmt.Sprintf("%s|%s|%s", appID, appTransID, key1)
	return hmacSHA256Hex(key1, raw)
}

// CallbackMAC returns HMAC-SHA256 hex of KEY2 over the callback data string.
func CallbackMAC(key2, data string) string {
	return hmacSHA256Hex(key2, data)
}

// VerifyCallbackMAC returns true when CallbackMAC(key2, data) equals mac.
func VerifyCallbackMAC(key2, data, mac string) bool {
	if key2 == "" || mac == "" {
		return false
	}
	expected := CallbackMAC(key2, data)
	return hmac.Equal([]byte(expected), []byte(mac))
}

// AppTransID builds yyMMdd_orderCode using the provided time (local calendar date).
func AppTransID(orderCode string, now time.Time) string {
	return fmt.Sprintf("%s_%s", now.Format("060102"), orderCode)
}

// EmbedData returns JSON embed_data with preferred_payment_method.
// vietqr=true → ["vietqr"]; otherwise ["zalopay_wallet"].
func EmbedData(vietqr bool) string {
	method := "zalopay_wallet"
	if vietqr {
		method = "vietqr"
	}
	payload := map[string]any{
		"preferred_payment_method": []string{method},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		// static shape; marshal cannot fail in practice
		return `{"preferred_payment_method":["zalopay_wallet"]}`
	}
	return string(b)
}

// MapReturnCode maps ZaloPay provider return_code to (status, legacyReturnCode).
// 1 → captured/0, 3 → pending/1, else failed/-1.
func MapReturnCode(returnCode int) (status string, legacyCode int) {
	switch returnCode {
	case 1:
		return StatusCaptured, 0
	case 3:
		return StatusPending, 1
	default:
		return StatusFailed, -1
	}
}

// FormatAmount formats VND as a decimal string for ZaloPay create.
func FormatAmount(vnd int64) string {
	return strconv.FormatInt(vnd, 10)
}

func hmacSHA256Hex(key, data string) string {
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

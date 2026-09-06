// Package shopeepay implements ShopeePay MPM signing, amount conversion, IP whitelist,
// and status mapping matching the legacy Python AVF payment service.
package shopeepay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
)

const (
	// AmountFactor converts VND đồng to ShopeePay API amount units.
	AmountFactor = 100
	// PaymentReferenceIDMaxLength is the PGW max length for payment_reference_id.
	PaymentReferenceIDMaxLength = 21

	TenantAVF = "avf"
	TenantTFO = "tfo"

	StatusCaptured = "captured"
	StatusPending  = "pending"
	StatusFailed   = "failed"

	TXStatusInitial     = 1
	TXStatusProcessing  = 2
	TXStatusSuccess     = 3
	TXStatusFailed      = 4
	TXStatusExpired     = 6
	TXStatusInvalidated = 7
)

// Credentials holds ShopeePay MPM credentials for one tenant.
type Credentials struct {
	ClientID      string
	SecretKey     string
	Endpoint      string
	MerchantExtID string
	StoreExtID    string
	CallbackURL   string
	TerminalID    string
}

// TenantKeys holds AVF and TFO credential variants.
type TenantKeys struct {
	AVF Credentials
	TFO Credentials
}

// ResolveTenant returns "tfo" when machineCode has prefix "TFO", otherwise "avf".
func ResolveTenant(machineCode string) string {
	if strings.HasPrefix(machineCode, "TFO") {
		return TenantTFO
	}
	return TenantAVF
}

// ForTenant returns credentials for the given tenant key.
func (k TenantKeys) ForTenant(tenant string) Credentials {
	if tenant == TenantTFO {
		return k.TFO
	}
	return k.AVF
}

// SignBody returns Base64(HMAC-SHA256(secret, body)).
func SignBody(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// VerifySignature returns true when SignBody(secret, body) equals signature (constant-time).
func VerifySignature(secret string, body []byte, signature string) bool {
	if secret == "" || signature == "" {
		return false
	}
	expected := SignBody(secret, body)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// ToAPIAmount converts VND đồng to ShopeePay API amount (×100).
func ToAPIAmount(vnd int64) int64 {
	return vnd * AmountFactor
}

// FromAPIAmount converts ShopeePay API amount back to VND đồng (÷100).
func FromAPIAmount(apiAmount int64) int64 {
	return apiAmount / AmountFactor
}

// ParseWhitelist parses a CSV of allowed callback IPs. Empty input → empty set.
func ParseWhitelist(csv string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, part := range strings.Split(csv, ",") {
		ip := strings.TrimSpace(part)
		if ip != "" {
			out[ip] = struct{}{}
		}
	}
	return out
}

// ClientIPAllowed returns true when whitelist is empty (allow all) or ip is listed.
func ClientIPAllowed(ip string, whitelist map[string]struct{}) bool {
	if len(whitelist) == 0 {
		return true
	}
	if ip == "" {
		return false
	}
	_, ok := whitelist[ip]
	return ok
}

// MapTxStatus maps ShopeePay transaction_status to (status, legacyReturnCode).
// Unknown codes map to pending; only documented terminal codes map to failed.
func MapTxStatus(txStatus int) (status string, legacyCode int) {
	d := MapTxStatusDetail(txStatus)
	return d.Status, d.LegacyCode
}

// TxStatusDetail is the normalized status plus recognition metadata.
type TxStatusDetail struct {
	Status     string
	LegacyCode int
	Recognized bool
}

// MapTxStatusDetail classifies a ShopeePay transaction_status.
func MapTxStatusDetail(txStatus int) TxStatusDetail {
	switch txStatus {
	case TXStatusSuccess:
		return TxStatusDetail{Status: StatusCaptured, LegacyCode: 0, Recognized: true}
	case TXStatusInitial, TXStatusProcessing:
		return TxStatusDetail{Status: StatusPending, LegacyCode: 1, Recognized: true}
	case TXStatusFailed, TXStatusExpired, TXStatusInvalidated:
		return TxStatusDetail{Status: StatusFailed, LegacyCode: -1, Recognized: true}
	default:
		return TxStatusDetail{Status: StatusPending, LegacyCode: 1, Recognized: false}
	}
}

// SoftACK is the JSON body returned to ShopeePay on successful callback receipt.
var SoftACK = []byte(`{"errcode":0,"debug_msg":"success"}`)

// SoftACKMap returns the soft-ACK payload as a map.
func SoftACKMap() map[string]any {
	return map[string]any{
		"errcode":   0,
		"debug_msg": "success",
	}
}

// SoftACKJSON returns SoftACK bytes (stable JSON).
func SoftACKJSON() []byte {
	b, err := json.Marshal(SoftACKMap())
	if err != nil {
		return SoftACK
	}
	return b
}

// BuildHeaders builds ShopeePay request headers with body signature.
func BuildHeaders(clientID, secret string, body []byte) map[string]string {
	return map[string]string{
		"Content-Type":      "application/json",
		"X-Airpay-ClientId": clientID,
		"X-Airpay-Req-H":    SignBody(secret, body),
	}
}

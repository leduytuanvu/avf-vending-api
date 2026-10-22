// Package shopeepay implements ShopeePay MPM signing, amount conversion, IP whitelist,
// and status mapping matching the legacy Python AVF payment service.
package shopeepay

import (
	"crypto/hmac"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
)

// DefaultStoreAllowlistFile is the LIVE SPP store_ext_id list (machine codes).
//
//go:embed store_ext_id_allowlist.txt
var DefaultStoreAllowlistFile string

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

	LiveHost = "api.wallet.airpay.vn"
	UATHost  = "api.uat.wallet.airpay.vn"

	HeaderClientID = "X-Airpay-ClientId"
	HeaderReqH     = "X-Airpay-Req-H"
	HeaderRespH    = "X-Airpay-Resp-H"

	QRCreatePath     = "/v3/merchant-host/qr/create"
	QRInvalidatePath = "/v3/merchant-host/qr/invalidate"
	TxnCheckPath     = "/v3/merchant-host/transaction/check"
	RefundCreatePath = "/v3/merchant-host/transaction/refund/create-new"
)

var (
	ErrCallbackForbiddenIP      = errors.New("forbidden ip")
	ErrCallbackMissingClientID  = errors.New("missing client id")
	ErrCallbackMissingSignature = errors.New("missing signature")
	ErrCallbackUnknownClientID  = errors.New("unknown client id")
	ErrCallbackInvalidSignature = errors.New("invalid signature")
	ErrStoreNotAllowlisted      = errors.New("store_ext_id is not allowlisted")
	ErrInvalidResponseSignature = errors.New("invalid response signature")
	ErrRedirectBlocked          = errors.New("redirect blocked")
	ErrEmptyEndpointHost        = errors.New("endpoint host is empty")
	ErrUATEndpointOnProduction  = errors.New("uat endpoint blocked on production")
	ErrNotLiveEndpoint          = errors.New("endpoint is not ShopeePay LIVE host")
	ErrLiveEndpointBlocked      = errors.New("live endpoint blocked")
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
	if strings.HasPrefix(strings.TrimSpace(machineCode), "TFO") {
		return TenantTFO
	}
	return TenantAVF
}

// ResolveStoreExtID matches legacy Python: AVF*/TFO* machine codes are the SPP store_ext_id.
func ResolveStoreExtID(machineCode, fallback string) string {
	code := strings.TrimSpace(machineCode)
	if code != "" && (strings.HasPrefix(code, "AVF") || strings.HasPrefix(code, "TFO")) {
		return code
	}
	return strings.TrimSpace(fallback)
}

// EndpointHost returns the hostname of a ShopeePay END_POINT URL.
func EndpointHost(endpoint string) string {
	raw := strings.TrimSpace(endpoint)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// ValidateEndpoint enforces LIVE host on production and blocks LIVE elsewhere.
func ValidateEndpoint(endpoint string, production bool) error {
	host := EndpointHost(endpoint)
	if host == "" {
		return ErrEmptyEndpointHost
	}
	if production {
		if host == LiveHost {
			return nil
		}
		if host == UATHost {
			return ErrUATEndpointOnProduction
		}
		return ErrNotLiveEndpoint
	}
	if host == LiveHost {
		return ErrLiveEndpointBlocked
	}
	return nil
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

// ParseAllowlistLines parses store_ext_id / machine codes, ignoring blanks and # comments.
func ParseAllowlistLines(text string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out[line] = struct{}{}
	}
	return out
}

// ParseAllowlistCSV parses a CSV of store_ext_id values.
func ParseAllowlistCSV(csv string) map[string]struct{} {
	return ParseAllowlistLines(strings.ReplaceAll(csv, ",", "\n"))
}

// MergeAllowlist unions CSV codes, file contents, and optional extra maps.
func MergeAllowlist(csv string, fileText string, extra ...map[string]struct{}) map[string]struct{} {
	out := ParseAllowlistCSV(csv)
	for code := range ParseAllowlistLines(fileText) {
		out[code] = struct{}{}
	}
	for _, m := range extra {
		for code := range m {
			if strings.TrimSpace(code) == "" {
				continue
			}
			out[code] = struct{}{}
		}
	}
	return out
}

// StoreAllowed reports whether storeExtID may call SPP. Empty allowlist + failClosed → deny.
func StoreAllowed(storeExtID string, allowlist map[string]struct{}, failClosed bool) bool {
	storeExtID = strings.TrimSpace(storeExtID)
	if len(allowlist) == 0 {
		return !failClosed
	}
	if storeExtID == "" {
		return false
	}
	_, ok := allowlist[storeExtID]
	return ok
}

// ClientIPAllowed returns true when whitelist is empty (allow all) or ip is listed.
func ClientIPAllowed(ip string, whitelist map[string]struct{}) bool {
	return ClientIPPolicy(ip, whitelist, false)
}

// ClientIPPolicy is fail-closed when whitelist is empty and failClosed is true (production/staging).
func ClientIPPolicy(ip string, whitelist map[string]struct{}, failClosed bool) bool {
	if len(whitelist) == 0 {
		return !failClosed
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
		"Content-Type": "application/json",
		HeaderClientID: clientID,
		HeaderReqH:     SignBody(secret, body),
	}
}

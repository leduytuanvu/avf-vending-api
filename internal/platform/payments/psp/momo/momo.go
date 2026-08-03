// Package momo implements MoMo Wallet create/query/IPN signing and status mapping
// matching the legacy Python AVF payment service.
package momo

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// Tenant identifiers used to select AVF vs TFO credentials.
const (
	TenantAVF = "avf"
	TenantTFO = "tfo"
)

// RequestTypeCreate is the MoMo captureWallet request type.
const RequestTypeCreate = "captureWallet"

// Status values returned by MapResultCode.
const (
	StatusCaptured = "captured"
	StatusPending  = "pending"
	StatusFailed   = "failed"
)

// Credentials holds MoMo gateway credentials for one tenant.
type Credentials struct {
	PartnerCode string
	AccessKey   string
	SecretKey   string
	Endpoint    string
	RedirectURL string
	IPNURL      string
	TerminalID  string
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

// ForTenant returns credentials for the given tenant key ("avf" or "tfo").
func (k TenantKeys) ForTenant(tenant string) Credentials {
	if tenant == TenantTFO {
		return k.TFO
	}
	return k.AVF
}

// CreateRequest holds fields required to build and sign a MoMo create request.
type CreateRequest struct {
	AccessKey   string
	Amount      string // VND as decimal string
	ExtraData   string
	IPNURL      string
	OrderID     string
	OrderInfo   string
	PartnerCode string
	RedirectURL string
	RequestID   string
	RequestType string // typically captureWallet
	StoreID     string
	Lang        string // default "vi"
}

// SignCreate returns HMAC-SHA256 hex of the create raw signature string:
//
//	accessKey=&amount=&extraData=&ipnUrl=&orderId=&orderInfo=&partnerCode=&redirectUrl=&requestId=&requestType=
func SignCreate(secret string, r CreateRequest) string {
	raw := fmt.Sprintf(
		"accessKey=%s&amount=%s&extraData=%s&ipnUrl=%s&orderId=%s&orderInfo=%s&partnerCode=%s&redirectUrl=%s&requestId=%s&requestType=%s",
		r.AccessKey, r.Amount, r.ExtraData, r.IPNURL, r.OrderID, r.OrderInfo, r.PartnerCode, r.RedirectURL, r.RequestID, r.RequestType,
	)
	return hmacSHA256Hex(secret, raw)
}

// SignQuery returns HMAC-SHA256 hex of:
//
//	accessKey=&orderId=&partnerCode=&requestId=
func SignQuery(secret, accessKey, orderID, partnerCode, requestID string) string {
	raw := fmt.Sprintf(
		"accessKey=%s&orderId=%s&partnerCode=%s&requestId=%s",
		accessKey, orderID, partnerCode, requestID,
	)
	return hmacSHA256Hex(secret, raw)
}

// IPNFields holds momo_wallet IPN callback fields used for signature verification.
type IPNFields struct {
	AccessKey    string
	Amount       string
	ExtraData    string
	Message      string
	OrderID      string
	OrderInfo    string
	OrderType    string
	PartnerCode  string
	PayType      string
	RequestID    string
	ResponseTime string
	ResultCode   string
	TransID      string
}

// SignIPN returns HMAC-SHA256 hex for momo_wallet IPN:
//
//	accessKey=&amount=&extraData=&message=&orderId=&orderInfo=&orderType=&partnerCode=&payType=&requestId=&responseTime=&resultCode=&transId=
func SignIPN(secret string, f IPNFields) string {
	raw := fmt.Sprintf(
		"accessKey=%s&amount=%s&extraData=%s&message=%s&orderId=%s&orderInfo=%s&orderType=%s&partnerCode=%s&payType=%s&requestId=%s&responseTime=%s&resultCode=%s&transId=%s",
		f.AccessKey, f.Amount, f.ExtraData, f.Message, f.OrderID, f.OrderInfo, f.OrderType, f.PartnerCode, f.PayType, f.RequestID, f.ResponseTime, f.ResultCode, f.TransID,
	)
	return hmacSHA256Hex(secret, raw)
}

// BuildCreateBody builds the MoMo create JSON body map including signature.
func BuildCreateBody(secret string, r CreateRequest) map[string]any {
	if r.RequestType == "" {
		r.RequestType = RequestTypeCreate
	}
	if r.Lang == "" {
		r.Lang = "vi"
	}
	if r.ExtraData == "" {
		r.ExtraData = ""
	}
	sig := SignCreate(secret, r)
	return map[string]any{
		"partnerCode": r.PartnerCode,
		"storeId":     r.StoreID,
		"requestType": r.RequestType,
		"ipnUrl":      r.IPNURL,
		"redirectUrl": r.RedirectURL,
		"orderId":     r.OrderID,
		"amount":      r.Amount,
		"lang":        r.Lang,
		"orderInfo":   r.OrderInfo,
		"requestId":   r.RequestID,
		"extraData":   r.ExtraData,
		"signature":   sig,
	}
}

// VerifyIPNSignature returns true when the provided signature matches SignIPN.
func VerifyIPNSignature(secret string, f IPNFields, signature string) bool {
	if secret == "" || signature == "" {
		return false
	}
	expected := SignIPN(secret, f)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// MapResultCode maps MoMo resultCode to normalized status.
// 0 → captured, 9000 → pending, else failed.
func MapResultCode(resultCode int) string {
	switch resultCode {
	case 0:
		return StatusCaptured
	case 9000:
		return StatusPending
	default:
		return StatusFailed
	}
}

// MapResultCodeToReturnCode maps MoMo resultCode to legacy return_code.
// 0 → 0, 9000 → 1, else -1.
func MapResultCodeToReturnCode(resultCode int) int {
	switch resultCode {
	case 0:
		return 0
	case 9000:
		return 1
	default:
		return -1
	}
}

// FormatAmount formats VND minor units (đồng) as a decimal string for MoMo.
func FormatAmount(vnd int64) string {
	return strconv.FormatInt(vnd, 10)
}

func hmacSHA256Hex(secret, raw string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(raw))
	return hex.EncodeToString(mac.Sum(nil))
}

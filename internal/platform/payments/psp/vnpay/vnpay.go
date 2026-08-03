// Package vnpay implements VNPay Merchant QR checksums and status mapping
// matching the legacy Python AVF payment service (MD5 pipe-joined, not web SDK HMAC).
package vnpay

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
)

// Fixed create-QR field constants from legacy process_vnpay.
const (
	MerchantName = "AVF VIET NAM"
	ServiceCode  = "03"
	PayType      = "03"
	CCY          = "704"
	MerchantType = "5045"
	CountryCode  = "VN"
)

// Status values returned by MapCode.
const (
	StatusCaptured = "captured"
	StatusFailed   = "failed"
)

// Credentials holds VNPay Merchant QR credentials.
type Credentials struct {
	AppID               string
	SecretKey           string
	SecretKeyCheckTrans string
	SecretKeyRefund     string
	MasterMerCode       string
	MerchantCode        string
	TerminalID          string
	EndpointURL         string
	EndpointQueryURL    string
	ReturnURL           string
}

// CreateChecksumFields holds variable fields for create-QR MD5 checksum.
type CreateChecksumFields struct {
	AppID         string
	MasterMerCode string
	MerchantCode  string
	TerminalID    string
	ProductID     string
	TxnID         string
	Amount        string
	TipAndFee     string
	ExpDate       string
	Secret        string
}

// CreateChecksum returns uppercase MD5 of:
//
//	appId|merchantName|serviceCode|countryCode|masterMerCode|merchantType|merchantCode|terminalId|payType|productId|txnId|amount|tipAndFee|ccy|expDate|secret
func CreateChecksum(f CreateChecksumFields) string {
	raw := strings.Join([]string{
		f.AppID,
		MerchantName,
		ServiceCode,
		CountryCode,
		f.MasterMerCode,
		MerchantType,
		f.MerchantCode,
		f.TerminalID,
		PayType,
		f.ProductID,
		f.TxnID,
		f.Amount,
		f.TipAndFee,
		CCY,
		f.ExpDate,
		f.Secret,
	}, "|")
	return md5Upper(raw)
}

// QueryChecksum returns uppercase MD5 of:
//
//	payDate|txnId|merchantCode|terminalId|secretCheckTrans
func QueryChecksum(payDate, txnID, merchantCode, terminalID, secretCheckTrans string) string {
	raw := fmt.Sprintf("%s|%s|%s|%s|%s", payDate, txnID, merchantCode, terminalID, secretCheckTrans)
	return md5Upper(raw)
}

// MapCode maps VNPay response code to (status, legacyReturnCode).
// "00" → captured/0, else failed/-1.
func MapCode(code string) (status string, legacyCode int) {
	if code == "00" {
		return StatusCaptured, 0
	}
	return StatusFailed, -1
}

func md5Upper(raw string) string {
	sum := md5.Sum([]byte(raw))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

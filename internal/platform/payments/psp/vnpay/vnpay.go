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
	StatusPending  = "pending"
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
// Unknown/transient codes map to pending; only documented terminal codes map to failed.
func MapCode(code string) (status string, legacyCode int) {
	return MapCodeDetail(code).Status, MapCodeDetail(code).LegacyCode
}

// CodeDetail is the normalized status plus recognition metadata.
type CodeDetail struct {
	Status     string
	LegacyCode int
	Recognized bool
}

// MapCodeDetail classifies a VNPay response code.
func MapCodeDetail(code string) CodeDetail {
	c := strings.TrimSpace(code)
	switch c {
	case "00", "08":
		return CodeDetail{Status: StatusCaptured, LegacyCode: 0, Recognized: true}
	case "09", "10", "11", "12", "13", "24", "51", "91", "97", "99":
		return CodeDetail{Status: StatusPending, LegacyCode: 1, Recognized: true}
	case "01", "02", "03", "04", "05", "06", "07", "19", "20", "21", "22", "23", "25", "26", "27", "28", "29", "30", "31", "32", "33", "34", "35", "36", "37", "38", "39", "40", "41", "42", "43", "44", "45", "46", "47", "48", "49", "50", "52", "53", "54", "55", "56", "57", "58", "59", "60", "61", "62", "63", "64", "65", "66", "67", "68", "69", "70", "71", "72", "73", "74", "75", "76", "77", "78", "79", "80", "81", "82", "83", "84", "85", "86", "87", "88", "89", "90", "92", "93", "94", "95", "96", "98":
		return CodeDetail{Status: StatusFailed, LegacyCode: -1, Recognized: true}
	case "":
		return CodeDetail{Status: StatusPending, LegacyCode: 1, Recognized: false}
	default:
		return CodeDetail{Status: StatusPending, LegacyCode: 1, Recognized: false}
	}
}

func md5Upper(raw string) string {
	sum := md5.Sum([]byte(raw))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

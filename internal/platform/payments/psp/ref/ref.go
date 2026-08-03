// Package ref generates and validates short PSP provider references.
// MaxLength matches ShopeePay's payment_reference_id limit (21).
package ref

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// MaxLength is the maximum allowed provider reference length (ShopeePay PGW limit).
const MaxLength = 21

// crockfordAlphabet is Douglas Crockford's Base32 alphabet (no I, L, O, U).
const crockfordAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// GenerateFromUUID returns a URL-safe, unique-ish reference ≤ MaxLength.
// Encodes the UUID with Crockford Base32 and truncates to MaxLength.
func GenerateFromUUID(u uuid.UUID) string {
	s := encodeCrockford(u[:])
	if len(s) > MaxLength {
		return s[:MaxLength]
	}
	return s
}

// Validate returns an error when ref is empty or longer than MaxLength.
func Validate(ref string) error {
	if ref == "" {
		return fmt.Errorf("provider reference is required")
	}
	if len(ref) > MaxLength {
		return fmt.Errorf("provider reference must be at most %d characters", MaxLength)
	}
	return nil
}

// encodeCrockford encodes raw bytes as Crockford Base32 (no padding).
func encodeCrockford(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	var bits uint64
	var nbits int
	var b strings.Builder
	b.Grow((len(data)*8 + 4) / 5)
	for _, byt := range data {
		bits = (bits << 8) | uint64(byt)
		nbits += 8
		for nbits >= 5 {
			nbits -= 5
			idx := byte((bits >> nbits) & 0x1F)
			b.WriteByte(crockfordAlphabet[idx])
		}
	}
	if nbits > 0 {
		idx := byte((bits << (5 - nbits)) & 0x1F)
		b.WriteByte(crockfordAlphabet[idx])
	}
	return b.String()
}

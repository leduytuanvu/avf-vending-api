package activation

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"regexp"
	"strings"
)

const ActivationCodeLength = 6

var activationCodePattern = regexp.MustCompile(`^[0-9]{6}$`)

func normalizeActivationCode(s string) string {
	return strings.TrimSpace(s)
}

func validActivationCode(s string) bool {
	return activationCodePattern.MatchString(normalizeActivationCode(s))
}

func randomActivationCode() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	n := binary.BigEndian.Uint32(b[:]) % 1_000_000
	return fmt.Sprintf("%06d", n), nil
}

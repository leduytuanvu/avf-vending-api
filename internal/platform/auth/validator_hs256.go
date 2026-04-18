package auth

import (
	"context"
	"time"
)

type hs256Validator struct {
	secrets [][]byte
	leeway  time.Duration
}

func newHS256Validator(primary, previous []byte, leeway time.Duration) *hs256Validator {
	var secrets [][]byte
	if len(primary) > 0 {
		secrets = append(secrets, primary)
	}
	if len(previous) > 0 {
		secrets = append(secrets, previous)
	}
	if leeway <= 0 {
		leeway = DefaultClockLeeway
	}
	return &hs256Validator{secrets: secrets, leeway: leeway}
}

func (v *hs256Validator) ValidateAccessToken(_ context.Context, raw string) (Principal, error) {
	if len(v.secrets) == 0 {
		return Principal{}, ErrMisconfigured
	}
	for _, sec := range v.secrets {
		payload, err := verifyHS256JWT(sec, raw)
		if err != nil {
			continue
		}
		return PrincipalFromJWTPayloadJSON(payload, v.leeway)
	}
	return Principal{}, ErrUnauthenticated
}

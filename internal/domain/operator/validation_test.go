package operator

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestValidateActorConsistency(t *testing.T) {
	tid := uuid.New()
	up := "sub-123"
	cases := []struct {
		name    string
		actor   string
		tid     *uuid.UUID
		prin    *string
		wantErr bool
	}{
		{"technician_ok", ActorTypeTechnician, &tid, nil, false},
		{"technician_missing_id", ActorTypeTechnician, nil, nil, true},
		{"technician_with_principal", ActorTypeTechnician, &tid, ptr("x"), true},
		{"user_ok", ActorTypeUser, nil, &up, false},
		{"user_with_tech", ActorTypeUser, &tid, &up, true},
		{"user_empty_principal", ActorTypeUser, nil, ptr(""), true},
		{"bad_actor", "OTHER", nil, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateActorConsistency(tc.actor, tc.tid, tc.prin)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected: %v", err)
			}
		})
	}
}

func ptr(s string) *string { return &s }

func TestValidateSessionExpiryBounds(t *testing.T) {
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	if err := ValidateSessionExpiryBounds(nil, now, MaxOperatorSessionTTL); err != nil {
		t.Fatalf("nil expiry: %v", err)
	}
	past := now.Add(-time.Minute)
	if err := ValidateSessionExpiryBounds(&past, now, MaxOperatorSessionTTL); err != ErrInvalidSessionExpiry {
		t.Fatalf("past: want ErrInvalidSessionExpiry, got %v", err)
	}
	ok := now.Add(2 * time.Hour)
	if err := ValidateSessionExpiryBounds(&ok, now, MaxOperatorSessionTTL); err != nil {
		t.Fatalf("future ok: %v", err)
	}
	far := now.Add(MaxOperatorSessionTTL + time.Hour)
	if err := ValidateSessionExpiryBounds(&far, now, MaxOperatorSessionTTL); err != ErrInvalidSessionExpiry {
		t.Fatalf("too far: want ErrInvalidSessionExpiry, got %v", err)
	}
}

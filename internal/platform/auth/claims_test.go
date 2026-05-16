package auth

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPrincipalFromJWTPayloadJSON_basicUser(t *testing.T) {
	payload, err := json.Marshal(map[string]any{
		"sub":   "user-1",
		"roles": []string{"member"},
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := PrincipalFromJWTPayloadJSON(payload, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if p.Subject != "user-1" {
		t.Fatalf("subject: %q", p.Subject)
	}
	if !p.HasRole(RoleOrgMember) {
		t.Fatal("expected member")
	}
}

func TestPrincipalFromJWTPayloadJSON_accountStatus(t *testing.T) {
	payload, err := json.Marshal(map[string]any{
		"sub":            "user-1",
		"roles":          []string{"viewer"},
		"account_status": "disabled",
		"exp":            time.Now().Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := PrincipalFromJWTPayloadJSON(payload, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !p.InteractiveAccountDisabled() {
		t.Fatal("expected disabled principal")
	}
}

func TestPrincipalFromJWTPayloadJSON_expiredRejected(t *testing.T) {
	payload, err := json.Marshal(map[string]any{
		"sub": "user-1",
		"exp": time.Now().Add(-2 * time.Hour).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = PrincipalFromJWTPayloadJSON(payload, time.Second)
	if err != ErrUnauthenticated {
		t.Fatalf("want ErrUnauthenticated, got %v", err)
	}
}

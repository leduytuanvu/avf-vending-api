package emqxadmin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEnsureUserCreated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "authentication/password_based") {
			t.Fatalf("path %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method %s", r.Method)
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "key" || pass != "secret" {
			t.Fatalf("auth")
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "key", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.EnsureUser(context.Background(), "019f2471-e47c-794e-a767-d95f8570ffa0", "pw"); err != nil {
		t.Fatal(err)
	}
}

func TestUpsertUserConflictUpdatesPassword(t *testing.T) {
	var putPassword string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			w.WriteHeader(http.StatusConflict)
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			var payload map[string]string
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatal(err)
			}
			putPassword = payload["password"]
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer srv.Close()
	c, err := NewClient(srv.URL, "key", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.UpsertUser(context.Background(), "machine-a", "new-pw"); err != nil {
		t.Fatal(err)
	}
	if putPassword != "new-pw" {
		t.Fatalf("expected password update, got %q", putPassword)
	}
}

func TestDeleteUserOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "machine-a") {
			t.Fatalf("path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c, err := NewClient(srv.URL, "key", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteUser(context.Background(), "machine-a"); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteUserNotFoundOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c, err := NewClient(srv.URL, "key", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteUser(context.Background(), "missing"); err != nil {
		t.Fatal(err)
	}
}

func TestManagementHost(t *testing.T) {
	c, err := NewClient("http://10.0.0.20:18083", "key", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if got := c.ManagementHost(); got != "10.0.0.20:18083" {
		t.Fatalf("ManagementHost=%q", got)
	}
	if got := (*Client)(nil).ManagementHost(); got != "" {
		t.Fatalf("nil ManagementHost=%q", got)
	}
}

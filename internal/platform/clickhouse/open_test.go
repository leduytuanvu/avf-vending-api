package clickhouse

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpen_DisabledReturnsNoop(t *testing.T) {
	t.Parallel()
	c, err := Open(context.Background(), Config{Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Fatal("nil client")
	}
	if err := c.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.InsertJSONEachRow(context.Background(), "t", []byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOpen_EnabledEmptyEndpoint(t *testing.T) {
	t.Parallel()
	_, err := Open(context.Background(), Config{Enabled: true, HTTPEndpoint: ""})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "HTTPEndpoint") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestOpen_EnabledHTTPEndpointPingAndInsert(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		if strings.Contains(q, "SELECT 1") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("1\n"))
			return
		}
		if strings.Contains(q, "INSERT") {
			_, _ = io.Copy(io.Discard, r.Body)
			_ = r.Body.Close()
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	u := srv.URL + "/default"
	c, err := Open(context.Background(), Config{Enabled: true, HTTPEndpoint: u})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.InsertJSONEachRow(context.Background(), "avf_outbox_mirror", []byte("{\"outbox_id\":1}\n")); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOpen_EnabledBadURL(t *testing.T) {
	t.Parallel()
	_, err := Open(context.Background(), Config{Enabled: true, HTTPEndpoint: "tcp://localhost:9000/db"})
	if err == nil {
		t.Fatal("expected error")
	}
}

package httpx

import (
	"testing"
	"time"
)

func TestNewSharesDefaultTransport(t *testing.T) {
	c1 := New(DefaultTimeout)
	c2 := New(5 * time.Second)
	if c1.HTTP.Transport != c2.HTTP.Transport {
		t.Fatal("expected shared defaultTransport for connection reuse")
	}
	if c1.HTTP.Transport != defaultTransport {
		t.Fatal("expected defaultTransport instance")
	}
}

func TestNewAppliesTimeout(t *testing.T) {
	c := New(3 * time.Second)
	if c.Timeout != 3*time.Second {
		t.Fatalf("timeout=%v", c.Timeout)
	}
	if c.HTTP.Timeout != 3*time.Second {
		t.Fatalf("client timeout=%v", c.HTTP.Timeout)
	}
}

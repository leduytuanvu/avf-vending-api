// Package httpx provides small HTTP helpers for PSP outbound calls.
package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultTimeout is used when a caller does not set a context deadline.
const DefaultTimeout = 10 * time.Second

// Client is a thin wrapper around http.Client for JSON and form POSTs.
type Client struct {
	HTTP    *http.Client
	Timeout time.Duration
}

// New returns a Client with the given timeout (falls back to DefaultTimeout).
func New(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Client{
		HTTP:    &http.Client{Timeout: timeout},
		Timeout: timeout,
	}
}

// PostJSON POSTs a JSON body and returns the response body bytes and status code.
func (c *Client) PostJSON(ctx context.Context, endpoint string, headers map[string]string, body any) ([]byte, int, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal json: %w", err)
	}
	return c.do(ctx, endpoint, "application/json", headers, payload)
}

// PostJSONBytes POSTs raw JSON bytes.
func (c *Client) PostJSONBytes(ctx context.Context, endpoint string, headers map[string]string, body []byte) ([]byte, int, error) {
	return c.do(ctx, endpoint, "application/json", headers, body)
}

// PostRaw POSTs raw bytes with an explicit Content-Type (e.g. text/plain JSON for VNPay create).
func (c *Client) PostRaw(ctx context.Context, endpoint, contentType string, headers map[string]string, body []byte) ([]byte, int, error) {
	return c.do(ctx, endpoint, contentType, headers, body)
}

// PostForm POSTs application/x-www-form-urlencoded fields.
func (c *Client) PostForm(ctx context.Context, endpoint string, headers map[string]string, fields map[string]string) ([]byte, int, error) {
	form := url.Values{}
	for k, v := range fields {
		form.Set(k, v)
	}
	return c.do(ctx, endpoint, "application/x-www-form-urlencoded", headers, []byte(form.Encode()))
}

func (c *Client) do(ctx context.Context, endpoint, contentType string, headers map[string]string, body []byte) ([]byte, int, error) {
	if c == nil {
		c = New(DefaultTimeout)
	}
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: c.Timeout}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok && c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range headers {
		if strings.EqualFold(k, "Content-Type") {
			continue
		}
		req.Header.Set(k, v)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return respBody, resp.StatusCode, nil
}

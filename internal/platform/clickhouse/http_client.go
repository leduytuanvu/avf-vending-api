package clickhouse

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type httpClient struct {
	baseURL  string // origin without path, e.g. http://localhost:8123
	database string
	user     string
	pass     string
	http     *http.Client
}

func newHTTPClient(raw string) (*httpClient, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("clickhouse: parse http url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("clickhouse: unsupported scheme %q (use http or https)", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("clickhouse: missing host in url")
	}
	db := strings.Trim(strings.TrimSpace(u.Path), "/")
	if db == "" {
		return nil, fmt.Errorf("clickhouse: url must include database path segment (e.g. /avf)")
	}
	user := ""
	pass := ""
	if u.User != nil {
		user = u.User.Username()
		pass, _ = u.User.Password()
	}
	origin := u.Scheme + "://" + u.Host
	return &httpClient{
		baseURL:  origin,
		database: db,
		user:     user,
		pass:     pass,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (c *httpClient) doQuery(ctx context.Context, query string, body io.Reader) error {
	if c == nil {
		return fmt.Errorf("clickhouse: nil client")
	}
	q := url.Values{}
	q.Set("database", c.database)
	q.Set("query", query)
	full := c.baseURL + "/?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, full, body)
	if err != nil {
		return err
	}
	if c.user != "" || c.pass != "" {
		req.SetBasicAuth(c.user, c.pass)
	}
	if body != nil {
		req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("clickhouse: http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("clickhouse: http status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func (c *httpClient) getQuery(ctx context.Context, query string) error {
	if c == nil {
		return fmt.Errorf("clickhouse: nil client")
	}
	q := url.Values{}
	q.Set("database", c.database)
	q.Set("query", query)
	full := c.baseURL + "/?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return err
	}
	if c.user != "" || c.pass != "" {
		req.SetBasicAuth(c.user, c.pass)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("clickhouse: http get: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("clickhouse: http status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func (c *httpClient) Ping(ctx context.Context) error {
	return c.getQuery(ctx, "SELECT 1")
}

func (c *httpClient) Close() error {
	return nil
}

// InsertJSONEachRow appends one JSONEachRow line (must include trailing newline).
func (c *httpClient) InsertJSONEachRow(ctx context.Context, table string, line []byte) error {
	if strings.TrimSpace(table) == "" {
		return fmt.Errorf("clickhouse: empty table")
	}
	q := fmt.Sprintf("INSERT INTO %s FORMAT JSONEachRow", table)
	return c.doQuery(ctx, q, bytes.NewReader(line))
}

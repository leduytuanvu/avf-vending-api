// Package emqxadmin provisions built-in-database MQTT users via the EMQX management API.
package emqxadmin

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

const defaultUsersPath = "/api/v5/authentication/password_based%3Abuilt_in_database/users"

// Client talks to EMQX management REST (loopback on the broker host).
type Client struct {
	BaseURL    string
	APIKey     string
	APISecret  string
	HTTPClient *http.Client
}

// NewClient returns a client when base URL and API credentials are configured.
func NewClient(baseURL, apiKey, apiSecret string) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	apiKey = strings.TrimSpace(apiKey)
	apiSecret = strings.TrimSpace(apiSecret)
	if baseURL == "" || apiKey == "" || apiSecret == "" {
		return nil, fmt.Errorf("emqxadmin: base URL and API credentials are required")
	}
	return &Client{
		BaseURL:   baseURL,
		APIKey:    apiKey,
		APISecret: apiSecret,
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}, nil
}

func (c *Client) usersURL(userID string) string {
	if strings.TrimSpace(userID) == "" {
		return c.BaseURL + defaultUsersPath
	}
	return c.BaseURL + defaultUsersPath + "/" + url.PathEscape(strings.TrimSpace(userID))
}

func (c *Client) do(ctx context.Context, method, target string, body []byte) (*http.Response, error) {
	if c == nil {
		return nil, fmt.Errorf("emqxadmin: nil client")
	}
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.APIKey, c.APISecret)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return httpClient.Do(req)
}

func readAPIError(resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("emqxadmin: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
}

// UpsertUser creates a built-in-database MQTT user or updates the password on conflict.
func (c *Client) UpsertUser(ctx context.Context, userID, password string) error {
	userID = strings.TrimSpace(userID)
	password = strings.TrimSpace(password)
	if userID == "" || password == "" {
		return fmt.Errorf("emqxadmin: user id and password are required")
	}
	body, err := json.Marshal(map[string]string{
		"user_id":  userID,
		"password": password,
	})
	if err != nil {
		return err
	}
	resp, err := c.do(ctx, http.MethodPost, c.usersURL(""), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		return nil
	case http.StatusConflict:
		return c.updateUserPassword(ctx, userID, password)
	default:
		return readAPIError(resp)
	}
}

func (c *Client) updateUserPassword(ctx context.Context, userID, password string) error {
	body, err := json.Marshal(map[string]string{"password": password})
	if err != nil {
		return err
	}
	resp, err := c.do(ctx, http.MethodPut, c.usersURL(userID), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return readAPIError(resp)
}

// DeleteUser removes a built-in-database MQTT user.
func (c *Client) DeleteUser(ctx context.Context, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("emqxadmin: user id is required")
	}
	resp, err := c.do(ctx, http.MethodDelete, c.usersURL(userID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	return readAPIError(resp)
}

// EnsureUser creates or updates a built-in-database MQTT user.
func (c *Client) EnsureUser(ctx context.Context, userID, password string) error {
	return c.UpsertUser(ctx, userID, password)
}

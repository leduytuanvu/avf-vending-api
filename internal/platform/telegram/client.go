// Package telegram delivers formatted operational alerts through the Telegram Bot API.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	defaultAPIBaseURL  = "https://api.telegram.org"
	maxResponseBytes   = 64 << 10
	maxMessageChars    = 3800
	defaultHTTPTimeout = 10 * time.Second
)

// Config contains Telegram credentials supplied only by environment-backed application config.
type Config struct {
	BotToken string
	ChatID   string
	APIBase  string
	HTTP     *http.Client
}

// Client is a small Telegram Bot API client.
type Client struct {
	token   string
	chatID  string
	apiBase string
	http    *http.Client
}

// NewClient creates a client. Missing credentials leave the client disabled.
func NewClient(cfg Config) *Client {
	base := strings.TrimRight(strings.TrimSpace(cfg.APIBase), "/")
	if base == "" {
		base = defaultAPIBaseURL
	}
	hc := cfg.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &Client{
		token:   strings.TrimSpace(cfg.BotToken),
		chatID:  strings.TrimSpace(cfg.ChatID),
		apiBase: base,
		http:    hc,
	}
}

// Enabled reports whether both required environment-backed Telegram credentials are present.
func (c *Client) Enabled() bool {
	return c != nil && c.token != "" && c.chatID != ""
}

// IncidentAlert is the outbox notification payload.
type IncidentAlert struct {
	SchemaVersion     int             `json:"schema_version,omitempty"`
	Source            string          `json:"source,omitempty"` // app|server
	MachineID         string          `json:"machine_id,omitempty"`
	MachineCode       string          `json:"machine_code,omitempty"`
	MachineName       string          `json:"machine_name,omitempty"`
	SerialNumber      string          `json:"serial_number,omitempty"`
	SiteID            string          `json:"site_id,omitempty"`
	ReportedMachineID string          `json:"reported_machine_id,omitempty"`
	OccurrenceID      string          `json:"occurrence_id,omitempty"`
	Fingerprint       string          `json:"fingerprint,omitempty"`
	OccurrenceCount   int64           `json:"occurrence_count,omitempty"`
	Severity          string          `json:"severity"`
	Code              string          `json:"code"`
	Title             string          `json:"title"`
	DedupeKey         string          `json:"dedupe_key"`
	GroupKey          string          `json:"group_key"`
	Detail            json.RawMessage `json:"detail"`
	DocumentURL       string          `json:"document_url,omitempty"`
	Service           string          `json:"service,omitempty"`
	Operation         string          `json:"operation,omitempty"`
	TraceID           string          `json:"trace_id,omitempty"`
	CorrelationID     string          `json:"correlation_id,omitempty"`
}

// SendError classifies Telegram Bot API failures without embedding the bot token or full URL.
type SendError struct {
	Status     int
	RetryAfter time.Duration
	Permanent  bool
	Msg        string
}

func (e *SendError) Error() string {
	if e == nil {
		return "telegram: send error"
	}
	if e.Msg != "" {
		return e.Msg
	}
	return fmt.Sprintf("telegram: send failed status=%d permanent=%v", e.Status, e.Permanent)
}

func (e *SendError) Retryable() bool {
	return e != nil && !e.Permanent
}

// SendIncident sends an incident summary. Disabled clients return a permanent configuration error
// (callers must not treat disabled as successful delivery).
func (c *Client) SendIncident(ctx context.Context, alert IncidentAlert) error {
	if !c.Enabled() {
		return &SendError{Permanent: true, Msg: "telegram: client not configured"}
	}
	return c.sendMessage(ctx, FormatIncident(alert), alert.DocumentURL)
}

// SendText sends a plain text message (server emergency path).
func (c *Client) SendText(ctx context.Context, text string) error {
	if !c.Enabled() {
		return &SendError{Permanent: true, Msg: "telegram: client not configured"}
	}
	return c.sendMessage(ctx, BoundMessage(text), "")
}

func (c *Client) sendMessage(ctx context.Context, text, documentURL string) error {
	body, err := json.Marshal(struct {
		ChatID string `json:"chat_id"`
		Text   string `json:"text"`
	}{
		ChatID: c.chatID,
		Text:   BoundMessage(text),
	})
	if err != nil {
		return err
	}
	if err := c.doJSON(ctx, "sendMessage", body); err != nil {
		return err
	}
	if strings.TrimSpace(documentURL) != "" {
		docBody, err := json.Marshal(struct {
			ChatID   string `json:"chat_id"`
			Document string `json:"document"`
		}{ChatID: c.chatID, Document: strings.TrimSpace(documentURL)})
		if err != nil {
			return err
		}
		return c.doJSON(ctx, "sendDocument", docBody)
	}
	return nil
}

func (c *Client) doJSON(ctx context.Context, method string, body []byte) error {
	// Build URL without exposing token in returned errors.
	u := c.apiBase + "/bot" + c.token + "/" + method
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram: build request: %w", sanitizeErr(err))
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return &SendError{Permanent: false, Msg: "telegram: transport: " + sanitizeErr(err).Error()}
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, maxResponseBytes)
	raw, _ := io.ReadAll(limited)

	var apiResp struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
		ErrorCode   int    `json:"error_code"`
		Parameters  *struct {
			RetryAfter int `json:"retry_after"`
		} `json:"parameters"`
	}
	_ = json.Unmarshal(raw, &apiResp)

	if resp.StatusCode == http.StatusTooManyRequests || apiResp.ErrorCode == 429 {
		ra := time.Duration(0)
		if apiResp.Parameters != nil && apiResp.Parameters.RetryAfter > 0 {
			ra = time.Duration(apiResp.Parameters.RetryAfter) * time.Second
		}
		return &SendError{
			Status:     resp.StatusCode,
			RetryAfter: ra,
			Permanent:  false,
			Msg:        "telegram: rate limited",
		}
	}
	if resp.StatusCode >= 500 {
		return &SendError{Status: resp.StatusCode, Permanent: false, Msg: fmt.Sprintf("telegram: upstream status %d", resp.StatusCode)}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		permanent := resp.StatusCode >= 400 && resp.StatusCode < 500
		desc := strings.TrimSpace(apiResp.Description)
		if desc == "" {
			desc = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return &SendError{Status: resp.StatusCode, Permanent: permanent, Msg: "telegram: " + redactSecrets(desc)}
	}
	if !apiResp.OK {
		desc := strings.TrimSpace(apiResp.Description)
		if desc == "" {
			desc = "ok=false"
		}
		permanent := apiResp.ErrorCode >= 400 && apiResp.ErrorCode < 500 && apiResp.ErrorCode != 429
		return &SendError{Status: apiResp.ErrorCode, Permanent: permanent, Msg: "telegram: " + redactSecrets(desc)}
	}
	return nil
}

var (
	botPathSecretRE = regexp.MustCompile(`(?i)/bot[^/\s"'\\]+`)
	botTokenRE      = regexp.MustCompile(`(?i)\bbot\d+:[A-Za-z0-9_-]+`)
)

func sanitizeErr(err error) error {
	if err == nil {
		return nil
	}
	return errors.New(redactSecrets(err.Error()))
}

func redactSecrets(s string) string {
	s = botPathSecretRE.ReplaceAllString(s, "/bot[REDACTED]")
	s = botTokenRE.ReplaceAllString(s, "bot[REDACTED]")
	return s
}

// BoundMessage truncates optional diagnostics while keeping the first essential lines.
func BoundMessage(text string) string {
	text = strings.TrimRight(text, "\n")
	if len([]rune(text)) <= maxMessageChars {
		return text
	}
	runes := []rune(text)
	return string(runes[:maxMessageChars-20]) + "\n…[truncated]"
}

// FormatIncident renders a concise, text-only operational summary with clear machine identity.
func FormatIncident(a IncidentAlert) string {
	title := strings.TrimSpace(a.Title)
	if title == "" {
		title = "Machine incident"
	}
	source := strings.ToUpper(strings.TrimSpace(a.Source))
	if source == "" {
		source = "APP"
	}
	sev := strings.ToUpper(strings.TrimSpace(a.Severity))
	detail := parseIncidentDetail(a.Detail)

	var b strings.Builder
	eventLabel := detailString(detail, "status", "event_type", "event")
	if eventLabel == "" {
		eventLabel = strings.TrimSpace(a.Code)
	}
	fmt.Fprintf(&b, "🚨 [%s][%s] %s\n", source, sev, title)
	if eventLabel != "" {
		fmt.Fprintf(&b, "EVENT: %s\n", eventLabel)
	}
	b.WriteString("\n")

	if strings.TrimSpace(a.MachineID) != "" || strings.TrimSpace(a.MachineCode) != "" {
		b.WriteString("MACHINE\n")
		if v := firstNonEmptyString(strings.TrimSpace(a.MachineCode), detailString(detail, "machine_code")); v != "" {
			fmt.Fprintf(&b, "Code: %s\n", v)
		}
		if v := strings.TrimSpace(a.MachineID); v != "" {
			fmt.Fprintf(&b, "ID: %s\n", v)
		}
		if v := firstNonEmptyString(strings.TrimSpace(a.MachineName), detailString(detail, "machine_name")); v != "" {
			fmt.Fprintf(&b, "Name: %s\n", v)
		}
		if v := strings.TrimSpace(a.SerialNumber); v != "" {
			fmt.Fprintf(&b, "Serial: %s\n", v)
		}
		b.WriteString("\n")
	}

	if v := firstNonEmptyString(detailString(detail, "machine_location", "site_name"), strings.TrimSpace(a.SiteID)); v != "" {
		b.WriteString("LOCATION\n")
		fmt.Fprintf(&b, "%s\n\n", v)
	}

	if v := firstNonEmptyString(detailString(detail, "occurred_at"), detailString(detail, "detected_at")); v != "" {
		b.WriteString("TIME\n")
		fmt.Fprintf(&b, "%s", v)
		if tz := detailString(detail, "timezone", "time_zone"); tz != "" {
			fmt.Fprintf(&b, " (%s)", tz)
		}
		b.WriteString("\n")
		if src := detailString(detail, "event_time_source"); src != "" {
			fmt.Fprintf(&b, "Source: %s\n", src)
		}
		b.WriteString("\n")
	}

	if v := strings.TrimSpace(a.OccurrenceID); v != "" {
		b.WriteString("INCIDENT\n")
		fmt.Fprintf(&b, "Occurrence: %s\n", v)
		fp := strings.TrimSpace(a.Fingerprint)
		if fp == "" {
			fp = strings.TrimSpace(a.DedupeKey)
		}
		if fp != "" {
			fmt.Fprintf(&b, "Fingerprint: %s\n", fp)
		}
		if a.OccurrenceCount > 0 {
			fmt.Fprintf(&b, "Count: %d\n", a.OccurrenceCount)
		}
		b.WriteString("\n")
	}

	if v := detailString(detail, "sell_readiness", "selling_impact"); v != "" {
		b.WriteString("SELLING\n")
		fmt.Fprintf(&b, "%s\n\n", v)
	}
	if v := detailString(detail, "primary_blocker", "primary_reason", "primaryBlocker"); v != "" {
		b.WriteString("PRIMARY REASON\n")
		fmt.Fprintf(&b, "%s\n\n", v)
	}
	if v := detailString(detail, "blocker_codes", "blockers", "block_reasons"); v != "" {
		b.WriteString("BLOCKERS\n")
		fmt.Fprintf(&b, "%s\n\n", truncateField(v, 400))
	}

	hardware := joinDetailFields(detail, "bill_state", "tcn_state", "network_state")
	if hardware != "" {
		b.WriteString("HARDWARE / NETWORK\n")
		fmt.Fprintf(&b, "%s\n\n", hardware)
	}
	if v := detailString(detail, "clock_skew_ms", "clockSkewMs"); v != "" {
		b.WriteString("CLOCK\n")
		fmt.Fprintf(&b, "Skew: %s ms\n\n", v)
	}
	if v := detailString(detail, "maintenance_source", "maintenanceSource"); v != "" {
		b.WriteString("MAINTENANCE\n")
		fmt.Fprintf(&b, "%s\n\n", v)
	}
	if v := detailString(detail, "recovery_status", "resolved_occurrence_id", "resolvedOccurrenceId"); v != "" {
		b.WriteString("RECOVERY\n")
		fmt.Fprintf(&b, "%s", v)
		if dur := detailString(detail, "duration_ms", "durationMs"); dur != "" {
			fmt.Fprintf(&b, " (%sms)", dur)
		}
		b.WriteString("\n\n")
	}

	if v := strings.TrimSpace(a.TraceID); v != "" {
		fmt.Fprintf(&b, "Trace: %s\n", v)
	}
	if v := strings.TrimSpace(a.CorrelationID); v != "" {
		fmt.Fprintf(&b, "Correlation: %s\n", v)
	}

	return BoundMessage(strings.TrimRight(b.String(), "\n"))
}

func parseIncidentDetail(raw json.RawMessage) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil
	}
	out := make(map[string]string)
	flattenIncidentDetail("", root, out)
	return out
}

func flattenIncidentDetail(prefix string, v any, out map[string]string) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			flattenIncidentDetail(key, child, out)
		}
	case []any:
		if len(x) == 0 {
			return
		}
		parts := make([]string, 0, len(x))
		for _, child := range x {
			switch c := child.(type) {
			case string:
				parts = append(parts, c)
			default:
				parts = append(parts, fmt.Sprint(c))
			}
		}
		out[prefix] = strings.Join(parts, ", ")
	default:
		if prefix == "" {
			return
		}
		out[prefix] = strings.TrimSpace(fmt.Sprint(x))
	}
}

func detailString(detail map[string]string, keys ...string) string {
	if detail == nil {
		return ""
	}
	for _, k := range keys {
		if v := strings.TrimSpace(detail[k]); v != "" {
			return v
		}
		alt := strings.ToLower(strings.ReplaceAll(k, "_", ""))
		for dk, dv := range detail {
			nk := strings.ToLower(strings.ReplaceAll(dk, "_", ""))
			if nk == alt && strings.TrimSpace(dv) != "" {
				return strings.TrimSpace(dv)
			}
		}
	}
	return ""
}

func joinDetailFields(detail map[string]string, keys ...string) string {
	if detail == nil {
		return ""
	}
	var parts []string
	for _, k := range keys {
		if v := detailString(detail, k); v != "" {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return strings.Join(parts, "; ")
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func truncateField(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// IsRetryable reports whether err should Nak/retry.
func IsRetryable(err error) bool {
	var se *SendError
	if errors.As(err, &se) {
		return se.Retryable()
	}
	return err != nil
}

// IsPermanent reports whether err should DLQ.
func IsPermanent(err error) bool {
	var se *SendError
	if errors.As(err, &se) {
		return se.Permanent
	}
	return false
}

// RetryAfter extracts Telegram retry_after when present.
func RetryAfter(err error) time.Duration {
	var se *SendError
	if errors.As(err, &se) {
		return se.RetryAfter
	}
	return 0
}

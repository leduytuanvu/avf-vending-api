package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
)

// HTTPStatusGateway loads provider-normalized payment state over HTTP (real outbound I/O; no synthetic PSP data).
// URLTemplate must contain exactly one "%s" placeholder expanded with the payment id string (UUID).
type HTTPStatusGateway struct {
	client      *http.Client
	urlTemplate string
	bearerToken string
}

// NewHTTPStatusGateway constructs a gateway. urlTemplate must include "%s" for payment id.
func NewHTTPStatusGateway(urlTemplate, bearerToken string) (*HTTPStatusGateway, error) {
	if strings.Count(urlTemplate, "%s") != 1 {
		return nil, fmt.Errorf("payments: RECONCILER_PAYMENT_PROBE_URL_TEMPLATE must contain exactly one %%s placeholder for payment id")
	}
	return &HTTPStatusGateway{
		client: &http.Client{
			Timeout: 25 * time.Second,
		},
		urlTemplate: strings.TrimSpace(urlTemplate),
		bearerToken: strings.TrimSpace(bearerToken),
	}, nil
}

var _ domaincommerce.PaymentProviderGateway = (*HTTPStatusGateway)(nil)

type probeHTTPResponse struct {
	NormalizedState string          `json:"normalized_state"`
	ProviderHint    json.RawMessage `json:"provider_hint"`
}

// FetchPaymentStatus performs a GET request and decodes JSON from the configured provider endpoint.
func (g *HTTPStatusGateway) FetchPaymentStatus(ctx context.Context, lookup domaincommerce.PaymentProviderLookup) (domaincommerce.PaymentStatusSnapshot, error) {
	if g == nil || g.client == nil {
		return domaincommerce.PaymentStatusSnapshot{}, fmt.Errorf("payments: nil HTTPStatusGateway")
	}
	u := fmt.Sprintf(g.urlTemplate, lookup.PaymentID.String())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return domaincommerce.PaymentStatusSnapshot{}, err
	}
	if g.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+g.bearerToken)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-AVF-Payment-Id", lookup.PaymentID.String())
	req.Header.Set("X-AVF-Order-Id", lookup.OrderID.String())
	req.Header.Set("X-AVF-Provider", lookup.Provider)

	resp, err := g.client.Do(req)
	if err != nil {
		return domaincommerce.PaymentStatusSnapshot{}, fmt.Errorf("payments: probe request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return domaincommerce.PaymentStatusSnapshot{}, fmt.Errorf("payments: probe read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return domaincommerce.PaymentStatusSnapshot{}, fmt.Errorf("payments: probe HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var parsed probeHTTPResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return domaincommerce.PaymentStatusSnapshot{}, fmt.Errorf("payments: probe json: %w", err)
	}
	st := strings.TrimSpace(strings.ToLower(parsed.NormalizedState))
	if st == "" {
		return domaincommerce.PaymentStatusSnapshot{}, fmt.Errorf("payments: probe response missing normalized_state")
	}
	hint := []byte(parsed.ProviderHint)
	if len(hint) == 0 {
		hint = body
	}
	return domaincommerce.PaymentStatusSnapshot{
		NormalizedState: st,
		ProviderHint:    hint,
	}, nil
}

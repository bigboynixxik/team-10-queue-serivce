// Package avito is the HTTP client for AvitoBackend, the external system that
// owns the physical stock. Queue Service only mirrors that number locally, so
// this client is the single place where the two systems agree on it
// (docs/design_context.md, пп. 8 и 9).
package avito

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// InternalTokenHeader authorises service-to-service calls. AvitoBackend is not a
// browser and has no session, so a shared secret is the whole mechanism — this
// closes the service-to-service authorisation question for the MVP
// (docs/design_context.md, раздел 13).
const InternalTokenHeader = "X-Internal-Token" //nolint:gosec // header name, not a credential

const defaultTimeout = 5 * time.Second

// IdempotencyKeyHeader lets AvitoBackend safely deduplicate retried stock
// decrement requests.
const IdempotencyKeyHeader = "Idempotency-Key"

// ErrUnexpectedStatus is returned when AvitoBackend answers with an unexpected code.
var ErrUnexpectedStatus = errors.New("avito: unexpected status")

// Client talks to AvitoBackend over HTTP.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New creates a client for the given AvitoBackend base URL. An empty timeout
// falls back to defaultTimeout — a stuck external call must not hold a queue
// slot forever.
func New(baseURL, token string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	return &Client{
		baseURL: baseURL,
		token:   token,
		http:    &http.Client{Timeout: timeout},
	}
}

type stockResponse struct {
	Available int `json:"available"`
}

type decrementRequest struct {
	Decrement int `json:"decrement"`
}

// GetInitialStock reads the physical stock of a product. It is called once per
// product, when Queue Service first sees it and has nothing cached yet.
func (c *Client) GetInitialStock(ctx context.Context, productID string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.stockURL(productID), nil)
	if err != nil {
		return 0, fmt.Errorf("avito.GetInitialStock build request: %w", err)
	}
	c.authorize(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("avito.GetInitialStock: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("avito.GetInitialStock: %w: %d", ErrUnexpectedStatus, resp.StatusCode)
	}

	var body stockResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, fmt.Errorf("avito.GetInitialStock decode: %w", err)
	}

	return body.Available, nil
}

// DecrementStock reports a sale. AvitoBackend is the source of truth for the
// physical stock, so this call is what makes a purchase real outside our service.
func (c *Client) DecrementStock(ctx context.Context, idempotencyKey string, productID string, quantity int) error {
	payload, err := json.Marshal(decrementRequest{Decrement: quantity})
	if err != nil {
		return fmt.Errorf("avito.DecrementStock encode: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, c.stockURL(productID), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("avito.DecrementStock build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(IdempotencyKeyHeader, idempotencyKey)
	c.authorize(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("avito.DecrementStock: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("avito.DecrementStock: %w: %d", ErrUnexpectedStatus, resp.StatusCode)
	}

	return nil
}

func (c *Client) stockURL(productID string) string {
	return fmt.Sprintf("%s/products/%s/stock", c.baseURL, productID)
}

func (c *Client) authorize(req *http.Request) {
	if c.token != "" {
		req.Header.Set(InternalTokenHeader, c.token)
	}
}

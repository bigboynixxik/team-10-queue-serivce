// Command avitomock stands in for AvitoBackend — the external Avito system that
// owns the physical stock and the checkout. It is deliberately dumb: it keeps
// stock in memory and never validates a purchase, because everything it does is
// out of scope of the case (docs/hackathon_case.md, «Что уже реализовано»).
//
// It exists so the user journey can be walked end to end: without a checkout
// there is nowhere to pay, and PURCHASED could only be reached by calling an
// internal endpoint by hand.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const (
	readHeaderTimeout = 10 * time.Second
	shutdownTimeout   = 10 * time.Second
	// internalTokenHeader mirrors avito.InternalTokenHeader on the other side.
	internalTokenHeader  = "X-Internal-Token" //nolint:gosec // header name, not a credential
	idempotencyKeyHeader = "Idempotency-Key"
)

var (
	errQueueService     = errors.New("queue service rejected the event")
	errRightUnavailable = errors.New("право на покупку больше не действует")
)

// stock keeps the physical stock per product. A product nobody asked about yet
// starts at defaultStock, which is what makes the mock usable without seeding.
type stock struct {
	mu           sync.Mutex
	counts       map[string]int
	processed    map[string]stockDecrementOperation
	defaultStock int
}

func newStock(defaultStock int) *stock {
	return &stock{
		counts:       make(map[string]int),
		processed:    make(map[string]stockDecrementOperation),
		defaultStock: defaultStock,
	}
}

type stockDecrementOperation struct {
	ProductID string
	Quantity  int
	Left      int
}

func (s *stock) get(productID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.counts[productID]; !ok {
		s.counts[productID] = s.defaultStock
	}

	return s.counts[productID]
}

// take removes quantity from the product and reports what is left. Repeated
// calls with the same idempotency key and payload return the original result
// without decrementing again.
func (s *stock) take(productID string, quantity int, idempotencyKey string) (left int, replay bool, conflict bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	operation := stockDecrementOperation{ProductID: productID, Quantity: quantity}
	if previous, ok := s.processed[idempotencyKey]; ok {
		if previous.ProductID != operation.ProductID || previous.Quantity != operation.Quantity {
			return previous.Left, false, true
		}

		return previous.Left, true, false
	}

	left, ok := s.counts[productID]
	if !ok {
		left = s.defaultStock
	}

	left -= quantity
	if left < 0 {
		left = 0
	}
	s.counts[productID] = left
	operation.Left = left
	s.processed[idempotencyKey] = operation

	return left, false, false
}

func (s *stock) set(productID string, available int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.counts[productID] = available
}

func main() {
	if err := run(); err != nil {
		slog.Error("avitomock stopped with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		port          = env("PORT", "9090")
		queueBaseURL  = env("QUEUE_BASE_URL", "http://backend:8080")
		internalToken = env("INTERNAL_TOKEN", "")
		defaultStock  = envInt("DEFAULT_STOCK", 3)
	)

	srv := &server{
		stock:         newStock(defaultStock),
		queueBaseURL:  queueBaseURL,
		internalToken: internalToken,
		http:          &http.Client{Timeout: 5 * time.Second},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /products/{product_id}/stock", srv.getStock)
	mux.HandleFunc("PATCH /products/{product_id}/stock", srv.patchStock)
	mux.HandleFunc("PUT /products/{product_id}/stock", srv.putStock)
	mux.HandleFunc("GET /checkout", srv.checkoutPage)
	mux.HandleFunc("POST /checkout/pay", srv.pay)

	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)

	go func() {
		slog.Info("avitomock started", "addr", httpServer.Addr, "default_stock", defaultStock)

		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	return httpServer.Shutdown(shutdownCtx)
}

type server struct {
	stock         *stock
	queueBaseURL  string
	internalToken string
	http          *http.Client
}

// authorized guards the service-to-service endpoints. Queue Service is the only
// legitimate caller; a browser must never reach them.
func (s *server) authorized(w http.ResponseWriter, r *http.Request) bool {
	if s.internalToken == "" || r.Header.Get(internalTokenHeader) == s.internalToken {
		return true
	}

	w.WriteHeader(http.StatusUnauthorized)

	return false
}

func (s *server) getStock(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(w, r) {
		return
	}

	writeJSON(w, http.StatusOK, map[string]int{"available": s.stock.get(r.PathValue("product_id"))})
}

func (s *server) patchStock(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(w, r) {
		return
	}

	idempotencyKey := r.Header.Get(idempotencyKeyHeader)
	if idempotencyKey == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var body struct {
		Decrement int `json:"decrement"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Decrement <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	productID := r.PathValue("product_id")
	left, replay, conflict := s.stock.take(productID, body.Decrement, idempotencyKey)
	if conflict {
		w.WriteHeader(http.StatusConflict)
		return
	}

	if replay {
		slog.Info("stock decrement replayed", "product_id", productID, "by", body.Decrement, "left", left)
	} else {
		slog.Info("stock decremented", "product_id", productID, "by", body.Decrement, "left", left)
	}

	writeJSON(w, http.StatusOK, map[string]int{"available": left})
}

// putStock is not part of the AvitoBackend contract — it exists so a demo can
// set a product to exactly one unit and show two buyers racing for it.
func (s *server) putStock(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Available int `json:"available"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Available < 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	s.stock.set(r.PathValue("product_id"), body.Available)
	writeJSON(w, http.StatusOK, map[string]int{"available": body.Available})
}

func (s *server) checkoutPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := checkoutTemplate.Execute(w, map[string]string{
		"Token":     r.URL.Query().Get("token"),
		"ProductID": r.URL.Query().Get("product_id"),
	}); err != nil {
		slog.Error("render checkout", "error", err)
	}
}

// pay is what the checkout button hits. The internal token never reaches the
// browser: the page calls this handler, and it calls Queue Service.
func (s *server) pay(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token     string `json:"token"`
		ProductID string `json:"product_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Token == "" || body.ProductID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := s.validateRight(r.Context(), body.Token, body.ProductID); err != nil {
		slog.Error("validate checkout right", "error", err)
		status := http.StatusBadGateway
		if errors.Is(err, errRightUnavailable) {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})

		return
	}

	orderID := "order-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if err := s.reportPayment(r.Context(), body.Token, orderID); err != nil {
		slog.Error("report payment", "error", err)
		status := http.StatusBadGateway
		if errors.Is(err, errRightUnavailable) {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})

		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"order_id": orderID})
}

func (s *server) validateRight(ctx context.Context, token, productID string) error {
	payload, err := json.Marshal(map[string]string{"product_id": productID})
	if err != nil {
		return fmt.Errorf("encode validation: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/internal/rights/%s/validate", s.queueBaseURL, token)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build validation request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if s.internalToken != "" {
		req.Header.Set(internalTokenHeader, s.internalToken)
	}

	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("call queue service: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound, http.StatusConflict, http.StatusForbidden:
		return errRightUnavailable
	default:
		return fmt.Errorf("%w: %d", errQueueService, resp.StatusCode)
	}
}

func (s *server) reportPayment(ctx context.Context, token, orderID string) error {
	payload, err := json.Marshal(map[string]string{"event": "payment_succeeded", "order_id": orderID})
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/rights/%s/events", s.queueBaseURL, token)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if s.internalToken != "" {
		req.Header.Set(internalTokenHeader, s.internalToken)
	}

	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("call queue service: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return errRightUnavailable
	}
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("%w: %d", errQueueService, resp.StatusCode)
	}

	return nil
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("write response", "error", err)
	}
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}

func envInt(name string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil {
		return fallback
	}

	return value
}

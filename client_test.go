package storlaunch

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// envelope wraps `data` in the API envelope shape callers will see.
func envelope(t *testing.T, data any) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"data":  data,
		"error": nil,
		"meta": map[string]any{
			"requestId": "req_test_1",
			"timestamp": "2026-01-01T00:00:00Z",
		},
	})
	if err != nil {
		t.Fatalf("envelope marshal: %v", err)
	}
	return b
}

// newTestClient spins up an httptest.Server and points a Client at it.
// The handler receives every request and is responsible for writing
// JSON envelopes (or error envelopes) back.
func newTestClient(t *testing.T, handler http.HandlerFunc, opts ...func(*ClientOptions)) (*Client, *httptest.Server, *recordingRoundTripper) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	rrt := &recordingRoundTripper{base: http.DefaultTransport}
	co := ClientOptions{
		APIKey:  "sk_test_go",
		BaseURL: srv.URL,
		HTTP:    &http.Client{Timeout: 5 * time.Second, Transport: rrt},
	}
	for _, fn := range opts {
		fn(&co)
	}
	c, err := NewClient(co)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c, srv, rrt
}

// recordingRoundTripper captures every outgoing request so tests can
// assert on the exact headers + body the SDK put on the wire.
type recordingRoundTripper struct {
	base     http.RoundTripper
	requests []capturedRequest
}

type capturedRequest struct {
	Method  string
	URL     string
	Headers http.Header
	Body    []byte
}

func (r *recordingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		body = b
		req.Body = io.NopCloser(strings.NewReader(string(b)))
	}
	r.requests = append(r.requests, capturedRequest{
		Method: req.Method, URL: req.URL.String(),
		Headers: req.Header.Clone(), Body: body,
	})
	return r.base.RoundTrip(req)
}

func (r *recordingRoundTripper) last() capturedRequest {
	return r.requests[len(r.requests)-1]
}

// ─── Construction ────────────────────────────────────────────────────────

func TestNewClient_RequiresAPIKey(t *testing.T) {
	t.Setenv("STORLAUNCH_API_KEY", "")
	if _, err := NewClient(ClientOptions{}); err == nil {
		t.Fatal("expected error for missing APIKey")
	}
}

func TestNewClient_DefaultsBaseURL(t *testing.T) {
	t.Setenv("STORLAUNCH_BASE_URL", "")
	c, err := NewClient(ClientOptions{APIKey: "sk_test_x"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.BaseURL() != DefaultBaseURL {
		t.Fatalf("expected default base URL, got %q", c.BaseURL())
	}
}

func TestNewClient_TrimsTrailingSlash(t *testing.T) {
	c, err := NewClient(ClientOptions{APIKey: "sk_test_x", BaseURL: "https://x.test/////"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.BaseURL() != "https://x.test" {
		t.Fatalf("expected trailing slashes trimmed, got %q", c.BaseURL())
	}
}

func TestNewClient_EnvFallback(t *testing.T) {
	t.Setenv("STORLAUNCH_API_KEY", "sk_live_env")
	t.Setenv("STORLAUNCH_BASE_URL", "https://from-env.test")
	c, err := NewClient(ClientOptions{})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.apiKey != "sk_live_env" || c.BaseURL() != "https://from-env.test" {
		t.Fatalf("env fallback failed: %+v", c)
	}
}

// ─── Header wire-checks ──────────────────────────────────────────────────

func TestRequest_WritesExpectedHeaders(t *testing.T) {
	c, _, rrt := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(envelope(t, map[string]any{"ok": true}))
	})

	_, err := c.Payment.Plans.Get(context.Background(), "plan_1")
	if err != nil {
		t.Fatalf("Get plan: %v", err)
	}

	req := rrt.last()
	if auth := req.Headers.Get("Authorization"); auth != "Bearer sk_test_go" {
		t.Fatalf("authorization header = %q, want the API key as a Bearer token", auth)
	}
	for name := range req.Headers {
		if strings.HasPrefix(strings.ToLower(name), "x-storlaunch") {
			t.Fatalf("unexpected %s header: the API authenticates the key alone", name)
		}
	}
	if req.Headers.Get("Accept") != "application/json" {
		t.Fatalf("missing Accept header")
	}
	if req.Headers.Get("Content-Type") != "" {
		t.Fatalf("GET request should not set Content-Type, got %q", req.Headers.Get("Content-Type"))
	}
	if req.Headers.Get("Idempotency-Key") != "" {
		t.Fatalf("GET request should not set Idempotency-Key, got %q", req.Headers.Get("Idempotency-Key"))
	}
}

func TestRequest_BodyTriggersContentType(t *testing.T) {
	c, _, rrt := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(envelope(t, map[string]any{"id": "cs_1"}))
	})

	_, err := c.Payment.CheckoutSessions.Create(context.Background(), Object{"amount": 1500})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	req := rrt.last()
	if req.Headers.Get("Content-Type") != "application/json" {
		t.Fatalf("expected JSON Content-Type, got %q", req.Headers.Get("Content-Type"))
	}
	if !strings.HasPrefix(req.Headers.Get("X-Idempotency-Key"), "idem_") {
		t.Fatalf("expected auto X-Idempotency-Key, got %q", req.Headers.Get("X-Idempotency-Key"))
	}
	if req.Headers.Get("Idempotency-Key") != req.Headers.Get("X-Idempotency-Key") {
		t.Fatalf("Idempotency-Key should carry the same key, got %q", req.Headers.Get("Idempotency-Key"))
	}
	if string(req.Body) == "" || !strings.Contains(string(req.Body), `"amount":1500`) {
		t.Fatalf("body not serialized correctly: %q", string(req.Body))
	}
}

// ─── Route round-trips ───────────────────────────────────────────────────

func TestRoundTrip_PaymentPlansList(t *testing.T) {
	c, _, rrt := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/payment/plans" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write(envelope(t, []any{
			map[string]any{"id": "plan_1"},
			map[string]any{"id": "plan_2"},
		}))
	})
	got, err := c.Payment.Plans.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 plans, got %d", len(got))
	}
	if !strings.HasSuffix(rrt.last().URL, "/api/v1/payment/plans") {
		t.Fatalf("unexpected URL: %q", rrt.last().URL)
	}
}

func TestRoundTrip_QueryParamsEncoded(t *testing.T) {
	c, _, rrt := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(envelope(t, []any{}))
	})
	_, err := c.Payment.Plans.List(context.Background(), map[string]any{
		"status": "active",
		"limit":  10,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	u := rrt.last().URL
	if !strings.Contains(u, "status=active") || !strings.Contains(u, "limit=10") {
		t.Fatalf("query params missing: %q", u)
	}
}

func TestRoundTrip_StorefrontProductsCreate(t *testing.T) {
	c, _, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/storefront/products" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		var got map[string]any
		_ = json.Unmarshal(b, &got)
		if got["name"] != "Widget" {
			t.Errorf("body missing name=Widget, got %v", got)
		}
		_, _ = w.Write(envelope(t, map[string]any{"id": "prod_1"}))
	})
	out, err := c.Storefront.Products.Create(context.Background(), Object{"name": "Widget"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if out["id"] != "prod_1" {
		t.Fatalf("expected id=prod_1, got %v", out["id"])
	}
}

func TestRoundTrip_BillingCheckout(t *testing.T) {
	c, _, rrt := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(envelope(t, map[string]any{"checkoutUrl": "https://pay.test/abc"}))
	})
	out, err := c.Billing.Checkout(context.Background(), BillingCheckoutInput{Plan: "pro", Interval: "year"})
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if out["checkoutUrl"] != "https://pay.test/abc" {
		t.Fatalf("unexpected data: %v", out)
	}
	if !strings.HasSuffix(rrt.last().URL, "/api/v1/billing/plugipay-invoice") {
		t.Fatalf("unexpected URL: %q", rrt.last().URL)
	}
	if body := string(rrt.last().Body); body != `{"plan":"pro","interval":"year"}` {
		t.Fatalf("unexpected body %q", body)
	}
}

func TestRoundTrip_ReportsPnL(t *testing.T) {
	c, _, rrt := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(envelope(t, map[string]any{"revenue": 12345}))
	})
	_, err := c.Reports.PnL(context.Background(), DateRange{From: "2026-01-01", To: "2026-01-31"})
	if err != nil {
		t.Fatalf("pnl: %v", err)
	}
	u := rrt.last().URL
	if !strings.Contains(u, "from=2026-01-01") || !strings.Contains(u, "to=2026-01-31") {
		t.Fatalf("query missing: %q", u)
	}
}

func TestRoundTrip_AccountDomainsAdd(t *testing.T) {
	c, _, rrt := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(envelope(t, map[string]any{"id": "dom_1", "domain": "shop.example.com"}))
	})
	out, err := c.Account.Domains.Add(context.Background(), DomainInput{Domain: "shop.example.com"})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if out["domain"] != "shop.example.com" {
		t.Fatalf("unexpected: %v", out)
	}
	if !strings.Contains(string(rrt.last().Body), `"domain":"shop.example.com"`) {
		t.Fatalf("body missing domain field: %q", rrt.last().Body)
	}
}

func TestRoundTrip_SubscriptionsCancelIs204(t *testing.T) {
	c, _, rrt := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	out, err := c.Payment.Subscriptions.Cancel(context.Background(), "sub_9", true)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if out != nil {
		t.Fatalf("a 204 has no data, got %v", out)
	}
	if req := rrt.last(); req.Method != http.MethodDelete || !strings.HasSuffix(req.URL, "/api/v1/payment/subscriptions/sub_9?immediate=true") {
		t.Fatalf("unexpected request %s %s", req.Method, req.URL)
	}
}

func TestRoundTrip_ExportLedgerReturnsCSV(t *testing.T) {
	c, _, rrt := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		_, _ = w.Write([]byte("id,amount\nle_1,100\n"))
	})
	csv, err := c.Reports.ExportLedger(context.Background())
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if csv != "id,amount\nle_1,100\n" {
		t.Fatalf("unexpected csv %q", csv)
	}
	if !strings.HasSuffix(rrt.last().URL, "/api/v1/ledger/entries.csv") {
		t.Fatalf("unexpected URL: %q", rrt.last().URL)
	}
}

// ─── Error envelope ──────────────────────────────────────────────────────

func TestRequest_ErrorEnvelopeWithRequestID(t *testing.T) {
	c, _, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		body, _ := json.Marshal(map[string]any{
			"data": nil,
			"error": map[string]any{
				"code":    "invalid_request",
				"message": "amount must be positive",
			},
			"meta": map[string]any{
				"requestId": "req_xyz",
				"timestamp": "2026-01-01T00:00:00Z",
			},
		})
		_, _ = w.Write(body)
	})
	_, err := c.Payment.CheckoutSessions.Create(context.Background(), Object{"amount": -1})
	if err == nil {
		t.Fatal("expected error")
	}
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if e.Status != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", e.Status)
	}
	if e.Code != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", e.Code)
	}
	if e.Message != "amount must be positive" {
		t.Errorf("message = %q", e.Message)
	}
	if e.RequestID != "req_xyz" {
		t.Errorf("requestId = %q", e.RequestID)
	}
	if !strings.Contains(e.Error(), "req_xyz") {
		t.Errorf("Error() should include requestId, got %q", e.Error())
	}
}

func TestRequest_NonJSONResponse(t *testing.T) {
	c, _, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<html>nginx ate it</html>"))
	})
	_, err := c.Payment.Plans.List(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	e, ok := err.(*Error)
	if !ok || e.Code != "invalid_response" {
		t.Fatalf("expected invalid_response error, got %+v", err)
	}
}

// ─── Webhook signature verifier ──────────────────────────────────────────

func TestVerifyWebhookSignature_RoundTrip(t *testing.T) {
	secret := "whsec_test_secret_1234567890"
	body := []byte(`{"id":"evt_x","type":"storlaunch.order.paid.v1","data":{"orderId":"ord_1"}}`)
	timestamp := int64(1_750_000_000)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.", timestamp) + string(body)))
	sigHeader := fmt.Sprintf("t=%d,v1=%s", timestamp, hex.EncodeToString(mac.Sum(nil)))

	clock := func() time.Time { return time.Unix(timestamp+10, 0) }
	if !VerifyWebhookSignature(body, sigHeader, secret, &VerifyWebhookSignatureOptions{Now: clock}) {
		t.Fatal("valid signature rejected")
	}
}

func TestVerifyWebhookSignature_TamperedBody(t *testing.T) {
	secret := "whsec_x"
	timestamp := int64(1_750_000_000)
	body := []byte(`{"hello":"world"}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.", timestamp) + string(body)))
	good := hex.EncodeToString(mac.Sum(nil))

	tampered := []byte(`{"hello":"mars"}`)
	sigHeader := fmt.Sprintf("t=%d,v1=%s", timestamp, good)
	clock := func() time.Time { return time.Unix(timestamp, 0) }

	if VerifyWebhookSignature(tampered, sigHeader, secret, &VerifyWebhookSignatureOptions{Now: clock}) {
		t.Fatal("tampered body verified true")
	}
}

func TestVerifyWebhookSignature_Expired(t *testing.T) {
	secret := "whsec_x"
	timestamp := int64(1_750_000_000)
	body := []byte(`{}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.", timestamp) + string(body)))
	sigHeader := fmt.Sprintf("t=%d,v1=%s", timestamp, hex.EncodeToString(mac.Sum(nil)))

	clock := func() time.Time { return time.Unix(timestamp+600, 0) } // 10 min — past default 5 min tolerance
	if VerifyWebhookSignature(body, sigHeader, secret, &VerifyWebhookSignatureOptions{Now: clock}) {
		t.Fatal("stale signature accepted")
	}
}

func TestVerifyWebhookSignature_MalformedHeader(t *testing.T) {
	if VerifyWebhookSignature([]byte("x"), "not-a-valid-header", "s", nil) {
		t.Fatal("bogus header accepted")
	}
	if VerifyWebhookSignature([]byte("x"), "", "s", nil) {
		t.Fatal("empty header accepted")
	}
	if VerifyWebhookSignature([]byte("x"), "t=abc,v1=deadbeef", "s", nil) {
		t.Fatal("non-numeric timestamp accepted")
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────

func hmacHex(t *testing.T, secret, message string) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

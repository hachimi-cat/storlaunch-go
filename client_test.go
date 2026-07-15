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

// fixedClock returns a deterministic clock for HMAC tests.
func fixedClock(unix int64) func() time.Time {
	return func() time.Time { return time.Unix(unix, 0) }
}

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
		KeyID:   "AKIASTOR_TEST",
		Secret:  "sk_test_secret",
		BaseURL: srv.URL,
		HTTP:    &http.Client{Timeout: 5 * time.Second, Transport: rrt},
		Clock:   fixedClock(1_750_000_000),
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

func TestNewClient_RequiresKeyIDAndSecret(t *testing.T) {
	if _, err := NewClient(ClientOptions{KeyID: "", Secret: "x"}); err == nil {
		t.Fatal("expected error for missing KeyID")
	}
	if _, err := NewClient(ClientOptions{KeyID: "x", Secret: ""}); err == nil {
		t.Fatal("expected error for missing Secret")
	}
}

func TestNewClient_DefaultsBaseURL(t *testing.T) {
	c, err := NewClient(ClientOptions{KeyID: "ak", Secret: "sk"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.BaseURL() != DefaultBaseURL {
		t.Fatalf("expected default base url %q, got %q", DefaultBaseURL, c.BaseURL())
	}
}

func TestNewClient_TrimsTrailingSlash(t *testing.T) {
	c, err := NewClient(ClientOptions{KeyID: "ak", Secret: "sk", BaseURL: "https://x.test/////"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.BaseURL() != "https://x.test" {
		t.Fatalf("expected trailing slashes trimmed, got %q", c.BaseURL())
	}
}

func TestNewClient_EnvFallback(t *testing.T) {
	t.Setenv("STORLAUNCH_KEY_ID", "ak_env")
	t.Setenv("STORLAUNCH_SECRET", "sk_env")
	t.Setenv("STORLAUNCH_BASE_URL", "https://from-env.test")
	c, err := NewClient(ClientOptions{})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.keyID != "ak_env" || c.secret != "sk_env" || c.BaseURL() != "https://from-env.test" {
		t.Fatalf("env fallback failed: %+v", c)
	}
}

// ─── ForMerchant cloning ─────────────────────────────────────────────────

func TestForMerchant_ClonesAndPinsAccount(t *testing.T) {
	c, err := NewClient(ClientOptions{KeyID: "ak", Secret: "sk", BaseURL: "https://x.test"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	clone := c.ForMerchant("acc_123")

	if clone == c {
		t.Fatal("ForMerchant returned the same pointer; expected a clone")
	}
	if clone.OnBehalfOf() != "acc_123" {
		t.Fatalf("clone obo = %q, want acc_123", clone.OnBehalfOf())
	}
	if c.OnBehalfOf() != "" {
		t.Fatalf("original client should not have been mutated, got obo=%q", c.OnBehalfOf())
	}
	if clone.http != c.http {
		t.Fatal("underlying http.Client should be shared between original and clone")
	}
	if clone.Payment == c.Payment {
		t.Fatal("resource namespaces should be re-installed on the clone")
	}
}

// ─── HMAC signing ────────────────────────────────────────────────────────

func TestSign_FormatMatchesNodeAndPython(t *testing.T) {
	c, _ := NewClient(ClientOptions{
		KeyID: "ak", Secret: "sk", BaseURL: "https://x.test", Clock: fixedClock(1_750_000_000),
	})

	sig, ts := c.sign("GET", "/api/v1/x", nil, "")
	if ts != "1750000000" {
		t.Fatalf("ts = %q, want 1750000000", ts)
	}

	// Recompute by hand: GET\n/api/v1/x\n1750000000\nsha256("")
	emptyHash := sha256.Sum256(nil)
	expected := hmacHex(t, "sk",
		"GET\n/api/v1/x\n1750000000\n"+hex.EncodeToString(emptyHash[:]))
	if sig != expected {
		t.Fatalf("signature mismatch:\n got  %s\n want %s", sig, expected)
	}
}

func TestSign_IncludesBodyHash(t *testing.T) {
	c, _ := NewClient(ClientOptions{
		KeyID: "ak", Secret: "sk", BaseURL: "https://x.test", Clock: fixedClock(42),
	})
	body := []byte(`{"a":1}`)
	sig, _ := c.sign("POST", "/api/v1/things", body, "")

	bodyHash := sha256.Sum256(body)
	expected := hmacHex(t, "sk", "POST\n/api/v1/things\n42\n"+hex.EncodeToString(bodyHash[:]))
	if sig != expected {
		t.Fatalf("body-hash sig mismatch:\n got  %s\n want %s", sig, expected)
	}
}

func TestSign_AppendsIdempotencyKeyLine(t *testing.T) {
	c, _ := NewClient(ClientOptions{
		KeyID: "ak", Secret: "sk", BaseURL: "https://x.test", Clock: fixedClock(42),
	})
	body := []byte(`{}`)

	withoutIdem, _ := c.sign("POST", "/p", body, "")
	withIdem, _ := c.sign("POST", "/p", body, "idem_xyz")

	if withoutIdem == withIdem {
		t.Fatal("idempotency key should change the signature")
	}

	bodyHash := sha256.Sum256(body)
	expected := hmacHex(t, "sk",
		"POST\n/p\n42\n"+hex.EncodeToString(bodyHash[:])+"\nidem_xyz")
	if withIdem != expected {
		t.Fatalf("idem-included sig mismatch:\n got  %s\n want %s", withIdem, expected)
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
	auth := req.Headers.Get("Authorization")
	if !strings.HasPrefix(auth, "Storlaunch-HMAC-SHA256 keyId=AKIASTOR_TEST, scope=*, signature=") {
		t.Fatalf("authorization header wrong shape: %q", auth)
	}
	if ts := req.Headers.Get("X-Storlaunch-Timestamp"); ts != "1750000000" {
		t.Fatalf("expected fixed timestamp, got %q", ts)
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
	if !strings.HasPrefix(req.Headers.Get("Idempotency-Key"), "idem_") {
		t.Fatalf("expected auto Idempotency-Key, got %q", req.Headers.Get("Idempotency-Key"))
	}
	if string(req.Body) == "" || !strings.Contains(string(req.Body), `"amount":1500`) {
		t.Fatalf("body not serialized correctly: %q", string(req.Body))
	}
}

func TestRequest_OnBehalfOfHeader(t *testing.T) {
	c, _, rrt := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(envelope(t, []any{}))
	})
	merchant := c.ForMerchant("acc_merchant_42")
	if _, err := merchant.Payment.Plans.List(context.Background(), nil); err != nil {
		t.Fatalf("list: %v", err)
	}
	if got := rrt.last().Headers.Get("X-Storlaunch-On-Behalf-Of"); got != "acc_merchant_42" {
		t.Fatalf("expected X-Storlaunch-On-Behalf-Of=acc_merchant_42, got %q", got)
	}
}

func TestRequest_NoOnBehalfOfWhenUnset(t *testing.T) {
	c, _, rrt := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(envelope(t, []any{}))
	})
	if _, err := c.Payment.Plans.List(context.Background(), nil); err != nil {
		t.Fatalf("list: %v", err)
	}
	if got := rrt.last().Headers.Get("X-Storlaunch-On-Behalf-Of"); got != "" {
		t.Fatalf("expected no on-behalf-of header, got %q", got)
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
	out, err := c.Billing.Checkout(context.Background(), BillingCheckoutInput{
		PlanID:     "plan_pro",
		SuccessURL: "https://x.test/ok",
		CancelURL:  "https://x.test/cancel",
	})
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if out["checkoutUrl"] != "https://pay.test/abc" {
		t.Fatalf("unexpected data: %v", out)
	}
	body := string(rrt.last().Body)
	if !strings.Contains(body, `"planId":"plan_pro"`) {
		t.Fatalf("expected planId in body, got %q", body)
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

func TestRoundTrip_BuyerDeleteAddress(t *testing.T) {
	c, _, rrt := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/checkout/addresses/addr_9" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write(envelope(t, map[string]any{"deleted": true}))
	})
	if _, err := c.Buyer.DeleteAddress(context.Background(), "addr_9"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if rrt.last().Method != http.MethodDelete {
		t.Fatalf("delete method not honored")
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

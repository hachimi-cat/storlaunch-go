// Package storlaunch is the official Go SDK for Storlaunch.
//
// Auth is a secret API key (sk_live_… / sk_test_…, minted under Settings →
// API keys) sent as `Authorization: Bearer <key>` on every request. A key
// belongs to one workspace.
//
// Mirrors the Node + Python SDKs route-for-route. The routes hang off
// resource namespaces on the Client: Payment, Storefront, Account,
// Analytics, Billing, Modules, ManualOrders, Onboarding, Shipping,
// Inventory, Ledger, Reports, Payouts, DiscountCodes.
package storlaunch

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Version is this SDK's version.
const Version = "0.3.0"

// DefaultBaseURL is the production Storlaunch API base.
const DefaultBaseURL = "https://storlaunch.com"

// DefaultTimeout matches the Node+Python defaults (30s).
const DefaultTimeout = 30 * time.Second

// ClientOptions configures a new Client. APIKey is required; every other
// field falls back to a sensible default (or env var).
type ClientOptions struct {
	// APIKey is a secret API key: sk_live_… or sk_test_…. Sent as
	// `Authorization: Bearer <APIKey>`. Defaults to env STORLAUNCH_API_KEY.
	APIKey string
	// BaseURL defaults to env STORLAUNCH_BASE_URL, then DefaultBaseURL.
	BaseURL string
	// Timeout is the per-request timeout. Zero falls back to DefaultTimeout.
	Timeout time.Duration
	// HTTP is the underlying http.Client. Zero falls back to a fresh
	// http.Client with Timeout. Pass your own to share connection pools
	// or inject middleware.
	HTTP *http.Client
}

// Client is the high-level Storlaunch API surface. The resource
// namespaces (Payment, Storefront, ...) are populated by NewClient and
// can be called directly: `c.Payment.Plans.List(ctx, nil)`.
type Client struct {
	apiKey  string
	baseURL string
	timeout time.Duration
	http    *http.Client

	// Resource namespaces — populated in NewClient.
	Payment       *PaymentResource
	Storefront    *StorefrontResource
	Account       *AccountResource
	Analytics     *AnalyticsResource
	Billing       *BillingResource
	Modules       *ModulesResource
	ManualOrders  *ManualOrdersResource
	Onboarding    *OnboardingResource
	Shipping      *ShippingResource
	Inventory     *InventoryResource
	Ledger        *LedgerResource
	Reports       *ReportsResource
	Payouts       *PayoutsResource
	DiscountCodes *DiscountCodesResource

	// API has every feature route, one method each (generated from the API
	// spec: api_generated.go), sent like every other call.
	API *GeneratedAPI
}

// NewClient validates options and returns a ready-to-use Client.
// Reads STORLAUNCH_API_KEY / STORLAUNCH_BASE_URL from env when the
// corresponding fields are empty.
func NewClient(opts ClientOptions) (*Client, error) {
	if opts.APIKey == "" {
		opts.APIKey = os.Getenv("STORLAUNCH_API_KEY")
	}
	if opts.BaseURL == "" {
		opts.BaseURL = os.Getenv("STORLAUNCH_BASE_URL")
	}
	if opts.BaseURL == "" {
		opts.BaseURL = DefaultBaseURL
	}
	if opts.APIKey == "" {
		return nil, newErr(0, "missing_api_key", "set STORLAUNCH_API_KEY env or ClientOptions.APIKey (an sk_live_… or sk_test_… key)")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultTimeout
	}
	if opts.HTTP == nil {
		opts.HTTP = &http.Client{Timeout: opts.Timeout}
	}
	c := &Client{
		apiKey:  opts.APIKey,
		baseURL: strings.TrimRight(opts.BaseURL, "/"),
		timeout: opts.Timeout,
		http:    opts.HTTP,
	}
	c.installResources()
	c.API = &GeneratedAPI{c: c}
	return c, nil
}

// apigenRequest is the call behind Client.API (api_generated.go): the same
// Bearer key, idempotency key on writes and envelope handling as Request.
func (c *Client) apigenRequest(ctx context.Context, method, path string, query url.Values, body map[string]any) (json.RawMessage, error) {
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	args := RequestArgs{Method: strings.ToUpper(method), Path: path}
	if body != nil {
		args.Body = body
	}
	if args.Method != http.MethodGet {
		args.IdempotencyKey = c.genIdem()
	}
	var out json.RawMessage
	args.Out = &out
	if err := c.Request(ctx, args); err != nil {
		return nil, err
	}
	return out, nil
}

// BaseURL returns the configured API base (no trailing slash). Exposed
// mainly for tests + diagnostics.
func (c *Client) BaseURL() string { return c.baseURL }

// RequestArgs is the input to Client.Request. Out (if non-nil) receives
// the unwrapped `data` payload via json.Unmarshal — or, when it is a
// *string, the raw body of a non-JSON success (a CSV export).
type RequestArgs struct {
	Method         string
	Path           string
	Body           any
	IdempotencyKey string
	Out            any
}

// Request dispatches a single API call. It sends `Authorization: Bearer
// <APIKey>`, a JSON body as `Content-Type: application/json`, and an
// idempotency key as both X-Idempotency-Key (what Storlaunch's own
// replay guard reads) and Idempotency-Key (what it forwards to Plugipay).
// Most callers want the typed resource wrappers; reach for Request when
// you need a route the SDK doesn't yet expose.
//
// On non-2xx (or 2xx with `error` set in the envelope) returns *Error.
// On 2xx the JSON envelope's `data` field is decoded into Args.Out
// (when non-nil); a 204 leaves Out untouched.
func (c *Client) Request(ctx context.Context, args RequestArgs) error {
	var bodyBytes []byte
	if args.Body != nil {
		b, err := json.Marshal(args.Body)
		if err != nil {
			return newErr(0, "invalid_body", err.Error())
		}
		bodyBytes = b
	}

	url := c.baseURL + args.Path
	var bodyReader io.Reader
	if bodyBytes != nil {
		bodyReader = bytes.NewReader(bodyBytes)
	}
	req, err := http.NewRequestWithContext(ctx, args.Method, url, bodyReader)
	if err != nil {
		return newErr(0, "invalid_request", err.Error())
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if bodyBytes != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if args.IdempotencyKey != "" {
		req.Header.Set("X-Idempotency-Key", args.IdempotencyKey)
		req.Header.Set("Idempotency-Key", args.IdempotencyKey)
	}

	res, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return newErr(0, "timeout", fmt.Sprintf("storlaunch request timed out: %s", ctx.Err()))
		}
		return newErr(0, "network_error", err.Error())
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)
	ok := res.StatusCode < 400
	if ok && len(raw) == 0 {
		return nil
	}
	var env apiEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		// A CSV export is text; a web page means the base URL is not the API.
		if s, isText := args.Out.(*string); ok && isText && !strings.HasPrefix(res.Header.Get("Content-Type"), "text/html") {
			*s = string(raw)
			return nil
		}
		snippet := string(raw)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return newErr(res.StatusCode, "invalid_response", "non-JSON response: "+snippet)
	}
	if !ok || env.Error != nil {
		code := "unknown"
		msg := fmt.Sprintf("HTTP %d", res.StatusCode)
		if env.Error != nil {
			if env.Error.Code != "" {
				code = env.Error.Code
			}
			if env.Error.Message != "" {
				msg = env.Error.Message
			}
		}
		out := &Error{Status: res.StatusCode, Code: code, Message: msg}
		if env.Meta != nil {
			out.RequestID = env.Meta.RequestID
		}
		return out
	}
	if args.Out != nil && len(env.Data) > 0 && string(env.Data) != "null" {
		if err := json.Unmarshal(env.Data, args.Out); err != nil {
			return newErr(res.StatusCode, "invalid_response", "data decode: "+err.Error())
		}
	}
	return nil
}

// Passthrough is the generic escape hatch for routes the typed surface
// doesn't yet cover. Mirrors the `passthrough()` helper in Node + Python.
func (c *Client) Passthrough(ctx context.Context, method, path string, body any, out any) error {
	idem := ""
	if method != http.MethodGet {
		idem = c.genIdem()
	}
	return c.Request(ctx, RequestArgs{Method: method, Path: path, Body: body, IdempotencyKey: idem, Out: out})
}

// ─── Internal helpers ───────────────────────────────────────────────────

type apiEnvelope struct {
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		DocURL  string `json:"docUrl,omitempty"`
	} `json:"error"`
	Meta *struct {
		RequestID string `json:"requestId"`
		Timestamp string `json:"timestamp"`
		Cursor    string `json:"cursor,omitempty"`
		HasMore   bool   `json:"hasMore,omitempty"`
	} `json:"meta,omitempty"`
}

// genIdem produces an Idempotency-Key for mutating routes that need
// one. UUIDv4-shaped; not cryptographically critical (only used to
// dedupe retried writes server-side).
func (c *Client) genIdem() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failing is fatal; fall back to time-based so we
		// don't panic. Vanishingly rare in practice.
		now := time.Now().UnixNano()
		for i := range b {
			b[i] = byte(now >> (uint(i) * 4))
		}
	}
	// Section 4.4 UUIDv4 markers (not strictly needed, but matches what
	// the other SDKs emit).
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("idem_%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// qs renders a map of query params, preserving Node/Python semantics:
// nil/empty values are skipped, all values are stringified, and the
// leading "?" is included only when at least one param survives.
func qs(params map[string]any) string {
	if len(params) == 0 {
		return ""
	}
	u := url.Values{}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	// Stable order — easier to assert in tests.
	sortStrings(keys)
	any := false
	for _, k := range keys {
		v := params[k]
		if v == nil {
			continue
		}
		// Skip empty strings to mirror the JS/Python `!= undefined && !=
		// null` check — but allow zero numbers/booleans.
		if s, ok := v.(string); ok && s == "" {
			continue
		}
		u.Set(k, fmt.Sprintf("%v", v))
		any = true
	}
	if !any {
		return ""
	}
	return "?" + u.Encode()
}

// sortStrings is a tiny dependency-free sort to keep go.mod minimal.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

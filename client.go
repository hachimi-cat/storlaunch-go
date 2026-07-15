// Package storlaunch is the official Go SDK for Storlaunch.
//
// Auth is HMAC-SHA256 partner-billing (Pattern 2, Shopify-Apps style).
// Every request is signed with the caller's keyId+secret. Optional
// merchant scoping is done via ForMerchant(accountId), which forwards
// the merchant id in the X-Storlaunch-On-Behalf-Of header.
//
// Mirrors the Node + Python SDKs route-for-route. The 47-route surface
// hangs off resource namespaces on the Client: Payment, Storefront,
// Account, Analytics, Billing, Modules, ManualOrders, Onboarding,
// Shipping, Inventory, Ledger, Reports, Payouts, DiscountCodes,
// InboundWebhooks, Buyer.
package storlaunch

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the production Storlaunch API base.
const DefaultBaseURL = "https://storlaunch.com"

// DefaultTimeout matches the Node+Python defaults (30s).
const DefaultTimeout = 30 * time.Second

// ClientOptions configures a new Client. KeyID and Secret are required;
// every other field falls back to a sensible default (or env var).
type ClientOptions struct {
	// KeyID is the HMAC access-key id, e.g. "AKIASTOR<random>".
	// Defaults to env STORLAUNCH_KEY_ID.
	KeyID string
	// Secret is the HMAC secret. Defaults to env STORLAUNCH_SECRET.
	Secret string
	// BaseURL defaults to env STORLAUNCH_BASE_URL, then DefaultBaseURL.
	BaseURL string
	// OnBehalfOf optionally pins every request to this merchant accountId
	// (sent as X-Storlaunch-On-Behalf-Of). Only valid when KeyID holds
	// the storlaunch:platform:admin scope. Prefer ForMerchant() for ad-hoc
	// scoping.
	OnBehalfOf string
	// Timeout is the per-request timeout. Zero falls back to DefaultTimeout.
	Timeout time.Duration
	// HTTP is the underlying http.Client. Zero falls back to a fresh
	// http.Client with Timeout. Pass your own to share connection pools
	// or inject middleware.
	HTTP *http.Client
	// Clock is the wall clock used for HMAC timestamps. Zero means
	// time.Now. Tests override this for determinism.
	Clock func() time.Time
}

// Client is the high-level Storlaunch API surface. The resource
// namespaces (Payment, Storefront, ...) are populated by NewClient and
// can be called directly: `c.Payment.Plans.List(ctx, nil)`.
type Client struct {
	keyID      string
	secret     string
	baseURL    string
	onBehalfOf string
	timeout    time.Duration
	http       *http.Client
	clock      func() time.Time

	// Resource namespaces — populated in NewClient.
	Payment         *PaymentResource
	Storefront      *StorefrontResource
	Account         *AccountResource
	Analytics       *AnalyticsResource
	Billing         *BillingResource
	Modules         *ModulesResource
	ManualOrders    *ManualOrdersResource
	Onboarding      *OnboardingResource
	Shipping        *ShippingResource
	Inventory       *InventoryResource
	Ledger          *LedgerResource
	Reports         *ReportsResource
	Payouts         *PayoutsResource
	DiscountCodes   *DiscountCodesResource
	InboundWebhooks *InboundWebhooksResource
	Buyer           *BuyerResource
}

// NewClient validates options and returns a ready-to-use Client.
// Reads STORLAUNCH_KEY_ID / STORLAUNCH_SECRET / STORLAUNCH_BASE_URL from
// env when the corresponding fields are empty.
func NewClient(opts ClientOptions) (*Client, error) {
	if opts.KeyID == "" {
		opts.KeyID = os.Getenv("STORLAUNCH_KEY_ID")
	}
	if opts.Secret == "" {
		opts.Secret = os.Getenv("STORLAUNCH_SECRET")
	}
	if opts.BaseURL == "" {
		opts.BaseURL = os.Getenv("STORLAUNCH_BASE_URL")
	}
	if opts.BaseURL == "" {
		opts.BaseURL = DefaultBaseURL
	}
	if opts.KeyID == "" {
		return nil, newErr(0, "missing_key_id", "set STORLAUNCH_KEY_ID env or ClientOptions.KeyID")
	}
	if opts.Secret == "" {
		return nil, newErr(0, "missing_secret", "set STORLAUNCH_SECRET env or ClientOptions.Secret")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultTimeout
	}
	if opts.HTTP == nil {
		opts.HTTP = &http.Client{Timeout: opts.Timeout}
	}
	if opts.Clock == nil {
		opts.Clock = time.Now
	}
	c := &Client{
		keyID:      opts.KeyID,
		secret:     opts.Secret,
		baseURL:    strings.TrimRight(opts.BaseURL, "/"),
		onBehalfOf: opts.OnBehalfOf,
		timeout:    opts.Timeout,
		http:       opts.HTTP,
		clock:      opts.Clock,
	}
	c.installResources()
	return c, nil
}

// ForMerchant returns a shallow clone of the receiver that scopes every
// request to the given merchant accountId (sent as
// X-Storlaunch-On-Behalf-Of). The underlying http.Client is shared, so
// no extra connection pool is allocated.
func (c *Client) ForMerchant(accountID string) *Client {
	clone := &Client{
		keyID:      c.keyID,
		secret:     c.secret,
		baseURL:    c.baseURL,
		onBehalfOf: accountID,
		timeout:    c.timeout,
		http:       c.http,
		clock:      c.clock,
	}
	clone.installResources()
	return clone
}

// BaseURL returns the configured API base (no trailing slash). Exposed
// mainly for tests + diagnostics.
func (c *Client) BaseURL() string { return c.baseURL }

// OnBehalfOf returns the merchant accountId this client is scoped to,
// or "" if the client makes platform-level calls.
func (c *Client) OnBehalfOf() string { return c.onBehalfOf }

// RequestArgs is the input to Client.Request. Out (if non-nil) receives
// the unwrapped `data` payload via json.Unmarshal.
type RequestArgs struct {
	Method         string
	Path           string
	Body           any
	IdempotencyKey string
	OnBehalfOf     string // overrides the client's default for this call
	Out            any
}

// Request signs and dispatches a single API call. Most callers want the
// typed resource wrappers; reach for Request when you need a route the
// SDK doesn't yet expose.
//
// On non-2xx (or 2xx with `error` set in the envelope) returns *Error.
// On 2xx the JSON envelope's `data` field is decoded into Args.Out
// (when non-nil).
func (c *Client) Request(ctx context.Context, args RequestArgs) error {
	var bodyBytes []byte
	if args.Body != nil {
		b, err := json.Marshal(args.Body)
		if err != nil {
			return newErr(0, "invalid_body", err.Error())
		}
		bodyBytes = b
	}

	sig, ts := c.sign(args.Method, args.Path, bodyBytes, args.IdempotencyKey)

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
	req.Header.Set("Authorization", fmt.Sprintf(
		"Storlaunch-HMAC-SHA256 keyId=%s, scope=*, signature=%s",
		c.keyID, sig,
	))
	req.Header.Set("X-Storlaunch-Timestamp", ts)
	if bodyBytes != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if args.IdempotencyKey != "" {
		req.Header.Set("Idempotency-Key", args.IdempotencyKey)
	}
	obo := args.OnBehalfOf
	if obo == "" {
		obo = c.onBehalfOf
	}
	if obo != "" {
		req.Header.Set("X-Storlaunch-On-Behalf-Of", obo)
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
	var env apiEnvelope
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &env); err != nil {
			snippet := string(raw)
			if len(snippet) > 200 {
				snippet = snippet[:200]
			}
			return newErr(res.StatusCode, "invalid_response", "non-JSON response: "+snippet)
		}
	}
	if res.StatusCode >= 400 || env.Error != nil {
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
	return c.Request(ctx, RequestArgs{Method: method, Path: path, Body: body, Out: out})
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

// sign computes the HMAC-SHA256 signature used for every request.
//
// String-to-sign:
//
//	METHOD\nPATH\nUNIX_TS\nSHA256(body)[\nIDEMPOTENCY_KEY]
//
// where SHA256(body) is the hex digest of the raw JSON body (or the
// empty string when there is no body). The idempotency-key line is
// appended only when an idempotency key was passed.
//
// MUST match sign() in sdk/node/src/client.ts and _sign() in
// sdk/python/storlaunch/client.py — drift here breaks every request.
func (c *Client) sign(method, path string, body []byte, idempotencyKey string) (sig, ts string) {
	ts = strconv.FormatInt(c.clock().Unix(), 10)
	hash := sha256.Sum256(body) // sum of empty body is the SHA256 of ""
	bodyHex := hex.EncodeToString(hash[:])
	stringToSign := strings.ToUpper(method) + "\n" + path + "\n" + ts + "\n" + bodyHex
	if idempotencyKey != "" {
		stringToSign += "\n" + idempotencyKey
	}
	mac := hmac.New(sha256.New, []byte(c.secret))
	mac.Write([]byte(stringToSign))
	sig = hex.EncodeToString(mac.Sum(nil))
	return sig, ts
}

// genIdem produces an Idempotency-Key for mutating routes that need
// one. UUIDv4-shaped; not cryptographically critical (only used to
// dedupe retried writes server-side).
func (c *Client) genIdem() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failing is fatal; fall back to time-based so we
		// don't panic. Vanishingly rare in practice.
		now := c.clock().UnixNano()
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

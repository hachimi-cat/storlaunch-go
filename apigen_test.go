package storlaunch

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// client.API (api_generated.go) goes through Request: the same Bearer key,
// idempotency key on writes and envelope handling as every hand-written call.
func TestAPISendsPathQueryBodyAndKey(t *testing.T) {
	var got struct {
		method, path, query, auth, idem string
		body                            map[string]any
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path, got.query = r.Method, r.URL.Path, r.URL.RawQuery
		got.auth, got.idem = r.Header.Get("Authorization"), r.Header.Get("X-Idempotency-Key")
		got.body = nil
		if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
			_ = json.Unmarshal(raw, &got.body)
		}
		_, _ = w.Write([]byte(`{"data":{"id":"dc_1"},"error":null,"meta":{"requestId":"r"}}`))
	}))
	defer srv.Close()
	c, err := NewClient(ClientOptions{APIKey: "sk_test_abc", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}

	out, err := c.API.DiscountCodesCreate(context.Background(), &DiscountCodesCreateArgs{Code: "SPRING", Type: "percent", Value: 10, Currency: "IDR"})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"id":"dc_1"}` {
		t.Fatalf("data = %s", out)
	}
	if got.method != "POST" || got.path != "/api/v1/discount-codes" || got.auth != "Bearer sk_test_abc" || got.idem == "" {
		t.Fatalf("sent %+v", got)
	}
	if got.body["code"] != "SPRING" || got.body["type"] != "percent" || got.body["value"] != float64(10) {
		t.Fatalf("body %v", got.body)
	}

	if _, err := c.API.DiscountCodesList(context.Background(), &DiscountCodesListArgs{Limit: 5}); err != nil {
		t.Fatal(err)
	}
	if got.method != "GET" || got.query != "limit=5" || got.idem != "" || got.body != nil {
		t.Fatalf("sent %+v", got)
	}
}

func TestAPIReportsTheErrorEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"data":null,"error":{"code":"NOT_FOUND","message":"no such code"},"meta":{"requestId":"r"}}`))
	}))
	defer srv.Close()
	c, _ := NewClient(ClientOptions{APIKey: "sk_test_abc", BaseURL: srv.URL})
	_, err := c.API.DiscountCodesGet(context.Background(), "dc_missing")
	e, ok := err.(*Error)
	if !ok || e.Status != 404 || e.Code != "NOT_FOUND" {
		t.Fatalf("err = %#v", err)
	}
}

package storlaunch

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// The webhook endpoint management calls and the delivery log's resend hit the routes
// the API serves.
func TestWebhookEndpointsAndResend(t *testing.T) {
	c, srv, rrt := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(envelope(t, map[string]any{"id": "x"}))
	})
	ctx := context.Background()
	active, rotate := true, true
	if _, err := c.Payment.WebhookEndpoints.Get(ctx, "we_1"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Payment.WebhookEndpoints.Update(ctx, "we_1", WebhookEndpointPatch{Active: &active, RotateSecret: &rotate}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Payment.WebhookEndpoints.EventTypes(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Payment.WebhookEndpoints.SendTest(ctx, "we_1"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Payment.WebhookEvents.Resend(ctx, "whd_1"); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"GET /api/v1/payment/webhook-endpoints/we_1",
		"PATCH /api/v1/payment/webhook-endpoints/we_1",
		"GET /api/v1/payment/webhook-endpoints/event-types",
		"POST /api/v1/payment/webhook-endpoints/we_1/test",
		"POST /api/v1/payment/webhook-events/whd_1/resend",
	}
	if len(rrt.requests) != len(want) {
		t.Fatalf("got %d requests, want %d", len(rrt.requests), len(want))
	}
	for i, r := range rrt.requests {
		if got := r.Method + " " + strings.TrimPrefix(r.URL, srv.URL); got != want[i] {
			t.Errorf("request %d: got %q, want %q", i, got, want[i])
		}
	}
	var body map[string]any
	if err := json.Unmarshal(rrt.requests[1].Body, &body); err != nil {
		t.Fatal(err)
	}
	if body["active"] != true || body["rotateSecret"] != true || len(body) != 2 {
		t.Errorf("PATCH body = %v", body)
	}
}

# storlaunch (Go)

Official Go SDK for [Storlaunch](https://storlaunch.com).

Mirrors the Node and Python SDKs route-for-route: API-key auth,
idempotency keys, and webhook signature verification.

## Install

```bash
go get github.com/hachimi-cat/storlaunch-go
```

Requires Go 1.22+. Zero dependencies beyond the standard library.

## Quickstart

Env vars (or pass them to `NewClient`):

```bash
STORLAUNCH_API_KEY=sk_live_...               # or sk_test_...; Settings → API keys
STORLAUNCH_BASE_URL=https://storlaunch.com   # optional; this is the default
```

```go
package main

import (
    "context"
    "fmt"

    storlaunch "github.com/hachimi-cat/storlaunch-go"
)

func main() {
    c, err := storlaunch.NewClient(storlaunch.ClientOptions{})
    if err != nil {
        panic(err)
    }

    plans, err := c.Payment.Plans.List(context.Background(), nil)
    if err != nil {
        panic(err)
    }
    fmt.Printf("%d plans\n", len(plans))
}
```

## Auth

Every request carries the key as a bearer token, and nothing else
authenticates:

```
Authorization: Bearer sk_live_…
```

A key belongs to one workspace and acts as its owner. It cannot create or
revoke API keys (`Account.APIKeys.Create/Revoke` answer 403 to a key; that
needs a signed-in session), and it cannot call the shopper routes under
`/api/v1/checkout`, which take the shopper's own storefront session.

Until 0.2.0 this SDK signed requests (`KeyID` + `Secret`,
`Storlaunch-HMAC-SHA256`) and scoped them with `ForMerchant` /
`X-Storlaunch-On-Behalf-Of`. The API never accepted either, so they are gone.

## Idempotency

Create-style mutations on the Payment, Storefront, Account, Manual
Orders, Shipping, Inventory, Ledger, Payouts and Discount Codes
namespaces auto-generate an idempotency key (`idem_<uuid>`), sent as both
`X-Idempotency-Key` (what Storlaunch's replay guard reads) and
`Idempotency-Key` (what it forwards to Plugipay), so retries are safe.

To pass your own key, use the lower-level `Request` helper:

```go
err := c.Request(ctx, storlaunch.RequestArgs{
    Method:         "POST",
    Path:           "/api/v1/payment/checkout-sessions",
    Body:           input,
    IdempotencyKey: "my-key-123",
    Out:            &result,
})
```

## Webhooks

```go
import "io"

func storlaunchWebhook(w http.ResponseWriter, r *http.Request) {
    body, _ := io.ReadAll(r.Body)
    sig := r.Header.Get("X-Storlaunch-Signature")
    if !storlaunch.VerifyWebhookSignature(body, sig, os.Getenv("STORLAUNCH_WEBHOOK_SECRET"), nil) {
        http.Error(w, "bad signature", 400)
        return
    }
    // ... handle event ...
    w.WriteHeader(204)
}
```

Format: `t=<unix>,v1=<hmac-sha256-hex>`, signed over
`fmt.Sprintf("%d.%s", unix, rawBody)`. Default replay window is 5
minutes; override via `VerifyWebhookSignatureOptions.ToleranceSeconds`.

## Errors

Every operation returns `error`, but the concrete type is always
`*storlaunch.Error` when the failure originated from the API (or the
SDK's own request plumbing):

```go
out, err := c.Payment.Plans.Get(ctx, "plan_x")
if err != nil {
    var se *storlaunch.Error
    if errors.As(err, &se) {
        log.Printf("storlaunch %d %s: %s (req=%s)",
            se.Status, se.Code, se.Message, se.RequestID)
    }
}
```

`Status == 0` indicates a client-side failure (timeout, network,
malformed response).

## Resource map

| Namespace | Notes |
|---|---|
| `Payment.{CheckoutSessions, Plans, Subscriptions, Invoices, Receipts, Customers, PortalSessions, WebhookEndpoints, WebhookEvents}` | Plugipay-backed payments. |
| `Storefront.{Products, Licenses, Deliveries, Public}` | Digital-product storefront. |
| `Account.{Pixels, AbandonedCart, Feeds, Blog, Referrals, APIKeys, AuditLog, Domains}` + `Profile/UpdateProfile` | Settings + marketing. |
| `Analytics` | `Overview`. |
| `Billing` | Saas-side billing (plans, usage, invoices, checkout). |
| `Modules` | Enable/disable optional modules. |
| `ManualOrders`, `Onboarding`, `Shipping`, `Inventory`, `Ledger`, `Reports`, `Payouts`, `DiscountCodes` | Top-level resources. |

For routes the SDK doesn't expose yet, use `Client.Passthrough`.

## License

MIT

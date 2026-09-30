# Changelog

## 0.2.0
- **Auth is an API key now.** Pass `ClientOptions.APIKey`: an `sk_live_…` / `sk_test_…` key from Settings → API keys (default: env `STORLAUNCH_API_KEY`), sent as `Authorization: Bearer <key>`. The `KeyID`/`Secret` request signing and `ForMerchant()`/`OnBehalfOf` (`X-Storlaunch-On-Behalf-Of`) are removed, with `ClientOptions.Clock`: the API never accepted them, so every call answered 401. A key belongs to one workspace.
- Writes send their idempotency key as `X-Idempotency-Key` (what Storlaunch's replay guard reads; subscription and portal-session creates require it) and `Idempotency-Key` (what it forwards to Plugipay).
- A 204 returns a nil Object; a CSV export returns its text.
- Methods now call the route the API really has: `Payment.Plans.Archive` → `DELETE /payment/plans/{id}`; `Payment.Subscriptions.Cancel(ctx, id, immediate)` → `DELETE /payment/subscriptions/{id}`; `Storefront.Licenses.Revoke(ctx, key)` → `DELETE /storefront/licenses/{key}`; `Account.APIKeys.Revoke` → `DELETE /account/api-keys/{id}` and `Create` takes an `APIKeyCreateInput{Name, Environment}`; `Account.Referrals.UpdateProgram` → `PUT`; `Billing.Checkout(BillingCheckoutInput{Plan, Interval, Currency})` → `POST /billing/plugipay-invoice`; `Modules.Enable/Disable` → `POST /modules {module, enabled}`; `Inventory.Levels({"variantId": …})` → `GET /inventory/stock`; `Ledger.List/Balances/Adjust` → `/ledger/entries`, `/ledger/balance`, `/ledger/adjustments`; `Reports.ExportLedger(ctx)` (a string) → `GET /ledger/entries.csv`; `Onboarding.CompleteStep` → `Onboarding.Complete(ctx, enablePayment)`.
- Removed, because the API has no such route: `Payment.Invoices.Finalize/Pay/Void`, `Payment.PlugipaySettings`, `Storefront.Products.ListFiles` (files come with `Products.Get`), `Storefront.Deliveries.Create`, `Analytics.Storefront/Funnel`, `Billing.CurrentPlan` (use `Billing.Subscription`), `Modules.Status` (use `Modules.List`, which now returns an Object), `ManualOrders.Create`, `DiscountCodes.Validate`, `InboundWebhooks`. Also removed: `Buyer`, whose shopper routes take the shopper's storefront session, never an API key.
- `Account.Blog.List` returns an Object (`{posts: [...]}`): it decoded into a List and failed with `data decode`.
- A test checks every hand-written method's route against the API spec (`backend/openapi.json`).
- `Client.API`: every feature route of the API, one method each (`c.API.DiscountCodesCreate(ctx, &DiscountCodesCreateArgs{…})`), generated from the API spec (`api_generated.go`) and sent like every other call.

## 0.1.0
- Initial release. Module path is github.com/hachimi-cat/storlaunch-go.

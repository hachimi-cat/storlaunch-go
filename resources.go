package storlaunch

import (
	"context"
	"net/http"
)

// Object and List are loose JSON types matching the Node SDK's
// `Record<string, unknown>` and `unknown[]`. Resource shapes can be
// added in a follow-up; the wrappers are typed against generic JSON
// today.
type (
	Object = map[string]any
	List   = []any
)

func (c *Client) installResources() {
	c.Payment = &PaymentResource{
		CheckoutSessions: &paymentCheckoutSessions{c: c},
		Plans:            &paymentPlans{c: c},
		Subscriptions:    &paymentSubscriptions{c: c},
		Invoices:         &paymentInvoices{c: c},
		Receipts:         &paymentReceipts{c: c},
		Customers:        &paymentCustomers{c: c},
		PortalSessions:   &paymentPortalSessions{c: c},
		WebhookEndpoints: &paymentWebhookEndpoints{c: c},
		WebhookEvents:    &paymentWebhookEvents{c: c},
	}
	c.Storefront = &StorefrontResource{
		Products:   &storefrontProducts{c: c},
		Licenses:   &storefrontLicenses{c: c},
		Deliveries: &storefrontDeliveries{c: c},
		Public:     &storefrontPublic{c: c},
	}
	c.Account = &AccountResource{
		c:             c,
		Pixels:        &accountPixels{c: c},
		AbandonedCart: &accountAbandonedCart{c: c},
		Feeds:         &accountFeeds{c: c},
		Blog:          &accountBlog{c: c},
		Referrals:     &accountReferrals{c: c},
		APIKeys:       &accountAPIKeys{c: c},
		AuditLog:      &accountAuditLog{c: c},
		Domains:       &accountDomains{c: c},
	}
	c.Analytics = &AnalyticsResource{c: c}
	c.Billing = &BillingResource{c: c}
	c.Modules = &ModulesResource{c: c}
	c.ManualOrders = &ManualOrdersResource{c: c}
	c.Onboarding = &OnboardingResource{c: c}
	c.Shipping = &ShippingResource{c: c}
	c.Inventory = &InventoryResource{c: c}
	c.Ledger = &LedgerResource{c: c}
	c.Reports = &ReportsResource{c: c}
	c.Payouts = &PayoutsResource{c: c}
	c.DiscountCodes = &DiscountCodesResource{c: c}
}

// ═══════════════════════════════════════════════════════════════════════
// PAYMENT
// ═══════════════════════════════════════════════════════════════════════

type PaymentResource struct {
	CheckoutSessions PaymentCheckoutSessions
	Plans            PaymentPlans
	Subscriptions    PaymentSubscriptions
	Invoices         PaymentInvoices
	Receipts         PaymentReceipts
	Customers        PaymentCustomers
	PortalSessions   PaymentPortalSessions
	WebhookEndpoints PaymentWebhookEndpoints
	WebhookEvents    PaymentWebhookEvents
}

type PaymentCheckoutSessions interface {
	List(ctx context.Context, params map[string]any) (List, error)
	Get(ctx context.Context, id string) (Object, error)
	Create(ctx context.Context, input Object) (Object, error)
}

type paymentCheckoutSessions struct{ c *Client }

func (r *paymentCheckoutSessions) List(ctx context.Context, params map[string]any) (List, error) {
	return doList(ctx, r.c, http.MethodGet, "/api/v1/payment/checkout-sessions"+qs(params), nil, "")
}
func (r *paymentCheckoutSessions) Get(ctx context.Context, id string) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/payment/checkout-sessions/"+id, nil, "")
}
func (r *paymentCheckoutSessions) Create(ctx context.Context, input Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/payment/checkout-sessions", input, r.c.genIdem())
}

type PaymentPlans interface {
	List(ctx context.Context, params map[string]any) (List, error)
	Get(ctx context.Context, id string) (Object, error)
	Create(ctx context.Context, input Object) (Object, error)
	Update(ctx context.Context, id string, patch Object) (Object, error)
	Archive(ctx context.Context, id string) (Object, error)
}

type paymentPlans struct{ c *Client }

func (r *paymentPlans) List(ctx context.Context, params map[string]any) (List, error) {
	return doList(ctx, r.c, http.MethodGet, "/api/v1/payment/plans"+qs(params), nil, "")
}
func (r *paymentPlans) Get(ctx context.Context, id string) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/payment/plans/"+id, nil, "")
}
func (r *paymentPlans) Create(ctx context.Context, input Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/payment/plans", input, r.c.genIdem())
}
func (r *paymentPlans) Update(ctx context.Context, id string, patch Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPatch, "/api/v1/payment/plans/"+id, patch, "")
}

// Archive archives the plan (DELETE /payment/plans/{id}); existing subscribers keep it.
func (r *paymentPlans) Archive(ctx context.Context, id string) (Object, error) {
	return doObject(ctx, r.c, http.MethodDelete, "/api/v1/payment/plans/"+id, nil, "")
}

type PaymentSubscriptions interface {
	List(ctx context.Context, params map[string]any) (List, error)
	Get(ctx context.Context, id string) (Object, error)
	Create(ctx context.Context, input Object) (Object, error)
	// Cancel cancels at the end of the current period, or now when immediate.
	Cancel(ctx context.Context, id string, immediate bool) (Object, error)
}

type paymentSubscriptions struct{ c *Client }

func (r *paymentSubscriptions) List(ctx context.Context, params map[string]any) (List, error) {
	return doList(ctx, r.c, http.MethodGet, "/api/v1/payment/subscriptions"+qs(params), nil, "")
}
func (r *paymentSubscriptions) Get(ctx context.Context, id string) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/payment/subscriptions/"+id, nil, "")
}
func (r *paymentSubscriptions) Create(ctx context.Context, input Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/payment/subscriptions", input, r.c.genIdem())
}
func (r *paymentSubscriptions) Cancel(ctx context.Context, id string, immediate bool) (Object, error) {
	path := "/api/v1/payment/subscriptions/" + id
	if immediate {
		path += "?immediate=true"
	}
	return doObject(ctx, r.c, http.MethodDelete, path, nil, "")
}

type PaymentInvoices interface {
	List(ctx context.Context, params map[string]any) (List, error)
	Get(ctx context.Context, id string) (Object, error)
}

type paymentInvoices struct{ c *Client }

func (r *paymentInvoices) List(ctx context.Context, params map[string]any) (List, error) {
	return doList(ctx, r.c, http.MethodGet, "/api/v1/payment/invoices"+qs(params), nil, "")
}
func (r *paymentInvoices) Get(ctx context.Context, id string) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/payment/invoices/"+id, nil, "")
}

type PaymentReceipts interface {
	List(ctx context.Context, params map[string]any) (List, error)
	Get(ctx context.Context, id string) (Object, error)
}

type paymentReceipts struct{ c *Client }

func (r *paymentReceipts) List(ctx context.Context, params map[string]any) (List, error) {
	return doList(ctx, r.c, http.MethodGet, "/api/v1/payment/receipts"+qs(params), nil, "")
}
func (r *paymentReceipts) Get(ctx context.Context, id string) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/payment/receipts/"+id, nil, "")
}

type PaymentCustomers interface {
	List(ctx context.Context, params map[string]any) (List, error)
	Get(ctx context.Context, id string) (Object, error)
	Create(ctx context.Context, input Object) (Object, error)
	Update(ctx context.Context, id string, patch Object) (Object, error)
}

type paymentCustomers struct{ c *Client }

func (r *paymentCustomers) List(ctx context.Context, params map[string]any) (List, error) {
	return doList(ctx, r.c, http.MethodGet, "/api/v1/payment/customers"+qs(params), nil, "")
}
func (r *paymentCustomers) Get(ctx context.Context, id string) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/payment/customers/"+id, nil, "")
}
func (r *paymentCustomers) Create(ctx context.Context, input Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/payment/customers", input, r.c.genIdem())
}
func (r *paymentCustomers) Update(ctx context.Context, id string, patch Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPatch, "/api/v1/payment/customers/"+id, patch, "")
}

// PortalSessionInput is the typed payload for PortalSessions.Create —
// the only route on PaymentResource that has a strictly required shape.
type PortalSessionInput struct {
	CustomerID string `json:"customerId"`
	ReturnURL  string `json:"returnUrl"`
}

type PaymentPortalSessions interface {
	Create(ctx context.Context, input PortalSessionInput) (Object, error)
}

type paymentPortalSessions struct{ c *Client }

func (r *paymentPortalSessions) Create(ctx context.Context, input PortalSessionInput) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/payment/portal-sessions", input, r.c.genIdem())
}

// WebhookEndpointInput mirrors the Node `{ url; events? }` payload.
type WebhookEndpointInput struct {
	URL    string   `json:"url"`
	Events []string `json:"events,omitempty"`
}

type PaymentWebhookEndpoints interface {
	List(ctx context.Context) (List, error)
	Create(ctx context.Context, input WebhookEndpointInput) (Object, error)
	Delete(ctx context.Context, id string) (Object, error)
}

type paymentWebhookEndpoints struct{ c *Client }

func (r *paymentWebhookEndpoints) List(ctx context.Context) (List, error) {
	return doList(ctx, r.c, http.MethodGet, "/api/v1/payment/webhook-endpoints", nil, "")
}
func (r *paymentWebhookEndpoints) Create(ctx context.Context, input WebhookEndpointInput) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/payment/webhook-endpoints", input, r.c.genIdem())
}
func (r *paymentWebhookEndpoints) Delete(ctx context.Context, id string) (Object, error) {
	return doObject(ctx, r.c, http.MethodDelete, "/api/v1/payment/webhook-endpoints/"+id, nil, "")
}

type PaymentWebhookEvents interface {
	List(ctx context.Context, params map[string]any) (List, error)
	Get(ctx context.Context, id string) (Object, error)
}

type paymentWebhookEvents struct{ c *Client }

func (r *paymentWebhookEvents) List(ctx context.Context, params map[string]any) (List, error) {
	return doList(ctx, r.c, http.MethodGet, "/api/v1/payment/webhook-events"+qs(params), nil, "")
}
func (r *paymentWebhookEvents) Get(ctx context.Context, id string) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/payment/webhook-events/"+id, nil, "")
}

// ═══════════════════════════════════════════════════════════════════════
// STOREFRONT
// ═══════════════════════════════════════════════════════════════════════

type StorefrontResource struct {
	Products   StorefrontProducts
	Licenses   StorefrontLicenses
	Deliveries StorefrontDeliveries
	Public     StorefrontPublic
}

type StorefrontProducts interface {
	List(ctx context.Context, params map[string]any) (List, error)
	Get(ctx context.Context, id string) (Object, error)
	Create(ctx context.Context, input Object) (Object, error)
	Update(ctx context.Context, id string, patch Object) (Object, error)
	Archive(ctx context.Context, id string) (Object, error)
	AddFile(ctx context.Context, productID string, input Object) (Object, error)
	RemoveFile(ctx context.Context, productID, fileID string) (Object, error)
}

type storefrontProducts struct{ c *Client }

func (r *storefrontProducts) List(ctx context.Context, params map[string]any) (List, error) {
	return doList(ctx, r.c, http.MethodGet, "/api/v1/storefront/products"+qs(params), nil, "")
}
func (r *storefrontProducts) Get(ctx context.Context, id string) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/storefront/products/"+id, nil, "")
}
func (r *storefrontProducts) Create(ctx context.Context, input Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/storefront/products", input, r.c.genIdem())
}
func (r *storefrontProducts) Update(ctx context.Context, id string, patch Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPatch, "/api/v1/storefront/products/"+id, patch, "")
}
func (r *storefrontProducts) Archive(ctx context.Context, id string) (Object, error) {
	return doObject(ctx, r.c, http.MethodDelete, "/api/v1/storefront/products/"+id, nil, "")
}
func (r *storefrontProducts) AddFile(ctx context.Context, productID string, input Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/storefront/products/"+productID+"/files", input, "")
}
func (r *storefrontProducts) RemoveFile(ctx context.Context, productID, fileID string) (Object, error) {
	return doObject(ctx, r.c, http.MethodDelete, "/api/v1/storefront/products/"+productID+"/files/"+fileID, nil, "")
}

type StorefrontLicenses interface {
	List(ctx context.Context, params map[string]any) (List, error)
	Get(ctx context.Context, id string) (Object, error)
	Issue(ctx context.Context, input Object) (Object, error)
	// Revoke revokes a license by its key (DELETE /storefront/licenses/{key}).
	Revoke(ctx context.Context, key string) (Object, error)
}

type storefrontLicenses struct{ c *Client }

func (r *storefrontLicenses) List(ctx context.Context, params map[string]any) (List, error) {
	return doList(ctx, r.c, http.MethodGet, "/api/v1/storefront/licenses"+qs(params), nil, "")
}
func (r *storefrontLicenses) Get(ctx context.Context, id string) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/storefront/licenses/"+id, nil, "")
}
func (r *storefrontLicenses) Issue(ctx context.Context, input Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/storefront/licenses", input, r.c.genIdem())
}
func (r *storefrontLicenses) Revoke(ctx context.Context, key string) (Object, error) {
	return doObject(ctx, r.c, http.MethodDelete, "/api/v1/storefront/licenses/"+key, nil, "")
}

type StorefrontDeliveries interface {
	List(ctx context.Context, params map[string]any) (List, error)
	Get(ctx context.Context, id string) (Object, error)
}

type storefrontDeliveries struct{ c *Client }

func (r *storefrontDeliveries) List(ctx context.Context, params map[string]any) (List, error) {
	return doList(ctx, r.c, http.MethodGet, "/api/v1/storefront/deliveries"+qs(params), nil, "")
}
func (r *storefrontDeliveries) Get(ctx context.Context, id string) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/storefront/deliveries/"+id, nil, "")
}

type StorefrontPublic interface {
	Get(ctx context.Context, slug string) (Object, error)
}

type storefrontPublic struct{ c *Client }

func (r *storefrontPublic) Get(ctx context.Context, slug string) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/storefront/public/"+slug, nil, "")
}

// ═══════════════════════════════════════════════════════════════════════
// ACCOUNT
// ═══════════════════════════════════════════════════════════════════════

type AccountResource struct {
	c             *Client
	Pixels        AccountPixels
	AbandonedCart AccountAbandonedCart
	Feeds         AccountFeeds
	Blog          AccountBlog
	Referrals     AccountReferrals
	APIKeys       AccountAPIKeys
	AuditLog      AccountAuditLog
	Domains       AccountDomains
}

// Profile fetches the merchant profile attached to the calling key.
func (a *AccountResource) Profile(ctx context.Context) (Object, error) {
	return doObject(ctx, a.c, http.MethodGet, "/api/v1/account", nil, "")
}

// UpdateProfile partially updates the merchant profile.
func (a *AccountResource) UpdateProfile(ctx context.Context, patch Object) (Object, error) {
	return doObject(ctx, a.c, http.MethodPatch, "/api/v1/account", patch, "")
}

type AccountPixels interface {
	Get(ctx context.Context) (Object, error)
	Update(ctx context.Context, patch Object) (Object, error)
}

type accountPixels struct{ c *Client }

func (r *accountPixels) Get(ctx context.Context) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/account/pixels", nil, "")
}
func (r *accountPixels) Update(ctx context.Context, patch Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPatch, "/api/v1/account/pixels", patch, "")
}

type AccountAbandonedCart interface {
	GetConfig(ctx context.Context) (Object, error)
	UpdateConfig(ctx context.Context, patch Object) (Object, error)
}

type accountAbandonedCart struct{ c *Client }

func (r *accountAbandonedCart) GetConfig(ctx context.Context) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/account/abandoned-cart", nil, "")
}
func (r *accountAbandonedCart) UpdateConfig(ctx context.Context, patch Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPatch, "/api/v1/account/abandoned-cart", patch, "")
}

type AccountFeeds interface {
	GetConfig(ctx context.Context) (Object, error)
	UpdateConfig(ctx context.Context, patch Object) (Object, error)
}

type accountFeeds struct{ c *Client }

func (r *accountFeeds) GetConfig(ctx context.Context) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/account/feeds", nil, "")
}
func (r *accountFeeds) UpdateConfig(ctx context.Context, patch Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPatch, "/api/v1/account/feeds", patch, "")
}

type AccountBlog interface {
	// List returns {posts: [...]}.
	List(ctx context.Context, params map[string]any) (Object, error)
	Get(ctx context.Context, id string) (Object, error)
	Create(ctx context.Context, input Object) (Object, error)
	Update(ctx context.Context, id string, patch Object) (Object, error)
	Publish(ctx context.Context, id string) (Object, error)
	Delete(ctx context.Context, id string) (Object, error)
}

type accountBlog struct{ c *Client }

func (r *accountBlog) List(ctx context.Context, params map[string]any) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/account/blog/posts"+qs(params), nil, "")
}
func (r *accountBlog) Get(ctx context.Context, id string) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/account/blog/posts/"+id, nil, "")
}
func (r *accountBlog) Create(ctx context.Context, input Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/account/blog/posts", input, "")
}
func (r *accountBlog) Update(ctx context.Context, id string, patch Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPatch, "/api/v1/account/blog/posts/"+id, patch, "")
}
func (r *accountBlog) Publish(ctx context.Context, id string) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/account/blog/posts/"+id+"/publish", Object{}, "")
}
func (r *accountBlog) Delete(ctx context.Context, id string) (Object, error) {
	return doObject(ctx, r.c, http.MethodDelete, "/api/v1/account/blog/posts/"+id, nil, "")
}

type AccountReferrals interface {
	GetProgram(ctx context.Context) (Object, error)
	UpdateProgram(ctx context.Context, program Object) (Object, error)
}

type accountReferrals struct{ c *Client }

func (r *accountReferrals) GetProgram(ctx context.Context) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/account/referrals", nil, "")
}
func (r *accountReferrals) UpdateProgram(ctx context.Context, program Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPut, "/api/v1/account/referrals", program, "")
}

// AccountAPIKeys: creating and revoking keys needs a signed-in session; an
// API key gets 403.
type AccountAPIKeys interface {
	List(ctx context.Context) (List, error)
	Create(ctx context.Context, input APIKeyCreateInput) (Object, error)
	Revoke(ctx context.Context, id string) (Object, error)
}

// APIKeyCreateInput: Environment is "production" (an sk_live_ key) or
// "sandbox" (sk_test_).
type APIKeyCreateInput struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
}

type accountAPIKeys struct{ c *Client }

func (r *accountAPIKeys) List(ctx context.Context) (List, error) {
	return doList(ctx, r.c, http.MethodGet, "/api/v1/account/api-keys", nil, "")
}
func (r *accountAPIKeys) Create(ctx context.Context, input APIKeyCreateInput) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/account/api-keys", input, r.c.genIdem())
}
func (r *accountAPIKeys) Revoke(ctx context.Context, id string) (Object, error) {
	return doObject(ctx, r.c, http.MethodDelete, "/api/v1/account/api-keys/"+id, nil, "")
}

type AccountAuditLog interface {
	List(ctx context.Context, params map[string]any) (List, error)
}

type accountAuditLog struct{ c *Client }

func (r *accountAuditLog) List(ctx context.Context, params map[string]any) (List, error) {
	return doList(ctx, r.c, http.MethodGet, "/api/v1/account/audit-log"+qs(params), nil, "")
}

// DomainInput is the strict shape for AccountDomains.Add.
type DomainInput struct {
	Domain string `json:"domain"`
}

type AccountDomains interface {
	List(ctx context.Context) (List, error)
	Add(ctx context.Context, input DomainInput) (Object, error)
	Verify(ctx context.Context, id string) (Object, error)
	Remove(ctx context.Context, id string) (Object, error)
}

type accountDomains struct{ c *Client }

func (r *accountDomains) List(ctx context.Context) (List, error) {
	return doList(ctx, r.c, http.MethodGet, "/api/v1/account/domains", nil, "")
}
func (r *accountDomains) Add(ctx context.Context, input DomainInput) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/account/domains", input, "")
}
func (r *accountDomains) Verify(ctx context.Context, id string) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/account/domains/"+id+"/verify", Object{}, "")
}
func (r *accountDomains) Remove(ctx context.Context, id string) (Object, error) {
	return doObject(ctx, r.c, http.MethodDelete, "/api/v1/account/domains/"+id, nil, "")
}

// ═══════════════════════════════════════════════════════════════════════
// ANALYTICS
// ═══════════════════════════════════════════════════════════════════════

type AnalyticsResource struct{ c *Client }

func (r *AnalyticsResource) Overview(ctx context.Context) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/analytics/overview", nil, "")
}

// ═══════════════════════════════════════════════════════════════════════
// BILLING
// ═══════════════════════════════════════════════════════════════════════

type BillingResource struct{ c *Client }

func (r *BillingResource) Plans(ctx context.Context) (List, error) {
	return doList(ctx, r.c, http.MethodGet, "/api/v1/billing/plans", nil, "")
}
func (r *BillingResource) Subscription(ctx context.Context) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/billing/subscription", nil, "")
}
func (r *BillingResource) Usage(ctx context.Context) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/billing/usage", nil, "")
}
func (r *BillingResource) Invoices(ctx context.Context, params map[string]any) (List, error) {
	return doList(ctx, r.c, http.MethodGet, "/api/v1/billing/invoices"+qs(params), nil, "")
}

// BillingCheckoutInput is the typed payload for BillingResource.Checkout.
// Plan is "pro", "business" or "scale"; Interval "month" (default) or "year".
type BillingCheckoutInput struct {
	Plan     string `json:"plan"`
	Interval string `json:"interval,omitempty"`
	Currency string `json:"currency,omitempty"`
}

// Checkout upgrades: it starts a Plugipay subscription for the tier and
// returns where to pay (POST /billing/plugipay-invoice).
func (r *BillingResource) Checkout(ctx context.Context, input BillingCheckoutInput) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/billing/plugipay-invoice", input, r.c.genIdem())
}
func (r *BillingResource) Cancel(ctx context.Context) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/billing/cancel", Object{}, "")
}

// ═══════════════════════════════════════════════════════════════════════
// MODULES
// ═══════════════════════════════════════════════════════════════════════

type ModulesResource struct{ c *Client }

// List returns {modules, allowed, plan}: each module's on/off state and
// which modules the plan allows.
func (r *ModulesResource) List(ctx context.Context) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/modules", nil, "")
}

// Enable turns a module on: "payment", "fulfillment" or "marketing".
func (r *ModulesResource) Enable(ctx context.Context, name string) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/modules", Object{"module": name, "enabled": true}, "")
}
func (r *ModulesResource) Disable(ctx context.Context, name string) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/modules", Object{"module": name, "enabled": false}, "")
}

// ═══════════════════════════════════════════════════════════════════════
// MANUAL ORDERS
// ═══════════════════════════════════════════════════════════════════════

type ManualOrdersResource struct{ c *Client }

func (r *ManualOrdersResource) List(ctx context.Context, params map[string]any) (List, error) {
	return doList(ctx, r.c, http.MethodGet, "/api/v1/manual-orders"+qs(params), nil, "")
}
func (r *ManualOrdersResource) Get(ctx context.Context, id string) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/manual-orders/"+id, nil, "")
}

// ═══════════════════════════════════════════════════════════════════════
// ONBOARDING
// ═══════════════════════════════════════════════════════════════════════

type OnboardingResource struct{ c *Client }

func (r *OnboardingResource) Status(ctx context.Context) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/onboarding", nil, "")
}

// Complete marks onboarding done; enablePayment also turns on the Payment module.
func (r *OnboardingResource) Complete(ctx context.Context, enablePayment bool) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/onboarding/complete", Object{"enablePayment": enablePayment}, "")
}

// ═══════════════════════════════════════════════════════════════════════
// SHIPPING
// ═══════════════════════════════════════════════════════════════════════

type ShippingResource struct{ c *Client }

func (r *ShippingResource) Couriers(ctx context.Context) (List, error) {
	return doList(ctx, r.c, http.MethodGet, "/api/v1/shipping/couriers", nil, "")
}
func (r *ShippingResource) Origin(ctx context.Context) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/shipping/origin", nil, "")
}
func (r *ShippingResource) SetOrigin(ctx context.Context, input Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPatch, "/api/v1/shipping/origin", input, "")
}
func (r *ShippingResource) Rates(ctx context.Context, input Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/shipping/rates", input, "")
}
func (r *ShippingResource) ListShipments(ctx context.Context, params map[string]any) (List, error) {
	return doList(ctx, r.c, http.MethodGet, "/api/v1/shipping/shipments"+qs(params), nil, "")
}
func (r *ShippingResource) CreateShipment(ctx context.Context, input Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/shipping/shipments", input, r.c.genIdem())
}

// ═══════════════════════════════════════════════════════════════════════
// INVENTORY
// ═══════════════════════════════════════════════════════════════════════

type InventoryResource struct{ c *Client }

// Levels returns one variant's stock levels: params["variantId"] is required.
func (r *InventoryResource) Levels(ctx context.Context, params map[string]any) (List, error) {
	return doList(ctx, r.c, http.MethodGet, "/api/v1/inventory/stock"+qs(params), nil, "")
}
func (r *InventoryResource) Movements(ctx context.Context, params map[string]any) (List, error) {
	return doList(ctx, r.c, http.MethodGet, "/api/v1/inventory/movements"+qs(params), nil, "")
}
func (r *InventoryResource) Adjust(ctx context.Context, input Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/inventory/adjust", input, r.c.genIdem())
}
func (r *InventoryResource) Warehouses(ctx context.Context) (List, error) {
	return doList(ctx, r.c, http.MethodGet, "/api/v1/inventory/warehouses", nil, "")
}
func (r *InventoryResource) AddWarehouse(ctx context.Context, input Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/inventory/warehouses", input, "")
}

// ═══════════════════════════════════════════════════════════════════════
// LEDGER
// ═══════════════════════════════════════════════════════════════════════

type LedgerResource struct{ c *Client }

func (r *LedgerResource) List(ctx context.Context, params map[string]any) (List, error) {
	return doList(ctx, r.c, http.MethodGet, "/api/v1/ledger/entries"+qs(params), nil, "")
}
func (r *LedgerResource) Balances(ctx context.Context) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/ledger/balance", nil, "")
}
func (r *LedgerResource) Adjust(ctx context.Context, input Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/ledger/adjustments", input, r.c.genIdem())
}

// ═══════════════════════════════════════════════════════════════════════
// REPORTS
// ═══════════════════════════════════════════════════════════════════════

type ReportsResource struct{ c *Client }

// DateRange is the from/to window used by every report endpoint.
type DateRange struct {
	From string
	To   string
}

func (r *ReportsResource) PnL(ctx context.Context, p DateRange) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/reports/pnl"+qs(map[string]any{"from": p.From, "to": p.To}), nil, "")
}
func (r *ReportsResource) CashFlow(ctx context.Context, p DateRange) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/reports/cash-flow"+qs(map[string]any{"from": p.From, "to": p.To}), nil, "")
}

// ExportLedger returns the whole ledger as CSV text (GET /ledger/entries.csv).
func (r *ReportsResource) ExportLedger(ctx context.Context) (string, error) {
	var csv string
	err := r.c.Request(ctx, RequestArgs{Method: http.MethodGet, Path: "/api/v1/ledger/entries.csv", Out: &csv})
	return csv, err
}

// ═══════════════════════════════════════════════════════════════════════
// PAYOUTS
// ═══════════════════════════════════════════════════════════════════════

type PayoutsResource struct{ c *Client }

func (r *PayoutsResource) List(ctx context.Context, params map[string]any) (List, error) {
	return doList(ctx, r.c, http.MethodGet, "/api/v1/payouts"+qs(params), nil, "")
}
func (r *PayoutsResource) Get(ctx context.Context, id string) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/payouts/"+id, nil, "")
}
func (r *PayoutsResource) Request(ctx context.Context, input Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/payouts", input, r.c.genIdem())
}
func (r *PayoutsResource) BankAccount(ctx context.Context) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/payouts/bank-account", nil, "")
}
func (r *PayoutsResource) UpdateBankAccount(ctx context.Context, input Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPatch, "/api/v1/payouts/bank-account", input, "")
}

// ═══════════════════════════════════════════════════════════════════════
// DISCOUNT CODES
// ═══════════════════════════════════════════════════════════════════════

type DiscountCodesResource struct{ c *Client }

func (r *DiscountCodesResource) List(ctx context.Context, params map[string]any) (List, error) {
	return doList(ctx, r.c, http.MethodGet, "/api/v1/discount-codes"+qs(params), nil, "")
}
func (r *DiscountCodesResource) Get(ctx context.Context, id string) (Object, error) {
	return doObject(ctx, r.c, http.MethodGet, "/api/v1/discount-codes/"+id, nil, "")
}
func (r *DiscountCodesResource) Create(ctx context.Context, input Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPost, "/api/v1/discount-codes", input, r.c.genIdem())
}
func (r *DiscountCodesResource) Update(ctx context.Context, id string, patch Object) (Object, error) {
	return doObject(ctx, r.c, http.MethodPatch, "/api/v1/discount-codes/"+id, patch, "")
}

// ─── doObject / doList ───────────────────────────────────────────────────
// These wrappers fold the common `out = X; err = c.Request(...); return`
// pattern into one helper apiece, so the resource methods stay
// boilerplate-light and easy to scan.

func doObject(ctx context.Context, c *Client, method, path string, body any, idem string) (Object, error) {
	var out Object
	err := c.Request(ctx, RequestArgs{Method: method, Path: path, Body: body, IdempotencyKey: idem, Out: &out})
	return out, err
}

func doList(ctx context.Context, c *Client, method, path string, body any, idem string) (List, error) {
	var out List
	err := c.Request(ctx, RequestArgs{Method: method, Path: path, Body: body, IdempotencyKey: idem, Out: &out})
	return out, err
}

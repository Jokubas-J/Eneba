// Package httpapi is the HTTP layer: routing and the handlers that translate
// between HTTP/JSON and the service packages. No business rules and no SQL here.
package httpapi

import "net/http"

// Dependencies is everything the HTTP layer needs, passed in explicitly (no global
// variables), so this struct is the list of what the handlers depend on.
type Dependencies struct {
	Catalog  Catalog
	Accounts Accounts
	Orders   Orders
	// StripeWebhookSecret verifies Stripe webhooks; empty = webhooks are refused.
	StripeWebhookSecret string
	// Environment is APP_ENV. The webhook ignores Stripe events of sessions created
	// by another environment (they share one Stripe account).
	Environment string
	// AllowedOrigin is the frontend's address (APP_BASE_URL); state-changing
	// requests from any other site are refused.
	AllowedOrigin string
	// SecureCookies sets the Secure flag on the session cookie (production).
	SecureCookies bool
}

// NewRouter builds the complete route table of the API.
//
// It returns the http.Handler interface rather than the concrete *http.ServeMux so
// that main.go and the tests depend only on "something that can serve HTTP". Tests
// call this same function, so they exercise the real routes, not a copy of them.
func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	accounts := authHandlers{accounts: deps.Accounts, secureCookies: deps.SecureCookies}

	// Every route lives under /api because Vercel proxies only /api/* to this server;
	// keeping the prefix here means paths are identical locally and in production.
	mux.HandleFunc("GET /api/healthz", handleHealth)
	mux.HandleFunc("GET /api/products", handleListProducts(deps.Catalog))
	mux.HandleFunc("GET /api/products/{slug}", handleProduct(deps.Catalog))
	mux.HandleFunc("GET /api/search/suggest", handleSuggest(deps.Catalog))
	mux.HandleFunc("POST /api/cart/validate", handleValidateCart(deps.Catalog))

	mux.HandleFunc("POST /api/auth/register", accounts.register)
	mux.HandleFunc("POST /api/auth/login", accounts.login)
	mux.HandleFunc("POST /api/auth/logout", accounts.logout)
	mux.HandleFunc("GET /api/auth/me", accounts.me)

	orders := orderHandlers{orders: deps.Orders, accounts: accounts, webhookSecret: deps.StripeWebhookSecret, environment: deps.Environment}
	mux.HandleFunc("POST /api/checkout", orders.checkout)
	mux.HandleFunc("GET /api/orders", orders.list)
	mux.HandleFunc("GET /api/orders/{id}", orders.get)
	mux.HandleFunc("POST /api/orders/{id}/confirm", orders.confirm)
	mux.HandleFunc("POST /api/payments/simulated/{id}", orders.paySimulated)
	// Called by Stripe's servers, not by a browser: no Origin header, so the Origin
	// check lets it through, and the signature check inside decides.
	mux.HandleFunc("POST /api/webhooks/stripe", orders.stripeWebhook)

	return checkOrigin(deps.AllowedOrigin, mux)
}

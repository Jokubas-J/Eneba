// Package httpapi is the HTTP layer: routing and the handlers that translate
// between HTTP/JSON and the service packages. No business rules and no SQL here.
package httpapi

import "net/http"

// NewRouter builds the complete route table of the API.
//
// Everything a handler needs is passed in as a parameter (no global variables), so
// the list of parameters is the list of what the HTTP layer depends on.
//
// It returns the http.Handler interface rather than the concrete *http.ServeMux so
// that main.go and the tests depend only on "something that can serve HTTP". Tests
// call this same function, so they exercise the real routes, not a copy of them.
func NewRouter(products Catalog) http.Handler {
	mux := http.NewServeMux()

	// Every route lives under /api because Vercel proxies only /api/* to this server;
	// keeping the prefix here means paths are identical locally and in production.
	mux.HandleFunc("GET /api/healthz", handleHealth)
	mux.HandleFunc("GET /api/products", handleListProducts(products))
	mux.HandleFunc("GET /api/search/suggest", handleSuggest(products))

	return mux
}

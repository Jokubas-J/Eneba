// Package httpapi holds the HTTP layer: routing and the handlers that translate
// between HTTP/JSON and the service packages.
package httpapi

import "net/http"

// NewRouter builds the complete route table of the API.
//
// It returns the http.Handler interface rather than the concrete *http.ServeMux so
// that main.go and the tests depend only on "something that can serve HTTP". Tests
// call this same function, so they exercise the real routes, not a copy of them.
func NewRouter() http.Handler {
	mux := http.NewServeMux()

	// Every route lives under /api because Vercel proxies only /api/* to this server;
	// keeping the prefix here means paths are identical locally and in production.
	mux.HandleFunc("GET /api/healthz", handleHealth)

	return mux
}

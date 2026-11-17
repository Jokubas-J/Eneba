package httpapi

import (
	"context"
	"net/http"

	"github.com/Jokubas-J/Eneba/backend/internal/catalog"
	"github.com/Jokubas-J/Eneba/backend/internal/home"
	"github.com/Jokubas-J/Eneba/backend/internal/menu"
)

// Home is what the homepage handlers need: the layout and one method per block.
type Home interface {
	Blocks(ctx context.Context) ([]home.Block, error)
	Banners(ctx context.Context) ([]home.Banner, error)
	BestSellers(ctx context.Context) ([]catalog.ProductCard, error)
	Deals(ctx context.Context) ([]catalog.ProductCard, error)
	NewReleases(ctx context.Context) ([]catalog.ProductCard, error)
	Categories(ctx context.Context) ([]home.Category, error)
}

// itemsResponse is the shape of every homepage endpoint: {"items": [...]}. An object
// rather than a bare array leaves room for more fields later without breaking clients.
type itemsResponse[T any] struct {
	Items []T `json:"items"`
}

// homeCacheControl lets the browser reuse a homepage answer for a
// minute: the content changes rarely, and the homepage is the most visited page.
// Stock shown on cards may therefore be up to a minute old; the product page and
// checkout always read it fresh.
const homeCacheControl = "public, max-age=60"

// serveItems turns one homepage query into a handler. Each block has its own
// endpoint, so one failing query makes only that block show its error state.
// (A generic function, like a C# generic method: T is the element type.)
func serveItems[T any](load func(context.Context) ([]T, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := load(r.Context())
		if err != nil {
			writeInternalError(w, r, err)
			return
		}
		if items == nil {
			items = []T{} // [] in JSON, not null
		}
		w.Header().Set("Cache-Control", homeCacheControl)
		writeJSON(w, http.StatusOK, itemsResponse[T]{Items: items})
	}
}

// Menu is what GET /api/menu needs.
type Menu interface {
	Menu(ctx context.Context) ([]menu.Section, error)
}

// registerHome wraps each method in a closure instead of passing h.Blocks directly:
// taking a method value from a nil interface panics at once in Go, and tests of
// other routes build the router without a Home. The closure looks h up per request.
func registerHome(mux *http.ServeMux, h Home) {
	mux.HandleFunc("GET /api/home", serveItems(func(ctx context.Context) ([]home.Block, error) { return h.Blocks(ctx) }))
	mux.HandleFunc("GET /api/home/banners", serveItems(func(ctx context.Context) ([]home.Banner, error) { return h.Banners(ctx) }))
	mux.HandleFunc("GET /api/home/best-sellers", serveItems(func(ctx context.Context) ([]catalog.ProductCard, error) { return h.BestSellers(ctx) }))
	mux.HandleFunc("GET /api/home/deals", serveItems(func(ctx context.Context) ([]catalog.ProductCard, error) { return h.Deals(ctx) }))
	mux.HandleFunc("GET /api/home/new-releases", serveItems(func(ctx context.Context) ([]catalog.ProductCard, error) { return h.NewReleases(ctx) }))
	mux.HandleFunc("GET /api/home/categories", serveItems(func(ctx context.Context) ([]home.Category, error) { return h.Categories(ctx) }))
}

// registerMenu serves the site menu. It changes as rarely as the homepage and
// uses the same one-minute cache and {"items": [...]} shape.
func registerMenu(mux *http.ServeMux, m Menu) {
	mux.HandleFunc("GET /api/menu", serveItems(func(ctx context.Context) ([]menu.Section, error) { return m.Menu(ctx) }))
}

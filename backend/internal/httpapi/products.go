package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/Jokubas-J/Eneba/backend/internal/catalog"
)

// Catalog is what the product handlers need from the catalog.
//
// The interface is declared here, by the code that USES it, and *catalog.Service
// satisfies it without saying so (Go interfaces are implicit; there is no
// "implements" keyword). It lets the handler tests pass in a small fake instead of
// a real service with a database behind it.
type Catalog interface {
	List(ctx context.Context, filter catalog.ListFilter) (catalog.ListResult, error)
	Suggest(ctx context.Context, text string) ([]catalog.ProductCard, error)
}

// maxFilterValues caps how many values one filter group may carry. The store has a
// handful of platforms and genres; hundreds of values is not a real request.
const maxFilterValues = 50

// handleListProducts serves GET /api/products, the data of the store page.
//
// Query parameters (all optional):
//
//	text                            search query (typos and partial names are fine)
//	platform, region, genre, type   may be repeated: ?platform=steam&platform=gog
//	price_min, price_max            whole cents, inclusive
//	sort                            relevance | popular | price_asc | price_desc | newest | oldest | name
//	page                            1-based
func handleListProducts(products Catalog) http.HandlerFunc {
	// The outer function runs once, when the router is built, and captures its
	// dependency; the returned function runs for every request. This is the usual
	// Go way to give a handler dependencies without global variables.
	return func(w http.ResponseWriter, r *http.Request) {
		filter, err := parseListFilter(r.URL.Query())
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}

		result, err := products.List(r.Context(), filter)
		if errors.Is(err, catalog.ErrInvalidFilter) {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		if err != nil {
			writeInternalError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

// parseListFilter does the HTTP half of validation: are the parameters well formed?
// Whether the values make sense together (min <= max...) is the service's decision.
func parseListFilter(query url.Values) (catalog.ListFilter, error) {
	filter := catalog.ListFilter{
		Text: query.Get("text"),
		// query["platform"] is every value of a repeated parameter;
		// query.Get would return only the first.
		Platforms: query["platform"],
		Regions:   query["region"],
		Genres:    query["genre"],
		Types:     query["type"],
	}
	for name, values := range map[string][]string{
		"platform": filter.Platforms, "region": filter.Regions, "genre": filter.Genres, "type": filter.Types,
	} {
		if len(values) > maxFilterValues {
			return catalog.ListFilter{}, fmt.Errorf("too many values for %s", name)
		}
	}

	sort, ok := catalog.ParseSort(query.Get("sort"))
	if !ok {
		return catalog.ListFilter{}, errors.New("unknown sort")
	}
	filter.Sort = sort

	var err error
	if filter.PriceMin, err = optionalInt(query, "price_min"); err != nil {
		return catalog.ListFilter{}, err
	}
	if filter.PriceMax, err = optionalInt(query, "price_max"); err != nil {
		return catalog.ListFilter{}, err
	}

	page, err := optionalInt(query, "page")
	if err != nil {
		return catalog.ListFilter{}, err
	}
	filter.Page = 1
	if page != nil {
		filter.Page = *page
	}
	return filter, nil
}

// optionalInt reads a whole-number parameter. It returns nil when the parameter is
// absent or empty, and an error when it is present but not a number.
func optionalInt(query url.Values, name string) (*int, error) {
	text := query.Get(name)
	if text == "" {
		return nil, nil
	}
	value, err := strconv.Atoi(text)
	if err != nil {
		return nil, fmt.Errorf("%s must be a whole number", name)
	}
	return &value, nil
}

// suggestResponse is the body of GET /api/search/suggest. The list is wrapped in an
// object so that fields can be added later without breaking the frontend.
type suggestResponse struct {
	Items []catalog.ProductCard `json:"items"`
}

// handleSuggest serves GET /api/search/suggest?q=..., the product hits of the
// search dropdown in the header. It is called while the visitor types.
func handleSuggest(products Catalog) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := products.Suggest(r.Context(), r.URL.Query().Get("q"))
		if err != nil {
			writeInternalError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, suggestResponse{Items: items})
	}
}

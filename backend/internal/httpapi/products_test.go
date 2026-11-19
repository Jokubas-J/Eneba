package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/Jokubas-J/Eneba/backend/internal/catalog"
)

// fakeLister stands in for the catalog service: it records the filter the handler
// built from the URL and returns whatever the test prepared. No database involved.
type fakeLister struct {
	gotFilter catalog.ListFilter
	gotText   string
	called    bool
	result    catalog.ListResult
	err       error
}

func (f *fakeLister) Trending(ctx context.Context) ([]string, error) {
	return []string{"elden ring"}, nil
}

func (f *fakeLister) Suggest(ctx context.Context, text string) (catalog.SuggestResult, error) {
	f.called = true
	f.gotText = text
	return catalog.SuggestResult{Suggestions: []string{"alpha 2"}, Items: f.result.Items, Total: 9}, f.err
}

func (f *fakeLister) Product(ctx context.Context, slug string) (catalog.ProductDetail, error) {
	f.called = true
	f.gotText = slug
	if slug == "missing" {
		return catalog.ProductDetail{}, catalog.ErrNotFound
	}
	return catalog.ProductDetail{Slug: slug, Name: "Alpha"}, f.err
}

func (f *fakeLister) ValidateCart(ctx context.Context, lines []catalog.CartLine) (catalog.ValidatedCart, error) {
	f.called = true
	if len(lines) > 0 && lines[0].Quantity > 10 {
		return catalog.ValidatedCart{}, fmt.Errorf("%w: quantity must be between 1 and 10", catalog.ErrInvalidFilter)
	}
	return catalog.ValidatedCart{Items: []catalog.CartItem{{OfferID: lines[0].OfferID, Quantity: lines[0].Quantity, PriceCents: 500}}, TotalCents: 500, ItemCount: 1}, f.err
}

func (f *fakeLister) List(ctx context.Context, filter catalog.ListFilter) (catalog.ListResult, error) {
	f.called = true
	f.gotFilter = filter
	return f.result, f.err
}

func intPtr(v int) *int { return &v }

func get(t *testing.T, lister Catalog, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	NewRouter(Dependencies{Catalog: lister}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestListProductsParsesTheQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  catalog.ListFilter
	}{
		{
			name:  "no parameters gives the defaults",
			query: "",
			want:  catalog.ListFilter{Page: 1},
		},
		{
			name:  "search text is passed on as typed",
			query: "?text=Witchr+3&sort=relevance",
			want:  catalog.ListFilter{Text: "Witchr 3", Sort: catalog.SortRelevance, Page: 1},
		},
		{
			name:  "repeated parameters become lists",
			query: "?platform=steam&platform=gog&region=europe&genre=rpg&genre=action&type=game",
			want: catalog.ListFilter{
				Platforms: []string{"steam", "gog"}, Regions: []string{"europe"},
				Genres: []string{"rpg", "action"}, Types: []string{"game"},
				Page: 1,
			},
		},
		{
			name:  "prices, sort and page",
			query: "?price_min=500&price_max=2000&sort=price_asc&page=3",
			want: catalog.ListFilter{
				PriceMin: intPtr(500), PriceMax: intPtr(2000), Sort: catalog.SortPriceAsc, Page: 3,
			},
		},
		{
			name:  "empty values are treated as absent",
			query: "?price_min=&sort=&page=",
			want:  catalog.ListFilter{Page: 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lister := &fakeLister{result: catalog.ListResult{Items: []catalog.ProductCard{}}}

			rec := get(t, lister, "/api/products"+tt.query)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
			}
			if !reflect.DeepEqual(lister.gotFilter, tt.want) {
				t.Errorf("filter = %+v\n  want   %+v", lister.gotFilter, tt.want)
			}
		})
	}
}

func TestListProductsRejectsMalformedQueries(t *testing.T) {
	tooMany := "?" + strings.Repeat("platform=x&", maxFilterValues+1)

	for _, query := range []string{
		"?sort=cheapest",
		"?sort=price_asc%3BDROP%20TABLE%20products",
		"?page=two",
		"?page=1.5",
		"?price_min=ten",
		"?price_max=9.99",
		tooMany,
	} {
		t.Run(query[:min(len(query), 40)], func(t *testing.T) {
			lister := &fakeLister{}

			rec := get(t, lister, "/api/products"+query)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			if lister.called {
				t.Error("the service was called for a malformed request")
			}
			var body errorBody
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("error response is not JSON: %v", err)
			}
			if body.Error.Code != "invalid_request" || body.Error.Message == "" {
				t.Errorf("error body = %+v, want code invalid_request and a message", body.Error)
			}
		})
	}
}

func TestListProductsMapsServiceErrors(t *testing.T) {
	t.Run("invalid filter becomes 400 with the reason", func(t *testing.T) {
		lister := &fakeLister{err: fmt.Errorf("%w: price_min is greater than price_max", catalog.ErrInvalidFilter)}

		rec := get(t, lister, "/api/products?price_min=5&price_max=1")

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "price_min is greater than price_max") {
			t.Errorf("body %s does not explain the problem", rec.Body)
		}
	})

	t.Run("any other failure becomes 500 without internal details", func(t *testing.T) {
		lister := &fakeLister{err: errors.New(`pq: relation "product_listing" does not exist at 10.0.0.5`)}

		rec := get(t, lister, "/api/products")

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rec.Code)
		}
		if body := rec.Body.String(); strings.Contains(body, "product_listing") || strings.Contains(body, "10.0.0.5") {
			t.Errorf("internal error text leaked to the client: %s", body)
		}
	})
}

func TestListProductsJSONShape(t *testing.T) {
	lister := &fakeLister{result: catalog.ListResult{
		Items: []catalog.ProductCard{
			{
				ID: 7, Slug: "alpha", Name: "Alpha", Type: "game", CoverURL: "https://example.test/a.jpg",
				Platform: catalog.Named{Slug: "steam", Name: "Steam"}, Region: catalog.Named{Slug: "global", Name: "Global"},
				PriceCents: intPtr(1000), OriginalPriceCents: intPtr(2000), DiscountPercent: intPtr(50), OfferCount: 1,
			},
			{ID: 8, Slug: "delta", Name: "Delta", Type: "game"}, // sold out: no prices
		},
		Total: 2, Page: 1, PageSize: 24,
		Facets: catalog.Facets{
			Platforms: []catalog.FacetOption{{Slug: "steam", Name: "Steam", Count: 2}},
			Regions:   []catalog.FacetOption{{Slug: "global", Name: "Global", Count: 2}},
			Genres:    []catalog.FacetOption{},
			Types:     []catalog.FacetOption{{Slug: "game", Name: "Game", Count: 2}},
		},
	}}

	rec := get(t, lister, "/api/products")

	// The exact JSON is the contract with the frontend, so it is compared as text
	// (after re-indenting both sides the same way).
	want := `{
		"items": [
			{"id": 7, "slug": "alpha", "name": "Alpha", "type": "game", "coverUrl": "https://example.test/a.jpg",
			 "platform": {"slug": "steam", "name": "Steam"}, "region": {"slug": "global", "name": "Global"},
			 "priceCents": 1000, "originalPriceCents": 2000, "discountPercent": 50, "offerCount": 1},
			{"id": 8, "slug": "delta", "name": "Delta", "type": "game", "coverUrl": "",
			 "platform": {"slug": "", "name": ""}, "region": {"slug": "", "name": ""},
			 "priceCents": null, "originalPriceCents": null, "discountPercent": null, "offerCount": 0}
		],
		"total": 2, "page": 1, "pageSize": 24,
		"facets": {
			"platforms": [{"slug": "steam", "name": "Steam", "count": 2}],
			"regions": [{"slug": "global", "name": "Global", "count": 2}],
			"genres": [],
			"types": [{"slug": "game", "name": "Game", "count": 2}]
		}
	}`
	var gotValue, wantValue any
	if err := json.Unmarshal(rec.Body.Bytes(), &gotValue); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Errorf("response = %s\nwant %s", rec.Body, want)
	}
}

func TestSuggest(t *testing.T) {
	t.Run("passes the query on and wraps the hits", func(t *testing.T) {
		lister := &fakeLister{result: catalog.ListResult{Items: []catalog.ProductCard{{ID: 1, Slug: "alpha", Name: "Alpha"}}}}

		rec := get(t, lister, "/api/search/suggest?q=witchr+3")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if lister.gotText != "witchr 3" {
			t.Errorf("query passed to the service = %q, want %q", lister.gotText, "witchr 3")
		}
		var body struct {
			Suggestions []string `json:"suggestions"`
			Items       []struct {
				Slug string `json:"slug"`
			} `json:"items"`
			Total int `json:"total"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("response is not JSON: %v", err)
		}
		if len(body.Items) != 1 || body.Items[0].Slug != "alpha" {
			t.Errorf("items = %+v, want the one hit", body.Items)
		}
		if len(body.Suggestions) != 1 || body.Suggestions[0] != "alpha 2" || body.Total != 9 {
			t.Errorf("suggestions = %v, total = %d; want [alpha 2], 9", body.Suggestions, body.Total)
		}
	})

	t.Run("a failure becomes 500 without internal details", func(t *testing.T) {
		lister := &fakeLister{err: errors.New("connection refused to 10.0.0.5")}

		rec := get(t, lister, "/api/search/suggest?q=abc")

		if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "10.0.0.5") {
			t.Errorf("status = %d, body = %s", rec.Code, rec.Body)
		}
	})
}

func TestProductHandler(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		lister := &fakeLister{}
		rec := get(t, lister, "/api/products/alpha-pc-steam-key-global")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if lister.gotText != "alpha-pc-steam-key-global" {
			t.Errorf("slug passed to the service = %q", lister.gotText)
		}
		if !strings.Contains(rec.Body.String(), `"name":"Alpha"`) {
			t.Errorf("body = %s", rec.Body)
		}
	})

	t.Run("unknown slug is 404 with the error shape", func(t *testing.T) {
		rec := get(t, &fakeLister{}, "/api/products/missing")
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
			t.Errorf("status = %d, body = %s", rec.Code, rec.Body)
		}
	})

	t.Run("other failures are 500", func(t *testing.T) {
		rec := get(t, &fakeLister{err: errors.New("db down")}, "/api/products/alpha")
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rec.Code)
		}
	})

	t.Run("the list route still answers without a slug", func(t *testing.T) {
		lister := &fakeLister{result: catalog.ListResult{Items: []catalog.ProductCard{}}}
		if rec := get(t, lister, "/api/products"); rec.Code != http.StatusOK {
			t.Errorf("GET /api/products status = %d, want 200", rec.Code)
		}
	})
}

func TestValidateCartHandler(t *testing.T) {
	post := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/cart/validate", strings.NewReader(body))
		NewRouter(Dependencies{Catalog: &fakeLister{}}).ServeHTTP(rec, req)
		return rec
	}

	rec := post(`{"items":[{"offerId":3,"quantity":1}]}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"totalCents":500`) {
		t.Errorf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("cart answer is cacheable")
	}

	if rec := post(`{"items":[{"offerId":3,"quantity":99}]}`); rec.Code != http.StatusBadRequest {
		t.Errorf("invalid quantity: status = %d, want 400", rec.Code)
	}
	if rec := post(`not json`); rec.Code != http.StatusBadRequest {
		t.Errorf("not JSON: status = %d, want 400", rec.Code)
	}
}

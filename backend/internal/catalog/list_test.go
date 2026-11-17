package catalog_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/Jokubas-J/Eneba/backend/internal/catalog"
	"github.com/Jokubas-J/Eneba/backend/internal/seed"
	"github.com/Jokubas-J/Eneba/backend/internal/testdb"
)

// keys returns n distinct fake keys for one offer of the fixture.
func keys(prefix string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("TEST-%s-%d", prefix, i)
	}
	return out
}

// fixture is a tiny hand-written catalog in which every price, stock level and
// date is chosen so that each test below has exactly one right answer.
//
//	slug     platform/region  genres        popularity  released    buyable price
//	alpha    steam/global     action        100         2020-01-01  1000 (an 800 offer is sold out)
//	bravo    steam/europe     rpg           300         2022-01-01  500  (two offers in stock)
//	charlie  gog/global       action, rpg   200         2021-01-01  3000 (no discount)
//	delta    gog/europe       rpg           50          unknown     none (sold out)
func fixture() seed.Dataset {
	return seed.Dataset{
		Platforms: []seed.Named{{Slug: "steam", Name: "Steam"}, {Slug: "gog", Name: "GOG.com"}},
		Regions:   []seed.Named{{Slug: "global", Name: "Global"}, {Slug: "europe", Name: "Europe"}},
		Genres:    []seed.Named{{Slug: "action", Name: "Action"}, {Slug: "rpg", Name: "RPG"}},
		Sellers:   []seed.Seller{{Name: "Seller A", Rating: 9.5}, {Name: "Seller B", Rating: 9.0}},
		Products: []seed.Product{
			{
				Slug: "alpha", Name: "Alpha", Type: "game", PlatformSlug: "steam", RegionSlug: "global",
				GenreSlugs: []string{"action"}, Popularity: 100, ReleaseDate: "2020-01-01", SteamAppID: 1,
				Offers: []seed.Offer{
					{SellerName: "Seller A", PriceCents: 1000, OriginalPriceCents: 2000, Keys: keys("alpha-a", 2)},
					{SellerName: "Seller B", PriceCents: 800, OriginalPriceCents: 2000, Keys: nil}, // cheaper but sold out
				},
			},
			{
				// Lower-case name on purpose: sorting by name must ignore case.
				Slug: "bravo", Name: "bravo", Type: "game", PlatformSlug: "steam", RegionSlug: "europe",
				GenreSlugs: []string{"rpg"}, Popularity: 300, ReleaseDate: "2022-01-01", SteamAppID: 2,
				Offers: []seed.Offer{
					{SellerName: "Seller A", PriceCents: 500, OriginalPriceCents: 1000, Keys: keys("bravo-a", 1)},
					{SellerName: "Seller B", PriceCents: 700, OriginalPriceCents: 1000, Keys: keys("bravo-b", 3)},
				},
			},
			{
				Slug: "charlie", Name: "Charlie", Type: "game", PlatformSlug: "gog", RegionSlug: "global",
				GenreSlugs: []string{"action", "rpg"}, Popularity: 200, ReleaseDate: "2021-01-01", SteamAppID: 3,
				Offers: []seed.Offer{
					{SellerName: "Seller B", PriceCents: 3000, OriginalPriceCents: 3000, Keys: keys("charlie-b", 1)},
				},
			},
			{
				Slug: "delta", Name: "Delta", Type: "game", PlatformSlug: "gog", RegionSlug: "europe",
				GenreSlugs: []string{"rpg"}, Popularity: 50, ReleaseDate: "", SteamAppID: 4,
				Offers: []seed.Offer{
					{SellerName: "Seller A", PriceCents: 400, OriginalPriceCents: 900, Keys: nil},
				},
			},
		},
	}
}

// newService loads a dataset into the test database and returns a service on it.
func newService(t *testing.T, dataset seed.Dataset) (*catalog.Service, context.Context) {
	t.Helper()
	pool := testdb.Open(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	if _, err := seed.Load(ctx, pool, dataset, true); err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	// Searches are recorded as suggestions, and the counts change their order. Each
	// test starts from the migration's state: curated suggestions, never searched.
	_, err := pool.Exec(ctx, `
		DELETE FROM search_suggestions WHERE source = 'visitor';
		UPDATE search_suggestions SET search_count = 0, last_result_count = 0, last_searched_at = NULL`)
	if err != nil {
		t.Fatalf("reset search suggestions: %v", err)
	}
	return catalog.NewService(catalog.NewRepository(pool)), ctx
}

func slugs(items []catalog.ProductCard) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.Slug
	}
	return out
}

func intPtr(v int) *int { return &v }

func TestListFiltersAndSorting(t *testing.T) {
	service, ctx := newService(t, fixture())

	tests := []struct {
		name   string
		filter catalog.ListFilter
		want   []string // slugs in the expected order
	}{
		// Sorting. delta has neither price nor date, so it is last in both directions.
		{"default order is most popular first", catalog.ListFilter{}, []string{"bravo", "charlie", "alpha", "delta"}},
		{"price ascending", catalog.ListFilter{Sort: catalog.SortPriceAsc}, []string{"bravo", "alpha", "charlie", "delta"}},
		{"price descending", catalog.ListFilter{Sort: catalog.SortPriceDesc}, []string{"charlie", "alpha", "bravo", "delta"}},
		{"newest first", catalog.ListFilter{Sort: catalog.SortNewest}, []string{"bravo", "charlie", "alpha", "delta"}},
		{"oldest first", catalog.ListFilter{Sort: catalog.SortOldest}, []string{"alpha", "charlie", "bravo", "delta"}},
		{"by name, ignoring case", catalog.ListFilter{Sort: catalog.SortName}, []string{"alpha", "bravo", "charlie", "delta"}},

		// Filtering (default order).
		{"one platform", catalog.ListFilter{Platforms: []string{"steam"}}, []string{"bravo", "alpha"}},
		{"two platforms are OR", catalog.ListFilter{Platforms: []string{"steam", "gog"}}, []string{"bravo", "charlie", "alpha", "delta"}},
		{"platform and region are AND", catalog.ListFilter{Platforms: []string{"steam"}, Regions: []string{"europe"}}, []string{"bravo"}},
		{"genre", catalog.ListFilter{Genres: []string{"action"}}, []string{"charlie", "alpha"}},
		{"a product with both selected genres is listed once", catalog.ListFilter{Genres: []string{"action", "rpg"}}, []string{"bravo", "charlie", "alpha", "delta"}},
		{"minimum price uses the buyable price", catalog.ListFilter{PriceMin: intPtr(900)}, []string{"charlie", "alpha"}},
		{"maximum price is inclusive and drops sold-out products", catalog.ListFilter{PriceMax: intPtr(1000)}, []string{"bravo", "alpha"}},
		{"price range", catalog.ListFilter{PriceMin: intPtr(501), PriceMax: intPtr(2999)}, []string{"alpha"}},
		{"type", catalog.ListFilter{Types: []string{"game"}}, []string{"bravo", "charlie", "alpha", "delta"}},
		{"unknown value matches nothing", catalog.ListFilter{Platforms: []string{"nintendo"}}, []string{}},
		{"type with no products", catalog.ListFilter{Types: []string{"dlc"}}, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.List(ctx, tt.filter)
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			if got := slugs(result.Items); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("products = %v, want %v", got, tt.want)
			}
			if result.Total != len(tt.want) {
				t.Errorf("total = %d, want %d", result.Total, len(tt.want))
			}
			if result.Items == nil {
				t.Error("Items is nil; it must be an empty list so the JSON is [] and not null")
			}
			if result.Page != 1 || result.PageSize != catalog.PageSize {
				t.Errorf("page = %d, pageSize = %d; want 1, %d", result.Page, result.PageSize, catalog.PageSize)
			}
		})
	}
}

func TestListPrices(t *testing.T) {
	service, ctx := newService(t, fixture())

	result, err := service.List(ctx, catalog.ListFilter{Sort: catalog.SortName})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(result.Items) != 4 {
		t.Fatalf("got %d products, want 4", len(result.Items))
	}
	alpha, bravo, charlie, delta := result.Items[0], result.Items[1], result.Items[2], result.Items[3]

	t.Run("a cheaper sold-out offer does not set the price", func(t *testing.T) {
		if alpha.PriceCents == nil || *alpha.PriceCents != 1000 {
			t.Errorf("alpha price = %v, want 1000 (the 800 offer has no keys)", deref(alpha.PriceCents))
		}
		if alpha.OfferCount != 1 {
			t.Errorf("alpha offerCount = %d, want 1", alpha.OfferCount)
		}
		if alpha.DiscountPercent == nil || *alpha.DiscountPercent != 50 {
			t.Errorf("alpha discount = %v, want 50", deref(alpha.DiscountPercent))
		}
	})

	t.Run("the cheapest of several in-stock offers wins", func(t *testing.T) {
		if bravo.PriceCents == nil || *bravo.PriceCents != 500 || bravo.OfferCount != 2 {
			t.Errorf("bravo price = %v, offerCount = %d; want 500, 2", deref(bravo.PriceCents), bravo.OfferCount)
		}
	})

	t.Run("no discount is 0, not null", func(t *testing.T) {
		if charlie.DiscountPercent == nil || *charlie.DiscountPercent != 0 {
			t.Errorf("charlie discount = %v, want 0", deref(charlie.DiscountPercent))
		}
	})

	t.Run("a sold-out product is listed without prices", func(t *testing.T) {
		if delta.PriceCents != nil || delta.OriginalPriceCents != nil || delta.DiscountPercent != nil {
			t.Errorf("delta prices = %v / %v / %v, want all null",
				deref(delta.PriceCents), deref(delta.OriginalPriceCents), deref(delta.DiscountPercent))
		}
		if delta.OfferCount != 0 {
			t.Errorf("delta offerCount = %d, want 0", delta.OfferCount)
		}
	})

	t.Run("platform and region carry slug and name", func(t *testing.T) {
		if charlie.Platform != (catalog.Named{Slug: "gog", Name: "GOG.com"}) || charlie.Region != (catalog.Named{Slug: "global", Name: "Global"}) {
			t.Errorf("charlie platform = %+v, region = %+v", charlie.Platform, charlie.Region)
		}
	})
}

// deref makes a *int printable in a failure message.
func deref(p *int) any {
	if p == nil {
		return "null"
	}
	return *p
}

func TestListPagination(t *testing.T) {
	// 30 products that all have the SAME popularity: only the unique tie-breaker in
	// ORDER BY keeps a product from appearing on two pages.
	const count = 30
	dataset := fixture()
	dataset.Products = nil
	for i := 0; i < count; i++ {
		dataset.Products = append(dataset.Products, seed.Product{
			Slug: fmt.Sprintf("product-%02d", i), Name: fmt.Sprintf("Product %02d", i), Type: "game",
			PlatformSlug: "steam", RegionSlug: "global", Popularity: 7, SteamAppID: 100 + i,
		})
	}
	service, ctx := newService(t, dataset)

	seen := map[string]bool{}
	for page, wantItems := range map[int]int{1: catalog.PageSize, 2: count - catalog.PageSize, 3: 0} {
		result, err := service.List(ctx, catalog.ListFilter{Page: page})
		if err != nil {
			t.Fatalf("List(page %d) error = %v", page, err)
		}
		if len(result.Items) != wantItems {
			t.Errorf("page %d has %d items, want %d", page, len(result.Items), wantItems)
		}
		if result.Total != count || result.Page != page {
			t.Errorf("page %d: total = %d, page = %d; want %d, %d", page, result.Total, result.Page, count, page)
		}
		for _, item := range result.Items {
			if seen[item.Slug] {
				t.Errorf("%s appears on more than one page", item.Slug)
			}
			seen[item.Slug] = true
		}
	}
	if len(seen) != count {
		t.Errorf("pages 1-3 contain %d different products, want %d", len(seen), count)
	}
}

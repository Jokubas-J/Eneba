package home_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Jokubas-J/Eneba/backend/internal/catalog"
	"github.com/Jokubas-J/Eneba/backend/internal/home"
	"github.com/Jokubas-J/Eneba/backend/internal/seed"
	"github.com/Jokubas-J/Eneba/backend/internal/testdb"
)

// Five products with known popularity, prices, release dates and genres:
//
//	slug       popularity  price/original  released     genres          stock
//	famous     1000        900 / 1000      2020-01-01   action          yes
//	niche      10          500 / 2000      2024-05-01   action, indie   yes
//	middle     100         1000 / 1000     2025-01-01   indie           yes
//	soldout    5000        —               2025-06-01   action          no (no keys)
//	upcoming   50          3000 / 3000     2099-01-01   action          yes
func setup(t *testing.T) (*home.Service, context.Context, func(slug string, units int, status string, paidAgo time.Duration)) {
	t.Helper()
	pool := testdb.Open(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	keyCount := 0 // key values must be unique across the whole fixture
	offer := func(price, original int, keys int) []seed.Offer {
		o := seed.Offer{SellerName: "Seller", PriceCents: price, OriginalPriceCents: original}
		for range keys {
			keyCount++
			o.Keys = append(o.Keys, fmt.Sprintf("TEST-KEY-%d", keyCount))
		}
		return []seed.Offer{o}
	}
	product := func(slug string, popularity int, released string, genres []string, offers []seed.Offer) seed.Product {
		return seed.Product{Slug: slug, Name: slug, Type: "game", PlatformSlug: "steam", RegionSlug: "global",
			Popularity: popularity, ReleaseDate: released, GenreSlugs: genres, Offers: offers, SteamAppID: popularity}
	}
	dataset := seed.Dataset{
		Platforms: []seed.Named{{Slug: "steam", Name: "Steam"}},
		Regions:   []seed.Named{{Slug: "global", Name: "Global"}},
		Genres:    []seed.Named{{Slug: "action", Name: "Action"}, {Slug: "indie", Name: "Indie"}},
		Sellers:   []seed.Seller{{Name: "Seller", Rating: 9}},
		Products: []seed.Product{
			product("famous", 1000, "2020-01-01", []string{"action"}, offer(900, 1000, 20)),
			product("niche", 10, "2024-05-01", []string{"action", "indie"}, offer(500, 2000, 20)),
			product("middle", 100, "2025-01-01", []string{"indie"}, offer(1000, 1000, 20)),
			product("soldout", 5000, "2025-06-01", []string{"action"}, offer(100, 1000, 0)),
			product("upcoming", 50, "2099-01-01", []string{"action"}, offer(3000, 3000, 20)),
		},
	}
	if _, err := pool.Exec(ctx, `TRUNCATE users, sessions RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	if _, err := seed.Load(ctx, pool, dataset, true); err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	var userID int64
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ('buyer@example.com', 'x') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}

	// sell records an order of units keys of a product, paid paidAgo ago.
	sell := func(slug string, units int, status string, paidAgo time.Duration) {
		t.Helper()
		var paidAt any
		if status == "paid" {
			paidAt = time.Now().Add(-paidAgo)
		}
		_, err := pool.Exec(ctx, `
			WITH o AS (
				INSERT INTO orders (user_id, status, total_cents, currency, payment_provider, expires_at, paid_at)
				VALUES ($1, $2, 0, 'EUR', 'simulated', now(), $3) RETURNING id
			)
			INSERT INTO order_items (order_id, offer_id, product_name, seller_name, unit_price_cents, quantity)
			SELECT o.id, off.id, p.name, 'Seller', off.price_cents, $5
			FROM o, offers off JOIN products p ON p.id = off.product_id
			WHERE p.slug = $4`, userID, status, paidAt, slug, units)
		if err != nil {
			t.Fatalf("record sale: %v", err)
		}
	}
	return home.NewService(pool), ctx, sell
}

func slugs(cards []catalog.ProductCard) []string {
	out := make([]string, len(cards))
	for i, c := range cards {
		out[i] = c.Slug
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestBestSellers(t *testing.T) {
	service, ctx, sell := setup(t)

	t.Run("without sales the order is popularity, in stock only", func(t *testing.T) {
		cards, err := service.BestSellers(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"famous", "middle", "upcoming", "niche"}; !equal(slugs(cards), want) {
			t.Errorf("got %v, want %v (soldout must be missing)", slugs(cards), want)
		}
	})

	t.Run("recent paid sales come first; old, pending and expired orders do not count", func(t *testing.T) {
		sell("niche", 3, "paid", 24*time.Hour)      // counts: 3
		sell("middle", 2, "paid", 24*time.Hour)     // counts: 2
		sell("middle", 50, "paid", 40*24*time.Hour) // too old
		sell("upcoming", 50, "pending", 0)          // not paid
		sell("upcoming", 50, "expired", 0)          // not paid
		cards, err := service.BestSellers(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"niche", "middle", "famous", "upcoming"}; !equal(slugs(cards), want) {
			t.Errorf("got %v, want %v", slugs(cards), want)
		}
	})
}

func TestDeals(t *testing.T) {
	service, ctx, _ := setup(t)
	cards, err := service.Deals(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// niche -75%, famous -10%; middle and upcoming have no discount; soldout has no price.
	if want := []string{"niche", "famous"}; !equal(slugs(cards), want) {
		t.Errorf("got %v, want %v", slugs(cards), want)
	}
	if cards[0].DiscountPercent == nil || *cards[0].DiscountPercent != 75 {
		t.Errorf("niche discount = %v, want 75", cards[0].DiscountPercent)
	}
}

func TestNewReleases(t *testing.T) {
	service, ctx, _ := setup(t)
	cards, err := service.NewReleases(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Newest first; the 2099 pre-order and the sold-out product are left out.
	if want := []string{"middle", "niche", "famous"}; !equal(slugs(cards), want) {
		t.Errorf("got %v, want %v", slugs(cards), want)
	}
}

func TestCategories(t *testing.T) {
	service, ctx, _ := setup(t)
	got, err := service.Categories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// action: famous, niche, upcoming (soldout does not count) = 3; indie: niche, middle = 2.
	// Icons come from taxonomy_icons (migration 00013), matched by slug.
	type row struct {
		slug, name string
		count      int
		icon       string
	}
	want := []row{{"action", "Action", 3, "swords"}, {"indie", "Indie", 2, "sparkles"}}
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i, w := range want {
		g := got[i]
		if g.Slug != w.slug || g.Name != w.name || g.Count != w.count || g.Icon == nil || *g.Icon != w.icon {
			t.Errorf("category %d = %+v (icon %v), want %+v", i, g, g.Icon, w)
		}
	}
}

// The layout and the banners come from migration 00011, which a re-seed leaves alone.
func TestLayoutAndBannersComeFromTheDatabase(t *testing.T) {
	service, ctx, _ := setup(t)
	blocks, err := service.Blocks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var types []string
	for _, b := range blocks {
		types = append(types, b.Type+" "+b.Source)
	}
	want := []string{
		"banner_mosaic /api/home/banners",
		"product_carousel /api/home/best-sellers",
		"product_carousel /api/home/deals",
		"product_carousel /api/home/new-releases",
		"category_grid /api/home/categories",
	}
	if !equal(types, want) {
		t.Errorf("blocks = %v, want %v", types, want)
	}
	banners, err := service.Banners(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(banners) != 4 || banners[0].Title != "ELDEN RING" {
		t.Errorf("banners = %+v, want 4 with ELDEN RING first", banners)
	}
}

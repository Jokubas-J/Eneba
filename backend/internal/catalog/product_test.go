package catalog_test

import (
	"errors"
	"testing"

	"github.com/Jokubas-J/Eneba/backend/internal/catalog"
	"github.com/Jokubas-J/Eneba/backend/internal/seed"
)

// productFixture: one game in three versions, and offers chosen so each rule of the
// offer comparison has exactly one right answer.
//
//	steam/global  Seller A  900  sold out    <- cheapest overall, but cannot be bought
//	              Seller B  1200 3 in stock
//	              Seller C  1000 1 in stock  <- cheapest IN STOCK
//	              Seller D  1000 5 in stock  same price as C, lower rating
//	              Seller E  500  inactive    <- not shown at all
//	steam/europe  Seller A  1100 2 in stock
//	gog/global    Seller B  1300 sold out
func productFixture() seed.Dataset {
	return seed.Dataset{
		Platforms: []seed.Named{{Slug: "steam", Name: "Steam"}, {Slug: "gog", Name: "GOG.com"}},
		Regions:   []seed.Named{{Slug: "global", Name: "Global"}, {Slug: "europe", Name: "Europe"}},
		Genres:    []seed.Named{{Slug: "rpg", Name: "RPG"}, {Slug: "action", Name: "Action"}},
		Sellers: []seed.Seller{
			{Name: "Seller A", Rating: 9.9, RatingCount: 100, IsVerified: true},
			{Name: "Seller B", Rating: 9.0, RatingCount: 50},
			{Name: "Seller C", Rating: 9.5, RatingCount: 20, IsVerified: true},
			{Name: "Seller D", Rating: 8.0, RatingCount: 10},
			{Name: "Seller E", Rating: 7.0, RatingCount: 5},
		},
		Products: []seed.Product{
			{
				Slug: "game-steam-global", Name: "Game (PC) Steam Key GLOBAL", Type: "game",
				Description: "A game.", PlatformSlug: "steam", RegionSlug: "global", SteamAppID: 42,
				GenreSlugs: []string{"rpg", "action"}, ReleaseDate: "2021-02-03", Developer: "Dev", Publisher: "Pub",
				Offers: []seed.Offer{
					{SellerName: "Seller A", PriceCents: 900, OriginalPriceCents: 2000},
					{SellerName: "Seller B", PriceCents: 1200, OriginalPriceCents: 2000, Keys: keys("b", 3)},
					{SellerName: "Seller C", PriceCents: 1000, OriginalPriceCents: 2000, Keys: keys("c", 1)},
					{SellerName: "Seller D", PriceCents: 1000, OriginalPriceCents: 2000, Keys: keys("d", 5)},
					{SellerName: "Seller E", PriceCents: 500, OriginalPriceCents: 2000, Keys: keys("e", 9)},
				},
			},
			{
				Slug: "game-steam-europe", Name: "Game (PC) Steam Key EUROPE", Type: "game",
				PlatformSlug: "steam", RegionSlug: "europe", SteamAppID: 42,
				Offers: []seed.Offer{{SellerName: "Seller A", PriceCents: 1100, OriginalPriceCents: 1500, Keys: keys("ea", 2)}},
			},
			{
				Slug: "game-gog-global", Name: "Game (PC) GOG.com Key GLOBAL", Type: "game",
				PlatformSlug: "gog", RegionSlug: "global", SteamAppID: 42,
				Offers: []seed.Offer{{SellerName: "Seller B", PriceCents: 1300, OriginalPriceCents: 1500}},
			},
			{
				// Another game: must not appear among the versions.
				Slug: "other-game", Name: "Other Game (PC) Steam Key GLOBAL", Type: "game",
				PlatformSlug: "steam", RegionSlug: "global", SteamAppID: 7,
			},
		},
	}
}

func TestProduct(t *testing.T) {
	service, ctx, pool := newServiceWithPool(t, productFixture())

	// The seed has no "inactive" flag; switch Seller E's offer off directly.
	if _, err := pool.Exec(ctx, `
		UPDATE offers SET is_active = false
		WHERE seller_id = (SELECT id FROM sellers WHERE name = 'Seller E')`); err != nil {
		t.Fatal(err)
	}

	p, err := service.Product(ctx, "game-steam-global")
	if err != nil {
		t.Fatalf("Product() error = %v", err)
	}

	t.Run("details", func(t *testing.T) {
		if p.Name != "Game (PC) Steam Key GLOBAL" || p.Platform.Name != "Steam" || p.Region.Slug != "global" {
			t.Errorf("product = %+v", p)
		}
		if p.ReleaseDate == nil || *p.ReleaseDate != "2021-02-03" || p.Developer != "Dev" || p.Publisher != "Pub" {
			t.Errorf("release date = %v, developer = %q, publisher = %q", p.ReleaseDate, p.Developer, p.Publisher)
		}
		if len(p.Genres) != 2 || p.Genres[0].Name != "Action" || p.Genres[1].Name != "RPG" {
			t.Errorf("genres = %+v, want Action, RPG", p.Genres)
		}
	})

	t.Run("offers: in stock by price, sold out last, inactive hidden", func(t *testing.T) {
		var got []string
		for _, o := range p.Offers {
			got = append(got, o.Seller.Name)
		}
		// C and D share the price; C has the better rating.
		want := []string{"Seller C", "Seller D", "Seller B", "Seller A"}
		if len(got) != len(want) {
			t.Fatalf("offers = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("offers = %v, want %v", got, want)
			}
		}
	})

	t.Run("stock is the number of available keys", func(t *testing.T) {
		stock := map[string]int{}
		for _, o := range p.Offers {
			stock[o.Seller.Name] = o.Stock
		}
		want := map[string]int{"Seller A": 0, "Seller B": 3, "Seller C": 1, "Seller D": 5}
		for name, n := range want {
			if stock[name] != n {
				t.Errorf("%s stock = %d, want %d", name, stock[name], n)
			}
		}
	})

	t.Run("a cheaper sold-out offer is not the cheapest", func(t *testing.T) {
		var cheapest []string
		for _, o := range p.Offers {
			if o.IsCheapest {
				cheapest = append(cheapest, o.Seller.Name)
			}
		}
		if len(cheapest) != 1 || cheapest[0] != "Seller C" {
			t.Errorf("cheapest = %v, want exactly [Seller C]", cheapest)
		}
	})

	t.Run("seller and discount", func(t *testing.T) {
		c := p.Offers[0]
		if c.Seller.Rating != 9.5 || c.Seller.RatingCount != 20 || !c.Seller.IsVerified {
			t.Errorf("seller = %+v", c.Seller)
		}
		if c.DiscountPercent != 50 {
			t.Errorf("discount = %d, want 50", c.DiscountPercent)
		}
	})

	t.Run("other versions of the same game", func(t *testing.T) {
		if len(p.OtherVersions) != 2 {
			t.Fatalf("other versions = %+v, want 2", p.OtherVersions)
		}
		gog, europe := p.OtherVersions[0], p.OtherVersions[1] // ordered by platform name
		if gog.Slug != "game-gog-global" || gog.PriceCents != nil || gog.OfferCount != 0 {
			t.Errorf("GOG version = %+v, want sold out", gog)
		}
		if europe.Slug != "game-steam-europe" || europe.PriceCents == nil || *europe.PriceCents != 1100 {
			t.Errorf("Europe version = %+v, want price 1100", europe)
		}
	})

	t.Run("all offers sold out: none is cheapest", func(t *testing.T) {
		gog, err := service.Product(ctx, "game-gog-global")
		if err != nil {
			t.Fatal(err)
		}
		if len(gog.Offers) != 1 || gog.Offers[0].IsCheapest || gog.Offers[0].Stock != 0 {
			t.Errorf("offers = %+v", gog.Offers)
		}
	})

	t.Run("no offers and no versions are empty lists", func(t *testing.T) {
		other, err := service.Product(ctx, "other-game")
		if err != nil {
			t.Fatal(err)
		}
		if other.Offers == nil || len(other.Offers) != 0 || other.OtherVersions == nil || other.Genres == nil {
			t.Errorf("product = %+v, want empty (not null) lists", other)
		}
		if other.ReleaseDate != nil {
			t.Errorf("release date = %v, want null", *other.ReleaseDate)
		}
	})

	t.Run("unknown slug", func(t *testing.T) {
		if _, err := service.Product(ctx, "no-such-product"); !errors.Is(err, catalog.ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

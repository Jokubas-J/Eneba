package menu_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Jokubas-J/Eneba/backend/internal/menu"
	"github.com/Jokubas-J/Eneba/backend/internal/seed"
	"github.com/Jokubas-J/Eneba/backend/internal/testdb"
)

// Three products: two on Steam (one of them sold out), one on PSN.
func setup(t *testing.T) (*menu.Service, context.Context) {
	t.Helper()
	pool := testdb.Open(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	keyCount := 0
	offers := func(keys int) []seed.Offer {
		o := seed.Offer{SellerName: "Seller", PriceCents: 500, OriginalPriceCents: 1000}
		for range keys {
			keyCount++
			o.Keys = append(o.Keys, fmt.Sprintf("MENU-KEY-%d", keyCount))
		}
		return []seed.Offer{o}
	}
	dataset := seed.Dataset{
		Platforms: []seed.Named{{Slug: "steam", Name: "Steam"}, {Slug: "psn", Name: "PSN"}, {Slug: "brand-new", Name: "Brand New"}},
		Regions:   []seed.Named{{Slug: "global", Name: "Global"}},
		Genres:    []seed.Named{{Slug: "action", Name: "Action"}, {Slug: "brand-new-genre", Name: "Brand New Genre"}},
		Sellers:   []seed.Seller{{Name: "Seller", Rating: 9}},
		Products: []seed.Product{
			{Slug: "a", Name: "A", Type: "game", PlatformSlug: "steam", RegionSlug: "global", SteamAppID: 1, GenreSlugs: []string{"action"}, Offers: offers(3)},
			{Slug: "b", Name: "B", Type: "game", PlatformSlug: "steam", RegionSlug: "global", SteamAppID: 2, GenreSlugs: []string{"brand-new-genre"}, Offers: offers(0)},
			{Slug: "c", Name: "C", Type: "game", PlatformSlug: "psn", RegionSlug: "global", SteamAppID: 3, GenreSlugs: []string{"brand-new-genre"}, Offers: offers(2)},
		},
	}
	if _, err := pool.Exec(ctx, `TRUNCATE users, sessions RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	if _, err := seed.Load(ctx, pool, dataset, true); err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	return menu.NewService(pool), ctx
}

func str(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

func TestMenu(t *testing.T) {
	service, ctx := setup(t)
	sections, err := service.Menu(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byTitle := map[string]menu.Section{}
	var order []string
	for _, s := range sections {
		byTitle[s.Title] = s
		order = append(order, s.Placement+":"+s.Title)
	}

	t.Run("sections come from the table, drawer first, in order", func(t *testing.T) {
		want := fmt.Sprint([]string{"drawer:All games", "drawer:Platforms", "drawer:Genres", "drawer:Regions",
			"drawer:By price", "drawer:Deals & discovery", "strip:Quick links"})
		if got := fmt.Sprint(order); got != want {
			t.Errorf("order = %s\nwant    %s", got, want)
		}
		if all := byTitle["All games"]; str(all.URL) != "/store" || len(all.Links) != 0 {
			t.Errorf("All games = %+v", all)
		}
	})

	t.Run("generated sections have live counts, store links and icons", func(t *testing.T) {
		platforms := byTitle["Platforms"].Links
		// steam: only "a" is in stock (1); psn: "c" (1); "brand-new" has no products: left out.
		if len(platforms) != 2 {
			t.Fatalf("platforms = %+v", platforms)
		}
		for _, l := range platforms {
			if *l.Count != 1 {
				t.Errorf("%s count = %d, want 1", l.Label, *l.Count)
			}
		}
		if platforms[0].URL != "/store?platform=psn" && platforms[0].URL != "/store?platform=steam" {
			t.Errorf("url = %s", platforms[0].URL)
		}
		genres := byTitle["Genres"].Links
		// A genre that is not in taxonomy_icons still appears, with no icon (the
		// frontend draws its default icon).
		got := map[string]string{}
		for _, l := range genres {
			got[l.Label] = fmt.Sprintf("%d %s %s", *l.Count, l.URL, str(l.Icon))
		}
		want := map[string]string{
			"Action":          "1 /store?genre=action swords",
			"Brand New Genre": "1 /store?genre=brand-new-genre <nil>",
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("genres = %v\nwant     %v", got, want)
		}
	})

	t.Run("hand-written sections keep their order and icons", func(t *testing.T) {
		deals := byTitle["Deals & discovery"].Links
		if len(deals) != 4 || deals[0].Label != "Biggest discounts" || str(deals[0].Icon) != "badge-percent" || deals[0].Count != nil {
			t.Errorf("deals = %+v", deals)
		}
		strip := byTitle["Quick links"].Links
		if len(strip) != 6 || strip[0].URL != "/store?platform=steam" {
			t.Errorf("strip = %+v", strip)
		}
	})
}

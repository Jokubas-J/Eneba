package seed_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Jokubas-J/Eneba/backend/internal/seed"
	"github.com/Jokubas-J/Eneba/backend/internal/testdb"
)

func TestLoad(t *testing.T) {
	pool := testdb.Open(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	games := []seed.Game{
		{SteamAppID: 10, Type: "game", Name: "First Game", PriceCents: 1999, Genres: []string{"Action", "RPG"}, ReleaseDate: "2020-03-04", Recommendations: 500,
			Screenshots: []seed.Screenshot{{ThumbnailURL: "https://cdn.test/1s.jpg", FullURL: "https://cdn.test/1.jpg"}, {ThumbnailURL: "https://cdn.test/2s.jpg", FullURL: "https://cdn.test/2.jpg"}}},
		{SteamAppID: 20, Type: "game", Name: "Second Game", PriceCents: 4999, Genres: []string{"RPG"}, ReleaseDate: ""},
	}
	dataset := seed.Generate(games, 1)

	// What the database must contain afterwards, counted from the dataset itself.
	wantOffers, wantKeys := 0, 0
	for _, p := range dataset.Products {
		wantOffers += len(p.Offers)
		for _, o := range p.Offers {
			wantKeys += len(o.Keys)
		}
	}

	counts, err := seed.Load(ctx, pool, dataset, true)
	if err != nil {
		t.Fatalf("Load(reset) error = %v", err)
	}
	if counts.Products != len(dataset.Products) || counts.Offers != wantOffers || counts.Keys != wantKeys {
		t.Errorf("counts = %+v, want %d products, %d offers, %d keys", counts, len(dataset.Products), wantOffers, wantKeys)
	}

	rowCounts := map[string]int{
		"products":     len(dataset.Products),
		"offers":       wantOffers,
		"license_keys": wantKeys,
		"sellers":      len(dataset.Sellers),
		"genres":       2,
		"platforms":    len(dataset.Platforms),
		"regions":      len(dataset.Regions),
	}
	for table, want := range rowCounts {
		var got int
		// Table names cannot be query parameters; these come from the map above.
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if got != want {
			t.Errorf("%s has %d rows, want %d", table, got, want)
		}
	}

	t.Run("screenshots are stored per product, in order", func(t *testing.T) {
		var products, perProduct int
		var first string
		err := pool.QueryRow(ctx, `
			SELECT count(DISTINCT product_id), max(n), min(first_url)
			FROM (
				SELECT product_id, count(*) OVER (PARTITION BY product_id) AS n,
				       first_value(full_url) OVER (PARTITION BY product_id ORDER BY sort_order) AS first_url
				FROM product_screenshots
			) s`).Scan(&products, &perProduct, &first)
		if err != nil {
			t.Fatal(err)
		}
		// Every generated version of game 10 gets its two screenshots; game 20 has none.
		if products == 0 || perProduct != 2 || first != "https://cdn.test/1.jpg" {
			t.Errorf("products with screenshots = %d, per product = %d, first = %q", products, perProduct, first)
		}
	})

	t.Run("every key starts available", func(t *testing.T) {
		var notAvailable int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM license_keys WHERE status <> 'available'`).Scan(&notAvailable); err != nil {
			t.Fatal(err)
		}
		if notAvailable != 0 {
			t.Errorf("%d keys are not available right after seeding", notAvailable)
		}
	})

	t.Run("unknown release date is stored as NULL", func(t *testing.T) {
		var withDate, withoutDate int
		err := pool.QueryRow(ctx, `
			SELECT count(*) FILTER (WHERE release_date = DATE '2020-03-04'),
			       count(*) FILTER (WHERE release_date IS NULL)
			FROM products`).Scan(&withDate, &withoutDate)
		if err != nil {
			t.Fatal(err)
		}
		if withDate == 0 || withoutDate == 0 {
			t.Errorf("products with the date: %d, with NULL: %d; want both above zero", withDate, withoutDate)
		}
	})

	t.Run("refuses to load into a filled database without reset", func(t *testing.T) {
		_, err := seed.Load(ctx, pool, dataset, false)
		if !errors.Is(err, seed.ErrNotEmpty) {
			t.Errorf("error = %v, want ErrNotEmpty", err)
		}
	})

	t.Run("reset replaces the data instead of adding to it", func(t *testing.T) {
		if _, err := seed.Load(ctx, pool, dataset, true); err != nil {
			t.Fatalf("second Load(reset) error = %v", err)
		}
		var products int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM products`).Scan(&products); err != nil {
			t.Fatal(err)
		}
		if products != len(dataset.Products) {
			t.Errorf("%d products after a second load, want %d", products, len(dataset.Products))
		}
	})
}

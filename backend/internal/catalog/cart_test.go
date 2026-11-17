package catalog_test

import (
	"errors"
	"testing"

	"github.com/Jokubas-J/Eneba/backend/internal/catalog"
)

func TestValidateCart(t *testing.T) {
	service, ctx, pool := newServiceWithPool(t, productFixture())

	// Offer ids by seller name for the steam/global product of the fixture:
	//   A 900 sold out, B 1200 x3, C 1000 x1, D 1000 x5, E 500 (made inactive below)
	ids := map[string]int64{}
	rows, err := pool.Query(ctx, `
		SELECT s.name, o.id FROM offers o
		JOIN sellers s ON s.id = o.seller_id
		JOIN products p ON p.id = o.product_id
		WHERE p.slug = 'game-steam-global'`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var name string
		var id int64
		if err := rows.Scan(&name, &id); err != nil {
			t.Fatal(err)
		}
		ids[name] = id
	}
	if _, err := pool.Exec(ctx, `UPDATE offers SET is_active = false WHERE id = $1`, ids["Seller E"]); err != nil {
		t.Fatal(err)
	}

	cart, err := service.ValidateCart(ctx, []catalog.CartLine{
		{OfferID: ids["Seller B"], Quantity: 2}, // fine: 2 x 1200
		{OfferID: ids["Seller C"], Quantity: 2}, // only 1 key left
		{OfferID: ids["Seller A"], Quantity: 1}, // sold out
		{OfferID: ids["Seller E"], Quantity: 1}, // inactive
		{OfferID: 999999, Quantity: 1},          // never existed
		{OfferID: ids["Seller D"], Quantity: 1}, // fine: 1 x 1000
	})
	if err != nil {
		t.Fatalf("ValidateCart() error = %v", err)
	}

	wantProblems := []string{"", "not_enough_stock", "sold_out", "not_found", "not_found", ""}
	if len(cart.Items) != len(wantProblems) {
		t.Fatalf("got %d items, want %d", len(cart.Items), len(wantProblems))
	}
	for i, want := range wantProblems {
		if cart.Items[i].Problem != want {
			t.Errorf("line %d: problem = %q, want %q", i, cart.Items[i].Problem, want)
		}
	}

	t.Run("the total counts only lines that can be bought, at database prices", func(t *testing.T) {
		if cart.TotalCents != 2*1200+1000 || cart.ItemCount != 3 {
			t.Errorf("total = %d, count = %d; want 3400, 3", cart.TotalCents, cart.ItemCount)
		}
	})

	t.Run("lines carry product, seller, price and stock", func(t *testing.T) {
		b := cart.Items[0]
		if b.Product.Slug != "game-steam-global" || b.Seller.Name != "Seller B" || b.PriceCents != 1200 ||
			b.Stock != 3 || b.DiscountPercent != 40 || b.Product.Platform.Name != "Steam" {
			t.Errorf("line = %+v", b)
		}
		if c := cart.Items[1]; c.Stock != 1 || c.Quantity != 2 {
			t.Errorf("short line = %+v, want stock 1, quantity 2 kept as requested", c)
		}
	})

	t.Run("an empty cart", func(t *testing.T) {
		empty, err := service.ValidateCart(ctx, nil)
		if err != nil || empty.Items == nil || len(empty.Items) != 0 || empty.TotalCents != 0 {
			t.Errorf("ValidateCart(nil) = %+v, %v", empty, err)
		}
	})

	t.Run("invalid carts are refused", func(t *testing.T) {
		tooMany := make([]catalog.CartLine, catalog.MaxCartLines+1)
		for i := range tooMany {
			tooMany[i] = catalog.CartLine{OfferID: int64(i + 1), Quantity: 1}
		}
		for name, lines := range map[string][]catalog.CartLine{
			"quantity 0":    {{OfferID: ids["Seller B"], Quantity: 0}},
			"quantity 11":   {{OfferID: ids["Seller B"], Quantity: 11}},
			"same offer x2": {{OfferID: ids["Seller B"], Quantity: 1}, {OfferID: ids["Seller B"], Quantity: 1}},
			"too many":      tooMany,
		} {
			if _, err := service.ValidateCart(ctx, lines); !errors.Is(err, catalog.ErrInvalidFilter) {
				t.Errorf("%s: error = %v, want ErrInvalidFilter", name, err)
			}
		}
	})
}

package admin_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Jokubas-J/Eneba/backend/internal/admin"
	"github.com/Jokubas-J/Eneba/backend/internal/catalog"
	"github.com/Jokubas-J/Eneba/backend/internal/orders"
	"github.com/Jokubas-J/Eneba/backend/internal/seed"
	"github.com/Jokubas-J/Eneba/backend/internal/testdb"
)

func TestDeleteTestAccounts(t *testing.T) {
	pool := testdb.Open(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	keys := make([]string, 5)
	for i := range keys {
		keys[i] = fmt.Sprintf("ADMIN-KEY-%d", i)
	}
	dataset := seed.Dataset{
		Platforms: []seed.Named{{Slug: "steam", Name: "Steam"}},
		Regions:   []seed.Named{{Slug: "global", Name: "Global"}},
		Sellers:   []seed.Seller{{Name: "Seller", Rating: 9}},
		Products: []seed.Product{{Slug: "game", Name: "Game", Type: "game", PlatformSlug: "steam", RegionSlug: "global", SteamAppID: 1,
			Offers: []seed.Offer{{SellerName: "Seller", PriceCents: 500, OriginalPriceCents: 1000, Keys: keys}}}},
	}
	if _, err := pool.Exec(ctx, `TRUNCATE users, sessions RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	if _, err := seed.Load(ctx, pool, dataset, true); err != nil {
		t.Fatal(err)
	}
	var offerID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM offers`).Scan(&offerID); err != nil {
		t.Fatal(err)
	}

	// Two test accounts with a paid order (2 keys) and a pending one (1 key), and a
	// real customer with a paid order (1 key) who must not be touched.
	user := func(email string) int64 {
		var id int64
		if err := pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, 'x') RETURNING id`, email).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO sessions (user_id, token_hash, expires_at) VALUES ($1, $2, now() + interval '1 day')`,
			id, []byte(email)); err != nil {
			t.Fatal(err)
		}
		return id
	}
	service := orders.NewService(pool, orders.SimulatedProvider{BaseURL: "http://localhost"})
	buy := func(userID int64, quantity int, pay bool) {
		r, err := service.Checkout(ctx, userID, []catalog.CartLine{{OfferID: offerID, Quantity: quantity}})
		if err != nil {
			t.Fatal(err)
		}
		if pay {
			if _, err := service.PaySimulated(ctx, userID, r.OrderID); err != nil {
				t.Fatal(err)
			}
		}
	}
	cookie := user("cookie-check-1@example.com")
	stripe := user("stripe-live-check-2@example.com")
	customer := user("customer@example.com")
	buy(cookie, 2, true)
	buy(stripe, 1, false)
	buy(customer, 1, true)

	patterns := []string{"cookie-check-%@example.com", "stripe-live-check-%@example.com"}
	count := func(sql string) int {
		var n int
		if err := pool.QueryRow(ctx, sql).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	t.Run("refuses patterns that could match everyone", func(t *testing.T) {
		for _, bad := range []string{"%", "%@example.com", "nobody"} {
			if _, err := admin.DeleteTestAccounts(ctx, pool, []string{bad}, true); err == nil {
				t.Errorf("pattern %q accepted", bad)
			}
		}
	})

	t.Run("dry run reports and changes nothing", func(t *testing.T) {
		got, err := admin.DeleteTestAccounts(ctx, pool, patterns, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Emails) != 2 || got.Orders != 2 || got.KeysReturned != 3 {
			t.Errorf("report = %+v, want 2 accounts, 2 orders, 3 keys", got)
		}
		if n := count(`SELECT count(*) FROM users`); n != 3 {
			t.Errorf("%d users after a dry run, want 3", n)
		}
	})

	t.Run("apply deletes the test accounts and returns their keys", func(t *testing.T) {
		if _, err := admin.DeleteTestAccounts(ctx, pool, patterns, true); err != nil {
			t.Fatal(err)
		}
		if n := count(`SELECT count(*) FROM users`); n != 1 {
			t.Errorf("%d users left, want only the customer", n)
		}
		if n := count(`SELECT count(*) FROM sessions`); n != 1 {
			t.Errorf("%d sessions left, want 1", n)
		}
		if n := count(`SELECT count(*) FROM orders`); n != 1 {
			t.Errorf("%d orders left, want the customer's 1", n)
		}
		// 5 keys: the customer's 1 stays sold, the other 4 are on sale again.
		if n := count(`SELECT count(*) FROM license_keys WHERE status = 'available'`); n != 4 {
			t.Errorf("%d keys available, want 4", n)
		}
		var owner int64
		if err := pool.QueryRow(ctx, `
			SELECT o.user_id FROM license_keys k JOIN order_items i ON i.id = k.order_item_id
			JOIN orders o ON o.id = i.order_id WHERE k.status = 'sold'`).Scan(&owner); err != nil || owner != customer {
			t.Errorf("sold key belongs to %d (%v), want the customer %d", owner, err, customer)
		}
	})
}

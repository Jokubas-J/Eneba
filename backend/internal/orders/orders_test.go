package orders_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Jokubas-J/Eneba/backend/internal/catalog"
	"github.com/Jokubas-J/Eneba/backend/internal/orders"
	"github.com/Jokubas-J/Eneba/backend/internal/seed"
	"github.com/Jokubas-J/Eneba/backend/internal/testdb"
)

// fakeProvider stands in for Stripe: it hands out session ids, can be told to fail,
// and says "paid" for the sessions in paid.
type fakeProvider struct {
	mu   sync.Mutex
	fail bool
	n    int
	paid map[string]bool
	// payment references of the orders sessions were created for
	references []string
}

func (p *fakeProvider) Name() string { return "stripe" }

func (p *fakeProvider) CreateSession(ctx context.Context, order orders.Order) (string, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.references = append(p.references, order.PaymentReference)
	if p.fail {
		return "", "", errors.New("stripe is down")
	}
	p.n++
	return fmt.Sprintf("cs_test_%d", p.n), "https://checkout.stripe.test/pay", nil
}

func (p *fakeProvider) IsPaid(ctx context.Context, sessionID string) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.paid[sessionID], nil
}

// The fixture: one product with two offers.
//
//	"last"  1 key,  price 2500   (for the race)
//	"many"  5 keys, price 1000
type fixture struct {
	service *orders.Service
	pool    *pgxpool.Pool
	ctx     context.Context
	last    int64 // offer ids
	many    int64
	users   []int64
}

func setup(t *testing.T, provider orders.PaymentProvider, userCount int) fixture {
	t.Helper()
	pool := testdb.Open(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	keys := func(prefix string, n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf("TEST-%s-%d", prefix, i)
		}
		return out
	}
	dataset := seed.Dataset{
		Platforms: []seed.Named{{Slug: "steam", Name: "Steam"}},
		Regions:   []seed.Named{{Slug: "global", Name: "Global"}},
		Sellers:   []seed.Seller{{Name: "Seller Last", Rating: 9}, {Name: "Seller Many", Rating: 9}},
		Products: []seed.Product{{
			Slug: "game", Name: "Game (PC) Steam Key GLOBAL", Type: "game", PlatformSlug: "steam", RegionSlug: "global", SteamAppID: 1,
			Offers: []seed.Offer{
				{SellerName: "Seller Last", PriceCents: 2500, OriginalPriceCents: 3000, Keys: keys("last", 1)},
				{SellerName: "Seller Many", PriceCents: 1000, OriginalPriceCents: 3000, Keys: keys("many", 5)},
			},
		}},
	}
	// Users first: TRUNCATE ... CASCADE also empties every table that refers to
	// users, which reaches orders, order items and, through them, license_keys. Done
	// after loading the catalog, it would delete the keys again.
	if _, err := pool.Exec(ctx, `TRUNCATE users, sessions RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	// Load -reset also empties orders, order items and payment events.
	if _, err := seed.Load(ctx, pool, dataset, true); err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	f := fixture{service: orders.NewService(pool, provider), pool: pool, ctx: ctx}
	if err := pool.QueryRow(ctx, `SELECT id FROM offers WHERE price_cents = 2500`).Scan(&f.last); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM offers WHERE price_cents = 1000`).Scan(&f.many); err != nil {
		t.Fatal(err)
	}
	for i := range userCount {
		var id int64
		err := pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, 'not used') RETURNING id`,
			fmt.Sprintf("buyer%d@example.com", i)).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		f.users = append(f.users, id)
	}
	return f
}

// keyCounts returns how many keys of an offer are in each status.
func (f fixture) keyCounts(t *testing.T, offerID int64) map[string]int {
	t.Helper()
	rows, err := f.pool.Query(f.ctx, `SELECT status, count(*) FROM license_keys WHERE offer_id = $1 GROUP BY status`, offerID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			t.Fatal(err)
		}
		counts[status] = n
	}
	return counts
}

func (f fixture) sessionOf(t *testing.T, orderID int64) string {
	t.Helper()
	var session string
	if err := f.pool.QueryRow(f.ctx, `SELECT provider_session_id FROM orders WHERE id = $1`, orderID).Scan(&session); err != nil {
		t.Fatal(err)
	}
	return session
}

func (f fixture) completed(eventID, session string) orders.PaymentEvent {
	return orders.PaymentEvent{Provider: "stripe", ID: eventID, Type: "checkout.session.completed",
		SessionID: session, Paid: true, Payload: json.RawMessage(`{}`)}
}

func TestRaceForTheLastKey(t *testing.T) {
	const buyers = 20
	f := setup(t, &fakeProvider{}, buyers)

	// 20 buyers press "checkout" for the single key at the same moment.
	var wg sync.WaitGroup
	results := make([]error, buyers)
	start := make(chan struct{})
	for i := range buyers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // release all goroutines together
			_, results[i] = f.service.Checkout(f.ctx, f.users[i], []catalog.CartLine{{OfferID: f.last, Quantity: 1}})
		}()
	}
	close(start)
	wg.Wait()

	succeeded, outOfStock := 0, 0
	for _, err := range results {
		var stockErr *orders.OutOfStockError
		switch {
		case err == nil:
			succeeded++
		case errors.As(err, &stockErr):
			outOfStock++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if succeeded != 1 || outOfStock != buyers-1 {
		t.Errorf("succeeded = %d, out of stock = %d; want 1 and %d", succeeded, outOfStock, buyers-1)
	}

	if got := f.keyCounts(t, f.last); got["reserved"] != 1 || got["available"] != 0 {
		t.Errorf("keys = %v, want exactly one reserved", got)
	}
	var orderCount int
	if err := f.pool.QueryRow(f.ctx, `SELECT count(*) FROM orders`).Scan(&orderCount); err != nil {
		t.Fatal(err)
	}
	if orderCount != 1 {
		t.Errorf("%d orders exist, want 1 (the failed checkouts must leave nothing behind)", orderCount)
	}
}

func TestDuplicateWebhook(t *testing.T) {
	f := setup(t, &fakeProvider{}, 1)
	result, err := f.service.Checkout(f.ctx, f.users[0], []catalog.CartLine{{OfferID: f.many, Quantity: 2}})
	if err != nil {
		t.Fatal(err)
	}
	event := f.completed("evt_1", f.sessionOf(t, result.OrderID))

	// Stripe delivers the same event twice.
	for range 2 {
		if err := f.service.HandlePaymentEvent(f.ctx, event); err != nil {
			t.Fatalf("HandlePaymentEvent: %v", err)
		}
	}

	var events int
	if err := f.pool.QueryRow(f.ctx, `SELECT count(*) FROM payment_events WHERE event_id = 'evt_1'`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Errorf("%d payment_events rows for the event, want 1", events)
	}
	if got := f.keyCounts(t, f.many); got["sold"] != 2 || got["available"] != 3 || got["reserved"] != 0 {
		t.Errorf("keys = %v, want 2 sold and 3 available", got)
	}
	order, err := f.service.Get(f.ctx, f.users[0], result.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != orders.StatusPaid || len(order.Items) != 1 || len(order.Items[0].Keys) != 2 {
		t.Errorf("order = %+v, want paid with 2 keys", order)
	}
}

func TestConcurrentFulfilment(t *testing.T) {
	provider := &fakeProvider{paid: map[string]bool{}}
	f := setup(t, provider, 1)
	result, err := f.service.Checkout(f.ctx, f.users[0], []catalog.CartLine{{OfferID: f.many, Quantity: 2}})
	if err != nil {
		t.Fatal(err)
	}
	session := f.sessionOf(t, result.OrderID)
	provider.mu.Lock()
	provider.paid[session] = true
	provider.mu.Unlock()

	// Five webhook deliveries (different event ids, as for completed +
	// async_payment_succeeded) and five success-page confirmations at once.
	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := range 5 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			errs <- f.service.HandlePaymentEvent(f.ctx, f.completed(fmt.Sprintf("evt_%d", i), session))
		}()
		go func() {
			defer wg.Done()
			_, err := f.service.Confirm(f.ctx, f.users[0], result.OrderID)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("error: %v", err)
		}
	}
	if got := f.keyCounts(t, f.many); got["sold"] != 2 || got["available"] != 3 {
		t.Errorf("keys = %v, want exactly 2 sold", got)
	}
}

func TestExpiryReleasesKeys(t *testing.T) {
	t.Run("the expired webhook returns the keys", func(t *testing.T) {
		f := setup(t, &fakeProvider{}, 1)
		result, err := f.service.Checkout(f.ctx, f.users[0], []catalog.CartLine{{OfferID: f.many, Quantity: 2}})
		if err != nil {
			t.Fatal(err)
		}
		if got := f.keyCounts(t, f.many); got["reserved"] != 2 || got["available"] != 3 {
			t.Fatalf("after checkout keys = %v, want 2 reserved", got)
		}
		err = f.service.HandlePaymentEvent(f.ctx, orders.PaymentEvent{Provider: "stripe", ID: "evt_exp",
			Type: "checkout.session.expired", SessionID: f.sessionOf(t, result.OrderID), Payload: json.RawMessage(`{}`)})
		if err != nil {
			t.Fatal(err)
		}
		if got := f.keyCounts(t, f.many); got["available"] != 5 {
			t.Errorf("after expiry keys = %v, want all 5 available", got)
		}
		order, _ := f.service.Get(f.ctx, f.users[0], result.OrderID)
		if order.Status != orders.StatusExpired {
			t.Errorf("status = %s, want expired", order.Status)
		}
	})

	t.Run("the clean-up job releases only orders past the grace period", func(t *testing.T) {
		f := setup(t, &fakeProvider{}, 1)
		old, err := f.service.Checkout(f.ctx, f.users[0], []catalog.CartLine{{OfferID: f.many, Quantity: 2}})
		if err != nil {
			t.Fatal(err)
		}
		recent, err := f.service.Checkout(f.ctx, f.users[0], []catalog.CartLine{{OfferID: f.last, Quantity: 1}})
		if err != nil {
			t.Fatal(err)
		}
		// Time travel: one order expired 10 minutes ago, the other 1 minute ago
		// (still inside the 5-minute grace for a late "paid" webhook).
		_, err = f.pool.Exec(f.ctx, `
			UPDATE orders SET expires_at = now() - CASE id WHEN $1 THEN interval '10 minutes' ELSE interval '1 minute' END
			WHERE id IN ($1, $2)`, old.OrderID, recent.OrderID)
		if err != nil {
			t.Fatal(err)
		}

		released, err := f.service.ReleaseExpired(f.ctx)
		if err != nil || released != 1 {
			t.Fatalf("ReleaseExpired() = %d, %v; want 1", released, err)
		}
		if got := f.keyCounts(t, f.many); got["available"] != 5 {
			t.Errorf("old order's keys = %v, want released", got)
		}
		if got := f.keyCounts(t, f.last); got["reserved"] != 1 {
			t.Errorf("recent order's keys = %v, want still reserved", got)
		}
	})
}

func TestExpiredOrderCannotBePaid(t *testing.T) {
	f := setup(t, &fakeProvider{}, 1)
	result, err := f.service.Checkout(f.ctx, f.users[0], []catalog.CartLine{{OfferID: f.many, Quantity: 1}})
	if err != nil {
		t.Fatal(err)
	}
	session := f.sessionOf(t, result.OrderID)
	expired := orders.PaymentEvent{Provider: "stripe", ID: "evt_a", Type: "checkout.session.expired", SessionID: session, Payload: json.RawMessage(`{}`)}
	if err := f.service.HandlePaymentEvent(f.ctx, expired); err != nil {
		t.Fatal(err)
	}
	// A "paid" event arriving afterwards must not deliver anything.
	if err := f.service.HandlePaymentEvent(f.ctx, f.completed("evt_b", session)); err != nil {
		t.Fatalf("HandlePaymentEvent: %v", err)
	}
	order, _ := f.service.Get(f.ctx, f.users[0], result.OrderID)
	if order.Status != orders.StatusExpired || len(order.Items[0].Keys) != 0 {
		t.Errorf("order = %+v, want expired with no keys", order)
	}
	if got := f.keyCounts(t, f.many); got["sold"] != 0 || got["available"] != 5 {
		t.Errorf("keys = %v, want none sold", got)
	}
}

func TestCheckoutIgnoresPricesFromTheBrowser(t *testing.T) {
	f := setup(t, &fakeProvider{}, 1)

	// What a modified browser could send: a price of 1 cent for everything. The
	// request type (the same shape as the HTTP handler's) has no price field, so the
	// decoder drops it.
	body := fmt.Sprintf(`{"items":[{"offerId":%d,"quantity":2,"priceCents":1,"unitPriceCents":1}],"totalCents":2}`, f.many)
	var request struct {
		Items []catalog.CartLine `json:"items"`
	}
	if err := json.Unmarshal([]byte(body), &request); err != nil {
		t.Fatal(err)
	}

	result, err := f.service.Checkout(f.ctx, f.users[0], request.Items)
	if err != nil {
		t.Fatal(err)
	}
	order, err := f.service.Get(f.ctx, f.users[0], result.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if order.TotalCents != 2000 || order.Items[0].UnitPriceCents != 1000 {
		t.Errorf("total = %d, unit price = %d; want 2000 and 1000 from the database", order.TotalCents, order.Items[0].UnitPriceCents)
	}
}

func TestOrdersAreOnlyVisibleToTheirOwner(t *testing.T) {
	f := setup(t, orders.SimulatedProvider{BaseURL: "http://localhost"}, 2)
	owner, stranger := f.users[0], f.users[1]
	result, err := f.service.Checkout(f.ctx, owner, []catalog.CartLine{{OfferID: f.many, Quantity: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(result.RedirectURL, fmt.Sprintf("/checkout/simulated/%d", result.OrderID)) {
		t.Errorf("redirect = %q, want the simulated payment page", result.RedirectURL)
	}

	t.Run("before payment there are no keys", func(t *testing.T) {
		order, err := f.service.Get(f.ctx, owner, result.OrderID)
		if err != nil || order.Status != orders.StatusPending || len(order.Items[0].Keys) != 0 {
			t.Errorf("order = %+v, %v", order, err)
		}
	})

	t.Run("another user gets not found everywhere", func(t *testing.T) {
		if _, err := f.service.Get(f.ctx, stranger, result.OrderID); !errors.Is(err, orders.ErrNotFound) {
			t.Errorf("Get: %v", err)
		}
		if _, err := f.service.Confirm(f.ctx, stranger, result.OrderID); !errors.Is(err, orders.ErrNotFound) {
			t.Errorf("Confirm: %v", err)
		}
		if _, err := f.service.PaySimulated(f.ctx, stranger, result.OrderID); !errors.Is(err, orders.ErrNotFound) {
			t.Errorf("PaySimulated: %v", err)
		}
		list, err := f.service.List(f.ctx, stranger)
		if err != nil || len(list) != 0 {
			t.Errorf("stranger's list = %v, %v", list, err)
		}
	})

	t.Run("the owner pays and sees the key", func(t *testing.T) {
		order, err := f.service.PaySimulated(f.ctx, owner, result.OrderID)
		if err != nil {
			t.Fatal(err)
		}
		if order.Status != orders.StatusPaid || len(order.Items[0].Keys) != 1 || !strings.HasPrefix(order.Items[0].Keys[0], "TEST-many-") {
			t.Errorf("order = %+v, want paid with one key", order)
		}
		list, _ := f.service.List(f.ctx, owner)
		if len(list) != 1 || list[0].Status != orders.StatusPaid {
			t.Errorf("history = %+v", list)
		}
	})
}

func TestProviderFailureReleasesKeys(t *testing.T) {
	f := setup(t, &fakeProvider{fail: true}, 1)
	if _, err := f.service.Checkout(f.ctx, f.users[0], []catalog.CartLine{{OfferID: f.many, Quantity: 3}}); err == nil {
		t.Fatal("Checkout succeeded although the provider failed")
	}
	if got := f.keyCounts(t, f.many); got["available"] != 5 {
		t.Errorf("keys = %v, want all 5 available again", got)
	}
}

func TestCheckoutRejectsBadCarts(t *testing.T) {
	f := setup(t, &fakeProvider{}, 1)
	tests := map[string]struct {
		lines []catalog.CartLine
		want  error
	}{
		"empty":         {nil, orders.ErrEmptyCart},
		"quantity 0":    {[]catalog.CartLine{{OfferID: f.many, Quantity: 0}}, orders.ErrInvalidCart},
		"same offer x2": {[]catalog.CartLine{{OfferID: f.many, Quantity: 1}, {OfferID: f.many, Quantity: 1}}, orders.ErrInvalidCart},
		"unknown offer": {[]catalog.CartLine{{OfferID: 999999, Quantity: 1}}, orders.ErrOfferNotFound},
	}
	for name, tt := range tests {
		if _, err := f.service.Checkout(f.ctx, f.users[0], tt.lines); !errors.Is(err, tt.want) {
			t.Errorf("%s: error = %v, want %v", name, err, tt.want)
		}
	}
	var stockErr *orders.OutOfStockError
	_, err := f.service.Checkout(f.ctx, f.users[0], []catalog.CartLine{{OfferID: f.many, Quantity: 6}})
	if !errors.As(err, &stockErr) || stockErr.Available != 5 {
		t.Errorf("6 of 5 keys: error = %v, want out of stock with 5 available", err)
	}
	if got := f.keyCounts(t, f.many); got["available"] != 5 {
		t.Errorf("keys = %v, want untouched", got)
	}
}

// Every order gets its own payment reference from the database (the idempotency key
// for Stripe is built from it), and it is a UUID, not the order id.
func TestEveryOrderGetsAUniquePaymentReference(t *testing.T) {
	provider := &fakeProvider{}
	f := setup(t, provider, 1)
	for range 3 {
		if _, err := f.service.Checkout(f.ctx, f.users[0], []catalog.CartLine{{OfferID: f.many, Quantity: 1}}); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	for _, ref := range provider.references {
		if len(ref) != 36 || strings.Count(ref, "-") != 4 {
			t.Errorf("reference %q is not a UUID", ref)
		}
		if seen[ref] {
			t.Errorf("reference %q used twice", ref)
		}
		seen[ref] = true
	}
	if len(seen) != 3 {
		t.Errorf("%d distinct references, want 3", len(seen))
	}
}

func TestPendingOrderLimits(t *testing.T) {
	t.Run("at most MaxPendingOrders unpaid orders", func(t *testing.T) {
		f := setup(t, &fakeProvider{}, 2)
		for i := range orders.MaxPendingOrders {
			if _, err := f.service.Checkout(f.ctx, f.users[0], []catalog.CartLine{{OfferID: f.many, Quantity: 1}}); err != nil {
				t.Fatalf("checkout %d: %v", i+1, err)
			}
		}
		_, err := f.service.Checkout(f.ctx, f.users[0], []catalog.CartLine{{OfferID: f.last, Quantity: 1}})
		if !errors.Is(err, orders.ErrTooManyPending) {
			t.Fatalf("checkout %d: error = %v, want ErrTooManyPending", orders.MaxPendingOrders+1, err)
		}
		// The refused checkout reserved nothing, and another user is not affected.
		if got := f.keyCounts(t, f.last); got["available"] != 1 {
			t.Errorf("last key = %v, want still available", got)
		}
		if _, err := f.service.Checkout(f.ctx, f.users[1], []catalog.CartLine{{OfferID: f.last, Quantity: 1}}); err != nil {
			t.Errorf("another user: %v", err)
		}
	})

	t.Run("paying or expiry frees a place", func(t *testing.T) {
		f := setup(t, &fakeProvider{}, 1)
		var ids []int64
		for range orders.MaxPendingOrders {
			r, err := f.service.Checkout(f.ctx, f.users[0], []catalog.CartLine{{OfferID: f.many, Quantity: 1}})
			if err != nil {
				t.Fatal(err)
			}
			ids = append(ids, r.OrderID)
		}
		// One order runs out of time (its clean-up has not run yet): it no longer counts.
		if _, err := f.pool.Exec(f.ctx, `UPDATE orders SET expires_at = now() - interval '1 minute' WHERE id = $1`, ids[0]); err != nil {
			t.Fatal(err)
		}
		if _, err := f.service.Checkout(f.ctx, f.users[0], []catalog.CartLine{{OfferID: f.many, Quantity: 1}}); err != nil {
			t.Errorf("after one order expired: %v", err)
		}
	})

	t.Run("at most MaxReservedKeys keys held", func(t *testing.T) {
		f := setup(t, &fakeProvider{}, 1)
		// 25 more keys on the "many" offer, enough to reach the limit.
		bigOffer := f.many
		_, err := f.pool.Exec(f.ctx, `
			INSERT INTO license_keys (offer_id, key_value)
			SELECT $1, 'BIG-' || n FROM generate_series(1, 25) n`, bigOffer)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.service.Checkout(f.ctx, f.users[0], []catalog.CartLine{{OfferID: bigOffer, Quantity: 10}}); err != nil {
			t.Fatal(err)
		}
		if _, err := f.service.Checkout(f.ctx, f.users[0], []catalog.CartLine{{OfferID: bigOffer, Quantity: 10}}); err != nil {
			t.Fatalf("20 keys in total is allowed: %v", err)
		}
		if _, err := f.service.Checkout(f.ctx, f.users[0], []catalog.CartLine{{OfferID: f.many, Quantity: 1}}); !errors.Is(err, orders.ErrTooManyPending) {
			t.Errorf("21st key: error = %v, want ErrTooManyPending", err)
		}
	})

	t.Run("simultaneous checkouts of one user cannot pass the limit together", func(t *testing.T) {
		f := setup(t, &fakeProvider{}, 1)
		const attempts = 8
		var wg sync.WaitGroup
		errs := make([]error, attempts)
		start := make(chan struct{})
		for i := range attempts {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, errs[i] = f.service.Checkout(f.ctx, f.users[0], []catalog.CartLine{{OfferID: f.many, Quantity: 1}})
			}()
		}
		close(start)
		wg.Wait()
		ok := 0
		for _, err := range errs {
			if err == nil {
				ok++
			} else if !errors.Is(err, orders.ErrTooManyPending) {
				t.Errorf("unexpected error: %v", err)
			}
		}
		if ok != orders.MaxPendingOrders {
			t.Errorf("%d checkouts went through, want exactly %d", ok, orders.MaxPendingOrders)
		}
	})
}

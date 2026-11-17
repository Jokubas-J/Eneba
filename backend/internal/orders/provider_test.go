package orders

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v87"
)

// fakeStripe imitates the one part of Stripe this test is about: idempotency keys.
// Like the real API, a key seen before with the SAME parameters returns the first
// response again; with DIFFERENT parameters it is refused with idempotency_error.
// Keys live per Stripe account, and dev and production share one test-mode account,
// so both environments' requests go to the same fakeStripe.
type fakeStripe struct {
	mu       sync.Mutex
	bodies   map[string]string // idempotency key -> request body
	sessions map[string]string // idempotency key -> session id returned
	forms    []url.Values      // every accepted request, for checking parameters
}

func newFakeStripe(t *testing.T) (*fakeStripe, *httptest.Server) {
	fake := &fakeStripe{bodies: map[string]string{}, sessions: map[string]string{}}
	server := httptest.NewServer(http.HandlerFunc(fake.serve))
	t.Cleanup(server.Close)
	return fake, server
}

func (f *fakeStripe) serve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/v1/checkout/sessions" {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	body := r.PostForm.Encode() // sorted, so equal parameters give an equal string
	key := r.Header.Get("Idempotency-Key")

	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if previous, seen := f.bodies[key]; seen {
		if previous != body {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":{"type":"idempotency_error","message":"Keys for idempotent requests can only be used with the same parameters they were first used with. Try using a key other than '%s'"}}`, key)
			return
		}
		fmt.Fprintf(w, `{"id":%q,"object":"checkout.session","url":"https://checkout.stripe.test/%s"}`, f.sessions[key], f.sessions[key])
		return
	}
	id := fmt.Sprintf("cs_test_%d", len(f.sessions)+1)
	f.bodies[key] = body
	f.sessions[key] = id
	f.forms = append(f.forms, r.PostForm)
	fmt.Fprintf(w, `{"id":%q,"object":"checkout.session","url":"https://checkout.stripe.test/%s"}`, id, id)
}

// providerFor builds a StripeProvider of one environment that talks to the fake.
func providerFor(server *httptest.Server, environment, baseURL string) *StripeProvider {
	backend := stripe.GetBackendWithConfig(stripe.APIBackend, &stripe.BackendConfig{
		URL:               stripe.String(server.URL),
		MaxNetworkRetries: stripe.Int64(0),
		LeveledLogger:     &stripe.LeveledLogger{Level: stripe.LevelNull},
	})
	client := stripe.NewClient("sk_test_fake", stripe.WithBackends(&stripe.Backends{API: backend, Connect: backend, Uploads: backend}))
	return newStripeProvider(client, baseURL, environment)
}

// testOrder is an order as the database would return it right after checkout:
// the id is a per-database sequence number, the payment reference is random.
func testOrder(id int64, productName string, priceCents int) Order {
	return Order{
		ID:               id,
		PaymentReference: rand.Text(), // the service gets a UUID from the database
		ExpiresAt:        time.Now().Add(ReservationTime),
		Items: []OrderItem{{
			ProductName: productName, SellerName: "Seller", UnitPriceCents: priceCents, Quantity: 1,
		}},
	}
}

// The bug found on the live site: the development database and production both had
// an order with id 2, both asked the shared Stripe account for a session, and
// production's request was refused because the idempotency key was built from the
// order id alone.
func TestStripeSessionsFromTwoDatabasesDoNotCollide(t *testing.T) {
	fake, server := newFakeStripe(t)
	development := providerFor(server, "development", "http://localhost:5173")
	production := providerFor(server, "production", "https://shop.example")

	// Same order id, different contents: exactly what two databases produce.
	if _, _, err := development.CreateSession(t.Context(), testOrder(2, "Among Us", 179)); err != nil {
		t.Fatalf("development: %v", err)
	}
	if _, _, err := production.CreateSession(t.Context(), testOrder(2, "Elden Ring", 3999)); err != nil {
		t.Fatalf("production order 2 was refused: %v", err)
	}
	if len(fake.forms) != 2 {
		t.Errorf("Stripe created %d sessions, want 2", len(fake.forms))
	}
}

// A retry for the SAME order must still be recognised as a retry, so a network
// hiccup can never create two payment pages for one order.
func TestStripeSessionRetryForTheSameOrderIsIdempotent(t *testing.T) {
	fake, server := newFakeStripe(t)
	provider := providerFor(server, "production", "https://shop.example")
	order := testOrder(7, "Elden Ring", 3999)

	first, _, err := provider.CreateSession(t.Context(), order)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := provider.CreateSession(t.Context(), order)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || len(fake.forms) != 1 {
		t.Errorf("sessions %q and %q, %d created; want the same session once", first, second, len(fake.forms))
	}
}

// The webhook tells environments apart by this metadata.
func TestStripeSessionCarriesTheEnvironment(t *testing.T) {
	fake, server := newFakeStripe(t)
	provider := providerFor(server, "production", "https://shop.example")
	order := testOrder(3, "Cyberpunk 2077", 2999)
	if _, _, err := provider.CreateSession(t.Context(), order); err != nil {
		t.Fatal(err)
	}
	form := fake.forms[0]
	if got := form.Get("metadata[environment]"); got != "production" {
		t.Errorf("metadata[environment] = %q, want production", got)
	}
	if got := form.Get("metadata[order_id]"); got != "3" {
		t.Errorf("metadata[order_id] = %q, want 3", got)
	}
	if got := form.Get("metadata[payment_reference]"); got != order.PaymentReference {
		t.Errorf("metadata[payment_reference] = %q, want %q", got, order.PaymentReference)
	}
}

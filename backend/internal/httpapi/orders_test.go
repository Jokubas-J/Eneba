package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v87/webhook"

	"github.com/Jokubas-J/Eneba/backend/internal/catalog"
	"github.com/Jokubas-J/Eneba/backend/internal/orders"
)

// fakeOrders records what the handlers pass on.
type fakeOrders struct {
	lines  []catalog.CartLine
	events []orders.PaymentEvent
}

func (f *fakeOrders) Checkout(ctx context.Context, userID int64, lines []catalog.CartLine) (orders.CheckoutResult, error) {
	f.lines = lines
	return orders.CheckoutResult{OrderID: 1, RedirectURL: "https://checkout.stripe.test"}, nil
}
func (f *fakeOrders) Get(ctx context.Context, userID, orderID int64) (orders.Order, error) {
	return orders.Order{}, orders.ErrNotFound
}
func (f *fakeOrders) List(ctx context.Context, userID int64) ([]orders.Order, error) { return nil, nil }
func (f *fakeOrders) Confirm(ctx context.Context, userID, orderID int64) (orders.Order, error) {
	return orders.Order{}, orders.ErrNotFound
}
func (f *fakeOrders) PaySimulated(ctx context.Context, userID, orderID int64) (orders.Order, error) {
	return orders.Order{}, orders.ErrNotFound
}
func (f *fakeOrders) HandlePaymentEvent(ctx context.Context, event orders.PaymentEvent) error {
	f.events = append(f.events, event)
	return nil
}

const testWebhookSecret = "whsec_test_secret"

func webhookRequest(t *testing.T, fake *fakeOrders, secret, body, signatureHeader string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/stripe", strings.NewReader(body))
	req.Header.Set("Stripe-Signature", signatureHeader)
	rec := httptest.NewRecorder()
	NewRouter(Dependencies{Orders: fake, Accounts: &fakeAccounts{}, StripeWebhookSecret: secret, AllowedOrigin: testOrigin}).ServeHTTP(rec, req)
	return rec
}

// sign produces the Stripe-Signature header for body at time ts, as Stripe would.
func sign(body string, secret string, ts time.Time) string {
	return webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: []byte(body), Secret: secret, Timestamp: ts}).Header
}

const completedEvent = `{"id":"evt_123","object":"event","type":"checkout.session.completed",
	"data":{"object":{"id":"cs_test_abc","object":"checkout.session","payment_status":"paid"}}}`

func TestStripeWebhookSignature(t *testing.T) {
	t.Run("a correctly signed event is passed on", func(t *testing.T) {
		fake := &fakeOrders{}
		rec := webhookRequest(t, fake, testWebhookSecret, completedEvent, sign(completedEvent, testWebhookSecret, time.Now()))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
		}
		if len(fake.events) != 1 {
			t.Fatalf("%d events passed on, want 1", len(fake.events))
		}
		e := fake.events[0]
		if e.ID != "evt_123" || e.SessionID != "cs_test_abc" || !e.Paid || e.Type != "checkout.session.completed" || e.Provider != "stripe" {
			t.Errorf("event = %+v", e)
		}
	})

	refused := []struct {
		name, secret, header string
		want                 int
	}{
		{"signed with another secret", testWebhookSecret, sign(completedEvent, "whsec_attacker", time.Now()), 400},
		{"no signature", testWebhookSecret, "", 400},
		{"older than 5 minutes (replayed)", testWebhookSecret, sign(completedEvent, testWebhookSecret, time.Now().Add(-10*time.Minute)), 400},
		{"webhooks not configured", "", sign(completedEvent, testWebhookSecret, time.Now()), 503},
	}
	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeOrders{}
			rec := webhookRequest(t, fake, tt.secret, completedEvent, tt.header)
			if rec.Code != tt.want || len(fake.events) != 0 {
				t.Errorf("status = %d, events = %d; want %d and none", rec.Code, len(fake.events), tt.want)
			}
		})
	}

	t.Run("a body changed after signing is refused", func(t *testing.T) {
		fake := &fakeOrders{}
		header := sign(completedEvent, testWebhookSecret, time.Now())
		tampered := strings.Replace(completedEvent, "cs_test_abc", "cs_test_other", 1)
		if rec := webhookRequest(t, fake, testWebhookSecret, tampered, header); rec.Code != 400 || len(fake.events) != 0 {
			t.Errorf("status = %d, events = %d", rec.Code, len(fake.events))
		}
	})

	t.Run("other event types are acknowledged and ignored", func(t *testing.T) {
		fake := &fakeOrders{}
		body := `{"id":"evt_9","object":"event","type":"customer.created","data":{"object":{}}}`
		if rec := webhookRequest(t, fake, testWebhookSecret, body, sign(body, testWebhookSecret, time.Now())); rec.Code != 200 || len(fake.events) != 0 {
			t.Errorf("status = %d, events = %d", rec.Code, len(fake.events))
		}
	})
}

func TestCheckoutHandler(t *testing.T) {
	request := func(fake *fakeOrders, body, cookie string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/checkout", strings.NewReader(body))
		req.Header.Set("Origin", testOrigin)
		if cookie != "" {
			req.Header.Set("Cookie", sessionCookie+"="+cookie)
		}
		rec := httptest.NewRecorder()
		NewRouter(Dependencies{Orders: fake, Accounts: &fakeAccounts{}, AllowedOrigin: testOrigin}).ServeHTTP(rec, req)
		return rec
	}

	t.Run("signing in is required", func(t *testing.T) {
		fake := &fakeOrders{}
		if rec := request(fake, `{"items":[{"offerId":1,"quantity":1}]}`, ""); rec.Code != http.StatusUnauthorized || fake.lines != nil {
			t.Errorf("status = %d, lines = %v; want 401 and the service not called", rec.Code, fake.lines)
		}
	})

	t.Run("prices sent by the browser never reach the service", func(t *testing.T) {
		fake := &fakeOrders{}
		rec := request(fake, `{"items":[{"offerId":7,"quantity":2,"priceCents":1}],"totalCents":2}`, "valid-token")
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
		}
		// catalog.CartLine has only offerId and quantity: there is nowhere to put a price.
		if len(fake.lines) != 1 || fake.lines[0] != (catalog.CartLine{OfferID: 7, Quantity: 2}) {
			t.Errorf("lines = %+v", fake.lines)
		}
	})

	t.Run("another user's or a missing order is 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/orders/5", nil)
		req.Header.Set("Cookie", sessionCookie+"=valid-token")
		rec := httptest.NewRecorder()
		NewRouter(Dependencies{Orders: &fakeOrders{}, Accounts: &fakeAccounts{}, AllowedOrigin: testOrigin}).ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})
}

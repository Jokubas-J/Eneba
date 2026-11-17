package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/stripe/stripe-go/v87"
	"github.com/stripe/stripe-go/v87/webhook"

	"github.com/Jokubas-J/Eneba/backend/internal/auth"
	"github.com/Jokubas-J/Eneba/backend/internal/catalog"
	"github.com/Jokubas-J/Eneba/backend/internal/orders"
)

// Orders is what the HTTP layer needs from the order service.
type Orders interface {
	Checkout(ctx context.Context, userID int64, lines []catalog.CartLine) (orders.CheckoutResult, error)
	Get(ctx context.Context, userID, orderID int64) (orders.Order, error)
	List(ctx context.Context, userID int64) ([]orders.Order, error)
	Confirm(ctx context.Context, userID, orderID int64) (orders.Order, error)
	PaySimulated(ctx context.Context, userID, orderID int64) (orders.Order, error)
	HandlePaymentEvent(ctx context.Context, event orders.PaymentEvent) error
}

type orderHandlers struct {
	orders        Orders
	accounts      authHandlers
	webhookSecret string // STRIPE_WEBHOOK_SECRET; empty = webhooks refused
}

// requireUser returns the signed-in user, or writes 401 and returns false.
func (h orderHandlers) requireUser(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	user, ok, err := h.accounts.currentUser(r)
	if err != nil {
		writeInternalError(w, r, err)
		return auth.User{}, false
	}
	if !ok {
		writeError(w, http.StatusUnauthorized, "not_signed_in", "Please log in first.")
		return auth.User{}, false
	}
	return user, true
}

// checkoutRequest is the body of POST /api/checkout: the cart, as offer ids and
// quantities. There is deliberately no price field. Go's JSON decoder ignores
// fields it does not know, so a "priceCents" sent by a modified browser is dropped
// before the service ever sees the request.
type checkoutRequest struct {
	Items []catalog.CartLine `json:"items"`
}

func (h orderHandlers) checkout(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	var body checkoutRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	result, err := h.orders.Checkout(r.Context(), user.ID, body.Items)
	var outOfStock *orders.OutOfStockError
	switch {
	case errors.As(err, &outOfStock):
		writeError(w, http.StatusConflict, "out_of_stock",
			"Not enough keys left for "+outOfStock.ProductName+" ("+strconv.Itoa(outOfStock.Available)+" available).")
		return
	case errors.Is(err, orders.ErrOfferNotFound):
		writeError(w, http.StatusConflict, "offer_unavailable", "An offer in your cart is no longer available.")
		return
	case errors.Is(err, orders.ErrEmptyCart), errors.Is(err, orders.ErrInvalidCart):
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	case err != nil:
		// Mostly "the payment provider could not be reached"; the service has already
		// released the reserved keys.
		log.Printf("checkout for user %d: %v", user.ID, err)
		writeError(w, http.StatusBadGateway, "payment_unavailable", "The payment could not be started. Please try again.")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// orderID reads {id} from the path. Anything that is not a positive number is
// treated as "no such order".
func orderID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusNotFound, "not_found", "Order not found.")
		return 0, false
	}
	return id, true
}

// writeOrder sends an order, or the matching error. Keys are in the body, so it
// must never be cached on the way.
func writeOrder(w http.ResponseWriter, r *http.Request, order orders.Order, err error) {
	switch {
	case errors.Is(err, orders.ErrNotFound):
		// Someone else's order gets the same answer as a missing one: no hint that
		// the id exists.
		writeError(w, http.StatusNotFound, "not_found", "Order not found.")
	case errors.Is(err, orders.ErrNotSimulated):
		writeError(w, http.StatusNotFound, "not_found", "The simulated payment is not enabled.")
	case errors.Is(err, orders.ErrOrderExpired):
		writeError(w, http.StatusConflict, "order_expired", "This order has expired. Please check out again.")
	case err != nil:
		writeInternalError(w, r, err)
	default:
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, order)
	}
}

func (h orderHandlers) get(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	id, ok := orderID(w, r)
	if !ok {
		return
	}
	order, err := h.orders.Get(r.Context(), user.ID, id)
	writeOrder(w, r, order, err)
}

func (h orderHandlers) confirm(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	id, ok := orderID(w, r)
	if !ok {
		return
	}
	order, err := h.orders.Confirm(r.Context(), user.ID, id)
	writeOrder(w, r, order, err)
}

func (h orderHandlers) paySimulated(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	id, ok := orderID(w, r)
	if !ok {
		return
	}
	order, err := h.orders.PaySimulated(r.Context(), user.ID, id)
	writeOrder(w, r, order, err)
}

type ordersResponse struct {
	Items []orders.Order `json:"items"`
}

func (h orderHandlers) list(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	list, err := h.orders.List(r.Context(), user.ID)
	if err != nil {
		writeInternalError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, ordersResponse{Items: list})
}

// Stripe sends events of a few kilobytes; this leaves plenty of room and still
// bounds what an anonymous caller can make the server read.
const maxWebhookBytes = 1 << 20

// stripeWebhook serves POST /api/webhooks/stripe.
//
// The request is anonymous: anyone on the internet can POST to this URL. What makes
// it trustworthy is the Stripe-Signature header, an HMAC-SHA256 of the exact raw body
// computed with the endpoint's secret, which only Stripe and we know. The body is
// therefore read as raw bytes and verified BEFORE it is parsed or anything is done.
// The signature also contains a timestamp; events older than 5 minutes are refused,
// so a recorded request cannot be replayed later.
func (h orderHandlers) stripeWebhook(w http.ResponseWriter, r *http.Request) {
	if h.webhookSecret == "" {
		writeError(w, http.StatusServiceUnavailable, "webhook_not_configured", "Webhooks are not configured.")
		return
	}
	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Could not read the body.")
		return
	}
	event, err := webhook.ConstructEventWithOptions(payload, r.Header.Get("Stripe-Signature"), h.webhookSecret,
		webhook.ConstructEventOptions{
			// The account's API version can differ from the library's; we only read
			// a few long-stable fields of a Checkout Session (id, payment_status).
			IgnoreAPIVersionMismatch: true,
		})
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_signature", "Invalid signature.")
		return
	}

	switch event.Type {
	case "checkout.session.completed", "checkout.session.async_payment_succeeded", "checkout.session.expired":
	default:
		// Other events (if the endpoint is subscribed to more) are acknowledged so
		// Stripe stops sending them, and otherwise ignored.
		w.WriteHeader(http.StatusOK)
		return
	}
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Unexpected event data.")
		return
	}

	err = h.orders.HandlePaymentEvent(r.Context(), orders.PaymentEvent{
		Provider:  "stripe",
		ID:        event.ID,
		Type:      string(event.Type),
		SessionID: session.ID,
		Paid:      session.PaymentStatus == stripe.CheckoutSessionPaymentStatusPaid,
		Payload:   payload,
	})
	if err != nil {
		// A 500 makes Stripe retry the event later (with backoff, for up to 3 days).
		writeInternalError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

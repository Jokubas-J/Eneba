// Package orders turns a cart into an order: it reserves license keys, starts the
// payment with a payment provider (Stripe or a simulated one), and delivers the keys
// when the payment is confirmed. Design and failure cases: docs/checkout.md.
package orders

import (
	"errors"
	"fmt"
	"time"
)

// Order statuses, as stored in orders.status.
const (
	StatusPending = "pending" // keys reserved, waiting for payment
	StatusPaid    = "paid"    // keys sold and visible to the buyer
	StatusExpired = "expired" // not paid in time; keys returned to stock
)

// ReservationTime is how long keys are held for an unpaid order. 30 minutes is the
// shortest expiry Stripe accepts for a Checkout Session, and the session is given
// the same expiry, so payment is impossible after the reservation ends.
const ReservationTime = 30 * time.Minute

// ExpiryGrace is how long after an order's expiry the clean-up job waits before
// releasing its keys. A "paid" webhook that is merely late is processed within it.
const ExpiryGrace = 5 * time.Minute

var (
	ErrNotFound      = errors.New("order not found")
	ErrEmptyCart     = errors.New("the cart is empty")
	ErrNotSimulated  = errors.New("the simulated payment provider is not enabled")
	ErrOrderExpired  = errors.New("the order has expired")
	ErrInvalidCart   = errors.New("invalid cart")
	ErrOfferNotFound = errors.New("an offer in the cart is no longer available")
)

// OutOfStockError says which offer did not have enough keys at checkout.
type OutOfStockError struct {
	OfferID     int64
	ProductName string
	Available   int
}

func (e *OutOfStockError) Error() string {
	return fmt.Sprintf("only %d keys left for %s", e.Available, e.ProductName)
}

// Order is an order as the buyer sees it.
type Order struct {
	ID         int64       `json:"id"`
	Status     string      `json:"status"`
	TotalCents int         `json:"totalCents"`
	Currency   string      `json:"currency"`
	Provider   string      `json:"paymentProvider"`
	CreatedAt  time.Time   `json:"createdAt"`
	ExpiresAt  time.Time   `json:"expiresAt"`
	PaidAt     *time.Time  `json:"paidAt"`
	Items      []OrderItem `json:"items"`
}

// OrderItem is one line of an order. Name and price are snapshots from the moment of
// checkout, so the order history stays right if the offer changes later.
type OrderItem struct {
	ID             int64  `json:"id"`
	OfferID        int64  `json:"offerId"`
	ProductSlug    string `json:"productSlug"`
	ProductName    string `json:"productName"`
	CoverURL       string `json:"coverUrl"`
	SellerName     string `json:"sellerName"`
	UnitPriceCents int    `json:"unitPriceCents"`
	Quantity       int    `json:"quantity"`
	// Keys are filled in only for paid orders: before payment nobody sees them.
	Keys []string `json:"keys"`
}

// CheckoutResult is what the browser needs after POST /api/checkout.
type CheckoutResult struct {
	OrderID int64 `json:"orderId"`
	// RedirectURL is where to send the buyer to pay: Stripe's hosted page, or our
	// own simulated payment page.
	RedirectURL string `json:"redirectUrl"`
}

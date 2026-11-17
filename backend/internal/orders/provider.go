package orders

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	"github.com/stripe/stripe-go/v87"
)

// PaymentProvider is a way to take the payment for an order. The order service does
// not know which one it talks to; PAYMENT_PROVIDER chooses at start-up.
type PaymentProvider interface {
	// Name is stored in orders.payment_provider ("stripe" or "simulated").
	Name() string
	// CreateSession starts a payment for the order and returns the provider's session
	// id and the address to send the buyer to.
	CreateSession(ctx context.Context, order Order) (sessionID, redirectURL string, err error)
	// IsPaid asks the provider whether a session has been paid. Used by the success
	// page, which may arrive before the webhook.
	IsPaid(ctx context.Context, sessionID string) (bool, error)
}

// --- Simulated provider ------------------------------------------------------

// SimulatedProvider takes no money. Its "payment page" is a page of our own frontend
// with a Pay button, which calls POST /api/payments/simulated/{id}. It exists so the
// whole flow works (and can be tested) without Stripe, and as a fallback for the demo.
type SimulatedProvider struct {
	BaseURL string // the frontend's address, APP_BASE_URL
}

func (p SimulatedProvider) Name() string { return "simulated" }

func (p SimulatedProvider) CreateSession(ctx context.Context, order Order) (string, string, error) {
	random := make([]byte, 12)
	if _, err := rand.Read(random); err != nil {
		return "", "", err
	}
	return "sim_" + hex.EncodeToString(random), fmt.Sprintf("%s/checkout/simulated/%d", p.BaseURL, order.ID), nil
}

// IsPaid is never true: a simulated payment is completed by the Pay button directly,
// not by asking the provider.
func (p SimulatedProvider) IsPaid(ctx context.Context, sessionID string) (bool, error) {
	return false, nil
}

// --- Stripe ------------------------------------------------------------------

// StripeProvider uses Stripe Checkout: Stripe hosts the payment page, so no card data
// ever reaches our server.
type StripeProvider struct {
	client      *stripe.Client
	baseURL     string
	environment string // APP_ENV
}

func NewStripeProvider(secretKey, baseURL, environment string) *StripeProvider {
	return newStripeProvider(stripe.NewClient(secretKey), baseURL, environment)
}

// newStripeProvider takes the client ready-made, so tests can point it at a fake
// Stripe server.
func newStripeProvider(client *stripe.Client, baseURL, environment string) *StripeProvider {
	return &StripeProvider{client: client, baseURL: baseURL, environment: environment}
}

func (p *StripeProvider) Name() string { return "stripe" }

func (p *StripeProvider) CreateSession(ctx context.Context, order Order) (string, string, error) {
	if order.PaymentReference == "" {
		// Without it the idempotency key would not be unique; refuse rather than risk
		// another collision.
		return "", "", errors.New("order has no payment reference")
	}
	orderID := strconv.FormatInt(order.ID, 10)
	params := &stripe.CheckoutSessionCreateParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModePayment)),
		// Card only. Without this Stripe also offers methods such as bank redirects
		// whose result can arrive minutes or days later, after the reservation has
		// expired and the keys may be sold to someone else. A card answers at once.
		AllowedPaymentMethodTypes: []*string{stripe.String("card")},
		// {CHECKOUT_SESSION_ID} is filled in by Stripe; the order page sends it back
		// to /confirm so we can ask Stripe whether that session is paid.
		SuccessURL: stripe.String(fmt.Sprintf("%s/orders/%d?session_id={CHECKOUT_SESSION_ID}", p.baseURL, order.ID)),
		CancelURL:  stripe.String(p.baseURL + "/cart"),
		// The session expires with the reservation: Stripe will not take a payment
		// after the keys could have been released.
		ExpiresAt:         stripe.Int64(order.ExpiresAt.Unix()),
		ClientReferenceID: stripe.String(orderID),
		// environment lets the webhook tell development sessions from production
		// ones: both databases share one Stripe test account, and Stripe sends every
		// event to every endpoint of the account.
		Metadata: map[string]string{
			"environment":       p.environment,
			"order_id":          orderID,
			"payment_reference": order.PaymentReference,
		},
	}
	// The amounts come from the order, which was priced from the database.
	for _, item := range order.Items {
		params.LineItems = append(params.LineItems, &stripe.CheckoutSessionCreateLineItemParams{
			Quantity: stripe.Int64(int64(item.Quantity)),
			PriceData: &stripe.CheckoutSessionCreateLineItemPriceDataParams{
				Currency:   stripe.String("eur"),
				UnitAmount: stripe.Int64(int64(item.UnitPriceCents)), // Stripe also counts in cents
				ProductData: &stripe.CheckoutSessionCreateLineItemPriceDataProductDataParams{
					Name:        stripe.String(item.ProductName),
					Description: stripe.String("Sold by " + item.SellerName + " · demo key"),
				},
			},
		})
	}
	// The idempotency key makes a retried request (network hiccup) return the same
	// session instead of creating a second one for the same order. Stripe keeps keys
	// per account, and the account is shared by development and production, so the
	// key must be unique across databases: the order's random payment reference, not
	// its id (two databases both have an order 2).
	params.SetIdempotencyKey("checkout-session-" + order.PaymentReference)

	session, err := p.client.V1CheckoutSessions.Create(ctx, params)
	if err != nil {
		return "", "", fmt.Errorf("create Stripe Checkout Session: %w", err)
	}
	return session.ID, session.URL, nil
}

func (p *StripeProvider) IsPaid(ctx context.Context, sessionID string) (bool, error) {
	session, err := p.client.V1CheckoutSessions.Retrieve(ctx, sessionID, nil)
	if err != nil {
		return false, fmt.Errorf("retrieve Stripe Checkout Session: %w", err)
	}
	return session.PaymentStatus == stripe.CheckoutSessionPaymentStatusPaid, nil
}

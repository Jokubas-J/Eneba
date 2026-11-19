package orders

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Jokubas-J/Eneba/backend/internal/catalog"
)

// Service implements checkout, payment confirmation and order history.
type Service struct {
	pool     *pgxpool.Pool
	provider PaymentProvider
}

func NewService(pool *pgxpool.Pool, provider PaymentProvider) *Service {
	return &Service{pool: pool, provider: provider}
}

// ProviderName says which payment provider is active ("stripe" or "simulated").
func (s *Service) ProviderName() string { return s.provider.Name() }

// validateLines applies the same limits as the cart page (catalog.ValidateCart).
func validateLines(lines []catalog.CartLine) error {
	if len(lines) == 0 {
		return ErrEmptyCart
	}
	if len(lines) > catalog.MaxCartLines {
		return fmt.Errorf("%w: too many lines", ErrInvalidCart)
	}
	seen := map[int64]bool{}
	for _, line := range lines {
		if line.Quantity < 1 || line.Quantity > catalog.MaxLineQuantity {
			return fmt.Errorf("%w: quantity must be between 1 and %d", ErrInvalidCart, catalog.MaxLineQuantity)
		}
		if seen[line.OfferID] {
			return fmt.Errorf("%w: an offer appears twice", ErrInvalidCart)
		}
		seen[line.OfferID] = true
	}
	return nil
}

// Checkout creates an order for the signed-in user, reserves its keys and starts the
// payment. lines carries only offer ids and quantities: every price is read from the
// database inside the transaction, so nothing the browser sends can change what is
// charged.
func (s *Service) Checkout(ctx context.Context, userID int64, lines []catalog.CartLine) (CheckoutResult, error) {
	if err := validateLines(lines); err != nil {
		return CheckoutResult{}, err
	}

	order, err := s.createOrderAndReserve(ctx, userID, lines)
	if err != nil {
		return CheckoutResult{}, err
	}

	sessionID, redirectURL, err := s.provider.CreateSession(ctx, order)
	if err != nil {
		// The keys are reserved but nobody can pay for them: give them back at once
		// instead of holding them for 30 minutes.
		if expireErr := s.expire(ctx, order.ID); expireErr != nil {
			log.Printf("order %d: release after failed payment session: %v", order.ID, expireErr)
		}
		return CheckoutResult{}, err
	}
	if _, err := s.pool.Exec(ctx, `UPDATE orders SET provider_session_id = $2 WHERE id = $1`, order.ID, sessionID); err != nil {
		return CheckoutResult{}, fmt.Errorf("store payment session: %w", err)
	}
	return CheckoutResult{OrderID: order.ID, RedirectURL: redirectURL}, nil
}

// createOrderAndReserve is the heart of the checkout: one transaction that either
// creates the order with all its keys reserved, or changes nothing.
func (s *Service) createOrderAndReserve(ctx context.Context, userID int64, lines []catalog.CartLine) (Order, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback(ctx) // no effect after Commit

	if err := checkPendingLimits(ctx, tx, userID, lines); err != nil {
		return Order{}, err
	}

	order := Order{Status: StatusPending, Currency: "EUR", Provider: s.provider.Name()}
	// The expiry comes from the database clock, like every other time comparison,
	// so the server's own clock cannot disagree with it.
	err = tx.QueryRow(ctx, `
		INSERT INTO orders (user_id, status, total_cents, currency, payment_provider, expires_at)
		VALUES ($1, 'pending', 0, 'EUR', $2, now() + make_interval(secs => $3))
		RETURNING id, created_at, expires_at, payment_reference::text`,
		userID, order.Provider, ReservationTime.Seconds()).Scan(&order.ID, &order.CreatedAt, &order.ExpiresAt, &order.PaymentReference)
	if err != nil {
		return Order{}, fmt.Errorf("create order: %w", err)
	}

	for _, line := range lines {
		var item OrderItem
		// The offer as it is NOW: active, its current price, product and seller names.
		err := tx.QueryRow(ctx, `
			SELECT o.id, p.slug, p.name, p.cover_url, s.name, o.price_cents
			FROM offers o
			JOIN products p ON p.id = o.product_id
			JOIN sellers s ON s.id = o.seller_id
			WHERE o.id = $1 AND o.is_active`, line.OfferID).Scan(
			&item.OfferID, &item.ProductSlug, &item.ProductName, &item.CoverURL, &item.SellerName, &item.UnitPriceCents)
		if errors.Is(err, pgx.ErrNoRows) {
			return Order{}, ErrOfferNotFound
		}
		if err != nil {
			return Order{}, fmt.Errorf("load offer %d: %w", line.OfferID, err)
		}
		item.Quantity = line.Quantity

		err = tx.QueryRow(ctx, `
			INSERT INTO order_items (order_id, offer_id, product_name, seller_name, unit_price_cents, quantity)
			VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
			order.ID, item.OfferID, item.ProductName, item.SellerName, item.UnitPriceCents, item.Quantity).Scan(&item.ID)
		if err != nil {
			return Order{}, fmt.Errorf("create order item: %w", err)
		}

		// Reserve the keys. The inner SELECT picks up to n available keys and locks
		// them; SKIP LOCKED makes it pass over keys that another checkout has locked
		// at this very moment instead of waiting for it. So two buyers can never get
		// the same key, and neither blocks the other. The outer UPDATE ties the picked
		// keys to this order item.
		rows, err := tx.Query(ctx, `
			UPDATE license_keys
			SET status = 'reserved', order_item_id = $2, reserved_until = $3
			WHERE id IN (
				SELECT id FROM license_keys
				WHERE offer_id = $1 AND status = 'available'
				ORDER BY id
				LIMIT $4
				FOR UPDATE SKIP LOCKED
			)
			RETURNING id`, item.OfferID, item.ID, order.ExpiresAt, item.Quantity)
		if err != nil {
			return Order{}, fmt.Errorf("reserve keys: %w", err)
		}
		reserved, err := pgx.CollectRows(rows, pgx.RowTo[int64])
		if err != nil {
			return Order{}, fmt.Errorf("reserve keys: %w", err)
		}
		if len(reserved) < item.Quantity {
			// Not enough: the deferred Rollback undoes the whole order, including keys
			// already reserved for earlier lines.
			return Order{}, &OutOfStockError{OfferID: item.OfferID, ProductName: item.ProductName, Available: len(reserved)}
		}

		order.TotalCents += item.UnitPriceCents * item.Quantity
		order.Items = append(order.Items, item)
	}

	if _, err := tx.Exec(ctx, `UPDATE orders SET total_cents = $2 WHERE id = $1`, order.ID, order.TotalCents); err != nil {
		return Order{}, fmt.Errorf("store order total: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Order{}, err
	}
	return order, nil
}

// checkPendingLimits refuses a checkout when the user already holds
// MaxPendingOrders unpaid orders, or when the new order would take their reserved
// keys past MaxReservedKeys.
//
// It first locks the user's row (FOR UPDATE) until the transaction ends. Without
// that, two checkouts of the same user at the same moment could both count "2
// pending" and both go ahead; with it, the second waits until the first has
// committed and then counts 3. Checkouts of different users do not wait for each other.
func checkPendingLimits(ctx context.Context, tx pgx.Tx, userID int64, lines []catalog.CartLine) error {
	if _, err := tx.Exec(ctx, `SELECT 1 FROM users WHERE id = $1 FOR UPDATE`, userID); err != nil {
		return fmt.Errorf("lock user %d: %w", userID, err)
	}
	var pendingOrders, reservedKeys int
	// Only orders that still hold keys: an expired order whose clean-up has not run
	// yet does not count against the user.
	err := tx.QueryRow(ctx, `
		SELECT count(DISTINCT o.id), COALESCE(sum(i.quantity), 0)
		FROM orders o
		JOIN order_items i ON i.order_id = o.id
		WHERE o.user_id = $1 AND o.status = 'pending' AND o.expires_at > now()`, userID).Scan(&pendingOrders, &reservedKeys)
	if err != nil {
		return fmt.Errorf("count pending orders: %w", err)
	}
	wanted := 0
	for _, line := range lines {
		wanted += line.Quantity
	}
	if pendingOrders >= MaxPendingOrders || reservedKeys+wanted > MaxReservedKeys {
		return ErrTooManyPending
	}
	return nil
}

// markPaid is the single place where an order becomes paid and its keys sold. It is
// safe to call any number of times, from several places at once (webhook, success
// page, a retried webhook): the UPDATE ... WHERE status = 'pending' succeeds for
// exactly one caller, because PostgreSQL locks the row and the others then see
// status = 'paid'. Everyone else finds 0 rows and does nothing.
//
// It runs inside the caller's transaction, so the webhook can record the event and
// fulfil the order atomically.
func markPaid(ctx context.Context, tx pgx.Tx, orderID int64) (fulfilled bool, err error) {
	var status string
	err = tx.QueryRow(ctx, `
		UPDATE orders SET status = 'paid', paid_at = now()
		WHERE id = $1 AND status = 'pending'
		RETURNING status`, orderID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		// Already paid (a repeat: fine) or expired (must not be delivered).
		var current string
		if err := tx.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&current); err != nil {
			return false, fmt.Errorf("load order %d: %w", orderID, err)
		}
		if current == StatusExpired {
			return false, ErrOrderExpired
		}
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("mark order %d paid: %w", orderID, err)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE license_keys SET status = 'sold', reserved_until = NULL
		WHERE status = 'reserved'
		  AND order_item_id IN (SELECT id FROM order_items WHERE order_id = $1)`, orderID)
	if err != nil {
		return false, fmt.Errorf("sell keys of order %d: %w", orderID, err)
	}
	var want int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(sum(quantity), 0) FROM order_items WHERE order_id = $1`, orderID).Scan(&want); err != nil {
		return false, err
	}
	if tag.RowsAffected() != want {
		// Cannot happen while reservations work as designed; refusing (and rolling
		// back) is better than marking an order paid with keys missing.
		return false, fmt.Errorf("order %d: %d keys reserved, %d expected", orderID, tag.RowsAffected(), want)
	}
	return true, nil
}

// expireTx marks a pending order expired and returns its reserved keys to stock.
// Like markPaid it only acts on a pending order, so it cannot undo a payment.
func expireTx(ctx context.Context, tx pgx.Tx, orderID int64) (bool, error) {
	tag, err := tx.Exec(ctx, `UPDATE orders SET status = 'expired' WHERE id = $1 AND status = 'pending'`, orderID)
	if err != nil {
		return false, fmt.Errorf("expire order %d: %w", orderID, err)
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	_, err = tx.Exec(ctx, `
		UPDATE license_keys SET status = 'available', order_item_id = NULL, reserved_until = NULL
		WHERE status = 'reserved'
		  AND order_item_id IN (SELECT id FROM order_items WHERE order_id = $1)`, orderID)
	if err != nil {
		return false, fmt.Errorf("release keys of order %d: %w", orderID, err)
	}
	return true, nil
}

// inTx runs fn in a transaction and commits if it returns no error.
func (s *Service) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// fulfil marks an order paid in its own transaction.
func (s *Service) fulfil(ctx context.Context, orderID int64) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		_, err := markPaid(ctx, tx, orderID)
		return err
	})
}

func (s *Service) expire(ctx context.Context, orderID int64) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		_, err := expireTx(ctx, tx, orderID)
		return err
	})
}

// ReleaseExpired expires every pending order whose time ran out more than ExpiryGrace
// ago and returns its keys. The server runs it every minute: a safety net for a lost
// "expired" webhook, and the only release path for the simulated provider.
func (s *Service) ReleaseExpired(ctx context.Context) (int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id FROM orders
		WHERE status = 'pending' AND expires_at < now() - make_interval(secs => $1)`, ExpiryGrace.Seconds())
	if err != nil {
		return 0, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if err := s.expire(ctx, id); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

// PaymentEvent is a provider notification (a Stripe webhook event), already
// verified by the HTTP layer.
type PaymentEvent struct {
	Provider  string
	ID        string // unique per provider; a repeated delivery has the same id
	Type      string
	SessionID string
	Paid      bool            // for completed sessions: payment_status == "paid"
	Payload   json.RawMessage // stored for the audit trail
}

// HandlePaymentEvent applies a provider event exactly once. The event id is stored in
// payment_events in the SAME transaction as its effect: a second delivery of the
// same event finds the id already stored and changes nothing. If applying the event
// fails, the transaction rolls back, the id is not stored, and the provider's retry
// gets another chance.
func (s *Service) HandlePaymentEvent(ctx context.Context, event PaymentEvent) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			INSERT INTO payment_events (provider, event_id, type, payload)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (provider, event_id) DO NOTHING`, event.Provider, event.ID, event.Type, event.Payload)
		if err != nil {
			return fmt.Errorf("record payment event: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return nil // seen before
		}

		var orderID int64
		err = tx.QueryRow(ctx, `SELECT id FROM orders WHERE provider_session_id = $1`, event.SessionID).Scan(&orderID)
		if errors.Is(err, pgx.ErrNoRows) {
			// Not one of ours (another app on the same Stripe account, or a test
			// event): recorded, nothing to do.
			return nil
		}
		if err != nil {
			return err
		}

		switch event.Type {
		case "checkout.session.completed", "checkout.session.async_payment_succeeded":
			if !event.Paid {
				return nil // e.g. a bank transfer still on its way; not used by us
			}
			_, err := markPaid(ctx, tx, orderID)
			if errors.Is(err, ErrOrderExpired) {
				// Should be impossible (the session expires with the order). Keep the
				// event, deliver nothing, and leave a trace for a manual refund.
				log.Printf("ALERT order %d: payment received after expiry (event %s); refund manually", orderID, event.ID)
				return nil
			}
			return err
		case "checkout.session.expired":
			_, err := expireTx(ctx, tx, orderID)
			return err
		}
		return nil
	})
}

// PaySimulated completes a payment with the simulated provider (the Pay button).
func (s *Service) PaySimulated(ctx context.Context, userID, orderID int64) (Order, error) {
	if s.provider.Name() != "simulated" {
		return Order{}, ErrNotSimulated
	}
	if _, err := s.Get(ctx, userID, orderID); err != nil {
		return Order{}, err // not this user's order
	}
	if err := s.fulfil(ctx, orderID); err != nil {
		return Order{}, err
	}
	return s.Get(ctx, userID, orderID)
}

// Confirm is called by the order page after the buyer returns from the payment page.
// If the order is still pending, it asks the provider whether the session is paid
// and fulfils it if so; the webhook will then find the order already paid.
func (s *Service) Confirm(ctx context.Context, userID, orderID int64) (Order, error) {
	order, sessionID, err := s.get(ctx, userID, orderID)
	if err != nil {
		return Order{}, err
	}
	if order.Status != StatusPending || sessionID == "" {
		return order, nil
	}
	paid, err := s.provider.IsPaid(ctx, sessionID)
	if err != nil {
		return Order{}, err
	}
	if !paid {
		return order, nil
	}
	if err := s.fulfil(ctx, orderID); err != nil && !errors.Is(err, ErrOrderExpired) {
		return Order{}, err
	}
	return s.Get(ctx, userID, orderID)
}

// Get returns one order of the user, with keys if it is paid. Another user's order
// is ErrNotFound, exactly like an order that does not exist.
func (s *Service) Get(ctx context.Context, userID, orderID int64) (Order, error) {
	order, _, err := s.get(ctx, userID, orderID)
	return order, err
}

func (s *Service) get(ctx context.Context, userID, orderID int64) (Order, string, error) {
	var order Order
	var sessionID *string
	err := s.pool.QueryRow(ctx, `
		SELECT id, status, total_cents, currency, payment_provider, created_at, expires_at, paid_at, provider_session_id
		FROM orders WHERE id = $1 AND user_id = $2`, orderID, userID).Scan(
		&order.ID, &order.Status, &order.TotalCents, &order.Currency, &order.Provider,
		&order.CreatedAt, &order.ExpiresAt, &order.PaidAt, &sessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, "", ErrNotFound
	}
	if err != nil {
		return Order{}, "", err
	}
	items, err := s.items(ctx, []int64{order.ID}, order.Status == StatusPaid)
	if err != nil {
		return Order{}, "", err
	}
	order.Items = items[order.ID]
	if order.Items == nil {
		order.Items = []OrderItem{}
	}
	session := ""
	if sessionID != nil {
		session = *sessionID
	}
	return order, session, nil
}

// List returns the user's orders, newest first, without keys (the order page shows
// them).
func (s *Service) List(ctx context.Context, userID int64) ([]Order, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, status, total_cents, currency, payment_provider, created_at, expires_at, paid_at
		FROM orders WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	orders, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Order, error) {
		var o Order
		err := row.Scan(&o.ID, &o.Status, &o.TotalCents, &o.Currency, &o.Provider, &o.CreatedAt, &o.ExpiresAt, &o.PaidAt)
		return o, err
	})
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(orders))
	for i, o := range orders {
		ids[i] = o.ID
	}
	items, err := s.items(ctx, ids, false)
	if err != nil {
		return nil, err
	}
	for i := range orders {
		orders[i].Items = items[orders[i].ID]
		if orders[i].Items == nil {
			orders[i].Items = []OrderItem{}
		}
	}
	return orders, nil
}

// items loads the lines of the given orders, keyed by order id. withKeys adds the
// sold keys of each line (only ever asked for a paid order).
func (s *Service) items(ctx context.Context, orderIDs []int64, withKeys bool) (map[int64][]OrderItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT i.order_id, i.id, i.offer_id, p.slug, i.product_name, p.cover_url, i.seller_name, i.unit_price_cents, i.quantity
		FROM order_items i
		JOIN offers o ON o.id = i.offer_id
		JOIN products p ON p.id = o.product_id
		WHERE i.order_id = ANY($1)
		ORDER BY i.id`, orderIDs)
	if err != nil {
		return nil, err
	}
	byOrder := map[int64][]OrderItem{}
	// position of each item within its order's slice, to attach the keys below
	position := map[int64]int{}
	type row struct {
		orderID int64
		item    OrderItem
	}
	all, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (row, error) {
		var x row
		err := r.Scan(&x.orderID, &x.item.ID, &x.item.OfferID, &x.item.ProductSlug, &x.item.ProductName,
			&x.item.CoverURL, &x.item.SellerName, &x.item.UnitPriceCents, &x.item.Quantity)
		x.item.Keys = []string{}
		return x, err
	})
	if err != nil {
		return nil, err
	}
	for _, x := range all {
		byOrder[x.orderID] = append(byOrder[x.orderID], x.item)
		position[x.item.ID] = len(byOrder[x.orderID]) - 1
	}
	if !withKeys || len(all) == 0 {
		return byOrder, nil
	}

	keyRows, err := s.pool.Query(ctx, `
		SELECT i.order_id, k.order_item_id, k.key_value
		FROM license_keys k JOIN order_items i ON i.id = k.order_item_id
		WHERE i.order_id = ANY($1) AND k.status = 'sold'
		ORDER BY k.id`, orderIDs)
	if err != nil {
		return nil, err
	}
	defer keyRows.Close()
	for keyRows.Next() {
		var orderID, itemID int64
		var key string
		if err := keyRows.Scan(&orderID, &itemID, &key); err != nil {
			return nil, err
		}
		item := &byOrder[orderID][position[itemID]]
		item.Keys = append(item.Keys, key)
	}
	return byOrder, keyRows.Err()
}

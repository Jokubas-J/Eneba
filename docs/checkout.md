# Checkout and key delivery

Code: `backend/internal/orders/` (rules, SQL, payment providers), `backend/internal/httpapi/orders.go`
(endpoints, webhook), `frontend/src/pages/OrderPage.tsx`, `OrdersPage.tsx`, `SimulatedPaymentPage.tsx`.
Tests: `backend/internal/orders/*_test.go`, `backend/internal/httpapi/orders_test.go`.

## When are keys taken? (decision)

A key is a unique thing: once it is sold to two people, one of them has paid for nothing. The
question is at which moment a key is tied to an order.

| | A. Reserve when the payment session is created | B. Assign only when the payment webhook arrives |
|---|---|---|
| What happens | `POST /api/checkout` locks N available keys of each offer and marks them `reserved` for this order, with an expiry. Payment turns them `sold`; an expired or abandoned payment returns them to `available`. | Checkout only creates the order. When Stripe reports "paid", the webhook takes N available keys. |
| Risk | Keys are unavailable to others for up to the reservation time even if the buyer walks away (stock looks lower than it is). | **The buyer can pay and then there is no key left**: two people pay for the last key at the same time and one of them must be refunded and disappointed. |
| Extra work | Expiry: release on Stripe's `checkout.session.expired` webhook, plus a periodic clean-up as a safety net. | Refund flow for "paid but sold out"; telling the buyer after they paid. |
| Who sees the failure | The buyer, *before* paying: "Only 0 left" at checkout. | The buyer, *after* paying. |

**Chosen: A, reserve at checkout.** A marketplace must never take money for a key it cannot
deliver; reserving moves every "sold out" moment to before the payment. Stock that is temporarily
held is the cheaper problem, and it fixes itself when the reservation expires. This is also how
PLAN.md §2 described it.

Details:

- **Reservation time = 30 minutes**, the shortest expiry Stripe allows for a Checkout Session.
  The session is created with the same `expires_at`, so Stripe refuses payment after that moment.
- **Release:** `checkout.session.expired` (sent by Stripe when the session expires) releases the
  keys immediately. A background job every minute also releases reservations whose order expired
  **more than 5 minutes ago** (safety net for a lost webhook, and the only path for the simulated
  provider). The 5-minute grace makes sure a "paid" webhook that is merely late is processed before
  its keys could be given to someone else.
- **Locking:** `SELECT … FOR UPDATE SKIP LOCKED LIMIT n` picks keys that no other transaction is
  holding. Two buyers racing for the last key: the first locks it, the second skips it, finds
  none and gets 409 `out_of_stock`. Nobody waits on anybody, and no key can be picked twice.

## Paying and fulfilment

```
POST /api/checkout {items:[{offerId, quantity}]}         (signed in)
  BEGIN
    read each offer's CURRENT price from the database   <- the browser sends no prices
    insert order (pending, expires_at = now + 30 min) and order_items (name/price snapshots)
    reserve keys FOR UPDATE SKIP LOCKED                 <- 409 out_of_stock if too few
  COMMIT
  provider.CreateSession(order)  -> redirect URL        <- on failure: order expired, keys released
```

**Fulfilment** (`markPaid`) is one statement that decides everything:
`UPDATE orders SET status='paid' … WHERE id=$1 AND status='pending'`. PostgreSQL row locking means
only the first caller gets "1 row updated"; every repeated or concurrent call (duplicate webhook,
success page, a retry) gets 0 rows and does nothing. In the same transaction the order's reserved
keys become `sold`. A key can therefore be delivered once, and an expired order (status already
`expired`) can never be marked paid.

Three paths call it:

1. **Stripe webhook** `checkout.session.completed` with `payment_status = paid`.
2. **Success page**: `POST /api/orders/{id}/confirm` asks Stripe for the session and fulfils if
   it is paid, so the buyer sees the keys without waiting for the webhook (Stripe recommends
   handling both).
3. **Simulated provider**: `POST /api/payments/simulated/{id}` (only when
   `PAYMENT_PROVIDER=simulated`).

**Webhook safety:** the signature in the `Stripe-Signature` header is checked against
`STRIPE_WEBHOOK_SECRET` (HMAC over the exact raw body, 5-minute tolerance against replays) before
anything is parsed. Each event id is stored in `payment_events` in the same transaction as its
effect; a repeated event finds its id already there and is acknowledged without doing anything.

## Failure cases

| Case | Handling | Test |
|---|---|---|
| Two buyers, one key | One order, the other gets `out_of_stock`; the key is reserved once | `TestRaceForTheLastKey` (20 goroutines) |
| Same webhook delivered twice / concurrently | One `payment_events` row, order paid once, keys sold once | `TestDuplicateWebhook`, `TestConcurrentFulfilment` |
| Buyer abandons payment | Expired webhook or the clean-up job returns the keys | `TestExpiryReleasesKeys` |
| Payment completed after the order expired | Not possible through Stripe (session expires with the order). If it ever happens, the order stays `expired`, nothing is delivered, and the event is logged for a manual refund | `TestExpiredOrderCannotBePaid` |
| Browser sends prices | There is no price field in the request; any extra JSON field is ignored; totals come from the database | `TestCheckoutIgnoresPricesFromTheBrowser` |
| Price changed after the cart was shown | The order uses the price at checkout; the Stripe page shows that total before paying | – |
| Someone else's order | 404, as if it did not exist (no hint that the id is valid) | `TestOrdersAreOnlyVisibleToTheirOwner` |
| Forged webhook | 400 before any database access | `TestStripeWebhookSignature` |
| Stripe unreachable at checkout | Order expired at once, keys released, 502 to the browser | `TestProviderFailureReleasesKeys` |

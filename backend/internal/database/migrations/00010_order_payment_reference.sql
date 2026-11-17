-- +goose Up

-- A random, globally unique reference per order, used for the payment provider's
-- idempotency key. The order id cannot be used for that: it is a per-database
-- sequence, and the development and production databases share one Stripe test
-- account, so both have an "order 2" (bug found on the live site, see
-- docs/journal.md 2026-11-17). gen_random_uuid() is built into PostgreSQL 13+.
-- Existing rows get a value of their own from the default.
ALTER TABLE orders ADD COLUMN payment_reference uuid NOT NULL DEFAULT gen_random_uuid();
ALTER TABLE orders ADD CONSTRAINT orders_payment_reference_key UNIQUE (payment_reference);

-- +goose Down
ALTER TABLE orders DROP COLUMN payment_reference;

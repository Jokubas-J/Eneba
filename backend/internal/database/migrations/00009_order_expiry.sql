-- +goose Up

-- When an unpaid order (and its reserved keys) expires. Set at checkout to the same
-- moment as the payment session's expiry; see docs/checkout.md.
ALTER TABLE orders ADD COLUMN expires_at timestamptz;
UPDATE orders SET expires_at = created_at + interval '30 minutes' WHERE expires_at IS NULL;
ALTER TABLE orders ALTER COLUMN expires_at SET NOT NULL;

-- The clean-up job looks for pending orders that have expired; a partial index keeps
-- that lookup small (paid and expired orders are never in it).
CREATE INDEX orders_pending_expires_at_idx ON orders (expires_at) WHERE status = 'pending';

-- +goose Down
DROP INDEX orders_pending_expires_at_idx;
ALTER TABLE orders DROP COLUMN expires_at;

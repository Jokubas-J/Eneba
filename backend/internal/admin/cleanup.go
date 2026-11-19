// Package admin holds maintenance tasks run by hand from the command line
// (cmd/admin), never by the web server.
package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestAccounts is what DeleteTestAccounts found (and deleted, unless it was a dry run).
type TestAccounts struct {
	Emails       []string
	Orders       int
	KeysReturned int // reserved or sold keys of those orders, put back on sale
}

// DeleteTestAccounts removes the accounts whose e-mail matches one of the SQL LIKE
// patterns (e.g. 'cookie-check-%@example.com'), with everything that hangs on them,
// in one transaction:
//
//  1. the keys their orders reserved or bought go back on sale: these were test
//     purchases made while verifying the live site, not real sales;
//  2. their orders (and with them the order lines) are deleted;
//  3. the accounts are deleted (and with them their sessions).
//
// The order matters because of the foreign keys: a key points at an order line and
// an order points at its user, and neither link deletes along. payment_events are
// kept: they are Stripe's record of what happened, with no link to a user.
//
// With apply false it only reports what it would do and changes nothing (the
// transaction is rolled back). Every pattern must contain '@', so a mistyped pattern
// such as '%' cannot select every account.
func DeleteTestAccounts(ctx context.Context, pool *pgxpool.Pool, patterns []string, apply bool) (TestAccounts, error) {
	if len(patterns) == 0 {
		return TestAccounts{}, errors.New("no e-mail patterns given")
	}
	for _, p := range patterns {
		if !strings.Contains(p, "@") || strings.HasPrefix(p, "%@") {
			return TestAccounts{}, fmt.Errorf("pattern %q is too broad: it needs a fixed part before and including '@'", p)
		}
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return TestAccounts{}, err
	}
	defer tx.Rollback(ctx) // the dry run ends here; after Commit it does nothing

	var result TestAccounts
	rows, err := tx.Query(ctx, `
		SELECT email FROM users WHERE email LIKE ANY($1) ORDER BY email FOR UPDATE`, patterns)
	if err != nil {
		return TestAccounts{}, fmt.Errorf("find accounts: %w", err)
	}
	if result.Emails, err = pgx.CollectRows(rows, pgx.RowTo[string]); err != nil {
		return TestAccounts{}, err
	}
	if len(result.Emails) == 0 {
		return result, nil
	}

	tag, err := tx.Exec(ctx, `
		UPDATE license_keys
		SET status = 'available', order_item_id = NULL, reserved_until = NULL
		WHERE order_item_id IN (
			SELECT i.id FROM order_items i
			JOIN orders o ON o.id = i.order_id
			JOIN users u ON u.id = o.user_id
			WHERE u.email LIKE ANY($1)
		)`, patterns)
	if err != nil {
		return TestAccounts{}, fmt.Errorf("return keys to stock: %w", err)
	}
	result.KeysReturned = int(tag.RowsAffected())

	tag, err = tx.Exec(ctx, `
		DELETE FROM orders WHERE user_id IN (SELECT id FROM users WHERE email LIKE ANY($1))`, patterns)
	if err != nil {
		return TestAccounts{}, fmt.Errorf("delete orders: %w", err)
	}
	result.Orders = int(tag.RowsAffected())

	if _, err := tx.Exec(ctx, `DELETE FROM users WHERE email LIKE ANY($1)`, patterns); err != nil {
		return TestAccounts{}, fmt.Errorf("delete accounts: %w", err)
	}

	if apply {
		if err := tx.Commit(ctx); err != nil {
			return TestAccounts{}, err
		}
	}
	return result, nil
}
